package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These are the ported cases from the node suite that findings.mjs was specified
// by. They exercise the interface the skills call — argv in, stdout, exit code
// and the run directory's artefacts out — not the package's internals.

type result struct {
	stdout string
	stderr string
	code   int
}

func run(t *testing.T, args ...string) result {
	t.Helper()
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	res := result{}
	if err := root.Execute(); err != nil {
		var errBuf bytes.Buffer
		res.code = Fail(&errBuf, err)
		res.stderr = errBuf.String()
	}
	res.stdout = out.String()
	return res
}

func runDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func write(t *testing.T, dir, name string, records ...map[string]any) {
	t.Helper()
	var b bytes.Buffer
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, name), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSONL(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func finding(over map[string]any) map[string]any {
	r := map[string]any{
		"surface": "code", "severity": "major", "file": "src/a.ts",
		"title": "missing null check", "body": "crashes on empty input",
		"confidence": 70, "line": 10,
	}
	for k, v := range over {
		r[k] = v
	}
	return r
}

func mustContain(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in:\n%s", want, got)
	}
}

func mustCode(t *testing.T, got result, want int) {
	t.Helper()
	if got.code != want {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", got.code, want, got.stdout, got.stderr)
	}
}

// ---------------------------------------------------------------- schema

func TestSchemaPrintsWithoutTouchingARunDir(t *testing.T) {
	got := run(t, "findings", "schema")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "findings-<source>.jsonl")
}

