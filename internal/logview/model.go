// Package logview runs a command (usually `flutter run`) in a pty and shows
// its output as a full-screen log viewer: device logs cleaned up and
// filterable, flutter's own keys passed through.
package logview

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/editor"
	"github.com/NaderMozaffari/fdev/internal/theme"
	"github.com/NaderMozaffari/fdev/internal/version"
)

const maxEntries = 20000

type Options struct {
	Root    string
	Title   string // the project, for the header
	Version string // the app's, for the header: v2.1.39 · build 439
	About   About  // the app, for the settings
	Command string // what the header shows
	Shell   string // what runs, by `sh -c`
	Env     []string
	Note    string // e.g. the device name
	// Plain is for commands that are not `flutter run` (builds, codegen):
	// no log toggles, and the filter applies to every line.
	Plain    bool
	Settings map[string]bool // the toggles; changed in place, the caller saves them
	Editor   string          // editor command template, see editor.Command
	Target   string          // names the saved log files
	Look     config.Look     // how lines are drawn; see SettingsForm
	OnLook   func(config.Look)
	// Durations is how long flutter's steps took (seconds, by step), for
	// the progress bars; changed in place, the caller saves it.
	Durations map[string]float64
	Theme     theme.Theme
	// UI is the theme and size picked in the settings; OnUI hears of them
	// changing: as they are tried (save false, and the old ones back on
	// cancel) and when saved.
	UI     config.UI
	OnUI   func(ui config.UI, save bool)
	Full   bool // start in full screen: the header and the logs only
	Width  int
	Height int
	// Replay shows a saved session (a .raw.log, or a readable .log)
	// instead of running Shell.
	Replay string
	// Values are keys, besides tokens, whose values in network headers and
	// bodies are kept at hand (values.go): fdev.yaml's logs.values.
	Values []string
}

// DoneMsg is sent when the user leaves the viewer after the command exited.
type DoneMsg struct {
	Code   int
	Err    error
	Took   time.Duration
	Errors int
	Replay bool // it showed a saved session
}

type (
	outputMsg []byte
	closedMsg struct{}
	exitMsg   struct {
		code int
		err  error
	}
	openedMsg struct {
		what string
		err  error
	}
)

type keyMap struct {
	Reload, Restart, Quit, Back                key.Binding
	Scroll, Page, Ends                         key.Binding
	Filter, Toggles, Parts, Open, Mouse, Help  key.Binding
	Look, Save, Wipe, Full, Window             key.Binding
	Select, Range, Marks, Values, Expand, Keep key.Binding
}

func newKeyMap() keyMap {
	b := func(keys []string, k, desc string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(k, desc))
	}
	return keyMap{
		Reload:  b([]string{"r"}, "r", "hot reload"),
		Restart: b([]string{"R"}, "R", "hot restart"),
		Quit:    b([]string{"q"}, "q", "quit app"),
		Back:    b([]string{"esc", "q"}, "esc", "back"),
		Scroll:  b([]string{"up", "down"}, "↑/↓", "scroll"),
		Page:    b([]string{"pgup", "pgdown"}, "pgup/pgdn", "page"),
		Ends:    b([]string{"home", "end"}, "home/end", "top/follow"),
		Filter:  b([]string{"/"}, "/", "filter"),
		Toggles: b([]string{"1", "9"}, "1-9", "show/hide logs"),
		Wipe:    b([]string{"ctrl+l"}, "ctrl+l", "clear the screen (the log stays saved)"),
		Parts:   b([]string{",", ".", ";"}, ", . ;", "time/level/tag"),
		Open:    b([]string{"↗"}, "click ↗", "open code"), // a mouse action; the key never matches
		Mouse:   b([]string{"m"}, "m", "mouse/select"),
		Help:    b([]string{"?"}, "?", "more"),
		Look:    b([]string{"l"}, "l", "look, theme & saving"),
		Full:    b([]string{"z"}, "z", "full screen"),
		Window:  b([]string{"e"}, "e", "open a long log in a window"),
		Save:    b([]string{"ctrl+s"}, "ctrl+s", "save log"),
		Select:  b([]string{"tab"}, "tab", "select a log: copy, pin, track…"),
		Range:   b([]string{"shift+up", "shift+down"}, "shift+↑/↓", "select a range"),
		Marks:   b([]string{"[", "]"}, "[ ]", "bookmarks"),
		Values:  b([]string{"k"}, "k", "tokens & values"),
		Expand:  b([]string{"x"}, "x", "expand all calls"),
		Keep:    b([]string{"S"}, "⤓ save", "save some, with a subject"), // the bar's button; S is flutter's
	}
}

