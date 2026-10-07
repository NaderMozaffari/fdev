package logview

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Kind is where a line came from.
type Kind int

const (
	KindTool   Kind = iota // flutter's own output: always shown
	KindApp                // the app's logs (Dart print, fdev records)
	KindNative             // other Android log tags
)

// Tone is a color role, resolved against the theme when rendering.
type Tone int

const (
	ToneNone Tone = iota
	ToneDim
	ToneRed
	ToneYellow
	ToneGreen
	ToneBlue
	ToneCyan
	ToneMagenta
	ToneAccent
)

// LineKind says how a continuation line is drawn.
type LineKind int

const (
	LinePlain  LineKind = iota
	LineJSON            // colored as JSON
	LineHeader          // "key: value"
	LineFrame           // a stack frame; dimmed unless it is app code
	LineDim
	LineQuery // a URL's query parameter: "?key=value", then "&key=value"
)

type Line struct {
	Text string
	Kind LineKind
}

// Field is a key=value after the message, as charmbracelet/log prints them.
type Field struct {
	Key, Value string
	Tone       Tone
}

// Entry is one log line with what belongs to it (continuation lines, stack
// trace, network headers and body).
type Entry struct {
	Kind     Kind
	Category string // a Toggles name; "" for tool lines
	// Level picks the label: debug, print, info, success, warning, error,
	// fatal, request, response, fail, a logcat letter (V D I W E F) for
	// native lines, or reload for flutter's reload messages.
	Level    string
	Tag      string
	Text     string // the first line
	TextTone Tone
	Fields   []Field
	Lines    []Line // always shown
	Stack    []Line // the first MaxStack frames unless expanded
	Details  []Line // network headers and body: with details on, or expanded
	At       string // path:line:col of the code that logged it
	Name     string // a network call's name, like the method that made it
	URL      string // a network call's full URL
	Time     time.Time
	Raw      []string // the original lines, for the raw view
	Expanded bool
	// A request and its response find each other (by the record's id, or
	// method and URL): a request without one is still waiting.
	Request, Response *Entry
	Values            []Value // values to keep at hand, found in this log (values.go)
	abandoned         bool    // a request whose response won't come: a restart, the end of the run
	bookmarked        bool    // marked to find again: [ and ] jump between them
	pinned            bool    // kept at the top of the logs (sticky.go)
	hidden            bool    // folded to an empty row with a button that shows it again
	clipped           bool    // too long to show folded: rows were left out
	cleared           bool    // before a clear: in the saved log, off the screen
	search            string  // folded text the filter matches
	cache             []row   // rendered rows, valid for the settings below
	cached            cacheKey
}

// method and path of a network entry, whose Text is "GET /path".
func (e *Entry) method() string {
	m, _, _ := strings.Cut(e.Text, " ")
	return m
}

func (e *Entry) path() string {
	_, p, _ := strings.Cut(e.Text, " ")
	return p
}

// Searchable is what the filter looks in, folded (see fold).
func (e *Entry) Searchable() string {
	if e.search == "" {
		var b strings.Builder
		b.WriteString(e.Tag + " " + e.Name + " " + e.Text)
		for _, f := range e.Fields {
			b.WriteString(" " + f.Key + "=" + f.Value)
		}
		for _, l := range e.Lines {
			b.WriteString(" " + l.Text)
		}
		for _, l := range e.Details {
			if l.Kind == LineQuery {
				b.WriteString(" " + l.Text)
			}
		}
		e.search = fold(b.String())
	}
	return e.search
}

// waiting is whether a request has no response yet, and may still get one.
func (e *Entry) waiting() bool {
	return e.Level == "request" && e.Response == nil && !e.abandoned
}

// status is a network entry's HTTP status, "" when it has none.
func (e *Entry) status() string {
	for _, f := range e.Fields {
		if f.Key == "status" {
			return f.Value
		}
	}
	return ""
}

var levelCategory = map[string]string{
	"debug": "debug", "print": "debug", "info": "info", "success": "success", "warning": "warning",
	"error": "error", "fatal": "error", "request": "network", "response": "network", "value": "debug",
}

// Default tags of the usual Log methods, not worth repeating after the badge.
var defaultTags = map[string]bool{
	"DEBUG": true, "INFO": true, "SUCCESS": true, "WARNING": true, "WARN": true, "ERROR": true,
	"WTF": true, "FATAL": true, "REQUEST": true, "RESPONSE": true, "OK": true,
}

var plainLevels = map[string]string{
	"DEBUG": "debug", "INFO": "info", "OK": "success", "WARN": "warning", "ERROR": "error",
	"FATAL": "fatal", "REQUEST": "request", "RESPONSE": "response",
}

