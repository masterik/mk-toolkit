package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/sessionaudit"
	"github.com/masterik/mk-toolkit/internal/core/storage"
)

func newAuditSessionsCmd() *cobra.Command {
	var days, top int
	var events bool

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Scan Claude Code transcripts for sandbox blocks, overrides and denials",
		Long: "Reads every transcript under <claude home>/projects (CLAUDE_HOME, else ~/.claude)\n" +
			"written in the last --days, subagents included, and reports each tool call the\n" +
			"sandbox or the permission gate had a say in: sandbox blocks, calls run with the\n" +
			"sandbox disabled, and auto-mode, rule and user denials.\n\n" +
			"Read-only. What a finding means for the user's settings is the sandbox-audit\n" +
			"skill's judgement, not this command's.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if days <= 0 {
				return usageErr("--days must be positive")
			}
			if top < 0 {
				return usageErr("--top must be 0 (all) or positive")
			}
			claude, _ := storage.ByName("claude")
			rep, err := sessionaudit.Scan(sessionaudit.Options{Home: claude.ResolveHome(), Days: days})
			if err != nil {
				return err
			}
			if !events {
				rep.Events = nil
			}
			if FromContext(cmd).JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			renderAudit(cmd.OutOrStdout(), rep, top)
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "window: transcripts written in the last N days")
	cmd.Flags().IntVar(&top, "top", 10, "buckets to print per grouping (human output)")
	cmd.Flags().BoolVar(&events, "events", false, "include every event in --json output")
	return cmd
}

func renderAudit(out io.Writer, r *sessionaudit.Report, top int) {
	c := r.Counts
	_, _ = fmt.Fprintf(out, "root=%s\ndays=%d\nsince=%s\ntranscripts=%d\nwith_events=%d\n",
		r.Root, r.Days, r.Since, r.Transcripts, r.WithEvents)
	_, _ = fmt.Fprintf(out, "sandbox_blocks=%d\noverrides=%d\noverrides_preemptive=%d\noverrides_read_only=%d\n",
		c.SandboxBlocks, c.Overrides, c.OverridesPreemptive, c.OverridesReadOnly)
	_, _ = fmt.Fprintf(out, "automode_denials=%d\nrule_denials=%d\nuser_denials=%d\n",
		c.AutoModeDenials, c.RuleDenials, c.UserDenials)
	if len(r.Unreadable) > 0 {
		_, _ = fmt.Fprintf(out, "unreadable=%d\n", len(r.Unreadable))
	}

	// Every outcome the scan produced, the common ones first, so a new one is
	// printed rather than silently dropped.
	var outcomes []string
	seen := map[string]bool{}
	keys := []string{"ok", "error", "automode_deny", "user_deny", "rule_deny"}
	rest := make([]string, 0, len(r.OverrideOutcomes))
	for k := range r.OverrideOutcomes {
		rest = append(rest, k)
	}
	sort.Strings(rest)
	for _, k := range append(keys, rest...) {
		if n := r.OverrideOutcomes[k]; n > 0 && !seen[k] {
			seen[k] = true
			outcomes = append(outcomes, fmt.Sprintf("%s:%d", k, n))
		}
	}
	if len(outcomes) > 0 {
		_, _ = fmt.Fprintf(out, "override_outcomes=%s\n", strings.Join(outcomes, " "))
	}

	renderBuckets(out, "sandbox blocks by target", r.BlockTargets, top)
	renderBuckets(out, "sandbox blocks by command", r.BlockHeads, top)
	renderBuckets(out, "overrides by command", r.OverrideHeads, top)
	renderBuckets(out, "auto-mode denials by reason", r.AutoModeReasons, top)

	defer func() {
		// Named, not just counted: a skill cannot say which transcripts its
		// counts are missing from a number.
		if len(r.Unreadable) > 0 {
			_, _ = fmt.Fprintln(out, "\nunreadable:")
			for _, p := range r.Unreadable {
				_, _ = fmt.Fprintf(out, "  %s\n", p)
			}
		}
	}()

	if len(r.Projects) > 0 {
		_, _ = fmt.Fprintln(out, "\nby project  (blocks / overrides, preemptive / automode / rule / user)")
		for _, p := range r.Projects {
			_, _ = fmt.Fprintf(out, "  %-40s %4d / %4d, %4d / %3d / %3d / %3d\n", p.Project,
				p.SandboxBlocks, p.Overrides, p.OverridesPreemptive, p.AutoModeDenials, p.RuleDenials, p.UserDenials)
		}
	}
}

func renderBuckets(out io.Writer, title string, bs []sessionaudit.Bucket, top int) {
	if len(bs) == 0 {
		return
	}
	_, _ = fmt.Fprintf(out, "\n%s\n", title)
	for i, b := range bs {
		if top > 0 && i == top {
			_, _ = fmt.Fprintf(out, "  … %d more (--top 0 for all)\n", len(bs)-top)
			break
		}
		_, _ = fmt.Fprintf(out, "  %5d  %s  [%s]\n", b.Count, b.Key, strings.Join(b.Projects, ", "))
	}
}