type Model struct {
	o       Options
	th      theme.Theme
	on      map[string]bool
	look    config.Look
	version int
	maxTag  int      // the widest tag so far, for the tag column
	raw     *os.File // the session's raw log, while saving

	lookForm *huh.Form
	formLook config.Look
	formUI   config.UI
	savedUI  config.UI // what the form goes back to when cancelled
	ui       config.UI // the theme (drawn with th) and size shown
	full     bool      // full screen: the header and the logs, no bars or help

	hover string // the bar item under the mouse

	// Motion (motion.go): smooth scrolling, details unfolding.
	scrollLeft int // lines still to scroll; negative is up
	boost      float64
	wheelDir   int
	lastWheel  time.Time
	moving     bool
	unfolding  *unfold
	dialog     *dialog // one entry, all of it, in a window (dialog.go)
	lit        *Entry  // what just opened, marked for a moment
	litAt      time.Time

	// The command's keys, for the command bar (commands.go).
	commands  []command
	tool      string // flutter, keys (found in the output), or ""
	inKeyList bool
	showMore  bool
	task      *task // the step flutter is working on, with a progress bar
	bar       progress.Model

	cmd     *exec.Cmd
	pty     term
	out     chan []byte
	pending []byte
	partial string

	parser  Parser
	entries []*Entry
	counts  map[string]int
	errors  int

	started time.Time
	exited  bool
	code    int
	err     error
	took    time.Duration

	width, height int
	vp            viewport.Model
	rows          []row // what the viewport shows, for clicks
	dirty         bool
	follow        bool
	unseen        int

	keys      keyMap
	help      help.Model
	spin      spinner.Model
	input     textinput.Model
	filtering bool
	filter    string       // folded
	terms     []filterTerm // the filter, read (filter.go)

	// Selected logs (select.go): the cursor, and the other end of a range.
	sel, anchor *Entry
	pins        []*Entry // pinned, in the order they were (sticky.go)
	tracks      []*track
	values      []*kept // tokens and such, at hand (values.go, vault.go)
	vault       *vault
	saveForm    *huh.Form // saving some logs (saveform.go)
	saveAns     saveAnswer
	expandAll   bool // x: every network log open
	mouse       bool
	toast       string
	toastUntil  time.Time
	status      string
	statusUntil time.Time
}

// term is where the command runs: a pty, or pipes on Windows (term_*.go).
type term interface {
	io.ReadWriteCloser
	Resize(rows, cols int) error
}

// New starts o.Shell in a pty (pipes on Windows).
func New(o Options) (*Model, tea.Cmd) {
	m := &Model{o: o, th: o.Theme, on: map[string]bool{}, counts: map[string]int{}, look: o.Look.WithDefaults(),
		width: max(o.Width, 20), height: max(o.Height, 8), mouse: true, follow: true, ui: o.UI.WithDefaults(), full: o.Full,
		started: time.Now(), keys: newKeyMap(), help: help.New(), dirty: true}
	for _, t := range Toggles {
		m.on[t.Name] = t.Default
		if v, ok := o.Settings[t.Name]; ok {
			m.on[t.Name] = v
		}
	}
	m.vp = viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(m.logHeight()))
	m.vp.MouseWheelEnabled = false
	m.spin = spinner.New(spinner.WithSpinner(spinner.Dot))
	m.bar = newBar()
	m.input = textinput.New()
	m.input.Prompt = "Filter: "
	m.input.Placeholder = "text, tag:Billing url:/v1/me status:4 level:error is:bookmarked -hide"
	m.parser.ValueKeys = o.Values
	m.SetTheme(o.Theme)
	m.offKeys()
	if o.Replay != "" {
		m.replay()
		return m, nil
	}

	m.cmd = shellCommand(o.Shell)
	m.cmd.Dir = o.Root
	m.cmd.Env = append(os.Environ(), o.Env...)
	f, err := startTerm(m.cmd, m.logHeight(), m.width)
	if err != nil {
		m.exited, m.err, m.code = true, err, -1
		m.entries = append(m.entries, &Entry{Kind: KindTool, Text: "could not start: " + err.Error(), TextTone: ToneRed, Time: time.Now()})
		return m, nil
	}
	m.pty = f
	if m.look.Save {
		if err := m.startRecording(); err != nil {
			m.setStatus("✘ can't save logs: " + err.Error())
		}
	}
	m.out = make(chan []byte, 64)
	go read(f, m.out)
	return m, tea.Batch(m.listen(), m.wait(), m.spin.Tick)
}

func read(f io.Reader, out chan<- []byte) {
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			out <- append([]byte(nil), buf[:n]...)
		}
		if err != nil {
			close(out)
			return
		}
	}
}

func (m *Model) listen() tea.Cmd {
	ch := m.out
	return func() tea.Msg {
		data, ok := <-ch
		if !ok {
			return closedMsg{}
		}
		return outputMsg(data)
	}
}

func (m *Model) wait() tea.Cmd {
	cmd := m.cmd
	return func() tea.Msg {
		err := cmd.Wait()
		var exit *exec.ExitError
		switch {
		case err == nil:
			return exitMsg{code: 0}
		case errors.As(err, &exit):
			return exitMsg{code: exit.ExitCode()}
		}
		return exitMsg{code: -1, err: err}
	}
}

func (m *Model) SetTheme(th theme.Theme) {
	m.th = th
	m.help.Styles = th.Help()
	m.input.SetStyles(th.List().Filter)
	m.spin.Style = lipgloss.NewStyle().Foreground(th.Fuchsia)
	m.bar = newBar()
	if m.lookForm != nil {
		m.lookForm.WithTheme(th.Huh())
	}
	m.version++
	m.dirty = true
}

// Full is whether the viewer is in full screen.
func (m *Model) Full() bool { return m.full }

// Status is whether the command exited, and if so whether it went well.
func (m *Model) Status() (exited, ok bool) {
	return m.exited, m.code == 0 && m.err == nil
}

// setFull goes in or out of full screen: the header and the logs only.
func (m *Model) setFull(on bool) {
	m.full = on
	if on {
		m.setStatus("full screen · z or ⤡ in the corner brings the bars back")
	}
	m.resize()
	m.resizePty()
}