func TestSchemaJSONIsTheProbe(t *testing.T) {
	got := run(t, "findings", "schema", "--json")
	mustCode(t, got, 0)
	var doc struct {
		Finding struct {
			File   string `json:"file"`
			Fields []struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
			} `json:"fields"`
		} `json:"finding"`
		Verdict struct {
			File string `json:"file"`
		} `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		t.Fatalf("schema --json is not JSON: %v\n%s", err, got.stdout)
	}
	if doc.Finding.File != "findings-<source>.jsonl" || doc.Verdict.File != "verdicts-<group>.jsonl" {
		t.Fatalf("unexpected shapes: %+v", doc)
	}
	var required []string
	for _, f := range doc.Finding.Fields {
		if f.Required {
			required = append(required, f.Name)
		}
	}
	if strings.Join(required, ",") != "surface,severity,file,title" {
		t.Fatalf("required fields: %v", required)
	}
}

// ---------------------------------------------------------------- validate

func TestValidateReportsMissingRequiredFields(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", map[string]any{"severity": "major"})
	got := run(t, "findings", "validate", dir)
	mustCode(t, got, 1)
	mustContain(t, got.stdout, "missing surface")
	mustContain(t, got.stdout, "missing file")
	mustContain(t, got.stdout, "missing title")
}

func TestValidateRejectsOutOfRangeConfidenceAndNonIntegerLine(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(map[string]any{"confidence": 150, "line": 1.5}))
	got := run(t, "findings", "validate", dir)
	mustCode(t, got, 1)
	mustContain(t, got.stdout, "confidence must be 0-100")
	mustContain(t, got.stdout, "line must be an integer")
}

func TestValidatePassesAWellFormedFile(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(nil))
	got := run(t, "findings", "validate", dir)
	mustCode(t, got, 0)
	mustContain(t, got.stdout, " ok")
}

// ---------------------------------------------------------------- reconcile

func TestReconcileMergesNearLineFindingsWithAConfidenceBoost(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-codex.jsonl", finding(map[string]any{"title": "missing null check", "line": 10, "confidence": 70}))
	write(t, dir, "findings-claude.jsonl", finding(map[string]any{"title": "npe on empty input", "line": 11, "confidence": 60}))
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "2")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "findings=1")
	mustContain(t, got.stdout, "merged=1")

	records := readJSONL(t, filepath.Join(dir, "reconciled.jsonl"))
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	if records[0]["confidence"] != float64(80) { // 70 base + 10 for a second source
		t.Fatalf("confidence: %v", records[0]["confidence"])
	}
	if sources := strings.Join(anyStrings(records[0]["sources"]), ","); sources != "claude,codex" {
		t.Fatalf("sources: %v", sources)
	}
}

func TestReconcileDoesNotMergeBeyondTheLineWindow(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(map[string]any{"line": 10}))
	write(t, dir, "findings-b.jsonl", finding(map[string]any{"line": 50}))
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "2")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "findings=2")
	mustContain(t, got.stdout, "merged=0")
}

func TestReconcileNormalizesAbsoluteAgainstRepoRelativePaths(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(map[string]any{"file": "src/a.ts", "line": 10}))
	write(t, dir, "findings-b.jsonl", finding(map[string]any{"file": "/repo/src/a.ts", "line": 10}))
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "2")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "findings=1")
}

func TestReconcileDropsAWeakSingletonOnlyWhenEverySourceReported(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(map[string]any{"severity": "minor", "confidence": 50}))
	write(t, dir, "findings-b.jsonl")
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "2")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "complete=true")
	mustContain(t, got.stdout, "dropped=1")
	if n := len(readJSONL(t, filepath.Join(dir, "reconciled.jsonl"))); n != 0 {
		t.Fatalf("want an empty reconciled.jsonl, got %d records", n)
	}
}

func TestReconcileDisablesTheDropRuleWhenASourceIsMissing(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(map[string]any{"severity": "minor", "confidence": 50}))
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "2")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "complete=false")
	mustContain(t, got.stdout, "dropped=0")
	mustContain(t, got.stdout, "drop_rule=disabled")
	if n := len(readJSONL(t, filepath.Join(dir, "reconciled.jsonl"))); n != 1 {
		t.Fatalf("want 1 record, got %d", n)
	}
}

func TestReconcileKeepsNonFindingsAside(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl",
		finding(nil),
		finding(map[string]any{"class": "open_question", "title": "is this intended?"}))
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "1")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "findings=1")
	mustContain(t, got.stdout, "aside=1")

	var ids []string
	for _, r := range readJSONL(t, filepath.Join(dir, "reconciled.jsonl")) {
		ids = append(ids, r["id"].(string))
	}
	if strings.Join(ids, ",") != "f01,x01" {
		t.Fatalf("ids: %v", ids)
	}
}

func TestReconcileFailsLoudlyWithNoFindingsFiles(t *testing.T) {
	mustCode(t, run(t, "findings", "reconcile", runDir(t)), 1)
}

func TestReconcileRejectsASourcesExpectedWithNoValue(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(nil))
	got := run(t, "findings", "reconcile", dir, "--sources-expected")
	mustCode(t, got, 2)
	mustContain(t, got.stderr, "needs a numeric value")
}

func TestReconcileRejectsANonNumericFlagValue(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(nil))
	got := run(t, "findings", "reconcile", dir, "--sim", "high")
	mustCode(t, got, 2)
	mustContain(t, got.stderr, "needs a numeric value")
}

// ---------------------------------------------------------------- group

func TestGroupSplitsByDirectoryAndWritesOneVerifyFilePerGroup(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "reconciled.jsonl",
		finding(map[string]any{"id": "f01", "file": "src/a.ts"}),
		finding(map[string]any{"id": "f02", "file": "docs/readme.md"}))
	// Folding small groups together (the default min-per-group=3) would merge
	// these two single-finding directories back into one — turn that off to see
	// the raw split.
	got := run(t, "findings", "group", dir, "--min-per-group", "1")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "groups=2")
	n := len(readJSONL(t, filepath.Join(dir, "verify-g1.jsonl"))) +
		len(readJSONL(t, filepath.Join(dir, "verify-g2.jsonl")))
	if n != 2 {
		t.Fatalf("want 2 findings across the groups, got %d", n)
	}
}

func TestGroupFoldsDirectoriesSmallerThanMinPerGroup(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "reconciled.jsonl",
		finding(map[string]any{"id": "f01", "file": "src/a.ts"}),
		finding(map[string]any{"id": "f02", "file": "docs/readme.md"}))
	got := run(t, "findings", "group", dir)
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "groups=1")
}

func TestGroupWithAHandfulOfFindingsSuggestsInline(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "reconciled.jsonl", finding(map[string]any{"id": "f01"}))
	got := run(t, "findings", "group", dir)
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "suggest=inline")
}

func TestGroupFailsWithoutAPriorReconcile(t *testing.T) {
	mustCode(t, run(t, "findings", "group", runDir(t)), 1)
}

// ---------------------------------------------------------------- report

func TestReportMergesVerdictsAndFlagsUnverified(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "reconciled.jsonl",
		finding(map[string]any{"id": "f01"}),
		finding(map[string]any{"id": "f02", "title": "second"}))
	write(t, dir, "verdicts-g1.jsonl", map[string]any{"id": "f01", "verdict": "confirmed", "reason": "reproduced"})
	got := run(t, "findings", "report", dir)
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "reportable=1")
	mustContain(t, got.stdout, "UNVERIFIED=f02")

	final := readJSONL(t, filepath.Join(dir, "final.jsonl"))
	if final[0]["verdict"] != "confirmed" {
		t.Fatalf("f01 verdict: %v", final[0]["verdict"])
	}
	if final[1]["verdict"] != nil {
		t.Fatalf("f02 verdict: %v", final[1]["verdict"])
	}
}

func TestReportAppliesARefinedVerdictsCorrections(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "reconciled.jsonl", finding(map[string]any{"id": "f01", "severity": "minor"}))
	write(t, dir, "verdicts-g1.jsonl", map[string]any{
		"id": "f01", "verdict": "refined", "severity": "critical", "reason": "worse than reported"})
	mustCode(t, run(t, "findings", "report", dir), 0)

	final := readJSONL(t, filepath.Join(dir, "final.jsonl"))
	if final[0]["severity"] != "critical" || final[0]["verdict"] != "refined" {
		t.Fatalf("final: %v", final[0])
	}
}

func TestReportTreatsAnUnrecognizedVerdictAsAnOrphan(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "reconciled.jsonl", finding(map[string]any{"id": "f01"}))
	write(t, dir, "verdicts-g1.jsonl", map[string]any{"id": "f01", "verdict": "maybe"})
	got := run(t, "findings", "report", dir)
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "unknown verdict maybe")
	mustContain(t, got.stdout, "UNVERIFIED=f01")
}

func TestReportFlagsAVerdictForAnUnknownID(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "reconciled.jsonl", finding(map[string]any{"id": "f01"}))
	write(t, dir, "verdicts-g1.jsonl", map[string]any{"id": "f99", "verdict": "confirmed"})
	got := run(t, "findings", "report", dir)
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "verdict for unknown id f99")
}

func TestBadUsageExits2(t *testing.T) {
	mustCode(t, run(t, "findings"), 2)
}

// ---------------------------------------------------------------- --json path

// The skill reads --json, so every stage must be parseable as well as printable.
func TestJSONOnEveryStage(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(map[string]any{"line": 10}))
	write(t, dir, "findings-b.jsonl", finding(map[string]any{"line": 11, "title": "npe"}))

	var validate struct {
		Files  []struct{ File string } `json:"files"`
		Errors int                     `json:"errors"`
	}
	decode(t, run(t, "findings", "validate", dir, "--json"), &validate)
	if len(validate.Files) != 2 || validate.Errors != 0 {
		t.Fatalf("validate: %+v", validate)
	}

	var reconciled struct {
		Findings int    `json:"findings"`
		Merged   int    `json:"merged"`
		Complete bool   `json:"complete"`
		DropRule string `json:"drop_rule"`
	}
	decode(t, run(t, "findings", "reconcile", dir, "--sources-expected", "2", "--json"), &reconciled)
	if reconciled.Findings != 1 || reconciled.Merged != 1 || !reconciled.Complete || reconciled.DropRule != "enabled" {
		t.Fatalf("reconcile: %+v", reconciled)
	}

	var grouped struct {
		Groups []struct {
			Slug string   `json:"slug"`
			IDs  []string `json:"ids"`
		} `json:"groups"`
		Suggest string `json:"suggest"`
	}
	decode(t, run(t, "findings", "group", dir, "--json"), &grouped)
	if len(grouped.Groups) != 1 || grouped.Suggest != "inline" || grouped.Groups[0].IDs[0] != "f01" {
		t.Fatalf("group: %+v", grouped)
	}

	write(t, dir, "verdicts-g1.jsonl", map[string]any{"id": "f01", "verdict": "confirmed"})
	var report struct {
		Reportable int      `json:"reportable"`
		Gating     int      `json:"gating"`
		Unverified []string `json:"unverified"`
		Kept       []struct {
			ID      string   `json:"id"`
			Sources []string `json:"sources"`
		} `json:"kept"`
	}
	decode(t, run(t, "findings", "report", dir, "--json"), &report)
	if report.Reportable != 1 || report.Gating != 1 || len(report.Unverified) != 0 {
		t.Fatalf("report: %+v", report)
	}
	if report.Kept[0].ID != "f01" || len(report.Kept[0].Sources) != 2 {
		t.Fatalf("kept: %+v", report.Kept)
	}
}

// The judgement sentences moved to Markdown; JSON carries structured facts only.
func TestJSONCarriesNoAgentDirectedProse(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl",
		finding(map[string]any{"line": 10}),
		finding(map[string]any{"line": 11, "title": "utterly different wording", "body": "nothing alike"}))
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "1", "--json")
	mustCode(t, got, 0)
	for _, phrase := range []string{"your call", "check it is one problem", "a lost verdict", "round trip"} {
		if strings.Contains(got.stdout, phrase) {
			t.Fatalf("judgement prose %q leaked into --json:\n%s", phrase, got.stdout)
		}
	}
	var res struct {
		Merges []struct {
			LowSim []struct {
				Sim   float64 `json:"sim"`
				Title string  `json:"title"`
			} `json:"low_sim"`
		} `json:"merges"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Merges) != 1 || len(res.Merges[0].LowSim) != 1 {
		t.Fatalf("expected one flagged thin merge: %+v", res.Merges)
	}
}

