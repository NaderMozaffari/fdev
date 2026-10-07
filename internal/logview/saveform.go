package logview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// Saving some of the logs (⤓ save, or s with logs selected): a subject and
// tags to find them by, which logs, and the format.
//
//	Subject   login fails after a token refresh
//	Tags      auth, bug
//	Logs      ● the 23 selected (12:01:03 – 12:01:09)
//	          ○ the 412 shown now (with the filter)
//	          ○ the 5 bookmarked
//	          ○ the whole session (1 873)
//	Format    ● text, to open in fdev again   ○ Markdown   ○ JSON
//
// A text save is a saved log like a session's (its raw output too, so it
// opens with all the viewer can do), starred so it is never cleaned up; it
// shows in the menu's Saved logs with its subject and tags. Markdown is for
// pasting in an issue, JSON for tools.

type saveAnswer struct {
	Subject, Tags, What, Format string
}

const (
	saveSelected = "selected"
	saveShown    = "shown"
	saveMarked   = "bookmarked"
	saveAll      = "all"
	formatText   = "text"
	formatMD     = "md"
	formatJSON   = "json"
)

func (m *Model) openSave() tea.Cmd {
	sel := m.selected()
	shown := m.shownEntries()
	var marked []*Entry
	for _, e := range m.entries {
		if e.bookmarked {
			marked = append(marked, e)
		}
	}
	m.saveAns = saveAnswer{What: saveAll, Format: formatText}
	var opts []huh.Option[string]
	if len(sel) > 0 {
		opts = append(opts, huh.NewOption(fmt.Sprintf("the %s selected (%s – %s)", plural(len(sel), "log"),
			sel[0].Time.Format("15:04:05"), sel[len(sel)-1].Time.Format("15:04:05")), saveSelected))
		m.saveAns.What = saveSelected
	}
	if len(shown) < len(m.entries) {
		opts = append(opts, huh.NewOption(fmt.Sprintf("the %s shown now (filters and toggles applied)", plural(len(shown), "log")), saveShown))
	}
	if len(marked) > 0 {
		opts = append(opts, huh.NewOption(fmt.Sprintf("the %s bookmarked", plural(len(marked), "log")), saveMarked))
		if len(sel) == 0 {
			m.saveAns.What = saveMarked
		}
	}
	opts = append(opts, huh.NewOption(fmt.Sprintf("the whole session (%s)", plural(len(m.entries), "log")), saveAll))

	m.saveForm = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Subject").Placeholder("what these logs show, e.g. login fails after a token refresh").
			CharLimit(120).Value(&m.saveAns.Subject),
		huh.NewInput().Title("Tags").Placeholder("comma separated, e.g. auth, bug").CharLimit(120).Value(&m.saveAns.Tags),
		huh.NewSelect[string]().Title("Logs").Options(opts...).Value(&m.saveAns.What),
		huh.NewSelect[string]().Title("Format").Options(
			huh.NewOption("Text · opens in fdev again, in Saved logs with its subject and tags", formatText),
			huh.NewOption("Markdown · for an issue or a chat", formatMD),
			huh.NewOption("JSON · every field, for tools", formatJSON),
		).Value(&m.saveAns.Format),
	).Title("Save logs").Description("to " + m.look.Dir + " in the project, which git ignores")).
		WithTheme(m.th.Huh()).WithShowHelp(!m.full)
	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
	m.saveForm.WithKeyMap(keys).WithWidth(min(m.width-4, 90))
	m.saveForm.Update(tea.WindowSizeMsg{Width: m.width, Height: m.logHeight()})
	m.resize()
	return m.saveForm.Init()
}

func (m *Model) updateSaveForm(msg tea.Msg) tea.Cmd {
	f, cmd := m.saveForm.Update(msg)
	if form, ok := f.(*huh.Form); ok {
		m.saveForm = form
	}
	switch m.saveForm.State {
	case huh.StateAborted:
		m.saveForm = nil
		m.resize()
		return nil
	case huh.StateCompleted:
		m.saveForm = nil
		m.resize()
		if path, err := m.saveSome(m.saveAns); err != nil {
			m.setStatus("✘ can't save: " + err.Error())
		} else {
			m.setStatus("✓ saved " + path)
			m.unselect()
		}
		return nil
	}
	return cmd
}

// saveEntries is the logs an answer picks.
func (m *Model) saveEntries(what string) []*Entry {
	switch what {
	case saveSelected:
		return m.selected()
	case saveShown:
		return m.shownEntries()
	case saveMarked:
		var out []*Entry
		for _, e := range m.entries {
			if e.bookmarked {
				out = append(out, e)
			}
		}
		return out
	}
	return m.entries
}

