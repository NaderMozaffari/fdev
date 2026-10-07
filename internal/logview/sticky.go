package logview

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Pinned and tracked logs stay at the top of the logs, a row each, however
// far the logs scroll:
//
//	⚑ 12:01:03 INFO  Billing: connected
//	◉ 12:04:51 RESP  ← getProfile /v1/me 200 142ms                 ×7  ✕
//	╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
//
// A pinned log is that one log. A tracked log is the logs like it (the
// same call, or the same message whatever its numbers): the row shows the
// newest of them and how many came, and lights up when one comes. A click
// on a row shows the log; on its ✕, unpins or stops tracking it.

// track follows the logs like one.
type track struct {
	key   string
	count int
	last  *Entry
	at    time.Time // when the newest came, for the row lighting up
}

var digits = regexp.MustCompile(`\d+`)

// trackKey is what logs like e share.
func trackKey(e *Entry) string {
	switch {
	case e.Category == "network":
		return "net " + e.method() + " " + e.path()
	case e.Kind == KindTool:
		return "tool " + digits.ReplaceAllString(stripANSI(e.Text), "#")
	}
	text := []rune(digits.ReplaceAllString(e.Text, "#"))
	if len(text) > 48 {
		text = text[:48]
	}
	return "log " + e.Level + " " + e.Tag + " " + string(text)
}

func (m *Model) trackOf(e *Entry) *track {
	if len(m.tracks) == 0 {
		return nil
	}
	k := trackKey(e)
	for _, t := range m.tracks {
		if t.key == k {
			return t
		}
	}
	return nil
}

func (m *Model) toggleTrack(e *Entry) {
	if t := m.trackOf(e); t != nil {
		m.untrack(t)
		m.setStatus("◎ stopped tracking it")
		return
	}
	t := &track{key: trackKey(e)}
	for _, x := range m.entries {
		if trackKey(x) == t.key {
			t.count++
			t.last, t.at = x, x.Time
		}
	}
	m.tracks = append(m.tracks, t)
	m.version++ // the ◉ on every log like it
	m.resize()
	m.setStatus(fmt.Sprintf("◉ tracking %s like it · it stays at the top", plural(t.count, "log")))
}

func (m *Model) untrack(t *track) {
	for i, x := range m.tracks {
		if x == t {
			m.tracks = append(m.tracks[:i], m.tracks[i+1:]...)
			break
		}
	}
	m.version++
	m.resize()
}

// tracked counts a new log for the track it is like.
func (m *Model) tracked(e *Entry) {
	if t := m.trackOf(e); t != nil {
		t.count++
		t.last, t.at = e, time.Now()
	}
}

func (m *Model) togglePin(e *Entry) {
	e.pinned, e.cached = !e.pinned, cacheKey{}
	if e.pinned {
		m.pins = append(m.pins, e)
		m.setStatus("⚑ pinned · it stays at the top; click its ✕ to unpin")
	} else {
		m.unpin(e)
		m.setStatus("⚐ unpinned")
	}
	m.resize()
}

func (m *Model) unpin(e *Entry) {
	e.pinned, e.cached = false, cacheKey{}
	for i, x := range m.pins {
		if x == e {
			m.pins = append(m.pins[:i], m.pins[i+1:]...)
			break
		}
	}
	m.dirty = true
}

// stickyItem is a row of the panel: a pin, or a track.
type stickyItem struct {
	pin   *Entry
	track *track
}

func (m *Model) stickyItems() []stickyItem {
	var out []stickyItem
	for _, e := range m.pins {
		out = append(out, stickyItem{pin: e})
	}
	for _, t := range m.tracks {
		out = append(out, stickyItem{track: t})
	}
	return out
}

// maxSticky is how many rows the panel takes at most.
func (m *Model) maxSticky() int { return max((m.height-8)/4, 1) }

// stickyRows is the panel's height: its rows and the line under them.
func (m *Model) stickyRows() int {
	if m.overlay() || m.on["raw"] {
		return 0
	}
	n := len(m.pins) + len(m.tracks)
	if n == 0 {
		return 0
	}
	return min(n, m.maxSticky()) + 1
}