// compact is whether the bars take as little room as they can: for a
// small window, or when the size is set to small.
func (m *Model) compact() bool {
	switch m.ui.Size {
	case config.SizeSmall:
		return true
	case config.SizeMedium, config.SizeLarge:
		return false
	}
	return m.height < 30 || m.width < 100
}

// ── Layout ───────────────────────────────────────────────────────────────────

// The screen: the header, the logs, the toggle bar (one or two rows) and
// the help (one row, or the full help).
func (m *Model) logHeight() int {
	return max(m.height-1-m.stickyRows()-m.headerRows()-m.toggleRows()-m.commandRows()-m.helpRows(), 1)
}

// logTop is the screen row the logs start on.
func (m *Model) logTop() int { return 1 + m.stickyRows() + m.headerRows() }

// headerRows is the table's column titles, when it shows.
func (m *Model) headerRows() int {
	if m.look.Layout == config.LayoutTable && !m.o.Plain && !m.on["raw"] && m.lookForm == nil && m.saveForm == nil {
		return 2
	}
	return 0
}

func (m *Model) helpRows() int {
	switch {
	case m.sel != nil && !m.filtering && !m.overlay():
		return 1 // the selection's actions
	case m.help.ShowAll && !m.filtering:
		return lipgloss.Height(m.help.FullHelpView(m.fullHelp()))
	case m.full && !m.filtering:
		return 0
	}
	return 1
}

func (m *Model) resizePty() {
	if m.pty != nil && !m.exited {
		_ = m.pty.Resize(m.logHeight(), m.width)
	}
}

// fitHeight gives the logs what the bar leaves: the bar can gain a row when
// a hidden count appears.
func (m *Model) fitHeight() {
	if h := m.logHeight(); m.vp.Height() != h {
		m.vp.SetHeight(h)
		m.dirty = true
		m.resizePty()
	}
}

func (m *Model) resize() {
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(m.logHeight())
	m.help.SetWidth(m.width)
	m.dirty = true
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if m.lookForm != nil || m.saveForm != nil {
		// Everything but the viewer's own messages goes to the form: huh
		// sends itself messages too (focus, next field).
		switch msg.(type) {
		case outputMsg, closedMsg, exitMsg, openedMsg, spinner.TickMsg, tea.WindowSizeMsg:
		default:
			if m.saveForm != nil {
				return m.updateSaveForm(msg)
			}
			return m.updateLookForm(msg)
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(msg.Width, 20), max(msg.Height, 8)
		m.resize()
		m.resizePty()
	case outputMsg:
		m.feed(msg)
		return m.listen()
	case motionMsg:
		return m.motion()
	case closedMsg:
		if m.partial != "" {
			m.addLine(strings.TrimSuffix(m.partial, "\r"))
			m.partial = ""
		}
	case exitMsg:
		m.exited, m.code, m.err, m.took = true, msg.code, msg.err, time.Since(m.started)
		m.parser.Abandon()
		m.version++
		if m.raw != nil {
			if path, err := m.save(); err == nil {
				m.setStatus("✓ saved " + path)
			}
		}
	case spinner.TickMsg:
		if !m.exited {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			if m.task != nil || m.parser.InFlight() > 0 {
				m.dirty = true // the progress bar moves, requests wait
			}
			return cmd
		}
	case openedMsg:
		if msg.err != nil {
			m.setStatus("✘ " + msg.err.Error())
		} else {
			m.setStatus("✓ opened " + msg.what)
		}
	case tea.PasteMsg:
		if m.filtering {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return cmd
		}
		m.write(msg.Content)
	case tea.MouseWheelMsg:
		if m.vault != nil {
			return nil
		}
		if m.dialog != nil {
			m.dialogWheel(msg.Button)
			return nil
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			return m.wheel(-1)
		case tea.MouseWheelDown:
			return m.wheel(1)
		}
	case tea.MouseClickMsg:
		if m.vault != nil {
			return m.vaultClick(msg.X, msg.Y)
		}
		if m.dialog != nil {
			return m.dialogClick(msg.X, msg.Y)
		}
		if msg.Button == tea.MouseLeft {
			return m.click(msg.X, msg.Y, msg.Mod.Contains(tea.ModShift))
		}
	case tea.MouseMotionMsg:
		m.fitHeight()
		m.hover = m.headerAt(msg.X, msg.Y)
		if m.hover == "" {
			m.hover = m.barItemAt(msg.X, msg.Y)
		}
		if m.hover == "" {
			m.hover = m.selBarAt(msg.X, msg.Y)
		}
		if i, _, ok := m.stickyAt(msg.X, msg.Y); ok && m.hover == "" {
			m.hover = fmt.Sprintf("sticky:%d", i)
		}
		if m.hover == "" {
			m.hover = m.commandAt(msg.X, msg.Y)
		}
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return nil
}

func (m *Model) feed(data []byte) {
	m.pending = append(m.pending, data...)
	n := completeUTF8(m.pending)
	text := m.partial + string(m.pending[:n])
	m.pending = append([]byte(nil), m.pending[n:]...)

	// A screen clear (flutter's `c`) clears the screen, not the log.
	parts := splitClears(text)
	for _, part := range parts[:len(parts)-1] {
		for _, line := range strings.Split(part, "\n") {
			if line = strings.TrimSuffix(line, "\r"); line != "" {
				m.outputLine(line)
			}
		}
		m.clearScreen("screen cleared")
	}
	lines := strings.Split(parts[len(parts)-1], "\n")
	m.partial = lines[len(lines)-1]
	for _, line := range lines[:len(lines)-1] {
		m.outputLine(strings.TrimSuffix(line, "\r"))
	}
	m.trackTask()
	m.dirty = true
}

func (m *Model) outputLine(line string) {
	m.taskFinished(cleanTool(line))
	m.detectCommands(cleanTool(line))
	m.addLine(line)
}

func completeUTF8(b []byte) int {
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			if utf8.FullRune(b[i:]) {
				return len(b)
			}
			return i
		}
	}
	return len(b)
}

