package launcher

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
)

// The launcher's small looks (config.UI.Launcher), for small windows:
// line puts the answers so far on one line above the choices, list puts
// each question on a rail, like npm create's. Both have one line of keys.
//
//	line:  fdev  Run › Android › Which flavor?
//	        ❯ dev     last used
//	          staging
//
//	list:  ┌  fdev  Acme Shop v2.4.0
//	       ◇  What do you want to do?
//	       │  Run
//	       ◆  Which flavor?
//	       │  ● dev
//	       │  ○ staging
//	       └  ↑↓ choose · enter next · ← back

// mode is the launcher's look: the one set, or for auto full when the
// window has room for it.
func (m *Model) mode() string {
	switch m.ui.Launcher {
	case config.LauncherLine, config.LauncherList, config.LauncherFull:
		return m.ui.Launcher
	}
	if m.width >= 80 && m.height >= 24 {
		return config.LauncherFull
	}
	return config.LauncherLine
}

// small is whether one line or list was picked (not auto): the other
// screens (the menu, forms) then take as little room as they can too.
func (m *Model) small() bool {
	return m.ui.Launcher == config.LauncherLine || m.ui.Launcher == config.LauncherList
}

// answered is a question already answered, and its answer.
type answered struct{ question, answer string }

// answers is what was picked so far: the steps before this one, then, once
// a target asks its questions, the target and its answers.
func (m *Model) answers() []answered {
	var out []answered
	for _, s := range m.steps() {
		if m.asking || s < m.step {
			out = append(out, answered{s.question(), m.stepValue(s)})
		}
	}
	if m.asking && m.job != nil {
		out = append(out, answered{"Which target?", m.job.target.Name})
		for _, name := range m.job.asked {
			if !m.job.auto[name] {
				out = append(out, answered{m.askTitle(name), m.answerLabel(name)})
			}
		}
	}
	return out
}

// question is the question being asked, and its hint (step 2 of 3).
func (m *Model) question() (string, string) {
	if m.asking {
		return m.qTitle, m.qHint
	}
	return m.step.question(), m.stepHint()
}

// window is which choices fit in rows: the first and one past the last,
// keeping the selected one in view.
func (m *Model) window(n, rows int) (int, int) {
	rows = max(rows, 1)
	first := 0
	if m.pickIndex >= rows {
		first = m.pickIndex - rows + 1
	}
	return first, min(first+rows, n)
}

// keysLine is the one line of keys at the bottom; none in full screen.
func (m *Model) keysLine() string {
	keys := "↑↓ choose · enter next · ← back · s settings · q quit"
	return m.fg(m.th.Muted).Render(keys)
}

func (m *Model) pickLine() string {
	th := m.th
	m.zones = nil
	question, _ := m.question()
	head := " " + m.fdevBadge() + " "
	m.addZone(zone{kind: "fdev", x0: 1, y0: 0, x1: 1 + lipgloss.Width(m.fdevBadge()), y1: 1})
	var trail []string
	for _, a := range m.answers() {
		trail = append(trail, a.answer)
	}
	q := m.fg(th.Indigo).Bold(true).Render(question)
	room := m.width - lipgloss.Width(head) - lipgloss.Width(q)
	path := ""
	for i := range trail {
		p := strings.Join(trail[i:], " › ") + " › "
		if i > 0 {
			p = "… " + p
		}
		if lipgloss.Width(p) <= room || i == len(trail)-1 {
			path = p
			break
		}
	}
	lines := []string{ansi.Truncate(head+m.fg(th.Muted).Render(path)+q, m.width, "…")}

	choices := m.curChoices()
	bottom := 1
	if m.full {
		bottom = 0
	}
	first, last := m.window(len(choices), m.height-len(lines)-bottom)
	for i := first; i < last; i++ {
		c := choices[i]
		pointer, title := "   ", c.title
		style := lipgloss.NewStyle()
		if i == m.pickIndex {
			pointer = m.fg(th.Pink).Render(" ❯ ")
			style = style.Foreground(th.Pink).Bold(true)
		}
		if m.hover == fmt.Sprintf("choice:%s", c.value) {
			style = style.Underline(true)
		}
		desc := c.desc
		if desc == "" {
			desc = c.meta
		}
		y := len(lines)
		m.addZone(zone{kind: "choice", value: c.value, index: i, x0: 0, y0: y, x1: m.width, y1: y + 1})
		lines = append(lines, ansi.Truncate(pointer+style.Render(title)+m.fg(th.Muted).Render("  "+desc), m.width, "…"))
	}
	return m.withKeys(lines)
}

func (m *Model) pickList() string {
	th := m.th
	m.zones = nil
	rail := m.fg(th.Subtle).Render("│")
	about := m.cfg.Title
	if v := m.cfg.ShortVersion(); v != "" {
		about += " " + v
	}
	head := []string{m.fg(th.Subtle).Render("┌") + " " + m.fdevBadge() + " " + m.fg(th.Muted).Render(about), rail}
	m.addZone(zone{kind: "fdev", x0: 2, y0: 0, x1: 2 + lipgloss.Width(m.fdevBadge()), y1: 1})

	var history []string
	for _, a := range m.answers() {
		history = append(history, m.fg(th.Green).Render("◇")+"  "+a.question, rail+"  "+m.fg(th.Muted).Render(a.answer), rail)
	}
	question, hint := m.question()
	asking := m.fg(th.Pink).Render("◆") + "  " + m.fg(th.Normal).Bold(true).Render(question) + m.fg(th.Muted).Render("  "+hint)

	// The choices get at least five rows (or all of them); the oldest
	// answers make room.
	choices := m.curChoices()
	bottom := 1 // └ and the keys
	free := m.height - len(head) - 1 - bottom
	want := min(len(choices), 5)
	if drop := len(history) - max(free-want, 0); drop > 0 {
		history = history[min(drop, len(history)):]
	}
	lines := append(append(head, history...), asking)
	first, last := m.window(len(choices), m.height-len(lines)-bottom)
	for i := first; i < last; i++ {
		c := choices[i]
		mark, title := m.fg(th.Muted).Render("○"), m.fg(th.Muted).Render(c.title)
		if i == m.pickIndex {
			mark, title = m.fg(th.Green).Render("●"), m.fg(th.Normal).Bold(true).Render(c.title)
			if d := c.desc; d != "" {
				title += m.fg(th.Muted).Render("  " + d)
			}
		}
		if m.hover == fmt.Sprintf("choice:%s", c.value) {
			title = lipgloss.NewStyle().Underline(true).Render(title)
		}
		y := len(lines)
		m.addZone(zone{kind: "choice", value: c.value, index: i, x0: 0, y0: y, x1: m.width, y1: y + 1})
		lines = append(lines, ansi.Truncate(m.fg(th.Pink).Render("│")+"  "+mark+" "+title, m.width, "…"))
	}
	if m.full {
		return m.withKeys(lines)
	}
	for len(lines) < m.height-1 {
		lines = append(lines, "")
	}
	end := m.fg(th.Pink).Render("└") + "  " + m.keysLine()
	return strings.Join(append(lines[:m.height-1], ansi.Truncate(end, m.width, "…")), "\n")
}

// withKeys fills the screen and puts the keys on its last line.
func (m *Model) withKeys(lines []string) string {
	h := m.height
	if !m.full {
		h--
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	lines = lines[:max(h, 0)]
	if !m.full {
		lines = append(lines, ansi.Truncate(" "+m.keysLine(), m.width, "…"))
	}
	return strings.Join(lines, "\n")
}
