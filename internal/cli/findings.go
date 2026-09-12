package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/findings"
)

func newFindingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "Validate, reconcile, group and report a review run's findings",
		Long: "Mechanical work on a review run's findings, over the JSONL files in a run directory.\n\n" +
			"What it does not do, on purpose: it never decides whether two findings are the same\n" +
			"*problem* (it merges on location and hands back the undecided pairs), never assigns\n" +
			"severity, never judges materiality, and never edits a file. Those are the reviewer's\n" +
			"and the skill's calls.\n\n" +
			"Exit: 0 ok, 1 malformed input, 2 bad usage.",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return usageErr("usage: mkit findings schema|validate|reconcile|group|report <run-dir>")
		},
	}
	// Every tunable gates a rule, so a missing or non-numeric value must fail
	// loudly rather than silently disable merging, LOW-SIM flagging or the drop
	// rule. pflag already refuses both; this only restates it in the script's words.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return numericFlagErr(err) })

	cmd.AddCommand(
		newFindingsSchemaCmd(),
		newFindingsValidateCmd(),
		newFindingsReconcileCmd(),
		newFindingsGroupCmd(),
		newFindingsReportCmd(),
	)
	return cmd
}

func numericFlagErr(err error) error {
	msg := err.Error()
	if name, ok := strings.CutPrefix(msg, "flag needs an argument: "); ok {
		return usageErr("%s needs a numeric value (got nothing)", name)
	}
	if strings.HasPrefix(msg, "invalid argument ") {
		value, rest, okv := strings.Cut(strings.TrimPrefix(msg, "invalid argument "), " for ")
		name, _, okn := strings.Cut(rest, " flag")
		if okv && okn {
			return usageErr("%s needs a numeric value (got %s)", strings.Trim(name, `"`), value)
		}
	}
	return &ExitError{Code: 2, Msg: msg}
}

// runDirArg is the one positional every subcommand but `schema` takes.
func runDirArg(args []string, use string) (string, error) {
	if len(args) != 1 || args[0] == "" {
		return "", usageErr("usage: mkit findings %s", use)
	}
	return args[0], nil
}

// exitFor maps a core error onto the documented status: a caller mistake is 2,
// malformed or missing input is 1.
func exitFor(err error) error {
	var usage *findings.UsageError
	if errors.As(err, &usage) {
		return &ExitError{Code: 2, Msg: usage.Msg}
	}
	var input *findings.InputError
	if errors.As(err, &input) {
		return &ExitError{Code: 1, Msg: input.Msg}
	}
	return err
}

func encodeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// ---------------------------------------------------------------- schema

func newFindingsSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print the findings and verdicts file contract",
		Long: "Print the contract each reviewer and verifier writes against.\n\n" +
			"It is also the presence probe: a `mkit` too old to know `findings` fails here,\n" +
			"at the start of a run, rather than halfway through one.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			doc := findings.Schema()
			if FromContext(cmd).JSON {
				return encodeJSON(cmd.OutOrStdout(), doc)
			}
			renderSchema(cmd.OutOrStdout(), doc)
			return nil
		},
	}
}

func renderSchema(out io.Writer, doc findings.SchemaDoc) {
	for _, shape := range []findings.ShapeDoc{doc.Finding, doc.Verdict} {
		_, _ = fmt.Fprintf(out, "%s\n", shape.File)
		for _, n := range shape.Notes {
			_, _ = fmt.Fprintf(out, "  - %s\n", n)
		}
		for _, f := range shape.Fields {
			req := "optional"
			if f.Required {
				req = "required"
			}
			typ := f.Type
			if len(f.Enum) > 0 {
				typ = strings.Join(f.Enum, "|")
			}
			_, _ = fmt.Fprintf(out, "  %-12s %-8s %s\n", f.Name, req, typ)
			if f.Note != "" {
				_, _ = fmt.Fprintf(out, "  %-12s %-8s %s\n", "", "", f.Note)
			}
		}
		_, _ = fmt.Fprintln(out)
	}
}

