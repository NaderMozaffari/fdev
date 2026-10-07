// Package launcher is fdev's menu: pick the platform, flavor and section,
// then a target; answer its questions (store edition, device, ...), run it
// in the log viewer, and come back. Saved logs open in the viewer too.
//
// The menu is a Bubbles list and the questions are Huh forms, both in
// Charm's own styles; the steps before the menu are fdev's own (picker.go).
package launcher

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/devices"
	"github.com/NaderMozaffari/fdev/internal/editor"
	"github.com/NaderMozaffari/fdev/internal/keys"
	"github.com/NaderMozaffari/fdev/internal/logview"
	"github.com/NaderMozaffari/fdev/internal/state"
	"github.com/NaderMozaffari/fdev/internal/theme"
	"github.com/NaderMozaffari/fdev/internal/version"
)

type screen int

const (
	screenMenu screen = iota
	screenPick
	screenAsk
	screenDevices
	screenLogs
)

// item is a row of the menu: a target, a recent run of one, or a saved log.
type item struct {
	title, desc string
	target      *config.Target
	recent      *job
	session     *logview.Session
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title + " " + i.desc }

// key identifies an item across menu rebuilds, to keep it selected.
func (i item) key() string {
	switch {
	case i.session != nil:
		return "log:" + i.session.Base
	case i.recent != nil:
		return "recent:" + commandLine(i.recent.target.Name, i.recent.env)
	case i.target != nil:
		return "target:" + i.target.Name
	}
	return ""
}

// job is a target with its answers, ready to run.
type job struct {
	target *config.Target
	env    map[string]string
	note   string
	at     time.Time // when it last ran, for recent runs
	asks   []string  // still to answer
	asked  []string  // answered, for going back
	auto   map[string]bool
	// want is the device a recent run used: taken without asking while it
	// is connected. wantName is its name.
	want, wantName string
}

// deviceChoice is an answer to a device question.
type deviceChoice struct {
	id, name   string
	retry, sim bool
}

type devicesMsg struct {
	list []devices.Device
	err  error
}

type openedMsg struct {
	what string
	err  error
}

// toolDoneMsg is a builtin tool (fdev wifi) back from the terminal.
type toolDoneMsg struct {
	name string
	err  error
}

type Model struct {
	cfg   *config.Config
	state *state.State
	th    theme.Theme

	screen      screen
	list        list.Model
	form        *huh.Form // the settings
	lookAnswer  config.Look
	uiAnswer    config.UI
	editingLook bool
	// ui is the theme and size shown: the saved ones (state.UI), or ones
	// being tried in the settings. termDark is whether the terminal is.
	ui       config.UI
	termDark bool
	full     bool   // full screen: no keys or buttons
	lookFrom screen // where the settings go back to
	job      *job
	// A target's question, shown in the picker (questions.go).
	asking   bool
	qTitle   string
	qHint    string
	qChoices []choice
	logs     *logview.Model
	ran      job
	spin     spinner.Model
	platform string

	branch  string
	changed int
	width   int
	height  int
	start   string // a target to open straight away (fdev <target>)

	// What the menu shows, picked in steps (picker.go).
	menuPlatform string
	menuFlavor   string
	tab          string // a target group, Recent or Saved logs
	step         pickStep
	pickIndex    int
	enterStart   time.Time // when the step opened, for its rows coming in

	sessions      []logview.Session
	confirmDelete string // the saved log x was pressed on once

	listW     int       // the list's width; the details panel has the rest
	menuRowH  int       // the menu's rows: 3, 2 or 1 lines, by the size
	menuGap   int       // and the blank lines between them
	hover     string    // the zone under the mouse, kind:value
	zones     []zone    // what the last view can click
	animStart time.Time // when the selection last moved, for its animation
	lastIndex int
}

func New(cfg *config.Config, st *state.State, start string) *Model {
	m := &Model{cfg: cfg, state: st, start: start, width: 80, height: 24, ui: st.UI.WithDefaults(), termDark: true}
	m.branch, m.changed = gitInfo(cfg.Root)
	m.list = list.New(nil, list.NewDefaultDelegate(), m.width, m.height)
	m.list.SetStatusBarItemName("target", "targets")
	m.list.StatusMessageLifetime = time.Minute
	// bubbles v2.2.1 binds Quit to "v"; fdev quits with q. ← and → are
	// back and run here, so paging is pgup/pgdown only.
	m.list.KeyMap.Quit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	m.list.KeyMap.NextPage = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "next page"))
	m.list.KeyMap.PrevPage = key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "prev page"))
	// The keys are on the buttons' row, across the whole screen.
	m.list.SetShowHelp(false)
	m.spin = spinner.New(spinner.WithSpinner(spinner.Dot))
	m.setTheme(theme.Named(m.ui.Theme, m.termDark))
	if menu := st.Menu; menu != nil {
		m.tab, m.menuPlatform, m.menuFlavor = menu["tab"], menu["platform"], menu["flavor"]
	}
	if t := cfg.Target(start); t != nil { // the menu fdev comes back to
		if p := t.PlatformName(); p != "" {
			m.menuPlatform = p
		}
		if t.Flavor != "" {
			m.menuFlavor = t.Flavor
		}
		m.tab = t.Group
	}
	m.loadSessions()
	m.fixFilters()
	m.buildMenu()
	return m
}

