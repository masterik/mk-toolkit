package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/profile"
)

func newRepoProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profile",
		Short: "Report gate commands, spec store, commit rules, reviewers, merge style and kept branches",
		Long: "Report how this repo works, merging what mkit discovers with what `mkit init` pinned.\n\n" +
			"Every value is tagged `discovered` or `pinned`. The distinction is the point: a\n" +
			"discovered value is re-derived every run and cannot go stale; a pinned one captured\n" +
			"something inspection could not establish, and can.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := FromContext(cmd)
			repo, err := gitrepo.Open("")
			if err != nil {
				return err
			}
			p, err := profile.Build(repo)
			if err != nil {
				return err
			}
			if opts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(p)
			}
			renderProfile(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

func renderProfile(out io.Writer, p *profile.Profile) {
	_, _ = fmt.Fprintf(out, "repo: %s\n", p.Toplevel)
	_, _ = fmt.Fprintf(out, "config: %s (%s)\n", p.Config.Path, p.Config.State)
	if p.Config.IgnoreSource != "" {
		_, _ = fmt.Fprintf(out, "  shadowed by %s\n", p.Config.IgnoreSource)
	}
	// Printed under the config, not beside a value: an unknown key belongs to no
	// value — being attached to nothing is exactly what is wrong with it.
	for _, pb := range p.ConfigProblems {
		_, _ = fmt.Fprintf(out, "  %s %s\n", pb.Detail, tag(string(pb.Kind)))
	}

	_, _ = fmt.Fprintln(out, "\nquality gate:")
	if p.Gate.Ecosystem != "" {
		_, _ = fmt.Fprintf(out, "  ecosystem: %s\n", p.Gate.Ecosystem)
	}
	if len(p.Gate.Steps) == 0 {
		_, _ = fmt.Fprintf(out, "  %s\n", cause(p.Gate.Cause, "no gate commands found or pinned"))
	} else if p.Gate.Cause != "" {
		// Pinned steps do not mean discovery worked. Printing the cause alongside
		// them is the difference between "these are the steps" and "these are the
		// steps we were told about, having failed to look for others".
		_, _ = fmt.Fprintf(out, "  discovery: %s %s\n", p.Gate.Cause, tag("unavailable"))
	}
	for _, s := range p.Gate.Steps {
		_, _ = fmt.Fprintf(out, "  %-10s %-40s %s\n", s.Step, s.Command, tag(string(s.Source)))
	}

	_, _ = fmt.Fprintln(out, "\nspec store:")
	renderValue(out, "  store", p.Spec.Store)
	renderValue(out, "  ref", p.Spec.Ref)

	_, _ = fmt.Fprintln(out)
	renderList(out, "commit scopes", p.Scopes)
	renderValue(out, "commit subject max", p.SubjectMax)
	renderList(out, "reviewers", p.Review)
	renderValue(out, "review mode", p.ReviewMode)
	_, _ = fmt.Fprintln(out)
	renderValue(out, "merge style", p.Merge)
	renderList(out, "cleanup keep", p.Keep)

	_, _ = fmt.Fprintln(out, "\npayload:")
	if p.Payload.Found {
		v := p.Payload.Version
		if v == "" {
			v = "unknown version"
		}
		_, _ = fmt.Fprintf(out, "  %s %s (%s)\n", v, p.Payload.Dir, p.Payload.Via)
	} else {
		_, _ = fmt.Fprintf(out, "  not found — %s\n", p.Payload.Remedy)
	}
}

func renderValue(out io.Writer, label string, v profile.Value) {
	if v.Source == profile.Unavailable {
		_, _ = fmt.Fprintf(out, "%s: %s %s\n", label, cause(v.Cause, "none"), tag(string(profile.Unavailable)))
		return
	}
	_, _ = fmt.Fprintf(out, "%s: %s %s\n", label, v.Value, tag(string(v.Source)))
}

func renderList(out io.Writer, label string, l profile.List) {
	if l.Source == profile.Unavailable || len(l.Values) == 0 {
		_, _ = fmt.Fprintf(out, "%s: %s %s\n", label, cause(l.Cause, "none"), tag(string(profile.Unavailable)))
		return
	}
	_, _ = fmt.Fprintf(out, "%s: %s %s\n", label, strings.Join(l.Values, ", "), tag(string(l.Source)))
}

func tag(s string) string { return "[" + s + "]" }

func cause(c, fallback string) string {
	if c == "" {
		return fallback
	}
	return c
}
