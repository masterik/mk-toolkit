package sessionaudit

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options scopes a scan.
type Options struct {
	// Home is the Claude Code home directory; transcripts are under
	// <Home>/projects. The caller resolves it — the CLI through the storage
	// package's provider table, so there is one CLAUDE_HOME rule, not two.
	Home string
	// Days is the window: a transcript last written more than Days ago is
	// skipped whole.
	Days int
	// Now is the scan's clock; zero means time.Now().
	Now time.Time
}

// Event is one tool call the sandbox or the permission gate had a say in.
type Event struct {
	Kind Kind `json:"kind"`
	// Outcome is set on an override only: what happened to the unsandboxed call
	// — "ok", "error", or the Kind of the refusal it met.
	Outcome string `json:"outcome,omitempty"`
	// Preemptive is set on an override only: nothing in this transcript had
	// been blocked by the sandbox before it. An override that follows no block
	// was a guess that the sandbox would fail, not a response to it failing.
	Preemptive bool `json:"preemptive,omitempty"`
	// ReadOnly is set on an override only: its command head neither writes nor
	// reaches the network, so the sandbox would have allowed it.
	ReadOnly bool   `json:"read_only,omitempty"`
	Project  string `json:"project"`
	Worktree string `json:"worktree,omitempty"`
	// Cwd is the session's working directory, as the transcript recorded it: the
	// one exact path back to the checkout, since a projects directory's name
	// encodes `/` and `.` alike and cannot be decoded.
	Cwd       string   `json:"cwd,omitempty"`
	Session   string   `json:"session"`
	Subagent  bool     `json:"subagent,omitempty"`
	Timestamp string   `json:"ts"`
	Tool      string   `json:"tool"`
	Head      string   `json:"head,omitempty"`
	Command   string   `json:"command"`
	Reason    string   `json:"reason,omitempty"`
	Targets   []string `json:"targets,omitempty"`
}

// Scan reads every transcript under <Home>/projects written within the window
// and returns the events in them, each transcript's in the order they happened.
// A transcript that cannot be read is named in Report.Unreadable and skipped: one
// bad file is not a reason to report nothing about the rest.
func Scan(opts Options) (*Report, error) {
	if opts.Days <= 0 {
		return nil, errors.New("days must be positive")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	since := now.Add(-time.Duration(opts.Days) * 24 * time.Hour)
	root := filepath.Join(opts.Home, "projects")

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New(root + " is not a directory")
	}

	rep := &Report{Root: root, Days: opts.Days, Since: since.UTC().Format(time.RFC3339)}
	homePrefix := encodedHomePrefix()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			rep.Unreadable = append(rep.Unreadable, path)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			rep.Unreadable = append(rep.Unreadable, path)
			return nil
		}
		if fi.ModTime().Before(since) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		evs, err := scanFile(path, rel, homePrefix)
		if err != nil {
			rep.Unreadable = append(rep.Unreadable, path)
			return nil
		}
		rep.Transcripts++
		if len(evs) > 0 {
			rep.WithEvents++
		}
		rep.Events = append(rep.Events, evs...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	rep.aggregate()
	return rep, nil
}

