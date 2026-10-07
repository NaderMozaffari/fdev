package launcher

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/icon"
	"github.com/NaderMozaffari/fdev/internal/sprite"
	"github.com/NaderMozaffari/fdev/internal/version"
)

// The picker asks, one step at a time, what the menu shows: the section
// (Recent, Run, Build, Tools, Saved logs), then the platform and the flavor
// among its targets. Steps with one answer are skipped. → or enter
// goes on, ← or esc goes back, and so do the buttons at the bottom.

type pickStep int

const (
	stepSection pickStep = iota
	stepPlatform
	stepFlavor
)

func (s pickStep) name() string {
	return [...]string{"Section", "Platform", "Flavor"}[s]
}

func (s pickStep) question() string {
	return [...]string{"What do you want to do?", "Which platform?", "Which flavor?"}[s]
}

// steps is the steps with more than one answer, in order.
func (m *Model) steps() []pickStep {
	var out []pickStep
	if len(m.sections()) > 1 {
		out = append(out, stepSection)
	}
	if len(m.platforms()) > 1 {
		out = append(out, stepPlatform)
	}
	if len(m.flavors(m.activePlatform())) > 1 {
		out = append(out, stepFlavor)
	}
	return out
}

// choice is an answer of a step, or of a target's question.
type choice struct {
	value, title, desc, meta string
	sprite                   string      // fdev's icon for it
	icon                     string      // or an app icon (a PNG)
	color                    color.Color // its accent
	facts                    [][2]string // for the card; the step's own when nil
	device                   *deviceChoice
}

// curChoices is what the picker shows: the step's answers, or the
// question's.
func (m *Model) curChoices() []choice {
	if m.asking {
		return m.qChoices
	}
	return m.choices(m.step)
}

func (m *Model) stepValue(s pickStep) string {
	return [...]string{m.tab, m.menuPlatform, m.menuFlavor}[s]
}

func (m *Model) setStepValue(s pickStep, v string) {
	switch s {
	case stepPlatform:
		m.menuPlatform = v
	case stepFlavor:
		m.menuFlavor = v
	case stepSection:
		m.tab = v
	}
	m.fixFilters()
	m.saveMenu()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (m *Model) choices(s pickStep) []choice {
	var out []choice
	switch s {
	case stepPlatform:
		for _, p := range m.platforms() {
			n := 0
			for _, t := range m.sectionTargets(m.tab) {
				if t.PlatformName() == p {
					n++
				}
			}
			out = append(out, choice{value: p, title: p, desc: strings.Join(m.flavors(p), " · "),
				meta: plural(n, "target", "targets"), sprite: sprite.For(p), color: platformColor(p)})
		}
	case stepFlavor:
		platform := m.activePlatform()
		for _, f := range m.flavors(platform) {
			n := 0
			for _, t := range m.sectionTargets(m.tab) {
				if t.Flavor == f && m.matches(t, platform, "") {
					n++
				}
			}
			var desc []string
			for _, key := range []string{"App", "Backend"} {
				if v := m.flavorFact(f, key); v != "" {
					desc = append(desc, v)
				}
			}
			out = append(out, choice{value: f, title: f, desc: strings.Join(desc, " · "),
				meta:   plural(n, "target", "targets") + onPlatform(platform),
				sprite: "flavor", icon: m.cfg.FlavorIcon(f), color: m.flavorColor(f)})
		}
	case stepSection:
		for _, sec := range m.sections() {
			c := choice{value: sec, title: sec, sprite: sprite.For(sec), color: sectionColor(sec)}
			switch sec {
			case recentTab:
				jobs := m.allRecent()
				c.desc = commandLine(jobs[0].target.Name, jobs[0].env) + " · " + ago(jobs[0].at)
				c.meta = plural(len(jobs), "run", "runs")
			case logsTab:
				logs := m.sessions
				starred := 0
				for _, l := range logs {
					if l.Starred {
						starred++
					}
				}
				c.desc = "newest " + sessionWhen(logs[0]) + " · " + logs[0].Target
				c.meta = plural(len(logs), "session", "sessions")
				if starred > 0 {
					c.meta += fmt.Sprintf(" · %d ★", starred)
				}
			default:
				var names []string
				targets := m.sectionTargets(sec)
				for _, t := range targets {
					names = append(names, t.Name)
				}
				c.desc = strings.Join(names, ", ")
				c.meta = plural(len(targets), "target", "targets")
			}
			out = append(out, c)
		}
	}
	return out
}

func onPlatform(p string) string {
	if p == "" {
		return ""
	}
	return " on " + p
}

// ── Moving between steps ─────────────────────────────────────────────────────

// startPicker opens the first step, or the menu when there is nothing to ask.
func (m *Model) startPicker() tea.Cmd {
	m.loadSessions()
	m.fixFilters()
	if steps := m.steps(); len(steps) > 0 {
		return m.openStep(steps[0])
	}
	return m.toMenu()
}

func (m *Model) openStep(s pickStep) tea.Cmd {
	m.screen, m.step, m.pickIndex, m.asking = screenPick, s, 0, false
	for i, c := range m.choices(s) {
		if c.value == m.stepValue(s) {
			m.pickIndex = i
		}
	}
	m.enterStart, m.animStart = time.Now(), time.Now()
	return animTick()
}

// confirm takes the selected answer and goes to the next step, or the menu.
func (m *Model) confirm() tea.Cmd {
	choices := m.curChoices()
	if m.pickIndex >= len(choices) {
		return nil
	}
	if m.asking {
		return m.answerChoice(choices[m.pickIndex])
	}
	m.setStepValue(m.step, choices[m.pickIndex].value)
	for _, s := range m.steps() {
		if s > m.step {
			return m.openStep(s)
		}
	}
	return m.toMenu()
}

// back goes to the step before; quit says what to do on the first.
func (m *Model) back(quit bool) tea.Cmd {
	if m.asking {
		return m.goBack()
	}
	steps := m.steps()
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i] < m.step {
			return m.openStep(steps[i])
		}
	}
	if quit {
		return tea.Quit
	}
	return nil
}

