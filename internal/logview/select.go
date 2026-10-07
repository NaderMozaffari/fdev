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

// Selecting logs: a click on a log, or tab, selects one; ↑/↓ move the
// selection and shift+↑/↓ (or a shift+click) stretch it over a range.
// While logs are selected, the bar at the bottom is what can be done with
// them, each a button with its key:
//
//	▌ 3 logs  c copy  j body  u URL  C cURL  f only like this  F hide like this
//	p pin  b bookmark  h hide  t track  s save  o open  g code  esc
//
// The keys go to fdev while logs are selected; esc gives them back.
//
// Hiding a log (h) leaves its row, empty, with a button that shows it
// again: the logs around it stay where they were.

type selAction struct {
	key, label, act string
}

// selActions are the actions for what is selected, in the bar's order.
func (m *Model) selActions() []selAction {
	e := m.sel
	one := m.anchor == nil || m.anchor == e
	acts := []selAction{{"c", "copy", "copy"}}
	if one && len(bodyOf(e)) > 0 {
		acts = append(acts, selAction{"j", "body", "body"})
	}
	if one && e.Category == "network" {
		acts = append(acts, selAction{"u", "URL", "url"}, selAction{"C", "cURL", "curl"})
	}
	if one && e.Kind != KindTool || e.Category == "network" {
		acts = append(acts, selAction{"f", "only like this", "only"}, selAction{"F", "hide like this", "hide"})
	}
	pin, mark, track := "pin", "bookmark", "track"
	if e.pinned {
		pin = "unpin"
	}
	if e.bookmarked {
		mark = "unmark"
	}
	if m.trackOf(e) != nil {
		track = "untrack"
	}
	if one {
		acts = append(acts, selAction{"p", pin, "pin"})
	}
	acts = append(acts, selAction{"b", mark, "bookmark"}, selAction{"h", "hide", "hide1"})
	if one {
		acts = append(acts, selAction{"t", track, "track"})
	}
	acts = append(acts, selAction{"s", "save", "save"})
	if one {
		acts = append(acts, selAction{"o", "open", "open"})
		if e.At != "" {
			acts = append(acts, selAction{"g", "code", "code"})
		}
	}
	return append(acts, selAction{"esc", "done", "done"})
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

func (m *Model) unselect() {
	m.sel, m.anchor = nil, nil
	m.dirty = true
	m.resize()
}

// startSelecting selects the lowest log on the screen.
func (m *Model) startSelecting() {
	m.refresh()
	top := m.vp.YOffset()
	for i := min(top+m.vp.Height(), len(m.rows)) - 1; i >= top && i >= 0; i-- {
		if e := m.rows[i].entry; e != nil && e.Level != "cleared" {
			m.selectEntry(e, false)
			m.resize()
			return
		}
	}
	m.setStatus("⊘ no log on the screen to select")
}

// moveSel moves the selection by d logs; extend stretches it.
func (m *Model) moveSel(d int, extend bool) {
	shown := m.shownEntries()
	i := indexOf(shown, m.sel)
	if i < 0 {
		m.unselect()
		return
	}
	i = min(max(i+d, 0), len(shown)-1)
	m.selectEntry(shown[i], extend)
	m.reveal(shown[i])
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

// jumpTo selects e and shows it, marked for a moment.
func (m *Model) jumpTo(e *Entry) tea.Cmd {
	if !m.visible(e) {
		m.setStatus("⊘ that log is hidden now: a filter or a toggle hides it")
		return nil
	}
	m.selectEntry(e, false)
	m.resize()
	m.reveal(e)
	m.lit, m.litAt = e, time.Now()
	m.dirty = true
	return m.startMotion()
}

// selKey handles a key while logs are selected; ok is false for keys that
// aren't the selection's (they work as usual).
func (m *Model) selKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "up":
		m.moveSel(-1, false)
	case "down":
		m.moveSel(1, false)
	case "shift+up":
		m.moveSel(-1, true)
	case "shift+down":
		m.moveSel(1, true)
	case "home":
		m.moveSel(-len(m.entries), msg.Mod.Contains(tea.ModShift))
	case "end":
		m.moveSel(len(m.entries), msg.Mod.Contains(tea.ModShift))
	case "esc":
		m.unselect()
	case "enter", "space":
		if expandable(m.sel, m.on["details"]) || len(m.sel.Lines) > 0 {
			return m.toggleExpanded(m.sel), true
		}
	case "tab":
		m.moveSel(1, false)
	case "shift+tab":
		m.moveSel(-1, false)
	default:
		for _, a := range m.selActions() {
			if a.key == msg.Text && a.key != "esc" {
				return m.selDo(a.act), true
			}
		}
		return nil, false
	}
	return nil, true
}

// selDo does a selection action.
func (m *Model) selDo(act string) tea.Cmd {
	e, all := m.sel, m.selected()
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
		m.unselect()
	case "save":
		return m.openSave()
	case "open":
		m.openDialog(e)
	case "code":
		return m.open(e)
	case "done":
		m.unselect()
	}
	return nil
}

// ── Bookmarks ────────────────────────────────────────────────────────────────

// nextBookmark selects the next bookmarked log down (dir 1) or up (-1)
// from the selection, or from the screen.
func (m *Model) nextBookmark(dir int) tea.Cmd {
	shown := m.shownEntries()
	from := indexOf(shown, m.sel)
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
		m.setStatus("⊘ no bookmarks yet · select a log and press b")
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

// selBar is the bar of actions for the selection, in place of the help.
func (m *Model) selBar() (string, []barItem) {
	th := m.th
	n := len(m.selected())
	head := lipgloss.NewStyle().Foreground(th.Fuchsia).Render("▌") +
		lipgloss.NewStyle().Bold(true).Foreground(th.Pink).Render(plural(n, "log")+" selected")
	if n == 1 {
		head += lipgloss.NewStyle().Foreground(th.Muted).Render(" · shift+↑/↓ more")
	}
	x := 1 + lipgloss.Width(head) + 2
	line := head + "  "
	var items []barItem
	keyStyle := lipgloss.NewStyle().Foreground(th.Pink).Bold(true)
	for _, a := range m.selActions() {
		text := keyStyle.Render(a.key) + " " + a.label
		style := lipgloss.NewStyle().Padding(0, 1).Background(th.BarBg).Foreground(th.Normal)
		if m.hover == "sel:"+a.act {
			style = style.Underline(true).Foreground(th.Pink)
		}
		chip := style.Render(text)
		w := lipgloss.Width(chip)
		if x+w > m.width {
			break
		}
		items = append(items, barItem{name: "sel:" + a.act, text: chip, x: x, width: w})
		line += chip + " "
		x += w + 1
	}
	return ansi.Truncate(" "+line, m.width, "…"), items
}

// selBarAt is the action under the mouse in the selection bar.
func (m *Model) selBarAt(x, y int) string {
	if m.sel == nil || y != m.height-1 {
		return ""
	}
	_, items := m.selBar()
	for _, it := range items {
		if x >= it.x && x < it.x+it.width {
			return it.name
		}
	}
	return ""
}