// ---------------------------------------------------------------- parity

// Unknown fields round-trip: reconcile and report re-serialize records wholesale,
// and a struct with known fields would silently drop what a reviewer added.
func TestUnknownFieldsSurviveReconcileAndReport(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl", finding(map[string]any{
		"fix": "guard the nil", "reviewer_note": map[string]any{"depth": 3}}))
	mustCode(t, run(t, "findings", "reconcile", dir, "--sources-expected", "1"), 0)
	write(t, dir, "verdicts-g1.jsonl", map[string]any{"id": "f01", "verdict": "confirmed"})
	mustCode(t, run(t, "findings", "report", dir), 0)

	final := readJSONL(t, filepath.Join(dir, "final.jsonl"))
	if final[0]["fix"] != "guard the nil" {
		t.Fatalf("fix lost: %v", final[0])
	}
	note, ok := final[0]["reviewer_note"].(map[string]any)
	if !ok || note["depth"] != float64(3) {
		t.Fatalf("reviewer_note lost: %v", final[0]["reviewer_note"])
	}
}

// An empty-string source takes the filename fallback: `||=`, not `??=`.
func TestEmptySourceTakesTheFilenameFallback(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-codex.jsonl", finding(map[string]any{"source": ""}))
	mustCode(t, run(t, "findings", "reconcile", dir, "--sources-expected", "1"), 0)
	final := readJSONL(t, filepath.Join(dir, "reconciled.jsonl"))
	if got := strings.Join(anyStrings(final[0]["sources"]), ","); got != "codex" {
		t.Fatalf("sources: %v", got)
	}
}

