// Package initplan decides what the `mkit init` form asks, what it offers and
// what it pre-selects — and turns the answers back into a config.
//
// It is pure data both ways: Plan takes what the repo already says (a pinned
// config, a discovered profile, the candidates Gather collected) and returns
// pages of questions; Apply takes that plan and the answers and returns the
// config to write. The TUI only renders a Plan and hands back Answers, so every
// rule that decides what gets pinned lives here, where a table test reaches it.
//
// Pre-selection follows CONTEXT.md: **pinned value → discovered value → form
// default**. Form defaults are exactly `merge.style = merge` and
// `review.mode = full`; everything else defaults to "don't pin", so the config
// stays the pinned remainder ADR 0001 describes.
package initplan

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/profile"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// Provenance says why an option is on offer, and so why it may be pre-selected.
type Provenance string

const (
	// Pinned — the value the existing config holds.
	Pinned Provenance = "pinned"
	// Discovered — what mkit reads off the repo every run.
	Discovered Provenance = "discovered"
	// Default — the form's suggestion when nothing is pinned or discovered.
	Default Provenance = "default"
	// Suggested — a candidate from the repo's shape (a directory, a remote, a
	// branch) that nothing currently uses as this value.
	Suggested Provenance = "suggested"
)

// Kind is how a question is answered.
type Kind string

const (
	// Select answers with exactly one option.
	Select Kind = "select"
	// Multi answers with any number of options; none ticked pins nothing.
	Multi Kind = "multi"
)

// DontPin is the value of every "don't pin" option: the key stays unset in the
// written config and discovery keeps answering it.
const DontPin = ""

// CustomValue is the value of the option that asks for free text ("custom…",
// "+ add…"). The text itself arrives in Answers.Custom.
const CustomValue = "\x00custom"

// Scope lead-in values. Not config values — the lead-in only decides whether the
// scope list is asked at all.
const (
	ScopesDiscover = "discover"
	ScopesPin      = "pin"
)

// Keys of the questions whose answers Apply reads. Gate steps are
// GateStepKey(step).
const (
	KeySpecStore  = "spec.store"
	KeySpecRef    = "spec.ref"
	KeyScopesMode = "commit.scopes.mode"
	KeyScopes     = "commit.scopes"
	KeySubjectMax = "commit.subject_max"
	KeyReviewers  = "review.reviewers"
	KeyReviewMode = "review.mode"
	KeyMerge      = "merge.style"
	KeyKeep       = "cleanup.keep"
	KeyGateNew    = "gate.new"
	gateStepPre   = "gate.step."
)

// GateStepKey is the question key for one gate step.
func GateStepKey(step string) string { return gateStepPre + step }

// Option is one choice.
type Option struct {
	Value       string
	Label       string
	Description string
	Provenance  Provenance
}

// Custom reports whether choosing the option asks for free text.
func (o Option) Custom() bool { return o.Value == CustomValue }

// Condition shows a question only while another question's answer includes Value.
type Condition struct {
	Key   string
	Value string
}

// Question is one field of the form.
type Question struct {
	Key         string
	Title       string
	Description string
	Kind        Kind
	Options     []Option
	// Selected is the pre-selection: one value for a Select, any for a Multi.
	Selected []string
	// ShowIf, when set, hides the question unless the condition holds.
	ShowIf *Condition
	// CustomTitle and CustomPlaceholder label the free-text prompt a custom
	// option opens. For a Multi the text is comma-separated.
	CustomTitle       string
	CustomPlaceholder string
	// Locked lists values a Multi always includes and never lets the user
	// untick — shown, not offered.
	Locked []string
	// Note is a sentence about the pinned tier: a rejected value, for instance.
	Note string
}

// Page is one screen of the form, named for the step it affects.
type Page struct {
	Title     string
	Questions []Question
}

// Plan is the whole form.
type Plan struct {
	Pages []Page
}

// Question returns the question with key, or nil.
func (p *Plan) Question(key string) *Question {
	for i := range p.Pages {
		for j := range p.Pages[i].Questions {
			if p.Pages[i].Questions[j].Key == key {
				return &p.Pages[i].Questions[j]
			}
		}
	}
	return nil
}

// Answers is what the form returns: the chosen option values per question, and
// the text typed behind a custom option.
type Answers struct {
	Choice map[string][]string
	Custom map[string]string
}

