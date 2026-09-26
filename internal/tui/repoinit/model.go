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

// Review page options.
const (
	actionWrite = iota
	actionBack
	actionAbort
)

// Model is the wizard. Its values are the answers themselves, so walking back
// to a prompt opens it on what was already chosen.
type Model struct {
	plan   *initplan.Plan
	sel    map[string]string
	multi  map[string][]string
	custom map[string]string
	cursor map[string]int // a multi-select's row cursor

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
		custom: map[string]string{}, cursor: map[string]int{}, width: 80, height: 24}
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
		for j := range m.plan.Pages[i].Questions {
			q := &m.plan.Pages[i].Questions[j]
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
			if i == 0 {
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
	switch msg.String() {
	case "up", "k":
		m.action = max(m.action-1, actionWrite)
	case "down", "j":
		m.action = min(m.action+1, actionAbort)
	case "pgdown", "ctrl+d", " ":
		m.scroll += m.previewRows()
	case "pgup", "ctrl+u":
		m.scroll -= m.previewRows()
	case "esc", "shift+tab":
		return m, m.back()
	case "enter":
		switch m.action {
		case actionWrite:
			if m.cfgErr != nil {
				m.err = m.cfgErr.Error()
				return m, nil
			}
			return m.finish(true)
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
// just given may have shown or hidden what follows.
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

func (m *Model) back() tea.Cmd {
	m.err = ""
	s := m.steps()
	if m.review {
		m.review = false
		m.at = s[len(s)-1].id()
		return m.enterStep(s[len(s)-1])
	}
	i := m.index(s)
	if i == 0 {
		return nil
	}
	m.at = s[i-1].id()
	return m.enterStep(s[i-1])
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
	m.review, m.action, m.scroll, m.err = true, actionWrite, 0, ""
	m.input.Blur()
	m.cfg, m.cfgErr = initplan.Apply(m.plan, m.Answers())
}

// ---- rendering ----

func (m *Model) inner() int { return max(m.width-3, 20) }

func bar() string { return sDim.Render(markBar) }

// row is one line under the rule.
func row(text string) string { return bar() + "  " + text }

// wrap breaks plain text to the column width, one styled row per line.
func (m *Model) wrap(text string, st lipgloss.Style) []string {
	if text == "" {
		return nil
	}
	w := lipgloss.NewStyle().Width(m.inner()).Render(text)
	var out []string
	for _, l := range strings.Split(w, "\n") {
		out = append(out, row(st.Render(strings.TrimRight(l, " "))))
	}
	return out
}

func (m *Model) progress(page int) string {
	return fmt.Sprintf("%d/%d · %s", page+1, len(m.plan.Pages), m.plan.Pages[page].Title)
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
	return []string{sDim.Render(markIntro) + "  " + sTitle.Render("mkit init"), bar()}
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
	lines := append(m.intro(), m.history(i, all)...)
	q := s.q
	title := q.Title
	if s.custom {
		title = q.CustomTitle
	}
	lines = append(lines, sAccent.Render(markActive)+"  "+sTitle.Render(title)+
		sDim.Render("  "+m.progress(s.page)))

	var help string
	switch {
	case s.custom:
		lines = append(lines, row(m.input.View()))
		help = "enter confirm"
	case q.Kind == initplan.Multi:
		lines = append(lines, m.wrap(q.Description, sDim)...)
		lines = append(lines, m.wrap(noteText(q), sWarn)...)
		lines = append(lines, m.viewMulti(q)...)
		help = "↑/↓ move · space toggle · enter confirm"
	default:
		lines = append(lines, m.wrap(q.Description, sDim)...)
		lines = append(lines, m.wrap(noteText(q), sWarn)...)
		lines = append(lines, m.viewSelect(q)...)
		help = "↑/↓ choose · enter confirm"
	}
	if m.err != "" {
		lines = append(lines, m.wrap(markError+" "+m.err, sBad)...)
	}
	if i == 0 {
		help += " · esc abort"
	} else {
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

// window is the slice of rows shown for a list of n with the cursor at c. It
// starts at the top and moves only once the cursor would leave it, so a
// pre-selection near the end never hides the options above it that fit.
func (m *Model) window(n, c int) (int, int) {
	rows := max(4, m.height-12)
	if n <= rows {
		return 0, n
	}
	from := 0
	if c >= rows {
		from = c - rows + 1
	}
	return from, from + rows
}

func (m *Model) viewSelect(q *initplan.Question) []string {
	c := m.selIndex(q)
	from, to := m.window(len(q.Options), c)
	var lines []string
	if from > 0 {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("↑ %d more", from))))
	}
	for i := from; i < to; i++ {
		o := q.Options[i]
		if i == c {
			lines = append(lines, row(sAccent.Render(markOn)+" "+label(o)))
		} else {
			lines = append(lines, row(sDim.Render(markOff+" "+o.Label)))
		}
	}
	if to < len(q.Options) {
		lines = append(lines, row(sDim.Render(fmt.Sprintf("↓ %d more", len(q.Options)-to))))
	}
	if d := q.Options[c].Description; d != "" {
		lines = append(lines, m.wrap("↳ "+d, sDim)...)
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
	from, to := m.window(len(q.Options), c)
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
			lines = append(lines, m.wrap("↳ "+d, sDim)...)
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
func (m *Model) previewRows() int { return max(3, m.height-10) }

func (m *Model) viewReview() []string {
	lines := m.intro()
	lines = append(lines, sAccent.Render(markActive)+"  "+sTitle.Render("Review")+
		sDim.Render("  "+repoconfig.RelPath))
	if m.cfgErr != nil {
		lines = append(lines, m.wrap(markError+" "+m.cfgErr.Error(), sBad)...)
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
	write := "Write the file"
	if m.cfgErr == nil && m.cfg.IsZero() {
		write = "Finish without writing"
	}
	for i, o := range []string{write, "Back — change an answer", "Abort — write nothing"} {
		if i == m.action {
			lines = append(lines, row(sAccent.Render(markOn)+" "+sFg.Render(o)))
		} else {
			lines = append(lines, row(sDim.Render(markOff+" "+o)))
		}
	}
	if m.err != "" {
		lines = append(lines, m.wrap(markError+" "+m.err, sBad)...)
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
		end := "writing " + repoconfig.RelPath
		if m.cfg != nil && m.cfg.IsZero() {
			end = "nothing to pin"
		}
		return append(lines, sDim.Render(markOutro)+"  "+end)
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
