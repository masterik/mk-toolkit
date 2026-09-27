package initplan

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/profile"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

func intp(n int) *int { return &n }

// selected returns a question's pre-selection and the provenance of the option
// it names, which is what a user sees as "why this one".
func selected(t *testing.T, p *Plan, key string) (string, Provenance) {
	t.Helper()
	q := p.Question(key)
	if q == nil {
		t.Fatalf("no question %q", key)
	}
	if len(q.Selected) != 1 {
		t.Fatalf("%s: selected %v, want exactly one", key, q.Selected)
	}
	for _, o := range q.Options {
		if o.Value == q.Selected[0] {
			return o.Value, o.Provenance
		}
	}
	t.Fatalf("%s: selected %q is not an option", key, q.Selected[0])
	return "", ""
}

func option(t *testing.T, p *Plan, key, value string) Option {
	t.Helper()
	for _, o := range p.Question(key).Options {
		if o.Value == value {
			return o
		}
	}
	t.Fatalf("%s: no option %q", key, value)
	return Option{}
}

func TestPreselectionIsPinnedThenDiscoveredThenDefault(t *testing.T) {
	disc := &profile.Profile{Merge: profile.Value{Value: "rebase", Source: profile.Discovered}}
	for _, tc := range []struct {
		name     string
		existing *repoconfig.Config
		disc     *profile.Profile
		key      string
		want     string
		prov     Provenance
	}{
		{"merge default", nil, nil, KeyMerge, "merge", Default},
		{"merge discovered", nil, disc, KeyMerge, "rebase", Discovered},
		{"merge pinned beats discovered", &repoconfig.Config{Merge: repoconfig.Merge{Style: "squash"}}, disc,
			KeyMerge, "squash", Pinned},
		{"review mode default", nil, nil, KeyReviewMode, "full", Default},
		{"review mode pinned", &repoconfig.Config{Review: repoconfig.Review{Mode: "quick"}}, nil,
			KeyReviewMode, "quick", Pinned},
		{"spec store has no default", nil, nil, KeySpecStore, DontPin, ""},
		{"spec store discovered", nil,
			&profile.Profile{Spec: profile.Spec{Store: profile.Value{Value: "files", Source: profile.Discovered}}},
			KeySpecStore, "files", Discovered},
		{"subject max has no default", nil, nil, KeySubjectMax, DontPin, ""},
		{"subject max pinned preset", &repoconfig.Config{Commit: repoconfig.Commit{SubjectMax: intp(72)}}, nil,
			KeySubjectMax, "72", Pinned},
		{"subject max pinned off-preset", &repoconfig.Config{Commit: repoconfig.Commit{SubjectMax: intp(64)}}, nil,
			KeySubjectMax, "64", Pinned},
		{"spec ref has no default", nil, nil, KeySpecRef, DontPin, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Build(Input{Existing: tc.existing, Discovered: tc.disc})
			got, prov := selected(t, p, tc.key)
			if got != tc.want || prov != tc.prov {
				t.Errorf("selected %q (%s), want %q (%s)", got, prov, tc.want, tc.prov)
			}
		})
	}
}

func TestEverySelectOffersDontPin(t *testing.T) {
	p := Build(Input{})
	for _, pg := range p.Pages {
		for _, q := range pg.Questions {
			if q.Kind != Select || q.Key == KeyScopesMode {
				continue
			}
			if !slices.ContainsFunc(q.Options, func(o Option) bool { return o.Value == DontPin }) {
				t.Errorf("%s offers no \"don't pin\"", q.Key)
			}
		}
	}
}

func TestEveryOptionIsDescribed(t *testing.T) {
	p := Build(Input{
		Discovered: &profile.Profile{Gate: profile.Gate{Steps: []profile.GateStep{{Step: "test", Command: "go test ./..."}}}},
		Candidates: Candidates{Remotes: []Remote{{Name: "origin", URL: "git@github.com:o/r.git"}}},
	})
	for _, pg := range p.Pages {
		for _, q := range pg.Questions {
			if q.Kind != Select {
				continue
			}
			for _, o := range q.Options {
				if o.Description == "" {
					t.Errorf("%s: option %q has no description", q.Key, o.Label)
				}
			}
		}
	}
}

