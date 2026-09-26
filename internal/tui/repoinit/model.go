// Package repoinit renders the wizard `mkit init` opens on a TTY.
//
// It renders an initplan.Plan and returns initplan.Answers, and nothing else.
// What is asked, what is offered, what is pre-selected, which answers are valid
// and how answers become a config are all initplan's — Update only moves a
// cursor, toggles a tick and walks between prompts, per the layering invariant.
package repoinit

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/masterik/mk-toolkit/internal/core/initplan"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// step is one prompt: a question, or the free-text prompt behind its custom
// option.
type step struct {
	page   int
	q      *initplan.Question
	custom bool
}

func (s step) id() string {
	if s.custom {
		return s.q.Key + ".custom"
	}
	return s.q.Key
}

// action is one choice on the review page.
type action struct {
	kind  int
	page  int // for actionEdit
	label string
	hint  string
}

const (
	actionWrite = iota
	actionEdit
	actionBack
	actionAbort
)

// walkMain is the walk over the pages that are not optional.
const walkMain = -1

// Model is the wizard. Its values are the answers themselves, so walking back
// to a prompt opens it on what was already chosen.
type Model struct {
	plan   *initplan.Plan
	sel    map[string]string
	multi  map[string][]string
	custom map[string]string
	cursor map[string]int // a multi-select's row cursor

	// walk is walkMain, or the index of the optional page opened from review.
	walk   int
	at     string // the current step's id; ignored on the review page
	review bool
	action int
	scroll int // the review preview's first line
	cfg    *repoconfig.Config
	cfgErr error

	input  textinput.Model
	err    string // why the current prompt refused Enter
	width  int
	height int

	done  bool
	write bool
}

// New opens the wizard on the plan's pre-selection.
func New(p *initplan.Plan) *Model {
	m := &Model{plan: p, sel: map[string]string{}, multi: map[string][]string{},
		custom: map[string]string{}, cursor: map[string]int{}, walk: walkMain, width: 80, height: 24}
	a := p.Preselected()
	for _, pg := range p.Pages {
		for _, q := range pg.Questions {
			if q.Kind == initplan.Multi {
				m.multi[q.Key] = slices.Clone(a.Choice[q.Key])
			} else if ch := a.Choice[q.Key]; len(ch) > 0 {
				m.sel[q.Key] = ch[0]
			} else {
				m.sel[q.Key] = initplan.DontPin
			}
		}
	}
	m.input = textinput.New()
	m.input.Prompt = ""
	if s := m.steps(); len(s) > 0 {
		m.at = s[0].id()
	} else {
		m.enterReview()
	}
	return m
}

// Answers reads the current values back.
func (m *Model) Answers() initplan.Answers {
	a := initplan.Answers{Choice: map[string][]string{}, Custom: map[string]string{}}
	for k, v := range m.sel {
		a.Choice[k] = []string{v}
	}
	for k, v := range m.multi {
		if len(v) > 0 {
			a.Choice[k] = slices.Clone(v)
		} else {
			a.Choice[k] = nil
		}
	}
	for k, v := range m.custom {
		if v != "" {
			a.Custom[k] = v
		}
	}
	return a
}

// Done reports whether the wizard finished, and Write whether it finished by
// choosing to write.
func (m *Model) Done() bool  { return m.done }
func (m *Model) Write() bool { return m.write }

// Current is the id of the prompt on screen, or "review".
func (m *Model) Current() string {
	if m.review {
		return "review"
	}
	return m.at
}

// steps is the walk as the answers so far shape it: a question initplan hides
// is skipped, and a custom option chosen adds its prompt right after.
func (m *Model) steps() []step {
	a := m.Answers()
	var out []step
	for i := range m.plan.Pages {
		pg := &m.plan.Pages[i]
		if (m.walk == walkMain && pg.Optional) || (m.walk != walkMain && m.walk != i) {
			continue
		}
		for j := range pg.Questions {
			q := &pg.Questions[j]
			if !initplan.Visible(*q, a) {
				continue
			}
			out = append(out, step{page: i, q: q})
			if initplan.WantsCustom(*q, a) {
				out = append(out, step{page: i, q: q, custom: true})
			}
		}
	}
	return out
}

