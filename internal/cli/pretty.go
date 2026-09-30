package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/masterik/mk-toolkit/internal/buildinfo"
	"github.com/masterik/mk-toolkit/internal/core/branchstatus"
	"github.com/masterik/mk-toolkit/internal/core/doctor"
	"github.com/masterik/mk-toolkit/internal/core/profile"
	"github.com/masterik/mk-toolkit/internal/tui/ui"
)

// The pretty renderers below are the human forms of the commands that have one.
// They are reached only when Options.Pretty is set — a terminal, no --json, no
// --no-tui — so the plain renderers beside them stay the machine contract.

// wrapIndent wraps text to width and indents every line after the first.
func wrapIndent(text string, width, indent int) string {
	if width < 20 {
		width = 20
	}
	lines := strings.Split(lipgloss.NewStyle().Width(width).Render(text), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
		if i > 0 {
			lines[i] = strings.Repeat(" ", indent) + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func prettyVersion(out io.Writer) {
	s := ui.New(out)
	_, _ = fmt.Fprintf(out, "%s %s\n", s.Title("mkit", buildinfo.Version),
		s.Dim(fmt.Sprintf("%s · %s", buildinfo.Commit, buildinfo.Date)))
}

func prettyDoctor(out io.Writer, r *doctor.Report) {
	s := ui.New(out)
	_, _ = fmt.Fprintln(out, s.Title("mkit doctor", buildinfo.Version))

	nameW := 0
	for _, c := range r.Checks {
		if w := lipgloss.Width(c.Name); w > nameW {
			nameW = w
		}
	}
	if nameW > 24 {
		nameW = 24
	}
	indent := 5 + nameW + 2

	var order []string
	byGroup := map[string][]doctor.Check{}
	for _, c := range r.Checks {
		if _, seen := byGroup[c.Group]; !seen {
			order = append(order, c.Group)
		}
		byGroup[c.Group] = append(byGroup[c.Group], c)
	}
	for _, g := range order {
		_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(g))
		for _, c := range byGroup[g] {
			name := c.Name
			pad := nameW - lipgloss.Width(name)
			if pad < 0 {
				pad = 0
			}
			detail := wrapIndent(c.Detail, s.Width()-indent, indent)
			_, _ = fmt.Fprintf(out, "  %s  %s%s  %s\n", s.Icon(string(c.Status)), name, strings.Repeat(" ", pad), detail)
			if c.Remedy != "" {
				rem := wrapIndent(c.Remedy, s.Width()-indent-2, indent+2)
				_, _ = fmt.Fprintf(out, "%s%s %s\n", strings.Repeat(" ", indent), s.Accent("→"), s.Dim(rem))
			}
		}
	}

	n := r.Counts()
	part := func(count int, icon, word string) string {
		text := fmt.Sprintf("%d %s", count, word)
		if count == 0 {
			return s.Dim(text)
		}
		return icon + " " + text
	}
	_, _ = fmt.Fprintf(out, "\n%s  %s  %s  %s\n",
		part(n[doctor.OK], s.Icon("ok"), "ok"),
		part(n[doctor.Warn], s.Icon("warn"), "warn"),
		part(n[doctor.Fail], s.Icon("fail"), "fail"),
		part(n[doctor.Unknown], s.Icon("?"), "unknown"))
}

func prettyProfile(out io.Writer, p *profile.Profile) {
	s := ui.New(out)
	_, _ = fmt.Fprintln(out, s.Title("mkit repo profile", p.Toplevel))
	const lw = 20

	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("config"))
	state := p.Config.State
	st := s.Dim(string(state))
	switch state {
	case "tracked":
		st = s.Good(string(state))
	case "shadowed":
		st = s.Bad(string(state))
	}
	_, _ = fmt.Fprintln(out, s.KV("file", p.Config.Path+"  "+st, lw))
	if p.Config.IgnoreSource != "" {
		_, _ = fmt.Fprintln(out, s.KV("shadowed by", s.Bad(p.Config.IgnoreSource), lw))
	}
	for _, pb := range p.ConfigProblems {
		_, _ = fmt.Fprintf(out, "  %s %s %s\n", s.Icon("warn"), wrapIndent(pb.Detail, s.Width()-6, 4), s.Dim(string(pb.Kind)))
	}

	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("quality gate"))
	if p.Gate.Ecosystem != "" {
		_, _ = fmt.Fprintln(out, s.KV("ecosystem", p.Gate.Ecosystem, lw))
	}
	if len(p.Gate.Steps) == 0 {
		_, _ = fmt.Fprintf(out, "  %s %s\n", s.Icon("warn"), s.Warn(cause(p.Gate.Cause, "no gate commands found or pinned")))
	} else {
		if p.Gate.Cause != "" {
			_, _ = fmt.Fprintf(out, "  %s discovery: %s\n", s.Icon("warn"), s.Warn(p.Gate.Cause))
		}
		rows := make([][]string, 0, len(p.Gate.Steps))
		for _, step := range p.Gate.Steps {
			rows = append(rows, []string{step.Step, step.Command, string(step.Source)})
		}
		_, _ = fmt.Fprintln(out, indentBlock(s.Table([]string{"STEP", "COMMAND", "SOURCE"}, rows,
			func(_, col int, text string) lipgloss.Style {
				if col == 2 {
					return sourceStyle(s, text)
				}
				if col == 0 {
					return s.AccentStyle()
				}
				return s.Style()
			}), 2))
	}

	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("conventions"))
	profValue(out, s, "spec store", p.Spec.Store, lw)
	profValue(out, s, "spec ref", p.Spec.Ref, lw)
	profList(out, s, "commit scopes", p.Scopes, lw)
	profValue(out, s, "subject max", p.SubjectMax, lw)
	profList(out, s, "reviewers", p.Review, lw)
	profValue(out, s, "review mode", p.ReviewMode, lw)
	profValue(out, s, "merge style", p.Merge, lw)
	profList(out, s, "cleanup keep", p.Keep, lw)

	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("payload"))
	if p.Payload.Found {
		v := p.Payload.Version
		if v == "" {
			v = "unknown version"
		}
		_, _ = fmt.Fprintln(out, s.KV("plugin", fmt.Sprintf("%s  %s", s.Good(v), s.Dim(p.Payload.Dir+" ("+p.Payload.Via+")")), lw))
	} else {
		_, _ = fmt.Fprintf(out, "  %s %s\n", s.Icon("fail"), "not found")
		_, _ = fmt.Fprintf(out, "    %s %s\n", s.Accent("→"), s.Dim(p.Payload.Remedy))
	}
}

