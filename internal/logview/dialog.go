package logview

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// dialog shows one entry, all of it, in a window over the logs: for a
// response body or a print too long to read folded in the list.
type dialog struct {
	entry  *Entry
	offset int      // the first content row shown
	lines  []string // the content, wrapped for width
	width  int
	ver    int
}

func (m *Model) openDialog(e *Entry) {
	m.dialog = &dialog{entry: e}
	m.dirty = true
}

func (m *Model) closeDialog() {
	m.dialog = nil
	m.dirty = true
}

// openLongest opens the lowest long entry on the screen.
func (m *Model) openLongest() {
	m.refresh()
	top := m.vp.YOffset()
	for i := min(top+m.vp.Height(), len(m.rows)) - 1; i >= top && i >= 0; i-- {
		if e := m.rows[i].entry; e != nil && (e.clipped || expandable(e, m.on["details"])) {
			m.openDialog(e)
			return
		}
	}
	m.setStatus("⊘ no long log on the screen")
}

// dialogContent is the entry without the list's columns: the line, where
// it was logged, its lines and stack, then headers and body.
func (m *Model) dialogContent(width int) []string {
	e := m.dialog.entry
	p := painter{m.th}
	muted := p.fg(m.th.Muted)
	name, col := label(m.th, e.Level)
	head := lipgloss.NewStyle().Bold(true).Foreground(col).Render(name) + " " + muted.Render(e.Time.Format("15:04:05.000")) + "  "
	if e.Tag != "" {
		head += lipgloss.NewStyle().Bold(true).Render(e.Tag+":") + " "
	}
	head += p.tone(e.TextTone).Render(e.Text)
	for _, f := range e.Fields {
		head += " " + muted.Render(f.Key+"=") + p.tone(f.Tone).Render(f.Value)
	}
	out := wrap(head, "", "", width)
	if e.At != "" {
		out = append(out, p.fg(m.th.Fuchsia).Render(iconGlyph+" ")+muted.Render(e.At))
	}
	section := func(title string, lines []Line) {
		if len(lines) == 0 {
			return
		}
		out = append(out, "", p.fg(m.th.Pink).Bold(true).Render(title))
		for _, l := range lines {
			out = append(out, wrap(p.line(l), "", "", width)...)
		}
	}
	if e.Category == "network" {
		// A response with its request: the request first, what was sent.
		if req := e.Request; req != nil {
			query, headers, body := splitDetails(req.Details)
			section("→ Request query", append(queryOf(req.Lines), query...))
			section("→ Request headers", headers)
			section("→ Request body", body)
		}
		query, headers, body := splitDetails(e.Details)
		in := ""
		if e.Request != nil {
			in = "← Response "
		}
		section("Query", append(queryOf(e.Lines), query...))
		section(in+"Headers", headers)
		section(in+"Body", body)
		if e.waiting() && !m.exited {
			out = append(out, "", p.fg(m.th.Pink).Render(fmt.Sprintf("⋯ waiting for the response · %s so far", short(time.Since(e.Time)))))
		} else if r := e.Response; r != nil {
			out = append(out, "", muted.Render("← response: ")+r.status()+muted.Render(" · ")+m.fieldValue(r, "took"))
		}
		return out
	}
	section("", e.Lines)
	section("Stack", e.Stack)
	return out
}