// Preselected returns the answers a user gets by accepting every pre-selection —
// what the form starts from, and what pressing Enter throughout submits.
func (p *Plan) Preselected() Answers {
	a := Answers{Choice: map[string][]string{}, Custom: map[string]string{}}
	for _, pg := range p.Pages {
		for _, q := range pg.Questions {
			a.Choice[q.Key] = slices.Clone(q.Selected)
		}
	}
	return a
}

// Visible reports whether q is asked, given the answers so far.
func Visible(q Question, a Answers) bool {
	if q.ShowIf == nil {
		return true
	}
	return slices.Contains(a.Choice[q.ShowIf.Key], q.ShowIf.Value)
}

// WantsCustom reports whether q's free-text prompt is asked.
func WantsCustom(q Question, a Answers) bool {
	return Visible(q, a) && slices.Contains(a.Choice[q.Key], CustomValue)
}

// Input is everything Plan reads. The planner never touches the repo; Gather
// collects this.
type Input struct {
	// Existing is the pinned tier — the config on disk, zero when there is none.
	Existing *repoconfig.Config
	// Discovered is the profile with no config applied (profile.Discover), so
	// its values are never pinned ones.
	Discovered *profile.Profile
	Candidates Candidates
}

// Candidates are the repo facts the form offers as options.
type Candidates struct {
	Remotes []Remote
	// Dirs are directory names offered as scopes.
	Dirs []string
	// Branches are the local branches.
	Branches []string
	// Protected are the branches cleanup keeps regardless of any keep list.
	Protected []string
}

// Remote is one git remote.
type Remote struct {
	Name string
	URL  string
}

// Build plans the form.
func Build(in Input) *Plan {
	if in.Existing == nil {
		in.Existing = &repoconfig.Config{}
	}
	if in.Discovered == nil {
		in.Discovered = &profile.Profile{}
	}
	return &Plan{Pages: []Page{
		{Title: "Gate", Questions: gateQuestions(in)},
		{Title: "Spec", Questions: []Question{specStore(in), specRef(in)}},
		{Title: "Commit", Questions: append(scopes(in), subjectMax(in))},
		{Title: "Review", Questions: []Question{reviewMode(in), reviewers(in)}},
		{Title: "Merge", Questions: []Question{mergeStyle(in)}},
		{Title: "Cleanup", Questions: []Question{keep(in)}},
	}}
}

// discoverOpt is the "don't pin" option of a Select.
func discoverOpt(what string) Option {
	return Option{Value: DontPin, Label: "don't pin", Description: "discover each run — " + what}
}

// rejected is the note for a pinned value Load refused. It is not treated as
// pinned: the field falls through to discovered or default.
func rejected(cfg *repoconfig.Config, key string) string {
	if pb := cfg.Problem(key); pb != nil {
		return "the pinned value was rejected: " + pb.Detail
	}
	return ""
}

// pick applies pinned → discovered → form default to a Select, tagging the
// winner's option. Values not already among the options are added.
func pick(q *Question, pinned, discovered, def string) {
	tag := func(v string, p Provenance) {
		for i := range q.Options {
			if q.Options[i].Value == v {
				if q.Options[i].Provenance == "" {
					q.Options[i].Provenance = p
				}
				return
			}
		}
		// Before "don't pin" and "custom…", which always close the list.
		at := len(q.Options)
		for at > 0 && (q.Options[at-1].Value == DontPin || q.Options[at-1].Custom()) {
			at--
		}
		q.Options = slices.Insert(q.Options, at, Option{Value: v, Label: v, Provenance: p})
	}
	if discovered != "" {
		tag(discovered, Discovered)
	}
	if pinned != "" {
		tag(pinned, Pinned)
	}
	switch {
	case pinned != "":
		q.Selected = []string{pinned}
	case discovered != "":
		q.Selected = []string{discovered}
	case def != "":
		tag(def, Default)
		q.Selected = []string{def}
	default:
		q.Selected = []string{DontPin}
	}
}

func gateQuestions(in Input) []Question {
	pinned := in.Existing.Gate.Commands
	var qs []Question
	seen := map[string]bool{}
	for _, s := range in.Discovered.Gate.Steps {
		seen[s.Step] = true
		qs = append(qs, gateStep(s.Step, s.Command, pinned[s.Step]))
	}
	// A pinned step discovery does not know is still this config's to keep.
	for _, step := range sortedKeys(pinned) {
		if !seen[step] {
			qs = append(qs, gateStep(step, "", pinned[step]))
		}
	}
	qs = append(qs, Question{
		Key: KeyGateNew, Kind: Select, Title: "Add a step",
		Description: "Pin a step discovery does not find at all.",
		Options: []Option{
			{Value: DontPin, Label: "no extra step", Description: "the gate is the steps above"},
			{Value: CustomValue, Label: "add a step…", Description: "pin one more step as step=command"},
		},
		Selected:          []string{DontPin},
		CustomTitle:       "New gate step",
		CustomPlaceholder: "step=command, e.g. e2e=npm run e2e",
	})
	return qs
}