// replay reads a saved session into the viewer, as if it had just run.
func (m *Model) replay() {
	m.exited = true
	if !strings.HasSuffix(m.o.Replay, ".raw.log") {
		m.o.Plain = true // already drawn: just text
	}
	data, err := os.ReadFile(m.o.Replay)
	if err != nil {
		m.err, m.code = err, -1
		m.entries = append(m.entries, &Entry{Kind: KindTool, Text: "can't read it: " + err.Error(), TextTone: ToneRed, Time: time.Now()})
		return
	}
	if info, err := os.Stat(m.o.Replay); err == nil {
		m.started = info.ModTime()
	}
	if base := filepath.Base(m.o.Replay); len(base) > len(timeLayout) {
		if t, err := time.ParseInLocation(timeLayout, base[:len(timeLayout)], time.Local); err == nil {
			m.started = t
		}
	}
	day, last := m.started, m.started
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		var t time.Time
		t, line = splitRaw(strings.TrimSuffix(line, "\r"), day, last)
		m.addLineAt(line, t)
		last = t
	}
	if m.partial != "" {
		m.addLineAt(m.partial, last)
		m.partial = ""
	}
	m.took = last.Sub(m.started)
	m.dirty = true
}

func (m *Model) addLine(line string) { m.addLineAt(line, time.Now()) }

func (m *Model) addLineAt(line string, now time.Time) {
	res := m.parser.Line(line, now)
	e := res.New
	if e == nil {
		return
	}
	m.entries = append(m.entries, e)
	if len(m.entries) > maxEntries {
		m.entries = m.entries[len(m.entries)-maxEntries:]
	}
	if m.raw != nil {
		for _, r := range e.Raw {
			fmt.Fprintln(m.raw, rawLine(e.Time, r))
		}
	}
	if w := lipgloss.Width(e.Tag); e.Category == "network" {
		m.growTag(len(e.method()))
	} else if e.Kind != KindTool {
		m.growTag(w)
	}
	m.counts[e.Category]++
	if e.Category == "error" {
		m.errors++
	}
	if m.expandAll && e.Category == "network" && len(e.Details) > 0 {
		e.Expanded = true
	}
	m.keep(e)
	m.tracked(e)
	if e.Level == "reload" {
		m.toast, m.toastUntil = e.Text, time.Now().Add(4*time.Second)
	}
	if !m.follow && m.visible(e) {
		m.unseen++
	}
}

// growTag widens the tag column; everything is drawn again when it does.
func (m *Model) growTag(w int) {
	if w > m.maxTag && m.maxTag < 18 {
		m.maxTag = w
		if m.look.Layout != config.LayoutLines {
			m.version++
		}
	}
}

func (m *Model) visible(e *Entry) bool {
	if e.cleared {
		return false
	}
	if m.o.Plain {
		return m.matches(e, m.terms)
	}
	if m.on["raw"] || e.Kind == KindTool {
		return true
	}
	if !m.on[e.Category] {
		return false
	}
	return m.matches(e, m.terms)
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	if m.filtering {
		switch msg.String() {
		case "enter":
			m.filtering = false
			m.setFilter(m.input.Value())
		case "esc":
			m.filtering = false
			m.resize()
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return cmd
		}
		return nil
	}

	if m.vault != nil {
		return m.vaultKey(msg)
	}
	if m.dialog != nil {
		return m.dialogKey(msg)
	}
	if m.sel != nil {
		if cmd, ok := m.selKey(msg); ok {
			return cmd
		}
	}
	if k := msg.String(); m.look.KeyOff(k) {
		m.setStatus("⊘ " + k + " is turned off · l opens the settings")
		return nil
	}
	switch msg.String() {
	case "ctrl+s":
		if m.o.Replay != "" {
			m.setStatus("this is a saved log already")
			return nil
		}
		m.saveNow()
		return nil
	case "up":
		m.scrollUp(1)
		return nil
	case "down":
		m.scrollDown(1)
		return nil
	case "shift+up", "shift+down":
		m.startSelecting()
		return nil
	case "pgup":
		return m.glide(-(m.vp.Height() - 1))
	case "pgdown":
		return m.glide(m.vp.Height() - 1)
	case "ctrl+l":
		m.clearScreen("cleared")
		return nil
	case "home":
		m.refresh()
		m.vp.GotoTop()
		m.follow = m.vp.AtBottom()
		return nil
	case "end":
		m.refresh()
		m.vp.GotoBottom()
		m.follow, m.unseen = true, 0
		return nil
	}

	// fdev's keys work from the start; they go to the command only while
	// it asks something (flutter's "Please choose one: " for a device).
	if m.exited || !m.prompting() {
		if !m.o.Plain {
			for _, t := range Toggles {
				if msg.Text == t.Key {
					m.toggle(t.Name)
					return nil
				}
			}
		}
		switch {
		case msg.Text == "z" || msg.String() == "f11":
			m.setFull(!m.full)
			return nil
		case msg.String() == "esc" && m.full:
			m.setFull(false)
			return nil
		case msg.Text == "e":
			m.openLongest()
			return nil
		case msg.String() == "tab":
			m.startSelecting()
			return nil
		case msg.Text == "k":
			m.openVault()
			return nil
		case msg.Text == "x" && !m.o.Plain:
			m.toggleExpandAll()
			return nil
		case msg.Text == "[":
			return m.nextBookmark(-1)
		case msg.Text == "]":
			return m.nextBookmark(1)
		case msg.Text == "/":
			return m.startFilter()
		case msg.Text == "?" || msg.Text == "0":
			m.toggleHelp()
			return nil
		case msg.Text == "l":
			return m.openLook()
		case msg.Text == "m":
			m.mouse = !m.mouse
			if m.mouse {
				m.setStatus("mouse on: click ↗ to open code, click a line to expand it")
			} else {
				m.setStatus("mouse off: select text with the mouse; m turns it back on")
			}
			return nil
		case msg.String() == "esc" && m.filter != "":
			m.setFilter("")
			return nil
		}
	}

	if m.exited {
		switch msg.String() {
		case "q", "esc", "enter", "ctrl+c":
			return m.done()
		}
		return nil
	}
	m.write(keyBytes(msg))
	return nil
}