// ---------------------------------------------------------------- validate

func newFindingsValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "validate <run-dir>",
		Short:         "Check every findings-*.jsonl against the contract",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := runDirArg(args, "validate <run-dir>")
			if err != nil {
				return err
			}
			report, err := findings.ValidateRun(dir)
			if err != nil {
				return exitFor(err)
			}
			out := cmd.OutOrStdout()
			if FromContext(cmd).JSON {
				if err := encodeJSON(out, report); err != nil {
					return err
				}
			} else {
				for _, f := range report.Files {
					state := "ok"
					if len(f.Errors) > 0 {
						state = fmt.Sprintf("errors=%d", len(f.Errors))
					}
					_, _ = fmt.Fprintf(out, "%s n=%d %s\n", f.File, f.Records, state)
					for _, p := range truncate(f.Errors, 10) {
						_, _ = fmt.Fprintf(out, "  %s\n", p)
					}
				}
			}
			if report.Errors > 0 {
				return &ExitError{Code: 1}
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------- reconcile

func newFindingsReconcileCmd() *cobra.Command {
	opts := findings.ReconcileOptions{SourcesExpected: 3, Sim: 0.6, Band: 0.3, Window: 2}
	cmd := &cobra.Command{
		Use:   "reconcile <run-dir>",
		Short: "Merge, score and drop a run's findings into reconciled.jsonl",
		Long: "Merge on the documented key — same file, within ±window lines — then score\n" +
			"confidence from distinct sources and apply the weak-singleton drop.\n\n" +
			"Text does not gate a merge: two reviewers describing one missing `await` at\n" +
			"lines 42 and 43 share a quarter of their tokens, so any similarity gate safe\n" +
			"enough to trust would let every real duplicate through. Location decides, and\n" +
			"nothing is discarded — every member rides along on the survivor in `also`.",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := runDirArg(args, "reconcile <run-dir> --sources-expected N")
			if err != nil {
				return err
			}
			res, err := findings.Reconcile(dir, opts)
			if err != nil {
				var invalid *findings.InvalidError
				if errors.As(err, &invalid) {
					return renderInvalid(cmd, invalid)
				}
				return exitFor(err)
			}
			if FromContext(cmd).JSON {
				return encodeJSON(cmd.OutOrStdout(), res)
			}
			renderReconciled(cmd.OutOrStdout(), res)
			return nil
		},
	}
	cmd.Flags().Float64Var(&opts.SourcesExpected, "sources-expected", opts.SourcesExpected, "how many reviewers were asked to report")
	cmd.Flags().Float64Var(&opts.Sim, "sim", opts.Sim, "similarity at or above which an unmerged pair is handed back for review")
	cmd.Flags().Float64Var(&opts.Band, "band", opts.Band, "similarity below which a merge is flagged as thin")
	cmd.Flags().Float64Var(&opts.Window, "window", opts.Window, "line distance within which two findings in one file merge")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return numericFlagErr(err) })
	return cmd
}

func renderInvalid(cmd *cobra.Command, invalid *findings.InvalidError) error {
	out := cmd.OutOrStdout()
	if FromContext(cmd).JSON {
		if err := encodeJSON(out, map[string]any{"invalid": len(invalid.Problems), "errors": invalid.Problems}); err != nil {
			return err
		}
		return &ExitError{Code: 1}
	}
	_, _ = fmt.Fprintf(out, "invalid=%d\n", len(invalid.Problems))
	for _, p := range truncate(invalid.Problems, 20) {
		_, _ = fmt.Fprintln(out, p)
	}
	if n := len(invalid.Problems) - 20; n > 0 {
		_, _ = fmt.Fprintf(out, "... %d more\n", n)
	}
	return &ExitError{Code: 1}
}

