package sessionaudit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A transcript is built from turns: an assistant tool_use and the user
// tool_result that answers it, the pairing a scan relies on.
type turn struct {
	tool    string
	input   map[string]any
	result  any // string, or []map[string]any content blocks
	isError bool
}

func writeTranscript(t *testing.T, home, rel string, age time.Duration, turns ...turn) {
	t.Helper()
	path := filepath.Join(home, "projects", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i, tu := range turns {
		id := "toolu_" + string(rune('a'+i))
		use := map[string]any{"type": "assistant", "timestamp": "2026-09-20T10:00:00Z", "message": map[string]any{
			"content": []any{map[string]any{"type": "tool_use", "id": id, "name": tu.tool, "input": tu.input}}}}
		res := map[string]any{"type": "user", "timestamp": "2026-09-20T10:00:01Z", "message": map[string]any{
			"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": tu.result, "is_error": tu.isError}}}}
		for _, l := range []any{use, res} {
			raw, err := json.Marshal(l)
			if err != nil {
				t.Fatal(err)
			}
			b.Write(raw)
			b.WriteByte('\n')
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if age != 0 {
		mt := time.Now().Add(-age)
		if err := os.Chtimes(path, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
}

func bash(cmd string, disable bool) map[string]any {
	in := map[string]any{"command": cmd}
	if disable {
		in["dangerouslyDisableSandbox"] = true
	}
	return in
}

const autoModeDenial = "Permission for this action was denied by the Claude Code auto mode classifier. " +
	"Reason: [Safety Bypass Flag]. If you have other tasks that don't depend on this action, continue."

func scan(t *testing.T, home string) *Report {
	t.Helper()
	rep, err := Scan(Options{Home: home, Days: 14})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func kinds(evs []Event) []string {
	var out []string
	for _, e := range evs {
		out = append(out, string(e.Kind))
	}
	return out
}

func TestEachOutcomeIsClassified(t *testing.T) {
	home := t.TempDir()
	writeTranscript(t, home, "-p-app/s1.jsonl", 0,
		turn{tool: "Bash", input: bash("curl https://x.example", false),
			result: "<sandbox_violations>\ndeny network-outbound x.example:443 (not in this command's allowed_domains)\n</sandbox_violations>"},
		turn{tool: "Bash", input: bash("git push -u origin feat 2>&1 | tail -3", false),
			result: "error: could not lock config file .git/config: Operation not permitted"},
		turn{tool: "Bash", input: bash("CODEX_UNSAFE_ALLOW_NO_SANDBOX=1 node codex.mjs", false), result: autoModeDenial, isError: true},
		turn{tool: "Bash", input: bash("grep env .gitignore", false), result: "Permission to use Bash with command grep env .gitignore has been denied.", isError: true},
		turn{tool: "Edit", input: map[string]any{"file_path": "a.go"}, result: "The user doesn't want to proceed with this tool use.", isError: true},
	)
	rep := scan(t, home)

	want := []string{"sandbox_block", "sandbox_block", "automode_deny", "rule_deny", "user_deny"}
	if got := kinds(rep.Events); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	if got := rep.Events[0].Targets; len(got) != 1 || got[0] != "network-outbound x.example:443" {
		t.Errorf("violation target = %v", got)
	}
	// `| tail` makes the push exit 0; the line's shape, not the status, finds it.
	if got := rep.Events[1].Targets; len(got) != 1 || !strings.Contains(got[0], "could not lock config file .git/config") {
		t.Errorf("EPERM target = %v", got)
	}
	if got := rep.Events[2].Reason; got != "[Safety Bypass Flag]" {
		t.Errorf("auto-mode reason = %q", got)
	}
	if rep.Counts.SandboxBlocks != 2 || rep.Counts.AutoModeDenials != 1 || rep.Counts.RuleDenials != 1 || rep.Counts.UserDenials != 1 {
		t.Errorf("counts = %+v", rep.Counts)
	}
}

// The measured failure this package exists to avoid: substring-matching a whole
// transcript counted every read of a doc that quotes the markers as a hit.
func TestQuotingAMarkerIsNotHittingIt(t *testing.T) {
	home := t.TempDir()
	doc := strings.Join([]string{
		"# ADR — state under a sandbox",
		"| `~/.claude/probe` | no | `Operation not permitted` |",
		"A write fails with `mkdir: /x: Operation not permitted`.",
		"- mkdir: /y: Operation not permitted",
		"42-# with `mkstemp failed on /z: Operation not permitted`",
		"The sentence " + autoModeDenial,
		"<sandbox_violations> is where the host is named.",
	}, "\n")
	writeTranscript(t, home, "-p-app/s1.jsonl", 0,
		turn{tool: "Bash", input: bash("cat docs/adr/0002.md", false), result: doc},
		turn{tool: "Bash", input: bash("grep -rn EPERM docs", false), result: doc, isError: true},
		turn{tool: "Read", input: map[string]any{"file_path": "docs/adr/0002.md"}, result: doc},
		turn{tool: "AskUserQuestion", input: map[string]any{}, result: "The user doesn't want to proceed with this tool use."},
	)
	rep := scan(t, home)
	// The one survivor is the `<sandbox_violations>` mention in Bash output: a
	// block that names no target. It is reported as (unnamed) rather than
	// dropped, because a real block with an empty violations list looks the same.
	for _, e := range rep.Events {
		if e.Kind != KindSandboxBlock || len(e.Targets) != 0 {
			t.Errorf("unexpected event %+v", e)
		}
	}
	if rep.Counts.AutoModeDenials != 0 || rep.Counts.UserDenials != 0 {
		t.Errorf("counts = %+v", rep.Counts)
	}
}

func TestOverridesArePreemptiveUntilABlock(t *testing.T) {
	home := t.TempDir()
	writeTranscript(t, home, "-p-app/s1.jsonl", 0,
		turn{tool: "Bash", input: bash("cat README.md", true), result: "hello"},
		turn{tool: "Bash", input: bash("cd /repo && bun run test", false), result: "error: EPERM: mkdir: /tmp/q: Operation not permitted", isError: true},
		turn{tool: "Bash", input: bash("cd /repo && bun run test", true), result: "12 passed"},
		turn{tool: "Bash", input: bash("gh pr merge 1 --merge", true), result: autoModeDenial, isError: true},
	)
	rep := scan(t, home)

	var ov []Event
	for _, e := range rep.Events {
		if e.Kind == KindOverride {
			ov = append(ov, e)
		}
	}
	if len(ov) != 3 {
		t.Fatalf("overrides = %d, want 3: %v", len(ov), kinds(rep.Events))
	}
	if !ov[0].Preemptive || !ov[0].ReadOnly || ov[0].Outcome != "ok" {
		t.Errorf("first override = %+v, want preemptive, read-only, ok", ov[0])
	}
	if ov[1].Preemptive || ov[1].Head != "bun run" {
		t.Errorf("second override = %+v, want reactive bun run", ov[1])
	}
	if ov[2].Outcome != "automode_deny" {
		t.Errorf("third override outcome = %q", ov[2].Outcome)
	}
	// The refused override is also counted as a denial, once.
	if rep.Counts.AutoModeDenials != 1 || rep.Counts.OverridesPreemptive != 1 || rep.Counts.OverridesReadOnly != 1 {
		t.Errorf("counts = %+v", rep.Counts)
	}
}

func TestWindowWorktreesAndSubagents(t *testing.T) {
	home := t.TempDir()
	block := turn{tool: "Bash", input: bash("git worktree remove x", false),
		result: "error: failed to delete '/r/.claude/worktrees/feat-a1b2c3/.vscode': Operation not permitted"}
	writeTranscript(t, home, "-p-app/s1.jsonl", 0, block)
	writeTranscript(t, home, "-p-app--claude-worktrees-feat-a1b2c3/s2.jsonl", 0, block)
	writeTranscript(t, home, "-p-app/s1/subagents/agent-1.jsonl", 0, block)
	writeTranscript(t, home, "-p-app/old.jsonl", 30*24*time.Hour, block)
	rep := scan(t, home)

	if rep.Transcripts != 3 {
		t.Errorf("transcripts = %d, want 3 (the 30-day-old one is outside the window)", rep.Transcripts)
	}
	if len(rep.Projects) != 1 || rep.Projects[0].SandboxBlocks != 3 {
		t.Errorf("projects = %+v, want one project with the worktree folded in", rep.Projects)
	}
	if len(rep.BlockTargets) != 1 || rep.BlockTargets[0].Count != 3 {
		t.Errorf("targets = %+v, want one normalized target", rep.BlockTargets)
	}
	var sub, wt int
	for _, e := range rep.Events {
		if e.Subagent {
			sub++
		}
		if e.Worktree != "" {
			wt++
		}
	}
	if sub != 1 || wt != 1 {
		t.Errorf("subagent=%d worktree=%d, want 1 and 1", sub, wt)
	}
}

func TestOwnReportIsNotReadBack(t *testing.T) {
	home := t.TempDir()
	writeTranscript(t, home, "-p-app/s1.jsonl", 0,
		turn{tool: "Bash", input: bash("go build ./... && ./mkit audit sessions --top 0", false),
			result: "sandbox blocks by target\n  9  error: could not lock config file .git/config: Operation not permitted"})
	if rep := scan(t, home); len(rep.Events) != 0 {
		t.Errorf("events = %v, want none", kinds(rep.Events))
	}
}

func TestHead(t *testing.T) {
	for cmd, want := range map[string]string{
		"cd /x && git -C /y push -u origin b": "git push",
		"export A=1; FOO=bar bun run test":    "bun run",
		"/usr/bin/git status --short":         "git status",
		"sed -n '1,5p' f":                     "sed -n",
		"python3 - <<'EOF'":                   "python3",
		"gh --repo o/r pr view":               "gh",
	} {
		if got := head(cmd); got != want {
			t.Errorf("head(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestMissingRootIsAnError(t *testing.T) {
	if _, err := Scan(Options{Home: t.TempDir(), Days: 14}); err == nil {
		t.Error("want an error for a home with no projects directory")
	}
	if _, err := Scan(Options{Home: t.TempDir(), Days: 0}); err == nil {
		t.Error("want an error for a non-positive window")
	}
}
