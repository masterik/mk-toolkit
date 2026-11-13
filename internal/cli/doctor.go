package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/doctor"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report prerequisites, plugin state, sandbox grants and old-release leftovers",
		Long: "Report what this machine and this repo will let the mkit plugin do: the tools\n" +
			"its skills compose (git, gh, bash, and optional wt/codex/coderabbit), the plugin's\n" +
			"state, sandbox grants, and what older mkit releases left behind.\n\n" +
			"Reports; fixes nothing. Every finding carries a remedy you run, and a finding\n" +
			"with no working remedy says so rather than offering configuration that changes\n" +
			"nothing. Human-run: the skills never call it, and need no binary to run.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := FromContext(cmd)
			// A missing repo is a state doctor reports, never a reason to fail:
			// the machine-level checks are useful outside a work tree.
			repo, _ := gitrepo.Open("")
			report := doctor.Run(doctor.Options{Repo: repo})

			if opts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			if opts.Pretty {
				prettyDoctor(cmd.OutOrStdout(), report)
				return nil
			}
			renderDoctor(cmd.OutOrStdout(), report)
			return nil
		},
	}
}

// Exit status stays 0 for a report with findings. `mkit doctor` answering "here is
// what is wrong" has succeeded; a non-zero exit would make it unusable in the `&&`
// chains a skill writes, and the findings are the output either way.
func renderDoctor(out io.Writer, r *doctor.Report) {
	// Grouped before printing, in first-seen order. Run order is not group order —
	// the sandbox checks straddle the repo ones — and a heading printed on every
	// change prints "sandbox" twice with unrelated rows between.
	var order []string
	byGroup := map[string][]doctor.Check{}
	for _, c := range r.Checks {
		if _, seen := byGroup[c.Group]; !seen {
			order = append(order, c.Group)
		}
		byGroup[c.Group] = append(byGroup[c.Group], c)
	}
	for _, group := range order {
		_, _ = fmt.Fprintf(out, "\n%s\n", group)
		for _, c := range byGroup[group] {
			_, _ = fmt.Fprintf(out, "  %-4s %-22s %s\n", mark(c.Status), c.Name, c.Detail)
			if c.Remedy != "" {
				_, _ = fmt.Fprintf(out, "       %-22s remedy: %s\n", "", c.Remedy)
			}
		}
	}
	counts := r.Counts()
	_, _ = fmt.Fprintf(out, "\n%d ok, %d warn, %d fail, %d unknown\n",
		counts[doctor.OK], counts[doctor.Warn], counts[doctor.Fail], counts[doctor.Unknown])
}

func mark(s doctor.Status) string {
	switch s {
	case doctor.OK:
		return "ok"
	case doctor.Warn:
		return "warn"
	case doctor.Fail:
		return "FAIL"
	}
	return "?"
}