func (m *Model) index(steps []step) int {
	for i, s := range steps {
		if s.id() == m.at {
			return i
		}
	}
	return 0
}

func (m *Model) current() (step, int, []step) {
	s := m.steps()
	i := m.index(s)
	return s[i], i, s
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m.finish(false)
		}
		if m.review {
			return m.updateReview(msg)
		}
		s, i, _ := m.current()
		switch msg.String() {
		case "esc":
			if i == 0 && m.walk == walkMain {
				return m.finish(false)
			}
			return m, m.back()
		case "shift+tab":
			return m, m.back()
		}
		if s.custom {
			return m.updateInput(s, msg)
		}
		return m.updateChoice(s, msg)
	}
	if !m.review {
		if s, _, _ := m.current(); s.custom {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) finish(write bool) (tea.Model, tea.Cmd) {
	m.done, m.write = true, write
	m.input.Blur()
	return m, tea.Quit
}

func (m *Model) updateChoice(s step, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	q := s.q
	n := len(q.Options)
	if q.Kind == initplan.Multi {
		c := m.cursor[q.Key]
		switch msg.String() {
		case "up", "k":
			m.cursor[q.Key] = max(c-1, 0)
		case "down", "j":
			m.cursor[q.Key] = min(c+1, n-1)
		case " ", "x":
			if n > 0 {
				v := q.Options[c].Value
				if i := slices.Index(m.multi[q.Key], v); i >= 0 {
					m.multi[q.Key] = slices.Delete(m.multi[q.Key], i, i+1)
				} else {
					m.multi[q.Key] = append(m.multi[q.Key], v)
				}
			}
		case "enter", "tab":
			return m, m.advance()
		}
		return m, nil
	}
	c := m.selIndex(q)
	switch msg.String() {
	case "up", "k":
		m.sel[q.Key] = q.Options[max(c-1, 0)].Value
	case "down", "j":
		m.sel[q.Key] = q.Options[min(c+1, n-1)].Value
	case "enter", "tab":
		return m, m.advance()
	}
	return m, nil
}

func (m *Model) updateInput(s step, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		text := strings.TrimSpace(m.input.Value())
		if err := initplan.Validate(s.q.Key, text); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.custom[s.q.Key] = text
		return m, m.advance()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = ""
	return m, cmd
}

func (m *Model) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	acts := m.actions()
	switch msg.String() {
	case "up", "k":
		m.action = max(m.action-1, 0)
	case "down", "j":
		m.action = min(m.action+1, len(acts)-1)
	case "pgdown", "ctrl+d", " ":
		m.scroll += m.previewRows()
	case "pgup", "ctrl+u":
		m.scroll -= m.previewRows()
	case "esc", "shift+tab":
		return m, m.back()
	case "enter":
		switch a := acts[m.action]; a.kind {
		case actionWrite:
			if m.cfgErr != nil {
				m.err = m.cfgErr.Error()
				return m, nil
			}
			return m.finish(true)
		case actionEdit:
			return m, m.open(a.page)
		case actionBack:
			return m, m.back()
		case actionAbort:
			return m.finish(false)
		}
	}
	m.scroll = max(0, min(m.scroll, len(m.previewLines())-m.previewRows()))
	return m, nil
}

func (m *Model) selIndex(q *initplan.Question) int {
	for i, o := range q.Options {
		if o.Value == m.sel[q.Key] {
			return i
		}
	}
	return 0
}

