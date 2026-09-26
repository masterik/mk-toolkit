package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/masterik/mk-toolkit/internal/core/gate"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// The ported cases from tests/bats/gate-detect.bats, which was `gate-detect.sh`'s
// spec.

// field reads a `key=value` line out of the output, as the skills do.
func field(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return v
		}
	}
	return ""
}

func detectRepo(t *testing.T) string {
	t.Helper()
	repo, _ := gateRepo(t)
	return repo
}

func put(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// nodeRepo commits a package.json with the named scripts, plus a lockfile so the
// package manager is pinned.
func nodeRepo(t *testing.T, repo string, scripts ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(`{"name":"t","scripts":{`)
	for i, s := range scripts {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%q:%q", s, "true")
	}
	b.WriteString("}}\n")
	put(t, repo, "package.json", b.String())
	put(t, repo, "package-lock.json", "")
	gateGit(t, repo, "add", "-A")
	gateGit(t, repo, "commit", "-q", "-m", "node repo")
}

func TestDetectNoManifest(t *testing.T) {
	detectRepo(t)
	res := run(t, "gate", "detect")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if got := field(res.stdout, "ecosystem"); got != "none" {
		t.Errorf("ecosystem = %q", got)
	}
	if got := fullChain(res.stdout); got != "none" {
		t.Errorf("full = %q", got)
	}
	// The fast tier is gone, not emitted as `none`: it had no consumer left, and
	// pinning it would have meant a config key for an output nothing reads.
	if strings.Contains(res.stdout, "fast") {
		t.Errorf("the fast tier is still being reported:\n%s", res.stdout)
	}
}

func TestDetectNodePackageManager(t *testing.T) {
	for lockfile, want := range map[string]string{
		"":                  "npm",
		"package-lock.json": "npm",
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"bun.lock":          "bun",
	} {
		t.Run(want+"/"+lockfile, func(t *testing.T) {
			repo := detectRepo(t)
			put(t, repo, "package.json", `{"scripts":{"test":"echo t"}}`)
			if lockfile != "" {
				put(t, repo, lockfile, "")
			}
			res := run(t, "gate", "detect")
			if got := field(res.stdout, "pm"); got != want {
				t.Errorf("pm = %q, want %q", got, want)
			}
			if got := field(res.stdout, "ecosystem"); got != "node" {
				t.Errorf("ecosystem = %q", got)
			}
			if got := fullChain(res.stdout); got != want+" run test" {
				t.Errorf("full = %q", got)
			}
		})
	}
}

func TestDetectNodeFullChainIsLintTypecheckTestBuild(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint", "typecheck", "test", "build")
	res := run(t, "gate", "detect")
	want := "npm run lint|npm run typecheck|npm run test|npm run build"
	if got := fullChain(res.stdout); got != want {
		t.Errorf("full = %q, want %q", got, want)
	}
	if got := strings.Join(stepVals(res.stdout, "source"), "|"); got != "discovered|discovered|discovered|discovered" {
		t.Errorf("full_source = %q", got)
	}
	if got := field(res.stdout, "scripts"); got != "build,lint,test,typecheck" {
		t.Errorf("scripts = %q", got)
	}
	if got := field(res.stdout, "scripts_state"); got != "ok" {
		t.Errorf("scripts_state = %q", got)
	}
}

// An unreadable package.json is not a package.json with no scripts. `no-jq` is
// gone with the fork that needed it; `unreadable` is the whole vocabulary now.
func TestDetectUnreadablePackageJSONIsNotNoScripts(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "package.json", "{ this is not json")
	res := run(t, "gate", "detect")
	if res.code != 0 {
		t.Fatalf("exit %d", res.code)
	}
	if got := field(res.stdout, "scripts"); got != "none" {
		t.Errorf("scripts = %q", got)
	}
	if got := field(res.stdout, "scripts_state"); got != "unreadable" {
		t.Errorf("scripts_state = %q", got)
	}
	if got := field(res.stdout, "ecosystem"); got != "node" {
		t.Errorf("ecosystem = %q — the manifest exists, it just could not be read", got)
	}
}

func TestDetectWorkspaces(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "package.json", `{"scripts":{"test":"x"},"workspaces":["packages/*"]}`)
	if got := field(run(t, "gate", "detect").stdout, "workspaces"); got != "yes" {
		t.Errorf("workspaces = %q", got)
	}
}

