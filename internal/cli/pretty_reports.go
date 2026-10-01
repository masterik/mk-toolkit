package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/masterik/mk-toolkit/internal/core/cache"
	"github.com/masterik/mk-toolkit/internal/core/claudecfg"
	"github.com/masterik/mk-toolkit/internal/core/sessionaudit"
	"github.com/masterik/mk-toolkit/internal/tui/ui"
)

// prettyCache renders a `cache prune` report. apply is the mode; result is nil
// for a dry run.
func prettyCache(out io.Writer, report *cache.Report, apply bool, result *cache.Result) {
	s := ui.New(out)
	mode := s.Pill("DRY-RUN", "accent")
	if apply {
		mode = s.Pill("APPLY", "warn")
	}
	_, _ = fmt.Fprintf(out, "%s  %s  %s\n", s.Title("mkit cache prune", ""), mode,
		s.Dim(fmt.Sprintf("older than %dd", report.Days)))

	for _, p := range report.Providers {
		name := p.Name
		if prov, ok := cache.ByName(p.Name); ok {
			name = prov.DisplayName
		}
		_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(name))
		_, _ = fmt.Fprintln(out, s.Dim("  "+p.Home))

		var max int64
		labelW := 0
		for _, c := range p.Categories {
			if c.Bytes > max {
				max = c.Bytes
			}
			if w := lipgloss.Width(c.Label); w > labelW {
				labelW = w
			}
		}
		for _, c := range p.Categories {
			frac := 0.0
			if max > 0 {
				frac = float64(c.Bytes) / float64(max)
			}
			pad := strings.Repeat(" ", labelW-lipgloss.Width(c.Label))
			if len(c.Entries) == 0 {
				_, _ = fmt.Fprintf(out, "  %s%s  %s  %s\n", s.Dim(c.Label), pad, s.Bar(0, 16), s.Dim("nothing to prune"))
				continue
			}
			unit := "files"
			if len(c.Entries) == 1 {
				unit = "file"
			}
			if c.Kind == cache.KindStaleDirs {
				unit = "dirs"
				if len(c.Entries) == 1 {
					unit = "dir"
				}
			}
			_, _ = fmt.Fprintf(out, "  %s%s  %s  %s  %s\n", c.Label, pad, s.Bar(frac, 16),
				s.Bold(fmt.Sprintf("%9s", cache.HumanBytes(c.Bytes))), s.Dim(fmt.Sprintf("%d %s", len(c.Entries), unit)))
		}
	}

	if apply && result != nil {
		_, _ = fmt.Fprintf(out, "\n%s %s %s\n", s.Icon("ok"), s.Bold("reclaimed"), s.Good(cache.HumanBytes(result.Bytes)))
		if len(result.Skipped) > 0 {
			_, _ = fmt.Fprintf(out, "%s %d paths skipped\n", s.Icon("warn"), len(result.Skipped))
		}
		if n := reportErrorCount(report) + len(result.Errors); n > 0 {
			_, _ = fmt.Fprintf(out, "%s %d paths unreadable\n", s.Icon("warn"), n)
		}
		return
	}
	total := report.TotalBytes()
	_, _ = fmt.Fprintf(out, "\n%s %s %s\n", s.Icon("ok"), s.Bold("reclaimable"), s.Accent(cache.HumanBytes(total)))
	if total > 0 {
		_, _ = fmt.Fprintf(out, "%s\n", s.Dim("  dry run — re-run with --apply to delete"))
	}
	if n := reportErrorCount(report); n > 0 {
		_, _ = fmt.Fprintf(out, "%s %d paths unreadable\n", s.Icon("warn"), n)
	}
}

// count colours a tally: dim at zero, the given tone above it.
func count(s *ui.S, n int, tone func(string) string) string {
	t := fmt.Sprintf("%d", n)
	if n == 0 {
		return s.Dim(t)
	}
	return tone(t)
}