// advance moves to the next prompt, recomputing the walk first: the answer
// just given may have shown or hidden what follows. Past the last prompt of a
// walk — the main one or an optional page — is the review page.
func (m *Model) advance() tea.Cmd {
	m.err = ""
	s := m.steps()
	i := m.index(s)
	if i+1 >= len(s) {
		m.enterReview()
		return nil
	}
	m.at = s[i+1].id()
	return m.enterStep(s[i+1])
}

// back moves to the previous prompt. From review it is the main walk's last
// prompt; from an optional page's first prompt it is review again.
func (m *Model) back() tea.Cmd {
	m.err = ""
	if m.review {
		m.walk = walkMain
		s := m.steps()
		// A plan with nothing to walk opens on review and stays there.
		if len(s) == 0 {
			return nil
		}
		m.review = false
		m.at = s[len(s)-1].id()
		return m.enterStep(s[len(s)-1])
	}
	s := m.steps()
	i := m.index(s)
	if i == 0 {
		if m.walk != walkMain {
			m.enterReview()
		}
		return nil
	}
	m.at = s[i-1].id()
	return m.enterStep(s[i-1])
}

// open walks an optional page from the review page.
func (m *Model) open(page int) tea.Cmd {
	prev := m.walk
	m.walk = page
	s := m.steps()
	if len(s) == 0 {
		m.walk = prev
		return nil
	}
	m.review, m.err = false, ""
	m.at = s[0].id()
	return m.enterStep(s[0])
}

func (m *Model) enterStep(s step) tea.Cmd {
	if !s.custom {
		m.input.Blur()
		return nil
	}
	m.input.SetValue(m.custom[s.q.Key])
	m.input.Placeholder = s.q.CustomPlaceholder
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *Model) enterReview() {
	back := m.walk
	m.review, m.walk, m.scroll, m.err = true, walkMain, 0, ""
	m.input.Blur()
	m.cfg, m.cfgErr = initplan.Apply(m.plan, m.Answers())
	// Returning from an optional page lands on that page's row, not on Write.
	m.action = 0
	for i, a := range m.actions() {
		if a.kind == actionEdit && a.page == back {
			m.action = i
		}
	}
}

// actions are the review page's choices: write, open each optional page, back,
// abort.
func (m *Model) actions() []action {
	write := "Write the file"
	if m.cfgErr == nil && m.cfg != nil && m.cfg.IsZero() {
		write = "Finish without writing"
	}
	out := []action{{kind: actionWrite, label: write}}
	for i, pg := range m.plan.Pages {
		if pg.Optional {
			out = append(out, action{kind: actionEdit, page: i, label: "Change " + strings.ToLower(pg.Title),
				hint: m.summary(i)})
		}
	}
	return append(out,
		action{kind: actionBack, label: "Back — change an answer above"},
		action{kind: actionAbort, label: "Abort — write nothing"})
}

// summary is what an optional page currently pins, for its review row.
func (m *Model) summary(page int) string {
	if m.cfg == nil {
		return ""
	}
	switch m.plan.Pages[page].Title {
	case "Gate":
		if len(m.cfg.Gate.Commands) == 0 {
			return "nothing pinned; discovered each run"
		}
		var names []string
		for k := range m.cfg.Gate.Commands {
			names = append(names, k)
		}
		sort.Strings(names)
		for i, k := range names {
			names[i] = k + ": " + m.cfg.Gate.Commands[k]
		}
		return "pins " + strings.Join(names, " · ")
	case "Cleanup":
		var keep []string
		if q := m.plan.Question(initplan.KeyKeep); q != nil {
			keep = append(keep, q.Locked...)
		}
		for _, k := range m.cfg.Cleanup.Keep {
			if !slices.Contains(keep, k) {
				keep = append(keep, k)
			}
		}
		if len(keep) == 0 {
			return "keeps the default branch"
		}
		return "keeps " + strings.Join(keep, ", ")
	}
	return ""
}

// ---- rendering ----

func (m *Model) inner() int { return max(m.width-3, 20) }

func bar() string { return sDim.Render(markBar) }