func TestDetectEcosystemTable(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		eco   string
		full  string
	}{
		{
			name:  "rust",
			files: map[string]string{"Cargo.toml": "[package]\nname=\"x\"\n"},
			eco:   "rust",
			full:  "cargo clippy --all-targets -- -D warnings|cargo test|cargo build",
		},
		{
			name:  "go",
			files: map[string]string{"go.mod": "module x\n"},
			eco:   "go",
			full:  "go vet ./...|go test ./...|go build ./...",
		},
		{
			name:  "deno",
			files: map[string]string{"deno.json": "{}\n"},
			eco:   "deno",
			full:  "deno lint|deno test",
		},
		{
			name:  "dotnet",
			files: map[string]string{"x.csproj": "<Project/>\n"},
			eco:   "dotnet",
			full:  "dotnet build --nologo|dotnet test --nologo",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := detectRepo(t)
			for name, body := range c.files {
				put(t, repo, name, body)
			}
			res := run(t, "gate", "detect")
			if got := field(res.stdout, "ecosystem"); got != c.eco {
				t.Errorf("ecosystem = %q, want %q", got, c.eco)
			}
			if got := fullChain(res.stdout); got != c.full {
				t.Errorf("full = %q, want %q", got, c.full)
			}
		})
	}
}

func TestDetectPythonMypy(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "pyproject.toml", "[tool.mypy]\nstrict = true\n")
	res := run(t, "gate", "detect")
	if got := field(res.stdout, "ecosystem"); got != "python" {
		t.Errorf("ecosystem = %q", got)
	}
	full := fullChain(res.stdout)
	for _, want := range []string{"pytest -q", "mypy ."} {
		if !strings.Contains(full, want) {
			t.Errorf("full = %q, want it to contain %q", full, want)
		}
	}
}

func TestDetectMultipleEcosystems(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "package.json", `{"scripts":{"test":"x"}}`)
	put(t, repo, "go.mod", "module x\n")
	if got := field(run(t, "gate", "detect").stdout, "ecosystem"); got != "node,go" {
		t.Errorf("ecosystem = %q", got)
	}
}

// A repo whose only signal is its own `check:` target still has a gate.
func TestDetectDocumentedCheckFillsAnEmptyChain(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "Makefile", "check:\n\techo ok\n")
	res := run(t, "gate", "detect")
	if got := fullChain(res.stdout); got != "make check" {
		t.Errorf("full = %q", got)
	}
	if got := strings.Join(stepVals(res.stdout, "source"), "|"); got != "documented" {
		t.Errorf("full_source = %q", got)
	}
}

// ...but it never replaces a discovered chain. A `check:` that lints and nothing
// else would otherwise silently drop the repo's tests and build, and a gate is
// never half-capable. Reported beside the chain instead, as evidence for an
// override rather than an override.
func TestDetectDocumentedCheckNeverReplacesADiscoveredChain(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint", "test")
	put(t, repo, "justfile", "check:\n\techo ok\n")
	res := run(t, "gate", "detect")
	if got := fullChain(res.stdout); got != "npm run lint|npm run test" {
		t.Errorf("full = %q", got)
	}
	if got := field(res.stdout, "documented"); got != "just check" {
		t.Errorf("documented = %q", got)
	}
}

func TestDetectDocsCandidates(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "package.json", `{"scripts":{"test":"x"}}`)
	put(t, repo, "AGENTS.md", "Run `npm run test` before every commit.\n")
	res := run(t, "gate", "detect")
	if !strings.Contains(res.stdout, "docs_candidates:") ||
		!strings.Contains(res.stdout, "AGENTS.md:1:") ||
		!strings.Contains(res.stdout, "npm run test") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
}

func TestDetectDocsCandidatesNone(t *testing.T) {
	detectRepo(t)
	if !strings.Contains(run(t, "gate", "detect").stdout, "docs_candidates=none") {
		t.Error("want docs_candidates=none")
	}
}

// --dir locates the repo; detection still runs from the toplevel, so a manifest
// that exists only in a subdirectory is not picked up.
func TestDetectDirOnlyLocatesTheRepo(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "package.json", `{"scripts":{"test":"x"}}`)
	put(t, repo, "sub/package.json", `{"scripts":{"build":"x"}}`)
	res := run(t, "gate", "detect", "--dir", filepath.Join(repo, "sub"))
	if got := field(res.stdout, "ecosystem"); got != "node" {
		t.Errorf("ecosystem = %q", got)
	}
	if got := fullChain(res.stdout); got != "npm run test" {
		t.Errorf("full = %q, want the toplevel's manifest", got)
	}
}