// keyBytes is what the terminal would send flutter for a key.
func keyBytes(msg tea.KeyPressMsg) string {
	switch msg.String() {
	case "enter":
		return "\r"
	case "backspace":
		return "\x7f"
	case "esc":
		return "\x1b"
	case "tab":
		return "\t"
	case "space":
		return " "
	case "ctrl+c":
		return "\x03"
	case "ctrl+d":
		return "\x04"
	case "ctrl+z":
		return "\x1a"
	}
	return msg.Text
}

func (m *Model) write(s string) {
	if s != "" && m.pty != nil && !m.exited {
		_, _ = io.WriteString(m.pty, s)
	}
}

func (m *Model) done() tea.Cmd {
	if m.pty != nil {
		_ = m.pty.Close()
	}
	if m.raw != nil {
		_ = m.raw.Close()
		m.raw = nil
	}
	msg := DoneMsg{Code: m.code, Err: m.err, Took: m.took, Errors: m.errors, Replay: m.o.Replay != ""}
	return func() tea.Msg { return msg }
}

func (m *Model) saveNow() {
	if path, err := m.save(); err != nil {
		m.setStatus("✘ can't save: " + err.Error())
	} else {
		m.setStatus("✓ saved " + path)
	}
}

func (m *Model) toggleHelp() {
	m.help.ShowAll = !m.help.ShowAll
	desc := "more"
	if m.help.ShowAll {
		desc = "close help"
	}
	m.keys.Help.SetHelp("?", desc)
	m.resize()
}

func (m *Model) toggle(name string) {
	m.on[name] = !m.on[name]
	if m.o.Settings != nil {
		m.o.Settings[name] = m.on[name]
	}
	m.changed()
}

func (m *Model) startFilter() tea.Cmd {
	m.filtering = true
	m.input.SetValue(m.filter)
	m.input.CursorEnd()
	m.resize()
	return m.input.Focus()
}

// changed re-renders everything for new settings and follows the output.
func (m *Model) changed() {
	m.version++
	m.follow, m.unseen = true, 0
	m.resize()
}

func (m *Model) setStatus(s string) {
	m.status, m.statusUntil = s, time.Now().Add(4*time.Second)
}

func (m *Model) scrollUp(n int) {
	m.refresh()
	m.vp.ScrollUp(n)
	m.follow = m.vp.AtBottom()
}

func (m *Model) scrollDown(n int) {
	m.refresh()
	m.vp.ScrollDown(n)
	if m.vp.AtBottom() {
		m.follow, m.unseen = true, 0
	}
}

// refresh puts the visible rows in the viewport, if anything changed.
func (m *Model) refresh() {
	if !m.dirty {
		return
	}
	m.dirty = false
	var rows []row
	spacer := ""
	if m.look.Layout == config.LayoutTable {
		spacer = strings.TrimRight(m.cols().blank(), " ")
	}
	groups := m.grouping()
	var chosen map[*Entry]bool
	if sel := m.selected(); len(sel) > 0 {
		chosen = map[*Entry]bool{}
		for _, e := range sel {
			chosen[e] = true
		}
	}
	for _, e := range m.entries {
		if !m.visible(e) {
			continue
		}
		if groups != nil {
			if label, ok := groups.next(e); ok {
				rows = append(rows, row{text: m.groupRule(label)})
			}
		}
		er := m.entryRows(e, m.width)
		if m.unfolding != nil && m.unfolding.entry == e {
			er = m.unfoldRows(e, m.width)
		}
		if m.lit == e {
			er = m.litRows(er)
		}
		if chosen[e] {
			er = m.selRows(er, e == m.sel)
		}
		rows = append(rows, er...)
		if e.Kind != KindTool && !m.on["raw"] {
			for i := 0; i < m.look.Spacing; i++ {
				rows = append(rows, row{text: spacer})
			}
		}
	}
	switch {
	case m.task != nil && !m.exited:
		rows = append(rows, row{text: m.taskRow()})
	case m.partial != "" && !CouldBeLog(m.partial):
		rows = append(rows, row{text: ansi.Truncate(cleanTool(m.partial), m.width, "…")})
	}
	m.rows = rows
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = r.text
	}
	offset := m.vp.YOffset()
	m.vp.SetContentLines(lines)
	if m.follow {
		m.vp.GotoBottom()
	} else {
		m.vp.SetYOffset(offset)
	}
}

