package logview

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// The values window (k, or the 🔑 chip): every value kept so far (see
// values.go), the newest of each name, to see and copy at any time.
//
//	╭─ 🔑 Values ──────────────────────────────────────────────── ✕ ╮
//	│ 1  accessToken    eyJhbGciOiJIUzI1NiIs… (812 chars)   12:01:03 │
//	│ 2  Authorization  Bearer eyJhbGciOiJIUzI1…            12:01:05 │
//	│                                                                │
//	│ accessToken · FdevLog.value · changed 2 times                  │
//	│ eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkw… │
//	│ ↑/↓ choose · enter or 1-9 copy · g its log · esc close         │
//	╰────────────────────────────────────────────────────────────────╯

// kept is a value and where it was last seen.
type kept struct {
	Value
	at      time.Time
	entry   *Entry
	changes int
}

type vault struct {
	cursor int
}

// keep takes the values a new log has.
func (m *Model) keep(e *Entry) {
	for _, v := range e.Values {
		found := false
		for _, k := range m.values {
			if k.Name == v.Name {
				if k.Value.Value != v.Value {
					k.changes++
				}
				k.Value, k.at, k.entry, found = v, e.Time, e, true
				break
			}
		}
		if !found {
			m.values = append(m.values, &kept{Value: v, at: e.Time, entry: e})
		}
	}
}

func (m *Model) openVault() {
	m.vault = &vault{}
	m.dirty = true
	m.resize()
}

func (m *Model) closeVault() {
	m.vault = nil
	m.dirty = true
	m.resize()
}

func (m *Model) vaultKey(msg tea.KeyPressMsg) tea.Cmd {
	v := m.vault
	switch k := msg.String(); {
	case k == "esc" || k == "q" || k == "k" || k == "ctrl+c":
		m.closeVault()
	case k == "up":
		v.cursor = max(v.cursor-1, 0)
	case k == "down":
		v.cursor = min(v.cursor+1, max(len(m.values)-1, 0))
	case k == "enter" || k == "c" || k == "y":
		return m.copyValue(v.cursor)
	case k == "g":
		if v.cursor < len(m.values) && m.values[v.cursor].entry != nil {
			e := m.values[v.cursor].entry
			m.closeVault()
			return m.jumpTo(e)
		}
	case len(msg.Text) == 1 && msg.Text >= "1" && msg.Text <= "9":
		i := int(msg.Text[0] - '1')
		if i < len(m.values) {
			v.cursor = i
			return m.copyValue(i)
		}
	}
	return nil
}

func (m *Model) copyValue(i int) tea.Cmd {
	if i < 0 || i >= len(m.values) {
		return nil
	}
	return m.copyText(m.values[i].Value.Value, m.values[i].Name)
}

// vaultRows is where the value rows start in the window, from the top of
// the screen.
const vaultTop = 2

func (m *Model) vaultClick(x, y int) tea.Cmd {
	if y == 1 && x >= m.width-5 && x < m.width-1 {
		m.closeVault()
		return nil
	}
	if i := y - vaultTop; i >= 0 && i < len(m.values) {
		m.vault.cursor = i
		return m.copyValue(i)
	}
	return nil
}

func (m *Model) vaultView(height int) string {
	th := m.th
	inner := max(m.width-4, 10)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	var body []string
	if len(m.values) == 0 {
		body = append(body,
			muted.Render("No values yet."),
			"",
			"Values are kept from the logs as they come, for this session only:",
			"  · tokens in network headers and bodies (authorization, access_token, …),",
			"    when the app doesn't mask them",
			"  · what the app hands over: "+lipgloss.NewStyle().Foreground(th.Pink).Render("FdevLog.value('accessToken', token)"),
			"  · the keys in fdev.yaml's logs.values, e.g. [userId, deviceId]",
		)
	}
	nameW := 4
	for _, k := range m.values {
		nameW = max(nameW, min(lipgloss.Width(k.Name), 24))
	}
	for i, k := range m.values {
		num := " "
		if i < 9 {
			num = fmt.Sprint(i + 1)
		}
		when := k.at.Format("15:04:05")
		valW := max(inner-nameW-lipgloss.Width(when)-7, 8)
		val := strings.ReplaceAll(k.Value.Value, "\n", " ")
		line := lipgloss.NewStyle().Foreground(th.Pink).Bold(true).Render(num) + "  " +
			lipgloss.NewStyle().Bold(true).Render(fit(k.Name, nameW)) + "  " +
			fit(ansi.Truncate(val, valW, "…"), valW) + "  " + muted.Render(when)
		if i == m.vault.cursor {
			line = paintBg(line, inner, th.BarBg)
		}
		body = append(body, line)
	}
	if c := m.vault.cursor; c < len(m.values) {
		k := m.values[c]
		about := k.Name + " · " + k.From
		if k.changes > 0 {
			about += " · changed " + plural(k.changes, "time")
		}
		body = append(body, "", lipgloss.NewStyle().Foreground(th.Pink).Bold(true).Render(about))
		rows := max(height-3-len(body)-1, 1)
		full := wrap(k.Value.Value, "", "", inner)
		if len(full) > rows {
			full = append(full[:rows-1], muted.Render(fmt.Sprintf("… %d more lines · enter copies it all", len(full)-rows+1)))
		}
		body = append(body, full...)
	}
	return m.box("🔑 Values", body, "↑/↓ choose · enter or 1-9 copy · g its log · esc close", height)
}

// box is a window over the logs: a border with a title and a ✕, the body,
// and a hint row at the bottom.
func (m *Model) box(title string, body []string, hint string, height int) string {
	inner := max(m.width-4, 10)
	rows := max(height-3, 1)
	if len(body) > rows {
		body = body[:rows]
	}
	for i, l := range body {
		body[i] = ansi.Truncate(l, inner, "")
	}
	for len(body) < rows {
		body = append(body, "")
	}
	body = append(body, lipgloss.NewStyle().Foreground(m.th.Muted).Render(ansi.Truncate(hint, inner, "…")))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(m.th.Fuchsia).
		Padding(0, 1).Width(m.width).Render(strings.Join(body, "\n"))
	lines := strings.Split(box, "\n")
	t := ansi.Truncate(" "+title+" ", max(m.width-12, 4), "…")
	closeBtn := lipgloss.NewStyle().Foreground(m.th.Pink).Render(" ✕ ")
	border := lipgloss.NewStyle().Foreground(m.th.Fuchsia)
	fill := max(m.width-3-lipgloss.Width(t)-lipgloss.Width(closeBtn)-1, 0)
	lines[0] = border.Render("╭─") + lipgloss.NewStyle().Bold(true).Render(t) +
		border.Render(strings.Repeat("─", fill)) + closeBtn + border.Render("╮")
	return strings.Join(lines, "\n")
}