// backFromMenu goes back to the last step; quit is for when there is none.
func (m *Model) backFromMenu(quit bool) tea.Cmd {
	if steps := m.steps(); len(steps) > 0 {
		return m.openStep(steps[len(steps)-1])
	}
	if quit {
		return tea.Quit
	}
	return nil
}

func (m *Model) toMenu() tea.Cmd {
	m.screen = screenMenu
	m.buildMenu()
	m.lastIndex = -1
	return m.selectionMoved()
}

func (m *Model) movePick(step int) tea.Cmd {
	n := len(m.curChoices())
	if n == 0 {
		return nil
	}
	i := min(max(m.pickIndex+step, 0), n-1)
	if i == m.pickIndex {
		return nil
	}
	m.pickIndex, m.animStart = i, time.Now()
	return animTick()
}

func (m *Model) updatePick(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k", "shift+tab":
			return m.movePick(-1)
		case "down", "j", "tab":
			return m.movePick(1)
		case "home", "g":
			return m.movePick(-len(m.curChoices()))
		case "end", "G":
			return m.movePick(len(m.curChoices()))
		case "right", "l", "enter", "space":
			return m.confirm()
		case "left", "h", "backspace":
			return m.back(false)
		case "esc":
			return m.back(true)
		case "q":
			return tea.Quit
		case "s":
			return m.editLook()
		}
		if msg.Text >= "1" && msg.Text <= "9" && len(msg.Text) == 1 {
			if i := int(msg.Text[0] - '1'); i < len(m.curChoices()) {
				m.pickIndex = i
				return m.confirm()
			}
		}
	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			return m.movePick(-1)
		} else if msg.Button == tea.MouseWheelDown {
			return m.movePick(1)
		}
	case tea.MouseMotionMsg:
		m.hover = ""
		if z, ok := m.zoneAt(msg.X, msg.Y); ok {
			m.hover = z.kind + ":" + z.value
			if z.kind == "choice" {
				return m.movePick(z.index - m.pickIndex)
			}
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return nil
		}
		if z, ok := m.zoneAt(msg.X, msg.Y); ok {
			return m.clickZone(z)
		}
	}
	return nil
}