var (
	logcat      = regexp.MustCompile(`^([VDIWEF])/([^(]*?)\s*\(\s*(\d+)\): ?(.*)$`)
	logcatStart = regexp.MustCompile(`^[VDIWEF]/[^(]*\(\s*\d+\):`)
	logcatAny   = regexp.MustCompile(`[VDIWEF]/[^\s(/]+\s*\(\s*\d+\): `)
	marker      = regexp.MustCompile(`^⟪fd(?: (\d+) (\d+)/(\d+))?⟫(.*)$`)
	plain       = regexp.MustCompile(`(?s)^(DEBUG|INFO|OK|WARN|ERROR|FATAL|REQUEST|RESPONSE)(?: \[(.*?)\])? (.*)$`)
	engineError = regexp.MustCompile(`^\[ERROR:[^\]]*\]\s*`)
)

// Parser turns output lines into entries. It keeps the state that spans
// lines: record chunks, exception blocks, the first API host.
type Parser struct {
	chunks      map[string]*chunk
	inException bool
	errorGroup  bool
	host        string
	last        *Entry
	pending     map[string][]*Entry // requests waiting for a response, by call
	// ValueKeys are more keys (besides tokens) whose values in network
	// headers and bodies are kept at hand: userId, deviceId, ...
	ValueKeys []string
	// Ready turns true once flutter has listed its key commands: from then
	// on the digit keys are log toggles, not answers to a flutter prompt.
	Ready bool
}

type chunk struct {
	parts map[int]string
	raw   []string
}

// Result of one line: a new entry, or an earlier one that grew.
type Result struct {
	New   *Entry
	Grown *Entry
}

func (p *Parser) Line(raw string, now time.Time) Result {
	raw = unglue(raw)
	if m := logcat.FindStringSubmatch(raw); m != nil {
		if strings.TrimSpace(m[2]) == "flutter" {
			return p.app(m[4], m[1], raw, now)
		}
		return p.native(m[1], strings.TrimSpace(m[2]), m[4], raw, now)
	}
	if msg, ok := strings.CutPrefix(raw, "flutter: "); ok { // iOS, macOS
		return p.app(msg, "I", raw, now)
	}
	if strings.HasPrefix(raw, "⟪fd") { // web
		return p.app(raw, "I", raw, now)
	}
	return p.tool(raw, now)
}

// unglue takes a device line out of flutter's progress spinner: while flutter
// shows "Syncing files to device...  ⣽", the device's lines are printed
// right after the spinner, on the same line, and the spinner may be drawn
// again after them (\r, \b).
func unglue(raw string) string {
	if logcatStart.MatchString(raw) || strings.HasPrefix(raw, "flutter: ") {
		return raw
	}
	loc := logcatAny.FindStringIndex(raw)
	if loc == nil || !spinnerBefore(raw[:loc[0]]) {
		return raw
	}
	line := raw[loc[0]:]
	if i := strings.IndexAny(line, "\r\b"); i >= 0 {
		line = line[:i]
	}
	return line
}

// spinnerBefore is whether s ends the way flutter's status lines do: a
// spinner frame (braille), or a carriage return or backspace redrawing one.
func spinnerBefore(s string) bool {
	s = strings.TrimRight(escapeSeq.ReplaceAllString(s, ""), " ")
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, "\r\b") {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r >= 0x2800 && r <= 0x28ff
}

// CouldBeLog is true for the start of a line that may turn out to be a device
// log line; such partial lines are not shown until they are complete.
func CouldBeLog(partial string) bool {
	if partial == "" {
		return false
	}
	if strings.ContainsRune("VDIWEF", rune(partial[0])) && (len(partial) == 1 || partial[1] == '/') {
		return logcatStart.MatchString(partial) || !strings.Contains(partial, "):")
	}
	return strings.HasPrefix(partial, "flutter:") || strings.HasPrefix("flutter:", partial) ||
		strings.HasPrefix(partial, "⟪") || strings.HasPrefix("⟪fd", partial)
}

var (
	toolError   = regexp.MustCompile(`(?i)^(error|fatal|failure|e:)[\s:]|: error:|build failed|^\[!\]|exception:`)
	toolWarning = regexp.MustCompile(`(?i)^(warning|w:)[\s:]|: warning:`)
)