// row is one line under the rule.
func row(text string) string { return bar() + "  " + text }

// wrap breaks plain text to the column width, less indent, one styled row per
// line.
func (m *Model) wrap(text string, st lipgloss.Style, indent int) []string {
	if text == "" {
		return nil
	}
	w := lipgloss.NewStyle().Width(m.inner() - indent).Render(text)
	pad := strings.Repeat(" ", indent)
	var out []string
	for _, l := range strings.Split(w, "\n") {
		out = append(out, row(pad+st.Render(strings.TrimRight(l, " "))))
	}
	return out
}

func (m *Model) progress(page int) string {
	if m.plan.Pages[page].Optional {
		return "optional"
	}
	n, at := 0, 0
	for i, pg := range m.plan.Pages {
		if !pg.Optional {
			n++
			if i == page {
				at = n
			}
		}
	}
	return fmt.Sprintf("%d/%d", at, n)
}

func label(o initplan.Option) string {
	if o.Provenance == "" {
		return sFg.Render(o.Label)
	}
	return sFg.Render(o.Label) + sDim.Render("  ("+string(o.Provenance)+")")
}

// answer is a prompt's value as the collapsed history shows it.
func (m *Model) answer(s step) string {
	q := s.q
	if s.custom {
		return m.custom[q.Key]
	}
	if q.Kind == initplan.Multi {
		var ls []string
		for _, o := range q.Options {
			if slices.Contains(m.multi[q.Key], o.Value) && !o.Custom() {
				ls = append(ls, o.Label)
			}
		}
		if slices.Contains(m.multi[q.Key], initplan.CustomValue) && m.custom[q.Key] != "" {
			ls = append(ls, m.custom[q.Key])
		}
		if len(ls) == 0 {
			return "none"
		}
		return strings.Join(ls, ", ")
	}
	return q.Options[m.selIndex(q)].Label
}