func (m *Model) click(x, y int, shift bool) tea.Cmd {
	m.fitHeight()
	m.refresh()
	switch m.headerAt(x, y) {
	case "full":
		m.setFull(!m.full)
		return nil
	case "fdev":
		version.Shown = !version.Shown
		return nil
	}
	if act := m.selBarAt(x, y); act != "" {
		return m.selDo(strings.TrimPrefix(act, "sel:"))
	}
	if _, _, ok := m.stickyAt(x, y); ok && !m.overlay() {
		return m.clickSticky(x, y)
	}
	h := m.vp.Height()
	top := m.logTop() // the first row of the logs
	if bar := y - top - h; bar >= 0 && bar < m.toggleRows() {
		return m.clickBar(m.barItemAt(x, y))
	}
	if row := y - top - h - m.toggleRows(); row >= 0 && row < m.commandRows() {
		return m.clickCommand(m.commandAt(x, y))
	}
	if m.overlay() || y < top || y >= top+h {
		return nil
	}
	i := m.vp.YOffset() + y - top
	if i < 0 || i >= len(m.rows) || m.rows[i].entry == nil {
		return nil
	}
	r := m.rows[i]
	e := r.entry
	if icon := m.cols().iconX(); r.first && e.At != "" && !m.on["raw"] && x >= icon && x < icon+2 {
		return m.open(e)
	}
	if r.more {
		m.openDialog(e)
		return nil
	}
	if e.Level == "cleared" {
		return nil
	}
	if e.hidden {
		e.hidden, e.cached = false, cacheKey{}
		m.dirty = true
		return nil
	}
	// A click selects the log (shift stretches the selection to it), and
	// opens or closes it.
	was := m.sel
	m.selectEntry(e, shift)
	m.resize()
	if shift || (r.first && x >= m.width-12 && was != e && !expandable(e, m.on["details"])) {
		return nil
	}
	if expandable(e, m.on["details"]) {
		// While following, new lines push the output up; while scrolled back,
		// the clicked line stays where it is.
		return m.toggleExpanded(e)
	}
	return nil
}

func expandable(e *Entry, details bool) bool {
	return (len(e.Details) > 0 && (!details || len(e.Details) > maxBody)) || len(e.Stack) > maxStack || e.clipped
}

func (m *Model) open(e *Entry) tea.Cmd {
	loc, err := editor.Resolve(m.o.Root, e.At)
	if err != nil {
		m.setStatus("✘ " + err.Error())
		return nil
	}
	cmd, inTerminal, err := editor.Command(loc, m.o.Editor)
	if err != nil {
		m.setStatus("✘ " + err.Error())
		return nil
	}
	what := fmt.Sprintf("%s:%d", relative(m.o.Root, loc.File), loc.Line)
	if inTerminal {
		return tea.ExecProcess(cmd, func(err error) tea.Msg { return openedMsg{what, err} })
	}
	return func() tea.Msg {
		err := cmd.Start()
		if err == nil {
			go func() { _ = cmd.Wait() }()
		}
		return openedMsg{what, err}
	}
}

func relative(root, path string) string {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	if rel, err := filepath.Rel(realRoot, path); err == nil {
		return rel
	}
	return path
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m *Model) View() (string, tea.MouseMode) {
	m.fitHeight()
	m.refresh()
	parts := []string{m.header()}
	if m.stickyRows() > 0 {
		parts = append(parts, m.stickyView()...)
	}
	switch {
	case m.lookForm != nil || m.saveForm != nil:
		pad := lipgloss.NewStyle().Padding(1, 2)
		if m.compact() {
			pad = lipgloss.NewStyle().Padding(0, 1)
		}
		f := m.lookForm
		if m.saveForm != nil {
			f = m.saveForm
		}
		form := pad.Render(f.View())
		parts = append(parts, lipgloss.PlaceVertical(m.logHeight(), lipgloss.Top, form))
	case m.vault != nil:
		parts = append(parts, m.vaultView(m.logHeight()+m.headerRows()))
	case m.dialog != nil:
		parts = append(parts, m.dialogView(m.logHeight()+m.headerRows()))
	case m.headerRows() > 0:
		parts = append(parts, m.tableHeader(m.cols())...)
		parts = append(parts, m.vp.View())
	default:
		parts = append(parts, m.vp.View())
	}
	if m.toggleRows() > 0 {
		parts = append(parts, m.toggleBar())
	}
	if m.commandRows() > 0 {
		parts = append(parts, m.commandBar())
	}
	if m.helpRows() > 0 {
		parts = append(parts, m.footer())
	}
	out := strings.Join(parts, "\n")
	if m.rtl() {
		out = visualLines(out)
	}
	mode := tea.MouseModeNone
	if m.mouse {
		mode = tea.MouseModeAllMotion // motion too, for the bar's hover
	}
	return out, mode
}