func (p *Parser) tool(raw string, now time.Time) Result {
	p.errorGroup, p.inException = false, false
	text := cleanTool(raw)
	e := &Entry{Kind: KindTool, Text: text, Time: now, Raw: []string{clean(raw)}}
	plainText := strings.TrimSpace(clean(text))
	// Build errors and warnings stand out, unless the tool colored them.
	if !strings.Contains(text, "\x1b[") {
		switch {
		case toolError.MatchString(plainText):
			e.TextTone, e.Category = ToneRed, "error"
		case toolWarning.MatchString(plainText):
			e.TextTone = ToneYellow
		}
	}
	if strings.HasPrefix(plainText, "Reloaded ") || strings.HasPrefix(plainText, "Restarted application") {
		e.Text, e.Level = plainText, "reload"
	}
	if strings.HasPrefix(plainText, "Restarted application") || strings.HasPrefix(plainText, "Application finished") ||
		strings.HasPrefix(plainText, "Lost connection to device") {
		p.Abandon() // the app that sent them is gone
	}
	if strings.Contains(plainText, "List all available interactive commands") {
		p.Ready = true
	}
	return p.add(e)
}

func (p *Parser) native(letter, tag, msg, raw string, now time.Time) Result {
	e := &Entry{Kind: KindNative, Category: "native", Level: letter,
		Tag: clean(tag), Text: clean(msg), Time: now, Raw: []string{clean(raw)}}
	if !strings.ContainsAny(letter, "WEF") {
		e.TextTone = ToneDim
	}
	return p.add(e)
}

func (p *Parser) app(msg, letter, raw string, now time.Time) Result {
	if m := marker.FindStringSubmatch(msg); m != nil {
		return p.chunk(m, raw, now)
	}
	msg = clean(msg)
	rawLine := clean(raw)

	if strings.HasPrefix(msg, "══╡ EXCEPTION CAUGHT BY") {
		p.inException = true
		return p.add(p.entry("error", "Flutter", strings.Trim(msg, "═╡╞ "), now, rawLine))
	}
	if p.inException && p.last != nil {
		trimmed := strings.TrimSpace(msg)
		if trimmed != "" && strings.Trim(trimmed, "═") == "" {
			p.inException = false
		} else {
			kind := LinePlain
			if strings.HasPrefix(strings.TrimSpace(msg), "#") {
				kind = LineFrame
			}
			p.last.Lines = append(p.last.Lines, Line{msg, kind})
		}
		return p.grow(rawLine)
	}

	if letter == "E" { // uncaught exceptions and their stack traces
		text := engineError.ReplaceAllString(msg, "")
		if p.errorGroup && p.last != nil && (strings.TrimSpace(text) == "" ||
			strings.HasPrefix(text, "#") || strings.HasPrefix(text, "<asynchronous")) {
			if strings.TrimSpace(text) != "" {
				p.last.Stack = append(p.last.Stack, Line{text, LineFrame})
			}
			return p.grow(rawLine)
		}
		result := p.add(p.entry("error", "", text, now, rawLine))
		p.errorGroup = true
		return result
	}
	p.errorGroup = false

	if strings.HasPrefix(msg, "Another exception was thrown:") {
		return p.add(p.entry("error", "Flutter", msg, now, rawLine))
	}
	if m := plain.FindStringSubmatch(msg); m != nil { // an app without FDEV_LOGS
		return p.add(p.message(plainLevels[m[1]], m[2], m[3], now, rawLine))
	}
	e := p.printEntry(msg, now)
	e.Raw = []string{rawLine}
	return p.add(e)
}

// printEntry is a print(): JSON and Dart maps, even cut short, are shown
// indented.
func (p *Parser) printEntry(msg string, now time.Time) *Entry {
	return p.message("print", "", msg, now, "")
}

// message is an app log whose message may be, or end with, a JSON value or
// a Dart map: that part is shown indented, as JSON.
func (p *Parser) message(level, tag, msg string, now time.Time, raw string) *Entry {
	e := p.entry(level, tag, msg, now, raw)
	lines, ok := prettyText(msg)
	if !ok && len(msg) > 200 {
		var loose []string
		if loose, ok = looseLines(msg); ok {
			lines = toLines(loose, LineJSON)
		}
	}
	if ok && len(lines) > 1 {
		e.Text, e.Lines = lines[0].Text, lines[1:]
	}
	return e
}

func (p *Parser) entry(level, tag, text string, now time.Time, raw string) *Entry {
	e := &Entry{Kind: KindApp, Category: levelCategory[level], Level: level,
		Tag: tag, Text: text, Time: now, Raw: []string{raw}}
	if defaultTags[strings.ToUpper(tag)] {
		e.Tag = ""
	}
	switch level {
	case "error", "fatal":
		e.TextTone = ToneRed
	case "warning":
		e.TextTone = ToneYellow
	}
	return e
}