func (m *Model) setTheme(th theme.Theme) {
	m.th = th
	m.list.Styles = th.List()
	m.list.Styles.Title = m.list.Styles.Title.Background(sectionColor(m.tab))
	m.list.SetDelegate(menuDelegate{m})
	m.spin.Style = lipgloss.NewStyle().Foreground(m.th.Fuchsia)
	if m.logs != nil {
		m.logs.SetTheme(m.th)
	}
	if m.form != nil {
		m.form.WithTheme(th.Huh())
	}
}

// showUI shows a theme and size: the saved ones, or ones being tried.
func (m *Model) showUI(ui config.UI) {
	m.ui = ui.WithDefaults()
	m.setTheme(theme.Named(m.ui.Theme, m.termDark))
	m.resizeList()
}

// logsUI is the log viewer's settings trying a theme and size, or saving.
func (m *Model) logsUI(ui config.UI, save bool) {
	m.showUI(ui)
	if save {
		m.state.UI = m.ui
		m.state.SaveUI()
	}
}

// toggleFull goes in or out of full screen: the menus without their keys
// and buttons.
func (m *Model) toggleFull() tea.Cmd {
	m.full = !m.full
	m.resizeList()
	if m.full && m.screen == screenMenu {
		return m.list.NewStatusMessage(m.fg(m.th.Muted).Render("full screen · z or ⤡ at the top brings the keys back"))
	}
	return nil
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	if m.cfg.Warning != "" {
		cmds = append(cmds, m.list.NewStatusMessage(m.fg(m.th.Red).Render("⚠ "+m.cfg.Warning)))
	}
	t := m.cfg.Target(m.start)
	switch {
	case t != nil:
		cmds = append(cmds, m.choose(t))
	case m.start != "":
		cmds = append(cmds, m.list.NewStatusMessage(m.fg(m.th.Red).Render(fmt.Sprintf("no target %q here", m.start))))
		fallthrough
	default:
		cmds = append(cmds, m.startPicker())
	}
	return tea.Batch(cmds...)
}

