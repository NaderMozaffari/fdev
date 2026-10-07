package launcher

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/keys"
	"github.com/NaderMozaffari/fdev/internal/logview"
	"github.com/NaderMozaffari/fdev/internal/state"
	"github.com/NaderMozaffari/fdev/internal/theme"
	"github.com/NaderMozaffari/fdev/internal/version"
)

// LogsOnly is `fdev logs -- <command>`: the log viewer without the menu.
type LogsOnly struct {
	logs     *logview.Model
	state    *state.State
	cmd      tea.Cmd
	ui       config.UI // the theme shown: saved, or tried in the settings
	termDark bool
	title    string // the terminal's, after fdev's name
	Code     int
}

// NewLogsOnly runs args in the log viewer, for the project cfg (one
// without a config when the folder has none).
func NewLogsOnly(cfg *config.Config, args []string, st *state.State) *LogsOnly {
	shell := make([]string, len(args))
	for i, a := range args {
		shell[i] = shellQuote(a)
	}
	m := &LogsOnly{state: st, ui: st.UI.WithDefaults(), termDark: true}
	app := appTitle(cfg.Title, cfg.ShortVersion())
	m.title = strings.Join([]string{app, strings.Join(args, " ")}, titleSep)
	about := logview.About{App: app}
	for _, f := range []config.Fact{{Key: "Version", Value: cfg.Version}, {Key: "Build", Value: cfg.Build},
		{Key: "Project", Value: version.Home(cfg.Root)}, {Key: "Runs", Value: strings.Join(args, " ")}} {
		if f.Value != "" {
			about.Facts = append(about.Facts, f)
		}
	}
	m.logs, m.cmd = logview.New(logview.Options{
		Root: cfg.Root, Title: cfg.Title, Version: cfg.AppVersion(), About: about,
		Command: strings.Join(args, " "), Shell: strings.Join(shell, " "),
		Env: []string{"FDEV_DART_DEFINES=" + config.DartDefine}, Settings: st.Logs,
		Editor: cfg.Editor, Theme: theme.Named(m.ui.Theme, true), Width: 80, Height: 24,
		Target: "logs", Look: look(st, cfg.Logs), OnLook: func(l config.Look) { st.Look = &l; st.Save() },
		Durations: st.Durations, UI: m.ui, OnUI: m.setUI, Values: cfg.Logs.Values,
	})
	return m
}

// setUI shows a theme and size the settings try, or saves them.
func (m *LogsOnly) setUI(ui config.UI, save bool) {
	m.ui = ui.WithDefaults()
	m.logs.SetTheme(theme.Named(m.ui.Theme, m.termDark))
	if save {
		m.state.UI = m.ui
		m.state.SaveUI()
	}
}

func (m *LogsOnly) Init() tea.Cmd { return tea.Batch(m.cmd, tea.RequestBackgroundColor) }

func (m *LogsOnly) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && !m.logs.Typing() {
		msg = keys.Latin(k) // shortcuts work in any keyboard layout
	}
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if !theme.Painted(msg.Color) { // the terminal's own, not a theme's
			m.termDark = msg.IsDark()
			m.logs.SetTheme(theme.Named(m.ui.Theme, m.termDark))
		}
		return m, nil
	case logview.DoneMsg:
		m.state.Save()
		m.Code = msg.Code
		return m, tea.Quit
	}
	return m, m.logs.Update(msg)
}

func (m *LogsOnly) View() tea.View {
	content, mouse := m.logs.View()
	v := tea.NewView(content)
	v.AltScreen, v.MouseMode, v.ProgressBar = true, mouse, m.logs.TerminalProgress()
	th := theme.Named(m.ui.Theme, m.termDark)
	v.BackgroundColor, v.ForegroundColor = th.TermBg, th.TermFg // nil: the terminal's own
	v.KeyboardEnhancements.ReportAlternateKeys = true
	v.WindowTitle = version.Name() + titleSep + m.title
	if exited, ok := m.logs.Status(); exited {
		v.WindowTitle += map[bool]string{true: titleSep + "✓", false: titleSep + "✘"}[ok]
	}
	return v
}

// shellQuote quotes s for sh, so `fdev logs -- ...` runs the arguments as given.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./=:,+@%") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func look(st *state.State, defaults config.Look) config.Look {
	if st.Look != nil {
		return st.Look.WithDefaults()
	}
	return defaults.WithDefaults()
}
