package logview

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Opening a log: the mouse over a log that has more to show (headers and a
// body, a long stack, rows left out) brings up a button at the right end of
// its first row on the screen, and a click on it opens or closes the log:
//
//	12:04:51 RESP   GET ← getProfile  /v1/me  status=200       ▸ more
//
// Only the button does: a click elsewhere on the log leaves it as it is.

// expanderW is the button's width; the columns layouts keep it free.
const expanderW = 8

// expanderText is the button for e, hot while the mouse is on it.
func (m *Model) expanderText(e *Entry) string {
	label := expandGlyph + " more"
	if e.Expanded {
		label = "▾ less"
	}
	s := lipgloss.NewStyle().Padding(0, 1).Background(m.th.BarBg).Foreground(m.th.Pink).Bold(true)
	if m.hover == "expand" {
		s = s.Background(m.th.Fuchsia).Foreground(m.th.Cream)
	}
	return s.Render(label)
}

// expanderY is the screen row of e's button: its first row on the screen.
func (m *Model) expanderY(e *Entry) (int, bool) {
	if e == nil || m.overlay() || m.sel != nil || e.hidden || !expandable(e, m.on["details"]) {
		return 0, false
	}
	m.fitHeight()
	m.refresh()
	top, h := m.vp.YOffset(), m.vp.Height()
	for i := top; i < min(top+h, len(m.rows)); i++ {
		if m.rows[i].entry == e {
			return m.logTop() + i - top, true
		}
	}
	return 0, false
}

// onExpander is whether x, y is on e's button.
func (m *Model) onExpander(e *Entry, x, y int) bool {
	y0, ok := m.expanderY(e)
	return ok && y == y0 && x >= m.width-expanderW-1 && x < m.width-1
}

// hoverLog notes the log under the mouse, whose button shows.
func (m *Model) hoverLog(x, y int) {
	m.hoverEntry = nil
	if r, ok := m.rowAt(x, y); ok && r.entry.Level != "cleared" {
		m.hoverEntry = r.entry
	}
}

// withExpander draws the button of the log under the mouse over the screen.
func (m *Model) withExpander(screen string) string {
	y, ok := m.expanderY(m.hoverEntry)
	if !ok {
		return screen
	}
	lines := strings.Split(screen, "\n")
	if y >= len(lines) {
		return screen
	}
	x := m.width - expanderW - 1
	l := lines[y]
	left := ansi.Truncate(l, x, "")
	left += strings.Repeat(" ", max(x-lipgloss.Width(left), 0))
	lines[y] = left + "\x1b[m" + m.expanderText(m.hoverEntry) + "\x1b[m" + ansi.TruncateLeft(l, x+expanderW, "")
	return strings.Join(lines, "\n")
}