// View implements tea.Model.
func (m *Model) View() string {
	var lines []string
	switch {
	case m.done:
		lines = m.viewDone()
	case m.review:
		lines = m.viewReview()
	default:
		lines = m.viewStep()
	}
	// The prompt in hand is at the bottom; what scrolls off is the oldest history.
	if len(lines) > m.height && m.height > 0 {
		lines = lines[len(lines)-m.height:]
	}
	// Prose is wrapped where it is built; what is still too wide here is a title
	// or the help line, which lose their tail rather than break the rule.
	if m.width > 0 {
		fit := lipgloss.NewStyle().MaxWidth(m.width)
		for i, l := range lines {
			lines[i] = fit.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) intro() []string {
	return []string{sDim.Render(markIntro) + "  " + sTitle.Render("mkit init") +
		sDim.Render("  pin how this repo works, in "+repoconfig.RelPath), bar()}
}

// history is one line per answered prompt; a custom option's text stands in
// for the option on its question's line once it has been typed.
func (m *Model) history(upto int, s []step) []string {
	var out []string
	for i := 0; i < upto; i++ {
		st := s[i]
		if st.custom {
			continue
		}
		v := m.answer(st)
		if m.sel[st.q.Key] == initplan.CustomValue && i+1 < upto && m.custom[st.q.Key] != "" {
			v = m.custom[st.q.Key]
		}
		out = append(out, sDim.Render(markDone)+"  "+st.q.Title+sDim.Render(" · "+v))
	}
	if len(out) > 0 {
		out = append(out, bar())
	}
	return out
}

func (m *Model) viewStep() []string {
	s, i, all := m.current()
	lines := m.intro()
	// History is the page's own when an optional page is open: review is where
	// the main walk's answers are.
	lines = append(lines, m.history(i, all)...)
	pg := m.plan.Pages[s.page]
	q := s.q

	lines = append(lines, sAccent.Render(markActive)+"  "+sTitle.Render(pg.Title)+
		sDim.Render("  "+m.progress(s.page)))
	lines = append(lines, m.wrap(pg.Intro, sDim, 0)...)
	lines = append(lines, bar())

	title := q.Title
	if s.custom {
		title = q.CustomTitle
	}
	lines = append(lines, row(sFg.Bold(true).Render(title)))

	var help string
	switch {
	case s.custom:
		lines = append(lines, row(sAccent.Render(markCursor)+" "+m.input.View()))
		help = "enter confirm"
	case q.Kind == initplan.Multi:
		lines = append(lines, m.wrap(q.Description, sDim, 0)...)
		lines = append(lines, m.wrap(noteText(q), sWarn, 0)...)
		lines = append(lines, m.viewMulti(q)...)
		help = "↑/↓ move · space toggle · enter confirm"
	default:
		lines = append(lines, m.wrap(q.Description, sDim, 0)...)
		lines = append(lines, m.wrap(noteText(q), sWarn, 0)...)
		lines = append(lines, m.viewSelect(q)...)
		help = "↑/↓ choose · enter confirm"
	}
	if m.err != "" {
		lines = append(lines, m.wrap(markError+" "+m.err, sBad, 0)...)
	}
	switch {
	case i == 0 && m.walk == walkMain:
		help += " · esc abort"
	case i == 0:
		help += " · esc back to review · ctrl+c abort"
	default:
		help += " · esc back · ctrl+c abort"
	}
	return append(lines, sDim.Render(markOutro)+"  "+sDim.Render(help))
}

func noteText(q *initplan.Question) string {
	if q.Note == "" {
		return ""
	}
	return "! " + q.Note
}

// window is the slice of rows shown for a list of n with the cursor at c, when
// rows fit. It starts at the top and moves only once the cursor would leave it,
// so a pre-selection near the end never hides the options above it that fit.
func window(n, c, rows int) (int, int) {
	rows = max(3, rows)
	if n <= rows {
		return 0, n
	}
	from := 0
	if c >= rows {
		from = c - rows + 1
	}
	return from, from + rows
}

// viewSelect shows every option with its description beneath it: a choice is
// only a choice when what each one does is on screen.
func (m *Model) viewSelect(q *initplan.Question) []string {
	c := m.selIndex(q)
	from, to := window(len(q.Options), c, (m.height-16)/2)
	var lines []string
	if from > 0 {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("↑ %d more", from))))
	}
	for i := from; i < to; i++ {
		o := q.Options[i]
		if i == c {
			lines = append(lines, row(sAccent.Render(markOn)+" "+label(o)))
		} else {
			lines = append(lines, row(sDim.Render(markOff)+" "+label(o)))
		}
		lines = append(lines, m.wrap(o.Description, sDim, 4)...)
	}
	if to < len(q.Options) {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("↓ %d more", len(q.Options)-to))))
	}
	return lines
}

func (m *Model) viewMulti(q *initplan.Question) []string {
	var lines []string
	// Locked values are shown ticked and never reachable by the cursor: they are
	// kept whatever the answer, so offering to untick one would be a lie.
	for _, l := range q.Locked {
		lines = append(lines, row("  "+sDim.Render(markTicked+" "+l+"  (always kept)")))
	}
	c := min(m.cursor[q.Key], len(q.Options)-1)
	from, to := window(len(q.Options), c, m.height-16)
	if from > 0 {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("  ↑ %d more", from))))
	}
	for i := from; i < to; i++ {
		o := q.Options[i]
		mark := sDim.Render(markUnticked)
		if slices.Contains(m.multi[q.Key], o.Value) {
			mark = sAccent.Render(markTicked)
		}
		lead := "  "
		text := sDim.Render(o.Label)
		if i == c {
			lead = sAccent.Render(markCursor) + " "
			text = label(o)
		}
		lines = append(lines, row(lead+mark+" "+text))
	}
	if to < len(q.Options) {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("  ↓ %d more", len(q.Options)-to))))
	}
	if c >= 0 {
		if d := q.Options[c].Description; d != "" {
			lines = append(lines, m.wrap("↳ "+d, sDim, 0)...)
		}
	}
	return lines
}