func renderReconciled(out io.Writer, r *findings.Reconciled) {
	_, _ = fmt.Fprintf(out, "wrote=%s\n", r.Wrote)
	_, _ = fmt.Fprintf(out, "sources_present=%s expected=%s complete=%t\n",
		strings.Join(r.SourcesPresent, ","), num(r.SourcesExpected), r.Complete)
	_, _ = fmt.Fprintf(out, "in=%d findings=%d merged=%d dropped=%d aside=%d\n",
		r.In, r.Findings, r.Merged, r.Dropped, r.Aside)
	_, _ = fmt.Fprintf(out, "counts=%s\n", renderCounts(r.Counts))
	if r.DropRule == "disabled" {
		_, _ = fmt.Fprintln(out, "drop_rule=disabled (a source is missing or degraded)")
	}
	for _, m := range r.Merges {
		line := fmt.Sprintf("merged %s %s:%s [%s]", m.ID, m.File, joinLines(m.Lines), strings.Join(m.Sources, "+"))
		if len(m.LowSim) > 0 {
			sims := make([]string, len(m.LowSim))
			titles := make([]string, len(m.LowSim))
			for i, l := range m.LowSim {
				sims[i] = num(l.Sim)
				titles[i] = fmt.Sprintf("%q", l.Title)
			}
			line += fmt.Sprintf(" LOW-SIM %s — also titled %s", strings.Join(sims, ","), strings.Join(titles, " and "))
		}
		_, _ = fmt.Fprintln(out, line)
	}
	for _, d := range r.DroppedItems {
		_, _ = fmt.Fprintf(out, "dropped %s:%s — %s — %s\n", d.File, lineStr(d.Line), d.Title, d.Reason)
	}
	for _, a := range r.AsideItems {
		_, _ = fmt.Fprintf(out, "%s %s %s:%s — %s\n", a.ID, a.Class, a.File, lineStr(a.Line), a.Title)
	}
	if len(r.ReviewPairs) > 0 {
		_, _ = fmt.Fprintf(out, "review_pairs=%d\n", len(r.ReviewPairs))
		for _, p := range truncatePairs(r.ReviewPairs, 10) {
			_, _ = fmt.Fprintf(out, "  sim=%s %s %s:%s %q <-> %s :%s %q\n",
				num(p.Sim), pairID(p.A.ID), p.A.File, lineStr(p.A.Line), p.A.Title,
				pairID(p.B.ID), lineStr(p.B.Line), p.B.Title)
		}
		if n := len(r.ReviewPairs) - 10; n > 0 {
			_, _ = fmt.Fprintf(out, "  ... %d more\n", n)
		}
	}
}

// ---------------------------------------------------------------- group

func newFindingsGroupCmd() *cobra.Command {
	opts := findings.GroupOptions{MaxGroups: 8, MinPer: 3}
	cmd := &cobra.Command{
		Use:   "group <run-dir>",
		Short: "Split reconciled findings into verification groups",
		Long: "Fold the smallest directories together until every group is worth a round trip.\n" +
			"One verifier per *finding* is the failure mode this avoids: the subagent round\n" +
			"trip costs more than the verification.",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := runDirArg(args, "group <run-dir>")
			if err != nil {
				return err
			}
			res, err := findings.GroupRun(dir, opts)
			if err != nil {
				return exitFor(err)
			}
			if FromContext(cmd).JSON {
				return encodeJSON(cmd.OutOrStdout(), res)
			}
			out := cmd.OutOrStdout()
			for _, g := range res.Groups {
				_, _ = fmt.Fprintf(out, "%s %s n=%d ids=%s file=%s\n", g.Slug, g.Name, g.N, strings.Join(g.IDs, ","), g.File)
			}
			_, _ = fmt.Fprintf(out, "groups=%d findings=%d files=%d\n", len(res.Groups), res.Findings, res.Files)
			_, _ = fmt.Fprintf(out, "suggest=%s\n", res.Suggest)
			return nil
		},
	}
	cmd.Flags().Float64Var(&opts.MaxGroups, "max-groups", opts.MaxGroups, "most groups to produce")
	cmd.Flags().Float64Var(&opts.MinPer, "min-per-group", opts.MinPer, "fold a group smaller than this into another")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return numericFlagErr(err) })
	return cmd
}