// The walk is the four choices a repo has to make; the gate and the keep list
// are found or defaulted, and optional — opened from the review page.
func TestPagesAreTheStepsInOrder(t *testing.T) {
	var walked, optional []string
	for _, pg := range Build(Input{}).Pages {
		if pg.Intro == "" {
			t.Errorf("page %s has no intro", pg.Title)
		}
		if pg.Optional {
			optional = append(optional, pg.Title)
		} else {
			walked = append(walked, pg.Title)
		}
	}
	if want := []string{"Spec", "Commit", "Review", "Merge"}; !reflect.DeepEqual(walked, want) {
		t.Errorf("walked pages %v, want %v", walked, want)
	}
	if want := []string{"Gate", "Cleanup"}; !reflect.DeepEqual(optional, want) {
		t.Errorf("optional pages %v, want %v", optional, want)
	}
}

// A rejected pinned value is not pinned: Load cleared it, the field falls
// through, and the question says why.
func TestRejectedPinFallsThroughWithANote(t *testing.T) {
	existing := &repoconfig.Config{Problems: []repoconfig.Problem{{Key: "merge.style", Detail: "`merge.style` is \"sqaush\""}}}
	p := Build(Input{Existing: existing})
	if got, prov := selected(t, p, KeyMerge); got != "merge" || prov != Default {
		t.Errorf("selected %q (%s), want merge (default)", got, prov)
	}
	if n := p.Question(KeyMerge).Note; !strings.Contains(n, "sqaush") {
		t.Errorf("note %q does not name the rejected value", n)
	}
}

// Accepting everything pins what was discovered — the gate's commands and the
// scopes history uses — plus the two form defaults. Reviewers found in
// CODEOWNERS are the exception: pinning them would replace pr's per-path match.
func TestAcceptingEveryPreselectionPinsWhatWasDiscovered(t *testing.T) {
	p := Build(Input{Discovered: &profile.Profile{
		Gate:   profile.Gate{Steps: []profile.GateStep{{Step: "test", Command: "go test ./..."}}},
		Scopes: profile.List{Values: []string{"cli"}, Source: profile.Discovered},
		Review: profile.List{Values: []string{"@a"}, Source: profile.Discovered},
	}})
	cfg, err := Apply(p, p.Preselected())
	if err != nil {
		t.Fatal(err)
	}
	want := &repoconfig.Config{
		Gate:   repoconfig.Gate{Commands: map[string]string{"test": "go test ./..."}},
		Commit: repoconfig.Commit{Scopes: []string{"cli"}},
		Review: repoconfig.Review{Mode: "full"},
		Merge:  repoconfig.Merge{Style: "merge"},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("config %+v, want %+v", cfg, want)
	}
}

