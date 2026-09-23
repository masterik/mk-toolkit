// Package sessionaudit reads Claude Code session transcripts and reports every
// sandbox and permission-gate event in them: what the OS sandbox blocked, what
// ran with the sandbox disabled, and what the auto-mode classifier, a permission
// rule or the user refused.
//
// It is read-only and returns data. Deciding what a finding means for the
// user's settings is judgement, and stays in the `sandbox-audit` skill.
package sessionaudit

import (
	"os"
	"regexp"
	"strings"
)

// Kind is what a tool result says happened to its call.
type Kind string

const (
	// KindSandboxBlock is a sandboxed Bash call the OS sandbox denied.
	KindSandboxBlock Kind = "sandbox_block"
	// KindOverride is a Bash call that ran with dangerouslyDisableSandbox. It is
	// recorded whatever happened to it; Outcome says what did.
	KindOverride Kind = "override"
	// KindAutoModeDeny is a call the auto-mode classifier refused.
	KindAutoModeDeny Kind = "automode_deny"
	// KindRuleDeny is a call a permissions.deny rule refused.
	KindRuleDeny Kind = "rule_deny"
	// KindUserDeny is a call the user rejected at the prompt.
	KindUserDeny Kind = "user_deny"
)

// The markers, verbatim as Claude Code writes them into a tool result. Each is
// matched against the result of the call it belongs to, never against a whole
// transcript: a session that reads a doc quoting one of these phrases is not a
// session that hit it, and substring-matching the file counted every such read.
//
// The three refusals are also anchored at the start of the result, because that
// is where Claude Code puts them: a refusal is the whole result, and a result
// that merely contains the sentence is output — a `cat` of this package, say.
const (
	markerAutoMode = "Permission for this action was denied by the Claude Code auto mode classifier"
	markerUserDeny = "The user doesn't want to proceed"
	markerEPERM    = "Operation not permitted"
)

var (
	reRuleDeny   = regexp.MustCompile(`^Permission to use .* has been denied`)
	reReason     = regexp.MustCompile(`(?s)Reason: (.*?)(?:\. If you have other tasks|$)`)
	reViolations = regexp.MustCompile(`(?s)<sandbox_violations>(.*?)</sandbox_violations>`)
	reDenyLine   = regexp.MustCompile(`^deny (\S+) (\S+)`)
	// An EPERM the way a failing tool prints it: strerror after a colon or an
	// `[Errno 1]`, as in `mkdir: /x: Operation not permitted`.
	reEPERMLine = regexp.MustCompile(`[:\]]\s*` + markerEPERM)
)

// denials is the `deny <operation> <target>` entries of a complete
// `<sandbox_violations>` block, normalized. A block is only evidence when it
// names something: the bare tag in a result is output that mentions it — a
// `cat` of a doc — and a block with no entry names nothing to act on.
func denials(text string) []string {
	var out []string
	for _, m := range reViolations.FindAllStringSubmatch(text, -1) {
		for _, line := range strings.Split(m[1], "\n") {
			if d := reDenyLine.FindStringSubmatch(strings.TrimSpace(line)); d != nil {
				out = append(out, d[1]+" "+normalize(d[2]))
			}
		}
	}
	return out
}

// epermLines is the result's lines that report an EPERM, as opposed to lines
// that mention one. The exit status cannot tell them apart — `git push … | tail`
// exits 0 whatever push did, and a `grep` over docs that quote the error exits 1
// — so the line's own shape decides: a tool's error line, not prose quoting it in
// backticks, and not a line of Markdown, JSON, a diff, a comment or a numbered
// listing (`42-`, `43 │`), which is a file being read, not a command failing.
func epermLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		if !reEPERMLine.MatchString(l) || strings.Contains(l, "`") || strings.Contains(l, "│") {
			continue
		}
		if strings.ContainsAny(l[:1], "|#>*{(+-'\"") || strings.HasPrefix(l, "//") || reListing.MatchString(l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

var reListing = regexp.MustCompile(`^\d+(?:[-:]\D| )`)

// classify says what a tool result reports about its call. It returns "" when
// the result reports none of the gate or sandbox outcomes.
func classify(tool, text string) (Kind, string) {
	lead := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(lead, markerAutoMode):
		reason := text
		if m := reReason.FindStringSubmatch(text); m != nil {
			reason = m[1]
		}
		return KindAutoModeDeny, clip(strings.TrimSpace(reason), 200)
	case strings.HasPrefix(lead, markerUserDeny) && tool != "AskUserQuestion":
		// A dismissed question is the user answering, not refusing a tool call.
		return KindUserDeny, ""
	case reRuleDeny.MatchString(lead):
		return KindRuleDeny, clip(reRuleDeny.FindString(lead), 200)
	case tool == "Bash" && len(denials(text)) > 0:
		return KindSandboxBlock, ""
	case tool == "Bash" && len(epermLines(text)) > 0:
		return KindSandboxBlock, ""
	}
	return "", ""
}

// targets names what the sandbox denied, one entry per distinct target, in the
// order they appear. A `<sandbox_violations>` block is authoritative — it names
// the operation and the host or path — and only when there is none do the
// result's own EPERM lines stand in, normalized so the same denial in two
// worktrees or two temp directories groups as one.
func targets(text string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if ds := denials(text); len(ds) > 0 {
		for _, d := range ds {
			add(d)
		}
		return out
	}
	for _, line := range epermLines(text) {
		add(normalize(clip(line, 160)))
	}
	return out
}