func (m *Model) fg(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

// buildMenu lists the section picked, keeping the selected item.
func (m *Model) buildMenu() {
	keep := ""
	if it, ok := m.list.SelectedItem().(item); ok {
		keep = it.key()
	}
	var items []list.Item
	switch m.tab {
	case recentTab:
		for _, j := range m.recentJobs() {
			j := j
			desc := ago(j.at)
			if j.note != "" {
				desc += " · " + j.note
			}
			shown := j.env
			if j.note != "" { // the device name is in the description; its id is noise
				shown = map[string]string{}
				for k, v := range j.env {
					if k != config.DeviceEnv {
						shown[k] = v
					}
				}
			}
			items = append(items, item{title: "↺ " + commandLine(j.target.Name, shown), desc: desc, recent: &j})
		}
		m.list.SetStatusBarItemName("run", "runs")
	case logsTab:
		for _, s := range m.savedLogs() {
			s := s
			title := sessionWhen(s) + " · " + s.Target
			if s.Subject != "" {
				title = sessionWhen(s) + " · " + s.Subject
			}
			if s.Starred {
				title = "★ " + title
			}
			desc := s.Command
			if desc == "" {
				desc = s.Target
			}
			if s.Note != "" {
				desc += " · " + s.Note
			}
			for _, t := range s.Tags {
				desc += " #" + t
			}
			items = append(items, item{title: title, desc: desc, session: &s})
		}
		m.list.SetStatusBarItemName("saved log", "saved logs")
	default:
		for _, t := range m.tabTargets(m.tab) {
			items = append(items, item{title: t.Name, desc: t.Desc, target: t})
		}
		m.list.SetStatusBarItemName("target", "targets")
	}
	m.list.SetItems(items)
	m.list.Select(0)
	for i, it := range items {
		if keep != "" && it.(item).key() == keep {
			m.list.Select(i)
		}
	}
	m.lastIndex = -1
	title := []string{tabSign(m.tab) + " " + m.tab}
	for _, v := range []string{m.activePlatform(), m.activeFlavor()} {
		if v != "" {
			title = append(title, v)
		}
	}
	m.list.Title = strings.Join(title, " · ")
	m.list.Styles.Title = m.list.Styles.Title.Background(sectionColor(m.tab))
	m.resizeList()
}

func tabSign(tab string) string {
	switch strings.ToLower(tab) {
	case "recent":
		return "↺"
	case "run":
		return "▶"
	case "build":
		return "■"
	case "tools", "other":
		return "✦"
	case strings.ToLower(logsTab):
		return "☰"
	}
	return "•"
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// resizeList fits the list under the header and above the buttons, with
// rows as big as the size says (for auto, the biggest that fits them all).
func (m *Model) resizeList() {
	m.listW = m.width
	if m.width >= 96 { // room for the details panel
		m.listW = min(max(m.width*11/20, 50), 76)
	}
	h := max(m.height-m.headerHeight()-m.bottomRows(), 5)
	avail := h - 4 // the list's title and status bar; no page dots when all fit
	n := len(m.list.VisibleItems())
	m.menuRowH, m.menuGap = menuRows(m.sizeFor(func(size string) bool {
		rowH, gap := menuRows(size)
		return n*(rowH+gap) <= avail
	}))
	m.list.SetDelegate(menuDelegate{m})
	m.list.SetSize(m.listW, h)
}

// menuRows is the lines of a menu row, and between rows, for a size.
func menuRows(size string) (rowH, gap int) {
	switch size {
	case config.SizeLarge:
		return 3, 1
	case config.SizeMedium:
		return 2, 1
	}
	return 1, 0
}

// sizeFor is the size set, or for auto the biggest that fits.
func (m *Model) sizeFor(fits func(size string) bool) string {
	if m.ui.Size != config.SizeAuto && m.ui.Size != "" {
		return m.ui.Size
	}
	for _, size := range []string{config.SizeLarge, config.SizeMedium} {
		if fits(size) {
			return size
		}
	}
	return config.SizeSmall
}

// compact is whether the screens leave out their blank lines: for a short
// window, or the small size.
func (m *Model) compact() bool {
	switch m.ui.Size {
	case config.SizeSmall:
		return true
	case config.SizeMedium, config.SizeLarge:
		return false
	}
	return m.height < 26
}

// bottomRows is the rows under the menu: the buttons and keys, none in
// full screen.
func (m *Model) bottomRows() int {
	if m.full {
		return 0
	}
	return 1
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && !m.typing() {
		msg = keys.Latin(k) // shortcuts work in any keyboard layout
	}
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if !theme.Painted(msg.Color) { // the terminal's own, not a theme's
			m.termDark = msg.IsDark()
			m.showUI(m.ui)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList()
	case animMsg:
		if m.animating() {
			return m, animTick()
		}
		return m, nil
	case openedMsg:
		if msg.err != nil {
			return m, m.list.NewStatusMessage(m.fg(m.th.Red).Render("✘ " + msg.err.Error()))
		}
		return m, m.list.NewStatusMessage(m.fg(m.th.Green).Render("✓ opened " + msg.what))
	case toolDoneMsg:
		m.screen = screenMenu
		var exit *exec.ExitError
		if errors.As(msg.err, &exit) { // it said why on the way
			return m, m.list.NewStatusMessage(m.fg(m.th.Muted).Render("■ " + msg.name + " • not connected"))
		}
		if msg.err != nil {
			return m, m.list.NewStatusMessage(m.fg(m.th.Red).Render("✘ " + msg.name + " • " + msg.err.Error()))
		}
		return m, m.list.NewStatusMessage(m.fg(m.th.Green).Render("✓ " + msg.name))
	}

	if m.screen == screenLogs && m.logs != nil {
		if done, ok := msg.(logview.DoneMsg); ok {
			return m, m.logsDone(done)
		}
		return m, m.logs.Update(msg)
	}

	switch msg := msg.(type) {
	case devicesMsg:
		return m, m.gotDevices(msg)
	case spinner.TickMsg:
		if m.screen == screenDevices {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" && !m.look().KeyOff("ctrl+c") {
			return m, tea.Quit
		}
		if !m.typing() && m.screen != screenAsk {
			switch {
			case msg.Text == "z" || msg.String() == "f11":
				return m, m.toggleFull()
			case msg.Text == "t" && (m.screen == screenMenu || m.screen == screenPick):
				return m, m.editLook()
			}
		}
	case tea.MouseMotionMsg:
		if m.screen == screenAsk || m.screen == screenDevices {
			m.hover = ""
			if z, ok := m.zoneAt(msg.X, msg.Y); ok {
				m.hover = z.kind + ":" + z.value
			}
		}
	case tea.MouseClickMsg:
		if (m.screen == screenAsk || m.screen == screenDevices) && msg.Button == tea.MouseLeft {
			if z, ok := m.zoneAt(msg.X, msg.Y); ok {
				return m, m.clickZone(z)
			}
		}
	}

	switch m.screen {
	case screenPick:
		return m, m.updatePick(msg)
	case screenDevices:
		if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "esc" || k.String() == "left") {
			return m, m.goBack()
		}
	case screenAsk:
		if k, ok := msg.(tea.KeyPressMsg); ok && !m.editingLook {
			switch k.String() {
			case "left":
				return m, m.goBack()
			case "right":
				return m, m.updateForm(tea.KeyPressMsg{Code: tea.KeyEnter})
			}
		}
		return m, m.updateForm(msg)
	case screenMenu:
		return m, m.updateMenu(msg)
	}
	return m, nil
}

func (m *Model) updateMenu(msg tea.Msg) tea.Cmd {
	filtering := m.list.FilterState() == list.Filtering
	if k, ok := msg.(tea.KeyPressMsg); ok && !filtering {
		if k.String() != "x" && k.String() != "delete" {
			m.confirmDelete = ""
		}
		switch k.String() {
		case "enter", "right":
			return m.runSelected()
		case "left":
			return m.backFromMenu(false)
		case "esc":
			if m.list.FilterState() == list.Unfiltered {
				return m.backFromMenu(true)
			}
		case "s":
			return m.editLook()
		case "*", "space":
			if m.tab == logsTab {
				return m.toggleStar()
			}
		case "x", "delete":
			if m.tab == logsTab {
				return m.deleteSession()
			}
		case "o":
			if m.tab == logsTab {
				return m.openSession()
			}
		}
	}
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return nil
		}
		if z, ok := m.zoneAt(msg.X, msg.Y); ok {
			return m.clickZone(z)
		}
		if i, ok := m.itemAt(msg.X, msg.Y); ok {
			if i == m.list.Index() {
				return m.runSelected()
			}
			m.list.Select(i)
		}
		return m.selectionMoved()
	case tea.MouseMotionMsg: // hovering selects
		m.hover = ""
		if z, ok := m.zoneAt(msg.X, msg.Y); ok {
			m.hover = z.kind + ":" + z.value
		}
		if i, ok := m.itemAt(msg.X, msg.Y); ok && i != m.list.Index() {
			m.list.Select(i)
		}
		return m.selectionMoved()
	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			m.list.CursorUp()
		} else if msg.Button == tea.MouseWheelDown {
			m.list.CursorDown()
		}
		return m.selectionMoved()
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return tea.Batch(cmd, m.selectionMoved())
}