func sourceStyle(s *ui.S, source string) lipgloss.Style {
	switch source {
	case "pinned":
		return s.VioletStyle()
	case "unavailable":
		return s.WarnStyle()
	}
	return s.DimStyle()
}

func profValue(out io.Writer, s *ui.S, label string, v profile.Value, lw int) {
	if v.Source == profile.Unavailable {
		_, _ = fmt.Fprintln(out, s.KV(label, s.Dim(cause(v.Cause, "none"))+"  "+s.Tag("unavailable"), lw))
		return
	}
	_, _ = fmt.Fprintln(out, s.KV(label, v.Value+"  "+s.Tag(string(v.Source)), lw))
}

func profList(out io.Writer, s *ui.S, label string, l profile.List, lw int) {
	if l.Source == profile.Unavailable || len(l.Values) == 0 {
		_, _ = fmt.Fprintln(out, s.KV(label, s.Dim(cause(l.Cause, "none"))+"  "+s.Tag("unavailable"), lw))
		return
	}
	_, _ = fmt.Fprintln(out, s.KV(label, strings.Join(l.Values, ", ")+"  "+s.Tag(string(l.Source)), lw))
}

func indentBlock(block string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(block, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

func prettyBranchStatus(out io.Writer, sc *branchstatus.Scan) {
	s := ui.New(out)
	_, _ = fmt.Fprintln(out, s.Title("mkit branch status", ""))
	const lw = 10

	state := func(v, ok string) string {
		if v == ok {
			return s.Good(v)
		}
		return s.Warn(v)
	}
	_, _ = fmt.Fprintln(out, s.Panel(
		s.KV("default", s.Accent(sc.Default), lw),
		s.KV("develop", orNone(sc.Develop), lw),
		s.KV("remote", orNone(sc.Remote), lw),
		s.KV("fetch", state(sc.Fetch, "ok"), lw),
		s.KV("gh", state(sc.GH, "ok"), lw),
		s.KV("protected", s.Violet(strings.Join(sc.Protected, "  ")), lw),
		s.KV("keep", orNone(strings.Join(sc.Keep, "  ")), lw),
	))
	if len(sc.KeepUnknown) > 0 {
		_, _ = fmt.Fprintf(out, "%s %s\n", s.Icon("warn"),
			s.Dim("pinned to keep, no local branch: ")+strings.Join(sc.KeepUnknown, ", "))
	}

	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(fmt.Sprintf("branches (%d)", len(sc.Branches))))
	brows := make([][]string, 0, len(sc.Branches))
	for _, b := range sc.Branches {
		brows = append(brows, []string{b.Name, string(b.Class), b.Upstream,
			dashIfEmpty(strings.Join(b.MergedInto, ",")), dashIfEmpty(b.PR)})
	}
	_, _ = fmt.Fprintln(out, s.Table([]string{"BRANCH", "STATE", "UPSTREAM", "MERGED INTO", "PR"}, brows,
		func(_, col int, text string) lipgloss.Style {
			switch col {
			case 1:
				return classStyle(s, text)
			case 2, 3, 4:
				if text == "-" || text == "none" {
					return s.DimStyle()
				}
				if text == "gone" {
					return s.WarnStyle()
				}
			}
			return s.Style()
		}))

	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(fmt.Sprintf("worktrees (%d)", len(sc.Worktrees))))
	if sc.WorktreesState != "ok" && sc.WorktreesState != "" {
		_, _ = fmt.Fprintf(out, "%s worktree list: %s\n", s.Icon("warn"), s.Warn(sc.WorktreesState))
	}
	wrows := make([][]string, 0, len(sc.Worktrees))
	for _, w := range sc.Worktrees {
		wrows = append(wrows, []string{w.Branch, w.Origin, w.Clean, w.Path})
	}
	_, _ = fmt.Fprintln(out, s.Table([]string{"BRANCH", "ORIGIN", "CLEAN", "PATH"}, wrows,
		func(_, col int, text string) lipgloss.Style {
			switch col {
			case 2:
				switch text {
				case "yes":
					return s.GoodStyle()
				case "no":
					return s.WarnStyle()
				}
				return s.BadStyle()
			case 3:
				return s.DimStyle()
			}
			return s.Style()
		}))

	if len(sc.ConfigProblems) > 0 {
		_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("notes"))
		for _, n := range sc.ConfigProblems {
			_, _ = fmt.Fprintf(out, "  %s %s\n", s.Icon("warn"), wrapIndent(n, s.Width()-6, 4))
		}
	}
}

