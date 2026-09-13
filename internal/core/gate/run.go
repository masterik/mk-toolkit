package gate

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// failPat is what a failure looks like in a build log, as a bounded excerpt for
// an agent that must not be handed the log itself.
var failPat = regexp.MustCompile(`FAIL|FAILED|Error:|error:|error\[|ERROR|✗|✖|✘|panic:|Exception|AssertionError|not ok |Traceback|\[error\]|failed with`)

// Step is one gate step: the name it is reported and logged under, the command
// bash executes, and the command the ledger is keyed on.
//
// Cmd and Norm differ only for the single-step call form, and the difference is
// load-bearing. See ParseSingle.
type Step struct {
	Name string
	Cmd  string
	Norm string
}

// RunOptions are `gate run`'s knobs. Zero Tail/Grep mean "print none".
type RunOptions struct {
	RunDir    string
	Tail      int
	Grep      int
	KeepGoing bool
	NoLedger  bool
}

// StepResult is one finished step. Failures and Tail are empty on a pass —
// nothing bounded is worth printing about a step that passed.
type StepResult struct {
	Step     string
	Cmd      string
	Exit     int
	Secs     int
	Log      string
	LogLines int
	// Failures are `<line>:<text>` hits, capped at RunOptions.Grep.
	Failures []string
	// Tail is the last RunOptions.Tail lines of the log.
	Tail []string
}

// Result is the whole invocation. Exit is the failing step's own code.
type Result struct {
	Steps      []StepResult
	FailedStep string
	Exit       int
}