// `line: null` is permitted, and must not read as line 0 — two unrelated
// findings in one file would merge.
func TestNullLinesDoNotMerge(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl",
		finding(map[string]any{"line": nil, "title": "one"}),
		finding(map[string]any{"line": nil, "title": "two"}))
	got := run(t, "findings", "reconcile", dir, "--sources-expected", "1")
	mustCode(t, got, 0)
	mustContain(t, got.stdout, "findings=2")
}

// Ids come from a sort with ties, so the same input must always produce the
// same ids — including across a mixed-case filename, where a locale-aware
// comparison and a byte comparison disagree.
func TestIDsAreStableAcrossTies(t *testing.T) {
	dir := runDir(t)
	write(t, dir, "findings-a.jsonl",
		finding(map[string]any{"file": "src/b.ts", "title": "b"}),
		finding(map[string]any{"file": "src/A.ts", "title": "A"}),
		finding(map[string]any{"file": "src/a.ts", "title": "a"}))
	var first []string
	for i := 0; i < 5; i++ {
		mustCode(t, run(t, "findings", "reconcile", dir, "--sources-expected", "1"), 0)
		var order []string
		for _, r := range readJSONL(t, filepath.Join(dir, "reconciled.jsonl")) {
			order = append(order, r["id"].(string)+"="+r["file"].(string))
		}
		if first == nil {
			first = order
			continue
		}
		if strings.Join(order, ",") != strings.Join(first, ",") {
			t.Fatalf("ids drifted: %v vs %v", order, first)
		}
	}
	if want := "f01=src/A.ts,f02=src/a.ts,f03=src/b.ts"; strings.Join(first, ",") != want {
		t.Fatalf("ids: %v, want %s", strings.Join(first, ","), want)
	}
}

func decode(t *testing.T, got result, into any) {
	t.Helper()
	mustCode(t, got, 0)
	if err := json.Unmarshal([]byte(got.stdout), into); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, got.stdout)
	}
}

func anyStrings(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(arr))
	for i, e := range arr {
		out[i], _ = e.(string)
	}
	return out
}