// gateStep offers keep / override for one step. There is no "drop": the schema
// pins commands by step and has no way to say a step must not run, so the form
// does not invent one.
func gateStep(step, discovered, pinned string) Question {
	q := Question{
		Key: GateStepKey(step), Kind: Select, Title: "Gate step: " + step,
		CustomTitle: "Command for " + step, CustomPlaceholder: "the command line to run",
	}
	if discovered != "" {
		q.Description = "Discovered: " + discovered
		q.Options = append(q.Options, Option{Value: DontPin, Label: "keep discovered",
			Description: "run `" + discovered + "`, re-discovered every run", Provenance: Discovered})
	} else {
		q.Description = "Not discovered — only the config names this step."
		q.Options = append(q.Options, Option{Value: DontPin, Label: "drop the pin",
			Description: "stop pinning this step; the gate is what discovery finds"})
	}
	if pinned != "" && pinned != discovered {
		q.Options = append(q.Options, Option{Value: pinned, Label: "keep pinned: " + pinned,
			Description: "run the pinned command instead of the discovered one", Provenance: Pinned})
	}
	q.Options = append(q.Options, Option{Value: CustomValue, Label: "override…",
		Description: "pin a different command for this step"})
	q.Selected = []string{DontPin}
	if pinned != "" && pinned != discovered {
		q.Selected = []string{pinned}
	}
	return q
}

var storeDescriptions = map[string]string{
	"github-issues": "specs and task graphs are GitHub issues",
	"gitlab":        "specs and task graphs are GitLab issues",
	"files":         "specs are Markdown files committed in this repo",
	"none":          "this repo keeps no specs; skills stop looking for one",
}

func specStore(in Input) Question {
	q := Question{Key: KeySpecStore, Kind: Select, Title: "Spec store",
		Description: "Where specs and task graphs live.",
		Note:        rejected(in.Existing, KeySpecStore)}
	for _, s := range repoconfig.SpecStores {
		q.Options = append(q.Options, Option{Value: s, Label: s, Description: storeDescriptions[s]})
	}
	q.Options = append(q.Options, discoverOpt("from docs/agents/issue-tracker.md"))
	disc := ""
	if v := in.Discovered.Spec.Store; v.Source == profile.Discovered {
		disc = v.Value
	}
	pick(&q, in.Existing.Spec.Store, disc, "")
	return q
}

func specRef(in Input) Question {
	q := Question{Key: KeySpecRef, Kind: Select, Title: "Spec ref",
		Description:       "Qualifies the store: owner/repo for a tracker, a path for files.",
		CustomTitle:       "Spec ref",
		CustomPlaceholder: "owner/repo, or a path such as docs/specs"}
	seen := map[string]bool{}
	for _, r := range in.Candidates.Remotes {
		slug := RemoteSlug(r.URL)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		q.Options = append(q.Options, Option{Value: slug, Label: slug,
			Description: "from remote " + r.Name, Provenance: Suggested})
	}
	q.Options = append(q.Options,
		Option{Value: CustomValue, Label: "custom…", Description: "type a ref"},
		discoverOpt("from the GitHub remote when the store is github-issues"))
	disc := ""
	if v := in.Discovered.Spec.Ref; v.Source == profile.Discovered {
		disc = v.Value
	}
	pick(&q, in.Existing.Spec.Ref, disc, "")
	// A discovered remote slug is re-tagged: it is what discovery answers, not
	// merely a candidate.
	for i := range q.Options {
		if q.Options[i].Value == disc && disc != "" {
			q.Options[i].Provenance = Discovered
		}
	}
	return q
}

// RemoteSlug is the path part of a remote URL — owner/repo on GitHub, the full
// group path on GitLab. "" for a URL with no recognisable path.
func RemoteSlug(url string) string {
	u := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	switch {
	case strings.Contains(u, "://"):
		_, rest, _ := strings.Cut(u, "://")
		_, path, ok := strings.Cut(rest, "/")
		if !ok {
			return ""
		}
		u = path
	case strings.Contains(u, ":"):
		// scp-like: git@host:owner/repo
		_, u, _ = strings.Cut(u, ":")
	default:
		return ""
	}
	u = strings.Trim(u, "/")
	if !strings.Contains(u, "/") {
		return ""
	}
	return u
}

