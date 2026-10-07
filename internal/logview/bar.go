package logview

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// The toggle bar: sections of chips, each a button for the mouse and a key
// for the keyboard. A chip that is on has its category's color behind it.
//
//	LOGS  1 info  2 success  3 warning …  │  SHOW  7 details …  │  COLUMNS  , time …  │  / filter  ⊞ expand  k values  ⚙ look  ⤓ save  ? help

type section struct {
	title string
	names []string
}

var sections = []section{
	{"LOGS", []string{"info", "success", "warning", "error", "debug", "network"}},
	{"SHOW", []string{"details", "native", "raw"}},
	{"COLUMNS", []string{"time", "label", "tag"}},
}

type barItem struct {
	name        string // a toggle, "section:<title>", filter, clear, look, save or help
	text        string
	x, y, width int
}

func toggleByName(name string) Toggle {
	for _, t := range Toggles {
		if t.Name == name {
			return t
		}
	}
	return Toggle{}
}

// chipColor is the color of a toggle that is on.
func (m *Model) chipColor(name string) color.Color {
	th := m.th
	switch name {
	case "info":
		return th.Info
	case "success":
		return th.Green
	case "warning":
		return th.Warn
	case "error":
		return th.Error
	case "debug":
		return th.Debug
	case "network":
		return th.Indigo
	case "details", "native", "raw":
		return th.Fuchsia
	}
	return th.Purple
}

func (m *Model) chip(name, text string, on bool, c color.Color) string {
	style := lipgloss.NewStyle().Padding(0, 1)
	if c == nil {
		c = m.th.Purple
	}
	switch {
	case on:
		style = style.Background(c).Foreground(onColor(c)).Bold(true)
	default:
		style = style.Background(m.th.BarBg).Foreground(m.th.Muted)
	}
	if m.hover == name {
		style = style.Underline(true).Foreground(m.th.Pink)
		if on {
			style = style.Foreground(onColor(c))
		}
	}
	return style.Render(text)
}

// barItems lays the bar out: row 0 is a rule; sections that don't fit move
// to the next row whole. Compact, there is no rule, no section titles and
// less space between sections.
func (m *Model) barItems() []barItem {
	if m.full {
		return nil
	}
	th := m.th
	compact := m.compact()
	titleStyle := lipgloss.NewStyle().Foreground(th.Muted).Bold(true)
	rule := lipgloss.NewStyle().Foreground(th.Subtle).Render("│")

	type group struct{ items []barItem }
	var groups []group
	if !m.o.Plain {
		for _, s := range sections {
			g := group{}
			if !compact {
				title := titleStyle.Render(s.title)
				if m.hover == "section:"+s.title {
					title = titleStyle.Foreground(th.Pink).Underline(true).Render(s.title)
				}
				g.items = append(g.items, barItem{name: "section:" + s.title, text: title})
			}
			for _, name := range s.names {
				t := toggleByName(name)
				label := t.Key + " " + name
				if n := m.counts[name]; !m.on[name] && n > 0 && name != "raw" && name != "details" {
					label += fmt.Sprintf(" ·%d", n) // hidden lines
				}
				g.items = append(g.items, barItem{name: name, text: m.chip(name, label, m.on[name], m.chipColor(name))})
			}
			groups = append(groups, g)
		}
	}
	actions := group{}
	if m.filter != "" {
		actions.items = append(actions.items,
			barItem{name: "filter", text: m.chip("filter", "/ "+ansi.Truncate(m.filter, 16, "…"), true, th.Pink)},
			barItem{name: "clear", text: m.chip("clear", "✕", false, nil)})
	} else {
		actions.items = append(actions.items, barItem{name: "filter", text: m.chip("filter", "/ filter", false, nil)})
	}
	if !m.o.Plain {
		// Open all calls, once there are calls to open; the values, once
		// there are values.
		if m.counts["network"] > 0 && m.on["network"] && !m.on["details"] {
			expand := "⊞ open all"
			if m.expandAll {
				expand = "⊟ fold all"
			}
			actions.items = append(actions.items, barItem{name: "expand", text: m.chip("expand", expand, m.expandAll, th.Indigo)})
		}
		if n := len(m.values); n > 0 {
			actions.items = append(actions.items, barItem{name: "values", text: m.chip("values", fmt.Sprintf("k values ·%d", n), true, th.Fuchsia)})
		}
		actions.items = append(actions.items, barItem{name: "look", text: m.chip("look", "⚙ look", false, nil)})
	}
	if m.o.Replay == "" {
		actions.items = append(actions.items, barItem{name: "wipe", text: m.chip("wipe", "⌫ clear", false, nil)})
	}
	actions.items = append(actions.items,
		barItem{name: "save", text: m.chip("save", "⤓ save", m.raw != nil, th.Red)},
		barItem{name: "full", text: m.chip("full", "⤢ full", false, nil)},
		barItem{name: "help", text: m.chip("help", "? help", m.help.ShowAll, th.Purple)})
	groups = append(groups, actions)

	var out []barItem
	x, y := 1, 1
	divider, gapW := "  "+rule+"  ", 5
	if compact {
		y, divider, gapW = 0, " "+rule+" ", 3
	}
	for gi, g := range groups {
		width := 0
		for i, it := range g.items {
			width += lipgloss.Width(it.text)
			if i > 0 {
				width++
			}
		}
		gap := 0
		if gi > 0 {
			gap = gapW
		}
		if x > 1 && x+gap+width > m.width {
			x, y, gap = 1, y+1, 0
		}
		if gap > 0 {
			out = append(out, barItem{name: "", text: divider, x: x, y: y, width: gap})
			x += gap
		}
		for i, it := range g.items {
			if i > 0 {
				x++
			}
			it.x, it.y, it.width = x, y, lipgloss.Width(it.text)
			out = append(out, it)
			x += it.width
		}
	}
	return out
}