// typing is whether keys are text being typed (a filter), not shortcuts.
func (m *Model) typing() bool {
	switch m.screen {
	case screenLogs:
		return m.logs != nil && m.logs.Typing()
	case screenMenu:
		return m.list.FilterState() == list.Filtering
	}
	return false
}

// itemAt is the index of the list item at a screen position.
func (m *Model) itemAt(x, y int) (int, bool) {
	// The list draws its title (2 rows) and status bar (2 rows) above the
	// items, and each item takes its rows and the blank ones after it.
	per := m.menuRowH + m.menuGap
	row := y - m.headerHeight() - 4
	if per <= 0 || row < 0 || row%per >= m.menuRowH || x >= m.listW || m.list.FilterState() == list.Filtering {
		return 0, false
	}
	i := row / per
	perPage := max(m.list.Paginator.PerPage, 1)
	index := m.list.Paginator.Page*perPage + i
	if i >= perPage || index >= len(m.list.VisibleItems()) {
		return 0, false
	}
	return index, true
}

// selectionMoved starts the selection's animation when it moved.
func (m *Model) selectionMoved() tea.Cmd {
	if m.list.Index() == m.lastIndex {
		return nil
	}
	m.lastIndex, m.animStart = m.list.Index(), time.Now()
	return animTick()
}

func (m *Model) runSelected() tea.Cmd {
	it, ok := m.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	switch {
	case it.session != nil:
		return m.replay(*it.session)
	case it.recent != nil:
		j := *it.recent
		return m.rerun(&j)
	}
	return m.choose(it.target)
}

// goBack is ← on a question: the question before, or the menu.
func (m *Model) goBack() tea.Cmd {
	if m.editingLook {
		m.form, m.editingLook, m.screen = nil, false, m.lookFrom
		m.showUI(m.state.UI) // not the theme or size being tried
		return nil
	}
	j := m.job
	for j != nil && len(j.asked) > 0 {
		name := j.asked[len(j.asked)-1]
		j.asked = j.asked[:len(j.asked)-1]
		j.asks = append([]string{name}, j.asks...)
		if !j.auto[name] { // skip answers nobody chose (the only device)
			return m.nextAsk()
		}
	}
	m.form, m.job, m.screen, m.asking = nil, nil, screenMenu, false
	return nil
}

