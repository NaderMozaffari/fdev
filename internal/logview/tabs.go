package logview

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// The tabs style's bottom line: filters to pick with a click, or tab and
// shift+tab, each with how many logs it has.
//
//	fdev  All  Network 8  Errors 1  Warnings 3  Bookmarked
type logTab struct{ title, filter, count string }

var logTabs = []logTab{
	{"All", "", ""},
	{"Network", "level:network", "network"},
	{"Errors", "level:error", "error"},
	{"Warnings", "level:warn", "warning"},
	{"Bookmarked", "is:bookmarked", ""},
}

// tab is the tab the filter is, or -1 when it is one typed with /.
func (m *Model) tab() int {
	for i, t := range logTabs {
		if m.filter == t.filter {
			return i
		}
	}
	return -1
}

func (m *Model) pickTab(i int) {
	if i >= 0 && i < len(logTabs) {
		m.setFilter(logTabs[i].filter)
	}
}

// tabItems lays the tabs out after fdev's name; their names are tab:0 to
// tab:4, for clicks.
func (m *Model) tabItems() []barItem {
	x := lipgloss.Width(m.fdevBadge()) + 1
	cur := m.tab()
	var out []barItem
	for i, t := range logTabs {
		text := t.title
		n := m.counts[t.count]
		if t.count == "error" {
			n = m.errors
		}
		if t.count != "" && n > 0 {
			text += fmt.Sprintf(" %d", n)
		}
		name := fmt.Sprintf("tab:%d", i)
		s := lipgloss.NewStyle().Padding(0, 1).Foreground(m.th.Muted)
		switch {
		case i == cur:
			s = s.Background(m.th.Pink).Foreground(onColor(m.th.Pink)).Bold(true)
		case t.count == "error" && n > 0:
			s = s.Foreground(m.th.Red)
		}
		if m.hover == name {
			s = s.Underline(true)
		}
		it := barItem{name: name, text: s.Render(text), x: x}
		it.width = lipgloss.Width(it.text)
		out = append(out, it)
		x += it.width
	}
	return out
}