// CommitTypes are the conventional-commit types. A directory named after one is
// not offered as a scope: a type and a scope are different things (CONTEXT.md).
var CommitTypes = []string{"feat", "fix", "docs", "style", "refactor", "perf", "test",
	"build", "ci", "chore", "revert"}

func scopes(in Input) []Question {
	history := in.Discovered.Scopes.Values
	pinned := in.Existing.Commit.Scopes

	lead := Question{Key: KeyScopesMode, Kind: Select, Title: "Commit scopes",
		Description: "Pinning freezes the list commit offers; discovery follows history."}
	hint := "no scopes in the last 200 commits yet"
	if len(history) > 0 {
		hint = "from the last 200 commits: " + strings.Join(history, ", ")
	}
	lead.Options = []Option{
		{Value: ScopesDiscover, Label: "keep discovering from history", Description: hint},
		{Value: ScopesPin, Label: "pin a list", Description: "choose the scopes commit may use"},
	}
	switch {
	case len(pinned) > 0:
		lead.Options[1].Provenance = Pinned
		lead.Selected = []string{ScopesPin}
	case len(history) > 0:
		lead.Options[0].Provenance = Discovered
		lead.Selected = []string{ScopesDiscover}
	default:
		lead.Options[1].Provenance = Default
		lead.Selected = []string{ScopesPin}
	}

	list := Question{Key: KeyScopes, Kind: Multi, Title: "Scopes to pin",
		Description: "Ticked scopes are pinned. None ticked pins nothing.",
		ShowIf:      &Condition{Key: KeyScopesMode, Value: ScopesPin},
		CustomTitle: "More scopes", CustomPlaceholder: "comma-separated, e.g. api, web"}
	seen := map[string]bool{}
	add := func(v string, p Provenance, desc string, tick bool) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		list.Options = append(list.Options, Option{Value: v, Label: v, Description: desc, Provenance: p})
		if tick {
			list.Selected = append(list.Selected, v)
		}
	}
	for _, s := range pinned {
		add(s, Pinned, "in the existing config", true)
	}
	// History scopes are ticked only when nothing is pinned: with a pinned list
	// the pin is the answer, and a scope history has since grown is an offer.
	for _, s := range history {
		add(s, Discovered, "used in history", len(pinned) == 0)
	}
	for _, d := range in.Candidates.Dirs {
		if !slices.Contains(CommitTypes, d) {
			add(d, Suggested, "a directory in this repo", false)
		}
	}
	list.Options = append(list.Options, Option{Value: CustomValue, Label: "+ add…",
		Description: "type scopes neither history nor the tree suggests"})
	return []Question{lead, list}
}

// SubjectPresets are the subject-length limits offered as options.
var SubjectPresets = []int{50, 72, 100}

func subjectMax(in Input) Question {
	q := Question{Key: KeySubjectMax, Kind: Select, Title: "Subject max",
		Description: "The longest commit subject this repo accepts. Not discoverable.",
		Note:        rejected(in.Existing, KeySubjectMax),
		CustomTitle: "Subject max", CustomPlaceholder: "a number of characters"}
	desc := map[int]string{50: "the classic git convention", 72: "fits a standard terminal line",
		100: "roomy; fits most web views"}
	for _, n := range SubjectPresets {
		q.Options = append(q.Options, Option{Value: strconv.Itoa(n), Label: strconv.Itoa(n), Description: desc[n]})
	}
	q.Options = append(q.Options,
		Option{Value: CustomValue, Label: "custom…", Description: "type another limit"},
		Option{Value: DontPin, Label: "don't pin", Description: "commit uses its own convention"})
	pinned := ""
	if in.Existing.Commit.SubjectMax != nil {
		pinned = strconv.Itoa(*in.Existing.Commit.SubjectMax)
	}
	pick(&q, pinned, "", "")
	return q
}

func reviewMode(in Input) Question {
	q := Question{Key: KeyReviewMode, Kind: Select, Title: "Review mode",
		Description: "The roster review opens with when you name none.",
		Note:        rejected(in.Existing, KeyReviewMode)}
	desc := map[string]string{
		"full":  "CodeRabbit + Codex + Claude — the thorough pass, the most tokens",
		"quick": "CodeRabbit + Codex, bugs and implementation only — cheaper, narrower",
	}
	for _, m := range repoconfig.ReviewModes {
		q.Options = append(q.Options, Option{Value: m, Label: m, Description: desc[m]})
	}
	q.Options = append(q.Options, discoverOpt("review falls back to full"))
	pick(&q, in.Existing.Review.Mode, "", "full")
	return q
}

