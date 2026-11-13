package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gate"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// docsCandidateLines bounds the whole `docs_candidates:` block, header included.
const docsCandidateLines = 21

func newGateDetectCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Report the repo's quality-gate commands",
		Long: "Detect the repo's quality-gate commands. Reports candidates; never runs them,\n" +
			"never decides which one the skill should trust.",
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
			d, err := gate.Detect(repo, cfg)
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
	// source by one. Nothing escaped it, and the misalignment
	// is invisible in the output. Putting `cmd=` last on its own line means the
	// command can contain anything, including the delimiter that used to break
	// it.
	if len(d.Steps) == 0 {
		_, _ = fmt.Fprintln(out, "full=none")
	} else {
		_, _ = fmt.Fprintln(out, "full:")
		for i, s := range d.Steps {
			// `cmd=` runs to the end of the line, so the line must be a line: a
			// command containing a newline would silently become two rows and
			// every field on the second would be read as another step's.
			shown := s.Cmd
			if strings.ContainsAny(shown, "\r\n") {
				shown = "<refused: command contains a newline, which the one-step-per-line " +
					"form cannot carry — pin it as a script and name the script>"
			}
			_, _ = fmt.Fprintf(out, "  %d source=%s cmd=%s\n", i+1, s.Origin, shown)
		}
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
}

type detectStepJSON struct {
	Step   string `json:"step"`
	Cmd    string `json:"cmd"`
	Source string `json:"source"`
}

func writeDetectJSON(out io.Writer, d *gate.Detection) error {
	j := detectJSON{
		Ecosystems: nonNil(d.Ecosystems), PM: d.PM, Documented: d.Documented,
		Scripts: nonNil(d.Scripts), ScriptsState: d.ScriptsState,
		Workspaces: d.Workspaces, DocsCandidates: nonNil(d.Docs),
	}
	j.Steps = make([]detectStepJSON, 0, len(d.Steps))
	for _, s := range d.Steps {
		j.Steps = append(j.Steps, detectStepJSON{Step: s.Step, Cmd: s.Cmd, Source: string(s.Origin)})
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
