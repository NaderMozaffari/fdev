package logview

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Selecting logs: hold ctrl (or ⌥, or shift) and click a log, or
// right-click it. That selects it and opens a menu by the mouse with what
// can be done with it:
//
//	   ╭─ 1 log ─────────────╮
//	   │ ⧉ copy              │
//	   │ ⧉ copy as cURL      │
//	   │ ⚑ pin to the top    │
//	   │ ◉ track logs like it│
//	   ╰─────────────────────╯
//
// With the menu open, the same click with shift stretches the selection to
// another log. A click on an item does it and closes the menu; a click
// anywhere else, esc or the wheel just closes it. The actions have no keys,
// and a plain click on a log only opens or closes it.
//
// Hiding a log leaves its row, empty, with a button that shows it again:
// the logs around it stay where they were.

type selAction struct {
	icon, label, act string
}

// selActions are the actions for what is selected, in the menu's order.
func (m *Model) selActions() []selAction {
	e := m.sel
	one := m.anchor == nil || m.anchor == e
	acts := []selAction{{"⧉", "copy", "copy"}}
	if one && len(bodyOf(e)) > 0 {
		acts = append(acts, selAction{"⧉", "copy the body", "body"})
	}
	if one && e.Category == "network" {
		acts = append(acts, selAction{"⧉", "copy the URL", "url"}, selAction{"⧉", "copy as cURL", "curl"})
	}
	if one && e.Kind != KindTool || e.Category == "network" {
		acts = append(acts, selAction{"≡", "only logs like it", "only"}, selAction{"⊘", "hide logs like it", "hide"})
	}
	if one {
		if e.pinned {
			acts = append(acts, selAction{"⚐", "unpin", "pin"})
		} else {
			acts = append(acts, selAction{"⚑", "pin to the top", "pin"})
		}
	}
	if e.bookmarked {
		acts = append(acts, selAction{"☆", "remove the bookmark", "bookmark"})
	} else {
		acts = append(acts, selAction{"★", "bookmark", "bookmark"})
	}
	acts = append(acts, selAction{"◌", "hide", "hide1"})
	if one {
		if m.trackOf(e) != nil {
			acts = append(acts, selAction{"◎", "stop tracking", "track"})
		} else {
			acts = append(acts, selAction{"◉", "track logs like it", "track"})
		}
	}
	acts = append(acts, selAction{"⤓", "save…", "save"})
	if one {
		acts = append(acts, selAction{"▭", "open in a window", "open"})
		if e.At != "" {
			acts = append(acts, selAction{iconGlyph, "open the code", "code"})
		}
	}
	return acts
}

// shownEntries is what the logs show now, in order.
func (m *Model) shownEntries() []*Entry {
	var out []*Entry
	for _, e := range m.entries {
		if m.visible(e) && e.Level != "cleared" {
			out = append(out, e)
		}
	}
	return out
}

func indexOf(list []*Entry, e *Entry) int {
	for i, x := range list {
		if x == e {
			return i
		}
	}
	return -1
}

// selected is the selected logs, in order: the range between the anchor
// and the cursor, or the one log.
func (m *Model) selected() []*Entry {
	if m.sel == nil {
		return nil
	}
	if m.anchor == nil || m.anchor == m.sel {
		return []*Entry{m.sel}
	}
	shown := m.shownEntries()
	a, b := indexOf(shown, m.anchor), indexOf(shown, m.sel)
	if a < 0 || b < 0 {
		return []*Entry{m.sel}
	}
	if a > b {
		a, b = b, a
	}
	return shown[a : b+1]
}

// selectEntry selects e; extend stretches the selection to it instead.
func (m *Model) selectEntry(e *Entry, extend bool) {
	switch {
	case extend && m.sel != nil:
		if m.anchor == nil {
			m.anchor = m.sel
		}
	default:
		m.anchor = nil
	}
	m.sel = e
	m.dirty = true
}

// unselect closes the menu, and follows the output again if it did.
func (m *Model) unselect() {
	if m.sel != nil && m.menuFollow {
		m.follow, m.unseen = true, 0
	}
	m.sel, m.anchor, m.menuFollow = nil, nil, false
	m.dirty = true
	m.resize()
}

// picks is whether a click selects: one with ctrl, ⌥ or shift held, or a
// right click. Terminals keep some of these for themselves (⌥ and shift
// often select text), so any of them will do.
func picks(msg tea.MouseClickMsg) bool {
	return msg.Button == tea.MouseRight ||
		msg.Button == tea.MouseLeft && msg.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModShift|tea.ModMeta) != 0
}

// pick selects the log at x, y (extend stretches the selection to it) and
// opens the menu there. The logs stop following while it is open, so the
// log stays by the menu.
func (m *Model) pick(x, y int, extend bool) tea.Cmd {
	r, ok := m.rowAt(x, y)
	if !ok || r.entry.Level == "cleared" || r.entry.hidden {
		if m.sel != nil {
			m.unselect()
		}
		return nil
	}
	if m.sel == nil && m.follow {
		m.follow, m.menuFollow = false, true
	}
	m.selectEntry(r.entry, extend)
	m.menuX, m.menuY = x, y
	m.resize()
	return nil
}