func reviewers(in Input) Question {
	q := Question{Key: KeyReviewers, Kind: Multi, Title: "Reviewers",
		Description: "Ticked reviewers are pinned. None ticked keeps reading CODEOWNERS.",
		CustomTitle: "More reviewers", CustomPlaceholder: "comma-separated, e.g. @alice, @org/team"}
	seen := map[string]bool{}
	for _, r := range in.Existing.Review.Reviewers {
		seen[r] = true
		q.Options = append(q.Options, Option{Value: r, Label: r, Description: "in the existing config", Provenance: Pinned})
		q.Selected = append(q.Selected, r)
	}
	for _, r := range in.Discovered.Review.Values {
		if !seen[r] {
			seen[r] = true
			q.Options = append(q.Options, Option{Value: r, Label: r, Description: "from CODEOWNERS", Provenance: Discovered})
		}
	}
	q.Options = append(q.Options, Option{Value: CustomValue, Label: "+ add…", Description: "type reviewer handles"})
	return q
}

func mergeStyle(in Input) Question {
	q := Question{Key: KeyMerge, Kind: Select, Title: "Merge style",
		Description: "How this repo integrates a branch.",
		Note:        rejected(in.Existing, KeyMerge)}
	desc := map[string]string{
		"merge":  "a merge commit per PR; every branch commit stays in history",
		"squash": "one commit per PR; the branch's commits are folded away",
		"rebase": "branch commits replayed onto the base; linear, no merge commit",
	}
	for _, s := range repoconfig.MergeStyles {
		q.Options = append(q.Options, Option{Value: s, Label: s, Description: desc[s]})
	}
	q.Options = append(q.Options, discoverOpt("from mkit.mergeStyle or pull.rebase in this repo's git config"))
	disc := ""
	if v := in.Discovered.Merge; v.Source == profile.Discovered && repoconfig.OneOf(v.Value, repoconfig.MergeStyles) {
		disc = v.Value
	}
	pick(&q, in.Existing.Merge.Style, disc, "merge")
	return q
}

func keep(in Input) Question {
	q := Question{Key: KeyKeep, Kind: Multi, Title: "Keep branches",
		Locked:      slices.Clone(in.Candidates.Protected),
		CustomTitle: "More branches", CustomPlaceholder: "comma-separated branch names"}
	if len(q.Locked) > 0 {
		q.Description = "Branches cleanup must never delete. Always kept: " +
			strings.Join(q.Locked, ", ") + "."
	} else {
		q.Description = "Branches cleanup must never delete. The default branch is kept regardless."
	}
	seen := map[string]bool{}
	for _, b := range q.Locked {
		seen[b] = true
	}
	for _, b := range in.Existing.Cleanup.Keep {
		// A pinned name that is also locked stays pre-selected, so Apply keeps
		// the line a human wrote; it is not offered, since it cannot be unticked.
		if slices.Contains(q.Locked, b) {
			q.Selected = append(q.Selected, b)
			continue
		}
		if !seen[b] {
			seen[b] = true
			q.Options = append(q.Options, Option{Value: b, Label: b, Description: "in the existing config", Provenance: Pinned})
			q.Selected = append(q.Selected, b)
		}
	}
	for _, b := range in.Candidates.Branches {
		if !seen[b] {
			seen[b] = true
			q.Options = append(q.Options, Option{Value: b, Label: b, Description: "a local branch", Provenance: Suggested})
		}
	}
	q.Options = append(q.Options, Option{Value: CustomValue, Label: "+ add…", Description: "type branch names"})
	return q
}

// Validate checks one free-text value for a question, with the same rules the
// flags apply — one producer per rule, in repoconfig.
func Validate(key, text string) error {
	text = strings.TrimSpace(text)
	switch {
	case key == KeySubjectMax:
		n, err := strconv.Atoi(text)
		if err != nil || n <= 0 {
			return fmt.Errorf("a subject length is %s", repoconfig.Rule("commit.subject_max"))
		}
	case key == KeyGateNew:
		step, command, ok := strings.Cut(text, "=")
		if !ok || strings.TrimSpace(step) == "" || strings.TrimSpace(command) == "" {
			return fmt.Errorf("expected step=command")
		}
	case strings.HasPrefix(key, gateStepPre), key == KeySpecRef:
		if text == "" {
			return fmt.Errorf("type a value, or go back and choose another option")
		}
	case key == KeyScopes, key == KeyReviewers, key == KeyKeep:
		if len(splitList(text)) == 0 {
			return fmt.Errorf("type at least one name, or untick + add")
		}
	default:
		if allowed := repoconfig.Allowed(key); allowed != nil && !repoconfig.OneOf(text, allowed) {
			return fmt.Errorf("expected one of %s", strings.Join(allowed, ", "))
		}
	}
	return nil
}