// goNext is the next button on a question: the form's enter.
func (m *Model) goNext() tea.Cmd {
	if m.screen == screenMenu {
		return m.runSelected()
	}
	if m.screen == screenAsk {
		return m.updateForm(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	return nil
}

// ── Saved logs ───────────────────────────────────────────────────────────────

func (m *Model) selectedSession() *logview.Session {
	if it, ok := m.list.SelectedItem().(item); ok {
		return it.session
	}
	return nil
}

func (m *Model) toggleStar() tea.Cmd {
	s := m.selectedSession()
	if s == nil {
		return nil
	}
	if err := logview.Star(m.cfg.Root, m.look().Dir, s.Base, !s.Starred); err != nil {
		return m.list.NewStatusMessage(m.fg(m.th.Red).Render("✘ " + err.Error()))
	}
	starred := !s.Starred
	m.loadSessions()
	m.buildMenu()
	if starred {
		return m.list.NewStatusMessage(m.fg(m.th.Yellow).Render("★ starred: kept when old logs are cleaned up"))
	}
	return m.list.NewStatusMessage(m.fg(m.th.Muted).Render("☆ star removed"))
}

func (m *Model) deleteSession() tea.Cmd {
	s := m.selectedSession()
	if s == nil {
		return nil
	}
	if m.confirmDelete != s.Base {
		m.confirmDelete = s.Base
		return m.list.NewStatusMessage(m.fg(m.th.Red).Render("press x again to delete " + sessionWhen(*s) + " · " + s.Target))
	}
	m.confirmDelete = ""
	if err := logview.Delete(m.cfg.Root, m.look().Dir, *s); err != nil {
		return m.list.NewStatusMessage(m.fg(m.th.Red).Render("✘ " + err.Error()))
	}
	i := m.list.Index()
	m.loadSessions()
	if len(m.savedLogs()) == 0 {
		return m.backFromMenu(false)
	}
	m.buildMenu()
	m.list.Select(min(i, len(m.list.Items())-1))
	return m.list.NewStatusMessage(m.fg(m.th.Muted).Render("deleted"))
}

// openSession opens the readable log (or the raw one) in the editor.
func (m *Model) openSession() tea.Cmd {
	s := m.selectedSession()
	if s == nil {
		return nil
	}
	path := s.Log
	if path == "" {
		path = s.Raw
	}
	loc, err := editor.Resolve(m.cfg.Root, path+":1")
	if err != nil {
		return m.list.NewStatusMessage(m.fg(m.th.Red).Render("✘ " + err.Error()))
	}
	cmd, inTerminal, err := editor.Command(loc, m.cfg.Editor)
	if err != nil {
		return m.list.NewStatusMessage(m.fg(m.th.Red).Render("✘ " + err.Error()))
	}
	what := s.Base
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

// replay shows a saved log in the viewer.
func (m *Model) replay(s logview.Session) tea.Cmd {
	command := s.Command
	if command == "" {
		command = s.Target
	}
	logs, cmd := logview.New(logview.Options{
		Root: m.cfg.Root, Title: m.cfg.Title, Version: m.cfg.AppVersion(), About: m.about(),
		Command: command, Note: s.Note, Replay: s.Path(),
		Settings: m.state.Logs, Editor: m.cfg.Editor, Theme: m.th, Width: m.width, Height: m.height,
		Target: s.Target, Look: m.look(), OnLook: m.saveLook, UI: m.ui, OnUI: m.logsUI, Full: m.full,
		Values: m.cfg.Logs.Values,
	})
	m.logs, m.ran, m.screen = logs, job{}, screenLogs
	return cmd
}

func sessionWhen(s logview.Session) string {
	if s.Time.IsZero() {
		return s.Base
	}
	if time.Since(s.Time) < 6*24*time.Hour {
		return s.Time.Format("Mon 15:04")
	}
	return s.Time.Format("2 Jan 15:04")
}

// ── Settings and questions ───────────────────────────────────────────────────

// look is the log viewer settings: the user's, else fdev.yaml's.
func (m *Model) look() config.Look {
	if m.state.Look != nil {
		return m.state.Look.WithDefaults()
	}
	return m.cfg.Logs.WithDefaults()
}

func (m *Model) saveLook(look config.Look) {
	m.state.Look = &look
	m.state.Save()
}

func (m *Model) editLook() tea.Cmd {
	m.lookAnswer, m.uiAnswer, m.editingLook, m.lookFrom = m.look(), m.ui, true, m.screen
	m.form = logview.SettingsForm(&m.lookAnswer, &m.uiAnswer, m.th, m.about()).WithKeyMap(m.formKeys()).
		WithWidth(min(m.width-4, 76)).WithShowHelp(!m.full)
	return m.startForm()
}

func (m *Model) updateForm(msg tea.Msg) tea.Cmd {
	if m.form == nil {
		return nil
	}
	f, cmd := m.form.Update(msg)
	if form, ok := f.(*huh.Form); ok {
		m.form = form
	}
	switch m.form.State {
	case huh.StateAborted:
		return m.goBack()
	case huh.StateCompleted:
		m.form = nil
		m.editingLook, m.screen = false, m.lookFrom
		m.saveLook(m.lookAnswer)
		m.state.UI = m.uiAnswer.WithDefaults()
		m.state.SaveUI()
		m.showUI(m.state.UI)
		m.loadSessions()
		return m.list.NewStatusMessage(m.fg(m.th.Green).Render("✓ settings saved"))
	}
	// A theme or size is tried as soon as it is under the cursor.
	if m.editingLook && m.uiAnswer != m.ui {
		m.showUI(m.uiAnswer)
	}
	return cmd
}

// about is the app for the settings' About page: its version, where it
// is, and what fdev found in it.
func (m *Model) about() logview.About {
	a := logview.About{App: appTitle(m.cfg.Title, m.cfg.ShortVersion())}
	add := func(k, v string) {
		if v != "" {
			a.Facts = append(a.Facts, config.Fact{Key: k, Value: v})
		}
	}
	add("Version", m.cfg.Version)
	add("Build", m.cfg.Build)
	add("Project", version.Home(m.cfg.Root))
	if m.branch != "" {
		branch := m.branch
		if m.changed > 0 {
			branch += fmt.Sprintf(" · %d changed", m.changed)
		}
		add("Branch", branch)
	}
	targets := m.cfg.Targets()
	add("Targets", fmt.Sprintf("%d in %s, from %s", len(targets),
		plural(len(m.cfg.Groups), "group", "groups"), m.cfg.Source))
	add("Platforms", strings.Join(values(targets, platformOf, platformOrder), ", "))
	add("Flavors", strings.Join(values(targets, flavorOf, nil), ", "))
	add("Saved logs", fmt.Sprintf("%s in %s", plural(len(m.sessions), "session", "sessions"), m.look().Dir))
	return a
}

// choose starts a target: its questions first, then the run.
func (m *Model) choose(t *config.Target) tea.Cmd {
	m.job = &job{target: t, env: map[string]string{}, asks: append([]string(nil), t.Ask...), auto: map[string]bool{}}
	return m.nextAsk()
}

// rerun runs a recent run again. One that ran on a device looks for it
// first, and asks for another when it is not connected.
func (m *Model) rerun(j *job) tea.Cmd {
	id, ask := j.env[config.DeviceEnv], deviceAskOf(j.target)
	if id == "" || ask == "" {
		return m.run(j)
	}
	env := maps.Clone(j.env)
	delete(env, config.DeviceEnv)
	m.job = &job{target: j.target, env: env, asks: []string{ask}, auto: map[string]bool{}, want: id, wantName: j.note}
	return m.nextAsk()
}

// deviceAskOf is the target's device question, or "".
func deviceAskOf(t *config.Target) string {
	for _, a := range t.Ask {
		if _, ok := config.DeviceAsk(a); ok {
			return a
		}
	}
	return ""
}

func (m *Model) formKeys() *huh.KeyMap {
	keys := huh.NewDefaultKeyMap()
	quit := []string{"esc", "ctrl+c"}
	if m.look().KeyOff("ctrl+c") {
		quit = quit[:1]
	}
	keys.Quit = key.NewBinding(key.WithKeys(quit...), key.WithHelp("esc", "back"))
	return keys
}

func (m *Model) startForm() tea.Cmd {
	m.screen = screenAsk
	// Tell the form what the program already knows.
	bg := color.Color(color.White)
	if m.th.Dark {
		bg = color.Black
	}
	m.form.Update(tea.BackgroundColorMsg{Color: bg})
	m.form.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height - m.headerHeight() - m.pad().GetVerticalPadding() - m.bottomRows()})
	return m.form.Init()
}

func (m *Model) nextAsk() tea.Cmd {
	j := m.job
	if len(j.asks) == 0 {
		return m.run(j)
	}
	name := j.asks[0]
	if platform, ok := config.DeviceAsk(name); ok {
		m.screen, m.platform, m.asking = screenDevices, platform, false
		root := m.cfg.Root
		return tea.Batch(m.spin.Tick, func() tea.Msg {
			list, err := devices.List(root, platform)
			return devicesMsg{list, err}
		})
	}
	return m.askQuestion(name)
}

func (m *Model) answerAsk(value string) tea.Cmd {
	j := m.job
	name := j.asks[0]
	j.env[m.cfg.Asks[name].Env] = value
	m.state.Answers[name] = value
	j.asks, j.asked = j.asks[1:], append(j.asked, name)
	return m.nextAsk()
}

func (m *Model) answerDevice(v deviceChoice, auto bool) tea.Cmd {
	j := m.job
	switch {
	case v.retry:
		return m.nextAsk()
	case v.sim:
		open := func() tea.Msg {
			_ = exec.Command("open", "-a", "Simulator").Run()
			time.Sleep(3 * time.Second) // let it boot a device
			return nil
		}
		m.screen = screenDevices
		return tea.Sequence(open, m.nextAsk())
	}
	name := j.asks[0]
	j.env[config.DeviceEnv] = v.id
	j.note = v.name
	if v.id != "" {
		m.state.Answers[name] = v.id
	}
	j.asks, j.asked = j.asks[1:], append(j.asked, name)
	j.auto[name] = auto
	return m.nextAsk()
}

func (m *Model) gotDevices(msg devicesMsg) tea.Cmd {
	if m.screen != screenDevices || m.job == nil {
		return nil
	}
	for _, d := range msg.list {
		if j := m.job; d.ID == j.want || (j.want == "" && len(msg.list) == 1) {
			return m.answerDevice(deviceChoice{id: d.ID, name: d.Name}, true)
		}
	}
	return m.deviceQuestion(msg.list, msg.err)
}

// devicesWord is "Android device", "iOS device" or just "device".
func devicesWord(platform, word string) string {
	switch platform {
	case "android":
		return "Android " + word
	case "ios":
		return "iOS " + word
	case "":
		return word
	}
	return platform + " " + word
}

// ── Running ──────────────────────────────────────────────────────────────────

func commandLine(target string, env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k, v := range env {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := []string{target}
	for _, k := range keys {
		parts = append(parts, k+"="+env[k])
	}
	return strings.Join(parts, " ")
}

func (m *Model) environ(j *job) []string {
	var env []string
	for k, v := range m.cfg.Env {
		env = append(env, k+"="+v)
	}
	for k, v := range j.env {
		env = append(env, k+"="+v)
	}
	if j.target.Logs {
		env = append(env, "FDEV_DART_DEFINES="+config.DartDefine)
	}
	return env
}

// run opens the log viewer on the job; builds and other commands run there
// too, so their output stays scrollable after they finish.
func (m *Model) run(j *job) tea.Cmd {
	m.job, m.asking = nil, false
	m.state.Remember(state.Run{Target: j.target.Name, Env: j.env, Note: j.note})
	if j.target.Builtin != "" {
		return m.runTool(j.target)
	}
	logs, cmd := logview.New(logview.Options{
		Root: m.cfg.Root, Title: m.cfg.Title, Version: m.cfg.AppVersion(), About: m.about(),
		Command: commandLine(j.target.Name, j.env), Shell: j.target.Run,
		Env: m.environ(j), Note: j.note, Plain: !j.target.Logs,
		Settings: m.state.Logs, Editor: m.cfg.Editor, Theme: m.th,
		Width: m.width, Height: m.height, UI: m.ui, OnUI: m.logsUI, Full: m.full,
		Target: j.target.Name, Look: m.look(), OnLook: m.saveLook, Durations: m.state.Durations,
		Values: m.cfg.Logs.Values,
	})
	m.logs, m.ran, m.screen = logs, *j, screenLogs
	return cmd
}

// runTool runs a builtin tool, fdev itself (fdev wifi), in the terminal:
// it asks things as it goes, which the log viewer isn't for.
func (m *Model) runTool(t *config.Target) tea.Cmd {
	m.screen = screenMenu
	exe, err := os.Executable()
	if err != nil {
		return m.list.NewStatusMessage(m.fg(m.th.Red).Render("✘ " + err.Error()))
	}
	cmd := exec.Command(exe, "wifi", "--wait")
	cmd.Dir = m.cfg.Root
	name := t.Name
	return tea.Exec(interruptible{cmd}, func(err error) tea.Msg { return toolDoneMsg{name, err} })
}

// interruptible runs a command that ctrl+c stops without stopping fdev:
// out of the alt screen, ctrl+c is a signal to both.
type interruptible struct{ *exec.Cmd }

func (c interruptible) Run() error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	return c.Cmd.Run()
}

func (c interruptible) SetStdin(r io.Reader)  { c.Stdin = r }
func (c interruptible) SetStdout(w io.Writer) { c.Stdout = w }
func (c interruptible) SetStderr(w io.Writer) { c.Stderr = w }

func (m *Model) logsDone(msg logview.DoneMsg) tea.Cmd {
	m.state.Save()
	m.full = m.logs.Full() // full screen stays as the viewer left it
	m.logs, m.screen = nil, screenMenu
	m.resizeList()
	m.loadSessions()
	if msg.Replay {
		m.buildMenu()
		return nil
	}
	m.buildMenu()
	m.list.ResetFilter()

	muted := m.fg(m.th.Muted)
	cmd := commandLine(m.ran.target.Name, m.ran.env)
	took := short(msg.Took)
	var result string
	switch {
	case msg.Err != nil:
		result = m.fg(m.th.Red).Render("✘ "+cmd) + muted.Render(" • "+msg.Err.Error())
	case msg.Code == 0:
		result = m.fg(m.th.Green).Render("✓ "+cmd) + muted.Render(" • done in "+took)
	case msg.Code == 130 || msg.Code == -1:
		result = muted.Render("■ " + cmd + " • stopped after " + took)
	default:
		result = m.fg(m.th.Red).Render("✘ "+cmd) + muted.Render(fmt.Sprintf(" • exit %d after %s", msg.Code, took))
	}
	if msg.Errors > 0 {
		result += m.fg(m.th.Red).Render(fmt.Sprintf(" • %d error line(s)", msg.Errors))
	}
	cmds := []tea.Cmd{m.list.NewStatusMessage(result)}
	if msg.Took >= 30*time.Second && !m.ran.target.Logs {
		cmds = append(cmds, m.notify(m.ran, msg.Code == 0 && msg.Err == nil, took))
	}
	return tea.Batch(cmds...)
}

// notify tells the desktop when a long build finishes.
func (m *Model) notify(j job, ok bool, took string) tea.Cmd {
	status := "Done"
	if !ok {
		status = "Failed"
	}
	text := fmt.Sprintf("%s: %s (%s)", status, j.target.Name, took)
	title := m.cfg.Title
	return func() tea.Msg {
		switch runtime.GOOS {
		case "darwin":
			// The texts are arguments of the script, never part of it.
			script := "on run argv\ndisplay notification (item 1 of argv) with title (item 2 of argv)\nend run"
			_ = exec.Command("osascript", "-e", script, text, title).Run()
		case "linux":
			if bin, err := exec.LookPath("notify-send"); err == nil {
				_ = exec.Command(bin, title, text).Run()
			}
		}
		return nil
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m *Model) View() tea.View {
	var content string
	var progressBar *tea.ProgressBar
	mouse := tea.MouseModeAllMotion // hovering selects
	switch {
	case m.screen == screenLogs && m.logs != nil:
		content, mouse = m.logs.View()
		progressBar = m.logs.TerminalProgress()
	case m.screen == screenPick:
		content = m.pickView()
	case m.screen == screenAsk && m.form != nil:
		m.zones = nil
		m.addSettingsZone()
		form := m.pad().Render(m.form.View())
		next := "next ›"
		if m.editingLook {
			next = "save ›"
		}
		content = m.bottom(m.header()+"\n"+m.spacer()+form, next)
	case m.screen == screenDevices:
		m.zones = nil
		m.addSettingsZone()
		looking := m.spin.View() + " " + m.fg(m.th.Indigo).Bold(true).Render("Looking for "+devicesWord(m.platform, "devices")+"…")
		content = m.bottom(m.header()+"\n"+m.spacer()+m.pad().Render(looking), "")
	default:
		content = m.menuView()
	}
	v := tea.NewView(content)
	v.AltScreen, v.MouseMode, v.ProgressBar = true, mouse, progressBar
	v.BackgroundColor, v.ForegroundColor = m.th.TermBg, m.th.TermFg // nil: the terminal's own
	v.KeyboardEnhancements.ReportAlternateKeys = true               // the US key of a combination, where the terminal can
	v.WindowTitle = m.windowTitle()
	return v
}

// pad is the space around a form or message: less when compact.
func (m *Model) pad() lipgloss.Style {
	if m.compact() {
		return lipgloss.NewStyle().Padding(0, 1)
	}
	return lipgloss.NewStyle().Padding(1, 2)
}

// bottom puts the buttons on the last row under content; full screen
// leaves them out.
func (m *Model) bottom(content, next string) string {
	lines := strings.Split(content, "\n")
	if m.full {
		for len(lines) < m.height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:m.height], "\n")
	}
	h := max(m.height-1, len(lines))
	for len(lines) < h {
		lines = append(lines, "")
	}
	lines = lines[:h]
	if next == "" {
		lines = append(lines, m.buttonsBackOnly(h))
	} else {
		lines = append(lines, ansi.Truncate(m.buttons(h, true, next)+m.fg(m.th.Muted).Render("   ← back • → next"), m.width, "…"))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) buttonsBackOnly(y int) string {
	b := lipgloss.NewStyle().Padding(0, 2).Background(m.th.BarBg).Foreground(m.th.Normal)
	if m.hover == "back:" {
		b = b.Foreground(m.th.Pink).Underline(true)
	}
	bt := b.Render("‹ back")
	m.addZone(zone{kind: "back", x0: 2, y0: y, x1: 2 + lipgloss.Width(bt), y1: y + 1})
	return "  " + bt + m.fg(m.th.Muted).Render("   ← or esc")
}

func (m *Model) menuView() string {
	m.zones = nil
	m.addSettingsZone()
	body := m.list.View()
	if m.listW < m.width {
		panel := m.panel(m.width-m.listW-2, m.height-m.headerHeight()-2)
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, "  ", lipgloss.NewStyle().PaddingTop(1).Render(panel))
	}
	next := "run ›"
	switch m.tab {
	case logsTab:
		next = "view ›"
	case recentTab:
		next = "run again ›"
	}
	content := m.header() + "\n" + m.spacer() + body
	lines := strings.Split(content, "\n")
	h := max(m.height-m.bottomRows(), 1)
	for len(lines) < h {
		lines = append(lines, "")
	}
	if m.full {
		return strings.Join(lines[:h], "\n")
	}
	keys := "↑↓ choose • / filter • s settings • t theme • z full screen • q quit"
	if m.tab == logsTab {
		keys = "↑↓ choose • space ★ star • x delete • o open in editor • / filter • z full screen • q quit"
	}
	row := m.buttons(h, len(m.steps()) > 0, next)
	row += m.fg(m.th.Muted).Render("   " + keys)
	lines = append(lines[:h], ansi.Truncate(row, m.width, "…"))
	return strings.Join(lines, "\n")
}

