// Package repoinit renders the paged form `mkit init` opens on a TTY.
//
// It renders an initplan.Plan and returns initplan.Answers, and nothing else.
// What is asked, what is offered, what is pre-selected and how answers become a
// config are all initplan's — no init logic lives here, per the layering
// invariant.
package repoinit

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"

	"github.com/masterik/mk-toolkit/internal/core/initplan"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// Bindings holds the form's live values, one pointer per field, so a rebuilt
// form (after Back on the review page) opens on the answers already given.
type Bindings struct {
	plan   *initplan.Plan
	sel    map[string]*string
	multi  map[string]*[]string
	custom map[string]*string
}

// NewBindings seeds the values from a set of answers — normally the plan's
// pre-selection.
func NewBindings(p *initplan.Plan, a initplan.Answers) *Bindings {
	b := &Bindings{plan: p, sel: map[string]*string{}, multi: map[string]*[]string{}, custom: map[string]*string{}}
	for _, pg := range p.Pages {
		for _, q := range pg.Questions {
			ch := a.Choice[q.Key]
			if q.Kind == initplan.Multi {
				v := append([]string(nil), ch...)
				b.multi[q.Key] = &v
			} else {
				v := initplan.DontPin
				if len(ch) > 0 {
					v = ch[0]
				}
				b.sel[q.Key] = &v
			}
			c := a.Custom[q.Key]
			b.custom[q.Key] = &c
		}
	}
	return b
}

// Answers reads the current values back.
func (b *Bindings) Answers() initplan.Answers {
	a := initplan.Answers{Choice: map[string][]string{}, Custom: map[string]string{}}
	for k, v := range b.sel {
		a.Choice[k] = []string{*v}
	}
	for k, v := range b.multi {
		a.Choice[k] = append([]string(nil), (*v)...)
	}
	for k, v := range b.custom {
		if *v != "" {
			a.Custom[k] = *v
		}
	}
	return a
}

// keyMap is huh's, with Esc added to abort: leaving is always safe, since
// nothing is written until the review page says Write.
func keyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("esc", "abort"))
	// Option lists are short; a filter prompt is noise in the help line.
	km.Select.Filter.SetEnabled(false)
	return km
}

// NewForm builds the question pages over the bindings.
func NewForm(b *Bindings) *huh.Form {
	var groups []*huh.Group
	n := len(b.plan.Pages)
	for i, pg := range b.plan.Pages {
		title := fmt.Sprintf("%smkit init · %d/%d · %s", diamond, i+1, n, pg.Title)
		var plain []huh.Field
		flush := func() {
			if len(plain) > 0 {
				groups = append(groups, huh.NewGroup(plain...).Title(title))
				plain = nil
			}
		}
		for _, q := range pg.Questions {
			f := field(b, q)
			if q.ShowIf != nil {
				flush()
				groups = append(groups, huh.NewGroup(f).Title(title).
					WithHideFunc(func() bool { return !initplan.Visible(q, b.Answers()) }))
			} else {
				plain = append(plain, f)
			}
			if hasCustom(q) {
				flush()
				groups = append(groups, huh.NewGroup(customInput(b, q)).Title(title).
					WithHideFunc(func() bool { return !initplan.WantsCustom(q, b.Answers()) }))
			}
		}
		flush()
	}
	return huh.NewForm(groups...).
		WithTheme(clackTheme()).
		WithKeyMap(keyMap()).
		WithShowHelp(true)
}

func hasCustom(q initplan.Question) bool {
	for _, o := range q.Options {
		if o.Custom() {
			return true
		}
	}
	return false
}

// label carries provenance as text, so the form reads the same without colour.
func label(o initplan.Option) string {
	if o.Provenance == "" {
		return o.Label
	}
	return fmt.Sprintf("%s  (%s)", o.Label, o.Provenance)
}

func describe(q initplan.Question) string {
	d := q.Description
	if q.Note != "" {
		d += "\n! " + q.Note
	}
	return d
}

func field(b *Bindings, q initplan.Question) huh.Field {
	if q.Kind == initplan.Multi {
		opts := make([]huh.Option[string], len(q.Options))
		for i, o := range q.Options {
			l := label(o)
			if o.Description != "" {
				l += " — " + o.Description
			}
			opts[i] = huh.NewOption(l, o.Value)
		}
		return huh.NewMultiSelect[string]().
			Key(q.Key).Title(q.Title).
			Description(describe(q) + "\nspace or x toggles · enter continues").
			Options(opts...).
			Filterable(false).
			Value(b.multi[q.Key])
	}
	opts := make([]huh.Option[string], len(q.Options))
	byValue := map[string]string{}
	for i, o := range q.Options {
		opts[i] = huh.NewOption(label(o), o.Value)
		byValue[o.Value] = o.Description
	}
	v := b.sel[q.Key]
	// huh scrolls a select's viewport to the option its value names when the
	// options are set, and never scrolls back: pre-selecting the fourth of five
	// options would hide the first three. So the options go in against a value
	// that names none (viewport at the top), and the real binding comes after,
	// which moves only the cursor.
	unset := "\x00unset"
	return huh.NewSelect[string]().
		Value(&unset).
		Key(q.Key).Title(q.Title).
		// The hovered option's description, so every choice explains itself
		// before it is taken.
		DescriptionFunc(func() string {
			if d := byValue[*v]; d != "" {
				return describe(q) + "\n› " + d
			}
			return describe(q)
		}, v).
		Options(opts...).
		Value(v)
}

func customInput(b *Bindings, q initplan.Question) huh.Field {
	return huh.NewInput().
		Key(q.Key + ".custom").
		Title(q.CustomTitle).
		Placeholder(q.CustomPlaceholder).
		Validate(func(s string) error { return initplan.Validate(q.Key, s) }).
		Value(b.custom[q.Key])
}

const (
	actionWrite = "write"
	actionBack  = "back"
	actionAbort = "abort"
)

// reviewForm shows the exact file Write would produce, then asks. The preview is
// the select's description, not a Note: a Note reads backticks as markup, and
// the file's comments are full of them — the page would no longer show the bytes
// that get written.
func reviewForm(preview string, empty bool, action *string) *huh.Form {
	title := diamond + "Review · " + repoconfig.RelPath
	body := preview
	write := huh.NewOption("Write the file", actionWrite)
	if empty {
		body = "Nothing to pin — every answer is \"don't pin\", so discovery already answers " +
			"everything and nothing will be written."
		write = huh.NewOption("Finish without writing", actionWrite)
	}
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Description(body).Options(
			write,
			huh.NewOption("Back — change an answer", actionBack),
			huh.NewOption("Abort — write nothing", actionAbort),
		).Value(action),
	)).WithTheme(clackTheme()).WithKeyMap(keyMap()).WithShowHelp(true)
}

// Run opens the form and returns the answers, and whether the user chose to
// write. A false second return means abort — the caller writes nothing.
func Run(p *initplan.Plan) (initplan.Answers, bool, error) {
	b := NewBindings(p, p.Preselected())
	for {
		if err := NewForm(b).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return initplan.Answers{}, false, nil
			}
			return initplan.Answers{}, false, err
		}
		a := b.Answers()
		cfg, err := initplan.Apply(p, a)
		if err != nil {
			return initplan.Answers{}, false, err
		}
		action := actionWrite
		if err := reviewForm(repoconfig.Render(cfg), cfg.IsZero(), &action).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return initplan.Answers{}, false, nil
			}
			return initplan.Answers{}, false, err
		}
		switch action {
		case actionWrite:
			return a, true, nil
		case actionAbort:
			return initplan.Answers{}, false, nil
		}
	}
}
