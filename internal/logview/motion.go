package logview

import (
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Motion in the viewer: smooth scrolling that speeds up while the wheel
// keeps turning, details that unfold (and fold) line by line with a bar
// beside them that fades, and clearing the screen without losing a line
// of the saved log.

type motionMsg struct{}

func motionTick() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return motionMsg{} })
}

const (
	wheelLines  = 4                      // a wheel notch, before speeding up
	wheelBoost  = 90 * time.Millisecond  // notches closer than this speed up
	unfoldFor   = 260 * time.Millisecond // details opening or closing
	highlightOn = 1500 * time.Millisecond
)

// unfold is an entry opening or closing.
type unfold struct {
	entry   *Entry
	start   time.Time
	opening bool
}

// wheel scrolls a notch of the wheel: dir is -1 up, 1 down.
func (m *Model) wheel(dir int) tea.Cmd {
	now := time.Now()
	if now.Sub(m.lastWheel) < wheelBoost && dir == m.wheelDir {
		m.boost = math.Min(m.boost+0.6, 5)
	} else {
		m.boost = 1
	}
	if dir != m.wheelDir {
		m.scrollLeft = 0 // turned the other way
	}
	m.lastWheel, m.wheelDir = now, dir
	m.scrollLeft += dir * int(math.Round(wheelLines*m.boost))
	limit := max(m.vp.Height()*3, 30)
	m.scrollLeft = min(max(m.scrollLeft, -limit), limit)
	return m.startMotion()
}

// glide scrolls n lines (negative up) smoothly: pages, and showing what
// opened.
func (m *Model) glide(n int) tea.Cmd {
	m.scrollLeft += n
	return m.startMotion()
}

func (m *Model) startMotion() tea.Cmd {
	if m.moving {
		return nil
	}
	m.moving = true
	return motionTick()
}

// motion moves everything one frame on.
func (m *Model) motion() tea.Cmd {
	if m.scrollLeft != 0 {
		step := int(math.Ceil(math.Abs(float64(m.scrollLeft)) * 0.3))
		before := m.vp.YOffset()
		if m.scrollLeft > 0 {
			m.scrollDown(step)
			m.scrollLeft = max(m.scrollLeft-step, 0)
		} else {
			m.scrollUp(step)
			m.scrollLeft = min(m.scrollLeft+step, 0)
		}
		if m.vp.YOffset() == before { // at the top or the bottom
			m.scrollLeft = 0
		}
	}
	if u := m.unfolding; u != nil {
		m.dirty = true
		if time.Since(u.start) >= unfoldFor {
			m.unfolding = nil
			if u.opening {
				m.showOpened(u.entry)
			}
		}
	}
	if m.lit != nil {
		m.dirty = true
		if time.Since(m.litAt) >= highlightOn {
			m.lit = nil
		}
	}
	if m.scrollLeft != 0 || m.unfolding != nil || m.lit != nil {
		return motionTick()
	}
	m.moving = false
	return nil
}

// toggleExpanded opens or closes an entry's details, unfolding them.
func (m *Model) toggleExpanded(e *Entry) tea.Cmd {
	e.Expanded = !e.Expanded
	m.unfolding = &unfold{entry: e, start: time.Now(), opening: e.Expanded}
	if e.Expanded {
		m.lit, m.litAt = e, time.Now()
	} else if m.lit == e {
		m.lit = nil
	}
	m.dirty = true
	return m.startMotion()
}

// showOpened scrolls, smoothly, so that what opened is on the screen (its
// first line staying on it).
func (m *Model) showOpened(e *Entry) {
	m.refresh()
	start, end := -1, -1
	for i, r := range m.rows {
		if r.entry == e {
			if start < 0 {
				start = i
			}
			end = i + 1
		}
	}
	top, h := m.vp.YOffset(), m.vp.Height()
	if start < 0 || end <= top+h {
		return
	}
	m.glide(min(end-(top+h), start-top))
}

// unfoldRows is what an unfolding entry shows now: its open rows, cut
// to how far it has come.
func (m *Model) unfoldRows(e *Entry, width int) []row {
	u := m.unfolding
	t := min(float64(time.Since(u.start))/float64(unfoldFor), 1)
	eased := 1 - math.Pow(1-t, 3)
	closed := len(m.rowsAs(e, false, width))
	open := m.rowsAs(e, true, width)
	n := closed + int(math.Round(float64(len(open)-closed)*eased))
	if !u.opening {
		n = len(open) - int(math.Round(float64(len(open)-closed)*eased))
	}
	if !u.opening && t >= 1 {
		return m.rowsAs(e, false, width)
	}
	return open[:min(max(n, 1), len(open))]
}

// rowsAs is an entry's rows, open or not.
func (m *Model) rowsAs(e *Entry, expanded bool, width int) []row {
	was := e.Expanded
	e.Expanded = expanded
	rows := m.entryRows(e, width)
	e.Expanded = was
	return rows
}

// litRows marks the rows of what just opened with a bar that fades.
func (m *Model) litRows(rows []row) []row {
	t := float64(time.Since(m.litAt)) / float64(highlightOn)
	c := lipgloss.Blend1D(10, m.th.Fuchsia, m.th.Subtle)[min(int(t*10), 9)]
	bar := lipgloss.NewStyle().Foreground(c).Render("▌")
	out := make([]row, len(rows))
	for i, r := range rows {
		r.text = bar + ansi.TruncateLeft(r.text, 1, "")
		out[i] = r
	}
	return out
}

// ── Clearing ─────────────────────────────────────────────────────────────────

// clearScreen empties the screen, not the log: everything so far is saved
// first (saving starts if it was off), the session's file goes on, and a
// line in it marks where the screen was cleared.
func (m *Model) clearScreen(why string) {
	if m.o.Replay == "" {
		if m.raw == nil {
			if err := m.startRecording(); err != nil {
				m.setStatus("✘ not cleared, can't save logs: " + err.Error())
				return
			}
		}
	}
	for _, e := range m.entries {
		e.cleared = true
	}
	now := time.Now()
	sep := &Entry{Kind: KindTool, Level: "cleared", Time: now,
		Text: "──── " + why + " at " + now.Format("15:04:05") + " · what came before is in the saved log ────"}
	sep.Raw = []string{"──── fdev: " + why + " at " + now.Format("15:04:05") + " ────"}
	m.entries = append(m.entries, sep)
	if m.raw != nil {
		for _, r := range sep.Raw {
			_, _ = m.raw.WriteString(rawLine(now, r) + "\n")
		}
	}
	m.counts, m.errors, m.unseen = map[string]int{}, 0, 0
	m.unfolding, m.lit = nil, nil
	if m.o.Replay == "" {
		if path, err := m.save(); err == nil {
			m.setStatus("✓ cleared · everything so far is in " + path)
		}
	}
	m.changed()
}

// feedClears splits output on the screen clears in it (flutter's `c`):
// what comes before each one is still logged.
func splitClears(text string) []string {
	var parts []string
	for {
		i, n := -1, 0
		for _, clear := range []string{"\x1b[2J", "\x1bc"} {
			if j := strings.Index(text, clear); j >= 0 && (i < 0 || j < i) {
				i, n = j, len(clear)
			}
		}
		if i < 0 {
			return append(parts, text)
		}
		parts = append(parts, text[:i])
		text = text[i+n:]
	}
}