func TestAllDontPinIsAZeroConfig(t *testing.T) {
	p := Build(Input{})
	a := p.Preselected()
	a.Choice[KeyMerge] = []string{DontPin}
	a.Choice[KeyReviewMode] = []string{DontPin}
	a.Choice[KeyScopesMode] = []string{ScopesDiscover}
	cfg, err := Apply(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsZero() {
		t.Errorf("config %+v, want zero", cfg)
	}
}

func TestScopesLeadIn(t *testing.T) {
	withHistory := &profile.Profile{Scopes: profile.List{Values: []string{"cli"}, Source: profile.Discovered}}
	if got, prov := selected(t, Build(Input{Discovered: withHistory}), KeyScopesMode); got != ScopesPin || prov != Discovered {
		t.Errorf("with history: %q (%s), want pin a list (discovered)", got, prov)
	}
	if got, _ := selected(t, Build(Input{}), KeyScopesMode); got != ScopesPin {
		t.Errorf("without history: %q, want pin a list", got)
	}
	pinned := &repoconfig.Config{Commit: repoconfig.Commit{Scopes: []string{"api"}}}
	if got, prov := selected(t, Build(Input{Existing: pinned, Discovered: withHistory}), KeyScopesMode); got != ScopesPin || prov != Pinned {
		t.Errorf("pinned: %q (%s), want pin a list (pinned)", got, prov)
	}
}

func TestScopeListTicksHistoryOffersDirectoriesAndSkipsTypes(t *testing.T) {
	p := Build(Input{
		Discovered: &profile.Profile{Scopes: profile.List{Values: []string{"cli", "tui"}, Source: profile.Discovered}},
		Candidates: Candidates{Dirs: []string{"cli", "core", "docs", "test", "plugin"}},
	})
	q := p.Question(KeyScopes)
	if q.ShowIf == nil || q.ShowIf.Key != KeyScopesMode || q.ShowIf.Value != ScopesPin {
		t.Errorf("scope list is not conditional on pin a list: %+v", q.ShowIf)
	}
	var values []string
	for _, o := range q.Options {
		values = append(values, o.Value)
	}
	want := []string{"cli", "tui", "core", "plugin", CustomValue}
	if !reflect.DeepEqual(values, want) {
		t.Errorf("options %q, want %q (deduplicated, types dropped, + add last)", values, want)
	}
	if !reflect.DeepEqual(q.Selected, []string{"cli", "tui"}) {
		t.Errorf("ticked %v, want the history scopes", q.Selected)
	}
	if o := option(t, p, KeyScopes, "core"); o.Provenance != Suggested {
		t.Errorf("directory candidate tagged %q, want suggested", o.Provenance)
	}
}

func TestScopeListIsIgnoredWhileDiscovering(t *testing.T) {
	p := Build(Input{Discovered: &profile.Profile{Scopes: profile.List{Values: []string{"cli"}, Source: profile.Discovered}}})
	a := p.Preselected() // list ticked underneath
	a.Choice[KeyScopesMode] = []string{ScopesDiscover}
	cfg, err := Apply(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Commit.Scopes) != 0 {
		t.Errorf("scopes %v pinned while the lead-in says keep discovering", cfg.Commit.Scopes)
	}
}

func TestScopesAddAppendsTypedNames(t *testing.T) {
	p := Build(Input{Candidates: Candidates{Dirs: []string{"core"}}})
	a := p.Preselected()
	a.Choice[KeyScopes] = []string{"core", CustomValue}
	a.Custom[KeyScopes] = " api, web ,core"
	cfg, err := Apply(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"core", "api", "web"}; !reflect.DeepEqual(cfg.Commit.Scopes, want) {
		t.Errorf("scopes %v, want %v", cfg.Commit.Scopes, want)
	}
	a.Custom[KeyScopes] = " , "
	if _, err := Apply(p, a); err == nil {
		t.Error("an empty + add was accepted")
	}
}

func TestSubjectMaxCustomIsValidated(t *testing.T) {
	p := Build(Input{})
	a := p.Preselected()
	a.Choice[KeySubjectMax] = []string{CustomValue}
	for _, bad := range []string{"0", "-3", "abc", ""} {
		a.Custom[KeySubjectMax] = bad
		if _, err := Apply(p, a); err == nil {
			t.Errorf("custom subject max %q accepted", bad)
		}
		if Validate(KeySubjectMax, bad) == nil {
			t.Errorf("Validate accepted %q", bad)
		}
	}
	a.Custom[KeySubjectMax] = "64"
	cfg, err := Apply(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commit.SubjectMax == nil || *cfg.Commit.SubjectMax != 64 {
		t.Errorf("subject max %v, want 64", cfg.Commit.SubjectMax)
	}
	var presets []string
	for _, o := range p.Question(KeySubjectMax).Options {
		presets = append(presets, o.Value)
	}
	if want := []string{"50", "72", "100", CustomValue, DontPin}; !reflect.DeepEqual(presets, want) {
		t.Errorf("options %q, want %q", presets, want)
	}
}

func TestKeepLocksTheProtectedBranches(t *testing.T) {
	in := Input{Candidates: Candidates{Branches: []string{"main", "release", "wip"}, Protected: []string{"main"}}}
	p := Build(in)
	q := p.Question(KeyKeep)
	if !reflect.DeepEqual(q.Locked, []string{"main"}) {
		t.Errorf("locked %v, want [main]", q.Locked)
	}
	if slices.ContainsFunc(q.Options, func(o Option) bool { return o.Value == "main" }) {
		t.Error("the default branch is offered as a toggle")
	}
	if !strings.Contains(q.Description, "main") {
		t.Errorf("description %q does not say main is kept", q.Description)
	}
	a := p.Preselected()
	a.Choice[KeyKeep] = []string{"release"}
	cfg, err := Apply(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Cleanup.Keep, []string{"release"}) {
		t.Errorf("keep %v, want [release] — the default branch is not written", cfg.Cleanup.Keep)
	}

	// Already pinned: kept on a rewrite, so the line a human wrote survives.
	in.Existing = &repoconfig.Config{Cleanup: repoconfig.Cleanup{Keep: []string{"main", "release"}}}
	p = Build(in)
	cfg, err = Apply(p, p.Preselected())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Cleanup.Keep, []string{"main", "release"}) {
		t.Errorf("keep %v, want [main release]", cfg.Cleanup.Keep)
	}
}

func TestGateKeepAndOverride(t *testing.T) {
	p := Build(Input{Discovered: &profile.Profile{Gate: profile.Gate{Steps: []profile.GateStep{
		{Step: "test", Command: "go test ./..."}, {Step: "lint", Command: "golangci-lint run"},
	}}}})
	q := p.Question(GateStepKey("test"))
	if got, prov := selected(t, p, GateStepKey("test")); got != "go test ./..." || prov != Discovered {
		t.Errorf("gate step pre-selects %q (%s), want the discovered command pinned", got, prov)
	}
	for _, o := range q.Options {
		if strings.Contains(strings.ToLower(o.Label), "drop") {
			t.Errorf("gate offers %q, which the schema cannot express", o.Label)
		}
	}
	a := p.Preselected()
	a.Choice[GateStepKey("lint")] = []string{CustomValue}
	a.Custom[GateStepKey("lint")] = "just lint"
	a.Choice[KeyGateNew] = []string{CustomValue}
	a.Custom[KeyGateNew] = "e2e = npm run e2e"
	cfg, err := Apply(p, a)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"test": "go test ./...", "lint": "just lint", "e2e": "npm run e2e"}
	if !reflect.DeepEqual(cfg.Gate.Commands, want) {
		t.Errorf("gate %v, want %v", cfg.Gate.Commands, want)
	}
	a.Custom[KeyGateNew] = "no-equals"
	if _, err := Apply(p, a); err == nil {
		t.Error("a new step without step=command was accepted")
	}
}