func (m *Model) clickZone(z zone) tea.Cmd {
	switch z.kind {
	case "choice":
		if z.index == m.pickIndex {
			return m.confirm()
		}
		return m.movePick(z.index - m.pickIndex)
	case "step":
		return m.openStep(pickStep(z.index))
	case "ask":
		return m.goToAsk(z.index)
	case "back":
		switch m.screen {
		case screenPick:
			return m.back(false)
		case screenMenu:
			return m.backFromMenu(false)
		}
		return m.goBack()
	case "next":
		if m.screen == screenPick {
			return m.confirm()
		}
		return m.goNext()
	case "quit":
		return tea.Quit
	case "settings":
		return m.editLook()
	case "full":
		return m.toggleFull()
	case "fdev":
		version.Shown = !version.Shown
	}
	return nil
}

// ── Click zones ──────────────────────────────────────────────────────────────

// zone is a clickable part of the last view: [x0, x1) × [y0, y1).
type zone struct {
	kind, value    string
	index          int
	x0, y0, x1, y1 int
}

func (m *Model) addZone(z zone) { m.zones = append(m.zones, z) }

func (m *Model) zoneAt(x, y int) (zone, bool) {
	for _, z := range m.zones {
		if x >= z.x0 && x < z.x1 && y >= z.y0 && y < z.y1 {
			return z, true
		}
	}
	return zone{}, false
}

// ── Animation ────────────────────────────────────────────────────────────────

const (
	staggerBy = 45 * time.Millisecond  // between the rows coming in
	enterFor  = 240 * time.Millisecond // a row coming in
	shineFor  = 650 * time.Millisecond // the shine across the selected icon
)

func ease(t float64) float64 {
	t = min(max(t, 0), 1)
	return 1 - math.Pow(1-t, 3)
}

// rowIn is how far row i has come in, 0 to 1.
func (m *Model) rowIn(i int) float64 {
	return ease(float64(time.Since(m.enterStart)-time.Duration(i)*staggerBy) / float64(enterFor))
}

// shine is where the shine is on the selected icon, 0 to 1 (or past it).
func (m *Model) shine() float64 {
	return float64(time.Since(m.animStart)-80*time.Millisecond) / float64(shineFor)
}

// animating is whether something on the screen still moves.
func (m *Model) animating() bool {
	if m.anim() < 1 || m.shine() < 1 {
		return true
	}
	return m.screen == screenPick && time.Since(m.enterStart) < enterFor+time.Duration(len(m.curChoices()))*staggerBy
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m *Model) pickView() string {
	th := m.th
	m.zones = nil
	lines := []string{m.header()}
	m.addSettingsZone()
	question := m.step.question()
	stepper := m.stepper
	if m.asking {
		question, stepper = m.qTitle, m.jobStepper
	}
	blank := []string{""} // between the parts, unless compact
	if m.compact() {
		blank = nil
	}
	lines = append(lines, blank...)
	lines = append(lines, m.withAbout(stepper(len(lines))))
	lines = append(lines, blank...)
	lines = append(lines, "  "+m.fg(th.Indigo).Bold(true).Render(question)+
		m.fg(th.Muted).Render("   "+m.stepHint()))
	lines = append(lines, blank...)

	choices := m.curChoices()
	panelW := 0
	if m.width >= 96 {
		panelW = min(max(m.width*2/5, 40), 56)
	}
	leftW := m.width - panelW - 2
	if panelW > 0 {
		leftW = min(leftW-2, 64)
	}
	top := len(lines)
	bottom := 3 // a blank row, the buttons and the keys
	if m.full {
		bottom = 0
	}
	avail := max(m.height-top-bottom, 4)
	size := m.sizeFor(func(size string) bool {
		rowH, gap := pickRows(size)
		return len(choices)*(rowH+gap)-gap <= avail
	})
	rowH, gap := pickRows(size)
	per := max((avail+gap)/(rowH+gap), 1)
	first := 0
	if m.pickIndex >= per {
		first = m.pickIndex - per + 1
	}

	var left []string
	for i := first; i < len(choices) && i < first+per; i++ {
		if i > first && gap > 0 {
			left = append(left, "")
		}
		y := top + len(left)
		m.addZone(zone{kind: "choice", value: choices[i].value, index: i, x0: 0, y0: y, x1: leftW + 2, y1: y + rowH})
		left = append(left, m.choiceRow(choices[i], i, i == m.pickIndex, leftW, rowH)...)
	}
	body := strings.Join(left, "\n")
	if panelW > 0 && m.pickIndex < len(choices) {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(leftW+2).Render(body), "  ",
			m.choicePanel(choices[m.pickIndex], panelW, avail))
	}
	lines = append(lines, strings.Split(body, "\n")...)

	// The buttons and keys at the bottom; none in full screen.
	if m.full {
		for len(lines) < m.height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:m.height], "\n")
	}
	for len(lines) < m.height-2 {
		lines = append(lines, "")
	}
	lines = lines[:max(m.height-2, top+1)]
	lines = append(lines, m.buttons(len(lines), m.hasBack(), "next ›"),
		ansi.Truncate("  "+m.fg(th.Muted).Render("← back • → or enter next • ↑↓ choose • 1-9 pick • s settings • t theme • z full screen • q quit"), m.width, "…"))
	return strings.Join(lines, "\n")
}

