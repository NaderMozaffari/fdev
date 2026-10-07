package logview

import (
	"cmp"
	"image/color"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/theme"
	"github.com/NaderMozaffari/fdev/internal/version"
)

// cols is the column layout of the log lines, for the look and toggles.
type cols struct {
	table        bool
	aligned      bool // columns or table: a tag column, network fields aligned
	timeW        int  // 0 when the time is hidden
	labelW       int  // 0 when the label is hidden
	tagW         int  // aligned only; 0 when the tag is hidden
	sep          string
	sepW         int
	timeFormat   string
	labelsBadges bool
}

const (
	statusW = 3 // "200", "ERR"
	tookW   = 7 // "12345ms"
	sizeW   = 7 // "123.4KB"
)

func (m *Model) cols() cols {
	look := m.look
	c := cols{table: look.Layout == config.LayoutTable, labelsBadges: look.Labels == config.LabelsBadge,
		timeFormat: "15:04:05"}
	c.aligned = look.Layout != config.LayoutLines
	if look.Time == config.TimeMillis {
		c.timeFormat = "15:04:05.000"
	}
	if m.on["time"] {
		c.timeW = len(c.timeFormat)
	}
	if m.on["label"] {
		c.labelW = labelWidth
		if c.labelsBadges {
			c.labelW = labelWidth + 2
		}
	}
	if c.aligned && m.on["tag"] {
		c.tagW = min(max(m.maxTag, len("flutter")), 18)
	}
	c.sep, c.sepW = " ", 1
	if c.table {
		c.sep = lipgloss.NewStyle().Foreground(m.th.Subtle).Render(" │ ")
		c.sepW = 3
	}
	return c
}

// widths of the columns before the message: time, label, icon, tag.
func (c cols) widths() []int {
	var w []int
	if c.timeW > 0 {
		w = append(w, c.timeW)
	}
	if c.labelW > 0 {
		w = append(w, c.labelW)
	}
	w = append(w, 1) // the ↗
	if c.tagW > 0 {
		w = append(w, c.tagW)
	}
	return w
}

// iconX is the screen column of the ↗.
func (c cols) iconX() int {
	x := 0
	if c.timeW > 0 {
		x += c.timeW + c.sepW
	}
	if c.labelW > 0 {
		x += c.labelW + c.sepW
	}
	return x
}

// blank is the prefix of a continuation line: empty columns, and the
// table's lines through them.
func (c cols) blank() string {
	var b strings.Builder
	for _, w := range c.widths() {
		b.WriteString(strings.Repeat(" ", w))
		b.WriteString(c.sep)
	}
	return b.String()
}

// header is the table's column titles and the rule under them.
func (m *Model) tableHeader(c cols) []string {
	title := lipgloss.NewStyle().Foreground(m.th.Muted).Bold(true)
	rule := lipgloss.NewStyle().Foreground(m.th.Subtle)
	var names []string
	if c.timeW > 0 {
		names = append(names, "TIME")
	}
	if c.labelW > 0 {
		names = append(names, "LEVEL")
	}
	names = append(names, " ")
	if c.tagW > 0 {
		names = append(names, "TAG")
	}
	var top, under strings.Builder
	for i, w := range c.widths() {
		name := names[i]
		if len(name) > w {
			name = name[:w]
		}
		top.WriteString(title.Render(name + strings.Repeat(" ", w-len(name))))
		top.WriteString(c.sep)
		under.WriteString(rule.Render(strings.Repeat("─", w) + "─┼─"))
	}
	rest := max(m.width-lipgloss.Width(top.String()), 0)
	top.WriteString(title.Render("MESSAGE"))
	under.WriteString(rule.Render(strings.Repeat("─", max(rest-1, 0))))
	return []string{top.String(), under.String()}
}

// labelText draws a level label: bold colored text, or a colored badge.
func (m *Model) labelText(c cols, level string) string {
	name, col := label(m.th, level)
	if c.labelsBadges {
		return lipgloss.NewStyle().Bold(true).Foreground(onColor(col)).Background(col).
			Width(c.labelW).Align(lipgloss.Center).Render(name)
	}
	return lipgloss.NewStyle().Bold(true).Foreground(col).Width(c.labelW).Render(name)
}

// onColor is a readable text color on a badge of c.
func onColor(c color.Color) color.Color {
	r, g, b, _ := c.RGBA()
	if (299*r+587*g+114*b)/1000 > 0x8000 {
		return lipgloss.Color("#1a1a1a")
	}
	return lipgloss.Color("#FFFDF5")
}

// Shortcuts are the key combinations the settings can turn off, for when
// they get hit by mistake.
var Shortcuts = []struct{ Key, Desc string }{
	{"ctrl+c", "stop the run (the Stop button still works); quit fdev"},
	{"ctrl+l", "clear the screen"},
	{"ctrl+s", "save the log"},
	{"ctrl+d", "end of input, sent to the command"},
	{"ctrl+z", "suspend, sent to the command"},
}

// About is the app, for the settings: its name and version on their
// first page, and more on their last (About).
type About struct {
	App   string // Acme Shop v2.1.39 (439)
	Facts []config.Fact
}

