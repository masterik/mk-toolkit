package repoinit

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/masterik/mk-toolkit/internal/core/initplan"
	"github.com/masterik/mk-toolkit/internal/core/profile"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

func testPlan() *initplan.Plan {
	return initplan.Build(initplan.Input{
		Discovered: &profile.Profile{
			Gate:   profile.Gate{Steps: []profile.GateStep{{Step: "test", Command: "go test ./..."}}},
			Scopes: profile.List{Values: []string{"cli"}, Source: profile.Discovered},
			Review: profile.List{Values: []string{"@a"}, Source: profile.Discovered},
		},
		Candidates: initplan.Candidates{
			Remotes:   []initplan.Remote{{Name: "origin", URL: "git@github.com:o/r.git"}},
			Dirs:      []string{"core"},
			Branches:  []string{"main", "release"},
			Protected: []string{"main"},
		},
	})
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		m.Update(key(k))
	}
}

// walkTo presses Enter until the prompt with id is on screen.
func walkTo(t *testing.T, m *Model, id string) {
	t.Helper()
	for range 50 {
		if m.Current() == id {
			return
		}
		press(m, "enter")
	}
	t.Fatalf("never reached %s; at %s", id, m.Current())
}

// The smoke test: headless, Enter through every prompt and Write, and what comes
// back is the plan's pre-selection. Deliberately thin — which options exist and
// what they pin is initplan's, and tested there.
func TestEnterThroughEveryPromptSubmitsThePreselection(t *testing.T) {
	plan := testPlan()
	want := plan.Preselected()

	tm := teatest.NewTestModel(t, New(plan), teatest.WithInitialTermSize(100, 40))
	for range 40 {
		tm.Send(key("enter"))
		time.Sleep(5 * time.Millisecond)
	}
	final := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*Model)
	if !final.Done() || !final.Write() {
		t.Fatalf("wizard did not finish by writing: done=%v write=%v", final.Done(), final.Write())
	}
	got := final.Answers()
	if !reflect.DeepEqual(got.Choice, want.Choice) {
		t.Errorf("answers %v\nwant    %v", got.Choice, want.Choice)
	}
	if len(got.Custom) != 0 {
		t.Errorf("custom text %v typed by pressing Enter", got.Custom)
	}
	cfg, err := initplan.Apply(plan, got)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Merge.Style != "merge" || cfg.Review.Mode != "full" {
		t.Errorf("config %+v, want the two form defaults", cfg)
	}
}

func TestEscAbortsOnlyOnTheFirstPrompt(t *testing.T) {
	m := New(testPlan())
	first := m.Current()
	press(m, "enter", "esc")
	if m.Done() || m.Current() != first {
		t.Fatalf("esc on the second prompt: done=%v at %s, want back at %s", m.Done(), m.Current(), first)
	}
	press(m, "esc")
	if !m.Done() || m.Write() {
		t.Fatalf("esc on the first prompt: done=%v write=%v, want an abort", m.Done(), m.Write())
	}
}

func TestCtrlCAbortsAnywhere(t *testing.T) {
	m := New(testPlan())
	walkTo(t, m, initplan.KeyMerge)
	press(m, "ctrl+c")
	if !m.Done() || m.Write() {
		t.Fatalf("done=%v write=%v, want an abort", m.Done(), m.Write())
	}
}

func TestBackFromReviewReturnsToTheLastPrompt(t *testing.T) {
	m := New(testPlan())
	walkTo(t, m, "review")
	press(m, "down", "enter")
	if m.Current() != initplan.KeyKeep {
		t.Fatalf("back from review landed on %s, want %s", m.Current(), initplan.KeyKeep)
	}
	press(m, "enter")
	if m.Current() != "review" {
		t.Fatalf("at %s, want review again", m.Current())
	}
	press(m, "down", "down", "enter")
	if !m.Done() || m.Write() {
		t.Fatalf("abort on review: done=%v write=%v", m.Done(), m.Write())
	}
}

