package launcher

import (
	"image/color"
	"strings"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/version"
)

// The terminal's title, e.g. its tab in VS Code:
//
//	🟡 fdev • Acme Shop v2.1.39 (439) • dev • SM-S918B • bazaar
//
// The circle is the flavor's color, as near as an emoji gets: titles have
// no colors of their own. A run that ended says how after it. fdev's own
// version is by its name while the header's shows it (version.Shown).
const titleSep = " • "

func (m *Model) windowTitle() string {
	parts := []string{version.Name(), appTitle(m.cfg.Title, m.cfg.ShortVersion())}
	if m.screen != screenLogs || m.logs == nil {
		return strings.Join(parts, titleSep)
	}
	t := m.ran.target
	if t == nil { // a saved log
		return "📜 " + strings.Join(append(parts, "saved log"), titleSep)
	}
	flavor := t.Flavor
	if flavor == "" {
		flavor = t.Name
	}
	parts = append(parts, flavor)
	if m.ran.note != "" {
		parts = append(parts, m.ran.note)
	}
	for _, name := range t.Ask {
		if _, ok := config.DeviceAsk(name); ok {
			continue
		}
		if ask := m.cfg.Asks[name]; ask != nil && m.ran.env[ask.Env] != "" {
			parts = append(parts, m.ran.env[ask.Env])
		}
	}
	title := colorDot(m.flavorColor(flavor)) + " " + strings.Join(parts, titleSep)
	if exited, ok := m.logs.Status(); exited {
		if ok {
			return title + titleSep + "✓"
		}
		return title + titleSep + "✘"
	}
	return title
}

// appTitle is the app's name and version, for the terminal's title.
func appTitle(name, v string) string {
	if v == "" {
		return name
	}
	return name + " " + v
}

// dots are the colored circle emoji, by the color each is drawn in.
var dots = []struct {
	r, g, b int
	dot     string
}{
	{0xE5, 0x39, 0x35, "🔴"},
	{0xF5, 0x7C, 0x00, "🟠"},
	{0xE8, 0xB4, 0x00, "🟡"},
	{0x43, 0xA0, 0x47, "🟢"},
	{0x21, 0x96, 0xF3, "🔵"},
	{0x7D, 0x56, 0xF4, "🟣"},
	{0x79, 0x55, 0x48, "🟤"},
	{0x30, 0x30, 0x30, "⚫"},
	{0xEE, 0xEE, 0xEE, "⚪"},
}

// colorDot is the circle emoji nearest to c.
func colorDot(c color.Color) string {
	r, g, b, _ := c.RGBA()
	best, dist := "", -1
	for _, d := range dots {
		dr, dg, db := int(r>>8)-d.r, int(g>>8)-d.g, int(b>>8)-d.b
		if n := dr*dr + dg*dg + db*db; dist < 0 || n < dist {
			best, dist = d.dot, n
		}
	}
	return best
}
