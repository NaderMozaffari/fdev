package logview

import (
	"image/color"
	"regexp"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// The command bar, under the toggle bar: buttons for the keys the running
// command takes, each with its key beside it, then fdev's own (stop, back).
// They are worked out from the output: flutter's (when it lists its key
// commands), a key list after a "key commands" or "shortcuts" line, or
// "press r + enter to restart" lines (Vite, Expo and the like).
//
//	FLUTTER  r Hot reload  R Hot restart  v DevTools  …  ⋯ more   ctrl+c Stop

type command struct {
	key   string // shown on the button: r, ctrl+c, enter
	label string
	send  string // written to the command; "" for fdev's own
	more  bool   // behind the more button
	act   string // fdev's own: stop, back, more
}

// flutterCommands are flutter run's keys (`h` lists them all); the usual
// ones first, the rest behind "more".
var flutterCommands = []command{
	{key: "r", label: "Reload"}, {key: "R", label: "Restart"},
	{key: "v", label: "DevTools"}, {key: "i", label: "Inspector"},
	{key: "s", label: "Screenshot"}, {key: "q", label: "Quit"},
	{key: "p", label: "Layout lines", more: true}, {key: "b", label: "Brightness", more: true},
	{key: "o", label: "Platform", more: true}, {key: "P", label: "Performance", more: true},
	{key: "c", label: "Clear", more: true}, {key: "d", label: "Detach", more: true},
	{key: "w", label: "Widget tree", more: true}, {key: "t", label: "Render tree", more: true},
	{key: "L", label: "Layer tree", more: true}, {key: "f", label: "Focus tree", more: true},
	{key: "S", label: "Semantics", more: true}, {key: "U", label: "Semantics (hit test)", more: true},
	{key: "I", label: "Invert big images", more: true}, {key: "a", label: "Build timeline", more: true},
	{key: "M", label: "SkSL shaders", more: true}, {key: "g", label: "Code generators", more: true},
	{key: "j", label: "Raster stats", more: true}, {key: "h", label: "Flutter help", more: true},
}

var (
	keyListHeader = regexp.MustCompile(`(?i)\b(key commands|keyboard shortcuts|shortcuts)\b`)
	keyListLine   = regexp.MustCompile(`^\s*([A-Za-z0-9?])\s{1,4}([A-Z][^.]{2,60}?)\.?(?:\s{2,}\(.*\))?\s*(?:🔥.*)?$`)
	pressLine     = regexp.MustCompile("(?i)\\bpress\\s+[\"'`]?([A-Za-z0-9?])[\"'`]?\\s*(\\+\\s*enter)?\\s*(?:to\\s+|│\\s*|\\|\\s*|:\\s*)([^.│|]{2,48})")
)

// detectCommands reads an output line for the keys the command takes.
func (m *Model) detectCommands(line string) {
	text := strings.TrimSpace(ansi.Strip(line))
	switch {
	case strings.Contains(text, "Flutter run key commands"):
		m.tool, m.inKeyList = "flutter", false
		m.commands = append([]command(nil), flutterCommands...)
		return
	case m.tool == "flutter" || text == "":
		return
	}
	if m.inKeyList {
		if k := keyListLine.FindStringSubmatch(text); k != nil {
			m.addCommand(command{key: k[1], label: k[2]})
			return
		}
		m.inKeyList = false
	}
	if keyListHeader.MatchString(text) && len(text) < 60 {
		m.inKeyList = true
		return
	}
	for _, p := range pressLine.FindAllStringSubmatch(text, -1) {
		c := command{key: p[1], label: strings.TrimSpace(p[3])}
		if p[2] != "" {
			c.key, c.send = p[1]+" ⏎", p[1]+"\r"
		}
		m.addCommand(c)
	}
}

func (m *Model) addCommand(c command) {
	if c.send == "" {
		c.send = c.key
	}
	r := []rune(c.label)
	c.label = string(unicode.ToUpper(r[0])) + string(r[1:])
	for i, old := range m.commands {
		if old.key == c.key {
			m.commands[i] = c
			return
		}
	}
	if m.tool == "" {
		m.tool = "keys"
	}
	c.more = len(m.commands) >= 6
	m.commands = append(m.commands, c)
}

// shownCommands is the buttons in the bar now: the command's while it
// runs (and the more ones when open), then fdev's.
func (m *Model) shownCommands() []command {
	var out []command
	if !m.exited && m.o.Replay == "" {
		more := false
		for _, c := range m.commands {
			if c.send == "" {
				c.send = c.key
			}
			if c.more {
				more = true
				if !m.showMore {
					continue
				}
			}
			out = append(out, c)
		}
		if more {
			label := "more"
			if m.showMore {
				label = "less"
			}
			out = append(out, command{key: "⋯", label: label, act: "more"})
		}
		stop := command{key: "ctrl+c", label: "Stop", act: "stop"}
		if m.look.KeyOff("ctrl+c") {
			stop.key = "■" // a button only
		}
		out = append(out, stop)
	} else {
		out = append(out, command{key: "enter", label: "Back", act: "back"})
	}
	return out
}

func (m *Model) commandColor(c command) color.Color {
	th := m.th
	label := strings.ToLower(c.label)
	switch {
	case c.act == "stop", strings.Contains(label, "quit"), strings.Contains(label, "exit"), strings.Contains(label, "terminate"):
		return th.Red
	case c.act == "back":
		return th.Purple
	case c.act == "more":
		return th.Muted
	case strings.Contains(label, "reload"):
		return th.Green
	case strings.Contains(label, "restart"):
		return th.Indigo
	case strings.Contains(label, "devtools"), strings.Contains(label, "inspector"), strings.Contains(label, "debug"):
		return th.Fuchsia
	}
	return lipgloss.Color("#7D56F4")
}

// commandItems lays the command bar out: fdev's buttons (more, stop or
// back) at the right of the first row, the command's from the left,
// wrapping onto up to four rows. Each item is named "cmd:<letter>".
func (m *Model) commandItems() []barItem {
	th := m.th
	cmds := m.shownCommands()
	render := func(i int, c command) barItem {
		name := "cmd:" + string(rune('a'+i))
		bg := m.commandColor(c)
		keyStyle := lipgloss.NewStyle().Padding(0, 1).Bold(true).Background(bg).Foreground(onColor(bg))
		labelStyle := lipgloss.NewStyle().Padding(0, 1).Background(th.BarBg).Foreground(th.Normal)
		if m.hover == name {
			labelStyle = labelStyle.Foreground(th.Pink).Underline(true)
		}
		text := keyStyle.Render(c.key) + labelStyle.Render(c.label)
		return barItem{name: name, text: text, width: lipgloss.Width(text)}
	}

	var out, own []barItem
	ownW := 0
	for i, c := range cmds {
		if c.act != "" {
			it := render(i, c)
			own = append(own, it)
			ownW += it.width + 1
		}
	}
	right := m.width - ownW // the command's buttons stop here on the first row
	x, y := 1, 0
	lastRow := 3 // the command's buttons wrap onto up to four rows
	if m.compact() {
		lastRow = 0 // one row; the ones that don't fit still take their keys
		if m.showMore {
			lastRow = 2
		}
	}
	if title := map[string]string{"flutter": "FLUTTER", "keys": "KEYS"}[m.tool]; title != "" && !m.exited && m.o.Replay == "" && m.width >= 130 {
		t := lipgloss.NewStyle().Foreground(th.Muted).Bold(true).Render(title)
		out = append(out, barItem{text: t, x: x, y: y, width: lipgloss.Width(t)})
		x += lipgloss.Width(t) + 2
	}
	for i, c := range cmds {
		if c.act != "" {
			continue
		}
		it := render(i, c)
		limit := m.width
		if y == 0 {
			limit = right - 1
		}
		if x+it.width > limit {
			if y == lastRow {
				break
			}
			x, y = 1, y+1
		}
		it.x, it.y = x, y
		out = append(out, it)
		x += it.width + 1
	}
	x = max(right, 1)
	for _, it := range own {
		it.x, it.y = x, 0
		out = append(out, it)
		x += it.width + 1
	}
	return out
}

func (m *Model) commandRows() int {
	if m.lookForm != nil || m.full {
		return 0
	}
	rows := 0
	for _, it := range m.commandItems() {
		rows = max(rows, it.y+1)
	}
	return rows
}

func (m *Model) commandBar() string {
	rows := make([]string, m.commandRows())
	for _, it := range m.commandItems() {
		rows[it.y] += strings.Repeat(" ", max(it.x-lipgloss.Width(rows[it.y]), 0)) + it.text
	}
	for i := range rows {
		rows[i] = ansi.Truncate(rows[i], m.width, "…")
	}
	return strings.Join(rows, "\n")
}

// commandAt is the command bar item at a screen position.
func (m *Model) commandAt(x, y int) string {
	row := y - 1 - m.headerRows() - m.vp.Height() - m.toggleRows()
	for _, it := range m.commandItems() {
		if it.name != "" && it.y == row && x >= it.x && x < it.x+it.width {
			return it.name
		}
	}
	return ""
}

// clickCommand presses a command bar button.
func (m *Model) clickCommand(name string) tea.Cmd {
	id, ok := strings.CutPrefix(name, "cmd:")
	if !ok || id == "" {
		return nil
	}
	cmds := m.shownCommands()
	i := int(id[0] - 'a')
	if i < 0 || i >= len(cmds) {
		return nil
	}
	c := cmds[i]
	switch c.act {
	case "more":
		m.showMore = !m.showMore
		m.resize()
	case "stop":
		m.write("\x03")
	case "back":
		return m.done()
	default:
		m.write(c.send)
		m.setStatus("→ " + c.label)
	}
	return nil
}