// toggleRows is the height of the bar: the rule and the chip rows.
func (m *Model) toggleRows() int {
	rows := 0
	for _, it := range m.barItems() {
		rows = max(rows, it.y+1)
	}
	return rows
}

func (m *Model) toggleBar() string {
	items := m.barItems()
	rows := make([]string, m.toggleRows())
	if !m.compact() {
		rows[0] = lipgloss.NewStyle().Foreground(m.th.Subtle).Render(strings.Repeat("─", m.width))
	}
	for _, it := range items {
		rows[it.y] += strings.Repeat(" ", max(it.x-lipgloss.Width(rows[it.y]), 0)) + it.text
	}
	for i := range rows {
		rows[i] = ansi.Truncate(rows[i], m.width, "…")
	}
	return strings.Join(rows, "\n")
}

// barItemAt is the bar item under the mouse, if any.
func (m *Model) barItemAt(x, y int) string {
	bar := y - 1 - m.headerRows() - m.vp.Height()
	for _, it := range m.barItems() {
		if it.name != "" && it.y == bar && x >= it.x && x < it.x+it.width {
			return it.name
		}
	}
	return ""
}

// clickBar does what the clicked item says.
func (m *Model) clickBar(name string) tea.Cmd {
	switch {
	case name == "filter":
		return m.startFilter()
	case name == "clear":
		m.filter = ""
		m.changed()
	case name == "look":
		return m.openLook()
	case name == "save":
		return m.openSave()
	case name == "expand":
		m.toggleExpandAll()
	case name == "values":
		m.openVault()
	case name == "wipe":
		m.clearScreen("cleared")
	case name == "help":
		m.toggleHelp()
	case name == "full":
		m.setFull(true)
	case strings.HasPrefix(name, "section:"):
		title := strings.TrimPrefix(name, "section:")
		for _, s := range sections {
			if s.title != title {
				continue
			}
			// All on turns them all off; otherwise all on.
			all := true
			for _, n := range s.names {
				all = all && m.on[n]
			}
			for _, n := range s.names {
				m.on[n] = !all
				if m.o.Settings != nil {
					m.o.Settings[n] = !all
				}
			}
			m.changed()
		}
	case name != "":
		m.toggle(name)
	}
	return nil
}