// menuClick is a click while the menu is open.
func (m *Model) menuClick(msg tea.MouseClickMsg) tea.Cmd {
	if act, in := m.menuAt(msg.X, msg.Y); in {
		if act != "" && msg.Button == tea.MouseLeft {
			return m.selDo(act)
		}
		return nil
	}
	if picks(msg) {
		return m.pick(msg.X, msg.Y, msg.Mod.Contains(tea.ModShift))
	}
	m.unselect()
	return nil
}

// reveal scrolls so e is on the screen, its first row a little down from
// the top when it has to move.
func (m *Model) reveal(e *Entry) {
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
	if start < 0 {
		return
	}
	top, h := m.vp.YOffset(), m.vp.Height()
	switch {
	case start < top:
		m.vp.SetYOffset(start)
	case end > top+h:
		m.vp.SetYOffset(min(start, end-h))
	default:
		return
	}
	m.follow = m.vp.AtBottom()
	if m.follow {
		m.unseen = 0
	}
}

// jumpTo shows e, marked for a moment.
func (m *Model) jumpTo(e *Entry) tea.Cmd {
	if !m.visible(e) {
		m.setStatus("⊘ that log is hidden now: a filter or a toggle hides it")
		return nil
	}
	m.jumped = e
	m.reveal(e)
	m.lit, m.litAt = e, time.Now()
	m.dirty = true
	return m.startMotion()
}

// selDo does a menu item, for what is selected, and closes the menu.
func (m *Model) selDo(act string) tea.Cmd {
	e, all := m.sel, m.selected()
	m.unselect()
	switch act {
	case "copy":
		var b strings.Builder
		for _, x := range all {
			writeEntry(&b, x)
		}
		return m.copyText(strings.TrimRight(b.String(), "\n"), plural(len(all), "log"))
	case "body":
		return m.copyText(linesText(bodyOf(e)), "the body")
	case "url":
		return m.copyText(e.URL, "the URL")
	case "curl":
		return m.copyText(curl(e), "the call as cURL")
	case "only":
		m.setFilter(likeThis(e))
		m.setStatus("✓ only logs like it · esc or ✕ shows them all again")
	case "hide":
		m.setFilter(strings.TrimSpace(m.filter + " -" + likeThis(e)))
		m.setStatus("✓ logs like it are hidden · the filter shows how: / changes it")
	case "pin":
		m.togglePin(e)
	case "bookmark":
		on := !e.bookmarked
		for _, x := range all {
			x.bookmarked = on
			x.cached = cacheKey{}
		}
		if on {
			m.setStatus(fmt.Sprintf("★ bookmarked %s · [ and ] jump between bookmarks", plural(len(all), "log")))
		} else {
			m.setStatus("☆ bookmark removed")
		}
		m.dirty = true
	case "track":
		m.toggleTrack(e)
	case "hide1":
		for _, x := range all {
			x.hidden, x.cached = true, cacheKey{}
		}
		m.setStatus(fmt.Sprintf("◌ hid %s · a click on its row shows it again", plural(len(all), "log")))
	case "save":
		return m.openSave(all)
	case "open":
		m.openDialog(e)
	case "code":
		return m.open(e)
	}
	return nil
}

// ── Bookmarks ────────────────────────────────────────────────────────────────

// nextBookmark shows the next bookmarked log down (dir 1) or up (-1) from
// the one it showed last, or from an end.
func (m *Model) nextBookmark(dir int) tea.Cmd {
	shown := m.shownEntries()
	from := indexOf(shown, m.jumped)
	if from < 0 {
		from = len(shown)
		if dir > 0 {
			from = -1
		}
	}
	for i := from + dir; i >= 0 && i < len(shown); i += dir {
		if shown[i].bookmarked {
			return m.jumpTo(shown[i])
		}
	}
	n := 0
	for _, e := range m.entries {
		if e.bookmarked {
			n++
		}
	}
	if n == 0 {
		m.setStatus("⊘ no bookmarks yet · ctrl+click a log to bookmark it")
	} else {
		m.setStatus("⊘ no more bookmarks that way")
	}
	return nil
}

// ── Copying ──────────────────────────────────────────────────────────────────

// curl is a network call as a cURL command: a response's request when it
// was logged, with its headers and body.
func curl(e *Entry) string {
	req := e
	if e.Request != nil {
		req = e.Request
	}
	_, headers, body := splitDetails(req.Details)
	var b strings.Builder
	b.WriteString("curl")
	if m := req.method(); m != "" && m != "GET" {
		b.WriteString(" -X " + m)
	}
	b.WriteString(" " + shellWord(req.URL))
	for _, h := range headers {
		if k, v, ok := strings.Cut(h.Text, ": "); ok && !masked(v) {
			b.WriteString(" \\\n  -H " + shellWord(k+": "+v))
		}
	}
	if req.Level == "request" && len(body) > 0 {
		b.WriteString(" \\\n  --data-raw " + shellWord(compactJSON(linesText(body))))
	}
	return b.String()
}