// headerHeight is the rows above the list: the header and a blank row
// (none when compact).
func (m *Model) headerHeight() int { return 1 + len(m.spacer()) }

// spacer is the blank line under the header, "" when compact.
func (m *Model) spacer() string {
	if m.compact() {
		return ""
	}
	return "\n"
}

// header is the top bar: fdev, the branch and where the targets come from.
func (m *Model) header() string {
	th := m.th
	muted := m.fg(th.Muted)
	s := " " + m.fdevBadge() + th.Badge(th.Cream, th.Purple).Bold(true).Render(m.cfg.Title)
	if v := m.cfg.AppVersion(); v != "" {
		s += th.Badge(th.Pink, th.BarBg).Render(v)
	}
	if m.branch != "" {
		s += " " + m.fg(th.Pink).Render("⎇ "+m.branch)
		if m.changed > 0 {
			s += muted.Render(fmt.Sprintf(" • %d changed", m.changed))
		}
	}
	s += muted.Render(" • " + m.cfg.Source)
	full, chip := m.fullChip(), m.settingsChip()
	s = ansi.Truncate(s, max(full.x-2, 10), "…")
	s += strings.Repeat(" ", max(full.x-lipgloss.Width(s), 1)) + full.text + " " + chip.text
	return s
}

// fdevBadge is fdev's name at the header's left; clicking it shows its
// version by it, or hides it.
func (m *Model) fdevBadge() string {
	s := m.th.Badge(m.th.Cream, m.th.Fuchsia).Bold(true)
	if m.hover == "fdev:" {
		s = s.Underline(true)
	}
	return s.Render(version.Name())
}