// The transcript line shape, as far as a scan needs it.
type line struct {
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
	Message   *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

type toolInput struct {
	Command                   string `json:"command"`
	DangerouslyDisableSandbox bool   `json:"dangerouslyDisableSandbox"`
}

type call struct {
	name    string
	summary string
	input   toolInput
}

func scanFile(path, rel, homePrefix string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	project, worktree := projectOf(strings.SplitN(rel, string(filepath.Separator), 2)[0], homePrefix)
	base := Event{
		Project:  project,
		Worktree: worktree,
		Session:  strings.TrimSuffix(filepath.Base(rel), ".jsonl"),
		Subagent: strings.Contains(rel, string(filepath.Separator)+"subagents"+string(filepath.Separator)),
	}

	calls := map[string]call{}
	blocked := false
	cwd := ""
	var out []Event

	// A reader, not a Scanner: a transcript line carrying a large tool result
	// runs to megabytes, past any fixed token limit.
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		raw, rerr := r.ReadBytes('\n')
		if len(raw) > 0 {
			var l line
			ok := json.Unmarshal(raw, &l) == nil
			if ok && cwd == "" && l.Cwd != "" {
				cwd = l.Cwd
				base.Cwd = cwd
				for i := range out {
					out[i].Cwd = cwd
				}
			}
			if ok && l.Message != nil && len(l.Message.Content) > 0 && l.Message.Content[0] == '[' {
				var blocks []block
				if json.Unmarshal(l.Message.Content, &blocks) == nil {
					for _, b := range blocks {
						switch b.Type {
						case "tool_use":
							c := call{name: b.Name}
							_ = json.Unmarshal(b.Input, &c.input)
							c.summary = c.input.Command
							if c.summary == "" {
								c.summary = string(b.Input)
							}
							calls[b.ID] = c
						case "tool_result":
							c, ok := calls[b.ToolUseID]
							if !ok {
								continue
							}
							evs := resultEvents(base, l.Timestamp, c, resultText(b.Content), b.IsError, blocked)
							for _, e := range evs {
								if e.Kind == KindSandboxBlock {
									blocked = true
								}
							}
							out = append(out, evs...)
						}
					}
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return nil, rerr
		}
	}
	return out, nil
}

// resultEvents turns one tool result into the events it reports: at most one
// refusal or block, plus an override record when the call disabled the sandbox.
func resultEvents(base Event, ts string, c call, text string, isError, blockedBefore bool) []Event {
	kind, reason := classify(c.name, text)
	e := base
	e.Timestamp = ts
	e.Tool = c.name
	e.Command = clip(c.summary, 400)
	if c.name == "Bash" {
		e.Head = head(c.input.Command)
	}

	// This command's own report quotes every denial it found; reading it back in
	// the next scan would count each of them again. Matched anywhere in the
	// command, since a build-then-run chain puts it after a `&&`.
	if strings.Contains(c.input.Command, "mkit audit sessions") {
		return nil
	}

	var out []Event
	if c.input.DangerouslyDisableSandbox {
		o := e
		o.Kind = KindOverride
		o.Preemptive = !blockedBefore
		o.ReadOnly = readOnly[o.Head]
		o.Reason = reason
		switch {
		case kind != "":
			o.Outcome = string(kind)
		case isError:
			o.Outcome = "error"
		default:
			o.Outcome = "ok"
		}
		out = append(out, o)
		// An unsandboxed call cannot also be a sandbox block; a refusal of it is
		// still its own event, so the classifier's denials count in one place.
		if kind == KindSandboxBlock || kind == "" {
			return out
		}
	}
	if kind == "" {
		return out
	}
	e.Kind = kind
	e.Reason = reason
	if kind == KindSandboxBlock {
		e.Targets = targets(text)
	}
	return append(out, e)
}

// resultText flattens a tool result's content, which is a string or a list of
// content blocks, to the text a marker is matched against.
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return string(raw)
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// encodedHomePrefix is $HOME the way Claude Code encodes a path into a projects
// directory name, `/` and `.` both becoming `-`, with the trailing separator.
func encodedHomePrefix() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return strings.NewReplacer("/", "-", ".", "-").Replace(home) + "-"
}

// projectOf names a projects directory for a reader: the home prefix dropped,
// and a Claude-managed worktree folded into the checkout it belongs to, since
// its sandbox findings are that checkout's findings.
func projectOf(dir, homePrefix string) (project, worktree string) {
	p := dir
	if homePrefix != "" {
		p = strings.TrimPrefix(p, homePrefix)
	}
	p = strings.TrimPrefix(p, "Projects-")
	if i := strings.Index(p, "--claude-worktrees-"); i >= 0 {
		return p[:i], p[i+len("--claude-worktrees-"):]
	}
	return p, ""
}