func (m *Model) previewLines() []string {
	switch {
	case m.cfgErr != nil:
		return nil
	case m.cfg.IsZero():
		return strings.Split(strings.TrimSpace(lipgloss.NewStyle().Width(m.inner()).Render(
			"Nothing to pin — every answer is \"don't pin\", so discovery already answers "+
				"everything and nothing will be written.")), "\n")
	}
	return strings.Split(strings.TrimRight(repoconfig.Render(m.cfg), "\n"), "\n")
}

// previewRows is how much of the file fits above the review options.
func (m *Model) previewRows() int { return max(3, m.height-8-2*len(m.actions())) }

func (m *Model) viewReview() []string {
	lines := m.intro()
	s := m.steps()
	lines = append(lines, m.history(len(s), s)...)
	lines = append(lines, sAccent.Render(markActive)+"  "+sTitle.Render("Review")+
		sDim.Render("  the file below is what Write puts in "+repoconfig.RelPath))
	if m.cfgErr != nil {
		lines = append(lines, m.wrap(markError+" "+m.cfgErr.Error(), sBad, 0)...)
	}
	pl := m.previewLines()
	from, to := m.scroll, min(m.scroll+m.previewRows(), len(pl))
	if from > 0 {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("↑ %d lines above", from))))
	}
	for _, l := range pl[from:to] {
		// The file is shown as written, bytes and all: a line wider than the
		// column is cut, never re-wrapped into something the file does not say.
		if lipgloss.Width(l) > m.inner() {
			l = string([]rune(l)[:max(0, m.inner()-1)]) + "…"
		}
		lines = append(lines, row(l))
	}
	if to < len(pl) {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("↓ %d more lines · pgdn", len(pl)-to))))
	}
	lines = append(lines, bar())
	for i, a := range m.actions() {
		mark := sDim.Render(markOff)
		if i == m.action {
			mark = sAccent.Render(markOn)
		}
		lines = append(lines, row(mark+" "+sFg.Render(a.label)))
		if a.hint != "" {
			lines = append(lines, m.wrap(a.hint, sDim, 4)...)
		}
	}
	if m.err != "" {
		lines = append(lines, m.wrap(markError+" "+m.err, sBad, 0)...)
	}
	return append(lines, sDim.Render(markOutro)+"  "+
		sDim.Render("↑/↓ choose · enter confirm · pgup/pgdn scroll · esc back · ctrl+c abort"))
}

// viewDone is what stays on the terminal once the wizard exits: every answer on
// one line each, and how it ended.
func (m *Model) viewDone() []string {
	s := m.steps()
	upto := len(s)
	if !m.write && !m.review {
		upto = m.index(s)
	}
	lines := append(m.intro(), m.history(upto, s)...)
	if m.write {
		for i, pg := range m.plan.Pages {
			if pg.Optional {
				lines = append(lines, sDim.Render(markDone)+"  "+pg.Title+sDim.Render(" · "+m.summary(i)))
			}
		}
		end := "writing " + repoconfig.RelPath
		if m.cfg != nil && m.cfg.IsZero() {
			end = "nothing to pin"
		}
		return append(lines, bar(), sDim.Render(markOutro)+"  "+end)
	}
	return append(lines, sBad.Render(markAbort)+"  "+"aborted — nothing written")
}

// Run opens the wizard and returns the answers, and whether the user chose to
// write. A false second return means abort — the caller writes nothing.
func Run(p *initplan.Plan) (initplan.Answers, bool, error) {
	final, err := tea.NewProgram(New(p)).Run()
	if err != nil {
		return initplan.Answers{}, false, err
	}
	m := final.(*Model)
	if !m.write {
		return initplan.Answers{}, false, nil
	}
	return m.Answers(), true, nil
}
