package launcher

import (
	"fmt"
	"image/color"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/icon"
	"github.com/NaderMozaffari/fdev/internal/sprite"
)

const animFor = 260 * time.Millisecond

// rowIconW is a menu row's icon width: 6×3 cells (6×6 pixels) on three
// lines, 4×2 on two and a 3-cell tile on one.
func rowIconW(rowH int) int {
	switch rowH {
	case 3:
		return 6
	case 2:
		return 4
	}
	return 3
}

type animMsg struct{}

func animTick() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return animMsg{} })
}

// anim is how far the selection's wipe-in is, 0 to 1 (eased).
func (m *Model) anim() float64 {
	t := float64(time.Since(m.animStart)) / float64(animFor)
	if t >= 1 {
		return 1
	}
	return 1 - math.Pow(1-t, 3)
}

// menuDelegate draws the menu's rows: icon, title, description, details
// (fewer lines for a smaller size); the selected one on a gradient that
// wipes in from the left.
type menuDelegate struct{ m *Model }

func (d menuDelegate) Height() int                         { return max(d.m.menuRowH, 1) }
func (d menuDelegate) Spacing() int                        { return d.m.menuGap }
func (d menuDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d menuDelegate) Render(w io.Writer, l list.Model, index int, li list.Item) {
	it, ok := li.(item)
	if !ok {
		return
	}
	fmt.Fprint(w, d.m.renderRow(it, index == l.Index(), l.Width()))
}

func (m *Model) itemTarget(it item) *config.Target {
	if it.recent != nil {
		return it.recent.target
	}
	if it.session != nil {
		return m.cfg.Target(it.session.Target)
	}
	return it.target
}

// rowIcon is the flavor's app icon, or a tile with the section's sign (a
// star for starred logs), rowH lines high.
func (m *Model) rowIcon(it item, rowH int) []string {
	th := m.th
	if it.session != nil {
		c, sign := th.Subtle, "☰"
		if it.session.Starred {
			c, sign = sectionColor(logsTab), "★"
		}
		return tile(c, th.Cream, sign, rowH)
	}
	if t := m.itemTarget(it); t != nil {
		if rowH > 1 {
			if lines := icon.Render(m.cfg.FlavorIcon(t.Flavor), rowIconW(rowH), rowH); lines != nil {
				return lines
			}
		}
		c := sectionColor(t.Group)
		if t.Flavor != "" && rowH == 1 { // too small for the app icon: its color
			c = m.flavorColor(t.Flavor)
		}
		return tile(c, th.Cream, tabSign(t.Group), rowH)
	}
	return tile(th.Purple, th.Cream, "✦", rowH)
}

// tile is a colored square of rowH lines with sign in its middle.
func tile(bg, fg color.Color, sign string, rowH int) []string {
	s := lipgloss.NewStyle().Background(bg).Foreground(fg).Bold(true)
	w := rowIconW(rowH)
	blank := s.Render(strings.Repeat(" ", w))
	mid := s.Render(fit(" "+sign, w))
	switch rowH {
	case 1:
		return []string{mid}
	case 2:
		return []string{mid, blank}
	}
	return []string{blank, s.Render(fit("  "+sign, w)), blank}
}