func (p *Parser) chunk(m []string, raw string, now time.Time) Result {
	seq, payload := m[1], m[4]
	if seq == "" {
		return p.record(payload, []string{clean(raw)}, now)
	}
	if p.chunks == nil {
		p.chunks = map[string]*chunk{}
	}
	c := p.chunks[seq]
	// After a hot restart the app counts from 1 again: a first part starts
	// the record over, whatever an unfinished one of that id left.
	if c == nil || m[2] == "1" {
		c = &chunk{parts: map[int]string{}}
		p.chunks[seq] = c
	}
	var part, count int
	fmt.Sscan(m[2], &part)
	fmt.Sscan(m[3], &count)
	c.parts[part] = payload
	c.raw = append(c.raw, clean(raw))
	if len(c.parts) < count {
		return Result{}
	}
	delete(p.chunks, seq)
	var b strings.Builder
	for i := 1; i <= count; i++ {
		b.WriteString(c.parts[i])
	}
	return p.record(b.String(), c.raw, now)
}

// record is one fdev record (docs/PROTOCOL.md).
type record struct {
	Level   string          `json:"l"`
	Tag     string          `json:"t"`
	Message string          `json:"m"`
	Stack   string          `json:"s"`
	At      string          `json:"at"`
	Phase   string          `json:"p"`
	Method  string          `json:"method"`
	URL     string          `json:"url"`
	Status  int             `json:"status"`
	Millis  *int            `json:"ms"`
	Headers json.RawMessage `json:"headers"`
	Body    json.RawMessage `json:"body"`
	Error   string          `json:"error"`
	Name    string          `json:"name"`
	ID      string          `json:"id"` // pairs a response with its request
	Key     string          `json:"k"`  // a value to keep at hand: its name
	Value   json.RawMessage `json:"v"`  // and the value
}

func (p *Parser) record(payload string, raw []string, now time.Time) Result {
	p.errorGroup, p.inException = false, false
	var r record
	if err := json.Unmarshal([]byte(payload), &r); err != nil {
		// Something the terminal put in the middle (an escape sequence, a
		// control character) breaks the JSON; without it, it may parse.
		if json.Unmarshal([]byte(clean(payload)), &r) != nil {
			e := p.printEntry(clean(payload), now)
			e.Raw = raw
			return p.add(e)
		}
	}
	var e *Entry
	switch r.Level {
	case "http":
		e = p.http(r, now)
	case "value":
		e = p.value(r, now)
	default:
		level := r.Level
		if _, ok := levelCategory[level]; !ok {
			level = "debug"
		}
		message := r.Message
		lines := toLines(strings.Split(message, "\n"), LinePlain)
		if pretty, ok := prettyText(message); ok {
			lines = pretty
		} else if loose, ok := looseLines(message); ok && len(message) > 200 {
			lines = toLines(loose, LineJSON)
		}
		e = p.entry(level, clean(r.Tag), clean(lines[0].Text), now, "")
		if len(lines) > 1 && lines[len(lines)-1].Kind == LineJSON {
			e.TextTone = ToneNone
		}
		for _, l := range lines[1:] {
			e.Lines = append(e.Lines, Line{clean(l.Text), l.Kind})
		}
		for _, l := range strings.Split(strings.TrimRight(r.Stack, "\n"), "\n") {
			if strings.TrimSpace(l) != "" {
				e.Stack = append(e.Stack, Line{clean(l), LineFrame})
			}
		}
	}
	e.At, e.Raw = clean(r.At), raw
	return p.add(e)
}