func (m *Model) header() string {
	th := m.th
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	fdev := m.fdevBadge()
	left := fdev + th.Badge(th.Cream, th.Purple).Render(m.o.Title)
	if m.o.Version != "" {
		left += th.Badge(th.Pink, th.BarBg).Render(m.o.Version)
	}
	left += " " + m.o.Command
	if m.o.Note != "" {
		left += muted.Render(" • " + m.o.Note)
	}
	if m.toast != "" && time.Now().Before(m.toastUntil) {
		left = fdev + " " + lipgloss.NewStyle().Foreground(th.Green).Render("✓ "+m.toast)
	}
	// In full screen there is no footer: what it would say goes here.
	if m.full && m.status != "" && time.Now().Before(m.statusUntil) {
		left = fdev + " " + m.statusText()
	}

	var right string
	switch {
	case m.o.Replay != "":
		right = th.Badge(th.Cream, th.Indigo).Render("saved") + muted.Render(" "+m.started.Format("Mon 2 Jan 15:04")+" · "+short(m.took))
	case !m.exited:
		right = m.spin.View() + muted.Render("running "+short(time.Since(m.started)))
		if m.raw != nil {
			right = lipgloss.NewStyle().Foreground(th.Red).Render("● rec") + muted.Render(" • ") + right
		}
	case m.code == 0 && m.err == nil:
		right = th.Badge(th.Cream, th.Green).Render("exited") + muted.Render(" after "+short(m.took))
	default:
		right = th.Badge(th.Cream, th.Red).Render(fmt.Sprintf("exit %d", m.code)) + muted.Render(" after "+short(m.took))
	}
	if m.errors > 0 {
		right = lipgloss.NewStyle().Foreground(th.Red).Render(plural(m.errors, "error")) + muted.Render(" • ") + right
	}
	if n := m.parser.InFlight(); n > 0 && !m.exited {
		right = lipgloss.NewStyle().Foreground(th.Indigo).Render(fmt.Sprintf("%s %d waiting", spinFrame(), n)) + muted.Render(" • ") + right
	}
	if m.full {
		if m.filter != "" {
			right = lipgloss.NewStyle().Foreground(th.Pink).Render("/ "+ansi.Truncate(m.filter, 16, "…")) + muted.Render(" • ") + right
		}
		if !m.follow {
			right = lipgloss.NewStyle().Foreground(th.Pink).Render(fmt.Sprintf("↓ %d new", m.unseen)) + muted.Render(" • ") + right
		}
		right += " " + m.fullChip()
	}
	right += " "
	space := m.width - lipgloss.Width(right)
	left = ansi.Truncate(left, max(space-1, 0), "…")
	return left + strings.Repeat(" ", max(space-lipgloss.Width(left), 1)) + right
}

// fullChip is the header's button out of full screen.
func (m *Model) fullChip() string {
	s := lipgloss.NewStyle().Padding(0, 1).Background(m.th.BarBg).Foreground(m.th.Muted)
	if m.hover == "full" {
		s = s.Foreground(m.th.Pink).Underline(true)
	}
	return s.Render("⤡ exit · z")
}

// fdevBadge is fdev's name at the header's left; clicking it shows its
// version by it, or hides it.
func (m *Model) fdevBadge() string {
	s := m.th.Badge(m.th.Cream, m.th.Fuchsia).Bold(true)
	if m.hover == "fdev" {
		s = s.Underline(true)
	}
	return s.Render(version.Name())
}

// headerAt is "fdev" over fdev's name and "full" over the header's full
// screen button.
func (m *Model) headerAt(x, y int) string {
	if y != 0 {
		return ""
	}
	if x >= 0 && x < lipgloss.Width(m.fdevBadge()) {
		return "fdev"
	}
	if !m.full {
		return ""
	}
	w := lipgloss.Width(m.fullChip())
	if x0 := m.width - 1 - w; x >= x0 && x < x0+w {
		return "full"
	}
	return ""
}

func (m *Model) shortHelp() []key.Binding {
	k := m.keys
	switch {
	case m.exited:
		return []key.Binding{k.Back, k.Scroll, k.Filter, k.Full, k.Help}
	case m.o.Plain:
		return []key.Binding{k.Scroll, k.Page, k.Filter, k.Save, k.Full}
	case !m.parser.Ready:
		return []key.Binding{k.Toggles, k.Filter, k.Select, k.Values, k.Look, k.Full, k.Help}
	}
	// The command's own keys are buttons, with their keys, in the command bar.
	return []key.Binding{k.Scroll, k.Toggles, k.Filter, k.Select, k.Values, k.Full, k.Help}
}

func (m *Model) fullHelp() [][]key.Binding {
	k := m.keys
	if m.o.Plain {
		return [][]key.Binding{{k.Scroll, k.Page, k.Ends}, {k.Filter, k.Back}, {k.Save, k.Full}}
	}
	return [][]key.Binding{
		{k.Reload, k.Restart, k.Quit},
		{k.Scroll, k.Page, k.Ends},
		{k.Toggles, k.Parts, k.Filter},
		{k.Select, k.Range, k.Marks, k.Values},
		{k.Open, k.Mouse, k.Expand, k.Window},
		{k.Look, k.Save, k.Keep, k.Wipe, k.Full},
	}
}