// settingsChip is the button for fdev's settings, at the header's right.
type chip struct {
	text string
	x, w int
}

func (m *Model) settingsChip() chip {
	s := lipgloss.NewStyle().Padding(0, 1).Background(m.th.BarBg).Foreground(m.th.Muted)
	if m.hover == "settings:" {
		s = s.Foreground(m.th.Pink).Underline(true)
	}
	text := s.Render("⚙ settings")
	w := lipgloss.Width(text)
	return chip{text: text, x: m.width - w - 1, w: w}
}

// fullChip is the button in and out of full screen, left of settings.
func (m *Model) fullChip() chip {
	s := lipgloss.NewStyle().Padding(0, 1).Background(m.th.BarBg).Foreground(m.th.Muted)
	if m.hover == "full:" {
		s = s.Foreground(m.th.Pink).Underline(true)
	}
	label := "⤢ full"
	if m.full {
		label = "⤡ exit · z"
	}
	text := s.Render(label)
	w := lipgloss.Width(text)
	return chip{text: text, x: m.settingsChip().x - w - 1, w: w}
}

func gitInfo(root string) (branch string, changed int) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", 0
	}
	status, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	for _, l := range strings.Split(string(status), "\n") {
		if strings.TrimSpace(l) != "" {
			changed++
		}
	}
	return strings.TrimSpace(string(out)), changed
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
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