// withAbout puts fdev's version and build, and the app's version, at the
// right of the picker's first row, when there is room.
func (m *Model) withAbout(line string) string {
	about := version.Line()
	if v := m.cfg.ShortVersion(); v != "" {
		about = m.cfg.Title + " " + v + "  ·  " + about
	}
	about = m.fg(m.th.Muted).Render(about)
	gap := m.width - lipgloss.Width(line) - lipgloss.Width(about) - 2
	if gap < 3 {
		return line
	}
	return line + strings.Repeat(" ", gap) + about
}

// pickRows is the lines of an answer in the picker, and between them, for
// a size.
func pickRows(size string) (rowH, gap int) {
	switch size {
	case config.SizeLarge:
		return 4, 1
	case config.SizeMedium:
		return 2, 1
	}
	return 1, 0
}

func (m *Model) stepHint() string {
	if m.asking {
		return m.qHint
	}
	steps := m.steps()
	for i, s := range steps {
		if s == m.step {
			return fmt.Sprintf("step %d of %d", i+1, len(steps))
		}
	}
	return ""
}

func (m *Model) hasBack() bool {
	if m.asking {
		return true
	}
	for _, s := range m.steps() {
		if s < m.step {
			return true
		}
	}
	return false
}

// stepper shows the steps: done ones with their answer (click to go back
// to them), the current one, and the ones to come.
func (m *Model) stepper(y int) string {
	th := m.th
	line, x := "  ", 2
	for i, s := range m.steps() {
		if i > 0 {
			sep := m.fg(th.Subtle).Render(" ─── ")
			line += sep
			x += lipgloss.Width(sep)
		}
		var text string
		switch {
		case s < m.step:
			v := m.stepValue(s)
			c := platformColor(v)
			switch s {
			case stepSection:
				c = sectionColor(v)
			case stepFlavor:
				c = m.flavorColor(v)
			}
			st := lipgloss.NewStyle().Foreground(th.Cream).Background(c).Padding(0, 1)
			if m.hover == fmt.Sprintf("step:%d", s) {
				st = st.Underline(true)
			}
			text = st.Render("✓ " + v)
			m.addZone(zone{kind: "step", value: fmt.Sprint(s), index: int(s), x0: x, y0: y, x1: x + lipgloss.Width(text), y1: y + 1})
		case s == m.step:
			text = m.fg(th.Pink).Bold(true).Render("● " + s.name())
		default:
			text = m.fg(th.Muted).Render("○ " + s.name())
		}
		line += text
		x += lipgloss.Width(text)
	}
	return line
}