// ---------------------------------------------------------------- report

func newFindingsReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "report <run-dir>",
		Short:         "Merge verdicts onto findings and tally what is reportable",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := runDirArg(args, "report <run-dir>")
			if err != nil {
				return err
			}
			res, err := findings.BuildReport(dir)
			if err != nil {
				return exitFor(err)
			}
			if FromContext(cmd).JSON {
				return encodeJSON(cmd.OutOrStdout(), res)
			}
			renderReport(cmd.OutOrStdout(), res)
			return nil
		},
	}
}

func renderReport(out io.Writer, r *findings.Report) {
	_, _ = fmt.Fprintf(out, "wrote=%s\n", r.Wrote)
	names := strings.Join(r.VerdictFiles, ",")
	if names == "" {
		names = "none"
	}
	_, _ = fmt.Fprintf(out, "verdict_files=%d %s\n", len(r.VerdictFiles), names)
	buckets := make([]string, len(r.Verdicts))
	for i, v := range r.Verdicts {
		buckets[i] = fmt.Sprintf("%s:%d", v.Verdict, v.N)
	}
	_, _ = fmt.Fprintf(out, "verdicts=%s\n", strings.Join(buckets, " "))
	_, _ = fmt.Fprintf(out, "reportable=%d counts=%s\n", r.Reportable, renderCounts(r.Counts))
	_, _ = fmt.Fprintf(out, "gating=%d (critical+major confirmed)\n", r.Gating)
	if len(r.Unverified) > 0 {
		_, _ = fmt.Fprintf(out, "UNVERIFIED=%s\n", strings.Join(r.Unverified, ","))
	}
	for _, o := range truncate(r.Orphans, 10) {
		_, _ = fmt.Fprintf(out, "orphan %s\n", o)
	}
	for _, it := range r.Kept {
		extra := ""
		if len(it.Sources) > 1 {
			extra = fmt.Sprintf(" [%s]", strings.Join(it.Sources, "+"))
		}
		_, _ = fmt.Fprintf(out, "%s [%s, %s] conf %s %s %s:%s — %s%s\n",
			it.ID, it.Surface, it.Severity, conf(it.Confidence), it.Verdict, it.File, lineStr(it.Line), it.Title, extra)
	}
	for _, it := range r.Decided {
		reason := ""
		if it.Reason != "" {
			reason = " — " + it.Reason
		}
		_, _ = fmt.Fprintf(out, "%s %s %s:%s — %s%s\n", it.ID, it.Verdict, it.File, lineStr(it.Line), it.Title, reason)
	}
	for _, it := range r.Aside {
		_, _ = fmt.Fprintf(out, "%s %s %s:%s — %s\n", it.ID, it.Class, it.File, lineStr(it.Line), it.Title)
	}
}

// ---------------------------------------------------------------- rendering

func truncate(ss []string, n int) []string {
	if len(ss) > n {
		return ss[:n]
	}
	return ss
}

func truncatePairs(ps []findings.ReviewPair, n int) []findings.ReviewPair {
	if len(ps) > n {
		return ps[:n]
	}
	return ps
}

func renderCounts(cs []findings.Count) string {
	if len(cs) == 0 {
		return "none"
	}
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = fmt.Sprintf("%d [%s, %s]", c.N, c.Surface, c.Severity)
	}
	return strings.Join(parts, ", ")
}

func lineStr(l *int) string {
	if l == nil {
		return "?"
	}
	return fmt.Sprint(*l)
}

func joinLines(ls []*int) string {
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = lineStr(l)
	}
	return strings.Join(parts, "~")
}

func pairID(id *string) string {
	if id == nil {
		return "dropped"
	}
	return *id
}

func conf(c *float64) string {
	if c == nil {
		return "?"
	}
	return num(*c)
}

func num(f float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%v", f), ".0")
}