func (m *Model) fieldValue(e *Entry, key string) string {
	for _, f := range e.Fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

// queryOf is the query lines among a request's lines.
func queryOf(lines []Line) []Line {
	var out []Line
	for _, l := range lines {
		if l.Kind == LineQuery {
			out = append(out, l)
		}
	}
	return out
}

// splitDetails parts a network entry's details into the URL's query, the
// headers and the body.
func splitDetails(details []Line) (query, headers, body []Line) {
	i := 0
	for i < len(details) && details[i].Kind == LineQuery {
		i++
	}
	j := i
	for j < len(details) && details[j].Kind == LineHeader {
		j++
	}
	return details[:i], details[i:j], details[j:]
}

func (m *Model) dialogView(height int) string {
	d := m.dialog
	inner := max(m.width-4, 10) // the border and a space each side
	if d.lines == nil || d.width != inner || d.ver != m.version {
		d.lines, d.width, d.ver = m.dialogContent(inner), inner, m.version
	}
	rows := max(height-3, 1) // the borders and the hint row
	d.offset = max(min(d.offset, len(d.lines)-rows), 0)
	shown := d.lines[d.offset:min(d.offset+rows, len(d.lines))]

	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	pos := "all"
	if len(d.lines) > rows {
		pos = fmt.Sprintf("%d–%d of %d", d.offset+1, d.offset+len(shown), len(d.lines))
	}
	hint := muted.Render("↑/↓ pgup/pgdn home/end scroll · c copy · esc close")
	hint = ansi.Truncate(hint, inner-lipgloss.Width(pos)-2, "…")
	hint += strings.Repeat(" ", max(inner-lipgloss.Width(hint)-lipgloss.Width(pos), 1)) + muted.Render(pos)

	body := make([]string, 0, rows+1)
	for _, l := range shown {
		body = append(body, ansi.Truncate(l, inner, ""))
	}
	for len(body) < rows {
		body = append(body, "")
	}
	body = append(body, hint)

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(m.th.Fuchsia).
		Padding(0, 1).Width(m.width).Render(strings.Join(body, "\n"))
	// The title and the close button in the top border.
	lines := strings.Split(box, "\n")
	title := " " + m.dialogTitle() + " "
	title = ansi.Truncate(title, max(m.width-12, 4), "…")
	closeBtn := lipgloss.NewStyle().Foreground(m.th.Pink).Render(" ✕ ")
	border := lipgloss.NewStyle().Foreground(m.th.Fuchsia)
	fill := max(m.width-3-lipgloss.Width(title)-lipgloss.Width(closeBtn)-1, 0)
	lines[0] = border.Render("╭─") + lipgloss.NewStyle().Bold(true).Render(title) +
		border.Render(strings.Repeat("─", fill)) + closeBtn + border.Render("╮")
	return strings.Join(lines, "\n")
}

func (m *Model) dialogTitle() string {
	e := m.dialog.entry
	if e.Category == "network" {
		return e.Text
	}
	name, _ := label(m.th, e.Level)
	if e.Tag != "" {
		return name + " · " + e.Tag
	}
	return name
}

func (m *Model) dialogScroll(n int) {
	m.dialog.offset = max(m.dialog.offset+n, 0) // dialogView keeps it in range
}

func (m *Model) dialogKey(msg tea.KeyPressMsg) tea.Cmd {
	page := max(m.logHeight()-4, 1)
	switch msg.String() {
	case "esc", "q", "e", "ctrl+c", "enter":
		m.closeDialog()
	case "up", "k":
		m.dialogScroll(-1)
	case "down", "j":
		m.dialogScroll(1)
	case "pgup", "b":
		m.dialogScroll(-page)
	case "pgdown", "space", "f":
		m.dialogScroll(page)
	case "home", "g":
		m.dialog.offset = 0
	case "end", "G":
		m.dialog.offset = len(m.dialog.lines)
	case "c", "y":
		return m.copyEntry(m.dialog.entry)
	}
	return nil
}

func (m *Model) dialogWheel(b tea.MouseButton) {
	switch b {
	case tea.MouseWheelUp:
		m.dialogScroll(-3)
	case tea.MouseWheelDown:
		m.dialogScroll(3)
	}
}

// dialogClick closes the window on its ✕.
func (m *Model) dialogClick(x, y int) tea.Cmd {
	if y == 1 && x >= m.width-5 && x < m.width-1 { // row 0 is the header
		m.closeDialog()
	}
	return nil
}

// copyEntry copies the body of a network entry, or the whole entry, as
// plain text: with the system's clipboard tool, else through the terminal
// (OSC 52, which not every terminal supports).
func (m *Model) copyEntry(e *Entry) tea.Cmd {
	if body := bodyOf(e); len(body) > 0 {
		return m.copyText(linesText(body), "the body")
	}
	lines := []Line{{e.Text, LinePlain}}
	lines = append(append(lines, e.Lines...), e.Stack...)
	return m.copyText(linesText(lines), "the log")
}

// copyText copies text, what says what it is: with the system's clipboard
// tool, else through the terminal (OSC 52, which not every terminal
// supports).
func (m *Model) copyText(text, what string) tea.Cmd {
	text = ansi.Strip(text)
	lines := strings.Count(text, "\n") + 1
	size := ""
	if lines > 1 {
		size = " (" + plural(lines, "line") + ")"
	}
	if name, args := clipboardTool(); name != "" {
		cmd := exec.Command(name, args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			m.setStatus("✓ copied " + what + size)
			return nil
		}
	}
	m.setStatus("✓ sent " + what + " to the terminal's clipboard" + size)
	return tea.SetClipboard(text)
}

func clipboardTool() (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy", nil
	case "windows":
		return "clip", nil
	}
	for _, t := range [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}} {
		if _, err := exec.LookPath(t[0]); err == nil {
			return t[0], t[1:]
		}
	}
	return "", nil
}

// bodyOf is what "copy body" copies: a network body, or the JSON a log
// printed.
func bodyOf(e *Entry) []Line {
	if e == nil {
		return nil
	}
	if _, _, body := splitDetails(e.Details); len(body) > 0 {
		return body
	}
	if n := len(e.Lines); n == 0 || e.Lines[n-1].Kind != LineJSON {
		return nil
	}
	// The value's first line ends with its bracket, after what came before
	// it ("user: {").
	var out []Line
	started := false
	if t := strings.TrimRight(e.Text, " "); e.Lines[0].Kind == LineJSON && (strings.HasSuffix(t, "{") || strings.HasSuffix(t, "[")) {
		out, started = append(out, Line{t[len(t)-1:], LineJSON}), true
	}
	for _, l := range e.Lines {
		switch {
		case l.Kind != LineJSON:
		case !started:
			t := strings.TrimRight(l.Text, " ")
			out, started = append(out, Line{t[len(t)-1:], LineJSON}), true
		default:
			out = append(out, l)
		}
	}
	return out
}

func linesText(lines []Line) string {
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = l.Text
	}
	return ansi.Strip(strings.Join(texts, "\n"))
}