// --force feeds the existing config in as the pinned tier: accepting every
// pre-selection rewrites the same values.
func TestForcePrefillRoundTrips(t *testing.T) {
	existing := &repoconfig.Config{
		Gate:    repoconfig.Gate{Commands: map[string]string{"test": "just test", "e2e": "npm run e2e"}},
		Spec:    repoconfig.Spec{Store: "github-issues", Ref: "o/r"},
		Commit:  repoconfig.Commit{Scopes: []string{"api", "web"}, SubjectMax: intp(64)},
		Review:  repoconfig.Review{Reviewers: []string{"@a"}, Mode: "quick"},
		Merge:   repoconfig.Merge{Style: "squash"},
		Cleanup: repoconfig.Cleanup{Keep: []string{"staging"}},
	}
	p := Build(Input{
		Existing:   existing,
		Discovered: &profile.Profile{Gate: profile.Gate{Steps: []profile.GateStep{{Step: "test", Command: "go test ./..."}}}},
		Candidates: Candidates{Remotes: []Remote{{Name: "origin", URL: "https://github.com/o/r.git"}}},
	})
	cfg, err := Apply(p, p.Preselected())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, existing) {
		t.Errorf("rewrite %+v\nwant     %+v", cfg, existing)
	}
}

func TestSpecRefOffersRemotes(t *testing.T) {
	p := Build(Input{
		Discovered: &profile.Profile{Spec: profile.Spec{
			Store: profile.Value{Value: "github-issues", Source: profile.Discovered},
			Ref:   profile.Value{Value: "o/r", Source: profile.Discovered}}},
		Candidates: Candidates{Remotes: []Remote{
			{Name: "origin", URL: "git@github.com:o/r.git"},
			{Name: "fork", URL: "https://gitlab.com/g/sub/r"},
			{Name: "dup", URL: "https://github.com/o/r"},
		}},
	})
	var values []string
	for _, o := range p.Question(KeySpecRef).Options {
		values = append(values, o.Value)
	}
	if want := []string{"o/r", "g/sub/r", CustomValue, DontPin}; !reflect.DeepEqual(values, want) {
		t.Errorf("options %q, want %q", values, want)
	}
	if got, prov := selected(t, p, KeySpecRef); got != "o/r" || prov != Discovered {
		t.Errorf("selected %q (%s), want o/r (discovered)", got, prov)
	}
}