// ParseSingle builds the step for `<step> -- <command...>`.
//
// argv is single-quoted for execution (doubling any embedded quote), because
// joining it on spaces would let bash re-split it and `-- printf '[%s]\n' 'foo
// bar'` would run as two arguments.
//
// The ledger key is the *pre-quoting* form, and that seam is the whole reason a
// `review` → `finish` lookup hits: `commit`/`review` call this form while
// `finish`/`pr` call --chain, so keyed on the quoted string the flagship lookup
// would miss every time and the feature would appear to work while caching
// nothing.
//
// The join is only reached when every argument survives the round trip. Joining
// argv on single spaces is otherwise lossy — `printf '%s' 'foo bar'` and
// `printf '%s' foo bar` join to the same string while executing differently, so
// a key built from it could serve one command's proof for the other. A false
// `fresh` is the one direction a ledger may never be wrong in.
func ParseSingle(name string, argv []string) Step {
	quoted := make([]string, 0, len(argv))
	joinable := true
	for _, arg := range argv {
		if strings.ContainsAny(arg, " \t\n\v\f\r") {
			joinable = false
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", `'\''`)+"'")
	}
	s := Step{Name: name, Cmd: strings.Join(quoted, " ")}
	if joinable {
		s.Norm = strings.Join(argv, " ")
	} else {
		s.Norm = s.Cmd
	}
	return s
}

// ParseChain builds the step for one `<step>=<command>` spec. The right-hand
// side is already the normalized form — this form never quotes.
func ParseChain(spec string) Step {
	// `${spec%%=*}` and `${spec#*=}`: with no `=` at all both yield the whole
	// string, which is what makes Validate's Name == Cmd test catch a spec that
	// is missing its separator rather than reporting an empty command.
	name, cmd, ok := strings.Cut(spec, "=")
	if !ok {
		cmd = spec
	}
	return Step{Name: name, Cmd: cmd, Norm: cmd}
}

var slug = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Validate rejects the malformed specs, all of them caller mistakes.
func Validate(steps []Step) error {
	if len(steps) == 0 {
		return fmt.Errorf("nothing to run")
	}
	for _, s := range steps {
		if s.Name == "" || s.Name == s.Cmd {
			return fmt.Errorf("malformed step spec: %s (want 'name=command')", s.Name)
		}
		// bash -c '' exits 0 and the step prints ok — a green gate that ran
		// nothing, the one failure class quality-gate.md forbids outright.
		if s.Cmd == "" {
			return fmt.Errorf("step '%s' has an empty command: a gate cannot pass by running nothing", s.Name)
		}
		if !slug.MatchString(s.Name) {
			return fmt.Errorf("name may only contain [a-zA-Z0-9_-], got: %s", s.Name)
		}
	}
	return nil
}

// Run executes steps in order, logging each in full and returning a bounded
// verdict. It stops at the first failure unless KeepGoing.
//
// Four invariants that must all hold at once, and hand-rolled one of them is
// always the one that slips: the command's full output goes to
// <run-dir>/gate-<step>.log, the exit code is captured before anything else can
// replace it, the chain stops on the first failure, and what reaches the agent
// is a verdict plus a bounded excerpt rather than the log.
//
// Buffered rather than streamed: the command's own output never reaches stdout
// at all, so the only thing to stream is a verdict line per step. The caller
// formats.
func Run(repo *gitrepo.Repo, steps []Step, opt RunOptions) (*Result, error) {
	if err := Validate(steps); err != nil {
		return nil, err
	}
	if fi, err := os.Stat(opt.RunDir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("run directory does not exist: %s (open it with `mkit run open`)", opt.RunDir)
	}

	// --- the gate ledger ---------------------------------------------------
	// What was proven, over which content. Every line of this is best effort:
	// it may not change the exit code, the output, or where a chain stops, and
	// it never skips a step. Deciding whether a proof is good enough to skip on
	// belongs to the SKILL.md, which reads `gate detect`'s classification.
	var ledger *Ledger
	var stamp Record
	if !opt.NoLedger && repo != nil {
		// Computed ONCE, before the first step — never per step. A chain whose
		// earlier step regenerates a tracked file would otherwise record two
		// different keys for one gate run, and the later record would claim
		// proof over content that step never read.
		if fp, err := Fingerprint(repo); err == nil && fp != "" {
			ledger = OpenLedger(repo)
			stamp = Record{
				Fingerprint: fp,
				Head:        gitLine(repo, "rev-parse", "HEAD"),
				Branch:      gitLine(repo, "rev-parse", "--abbrev-ref", "HEAD"),
				Skill:       SkillFromRunDir(opt.RunDir),
			}
		}
	}

	res := &Result{}
	for _, s := range steps {
		sr := runStep(s, opt)
		res.Steps = append(res.Steps, sr)

		rec := stamp
		rec.Step, rec.Cmd, rec.Exit, rec.Secs, rec.Log = s.Name, s.Norm, sr.Exit, sr.Secs, sr.Log
		ledger.Append(rec)

		if sr.Exit != 0 {
			// Under --keep-going the *last* failure is what the verdict names,
			// which is what the shell did and what the skills read today. With
			// one failing step — the case the gate is actually run for — the
			// distinction does not arise.
			res.Exit, res.FailedStep = sr.Exit, s.Name
			if !opt.KeepGoing {
				break
			}
		}
	}
	return res, nil
}

func runStep(s Step, opt RunOptions) StepResult {
	sr := StepResult{Step: s.Name, Cmd: s.Cmd, Log: opt.RunDir + "/gate-" + s.Name + ".log"}

	f, err := os.Create(sr.Log)
	if err != nil {
		// Nowhere to put the output is not a step that passed.
		sr.Exit = 2
		return sr
	}

	start := time.Now()
	cmd := exec.Command("bash", "-c", s.Cmd)
	cmd.Stdout = f
	cmd.Stderr = f
	// Both call forms execute through `bash -c`: turning the single-step form
	// into a direct argv exec would change what `-- sh -c 'a && b'` means.
	runErr := cmd.Run()
	// Captured before anything else can replace it.
	sr.Exit = exitCode(runErr)
	sr.Secs = int(time.Since(start).Seconds())
	_ = f.Close()

	if sr.Exit == 0 {
		return sr
	}
	sr.Failures = grepLog(sr.Log, opt.Grep)
	sr.Tail, sr.LogLines = tailLog(sr.Log, opt.Tail)
	return sr
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code >= 0 {
			return code
		}
		// Killed by a signal. ExitCode is -1 here and the number is only in the
		// wait status, so reporting a bare 128 threw away which signal it was —
		// the shell recorded 143 for SIGTERM and 130 for a Ctrl-C'd step, and the
		// ledger stores this verbatim. 128+n is bash's own convention.
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return 128
	}
	// bash itself could not be started — the shell's answer for that is 127.
	return 127
}

// grepLog returns up to max `<line>:<text>` hits on the failure pattern, in file
// order. The cap is the point: what reaches the agent is an excerpt, never the
// log.
func grepLog(path string, max int) []string {
	if max <= 0 {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var hits []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan(); n++ {
		if failPat.MatchString(sc.Text()) {
			hits = append(hits, fmt.Sprintf("%d:%s", n, sc.Text()))
			if len(hits) >= max {
				break
			}
		}
	}
	return hits
}

// tailLog returns the last max lines and the total line count.
func tailLog(path string, max int) ([]string, int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer func() { _ = f.Close() }()
	var ring []string
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		n++
		if max <= 0 {
			continue
		}
		ring = append(ring, sc.Text())
		if len(ring) > max {
			ring = ring[1:]
		}
	}
	return ring, n
}

func gitLine(repo *gitrepo.Repo, args ...string) string {
	out, err := gitOut(repo.Toplevel, args...)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}