func classStyle(s *ui.S, class string) lipgloss.Style {
	switch branchstatus.Class(class) {
	case branchstatus.Protected:
		return s.VioletStyle().Bold(true)
	case branchstatus.Current:
		return s.AccentStyle().Bold(true)
	case branchstatus.Merged, branchstatus.MergedPR:
		return s.GoodStyle()
	case branchstatus.OpenPR:
		return s.AccentStyle()
	case branchstatus.ClosedPR, branchstatus.Gone, branchstatus.Unpushed:
		return s.WarnStyle()
	}
	return s.Style()
}

func prettyInit(out io.Writer, res initResult) {
	s := ui.New(out)
	if res.Written {
		_, _ = fmt.Fprintf(out, "%s %s %s\n", s.Icon("ok"), s.Bold("wrote"), res.Path)
	} else {
		icon := "warn"
		if res.State == "tracked" {
			icon = "ok"
		}
		_, _ = fmt.Fprintf(out, "%s %s  %s\n", s.Icon(icon), res.Path, s.Dim(res.State))
	}
	if res.Detail != "" {
		_, _ = fmt.Fprintf(out, "  %s\n", wrapIndent(res.Detail, s.Width()-4, 2))
	}
	if res.Remedy != "" {
		_, _ = fmt.Fprintf(out, "  %s %s\n", s.Accent("→"), s.Dim(wrapIndent(res.Remedy, s.Width()-6, 4)))
	}
}