// buttons is the row of ‹ back (or quit) and the next button.
func (m *Model) buttons(y int, back bool, next string) string {
	th := m.th
	backText, backKind := "‹ back", "back"
	if !back {
		backText, backKind = "✕ quit", "quit"
	}
	b := lipgloss.NewStyle().Padding(0, 2).Background(th.BarBg).Foreground(th.Normal)
	if m.hover == backKind+":" {
		b = b.Foreground(th.Pink).Underline(true)
	}
	n := lipgloss.NewStyle().Padding(0, 2).Background(th.Fuchsia).Foreground(th.Cream).Bold(true)
	if m.hover == "next:" {
		n = n.Background(th.Pink).Underline(true)
	}
	bt, nt := b.Render(backText), n.Render(next)
	m.addZone(zone{kind: backKind, x0: 2, y0: y, x1: 2 + lipgloss.Width(bt), y1: y + 1})
	nx := 2 + lipgloss.Width(bt) + 2
	m.addZone(zone{kind: "next", x0: nx, y0: y, x1: nx + lipgloss.Width(nt), y1: y + 1})
	return "  " + bt + "  " + nt
}

// addSettingsZone makes the header's buttons clickable: settings and full
// screen.
func (m *Model) addSettingsZone() {
	m.addZone(zone{kind: "fdev", x0: 1, y0: 0, x1: 1 + lipgloss.Width(m.fdevBadge()), y1: 1})
	c := m.settingsChip()
	m.addZone(zone{kind: "settings", x0: c.x, y0: 0, x1: c.x + c.w, y1: 1})
	f := m.fullChip()
	m.addZone(zone{kind: "full", x0: f.x, y0: 0, x1: f.x + f.w, y1: 1})
}

// choiceRow draws an answer: its icon, title, description and count, on
// the selection's gradient when selected. Rows slide in when a step opens.
func (m *Model) choiceRow(c choice, i int, selected bool, width, height int) []string {
	th := m.th
	in := m.rowIn(i)
	shine := -1.0
	if selected {
		shine = m.shine()
	}
	var icons []string
	iconW := sprite.Width
	switch {
	case height >= 4:
		if c.icon != "" {
			icons = padLines(icon.Render(c.icon, 8, 4), 1, sprite.Width)
		}
		if icons == nil {
			icons = sprite.Render(c.sprite, c.color, 1, shine)
		}
	case height == 1: // a tile with the first letter
		iconW = 3
		tile := lipgloss.NewStyle().Background(c.color).Foreground(th.Cream).Bold(true)
		icons = []string{tile.Render(fit(" "+initial(c.title), 3))}
	default:
		iconW = 4
		if c.icon != "" {
			icons = icon.Render(c.icon, 4, 2)
		}
		if icons == nil {
			tile := lipgloss.NewStyle().Background(c.color).Foreground(th.Cream).Bold(true)
			icons = []string{tile.Render(fit(" "+initial(c.title), 4)), tile.Render("    ")}
		}
	}

	title := lipgloss.NewStyle().Foreground(th.Normal).Bold(true)
	desc := lipgloss.NewStyle().Foreground(th.Muted)
	bar := " "
	if selected {
		title = title.Foreground(th.Pink)
		desc = desc.Foreground(th.PinkDim)
		bar = m.fg(th.Fuchsia).Render("▌")
	}
	textW := max(width-iconW-3, 10)
	reveal := 1.0
	if selected {
		reveal = m.anim()
	}
	num := m.fg(th.Subtle).Render(fmt.Sprintf("%d", i+1))
	if selected {
		num = m.fg(c.color).Bold(true).Render("→")
	}
	titleSpans := []span{{"  ", title}, {c.title, title}}
	var texts []string
	switch {
	case height == 1: // title, count and description on one line
		texts = []string{m.paint(append(titleSpans, span{"  " + c.meta, lipgloss.NewStyle().Foreground(c.color)},
			span{"  " + c.desc, desc}), textW, selected, reveal)}
	case height >= 4:
		texts = []string{
			m.paint(titleSpans, textW, selected, reveal),
			m.paint([]span{{"  ", desc}, {c.desc, desc}}, textW, selected, reveal),
			m.paint([]span{{"  ", desc}, {c.meta, lipgloss.NewStyle().Foreground(c.color)}}, textW, selected, reveal),
			m.paint(nil, textW, selected, reveal),
		}
	default:
		texts = []string{
			m.paint(append(titleSpans, span{"  " + c.meta, lipgloss.NewStyle().Foreground(c.color)}), textW, selected, reveal),
			m.paint([]span{{"  ", desc}, {c.desc, desc}}, textW, selected, reveal),
		}
	}
	out := make([]string, height)
	for j := range out {
		n := " "
		if j == 0 {
			n = num
		}
		ic := strings.Repeat(" ", iconW)
		if j < len(icons) {
			ic = icons[j]
		}
		out[j] = bar + n + ic + texts[j]
	}
	if in < 1 { // coming in from the right, faint
		shift := int((1 - in) * 8)
		for j := range out {
			line := strings.Repeat(" ", shift) + out[j]
			if in < 0.4 {
				line = m.fg(th.Subtle).Render(ansi.Strip(line))
			}
			out[j] = ansi.Truncate(line, width+2, "")
		}
	}
	return out
}