// splitTags reads "auth, bug #login" as auth, bug, login.
func splitTags(s string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '#' || r == '،' || unicode.IsSpace(r) && r != ' ' }) {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// slug is s for a file name: letters (any script), digits and dashes.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
		if b.Len() > 60 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

// saveSome writes the logs an answer picks; it returns the path, relative
// to the root.
func (m *Model) saveSome(a saveAnswer) (string, error) {
	entries := m.saveEntries(a.What)
	if len(entries) == 0 {
		return "", fmt.Errorf("no logs to save")
	}
	dir, err := logsDir(m.o.Root, m.look.Dir)
	if err != nil {
		return "", err
	}
	name := slug(a.Subject)
	if name == "" {
		name = unsafeName.ReplaceAllString(m.o.Target, "-")
	}
	if name == "" {
		name = "logs"
	}
	base := time.Now().Format(timeLayout) + "_" + name
	tags := splitTags(a.Tags)
	var path string
	switch a.Format {
	case formatMD:
		path = filepath.Join(dir, base+".md")
		err = os.WriteFile(path, []byte(m.markdown(a.Subject, tags, entries)), 0o600)
	case formatJSON:
		path = filepath.Join(dir, base+".json")
		var data []byte
		data, err = m.jsonExport(a.Subject, tags, entries)
		if err == nil {
			err = os.WriteFile(path, data, 0o600)
		}
	default:
		path = filepath.Join(dir, base+".log")
		var b strings.Builder
		m.writeHeader(&b, a.Subject, tags, entries)
		for _, e := range entries {
			writeEntry(&b, e)
		}
		if err = os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
			break
		}
		// The raw output too: opened again, it is the logs as they came.
		var raw strings.Builder
		for _, e := range entries {
			for _, r := range e.Raw {
				raw.WriteString(rawLine(e.Time, r) + "\n")
			}
		}
		if err = os.WriteFile(filepath.Join(dir, base+".raw.log"), []byte(raw.String()), 0o600); err != nil {
			break
		}
		err = Star(m.o.Root, m.look.Dir, base, true) // picked by hand: kept
	}
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(m.o.Root, path); err == nil {
		return rel, nil
	}
	return path, nil
}

// writeHeader is a saved log's first lines, which Sessions reads back.
func (m *Model) writeHeader(b *strings.Builder, subject string, tags []string, entries []*Entry) {
	fmt.Fprintf(b, "# %s · %s · started %s\n", m.o.Title, m.o.Command, m.started.Format(time.RFC3339))
	if m.o.Note != "" {
		fmt.Fprintf(b, "# %s\n", m.o.Note)
	}
	if subject = strings.TrimSpace(subject); subject != "" {
		fmt.Fprintf(b, "# subject: %s\n", oneLine(subject))
	}
	if len(tags) > 0 {
		fmt.Fprintf(b, "# tags: %s\n", strings.Join(tags, ", "))
	}
	if len(entries) > 0 && len(entries) < len(m.entries) {
		fmt.Fprintf(b, "# logs: %s, %s – %s\n", plural(len(entries), "log"),
			entries[0].Time.Format("15:04:05"), entries[len(entries)-1].Time.Format("15:04:05"))
	}
	b.WriteString("\n")
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func (m *Model) markdown(subject string, tags []string, entries []*Entry) string {
	var b strings.Builder
	title := strings.TrimSpace(subject)
	if title == "" {
		title = m.o.Title + " logs"
	}
	fmt.Fprintf(&b, "# %s\n\n", oneLine(title))
	if len(tags) > 0 {
		b.WriteString(strings.Join(func() []string {
			out := make([]string, len(tags))
			for i, t := range tags {
				out[i] = "`#" + t + "`"
			}
			return out
		}(), " ") + "\n\n")
	}
	fmt.Fprintf(&b, "- **Command:** `%s`\n", m.o.Command)
	if m.o.Note != "" {
		fmt.Fprintf(&b, "- **Device:** %s\n", m.o.Note)
	}
	fmt.Fprintf(&b, "- **Logs:** %s, %s – %s\n\n", plural(len(entries), "log"),
		entries[0].Time.Format("2006-01-02 15:04:05"), entries[len(entries)-1].Time.Format("15:04:05"))
	b.WriteString("```text\n")
	for _, e := range entries {
		var one strings.Builder
		writeEntry(&one, e)
		b.WriteString(strings.ReplaceAll(one.String(), "```", "ˋˋˋ"))
	}
	b.WriteString("```\n")
	return b.String()
}

// exported is a log in a JSON save.
type exported struct {
	Time    string            `json:"time"`
	Level   string            `json:"level,omitempty"`
	Tag     string            `json:"tag,omitempty"`
	Text    string            `json:"text"`
	Name    string            `json:"name,omitempty"`
	URL     string            `json:"url,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
	Query   []string          `json:"query,omitempty"`
	Lines   []string          `json:"lines,omitempty"`
	Stack   []string          `json:"stack,omitempty"`
	Headers []string          `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
	At      string            `json:"at,omitempty"`
	Marked  bool              `json:"bookmarked,omitempty"`
}

func (m *Model) jsonExport(subject string, tags []string, entries []*Entry) ([]byte, error) {
	texts := func(lines []Line) []string {
		var out []string
		for _, l := range lines {
			out = append(out, stripANSI(l.Text))
		}
		return out
	}
	var logs []exported
	for _, e := range entries {
		x := exported{Time: e.Time.Format(time.RFC3339Nano), Level: e.Level, Tag: e.Tag, Text: stripANSI(e.Text),
			Name: e.Name, URL: e.URL, At: e.At, Marked: e.bookmarked, Stack: texts(e.Stack)}
		if len(e.Fields) > 0 {
			x.Fields = map[string]string{}
			for _, f := range e.Fields {
				x.Fields[f.Key] = f.Value
			}
		}
		query, headers, body := splitDetails(e.Details)
		for _, l := range e.Lines {
			if l.Kind == LineQuery {
				query = append(query, l)
			} else {
				x.Lines = append(x.Lines, stripANSI(l.Text))
			}
		}
		x.Query, x.Headers = texts(query), texts(headers)
		if len(body) > 0 {
			text := linesText(body)
			if json.Valid([]byte(text)) {
				x.Body = json.RawMessage(compactJSON(text))
			} else {
				x.Body, _ = json.Marshal(text)
			}
		}
		logs = append(logs, x)
	}
	return json.MarshalIndent(map[string]any{
		"subject": strings.TrimSpace(subject), "tags": tags, "project": m.o.Title, "command": m.o.Command,
		"note": m.o.Note, "started": m.started.Format(time.RFC3339), "logs": logs,
	}, "", "  ")
}