// fit pads or cuts s to w cells.
func fit(s string, w int) string {
	if lipgloss.Width(s) > w {
		return ansi.Truncate(s, w, "")
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

type span struct {
	text  string
	style lipgloss.Style
}

// paint draws spans across width cells, on the selection gradient when
// selected (revealed up to reveal of the width).
func (m *Model) paint(spans []span, width int, selected bool, reveal float64) string {
	var bgs []color.Color
	if selected {
		bgs = lipgloss.Blend1D(max(width, 2), m.selFrom(), m.selTo())
	}
	shown := int(float64(width) * reveal)
	var b strings.Builder
	x := 0
	put := func(g string, style lipgloss.Style, w int) {
		if selected && x < shown {
			style = style.Background(bgs[min(x, len(bgs)-1)])
		}
		b.WriteString(style.Render(g))
		x += w
	}
	for _, s := range spans {
		gr := uniseg.NewGraphemes(s.text)
		for gr.Next() && x < width {
			w := gr.Width()
			if x+w > width {
				break
			}
			put(gr.Str(), s.style, w)
		}
	}
	for x < width {
		put(" ", lipgloss.NewStyle(), 1)
	}
	return b.String()
}

func (m *Model) selFrom() color.Color { return m.th.SelFrom }

func (m *Model) selTo() color.Color { return m.th.SelTo }

// rowDetails is a row's third line: what it asks, when it last ran, or a
// saved log's size.
func (m *Model) rowDetails(it item) string {
	switch {
	case it.session != nil:
		parts := []string{size(it.session.Size)}
		if it.session.Raw == "" {
			parts = append(parts, "text only")
		}
		return strings.Join(parts, " · ")
	case it.recent != nil:
		return it.recent.target.Desc
	case it.target != nil:
		var parts []string
		if asks := m.askNames(it.target); asks != "" {
			parts = append(parts, "asks "+asks)
		}
		if last := m.lastRun(it.target.Name); last != "" {
			parts = append(parts, "↺ "+last)
		}
		return strings.Join(parts, " · ")
	}
	return ""
}

func (m *Model) renderRow(it item, selected bool, width int) string {
	th := m.th
	bar := " "
	title := lipgloss.NewStyle().Foreground(th.Normal).Bold(true)
	desc := lipgloss.NewStyle().Foreground(th.Muted)
	detail := lipgloss.NewStyle().Foreground(th.Subtle)
	if it.session != nil && it.session.Starred {
		title = title.Foreground(sectionColor(logsTab))
	}
	if selected {
		bar = lipgloss.NewStyle().Foreground(th.Fuchsia).Render("▌")
		title = title.Foreground(th.Pink)
		desc = desc.Foreground(th.PinkDim)
		detail = detail.Foreground(th.Muted)
	}
	rowH := max(m.menuRowH, 1)
	icons := m.rowIcon(it, rowH)
	iconW := rowIconW(rowH)
	rest := max(width-iconW-3, 10)
	reveal := 1.0
	if selected {
		reveal = m.anim()
	}

	descText := it.desc
	if t := it.target; t != nil {
		parts := []string{}
		if p := t.PlatformName(); p != "" {
			parts = append(parts, p)
		}
		if t.Desc != "" {
			parts = append(parts, t.Desc)
		}
		descText = strings.Join(parts, " · ")
	}
	var texts []string
	switch rowH {
	case 1: // the title, and the description after it
		texts = []string{m.paint([]span{{" ", title}, {it.title, title}, {"  ", desc}, {descText, desc}}, rest, selected, reveal)}
	case 2:
		texts = []string{
			m.paint([]span{{" ", title}, {it.title, title}}, rest, selected, reveal),
			m.paint([]span{{" ", desc}, {descText, desc}}, rest, selected, reveal),
		}
	default:
		texts = []string{
			m.paint([]span{{" ", title}, {it.title, title}}, rest, selected, reveal),
			m.paint([]span{{" ", desc}, {descText, desc}}, rest, selected, reveal),
			m.paint([]span{{" ", detail}, {m.rowDetails(it), detail}}, rest, selected, reveal),
		}
	}
	lines := make([]string, rowH)
	for i := range lines {
		ic := strings.Repeat(" ", iconW)
		if i < len(icons) {
			ic = icons[i]
		}
		lines[i] = bar + ic + texts[i]
	}
	return strings.Join(lines, "\n")
}

func size(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// ── The details panel ────────────────────────────────────────────────────────

func (m *Model) panel(width, height int) string {
	it, ok := m.list.SelectedItem().(item)
	if !ok || width < 30 {
		return ""
	}
	th := m.th
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	inner := width - 4

	t := m.itemTarget(it)
	accent := sectionColor(m.tab)
	if t != nil && t.Flavor != "" {
		accent = m.flavorColor(t.Flavor)
	}
	var head []string
	head = append(head, lipgloss.NewStyle().Foreground(th.Pink).Bold(true).Render(it.title))
	var badges []string
	if t != nil {
		if p := t.PlatformName(); p != "" {
			badges = append(badges, th.Badge(th.Cream, platformColor(p)).Render(p))
		}
		if t.Flavor != "" {
			badges = append(badges, th.Badge(th.Cream, m.flavorColor(t.Flavor)).Render(t.Flavor))
		}
		if t.Group != "" {
			badges = append(badges, th.Badge(th.Cream, sectionColor(t.Group)).Render(t.Group))
		}
		if t.Logs {
			badges = append(badges, th.Badge(th.Normal, th.BarBg).Render("logs"))
		}
	}
	if len(badges) > 0 {
		head = append(head, strings.Join(badges, " "))
	}
	desc := it.desc
	if t != nil && it.recent == nil && it.session == nil {
		desc = t.Desc
	}

	// The app icon, as big as there is room for.
	iconW, iconH := 28, 14
	switch {
	case height < 24 || inner < 46:
		iconW, iconH = 12, 6
	case height < 34 || inner < 60:
		iconW, iconH = 20, 10
	}
	var art []string
	if t != nil {
		art = icon.Render(m.cfg.FlavorIcon(t.Flavor), iconW, iconH)
	}
	if art == nil {
		switch {
		case it.session != nil && it.session.Starred:
			art = sprite.Render("star", sectionColor(logsTab), 1, m.shine())
		case it.session != nil:
			art = sprite.Render("logs", th.Muted, 1, m.shine())
		case t != nil:
			art = sprite.Render(sprite.For(t.Group), sectionColor(t.Group), 1, m.shine())
		}
	}
	textW, artW := inner, 0
	if art != nil {
		artW = lipgloss.Width(art[0])
		if artW < 20 {
			textW = inner - artW - 2
		}
	}
	if desc != "" {
		head = append(head, "")
		head = append(head, strings.Split(ansi.Wrap(desc, max(textW, 10), ""), "\n")...)
	}
	if v := m.cfg.ShortVersion(); v != "" && t != nil && t.Flavor != "" {
		head = append(head, "", muted.Render(v))
	}
	top := strings.Join(head, "\n")
	if art != nil {
		if artW >= 20 { // a big icon goes above the text
			top = lipgloss.PlaceHorizontal(inner, lipgloss.Center, strings.Join(art, "\n")) + "\n\n" + top
		} else {
			top = lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(art, "\n"), "  ", top)
		}
	}

	// Facts: the flavor's, then what the target runs and asks; or the
	// saved log's.
	var facts []config.Fact
	switch {
	case it.session != nil:
		s := it.session
		if s.Subject != "" {
			facts = append(facts, config.Fact{Key: "Subject", Value: s.Subject})
		}
		if len(s.Tags) > 0 {
			facts = append(facts, config.Fact{Key: "Tags", Value: "#" + strings.Join(s.Tags, " #")})
		}
		facts = append(facts, config.Fact{Key: "Ran", Value: s.Time.Format("Mon 2 Jan 2006, 15:04")})
		if s.Command != "" {
			facts = append(facts, config.Fact{Key: "Command", Value: s.Command})
		}
		if s.Note != "" {
			facts = append(facts, config.Fact{Key: "Device", Value: s.Note})
		}
		var files []string
		for _, p := range []string{s.Log, s.Raw} {
			if p != "" {
				files = append(files, relative(m.cfg.Root, p))
			}
		}
		star := "no · space stars it; starred logs are never cleaned up"
		if s.Starred {
			star = "★ yes · kept when old logs are cleaned up"
		}
		facts = append(facts,
			config.Fact{Key: "Files", Value: strings.Join(files, "\n")},
			config.Fact{Key: "Size", Value: size(s.Size)},
			config.Fact{Key: "Starred", Value: star},
			config.Fact{Key: "Keys", Value: "→ view · space star · x delete · o open in editor"})
	case t != nil:
		if f := m.cfg.Flavors[t.Flavor]; f != nil {
			facts = append(facts, f.Info...)
		}
		facts = append(facts, config.Fact{Key: "Runs", Value: t.Run})
		if asks := m.askNames(t); asks != "" {
			facts = append(facts, config.Fact{Key: "Asks", Value: asks})
		}
		if last := m.lastRun(t.Name); last != "" {
			facts = append(facts, config.Fact{Key: "Last run", Value: last})
		}
	}
	keyW := 0
	for _, f := range facts {
		keyW = max(keyW, lipgloss.Width(f.Key))
	}
	var rows []string
	for _, f := range facts {
		var lines []string
		for _, v := range strings.Split(f.Value, "\n") {
			lines = append(lines, strings.Split(ansi.Wrap(v, max(inner-keyW-2, 10), ""), "\n")...)
		}
		style := lipgloss.NewStyle()
		switch {
		case f.Key == "Runs" || f.Key == "Command":
			style = style.Foreground(th.FilterKey)
		case f.Key == "Starred" && it.session != nil && it.session.Starred:
			style = style.Foreground(sectionColor(logsTab))
		}
		for i, l := range lines {
			k := ""
			if i == 0 {
				k = f.Key
			}
			rows = append(rows, muted.Render(fmt.Sprintf("%-*s", keyW, k))+"  "+style.Render(l))
		}
	}
	body := top
	if len(rows) > 0 {
		rule := lipgloss.NewStyle().Foreground(th.Subtle).Render(strings.Repeat("─", inner))
		body += "\n\n" + rule + "\n" + strings.Join(rows, "\n")
	}

	// The border lights up in the flavor's color as the selection lands.
	border := lipgloss.Blend1D(10, th.Subtle, accent)[min(int(m.anim()*9), 9)]
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).Width(width).MaxHeight(height).Render(body)
}

func relative(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

func (m *Model) askNames(t *config.Target) string {
	var names []string
	for _, a := range t.Ask {
		if p, ok := config.DeviceAsk(a); ok {
			names = append(names, devicesWord(p, "device"))
			continue
		}
		ask := m.cfg.Asks[a]
		name := a
		if ask != nil && ask.Title != "" {
			name = ask.Title
		}
		if last, ok := m.state.Answers[a]; ok && ask != nil {
			for _, o := range ask.Options {
				if o.Value == last {
					name += " (" + o.Label + " last time)"
				}
			}
		}
		names = append(names, name)
	}
	return strings.Join(names, " · ")
}

func (m *Model) lastRun(target string) string {
	for _, r := range m.state.Recent {
		if r.Target != target {
			continue
		}
		parts := []string{ago(r.At)}
		for k, v := range r.Env {
			if v != "" && k != config.DeviceEnv {
				parts = append(parts, k+"="+v)
			}
		}
		if r.Note != "" {
			parts = append(parts, r.Note)
		}
		return strings.Join(parts, " · ")
	}
	return ""
}