var normalizers = []struct {
	re   *regexp.Regexp
	with string
}{
	// The Darwin per-user temp dir: its two random components differ per user.
	{regexp.MustCompile(`/var/folders/[^/\s]+/[^/\s]+/T/[^\s'":]*`), "$$DARWIN_TMPDIR/…"},
	// One worktree of many; the denial is the same whichever it is.
	{regexp.MustCompile(`\.claude/worktrees/[^/\s'":]+`), ".claude/worktrees/*"},
	{regexp.MustCompile(`\.git/worktrees/[^/\s'":]+`), ".git/worktrees/*"},
	// git naming each file a checkout could not replace: one finding, not forty.
	{regexp.MustCompile(`(unable to (?:unlink old|create file)) '?[^':]+'?`), "$1 <file>"},
	// Session ids, job ids, mktemp suffixes, pids.
	{regexp.MustCompile(`[0-9a-f]{8,}(?:-[0-9a-f]{4,})*`), "#"},
	{regexp.MustCompile(`\d{4,}`), "#"},
}

func normalize(s string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		s = strings.ReplaceAll(s, home, "~")
	}
	for _, n := range normalizers {
		s = n.re.ReplaceAllString(s, n.with)
	}
	return s
}

// readOnly is the command heads that neither write nor reach the network. An
// override on one of these was never needed: the sandbox allows reads.
var readOnly = map[string]bool{
	"cat": true, "head": true, "tail": true, "grep": true, "rg": true, "ls": true,
	"find": true, "wc": true, "stat": true, "which": true, "echo": true, "pwd": true,
	"test": true, "file": true, "jq": true, "sed -n": true, "diff": true, "tree": true,
	"git status": true, "git log": true, "git diff": true, "git show": true,
}

// verbTools are the tools whose first argument is what the call does. Their head
// keeps it: `git push` and `git status` are not the same finding.
var verbTools = map[string]bool{
	"git": true, "gh": true, "go": true, "bun": true, "npm": true, "pnpm": true,
	"yarn": true, "dotnet": true, "docker": true, "brew": true, "wt": true,
	"mkit": true, "just": true, "make": true, "cargo": true, "kubectl": true,
	"az": true, "coderabbit": true, "codex": true,
}

// reWrites finds what can write or run something unseen whatever the heads say:
// an output redirection, a substitution, a background job. `cat a > b` writes;
// `echo $(rm x)` deletes.
var reWrites = regexp.MustCompile("[>`]|\\$\\(|(?:^|[^&])&(?:[^&]|$)")

// reHarmless is the redirections that only move a read's own output around:
// stderr onto stdout, or into /dev/null. Nearly every read an agent runs ends
// in one, and counting it as a write left almost no read-only override.
var reHarmless = regexp.MustCompile(`\d?>&\d|\d?>\s*/dev/null`)

// reWritingFlag is the options that make a read-only head write or run
// something: find's actions, `sed -i`, and git's `--output=<file>`.
var reWritingFlag = regexp.MustCompile(`\s-(?:delete|exec|execdir|ok|okdir|fprint0?|fprintf|fls)\b|\s-i\b|\s--output\b`)

// reStages splits a command line into the commands it runs.
var reStages = regexp.MustCompile(`\|\||&&|[|;\n]`)

// isReadOnly reports whether a command only reads: every command in it —
// pipeline stages and `;`/`&&`/`||` sequences alike — is a read-only one, and
// nothing but a harmless redirection writes. `cat a; echo ---; cat b` reads;
// `cat f | tee g` and `git status && rm -rf x` do not.
func isReadOnly(command string) bool {
	c := reHarmless.ReplaceAllString(strings.TrimSpace(command), "")
	if c == "" || reWrites.MatchString(c) {
		return false
	}
	for _, stage := range reStages.Split(c, -1) {
		stage = strings.TrimSpace(stage)
		if stage == "" {
			continue
		}
		if f := strings.Fields(stage); f[0] == "cd" {
			continue
		}
		if !readOnly[head(stage)] || reWritingFlag.MatchString(stage) {
			return false
		}
	}
	return true
}

var (
	rePrelude = regexp.MustCompile(`^(?:(?:cd|export)\s+[^;&\n]+(?:&&|;|\n)\s*|[A-Za-z_][A-Za-z0-9_]*=\S*\s+)+`)
	reSedN    = regexp.MustCompile(`^sed\s+-n\b`)
)

// head reduces a command to what it runs: the leading `cd …&&`, `export …;` and
// `VAR=value` prefixes dropped, the binary's basename, and — for a verb tool —
// its subcommand, skipping `git -C <path>`.
func head(command string) string {
	c := rePrelude.ReplaceAllString(strings.TrimSpace(command), "")
	if reSedN.MatchString(c) {
		return "sed -n"
	}
	w := strings.Fields(c)
	if len(w) == 0 {
		return ""
	}
	h := w[0]
	if i := strings.LastIndex(h, "/"); i >= 0 {
		h = h[i+1:]
	}
	if !verbTools[h] {
		return h
	}
	rest := w[1:]
	if h == "git" && len(rest) >= 2 && rest[0] == "-C" {
		rest = rest[2:]
	}
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		return h + " " + rest[0]
	}
	return h
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Cut on a rune boundary; a transcript is UTF-8 and a split rune is not.
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n] + "…"
}
