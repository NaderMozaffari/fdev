package logview

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/theme"
)

// Toggle is one key of the log viewer: a category of lines, or a part of
// each line (View).
type Toggle struct {
	Key, Name, Desc string
	Default         bool
	View            bool
}

var Toggles = []Toggle{
	{"1", "info", "info logs", true, false},
	{"2", "success", "success logs", true, false},
	{"3", "warning", "warnings", true, false},
	{"4", "error", "errors, fatal logs and uncaught exceptions", true, false},
	{"5", "debug", "debug logs and plain print()", true, false},
	{"6", "network", "HTTP requests and responses, one line each", true, false},
	{"7", "details", "headers and bodies under every network line", false, false},
	{"8", "native", "Android logs from other tags (EGL, ViewRootImpl, ...)", false, false},
	{"9", "raw", "the original output, I/flutter (pid) prefixes and all", false, false},
	{",", "time", "the time column", true, true},
	{".", "label", "the level column", true, true},
	{";", "tag", "the tag before the message", true, true},
}

const (
	maxStack    = 8
	maxBody     = 60     // rows of lines or body shown before "… more"
	maxHead     = 4      // rows of a long first line
	hintMark    = "\x00" // starts a "… more" row: a click there opens the window
	labelWidth  = 4      // DEBU INFO WARN ERRO FATA, as charmbracelet/log prints them
	iconGlyph   = "↗"
	expandGlyph = "▸"
)

type row struct {
	entry *Entry
	first bool
	more  bool // the "… more" row: a click opens the entry in a window
	text  string
}

// label is the level column, in charmbracelet/log's words and colors.
func label(th theme.Theme, level string) (string, color.Color) {
	switch level {
	case "debug", "V", "D":
		return "DEBU", th.Debug
	case "print":
		return "PRNT", th.Muted
	case "info", "I":
		return "INFO", th.Info
	case "success":
		return "DONE", th.Green
	case "warning", "W":
		return "WARN", th.Warn
	case "error", "E":
		return "ERRO", th.Error
	case "fatal", "F":
		return "FATA", th.Fatal
	case "request", "response":
		return strings.ToUpper(level[:min(len(level), labelWidth)]), phaseColor(th, level)
	case "fail":
		return "FAIL", th.Error
	case "value":
		return "VALU", th.Fuchsia
	}
	return strings.ToUpper(level), th.Muted
}

// phaseColor tells a request from its response: blue going out, pink
// coming back, red when it failed.
func phaseColor(th theme.Theme, level string) color.Color {
	switch level {
	case "request":
		return th.Indigo
	case "response":
		return th.Fuchsia
	}
	return th.Red
}

// phaseArrow is the direction of a network line, before its name.
func phaseArrow(level string) string {
	switch level {
	case "request":
		return "→"
	case "response":
		return "←"
	}
	return "✘"
}

var jsonToken = regexp.MustCompile(`("(?:\\.|[^"\\])*")(\s*:)?|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|true|false|null)`)

type painter struct{ th theme.Theme }