// Apply turns answers into the config to write. The result holds only what the
// answers pin: "don't pin" leaves a key unset, and a config whose every answer
// is "don't pin" is zero.
func Apply(p *Plan, a Answers) (*repoconfig.Config, error) {
	cfg := &repoconfig.Config{}
	one := func(key string) (string, error) {
		q := p.Question(key)
		if q == nil || !Visible(*q, a) {
			return "", nil
		}
		ch := a.Choice[key]
		if len(ch) == 0 {
			return "", nil
		}
		if ch[0] != CustomValue {
			return ch[0], nil
		}
		text := strings.TrimSpace(a.Custom[key])
		if err := Validate(key, text); err != nil {
			return "", fmt.Errorf("%s: %w", q.Key, err)
		}
		return text, nil
	}
	many := func(key string) ([]string, error) {
		q := p.Question(key)
		if q == nil || !Visible(*q, a) {
			return nil, nil
		}
		var out []string
		add := func(v string) {
			if v != "" && !slices.Contains(out, v) {
				out = append(out, v)
			}
		}
		for _, v := range a.Choice[key] {
			if v != CustomValue {
				add(v)
			}
		}
		if slices.Contains(a.Choice[key], CustomValue) {
			if err := Validate(key, a.Custom[key]); err != nil {
				return nil, fmt.Errorf("%s: %w", q.Key, err)
			}
			for _, v := range splitList(a.Custom[key]) {
				add(v)
			}
		}
		return out, nil
	}

	for _, pg := range p.Pages {
		for _, q := range pg.Questions {
			if !strings.HasPrefix(q.Key, gateStepPre) {
				continue
			}
			v, err := one(q.Key)
			if err != nil {
				return nil, err
			}
			if v != "" {
				setGate(cfg, strings.TrimPrefix(q.Key, gateStepPre), v)
			}
		}
	}
	if v, err := one(KeyGateNew); err != nil {
		return nil, err
	} else if v != "" {
		step, command, _ := strings.Cut(v, "=")
		setGate(cfg, strings.TrimSpace(step), strings.TrimSpace(command))
	}

	var err error
	if cfg.Spec.Store, err = one(KeySpecStore); err != nil {
		return nil, err
	}
	if cfg.Spec.Ref, err = one(KeySpecRef); err != nil {
		return nil, err
	}
	if cfg.Commit.Scopes, err = many(KeyScopes); err != nil {
		return nil, err
	}
	sm, err := one(KeySubjectMax)
	if err != nil {
		return nil, err
	}
	if sm != "" {
		n, err := strconv.Atoi(sm)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("%s %q: a subject length is %s", KeySubjectMax, sm, repoconfig.Rule("commit.subject_max"))
		}
		cfg.Commit.SubjectMax = &n
	}
	if cfg.Review.Reviewers, err = many(KeyReviewers); err != nil {
		return nil, err
	}
	if cfg.Review.Mode, err = one(KeyReviewMode); err != nil {
		return nil, err
	}
	if cfg.Merge.Style, err = one(KeyMerge); err != nil {
		return nil, err
	}
	keepList, err := many(KeyKeep)
	if err != nil {
		return nil, err
	}
	// A locked branch is protected whatever the list says, so it is written only
	// if the existing config already named it — a rewrite should not churn a
	// line a human put there.
	if q := p.Question(KeyKeep); q != nil {
		for _, l := range q.Locked {
			if slices.Contains(keepList, l) {
				continue
			}
			if slices.Contains(q.Selected, l) {
				keepList = append([]string{l}, keepList...)
			}
		}
	}
	cfg.Cleanup.Keep = keepList
	return cfg, nil
}

func setGate(cfg *repoconfig.Config, step, command string) {
	if cfg.Gate.Commands == nil {
		cfg.Gate.Commands = map[string]string{}
	}
	cfg.Gate.Commands[step] = command
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}