// SettingsForm is fdev's settings (key l in the viewer, s in the menu): the
// theme and size of the screens, how logs are drawn, the turned-off
// shortcuts, and what fdev and the app are. It writes into look and ui; the
// caller applies them when the form completes, and can try the theme and
// size as they change.
func SettingsForm(look *config.Look, ui *config.UI, th theme.Theme, about About) *huh.Form {
	var themes []huh.Option[string]
	for _, o := range theme.Options {
		themes = append(themes, huh.NewOption(o.Title+" · "+o.Desc, o.Name))
	}
	summary := "fdev " + version.Short()
	if about.App != "" {
		summary = about.App + "  ·  " + summary
	}
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(summary+"  ·  more under About"),
			huh.NewSelect[string]().Title("Theme").
				Description("tried as you move over it; esc puts the old one back").
				Options(themes...).Value(&ui.Theme),
			huh.NewSelect[string]().Title("Size of menus and their items").Options(
				huh.NewOption("Auto · the biggest that fits the window", config.SizeAuto),
				huh.NewOption("Small · one line an item, compact bars", config.SizeSmall),
				huh.NewOption("Medium · two lines an item", config.SizeMedium),
				huh.NewOption("Large · big icons, three or four lines an item", config.SizeLarge),
			).Value(&ui.Size),
		).Title("Screen").Description("For every project · z is full screen: no keys or buttons"),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Layout").Options(
				huh.NewOption("Lines · compact, like charmbracelet/log", config.LayoutLines),
				huh.NewOption("Columns · tags, URLs and statuses lined up", config.LayoutColumns),
				huh.NewOption("Table · columns with borders and a header", config.LayoutTable),
			).Value(&look.Layout),
			huh.NewSelect[string]().Title("Labels").Options(
				huh.NewOption("Text · INFO in color", config.LabelsText),
				huh.NewOption("Badges · INFO on a colored background", config.LabelsBadge),
			).Value(&look.Labels),
			huh.NewSelect[int]().Title("Space between logs").Options(
				huh.NewOption("None", 0),
				huh.NewOption("One blank line", 1),
				huh.NewOption("Two blank lines", 2),
			).Value(&look.Spacing),
			huh.NewSelect[string]().Title("Line between related logs").Options(
				huh.NewOption("After a pause · each burst of logs is a group", config.GroupPause),
				huh.NewOption("When the tag changes · a run of one tag is a group", config.GroupTag),
				huh.NewOption("Off", config.GroupOff),
			).Value(&look.Group),
			huh.NewSelect[string]().Title("Time").Options(
				huh.NewOption("14:02:11", config.TimeClock),
				huh.NewOption("14:02:11.482", config.TimeMillis),
			).Value(&look.Time),
			huh.NewSelect[string]().Title("Persian and Arabic text").Options(
				huh.NewOption("Auto · fdev joins the letters and lays them out right to left, unless the terminal does", RTLAuto),
				huh.NewOption("fdev does it · for VS Code, iTerm2, Ghostty, kitty, Windows Terminal", RTLFdev),
				huh.NewOption("The terminal does it · Terminal.app, GNOME Terminal, Konsole", RTLTerminal),
			).Value(&look.RTL),
			huh.NewConfirm().Title("Save every session").
				Description("to "+look.Dir+" in the project, which git ignores; ctrl+s saves any time").
				Affirmative("Yes").Negative("No").Value(&look.Save),
		).Title("Log look").Description("How logs are drawn; saved for this project"),
		huh.NewGroup(
			huh.NewMultiSelect[string]().Title("Turned-off shortcuts").
				Description("x or space picks one; a picked shortcut does nothing when pressed").
				Options(shortcutOptions(look.Off)...).Height(len(Shortcuts)+2).Value(&look.Off),
		).Title("Shortcuts").Description("For the ones that get hit by mistake; saved for this project"),
		huh.NewGroup(
			huh.NewNote().Title("fdev "+version.Short()).
				Description(factLines(version.Facts())+"\n\n"+noteTitle(cmp.Or(about.App, "The app"))+"\n"+factLines(about.Facts)).
				Next(true).NextLabel("Save"),
		).Title("About").Description("fdev and the app it runs; enter saves the settings"),
	).
		WithTheme(th.Huh()).
		WithShowHelp(true)
}

// factLines is facts a line each, their keys lined up, for a note.
func factLines(facts []config.Fact) string {
	w := 0
	for _, f := range facts {
		w = max(w, len([]rune(f.Key)))
	}
	lines := make([]string, len(facts))
	for i, f := range facts {
		pad := strings.Repeat(" ", w-len([]rune(f.Key))+2)
		lines[i] = "  " + noteTitle(f.Key) + pad + noteEscape(f.Value)
	}
	return strings.Join(lines, "\n")
}

// noteTitle is s in bold, in a note.
func noteTitle(s string) string { return "*" + noteEscape(s) + "*" }

// noteEscape keeps a note from reading _, * and ` in s as styles.
func noteEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("\\_*`", r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func shortcutOptions(off []string) []huh.Option[string] {
	var out []huh.Option[string]
	for _, k := range Shortcuts {
		o := huh.NewOption(k.Key+" · "+k.Desc, k.Key)
		for _, v := range off {
			if v == k.Key {
				o = o.Selected(true)
			}
		}
		out = append(out, o)
	}
	return out
}
