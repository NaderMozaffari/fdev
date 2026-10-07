// Package theme is fdev's palettes: Charm's, as used by Bubbles (list), Huh
// (ThemeCharm) and Log, for dark and light terminals, and a few well-known
// ones that paint the terminal's background too.
package theme

import (
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

type Theme struct {
	Name string
	Dark bool
	// TermBg and TermFg are the terminal's background and text color while fdev
	// runs; nil keeps the terminal's own (Charm's themes).
	TermBg, TermFg color.Color

	// Bubbles list and Huh ThemeCharm.
	Purple    color.Color // list title background
	Cream     color.Color
	Fuchsia   color.Color // huh selector, buttons
	Pink      color.Color // selected list item
	PinkDim   color.Color // selected item border and description
	Indigo    color.Color // huh titles
	Green     color.Color
	Red       color.Color
	Yellow    color.Color
	Normal    color.Color
	Muted     color.Color // descriptions, status bar
	Subtle    color.Color // separators, inactive dots
	BarBg     color.Color // badge backgrounds
	FilterKey color.Color // list filter prompt
	// SelFrom and SelTo are the selected row's gradient in the menus.
	SelFrom, SelTo color.Color

	// charmbracelet/log levels.
	Debug, Info, Warn, Error, Fatal color.Color
}

// Auto is the theme that follows the terminal: Charm's, dark or light.
const Auto = "auto"

// Option is a theme the settings offer.
type Option struct {
	Name, Title, Desc string
}

// Options is every theme, Auto first.
var Options = []Option{
	{Auto, "Auto", "Charm's colors, dark or light like the terminal"},
	{"charm-dark", "Charm dark", "Charm's colors on a dark background"},
	{"charm-light", "Charm light", "Charm's colors on a light background"},
	{"dracula", "Dracula", "purple and pink on a dark gray"},
	{"nord", "Nord", "frosty blues on a polar night"},
	{"tokyo-night", "Tokyo Night", "neon blues on a deep navy"},
	{"catppuccin", "Catppuccin Mocha", "soft pastels on a dark mocha"},
	{"gruvbox", "Gruvbox", "warm retro colors on a dark brown"},
	{"latte", "Catppuccin Latte", "soft pastels on a light background"},
	{"solarized", "Solarized Light", "low contrast on a cream background"},
}

// Title is a theme's name as the settings show it.
func Title(name string) string {
	for _, o := range Options {
		if o.Name == name {
			return o.Title
		}
	}
	return Options[0].Title
}

// Named is the theme called name; Auto (and an unknown name) is Charm's,
// dark when the terminal is.
func Named(name string, terminalDark bool) Theme {
	c := lipgloss.Color
	switch name {
	case "charm-dark":
		t := New(true)
		t.TermBg, t.TermFg = c("#16161E"), c("#DDDDDD")
		return named(name, t)
	case "charm-light":
		t := New(false)
		t.TermBg, t.TermFg = c("#FBFAF7"), c("#1A1A1A")
		return named(name, t)
	case "dracula":
		return named(name, palette{dark: true, bg: "#282A36", fg: "#F8F8F2", muted: "#6272A4", subtle: "#44475A", bar: "#343746",
			purple: "#6C5BB8", fuchsia: "#FF79C6", pink: "#FF92DF", pinkDim: "#BD5A9F", indigo: "#BD93F9",
			green: "#50FA7B", red: "#FF5555", yellow: "#F1FA8C", cyan: "#8BE9FD", orange: "#FFB86C",
			selFrom: "#5A3E6E", selTo: "#2C2E3B"}.theme())
	case "nord":
		return named(name, palette{dark: true, bg: "#2E3440", fg: "#ECEFF4", muted: "#7B88A1", subtle: "#434C5E", bar: "#3B4252",
			purple: "#5E81AC", fuchsia: "#B48EAD", pink: "#D3A5CB", pinkDim: "#8F7089", indigo: "#81A1C1",
			green: "#A3BE8C", red: "#BF616A", yellow: "#EBCB8B", cyan: "#88C0D0", orange: "#D08770",
			selFrom: "#4C566A", selTo: "#313744"}.theme())
	case "tokyo-night":
		return named(name, palette{dark: true, bg: "#1A1B26", fg: "#C0CAF5", muted: "#565F89", subtle: "#292E42", bar: "#24283B",
			purple: "#3D59A1", fuchsia: "#BB9AF7", pink: "#F7768E", pinkDim: "#9D7CD8", indigo: "#7AA2F7",
			green: "#9ECE6A", red: "#F7768E", yellow: "#E0AF68", cyan: "#7DCFFF", orange: "#FF9E64",
			selFrom: "#33335A", selTo: "#1D1E2C"}.theme())
	case "catppuccin":
		return named(name, palette{dark: true, bg: "#1E1E2E", fg: "#CDD6F4", muted: "#7F849C", subtle: "#45475A", bar: "#313244",
			purple: "#8839EF", fuchsia: "#F5C2E7", pink: "#CBA6F7", pinkDim: "#9A7BC4", indigo: "#89B4FA",
			green: "#A6E3A1", red: "#F38BA8", yellow: "#F9E2AF", cyan: "#94E2D5", orange: "#FAB387",
			selFrom: "#45375C", selTo: "#212131"}.theme())
	case "gruvbox":
		return named(name, palette{dark: true, bg: "#282828", fg: "#EBDBB2", muted: "#928374", subtle: "#504945", bar: "#3C3836",
			purple: "#8F3F71", fuchsia: "#D3869B", pink: "#FE8019", pinkDim: "#B16286", indigo: "#83A598",
			green: "#B8BB26", red: "#FB4934", yellow: "#FABD2F", cyan: "#8EC07C", orange: "#FE8019",
			selFrom: "#4E3A2A", selTo: "#2C2A28"}.theme())
	case "latte":
		return named(name, palette{dark: false, bg: "#EFF1F5", fg: "#4C4F69", muted: "#8C8FA1", subtle: "#CCD0DA", bar: "#E1E4EB",
			purple: "#7287FD", fuchsia: "#EA76CB", pink: "#8839EF", pinkDim: "#B48AE8", indigo: "#1E66F5",
			green: "#40A02B", red: "#D20F39", yellow: "#DF8E1D", cyan: "#179299", orange: "#FE640B",
			selFrom: "#DCCBF5", selTo: "#ECEEF3"}.theme())
	case "solarized":
		return named(name, palette{dark: false, bg: "#FDF6E3", fg: "#586E75", muted: "#93A1A1", subtle: "#E4DCC6", bar: "#EEE8D5",
			purple: "#6C71C4", fuchsia: "#D33682", pink: "#D33682", pinkDim: "#C47FA5", indigo: "#268BD2",
			green: "#859900", red: "#DC322F", yellow: "#B58900", cyan: "#2AA198", orange: "#CB4B16",
			selFrom: "#F2DDE6", selTo: "#FAF3E0"}.theme())
	}
	return named(Auto, New(terminalDark))
}

// Painted reports whether c is a background a theme paints, so a report of
// the terminal's background that is one of them says nothing about it.
func Painted(c color.Color) bool {
	if c == nil {
		return false
	}
	r, g, b, _ := c.RGBA()
	for _, o := range Options {
		if bg := Named(o.Name, true).TermBg; bg != nil {
			r2, g2, b2, _ := bg.RGBA()
			if r>>8 == r2>>8 && g>>8 == g2>>8 && b>>8 == b2>>8 {
				return true
			}
		}
	}
	return false
}

func named(name string, t Theme) Theme {
	t.Name = name
	return t
}

// palette is a theme's colors, as hex.
type palette struct {
	dark                                   bool
	bg, fg, muted, subtle, bar             string
	purple, fuchsia, pink, pinkDim, indigo string
	green, red, yellow, cyan, orange       string
	selFrom, selTo                         string
}

func (p palette) theme() Theme {
	c := lipgloss.Color
	return Theme{
		Dark: p.dark, TermBg: c(p.bg), TermFg: c(p.fg),
		Purple: c(p.purple), Cream: c("#FFFDF5"), Fuchsia: c(p.fuchsia), Pink: c(p.pink), PinkDim: c(p.pinkDim),
		Indigo: c(p.indigo), Green: c(p.green), Red: c(p.red), Yellow: c(p.yellow),
		Normal: c(p.fg), Muted: c(p.muted), Subtle: c(p.subtle), BarBg: c(p.bar), FilterKey: c(p.green),
		SelFrom: c(p.selFrom), SelTo: c(p.selTo),
		Debug: c(p.indigo), Info: c(p.cyan), Warn: c(p.yellow), Error: c(p.red), Fatal: c(p.orange),
	}
}

// New is Charm's palette, for a dark or a light terminal.
func New(dark bool) Theme {
	ld := lipgloss.LightDark(dark)
	c := lipgloss.Color
	return Theme{
		Name:      Auto,
		Dark:      dark,
		Purple:    c("62"),
		Cream:     c("#FFFDF5"),
		Fuchsia:   c("#F780E2"),
		Pink:      c("#EE6FF8"),
		PinkDim:   ld(c("#F793FF"), c("#AD58B4")),
		Indigo:    ld(c("#5A56E0"), c("#7571F9")),
		Green:     ld(c("#02BA84"), c("#02BF87")),
		Red:       ld(c("#FF4672"), c("#ED567A")),
		Yellow:    ld(c("#C9A300"), c("192")),
		Normal:    ld(c("#1a1a1a"), c("#dddddd")),
		Muted:     ld(c("#A49FA5"), c("#777777")),
		Subtle:    ld(c("#DDDADA"), c("#3C3C3C")),
		BarBg:     ld(c("#EEEAF0"), c("#2A2A2A")),
		FilterKey: ld(c("#04B575"), c("#ECFD65")),
		SelFrom:   ld(c("#F6D5F7"), c("#4A2C5E")),
		SelTo:     ld(c("#FBF8FB"), c("#1B1B24")),
		Debug:     c("63"),
		Info:      ld(c("#00A08A"), c("86")),
		Warn:      ld(c("#A38F00"), c("192")),
		Error:     c("204"),
		Fatal:     c("134"),
	}
}

func (t Theme) Fg(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// Badge is text on a colored background, like the Bubbles list title.
func (t Theme) Badge(fg, bg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(0, 1)
}

// Help is the Bubbles help's styles in the theme's colors.
func (t Theme) Help() help.Styles {
	s := help.DefaultStyles(t.Dark)
	if t.Name == Auto {
		return s
	}
	s.ShortKey = s.ShortKey.Foreground(t.Normal)
	s.ShortDesc = s.ShortDesc.Foreground(t.Muted)
	s.ShortSeparator = s.ShortSeparator.Foreground(t.Subtle)
	s.Ellipsis = s.Ellipsis.Foreground(t.Subtle)
	s.FullKey = s.FullKey.Foreground(t.Normal)
	s.FullDesc = s.FullDesc.Foreground(t.Muted)
	s.FullSeparator = s.FullSeparator.Foreground(t.Subtle)
	return s
}

// List is the Bubbles list's styles in the theme's colors.
func (t Theme) List() list.Styles {
	s := list.DefaultStyles(t.Dark)
	if t.Name == Auto {
		return s
	}
	s.StatusBar = s.StatusBar.Foreground(t.Muted)
	s.StatusEmpty = s.StatusEmpty.Foreground(t.Subtle)
	s.StatusBarActiveFilter = s.StatusBarActiveFilter.Foreground(t.Normal)
	s.StatusBarFilterCount = s.StatusBarFilterCount.Foreground(t.Subtle)
	s.NoItems = s.NoItems.Foreground(t.Muted)
	s.ActivePaginationDot = s.ActivePaginationDot.Foreground(t.Muted)
	s.InactivePaginationDot = s.InactivePaginationDot.Foreground(t.Subtle)
	s.ArabicPagination = s.ArabicPagination.Foreground(t.Subtle)
	s.DividerDot = s.DividerDot.Foreground(t.Subtle)
	s.Filter.Focused.Prompt = s.Filter.Focused.Prompt.Foreground(t.FilterKey)
	s.Filter.Blurred.Prompt = s.Filter.Blurred.Prompt.Foreground(t.FilterKey)
	s.DefaultFilterCharacterMatch = s.DefaultFilterCharacterMatch.Foreground(t.Pink)
	return s
}

// Huh is Huh's Charm theme in the theme's colors.
func (t Theme) Huh() huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		s := huh.ThemeCharm(t.Dark)
		if t.Name == Auto {
			return s
		}
		f := &s.Focused
		f.Base = f.Base.BorderForeground(t.Subtle)
		f.Card = f.Base
		f.Title = f.Title.Foreground(t.Indigo)
		f.NoteTitle = f.NoteTitle.Foreground(t.Indigo)
		f.Description = f.Description.Foreground(t.Muted)
		f.ErrorIndicator = f.ErrorIndicator.Foreground(t.Red)
		f.ErrorMessage = f.ErrorMessage.Foreground(t.Red)
		f.SelectSelector = f.SelectSelector.Foreground(t.Fuchsia)
		f.NextIndicator = f.NextIndicator.Foreground(t.Fuchsia)
		f.PrevIndicator = f.PrevIndicator.Foreground(t.Fuchsia)
		f.Option = f.Option.Foreground(t.Normal)
		f.MultiSelectSelector = f.MultiSelectSelector.Foreground(t.Fuchsia)
		f.SelectedOption = f.SelectedOption.Foreground(t.Green)
		f.SelectedPrefix = f.SelectedPrefix.Foreground(t.Green)
		f.UnselectedPrefix = f.UnselectedPrefix.Foreground(t.Muted)
		f.UnselectedOption = f.UnselectedOption.Foreground(t.Normal)
		f.FocusedButton = f.FocusedButton.Foreground(t.Cream).Background(t.Fuchsia)
		f.Next = f.FocusedButton
		f.BlurredButton = f.BlurredButton.Foreground(t.Normal).Background(t.BarBg)
		f.TextInput.Cursor = f.TextInput.Cursor.Foreground(t.Green)
		f.TextInput.Placeholder = f.TextInput.Placeholder.Foreground(t.Subtle)
		f.TextInput.Prompt = f.TextInput.Prompt.Foreground(t.Fuchsia)
		s.Blurred = s.Focused
		s.Blurred.Base = s.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
		s.Blurred.Card = s.Blurred.Base
		s.Blurred.NextIndicator = lipgloss.NewStyle()
		s.Blurred.PrevIndicator = lipgloss.NewStyle()
		s.Group.Title = s.Focused.Title
		s.Group.Description = s.Focused.Description
		s.Help = t.Help()
		return s
	})
}