func (m *Model) footer() string {
	th := m.th
	pad := lipgloss.NewStyle().PaddingLeft(1)
	switch {
	case m.filtering:
		m.input.SetWidth(m.width - 12)
		return pad.Render(m.input.View())
	case m.status != "" && time.Now().Before(m.statusUntil):
		return pad.Render(m.statusText())
	case m.sel != nil && !m.overlay():
		line, _ := m.selBar()
		return line
	case m.help.ShowAll:
		return pad.Render(m.help.FullHelpView(m.fullHelp()))
	case !m.follow:
		more := m.help.Styles.ShortSeparator.Render(" • ") + m.help.ShortHelpView([]key.Binding{m.keys.Ends})
		return pad.Render(lipgloss.NewStyle().Foreground(th.Pink).Render(fmt.Sprintf("↓ %d new", m.unseen)) + more)
	case !m.exited && m.prompting():
		return pad.Render(lipgloss.NewStyle().Foreground(th.Pink).Render("✎ the command is asking: your keys go to it"))
	}
	return pad.Render(m.help.ShortHelpView(m.shortHelp()))
}

// statusText is the status in its color: red for an error, yellow for a
// key that is off.
func (m *Model) statusText() string {
	c := m.th.Green
	switch {
	case strings.HasPrefix(m.status, "✘"):
		c = m.th.Red
	case strings.HasPrefix(m.status, "⊘"):
		c = m.th.Warn
	}
	return lipgloss.NewStyle().Foreground(c).Render(m.status)
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func short(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %02ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %02dm", s/3600, s%3600/60)
}

// ── Look ─────────────────────────────────────────────────────────────────────

func (m *Model) openLook() tea.Cmd {
	m.formLook, m.formUI, m.savedUI = m.look, m.ui, m.ui
	m.lookForm = SettingsForm(&m.formLook, &m.formUI, m.th, m.o.About).WithShowHelp(!m.full)
	keys := huh.NewDefaultKeyMap()
	quit := []string{"esc", "ctrl+c"}
	if m.look.KeyOff("ctrl+c") {
		quit = quit[:1]
	}
	keys.Quit = key.NewBinding(key.WithKeys(quit...), key.WithHelp("esc", "cancel"))
	m.lookForm.WithKeyMap(keys).WithWidth(min(m.width-4, 76))
	m.lookForm.Update(tea.WindowSizeMsg{Width: m.width, Height: m.logHeight()})
	m.resize()
	return m.lookForm.Init()
}

func (m *Model) updateLookForm(msg tea.Msg) tea.Cmd {
	f, cmd := m.lookForm.Update(msg)
	if form, ok := f.(*huh.Form); ok {
		m.lookForm = form
	}
	switch m.lookForm.State {
	case huh.StateAborted:
		m.lookForm = nil
		m.setUI(m.savedUI, false) // what it was before trying others
		return nil
	case huh.StateCompleted:
		m.lookForm = nil
		m.setUI(m.formUI, true)
		m.applyLook(m.formLook)
		return nil
	}
	// A theme or size is tried as soon as it is under the cursor.
	if m.formUI != m.ui {
		m.setUI(m.formUI, false)
	}
	return cmd
}

// setUI shows a theme and size; save keeps them.
func (m *Model) setUI(ui config.UI, save bool) {
	ui = ui.WithDefaults()
	changed := ui != m.ui
	m.ui = ui
	if m.o.OnUI != nil {
		m.o.OnUI(ui, save) // the caller knows the terminal, for Auto: it calls SetTheme
	} else if changed {
		m.SetTheme(theme.Named(ui.Theme, m.th.Dark))
	}
	m.resize()
	m.resizePty()
}

func (m *Model) applyLook(look config.Look) {
	m.look = look.WithDefaults()
	if m.o.OnLook != nil {
		m.o.OnLook(m.look)
	}
	switch {
	case m.look.Save && m.raw == nil:
		if err := m.startRecording(); err != nil {
			m.setStatus("✘ can't save logs: " + err.Error())
		} else {
			m.setStatus("● saving this session to " + m.look.Dir)
		}
	case !m.look.Save && m.raw != nil:
		_ = m.raw.Close()
		m.raw = nil
	}
	m.offKeys()
	m.changed()
}

// offKeys takes the turned-off shortcuts out of the help.
func (m *Model) offKeys() {
	m.keys.Wipe.SetEnabled(!m.look.KeyOff("ctrl+l"))
	m.keys.Save.SetEnabled(!m.look.KeyOff("ctrl+s"))
}

// Typing is whether keys go into a text field (the filter, a saved log's
// subject), not to shortcuts: as typed, in any keyboard layout.
func (m *Model) Typing() bool { return m.filtering || m.saveForm != nil }

// toggleExpandAll opens every network log with details, or closes them.
func (m *Model) toggleExpandAll() {
	m.expandAll = !m.expandAll
	n := 0
	for _, e := range m.entries {
		if e.Category == "network" && len(e.Details) > 0 && e.Expanded != m.expandAll {
			e.Expanded = m.expandAll
			n++
		}
	}
	if m.expandAll {
		m.setStatus(fmt.Sprintf("⊞ %s open, and new ones open as they come · x closes them", plural(n, "call")))
	} else {
		m.setStatus("⊟ calls closed")
	}
	m.dirty = true
}

// prompting is whether the command is waiting for an answer: its unfinished
// last line reads like a question ("Please choose one (or "q" to quit): ").
func (m *Model) prompting() bool {
	if m.exited || m.partial == "" || m.task != nil || CouldBeLog(m.partial) {
		return false
	}
	line := strings.TrimRight(ansi.Strip(overwrite(m.partial)), " ")
	if line == "" {
		return false
	}
	return strings.ContainsAny(line[len(line)-1:], ":?]>)")
}