func TestRemoteSlug(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:o/r.git":         "o/r",
		"https://github.com/o/r.git":     "o/r",
		"https://github.com/o/r/":        "o/r",
		"ssh://git@gitlab.com/g/s/r.git": "g/s/r",
		"/local/path/repo":               "",
		"https://example.com":            "",
	} {
		if got := RemoteSlug(url); got != want {
			t.Errorf("RemoteSlug(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestReviewersDefaultToDontPin(t *testing.T) {
	p := Build(Input{Discovered: &profile.Profile{Review: profile.List{Values: []string{"@a", "@b"}, Source: profile.Discovered}}})
	q := p.Question(KeyReviewers)
	if len(q.Selected) != 0 {
		t.Errorf("ticked %v, want none — CODEOWNERS keeps being read", q.Selected)
	}
	if o := option(t, p, KeyReviewers, "@a"); o.Provenance != Discovered {
		t.Errorf("CODEOWNERS owner tagged %q", o.Provenance)
	}
}

func TestCustomSelectValuesAreValidated(t *testing.T) {
	p := Build(Input{})
	a := p.Preselected()
	a.Choice[KeySpecRef] = []string{CustomValue}
	a.Custom[KeySpecRef] = "  "
	if _, err := Apply(p, a); err == nil {
		t.Error("a blank custom spec ref was accepted")
	}
	a.Custom[KeySpecRef] = "docs/specs"
	cfg, err := Apply(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec.Ref != "docs/specs" {
		t.Errorf("ref %q", cfg.Spec.Ref)
	}
}

// Re-init shows a changed discovery beside the pin, and keeps the pin unless the
// user switches — the tool suggests, the user re-discovers.
func TestGateReinitSuggestsWhatDiscoveryNowFinds(t *testing.T) {
	p := Build(Input{
		Existing:   &repoconfig.Config{Gate: repoconfig.Gate{Commands: map[string]string{"test": "go test ./..."}}},
		Discovered: &profile.Profile{Gate: profile.Gate{Steps: []profile.GateStep{{Step: "test", Command: "just test"}}}},
	})
	if got, prov := selected(t, p, GateStepKey("test")); got != "go test ./..." || prov != Pinned {
		t.Errorf("pre-selects %q (%s), want the pin kept", got, prov)
	}
	if o := option(t, p, GateStepKey("test"), "just test"); o.Provenance != Discovered || !strings.Contains(o.Label, "switch") {
		t.Errorf("suggestion %+v, want a discovered switch option", o)
	}
	if d := p.Question(GateStepKey("test")).Description; !strings.Contains(d, "just test") {
		t.Errorf("description %q does not name the suggestion", d)
	}
}

// A pinned value discovery also finds is pre-selected because it is pinned, and
// says so; an off-preset value pick adds is still described.
func TestPickTagsThePinAndDescribesWhatItAdds(t *testing.T) {
	p := Build(Input{
		Existing: &repoconfig.Config{
			Merge:  repoconfig.Merge{Style: "rebase"},
			Commit: repoconfig.Commit{SubjectMax: intp(64)},
			Spec:   repoconfig.Spec{Ref: "o/r"},
		},
		Discovered: &profile.Profile{
			Merge: profile.Value{Value: "rebase", Source: profile.Discovered},
			Spec:  profile.Spec{Ref: profile.Value{Value: "o/r", Source: profile.Discovered}},
		},
		Candidates: Candidates{Remotes: []Remote{{Name: "origin", URL: "git@github.com:o/r.git"}}},
	})
	for _, key := range []string{KeyMerge, KeySpecRef} {
		if _, prov := selected(t, p, key); prov != Pinned {
			t.Errorf("%s: pre-selection tagged %q, want pinned", key, prov)
		}
	}
	if o := option(t, p, KeySubjectMax, "64"); o.Description == "" || o.Provenance != Pinned {
		t.Errorf("added option %+v, want a described pin", o)
	}
}