func TestDetectDirThatDoesNotExist(t *testing.T) {
	repo := detectRepo(t)
	res := run(t, "gate", "detect", "--dir", filepath.Join(repo, "does-not-exist"))
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2", res.code)
	}
	if !strings.Contains(res.stderr, "cannot enter") {
		t.Errorf("stderr = %q", res.stderr)
	}
}

func TestDetectRejectsAnUnknownOption(t *testing.T) {
	detectRepo(t)
	if res := run(t, "gate", "detect", "--bogus"); res.code != 2 {
		t.Errorf("exit = %d, want 2", res.code)
	}
}

func TestDetectFailsOutsideAGitRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if res := run(t, "gate", "detect"); res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
}

// --- the gate ledger -------------------------------------------------------

// forge appends one record. Only for what a real run will never write: a head
// that no longer resolves, a proof from another tree, an epoch hours in the past.
func forge(t *testing.T, repo, cmd string, exit int, age time.Duration, fp, head string) {
	t.Helper()
	if fp == "" {
		// The current content's own hash — taken from the package rather than
		// from `gate detect`, which only prints one once the ledger is
		// non-empty, and the first forged record precedes any ledger at all.
		g, err := gitrepo.Open(repo)
		if err != nil {
			t.Fatal(err)
		}
		if fp, err = gate.Fingerprint(g); err != nil {
			t.Fatal(err)
		}
	}
	if head == "" {
		head = gateGit(t, repo, "rev-parse", "HEAD")
	}
	rec := map[string]any{
		"kind": "gate", "ts": "forged", "epoch": time.Now().Add(-age).Unix(),
		"fingerprint": fp, "step": "forged", "cmd": cmd, "exit": exit, "secs": 0,
		"head": head, "branch": "main", "skill": "test", "log": "-",
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	path := ledgerPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// A run directory to hand `gate run`, so the two halves meet on the one key they
// share — the exact command string.
func gateRun(t *testing.T, repo string, args ...string) result {
	t.Helper()
	rd := filepath.Join(repo, ".mkit", "run-detect")
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	return run(t, append([]string{"gate", "run", rd}, args...)...)
}

func TestDetectWithNoLedgerReportsEmptyAndNoFingerprint(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	res := run(t, "gate", "detect")
	if got := field(res.stdout, "gate_cache"); got != "empty" {
		t.Errorf("gate_cache = %q", got)
	}
	// Not one class, and no age bound either: there is nothing to compare
	// against, so reporting a bound would imply there was.
	for _, key := range []string{"gate_fingerprint", "gate_max_age_min"} {
		if strings.Contains(res.stdout, key+"=") {
			t.Errorf("%s was reported with nothing to classify:\n%s", key, res.stdout)
		}
	}
	// Every step's cache field is `-`, and gate_cache= carries the reason once.
	// Asserting the absence of a `full_cache=` line would pass for free now that
	// no such line exists in any state.
	for i, got := range stepVals(res.stdout, "cache") {
		if got != "-" {
			t.Errorf("step %d cache = %q, want - with no ledger", i+1, got)
		}
	}
	if len(stepCmds(res.stdout)) == 0 {
		t.Error("no steps in the full: block, so the cache assertion proves nothing")
	}
}

func TestDetectNoCacheReportsOffAndStillReportsEveryOtherFact(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	res := run(t, "gate", "detect", "--no-cache")
	if got := field(res.stdout, "gate_cache"); got != "off" {
		t.Errorf("gate_cache = %q", got)
	}
	if got := fullChain(res.stdout); got != "npm run lint" {
		t.Errorf("full = %q", got)
	}
	if got := field(res.stdout, "ecosystem"); got != "node" {
		t.Errorf("ecosystem = %q", got)
	}
}

func TestDetectAStepJustRunComesBackFresh(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	gateRun(t, repo, "lint", "--", "npm", "run", "lint")

	res := run(t, "gate", "detect")
	if got := strings.Join(stepVals(res.stdout, "cache"), "|"); got != "fresh" {
		t.Errorf("full_cache = %q", got)
	}
	if got := strings.Join(stepVals(res.stdout, "exit"), "|"); got != "0" {
		t.Errorf("full_cache_exit = %q", got)
	}
	if got := field(res.stdout, "gate_fingerprint"); len(got) != 16 {
		t.Errorf("gate_fingerprint = %q", got)
	}
}

// The flagship property, end to end. `review` gates a dirty tree, `finish`
// commits and gates again; if committing moved the key, the cache would miss
// exactly when it is meant to pay off.
func TestDetectAProofSurvivesCommittingTheTreeUnchanged(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	put(t, repo, "feature.txt", "work in progress\n")
	put(t, repo, "a.txt", "edited\n")
	gateRun(t, repo, "lint", "--", "npm", "run", "lint")

	before := run(t, "gate", "detect").stdout
	if got := strings.Join(stepVals(before, "cache"), "|"); got != "fresh" {
		t.Fatalf("full_cache = %q before the commit", got)
	}

	gateGit(t, repo, "add", "-A")
	gateGit(t, repo, "commit", "-q", "-m", "commit exactly the content the gate ran over")

	after := run(t, "gate", "detect").stdout
	if got := strings.Join(stepVals(after, "cache"), "|"); got != "fresh" {
		t.Errorf("full_cache = %q after the commit", got)
	}
	if field(before, "gate_fingerprint") != field(after, "gate_fingerprint") {
		t.Errorf("the fingerprint moved: %q -> %q",
			field(before, "gate_fingerprint"), field(after, "gate_fingerprint"))
	}
}

func TestDetectClassifies(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	gateRun(t, repo, "lint", "--", "npm", "run", "lint")

	// Drifted: the content moved after the proof.
	put(t, repo, "a.txt", "changed after the gate ran\n")
	if got := strings.Join(stepVals(run(t, "gate", "detect").stdout, "cache"), "|"); got != "drifted" {
		t.Errorf("full_cache = %q, want drifted", got)
	}

	// Failed, and still failed past the age bound: a tree proven red is worth
	// saying however old the proof is. Only a *pass* expires.
	if err := os.Remove(ledgerPath(repo)); err != nil {
		t.Fatal(err)
	}
	forge(t, repo, "npm run lint", 1, 2*time.Hour, "", "")
	res := run(t, "gate", "detect")
	if got := strings.Join(stepVals(res.stdout, "cache"), "|"); got != "failed" {
		t.Errorf("full_cache = %q, want failed", got)
	}
	if got := strings.Join(stepVals(res.stdout, "age"), "|"); got != "2h" {
		t.Errorf("full_cache_age = %q", got)
	}

	// Stale: a pass past the bound.
	if err := os.Remove(ledgerPath(repo)); err != nil {
		t.Fatal(err)
	}
	forge(t, repo, "npm run lint", 0, 2*time.Hour, "", "")
	if got := strings.Join(stepVals(run(t, "gate", "detect").stdout, "cache"), "|"); got != "stale" {
		t.Errorf("full_cache = %q, want stale", got)
	}

	// Unknown head: the fingerprint matches, so only the head can disqualify
	// this record — content alone cannot tell you the proof came from a history
	// you still have.
	if err := os.Remove(ledgerPath(repo)); err != nil {
		t.Fatal(err)
	}
	forge(t, repo, "npm run lint", 0, time.Minute, "", "0000000000000000000000000000000000000dead")
	if got := strings.Join(stepVals(run(t, "gate", "detect").stdout, "cache"), "|"); got != "unknown-head" {
		t.Errorf("full_cache = %q, want unknown-head", got)
	}
}

// The three full steps get three different answers, and only their positions say
// which is which. A reader that sorted, reversed, deduped or packed them would
// still print one of each — in the wrong slot.
func TestDetectClassesLineUpPositionallyWithFull(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "package.json", `{"name":"t","scripts":{"lint":"true","test":"true","build":"exit 1"}}`)
	put(t, repo, "package-lock.json", "")
	gateGit(t, repo, "add", "-A")
	gateGit(t, repo, "commit", "-q", "-m", "node repo")

	gateRun(t, repo, "test", "--", "npm", "run", "test")
	if res := gateRun(t, repo, "build", "--", "npm", "run", "build"); res.code != 1 {
		t.Fatalf("build exit = %d, want 1", res.code)
	}

	res := run(t, "gate", "detect")
	if got := fullChain(res.stdout); got != "npm run lint|npm run test|npm run build" {
		t.Fatalf("full = %q", got)
	}
	if got := strings.Join(stepVals(res.stdout, "cache"), "|"); got != "none|fresh|failed" {
		t.Errorf("full_cache = %q", got)
	}
	if got := strings.Join(stepVals(res.stdout, "exit"), "|"); got != "-|0|1" {
		t.Errorf("full_cache_exit = %q", got)
	}
	if got := strings.Join(stepVals(res.stdout, "age"), "|"); !strings.HasPrefix(got, "-|") {
		t.Errorf("full_cache_age = %q", got)
	}
}

// A different check that happens to share a step name proves nothing about it.
func TestDetectTheKeyIsTheExactCommandNotTheStepName(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint", "test")
	forge(t, repo, "npm run lint", 0, time.Minute, "", "")
	forge(t, repo, "npm run test --coverage", 0, time.Minute, "", "")
	if got := strings.Join(stepVals(run(t, "gate", "detect").stdout, "cache"), "|"); got != "fresh|none" {
		t.Errorf("full_cache = %q", got)
	}
}

func TestDetectNewestRecordWins(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	forge(t, repo, "npm run lint", 0, time.Minute, "", "")
	forge(t, repo, "npm run lint", 1, time.Minute, "", "")
	res := run(t, "gate", "detect")
	if got := strings.Join(stepVals(res.stdout, "cache"), "|"); got != "failed" {
		t.Errorf("full_cache = %q", got)
	}
	if got := strings.Join(stepVals(res.stdout, "exit"), "|"); got != "1" {
		t.Errorf("full_cache_exit = %q", got)
	}
}

// Detection itself is unaffected; the annotation is what degrades.
func TestDetectAMalformedLedgerDoesNotBreakDetection(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	gateRun(t, repo, "lint", "--", "npm", "run", "lint")
	f, err := os.OpenFile(ledgerPath(repo), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("not json at all\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	res := run(t, "gate", "detect")
	if res.code != 0 {
		t.Fatalf("exit = %d", res.code)
	}
	if got := fullChain(res.stdout); got != "npm run lint" {
		t.Errorf("full = %q", got)
	}
	if got := field(res.stdout, "ecosystem"); got != "node" {
		t.Errorf("ecosystem = %q", got)
	}
	if got := field(res.stdout, "gate_cache"); got != "empty" {
		t.Errorf("gate_cache = %q — a ledger that cannot be read has nothing to say", got)
	}
}

func TestDetectJSON(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint", "test")
	res := run(t, "gate", "detect", "--json", "--no-cache")
	var got struct {
		Ecosystems []string `json:"ecosystems"`
		PM         string   `json:"pm"`
		Steps      []struct {
			Step   string `json:"step"`
			Cmd    string `json:"cmd"`
			Source string `json:"source"`
		} `json:"steps"`
		ScriptsState string `json:"scripts_state"`
		CacheCause   string `json:"gate_cache"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, res.stdout)
	}
	if len(got.Ecosystems) != 1 || got.Ecosystems[0] != "node" || got.PM != "npm" {
		t.Errorf("%+v", got)
	}
	if len(got.Steps) != 2 || got.Steps[0].Step != "lint" || got.Steps[1].Source != "discovered" {
		t.Errorf("steps = %+v", got.Steps)
	}
	if got.ScriptsState != "ok" || got.CacheCause != "off" {
		t.Errorf("%+v", got)
	}
}

// Pinned wins per step name, and a pinned step discovery did not find is
// appended. After this the merge exists in exactly one place.
func TestDetectPinnedCommandsWinPerStepName(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint", "test")
	put(t, repo, ".mkit/config.toml", "version = 1\n\n[gate.commands]\ntest = \"npm run test -- --ci\"\nbuild = \"npm run build\"\n")

	res := run(t, "gate", "detect", "--no-cache")
	if got := fullChain(res.stdout); got != "npm run lint|npm run test -- --ci|npm run build" {
		t.Errorf("full = %q", got)
	}
	if got := strings.Join(stepVals(res.stdout, "source"), "|"); got != "discovered|pinned|pinned" {
		t.Errorf("full_source = %q", got)
	}
}

// A pinned command may contain `|` — `pytest -q || exit 1` is an ordinary step,
// and shell has no other way to say "fail the gate if this fails". The old
// encoding joined every command into one pipe-delimited `full=` line and read
// `full_source=` / `full_cache=` positionally against it, so a single pinned pipe
// split into two fields and shifted every later step's source and verdict by one
// — silently, because the output still looked well-formed. The block gives each
// step its own line with `cmd=` last, so the command can contain anything.
func TestDetectAPinnedCommandMayContainThePipeCharacter(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint")
	put(t, repo, ".mkit/config.toml",
		"[gate.commands]\nlint = \"npm run lint || exit 1\"\n")

	res := run(t, "gate", "detect")

	cmds := stepCmds(res.stdout)
	if len(cmds) != 1 {
		t.Fatalf("got %d steps, want 1: %q", len(cmds), cmds)
	}
	if cmds[0] != "npm run lint || exit 1" {
		t.Errorf("cmd = %q — the command lost or gained a field at the pipe", cmds[0])
	}
	// The alignment that used to shift: one step, one source, one cache verdict.
	if got := stepVals(res.stdout, "source"); len(got) != 1 || got[0] != "pinned" {
		t.Errorf("source = %q, want [pinned]", got)
	}
	if got := stepVals(res.stdout, "cache"); len(got) != 1 {
		t.Errorf("got %d cache verdicts for 1 step: %q", len(got), got)
	}
}

// Every per-step field stays aligned across a multi-step chain where one pinned
// step carries a pipe. A count that matches for one step cannot catch a shift.
func TestDetectStepFieldsStayAlignedAcrossAPinnedPipe(t *testing.T) {
	repo := detectRepo(t)
	nodeRepo(t, repo, "lint", "test", "build")
	put(t, repo, ".mkit/config.toml",
		"[gate.commands]\ntest = \"npm run test || exit 1\"\n")

	res := run(t, "gate", "detect")

	cmds := stepCmds(res.stdout)
	if len(cmds) != 3 {
		t.Fatalf("got %d steps, want 3: %q", len(cmds), cmds)
	}
	for _, key := range []string{"source", "cache", "exit", "age"} {
		if got := stepVals(res.stdout, key); len(got) != len(cmds) {
			t.Errorf("%s has %d values for %d steps: %q", key, len(got), len(cmds), got)
		}
	}
	// The pinned step is the one carrying the pipe, and it is the one marked
	// pinned — a shift would move that label onto a neighbour.
	sources := stepVals(res.stdout, "source")
	for i, c := range cmds {
		wantPinned := strings.Contains(c, "||")
		if (sources[i] == "pinned") != wantPinned {
			t.Errorf("step %d cmd=%q source=%q — the label shifted", i+1, c, sources[i])
		}
	}
}

// stepVals reads one field from every line of the `full:` block, in order.
// Joined with "|" it reproduces what the old pipe-parallel `full_source=` /
// `full_cache=` lines carried — which is the point: the data did not change, only
// the encoding that made a command containing `|` shift every field by one.
func stepVals(out, key string) []string {
	var vals []string
	for _, line := range blockLines(out) {
		for _, f := range strings.Fields(line) {
			if v, ok := strings.CutPrefix(f, key+"="); ok {
				vals = append(vals, v)
				break
			}
		}
	}
	return vals
}

// stepCmds reads each step's command. `cmd=` is last on the line and runs to the
// end of it, so a command may contain spaces, pipes, or anything else.
func stepCmds(out string) []string {
	var cmds []string
	for _, line := range blockLines(out) {
		if _, after, ok := strings.Cut(line, " cmd="); ok {
			cmds = append(cmds, after)
		}
	}
	return cmds
}

// blockLines returns the indented lines of the `full:` block.
func blockLines(out string) []string {
	var in bool
	var got []string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "full:":
			in = true
		case in && strings.HasPrefix(line, "  "):
			got = append(got, strings.TrimSpace(line))
		case in:
			return got
		}
	}
	return got
}

// fullChain is the old `full=` string, rebuilt from the block, so the assertions
// below keep reading as the chain they are about.
func fullChain(out string) string {
	c := stepCmds(out)
	if len(c) == 0 {
		return field(out, "full")
	}
	return strings.Join(c, "|")
}

// A repo's own task runner answers for the steps it defines: its `test` carries
// the flags and tools the maintainers chose, which `go test ./...` does not.
func TestDetectPrefersTheRepoRunnersRecipes(t *testing.T) {
	repo := detectRepo(t)
	put(t, repo, "go.mod", "module x\n")
	put(t, repo, "justfile", "lint:\n\tgolangci-lint run\ntest:\n\tgo test -race ./...\nrelease v:\n\techo\n")
	res := run(t, "gate", "detect")
	if got := fullChain(res.stdout); got != "just lint|go vet ./...|just test|go build ./..." {
		t.Errorf("full = %q", got)
	}
	if got := field(res.stdout, "ecosystem"); got != "go,just" {
		t.Errorf("ecosystem = %q", got)
	}
}