// overlay is whether something covers the logs: the settings, saving, a
// log in a window, the values.
func (m *Model) overlay() bool {
	return m.lookForm != nil || m.saveForm != nil || m.dialog != nil || m.vault != nil
}

func (m *Model) stickyView() []string {
	items := m.stickyItems()
	rows := m.stickyRows() - 1
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	closeBtn := muted.Render(" ✕ ")
	var out []string
	for i, it := range items {
		if i == rows {
			break
		}
		var icon, tail string
		var e *Entry
		hover := m.hover == fmt.Sprintf("sticky:%d", i)
		if it.pin != nil {
			e = it.pin
			icon = lipgloss.NewStyle().Foreground(m.th.Pink).Render("⚑")
		} else {
			e = it.track.last
			c := m.th.Info
			if time.Since(it.track.at) < highlightOn {
				c = m.th.Pink
			}
			icon = lipgloss.NewStyle().Foreground(c).Bold(true).Render("◉")
			tail = muted.Render(fmt.Sprintf(" ×%d", it.track.count))
		}
		if i == rows-1 && len(items) > rows {
			tail += muted.Render(fmt.Sprintf("  +%d more", len(items)-rows))
		}
		avail := m.width - 2 - lipgloss.Width(tail) - lipgloss.Width(closeBtn)
		text := ""
		if e != nil {
			text = m.summary(e, avail)
		}
		if hover {
			text = lipgloss.NewStyle().Underline(true).Render(ansi.Strip(text))
		}
		text = fit(text, avail)
		line := icon + " " + text + tail + closeBtn
		if hover {
			line = paintBg(line, m.width, m.th.BarBg)
		}
		out = append(out, line)
	}
	rule := lipgloss.NewStyle().Foreground(m.th.Subtle).Render(strings.Repeat("╌", m.width))
	return append(out, rule)
}

// stickyAt is the panel's row under the mouse, and whether it is on its ✕.
func (m *Model) stickyAt(x, y int) (int, bool, bool) {
	i := y - 1
	if i < 0 || i >= m.stickyRows()-1 {
		return 0, false, false
	}
	return i, x >= m.width-3, true
}

func (m *Model) clickSticky(x, y int) tea.Cmd {
	i, onClose, ok := m.stickyAt(x, y)
	items := m.stickyItems()
	if !ok || i >= len(items) {
		return nil
	}
	it := items[i]
	switch {
	case onClose && it.pin != nil:
		m.unpin(it.pin)
		m.resize()
	case onClose:
		m.untrack(it.track)
	case it.pin != nil:
		return m.jumpTo(it.pin)
	case it.track.last != nil:
		return m.jumpTo(it.track.last)
	}
	return nil
}

// summary is a log on one row: its time, level and message; a network
// log's call, status and time.
func (m *Model) summary(e *Entry, width int) string {
	p := painter{m.th}
	muted := p.fg(m.th.Muted)
	s := muted.Render(e.Time.Format("15:04:05")) + " "
	if e.Kind != KindTool {
		name, col := label(m.th, e.Level)
		s += lipgloss.NewStyle().Bold(true).Foreground(col).Render(fit(name, labelWidth)) + " "
	}
	switch {
	case e.Category == "network":
		s += lipgloss.NewStyle().Bold(true).Render(e.method()) + " " + m.networkTitle(e)
		for _, f := range e.Fields {
			if f.Key == "status" || f.Key == "took" || f.Key == "error" {
				s += " " + p.tone(f.Tone).Render(f.Value)
			}
		}
	case e.Tag != "":
		s += lipgloss.NewStyle().Bold(true).Render(e.Tag+":") + " " + p.tone(e.TextTone).Render(e.Text)
	default:
		s += p.tone(e.TextTone).Render(e.Text)
	}
	return ansi.Truncate(strings.ReplaceAll(s, "\n", " "), width, "…")
}