func TestCustomTextIsValidatedBeforeItAdvances(t *testing.T) {
	m := New(testPlan())
	walkTo(t, m, initplan.KeySubjectMax)
	// 50, 72, 100, custom…, don't pin — the pre-selection is the last.
	press(m, "up", "enter")
	custom := initplan.KeySubjectMax + ".custom"
	if m.Current() != custom {
		t.Fatalf("at %s, want %s", m.Current(), custom)
	}
	press(m, "0", "enter")
	if m.Current() != custom || !strings.Contains(m.View(), markError) {
		t.Fatalf("0 accepted as a subject length; at %s", m.Current())
	}
	m.input.SetValue("")
	press(m, "6", "0", "enter")
	if m.Current() == custom {
		t.Fatalf("60 refused: %s", m.err)
	}
	if got := m.Answers().Custom[initplan.KeySubjectMax]; got != "60" {
		t.Errorf("custom = %q, want 60", got)
	}
	// Back to the prompt: it opens on what was typed.
	press(m, "esc")
	if m.Current() != custom || m.input.Value() != "60" {
		t.Errorf("back: at %s holding %q", m.Current(), m.input.Value())
	}
}

func TestALockedBranchIsShownButNeverToggled(t *testing.T) {
	m := New(testPlan())
	walkTo(t, m, initplan.KeyKeep)
	if !strings.Contains(m.View(), markTicked+" main  (always kept)") {
		t.Fatalf("locked main not shown:\n%s", m.View())
	}
	q := m.plan.Question(initplan.KeyKeep)
	for range q.Options {
		press(m, "space", "down")
	}
	if got := m.Answers().Choice[initplan.KeyKeep]; len(got) != len(q.Options) {
		t.Fatalf("toggled %v, want every offered option", got)
	}
	for _, v := range m.Answers().Choice[initplan.KeyKeep] {
		if v == "main" {
			t.Errorf("the locked branch was reachable by the cursor")
		}
	}
}

// huh scrolled a select to its value and never back; the window here starts at
// the top and moves only when the cursor would leave it.
func TestAPreselectionNearTheEndKeepsTheOptionsAboveInView(t *testing.T) {
	plan := initplan.Build(initplan.Input{Existing: &repoconfig.Config{Merge: repoconfig.Merge{Style: "rebase"}}})
	m := New(plan)
	walkTo(t, m, initplan.KeyMerge)
	v := m.View()
	for _, want := range []string{"merge", "squash", "rebase", "don't pin"} {
		if !strings.Contains(v, want) {
			t.Errorf("%q not in view:\n%s", want, v)
		}
	}
}

func TestLongListsScrollAroundTheCursor(t *testing.T) {
	var branches []string
	for i := range 30 {
		branches = append(branches, string(rune('a'+i%26))+strings.Repeat("x", i/26))
	}
	m := New(initplan.Build(initplan.Input{Candidates: initplan.Candidates{Branches: branches}}))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	walkTo(t, m, initplan.KeyKeep)
	if !strings.Contains(m.View(), "↓ ") || strings.Contains(m.View(), "↑ ") {
		t.Fatalf("at the top, want only a below marker:\n%s", m.View())
	}
	for range 29 {
		press(m, "down")
	}
	v := m.View()
	if !strings.Contains(v, "↑ ") || !strings.Contains(v, markCursor+" ") {
		t.Fatalf("at the bottom, want an above marker and the cursor:\n%s", v)
	}
}

func TestEveryLineFitsANarrowTerminal(t *testing.T) {
	m := New(testPlan())
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 40})
	for range 20 {
		for _, l := range strings.Split(m.View(), "\n") {
			if w := lipgloss.Width(l); w > 40 {
				t.Fatalf("at %s, a line %d wide: %q", m.Current(), w, l)
			}
		}
		if m.Current() == "review" {
			return
		}
		press(m, "enter")
	}
}
