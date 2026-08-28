package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/storage"
	tui "github.com/masterik/mk-toolkit/internal/tui/storageprune"
)

func newStoragePruneCmd() *cobra.Command {
	var days int
	var provider string
	var apply bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Report (and, with --apply, delete) stale local provider storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := FromContext(cmd)

			report, err := storage.Scan(storage.Options{Days: days, Provider: provider})
			if err != nil {
				return err
			}

			if !apply {
				return renderDryRun(cmd, opts, report)
			}

			if opts.Interactive && !opts.Yes {
				sel, ok, err := tui.Run(report)
				if err != nil {
					return err
				}
				if !ok {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "aborted — nothing deleted")
					return nil
				}
				return applyAndRender(cmd, opts, report, sel)
			}

			return applyAndRender(cmd, opts, report, storage.SelectAll(report))
		},
	}

	cmd.Flags().IntVar(&days, "days", 7, "retention window in days")
	cmd.Flags().StringVar(&provider, "provider", "all", "claude | codex | all")
	cmd.Flags().BoolVar(&apply, "apply", false, "delete instead of reporting")

	return cmd
}

func renderDryRun(cmd *cobra.Command, opts Options, report *storage.Report) error {
	if opts.JSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "mode: DRY-RUN   retention: %dd\n\n", report.Days)
	for _, p := range report.Providers {
		renderProvider(out, p, report.Days)
	}

	total := report.TotalBytes()
	_, _ = fmt.Fprintf(out, "total reclaimable: %s\n", storage.HumanBytes(total))
	if total > 0 {
		_, _ = fmt.Fprintln(out, "(dry run — re-run with --apply to delete)")
	}
	if n := reportErrorCount(report); n > 0 {
		_, _ = fmt.Fprintf(out, "%d paths unreadable\n", n)
	}
	return nil
}

func applyAndRender(cmd *cobra.Command, opts Options, report *storage.Report, sel *storage.Selection) error {
	result, err := storage.Apply(report, sel)
	if err != nil {
		return err
	}

	if opts.JSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "mode: APPLY   retention: %dd\n\n", report.Days)
	for _, p := range report.Providers {
		renderProvider(out, p, report.Days)
	}

	_, _ = fmt.Fprintf(out, "total reclaimed: %s\n", storage.HumanBytes(result.Bytes))
	if len(result.Skipped) > 0 {
		_, _ = fmt.Fprintf(out, "%d paths skipped\n", len(result.Skipped))
	}
	if n := reportErrorCount(report) + len(result.Errors); n > 0 {
		_, _ = fmt.Fprintf(out, "%d paths unreadable\n", n)
	}
	return nil
}

func renderProvider(out io.Writer, p storage.ProviderReport, days int) {
	name := p.Name
	if prov, ok := storage.ByName(p.Name); ok {
		name = prov.DisplayName
	}
	_, _ = fmt.Fprintf(out, "%s (%s):\n", name, p.Home)
	for _, c := range p.Categories {
		renderCategory(out, c, days)
	}
	_, _ = fmt.Fprintln(out)
}

func renderCategory(out io.Writer, c storage.CategoryReport, days int) {
	switch c.Kind {
	case storage.KindFiles:
		if len(c.Entries) == 0 {
			_, _ = fmt.Fprintf(out, "  %-38s nothing older than %dd\n", c.Label, days)
			return
		}
		_, _ = fmt.Fprintf(out, "  %-38s %4d files, %s\n", c.Label, len(c.Entries), storage.HumanBytes(c.Bytes))
	case storage.KindStaleDirs:
		if len(c.Entries) == 0 {
			_, _ = fmt.Fprintf(out, "  %-38s nothing stale\n", c.Label)
			return
		}
		_, _ = fmt.Fprintf(out, "  %-38s %4d dirs,  %s\n", c.Label, len(c.Entries), storage.HumanBytes(c.Bytes))
	}
}

func reportErrorCount(report *storage.Report) int {
	var n int
	for _, p := range report.Providers {
		for _, c := range p.Categories {
			n += len(c.Errors)
		}
	}
	return n
}
