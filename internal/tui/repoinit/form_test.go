package repoinit

import (
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/masterik/mk-toolkit/internal/core/initplan"
	"github.com/masterik/mk-toolkit/internal/core/profile"
)

// The smoke test: headless, Enter through every page, and what comes back is
// the plan's pre-selection. Deliberately thin — which options exist and what
// they pin is initplan's, and tested there.
func TestEnterThroughEveryPageSubmitsThePreselection(t *testing.T) {
	plan := initplan.Build(initplan.Input{
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
	want := plan.Preselected()
	b := NewBindings(plan, want)
	form := NewForm(b)
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Quit

	tm := teatest.NewTestModel(t, form, teatest.WithInitialTermSize(100, 40))
	for range 40 {
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(10 * time.Millisecond)
	}
	final, ok := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*huh.Form)
	if !ok || final.State != huh.StateCompleted {
		t.Fatalf("form did not complete: %+v", final.State)
	}

	got := b.Answers()
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