// masked is whether a value was hidden by the app (fdev_log masks secrets
// as "••• (812 chars)"): not worth keeping.
func masked(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || strings.HasPrefix(v, "•") || strings.HasPrefix(v, "***") || v == "null"
}

func shellWord(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// compactJSON puts indented JSON back on one line; other text is as it is.
func compactJSON(s string) string {
	if n, ok := parseJSON([]byte(s)); ok {
		return compact(n)
	}
	return s
}

func compact(n *node) string {
	switch {
	case n.object:
		parts := make([]string, len(n.values))
		for i, v := range n.values {
			parts[i] = encode(n.keys[i]) + ":" + compact(v)
		}
		return "{" + strings.Join(parts, ",") + "}"
	case n.array:
		parts := make([]string, len(n.values))
		for i, v := range n.values {
			parts[i] = compact(v)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	return n.scalar
}

// ── Drawing ──────────────────────────────────────────────────────────────────

// selRows paints the rows of a selected log.
func (m *Model) selRows(rows []row, cursor bool) []row {
	bg := m.th.BarBg
	out := make([]row, len(rows))
	for i, r := range rows {
		mark := " "
		if cursor {
			mark = lipgloss.NewStyle().Foreground(m.th.Fuchsia).Render("▌")
		}
		r.text = paintBg(mark+ansi.TruncateLeft(r.text, 1, ""), m.width, bg)
		out[i] = r
	}
	return out
}

// paintBg puts a background behind a styled row, through its resets, to
// width.
func paintBg(text string, width int, bg color.Color) string {
	probe := lipgloss.NewStyle().Background(bg).Render("x")
	seq, _, _ := strings.Cut(probe, "x")
	if seq == "" {
		return text
	}
	t := resetSeq.ReplaceAllStringFunc(text, func(r string) string { return r + seq })
	return seq + t + strings.Repeat(" ", max(width-lipgloss.Width(text), 0)) + "\x1b[m"
}

// menuRect is where the menu is on the screen: under the clicked row, or
// above it when there is no room below, kept on the screen.
func (m *Model) menuRect(box []string) (x, y, w, h int) {
	w, h = lipgloss.Width(box[0]), len(box)
	x, y = m.menuX, m.menuY+1
	if y+h > m.height {
		y = m.menuY - h
	}
	return max(min(x, m.width-w), 0), max(min(y, m.height-h), 0), w, h
}

// menuBox is the menu's rows.
func (m *Model) menuBox() []string {
	th := m.th
	acts := m.selActions()
	title := " " + plural(len(m.selected()), "log") + " "
	inner := lipgloss.Width(title) + 2
	for _, a := range acts {
		inner = max(inner, lipgloss.Width(a.icon)+lipgloss.Width(a.label)+3)
	}
	border := lipgloss.NewStyle().Foreground(th.Fuchsia).Background(th.BarBg)
	head := lipgloss.NewStyle().Bold(true).Foreground(th.Pink).Background(th.BarBg)
	out := []string{border.Render("╭─") + head.Render(title) + border.Render(strings.Repeat("─", inner-1-lipgloss.Width(title))+"╮")}
	for _, a := range acts {
		item := lipgloss.NewStyle().Width(inner).Background(th.BarBg).Foreground(th.Normal)
		icon := lipgloss.NewStyle().Background(th.BarBg).Foreground(th.Pink)
		if m.hover == "menu:"+a.act {
			item = item.Background(th.Fuchsia).Foreground(th.Cream).Bold(true)
			icon = icon.Background(th.Fuchsia).Foreground(th.Cream)
		}
		text := icon.Render(" "+a.icon+" ") + item.UnsetWidth().Render(a.label)
		text += item.UnsetWidth().Render(strings.Repeat(" ", max(inner-lipgloss.Width(text), 0)))
		out = append(out, border.Render("│")+text+border.Render("│"))
	}
	return append(out, border.Render("╰"+strings.Repeat("─", inner)+"╯"))
}

// menuAt is the menu's item under the mouse, and whether the mouse is on
// the menu at all.
func (m *Model) menuAt(x, y int) (string, bool) {
	if m.sel == nil || m.overlay() {
		return "", false
	}
	x0, y0, w, h := m.menuRect(m.menuBox())
	if x < x0 || x >= x0+w || y < y0 || y >= y0+h {
		return "", false
	}
	if i := y - y0 - 1; x > x0 && x < x0+w-1 && i >= 0 && i < h-2 {
		return m.selActions()[i].act, true
	}
	return "", true
}

// withMenu draws the menu over the screen, while logs are selected.
func (m *Model) withMenu(screen string) string {
	if m.sel == nil || m.overlay() {
		return screen
	}
	box := m.menuBox()
	x, y, w, _ := m.menuRect(box)
	lines := strings.Split(screen, "\n")
	for i, b := range box {
		if y+i >= len(lines) {
			break
		}
		l := lines[y+i]
		left := ansi.Truncate(l, x, "")
		left += strings.Repeat(" ", max(x-lipgloss.Width(left), 0))
		lines[y+i] = left + "\x1b[m" + b + "\x1b[m" + ansi.TruncateLeft(l, x+w, "")
	}
	return strings.Join(lines, "\n")
}