// initial is the first letter of s, for a tile.
func initial(s string) string {
	for _, r := range s {
		return string(r)
	}
	return " "
}

func padLines(lines []string, left, width int) []string {
	if lines == nil {
		return nil
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		l = strings.Repeat(" ", left) + l
		out[i] = l + strings.Repeat(" ", max(width-lipgloss.Width(l), 0))
	}
	return out
}

// choicePanel is the card beside the answers: the selected one, big.
func (m *Model) choicePanel(c choice, width, height int) string {
	th := m.th
	inner := width - 4
	var art []string
	if c.icon != "" {
		w := min(24, inner)
		art = icon.Render(c.icon, w, w/2)
	}
	if art == nil {
		art = sprite.Render(c.sprite, c.color, 2, m.shine())
	}
	head := []string{
		lipgloss.NewStyle().Foreground(c.color).Bold(true).Render(c.title),
		m.fg(th.Muted).Render(c.meta),
	}
	facts := c.facts
	switch step := m.step; {
	case m.asking:
	case step == stepPlatform:
		facts = append(facts, [2]string{"Flavors", strings.Join(m.flavors(c.value), ", ")})
		for _, t := range m.sectionTargets(m.tab) {
			if t.PlatformName() == c.value {
				facts = append(facts, [2]string{t.Name, t.Desc})
			}
		}
	case step == stepFlavor:
		if f := m.cfg.Flavors[c.value]; f != nil {
			for _, fact := range f.Info {
				facts = append(facts, [2]string{fact.Key, fact.Value})
			}
		}
	case step == stepSection:
		switch c.value {
		case recentTab:
			for _, j := range m.allRecent() {
				facts = append(facts, [2]string{ago(j.at), commandLine(j.target.Name, j.env)})
			}
		case logsTab:
			for i, s := range m.sessions {
				if i == 8 {
					break
				}
				star := " "
				if s.Starred {
					star = "★"
				}
				facts = append(facts, [2]string{star + " " + sessionWhen(s), s.Target})
			}
		default:
			for _, t := range m.sectionTargets(c.value) {
				facts = append(facts, [2]string{t.Name, t.Desc})
			}
		}
	}
	keyW := 0
	for _, f := range facts {
		keyW = min(max(keyW, lipgloss.Width(f[0])), inner/2)
	}
	var rows []string
	for _, f := range facts {
		value := strings.Split(ansi.Wrap(f[1], max(inner-keyW-2, 8), ""), "\n")
		for i, v := range value {
			k := ""
			if i == 0 {
				k = ansi.Truncate(f[0], keyW, "…")
			}
			rows = append(rows, m.fg(th.Muted).Render(fmt.Sprintf("%-*s", keyW, k))+"  "+v)
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.PlaceHorizontal(inner, lipgloss.Center, strings.Join(art, "\n")),
		"",
		lipgloss.PlaceHorizontal(inner, lipgloss.Center, strings.Join(head, "\n")),
	)
	if len(rows) > 0 {
		body += "\n\n" + m.fg(th.Subtle).Render(strings.Repeat("─", inner)) + "\n" + strings.Join(rows, "\n")
	}
	border := lipgloss.Blend1D(10, c.color, th.Subtle)[9-min(int(m.anim()*9), 9)]
	if m.anim() >= 1 {
		border = c.color
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).Width(width).MaxHeight(height).Render(body)
}