func (p painter) fg(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func (p painter) tone(t Tone) lipgloss.Style {
	switch t {
	case ToneDim:
		return p.fg(p.th.Muted)
	case ToneRed:
		return p.fg(p.th.Red)
	case ToneYellow:
		return p.fg(p.th.Warn)
	case ToneGreen:
		return p.fg(p.th.Green)
	case ToneBlue:
		return p.fg(p.th.Indigo)
	case ToneCyan:
		return p.fg(p.th.Info)
	case ToneMagenta:
		return p.fg(p.th.Fatal)
	case ToneAccent:
		return p.fg(p.th.Fuchsia)
	}
	return lipgloss.NewStyle()
}

func (p painter) line(l Line) string {
	switch l.Kind {
	case LineJSON:
		return jsonToken.ReplaceAllStringFunc(l.Text, func(tok string) string {
			m := jsonToken.FindStringSubmatch(tok)
			switch {
			case m[1] != "" && m[2] != "":
				return p.fg(p.th.Indigo).Render(m[1]) + m[2]
			case m[1] != "":
				return p.fg(p.th.Green).Render(m[1])
			}
			return p.fg(p.th.Pink).Render(m[3])
		})
	case LineHeader:
		key, value, _ := strings.Cut(l.Text, ": ")
		return p.fg(p.th.Muted).Render(key+"=") + value
	case LineFrame:
		if isLibraryFrame(l.Text) {
			return p.fg(p.th.Muted).Render(l.Text)
		}
		return l.Text
	case LineDim:
		return p.fg(p.th.Muted).Render(l.Text)
	case LineQuery:
		sep, rest := l.Text[:1], l.Text[1:]
		key, value, _ := strings.Cut(rest, "=")
		return p.fg(p.th.Muted).Render(sep+" ") + p.fg(p.th.Indigo).Render(key) +
			p.fg(p.th.Muted).Render(" = ") + value
	}
	return l.Text
}

// Frames of the SDK and of packages, dimmed so the app's own frames stand out.
func isLibraryFrame(s string) bool {
	for _, marker := range []string{"dart:", "package:flutter/", "<asynchronous", ".pub-cache", "/flutter/packages/"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

// wrap breaks s into rows of width, the first prefixed with first and the
// others with indent.
func wrap(s, first, indent string, width int) []string {
	avail := max(width-lipgloss.Width(first), 10)
	lines := strings.Split(ansi.Wrap(s, avail, ""), "\n")
	out := make([]string, len(lines))
	for i, l := range lines {
		if i == 0 {
			out[i] = first + l
		} else {
			out[i] = indent + l
		}
	}
	return out
}

type cacheKey struct {
	version, width  int
	expanded, valid bool
	marks           int // bookmarked, pinned, hidden
	frame           int // a waiting request's spinner
}

func (m *Model) entryRows(e *Entry, width int) []row {
	key := cacheKey{m.version, width, e.Expanded, true, marks(e), 0}
	if e.waiting() && !m.exited {
		key.frame = frame() + 1
	}
	if e.cached == key {
		return e.cache
	}
	texts := m.renderEntry(e, width)
	rows := make([]row, len(texts))
	for i, t := range texts {
		more := strings.HasPrefix(t, hintMark)
		rows[i] = row{entry: e, first: i == 0, more: more, text: strings.TrimPrefix(t, hintMark)}
	}
	e.cached, e.cache = key, rows
	return rows
}

func (m *Model) renderEntry(e *Entry, width int) []string {
	p := painter{m.th}
	muted := p.fg(m.th.Muted)

	if m.on["raw"] {
		var out []string
		for _, r := range e.Raw {
			out = append(out, wrap(r, "", "", width)...)
		}
		return out
	}
	if e.Level == "cleared" { // where the screen was cleared: a rule across it
		text := " " + strings.Trim(e.Text, "─ ") + " "
		side := max((width-lipgloss.Width(text))/2, 2)
		line := strings.Repeat("─", side) + text + strings.Repeat("─", max(width-side-lipgloss.Width(text), 2))
		return []string{p.fg(m.th.PinkDim).Render(ansi.Truncate(line, width, ""))}
	}
	c := m.cols()
	if e.Kind == KindTool && (!c.aligned || m.o.Plain) {
		if e.Level == "reload" {
			return wrap(p.fg(m.th.Green).Render("✓ "+e.Text), "", "  ", width)
		}
		return wrap(p.tone(e.TextTone).Render(e.Text), "", "", width)
	}

	// time LEVL ↗ tag message key=value, like charmbracelet/log; in the
	// columns and table layouts the tag has a column of its own.
	prefix := ""
	if c.timeW > 0 {
		prefix += muted.Render(e.Time.Format(c.timeFormat)) + c.sep
	}
	if c.labelW > 0 {
		if e.Kind == KindTool {
			prefix += strings.Repeat(" ", c.labelW) + c.sep
		} else {
			prefix += m.labelText(c, e.Level) + c.sep
		}
	}
	if e.At != "" {
		prefix += p.fg(m.th.Fuchsia).Render(iconGlyph) + c.sep
	} else {
		prefix += " " + c.sep
	}
	indent := c.blank()
	if !c.aligned {
		indent = strings.Repeat(" ", lipgloss.Width(prefix))
	}
	if e.hidden { // an empty row, and the button that shows the log again
		if c.tagW > 0 {
			prefix += strings.Repeat(" ", c.tagW) + c.sep
		}
		return []string{prefix + p.fg(m.th.Subtle).Render("┄┄ hidden ┄┄ ") +
			p.fg(m.th.Pink).Render("◌ show") + muted.Render(" · click")}
	}

	var text string
	switch {
	case e.Kind == KindTool:
		text = p.tone(e.TextTone).Render(e.Text)
		if e.Level == "reload" {
			text = p.fg(m.th.Green).Render("✓ " + e.Text)
		}
		if c.tagW > 0 {
			prefix += muted.Render(fit("flutter", c.tagW)) + c.sep
		}
	case e.Category == "network" && c.aligned:
		if c.tagW > 0 {
			prefix += lipgloss.NewStyle().Bold(true).Render(fit(e.method(), c.tagW)) + c.sep
		}
		text = m.networkColumns(e, c, width-lipgloss.Width(prefix)-7) // 7: the ▸ more
	case e.Category == "network":
		text = lipgloss.NewStyle().Bold(true).Render(e.method()) + " " + m.networkTitle(e)
		for _, f := range e.Fields {
			text += " " + muted.Render(f.Key+"=") + p.tone(f.Tone).Render(f.Value)
		}
		if e.Level == "request" {
			status, took, waiting := m.waitText(e)
			if waiting {
				text += " " + p.fg(m.th.Indigo).Bold(true).Render(status) + " " + p.fg(m.th.Indigo).Render(took)
			} else {
				text += " " + muted.Render("→ "+status+" "+took)
			}
		}
	default:
		text = p.tone(e.TextTone).Render(e.Text)
		tag := e.Tag
		if !m.on["tag"] {
			tag = ""
		}
		switch {
		case c.tagW > 0:
			style := lipgloss.NewStyle().Bold(true).Faint(e.Kind == KindNative)
			prefix += style.Render(fit(tag, c.tagW)) + c.sep
		case tag != "":
			text = lipgloss.NewStyle().Bold(true).Faint(e.Kind == KindNative).Render(tag+":") + " " + text
		}
		for _, f := range e.Fields {
			text += " " + muted.Render(f.Key+"=") + p.tone(f.Tone).Render(f.Value)
		}
	}
	if e.Category == "network" && e.Level != "request" {
		if keys := queryKeys(e.Details); keys != "" && !e.Expanded && !m.on["details"] {
			text += " " + muted.Render(ansi.Truncate("?"+keys, 28, "…"))
		}
	}
	var marked []string
	if e.bookmarked {
		marked = append(marked, p.fg(m.th.Warn).Render("★"))
	}
	if e.pinned {
		marked = append(marked, p.fg(m.th.Pink).Render("⚑"))
	}
	if m.trackOf(e) != nil {
		marked = append(marked, p.fg(m.th.Info).Render("◉"))
	}
	if len(marked) > 0 {
		text = strings.Join(marked, "") + " " + text
	}
	switch {
	case e.Category == "network" && len(e.Details) > 0 && !m.on["details"]:
		// The button that opens the call: its headers and body.
		if e.Expanded {
			text += " " + p.fg(m.th.Pink).Render("▾ less")
		} else {
			text += " " + p.fg(m.th.Pink).Render(expandGlyph+" more")
		}
	case (len(e.Details) > 0 && !m.on["details"]) || len(e.Stack) > maxStack:
		glyph := expandGlyph
		if e.Expanded {
			glyph = "▾"
		}
		text += " " + muted.Render(glyph)
	}

	// Folded, a long entry shows its first rows and how many more there
	// are; a click on that row opens it all in a window (dialog.go).
	folded := !e.Expanded
	out := wrap(text, prefix, indent, width)
	hidden := 0
	if folded && len(out) > maxHead {
		hidden = len(out) - maxHead
		out = out[:maxHead]
		out[maxHead-1] += muted.Render("…")
	}
	var rest []string
	for _, l := range e.Lines {
		rest = append(rest, wrap(p.line(l), indent, indent, width)...)
	}
	stack := e.Stack
	if folded && len(stack) > maxStack {
		hidden += len(stack) - maxStack
		stack = stack[:maxStack]
	}
	for _, l := range stack {
		rest = append(rest, wrap(p.line(l), indent, indent, width)...)
	}
	if m.on["details"] || e.Expanded {
		bar := p.fg(m.th.PinkDim).Render("┃ ")
		for _, l := range e.Details {
			rest = append(rest, wrap(p.line(l), indent+bar, indent+bar, width)...)
		}
	}
	if folded && len(rest) > maxBody {
		hidden += len(rest) - maxBody
		rest = rest[:maxBody]
	}
	out = append(out, rest...)
	if folded {
		e.clipped = hidden > 0
	}
	switch {
	case folded && hidden > 0:
		out = append(out, hintMark+indent+muted.Render(fmt.Sprintf("… %d more lines · ", hidden))+
			p.fg(m.th.Pink).Render("⤢ open")+muted.Render(" (e) · click the log to unfold"))
	case !folded && e.clipped:
		out = append(out, hintMark+indent+p.fg(m.th.Pink).Render("⤢ open in a window")+
			muted.Render(" (e) · click the log to fold"))
	}
	return out
}

// networkColumns is a network line in the aligned layouts: status, URL,
// time and size, each in its own column; other fields after them.
func (m *Model) networkColumns(e *Entry, c cols, avail int) string {
	p := painter{m.th}
	muted := p.fg(m.th.Muted)
	status := muted.Render(fit("···", statusW))
	var took, size string
	if e.Level == "request" {
		s, t, waiting := m.waitText(e)
		if waiting {
			status = p.fg(m.th.Indigo).Bold(true).Render(fit(" "+s, statusW))
		} else {
			status = muted.Render(fit(s, statusW))
		}
		if took = t; e.Response == nil && !waiting {
			took = ""
		}
	}
	var rest []string
	for _, f := range e.Fields {
		switch f.Key {
		case "status":
			status = p.tone(f.Tone).Bold(true).Render(fit(f.Value, statusW))
		case "took":
			took = f.Value
		case "size":
			size = f.Value
		default:
			rest = append(rest, muted.Render(f.Key+"=")+p.tone(f.Tone).Render(f.Value))
		}
	}
	if e.Level == "fail" && !strings.Contains(status, "0") {
		status = p.fg(m.th.Red).Bold(true).Render(fit("ERR", statusW))
	}
	sep, sepW := " ", 1
	if c.table {
		sep, sepW = c.sep, c.sepW
	}
	url := m.networkTitle(e)
	urlW := max(avail-statusW-tookW-sizeW-3*sepW, 12)
	if !e.Expanded {
		url = ansi.Truncate(url, urlW, "…")
	}
	line := status + sep + fit(url, urlW) + sep +
		muted.Render(fmt.Sprintf("%*s", tookW, took)) + sep + muted.Render(fmt.Sprintf("%*s", sizeW, size))
	if len(rest) > 0 {
		line += "  " + strings.Join(rest, " ")
	}
	return line
}

// networkTitle is the arrow, the call's name and its path: the name in the
// arrow's color, the path dimmed after it. Without a name, the path alone.
func (m *Model) networkTitle(e *Entry) string {
	col := phaseColor(m.th, e.Level)
	arrow := lipgloss.NewStyle().Foreground(col).Bold(true).Render(phaseArrow(e.Level))
	if e.Name == "" {
		return arrow + " " + e.path()
	}
	return arrow + " " + lipgloss.NewStyle().Foreground(col).Bold(true).Render(e.Name) + "  " +
		lipgloss.NewStyle().Foreground(m.th.Muted).Render(e.path())
}

// queryKeys is the names of a URL's query parameters, "type&page".
func queryKeys(lines []Line) string {
	var keys []string
	for _, l := range lines {
		if l.Kind == LineQuery {
			k, _, _ := strings.Cut(l.Text[1:], "=")
			keys = append(keys, k)
		}
	}
	return strings.Join(keys, "&")
}

func marks(e *Entry) int {
	n := 0
	for i, b := range []bool{e.bookmarked, e.pinned, e.hidden} {
		if b {
			n |= 1 << i
		}
	}
	return n
}

// The spinner of a request waiting for its response, by the clock.
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func frame() int { return int(time.Now().UnixMilli()/90) % len(spinFrames) }

func spinFrame() string { return spinFrames[frame()] }

// pendingMax is how long a request waits before it counts as unanswered.
const pendingMax = 5 * time.Minute

// waitText is a request's wait: the spinner and the time so far, or what
// came back.
func (m *Model) waitText(e *Entry) (status, took string, waiting bool) {
	if r := e.Response; r != nil {
		return r.status(), m.fieldValue(r, "took"), false
	}
	if !e.waiting() || m.exited || time.Since(e.Time) > pendingMax {
		return "⊘", "no reply", false
	}
	return spinFrame(), fmt.Sprintf("%.1fs", time.Since(e.Time).Seconds()), true
}

// fit pads or cuts s to exactly w cells.
func fit(s string, w int) string {
	if lipgloss.Width(s) > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}
