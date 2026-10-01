package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/cache"
	"github.com/masterik/mk-toolkit/internal/core/claudecfg"
	"github.com/masterik/mk-toolkit/internal/core/sessionaudit"
)

func newAuditSandboxCmd() *cobra.Command { return auditSandboxCmd("sandbox", false) }

// newAuditSessionsCmd is the old name, kept hidden for one release: a plugin
// one release behind still opens with `mkit audit sessions`. Drop it the
// release after.
func newAuditSessionsCmd() *cobra.Command { return auditSandboxCmd("sessions", true) }

func auditSandboxCmd(use string, hidden bool) *cobra.Command {
	var days, top int
	var events, noConfig bool

	cmd := &cobra.Command{
		Use:    use,
		Hidden: hidden,
		Short:  "Report sandbox blocks, overrides and denials in past sessions, against the current settings",
		Long: "Reads every transcript under <claude home>/projects (CLAUDE_HOME, else ~/.claude)\n" +
			"written in the last --days, subagents included, and reports each tool call the\n" +
			"sandbox or the permission gate had a say in: sandbox blocks, calls run with the\n" +
			"sandbox disabled, and auto-mode, rule and user denials.\n\n" +
			"Then the settings that govern them: the user's settings.json and the project and\n" +
			"local settings of every project the events ran in, with each sandbox-block target\n" +
			"marked covered=<file>:<entry> when an entry already matches, or protected=yes when\n" +
			"it is in a path no allowlist entry can open. --no-config skips that read.\n\n" +
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
			claude, _ := cache.ByName("claude")
			rep, err := sessionaudit.Scan(sessionaudit.Options{Home: claude.ResolveHome(), Days: days})
			if err != nil {
				return err
			}
			if !noConfig {
				rep.AttachConfig(claude.ResolveHome())
			}
			if !events {
				rep.Events = nil
			}
			if FromContext(cmd).JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			if FromContext(cmd).Pretty {
				prettyAudit(cmd.OutOrStdout(), rep, top)
				return nil
			}
			renderAudit(cmd.OutOrStdout(), rep, top)
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "window: transcripts written in the last N days")
	cmd.Flags().IntVar(&top, "top", 10, "buckets to print per grouping (human output)")
	cmd.Flags().BoolVar(&noConfig, "no-config", false, "skip reading the settings files")
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
		// Settings files apart from transcripts: a counts-are-lower-bounds
		// caveat is not what "could not read settings.json" means — there it is
		// "what is configured is unknown", not "less was configured".
		if r.Config != nil && len(r.Config.Unreadable) > 0 {
			_, _ = fmt.Fprintln(out, "\nconfig_unreadable:")
			for _, p := range r.Config.Unreadable {
				_, _ = fmt.Fprintf(out, "  %s\n", p)
			}
		}
	}()
	defer renderConfig(out, r.Config)

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
		_, _ = fmt.Fprintf(out, "  %5d  %s  [%s]%s\n", b.Count, b.Key, strings.Join(b.Projects, ", "), coverMark(b))
	}
}

// coverMark is the config's say on a bucket, as the key=value tail a skill reads.
func coverMark(b sessionaudit.Bucket) string {
	switch {
	case b.CoveredBy != nil:
		return fmt.Sprintf("  covered=%s:%s", b.CoveredBy.File, b.CoveredBy.Entry)
	case b.Protected:
		return "  protected=yes"
	}
	return ""
}

// renderConfig prints what the settings say: the config home, then each file
// with the keys the audit reasons about, tagged by the file they came from.
func renderConfig(out io.Writer, c *claudecfg.Result) {
	if c == nil {
		return
	}
	_, _ = fmt.Fprintf(out, "\nconfig_dir=%s\nclaude_md=%s\n", c.Home, yesNo(c.ClaudeMD))
	bad := map[string]bool{}
	for _, p := range c.Unreadable {
		bad[p] = true
	}
	switch {
	case c.User != nil:
		renderConfigFile(out, *c.User)
	case bad[filepath.Join(c.Home, "settings.json")]:
		// Not "absent": the file is there and could not be read.
		_, _ = fmt.Fprintln(out, "user_settings=unreadable")
	default:
		_, _ = fmt.Fprintln(out, "user_settings=absent")
	}
	for _, p := range c.Projects {
		_, _ = fmt.Fprintf(out, "project_root=%s\n", p.Root)
		if len(p.Files) == 0 {
			state := "none"
			for _, name := range []string{"settings.json", "settings.local.json"} {
				if bad[filepath.Join(p.Root, ".claude", name)] {
					state = "unreadable"
				}
			}
			_, _ = fmt.Fprintf(out, "  settings=%s\n", state)
		}
		for _, f := range p.Files {
			renderConfigFile(out, f)
		}
	}
}

func renderConfigFile(out io.Writer, f claudecfg.File) {
	_, _ = fmt.Fprintf(out, "  settings=%s scope=%s\n", f.Path, f.Scope)
	for _, kv := range configLists(f) {
		_, _ = fmt.Fprintf(out, "    %s: %s\n", kv.key, strings.Join(kv.vals, " | "))
	}
}

type configList struct {
	key  string
	vals []string
}

// configLists is a file's non-empty extracted keys in print order, shared by the
// plain and the styled form so neither omits what the other shows. env lists
// names only — values can be secrets.
func configLists(f claudecfg.File) []configList {
	var out []configList
	for _, kv := range []configList{
		{"allowed_domains", f.AllowedDomains}, {"allow_write", f.AllowWrite},
		{"excluded_commands", f.ExcludedCommands}, {"allow", f.Allow}, {"ask", f.Ask},
		{"deny", f.Deny}, {"additional_directories", f.AdditionalDirs},
		{"automode_allow", f.AutoModeAllow}, {"automode_soft_deny", f.AutoModeSoftDeny},
	} {
		if len(kv.vals) > 0 {
			out = append(out, kv)
		}
	}
	if len(f.Env) > 0 {
		keys := make([]string, 0, len(f.Env))
		for k := range f.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out = append(out, configList{"env", keys})
	}
	if len(f.UnknownKeys) > 0 {
		out = append(out, configList{"unknown_keys", f.UnknownKeys})
	}
	return out
}