func (p *Parser) http(r record, now time.Time) *Entry {
	u, _ := url.Parse(r.URL)
	path, host := r.URL, ""
	var query []Line
	if u != nil {
		path = u.EscapedPath()
		if path == "" {
			path = "/"
		}
		query = queryLines(u.RawQuery)
		if p.host == "" {
			p.host = u.Host
		}
		if u.Host != p.host {
			host = u.Host
		}
	}
	method := clean(r.Method)
	e := &Entry{Kind: KindApp, Category: "network", Time: now, Text: method + " " + clean(path),
		Name: clean(r.Name), URL: clean(r.URL)}
	call := r.ID
	if call == "" {
		call = method + " " + r.URL
	}
	switch r.Phase {
	case "request":
		e.Level = "request"
		e.Lines = query // always shown, under the line
		if p.pending == nil {
			p.pending = map[string][]*Entry{}
		}
		p.pending[call] = append(p.pending[call], e)
	case "response":
		e.Level = "response"
		tone := ToneRed
		if r.Status > 0 && r.Status < 300 {
			tone = ToneGreen
		} else if r.Status < 400 {
			tone = ToneYellow
		}
		e.Fields = append(e.Fields, Field{"status", fmt.Sprint(r.Status), tone})
	default:
		e.Level = "fail"
		if r.Status != 0 {
			e.Fields = append(e.Fields, Field{"status", fmt.Sprint(r.Status), ToneRed})
		}
	}
	if r.Error != "" {
		e.Fields = append(e.Fields, Field{"error", clean(r.Error), ToneRed})
	}
	if r.Millis != nil {
		e.Fields = append(e.Fields, Field{"took", fmt.Sprintf("%dms", *r.Millis), ToneNone})
	}
	if len(r.Body) > 0 && r.Phase != "request" {
		e.Fields = append(e.Fields, Field{"size", size(len(r.Body)), ToneNone})
	}
	if host != "" {
		e.Fields = append(e.Fields, Field{"host", clean(host), ToneNone})
	}
	if r.Phase != "request" {
		e.Details = append(e.Details, query...)
		if waiting := p.pending[call]; len(waiting) > 0 {
			req := waiting[0]
			if p.pending[call] = waiting[1:]; len(waiting) == 1 {
				delete(p.pending, call)
			}
			req.Response, e.Request = e, req
			req.cached = cacheKey{} // it shows the response's status now
		}
	}

	from := method + " " + clean(path)
	headers, ok := parseJSON(r.Headers)
	if ok && headers.object {
		e.Values = append(e.Values, p.findValues(headers, from+" · "+r.Phase+" header")...)
	}
	if body, ok := parseJSON(r.Body); ok {
		e.Values = append(e.Values, p.findValues(body, from+" · "+r.Phase+" body")...)
	}
	if ok && headers.object {
		for i, key := range headers.keys {
			value := headers.values[i].scalar
			if v := headers.values[i]; v.object || v.array { // e.g. report-to
				value = strings.Join(strings.Fields(pretty(v, "")), " ")
			}
			var s string
			if json.Unmarshal([]byte(value), &s) == nil {
				value = s
			}
			e.Details = append(e.Details, Line{clean(key) + ": " + clean(value), LineHeader})
		}
	}
	if len(r.Body) > 0 {
		e.Details = append(e.Details, bodyLines(r.Body)...)
	}
	return e
}

// bodyLines shows a body: JSON pretty-printed, including a JSON string that
// holds JSON; other text as it is.
func bodyLines(body json.RawMessage) []Line {
	if lines, ok := jsonLines(body); ok {
		return toLines(lines, LineJSON)
	}
	var text string
	if json.Unmarshal(body, &text) == nil {
		if lines, ok := jsonLines([]byte(text)); ok {
			return toLines(lines, LineJSON)
		}
		if lines, ok := looseLines(text); ok { // cut short by the app
			return toLines(lines, LineJSON)
		}
		return toLines(strings.Split(text, "\n"), LinePlain)
	}
	return toLines([]string{string(body)}, LinePlain)
}

// queryLines are a URL's query parameters, one a line, decoded.
func queryLines(raw string) []Line {
	var out []Line
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		if s, err := url.QueryUnescape(k); err == nil {
			k = s
		}
		if s, err := url.QueryUnescape(v); err == nil {
			v = s
		}
		sep := "&"
		if len(out) == 0 {
			sep = "?"
		}
		out = append(out, Line{sep + clean(k) + "=" + clean(v), LineQuery})
	}
	return out
}

// Abandon gives up on the requests still waiting: the app restarted or
// stopped, their responses won't come.
func (p *Parser) Abandon() {
	for _, waiting := range p.pending {
		for _, e := range waiting {
			e.abandoned, e.cached = true, cacheKey{}
		}
	}
	p.pending = nil
}

// InFlight is how many requests are waiting for their response.
func (p *Parser) InFlight() int {
	n := 0
	for _, waiting := range p.pending {
		n += len(waiting)
	}
	return n
}

func toLines(texts []string, kind LineKind) []Line {
	out := make([]Line, len(texts))
	for i, t := range texts {
		out[i] = Line{clean(t), kind}
	}
	return out
}

func (p *Parser) add(e *Entry) Result {
	p.last = e
	return Result{New: e}
}

func (p *Parser) grow(raw string) Result {
	p.last.Raw = append(p.last.Raw, raw)
	p.last.search, p.last.cached = "", cacheKey{}
	return Result{Grown: p.last}
}

func size(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(n)/1024/1024)
}