func prettyAudit(out io.Writer, r *sessionaudit.Report, top int) {
	s := ui.New(out)
	c := r.Counts
	_, _ = fmt.Fprintf(out, "%s  %s\n", s.Title("mkit audit sandbox", ""),
		s.Dim(fmt.Sprintf("last %dd · %d transcripts · %d with events", r.Days, r.Transcripts, r.WithEvents)))
	_, _ = fmt.Fprintln(out, s.Dim("  "+r.Root))

	// Unreadable transcripts first: every count below is a lower bound, and a scan
	// that read nothing must not look like an audit that found nothing.
	if len(r.Unreadable) > 0 {
		_, _ = fmt.Fprintf(out, "\n%s %s\n", s.Icon("warn"),
			s.Warn(fmt.Sprintf("%d transcripts unreadable — the counts below are lower bounds", len(r.Unreadable))))
		for _, p := range r.Unreadable {
			_, _ = fmt.Fprintf(out, "  %s\n", s.Dim(p))
		}
		if r.Transcripts == 0 {
			// The settings were read independently of the transcripts.
			prettyConfig(out, s, r.Config)
			_, _ = fmt.Fprintf(out, "\n%s\n", s.Dim("nothing could be read, so there is no audit to show"))
			return
		}
		_, _ = fmt.Fprintln(out)
	}

	const lw = 22
	_, _ = fmt.Fprintln(out, s.Panel(
		s.KV("sandbox blocks", count(s, c.SandboxBlocks, s.Warn), lw),
		s.KV("overrides", fmt.Sprintf("%s  %s", count(s, c.Overrides, s.Warn),
			s.Dim(fmt.Sprintf("%d preemptive · %d read-only", c.OverridesPreemptive, c.OverridesReadOnly))), lw),
		s.KV("auto-mode denials", count(s, c.AutoModeDenials, s.Bad), lw),
		s.KV("rule denials", count(s, c.RuleDenials, s.Bad), lw),
		s.KV("user denials", count(s, c.UserDenials, s.Bad), lw),
	))

	if len(r.OverrideOutcomes) > 0 {
		keys := make([]string, 0, len(r.OverrideOutcomes))
		for k := range r.OverrideOutcomes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s %d", k, r.OverrideOutcomes[k]))
		}
		_, _ = fmt.Fprintf(out, "%s %s\n", s.Dim("override outcomes"), strings.Join(parts, s.Dim("  ·  ")))
	}

	prettyBuckets(out, s, "sandbox blocks by target", r.BlockTargets, top)
	prettyBuckets(out, s, "sandbox blocks by command", r.BlockHeads, top)
	prettyBuckets(out, s, "overrides by command", r.OverrideHeads, top)
	prettyBuckets(out, s, "auto-mode denials by reason", r.AutoModeReasons, top)

	if len(r.Projects) > 0 {
		_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("by project"))
		rows := make([][]string, 0, len(r.Projects))
		for _, p := range r.Projects {
			rows = append(rows, []string{p.Project,
				fmt.Sprint(p.SandboxBlocks), fmt.Sprint(p.Overrides), fmt.Sprint(p.OverridesPreemptive),
				fmt.Sprint(p.AutoModeDenials), fmt.Sprint(p.RuleDenials), fmt.Sprint(p.UserDenials)})
		}
		_, _ = fmt.Fprintln(out, s.Table([]string{"PROJECT", "BLOCKS", "OVERRIDES", "PREEMPT", "AUTO", "RULE", "USER"}, rows,
			func(_, col int, text string) lipgloss.Style {
				if col > 0 {
					if text == "0" {
						return s.DimStyle()
					}
					return s.WarnStyle()
				}
				return s.Style()
			}))
	}
	prettyConfig(out, s, r.Config)
}

func prettyBuckets(out io.Writer, s *ui.S, title string, bs []sessionaudit.Bucket, top int) {
	if len(bs) == 0 {
		return
	}
	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(title))
	max := bs[0].Count
	for _, b := range bs {
		if b.Count > max {
			max = b.Count
		}
	}
	for i, b := range bs {
		if top > 0 && i == top {
			_, _ = fmt.Fprintf(out, "  %s\n", s.Dim(fmt.Sprintf("… %d more (--top 0 for all)", len(bs)-top)))
			break
		}
		frac := 0.0
		if max > 0 {
			frac = float64(b.Count) / float64(max)
		}
		mark := ""
		switch {
		case b.CoveredBy != nil:
			mark = "  " + s.Good("covered by "+filepath.Base(b.CoveredBy.File)+": "+b.CoveredBy.Entry)
		case b.Protected:
			mark = "  " + s.Warn("protected path")
		}
		_, _ = fmt.Fprintf(out, "  %s  %s  %s  %s%s\n", s.Bold(fmt.Sprintf("%5d", b.Count)), s.Bar(frac, 10), b.Key,
			s.Dim("["+strings.Join(b.Projects, ", ")+"]"), mark)
	}
}

func prettyConfig(out io.Writer, s *ui.S, c *claudecfg.Result) {
	if c == nil {
		return
	}
	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("config"))
	_, _ = fmt.Fprintf(out, "  %s\n", s.Dim(c.Home+" · CLAUDE.md "+yesNo(c.ClaudeMD)))
	if len(c.Unreadable) > 0 {
		_, _ = fmt.Fprintf(out, "%s %s\n", s.Icon("warn"),
			s.Warn(fmt.Sprintf("%d settings files unreadable — what is configured there is unknown", len(c.Unreadable))))
		for _, p := range c.Unreadable {
			_, _ = fmt.Fprintf(out, "  %s\n", s.Dim(p))
		}
	}
	rows := [][]string{}
	for _, f := range c.Files() {
		rows = append(rows, []string{f.Scope, f.Path,
			fmt.Sprint(len(f.AllowedDomains)), fmt.Sprint(len(f.AllowWrite)), fmt.Sprint(len(f.ExcludedCommands)),
			fmt.Sprint(len(f.Allow) + len(f.Ask) + len(f.Deny)), strings.Join(f.UnknownKeys, ", ")})
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintf(out, "  %s\n", s.Dim("no settings files found"))
		return
	}
	_, _ = fmt.Fprintln(out, s.Table([]string{"SCOPE", "FILE", "DOMAINS", "WRITE", "EXCL", "RULES", "UNKNOWN KEYS"}, rows,
		func(_, _ int, _ string) lipgloss.Style { return s.Style() }))
	// The values themselves, as the plain form lists them: a count says a file
	// has entries, not which.
	for _, f := range c.Files() {
		_, _ = fmt.Fprintf(out, "\n  %s\n", s.Bold(f.Path))
		for _, kv := range configLists(f) {
			_, _ = fmt.Fprintf(out, "    %s  %s\n", s.Dim(kv.key+":"), strings.Join(kv.vals, s.Dim(" | ")))
		}
	}
}
