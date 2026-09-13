package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gate"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// docsCandidateLines bounds the whole `docs_candidates:` block, header included.
const docsCandidateLines = 21

func newGateDetectCmd() *cobra.Command {
	var dir string
	var noCache bool

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Report the repo's quality-gate commands, and what the ledger already proved",
		Long: "Detect the repo's quality-gate commands. Reports candidates; never runs them,\n" +
			"never decides which one the skill should trust.\n\n" +
			"Each proposed command is annotated with what `mkit gate run` already proved over\n" +
			"the current content — `full_cache=` and friends. Nothing here skips a step: that\n" +
			"trade-off is the skill's, and a skipped step must be reported as `cached`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := gitrepo.Open(dir)
			if err != nil {
				if dir != "" {
					// A --dir that is not a directory at all is a caller
					// mistake; a directory outside a work tree is not.
					if !isDir(dir) {
						return usageErr("cannot enter: %s", dir)
					}
				}
				return err
			}
			cfg, _, err := repoconfig.Load(repo.Toplevel)
			if err != nil {
				return err
			}
			d, err := gate.Detect(repo, cfg, gate.DetectOptions{NoCache: noCache})
			if err != nil {
				return err
			}
			if FromContext(cmd).JSON {
				return writeDetectJSON(cmd.OutOrStdout(), d)
			}
			renderDetect(cmd.OutOrStdout(), d)
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "locate the repo from this path (detection still runs from the toplevel)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "drop the ledger annotation entirely")
	return cmd
}

func renderDetect(out io.Writer, d *gate.Detection) {
	if d.PM != "" {
		_, _ = fmt.Fprintf(out, "pm=%s\n", d.PM)
	}
	_, _ = fmt.Fprintf(out, "ecosystem=%s\n", orNone(strings.Join(d.Ecosystems, ",")))

	// One block, one line per step, command last.
	//
	// This was five pipe-joined lines read positionally against each other. A
	// command is arbitrary shell and `|` is legal inside one — `pytest -q || exit
	// 1` is an ordinary pinned step — so a single `[gate.commands]` entry
	// containing a pipe split into two fields and silently shifted every step's
	// source and cache verdict by one. Nothing escaped it, and the misalignment
	// is invisible in the output. Putting `cmd=` last on its own line means the
	// command can contain anything, including the delimiter that used to break
	// it.
	if len(d.Steps) == 0 {
		_, _ = fmt.Fprintln(out, "full=none")
	} else {
		_, _ = fmt.Fprintln(out, "full:")
		for i, s := range d.Steps {
			// Cache fields are `-` when there is no usable ledger; gate_cache=
			// below says why, once, rather than on every step.
			cache, exit, age := "-", "-", "-"
			if d.CacheCause == "" {
				cache = string(s.Cache.Class)
				if s.Cache.Found {
					exit = fmt.Sprint(s.Cache.Exit)
					age = ageHuman(s.Cache.Age)
				}
			}
			_, _ = fmt.Fprintf(out, "  %d source=%s cache=%s exit=%s age=%s cmd=%s\n",
				i+1, s.Origin, cache, exit, age, s.Cmd)
		}
	}
	if d.CacheCause != "" {
		_, _ = fmt.Fprintf(out, "gate_cache=%s\n", d.CacheCause)
	} else {
		_, _ = fmt.Fprintf(out, "gate_fingerprint=%s\n", d.Fingerprint)
		// Reported, not just applied. `stale` is the one class that is a
		// judgement about time rather than about content, so the number behind
		// it belongs where the skill making the skip decision can see it.
		_, _ = fmt.Fprintf(out, "gate_max_age_min=%d\n", int(gate.MaxAge.Minutes()))
	}
	if d.Documented != "" {
		// The repo's own declared check, reported beside an inferred chain
		// rather than replacing it — evidence for an override, not an override.
		_, _ = fmt.Fprintf(out, "documented=%s\n", d.Documented)
	}
	_, _ = fmt.Fprintf(out, "scripts=%s\n", orNone(strings.Join(d.Scripts, ",")))
	_, _ = fmt.Fprintf(out, "scripts_state=%s\n", d.ScriptsState)
	_, _ = fmt.Fprintf(out, "workspaces=%s\n", yesNo(d.Workspaces))

	if len(d.Docs) == 0 {
		_, _ = fmt.Fprintln(out, "docs_candidates=none")
		return
	}
	_, _ = fmt.Fprintln(out, "docs_candidates:")
	for i, line := range d.Docs {
		if i+1 >= docsCandidateLines {
			break
		}
		_, _ = fmt.Fprintln(out, line)
	}
}

// ageHuman is "6m" / "2h" / "3d". Ages are reported on every class so a human can
// always see how old a proof is.
func ageHuman(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 0:
		return "?"
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm", s/60)
	case s < 86400:
		return fmt.Sprintf("%dh", s/3600)
	default:
		return fmt.Sprintf("%dd", s/86400)
	}
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

type detectJSON struct {
	Ecosystems     []string         `json:"ecosystems"`
	PM             string           `json:"pm,omitempty"`
	Steps          []detectStepJSON `json:"steps"`
	Documented     string           `json:"documented,omitempty"`
	Scripts        []string         `json:"scripts"`
	ScriptsState   string           `json:"scripts_state"`
	Workspaces     bool             `json:"workspaces"`
	DocsCandidates []string         `json:"docs_candidates"`
	Fingerprint    string           `json:"gate_fingerprint,omitempty"`
	MaxAgeMin      int              `json:"gate_max_age_min,omitempty"`
	CacheCause     string           `json:"gate_cache,omitempty"`
}

type detectStepJSON struct {
	Step   string           `json:"step"`
	Cmd    string           `json:"cmd"`
	Source string           `json:"source"`
	Cache  *detectCacheJSON `json:"cache,omitempty"`
}

type detectCacheJSON struct {
	Class      string `json:"class"`
	Exit       *int   `json:"exit,omitempty"`
	AgeSeconds *int   `json:"age_seconds,omitempty"`
}

func writeDetectJSON(out io.Writer, d *gate.Detection) error {
	j := detectJSON{
		Ecosystems: nonNil(d.Ecosystems), PM: d.PM, Documented: d.Documented,
		Scripts: nonNil(d.Scripts), ScriptsState: d.ScriptsState,
		Workspaces: d.Workspaces, DocsCandidates: nonNil(d.Docs),
		Fingerprint: d.Fingerprint, CacheCause: d.CacheCause,
	}
	if d.CacheCause == "" {
		j.MaxAgeMin = int(gate.MaxAge.Minutes())
	}
	j.Steps = make([]detectStepJSON, 0, len(d.Steps))
	for _, s := range d.Steps {
		st := detectStepJSON{Step: s.Step, Cmd: s.Cmd, Source: string(s.Origin)}
		if d.CacheCause == "" {
			c := &detectCacheJSON{Class: string(s.Cache.Class)}
			if s.Cache.Found {
				exit, age := s.Cache.Exit, int(s.Cache.Age.Seconds())
				c.Exit, c.AgeSeconds = &exit, &age
			}
			st.Cache = c
		}
		j.Steps = append(j.Steps, st)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(j)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// isDir distinguishes "that path is not a directory" (a caller mistake) from
// "that directory is not in a work tree" (a state to report).
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
