package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/devices"
	"github.com/NaderMozaffari/fdev/internal/logview"
	"github.com/NaderMozaffari/fdev/internal/state"
)

func load(t *testing.T, root, yaml string) *config.Config {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "fdev.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func newState(menu map[string]string) *state.State {
	return &state.State{Answers: map[string]string{}, Logs: map[string]bool{}, Menu: menu}
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "left":
			msg = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		m.Update(msg)
	}
}

func screenText(m *Model) string { return ansi.Strip(m.View().Content) }

func itemNames(m *Model) string {
	var names []string
	for _, it := range m.list.Items() {
		names = append(names, it.(item).title)
	}
	return strings.Join(names, " ")
}

func TestMenuPanelAndHover(t *testing.T) {
	root := t.TempDir()
	cfg := load(t, root, `title: Demo
flavors:
  dev:
    info: {App: Demo Dev, Backend: test.example.com}
groups:
  - name: Run
    targets:
      - {name: dev, desc: Test app, run: make dev, flavor: dev, logs: true, ask: [store, "device:android"]}
      - {name: codegen, desc: Store app, run: make prod}
      - {name: prod, desc: Hidden, run: make prod, flavor: prod}
asks:
  store: {title: Store edition, env: STORE, options: [{label: Play, value: ""}]}
`)
	os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte("version: 1.2.3+45\n"), 0o644)
	cfg.Version, cfg.Build = "1.2.3", "45"
	m := New(cfg, newState(nil), "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	screen := screenText(m)
	for _, want := range []string{"Demo", "v1.2.3 · build 45", "dev", "Android · Test app", "App", "Demo Dev",
		"Backend", "make dev", "Store edition · Android device", "run ›"} {
		if !strings.Contains(screen, want) {
			t.Errorf("menu has no %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "PLATFORM") || strings.Contains(screen, "Hidden") {
		t.Errorf("the menu shows filters or another flavor:\n%s", screen)
	}

	// Hovering the second item selects it; clicking it then runs it.
	y := m.headerHeight() + 4 + m.menuRowH + m.menuGap
	m.Update(tea.MouseMotionMsg{X: 5, Y: y})
	if m.list.Index() != 1 {
		t.Fatalf("hover selected %d", m.list.Index())
	}
	if screen := screenText(m); !strings.Contains(screen, "Store app") || !strings.Contains(screen, "make prod") {
		t.Errorf("panel did not follow the selection:\n%s", screen)
	}
	if _, ok := m.itemAt(m.listW+2, y); ok {
		t.Error("the panel is not a list item")
	}
}

const pickYAML = `groups:
  - name: Run
    targets:
      - {name: dev, run: make dev, flavor: dev, platform: Android}
      - {name: staging, run: make staging, flavor: staging, platform: Android}
      - {name: web-dev, run: make web-dev, flavor: dev, platform: Web}
  - name: Tools
    targets:
      - {name: clean, run: make clean}
`

func TestPickerSteps(t *testing.T) {
	cfg := load(t, t.TempDir(), pickYAML)
	m := New(cfg, newState(map[string]string{"platform": "Android", "flavor": "staging"}), "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m.Init()

	// The section comes first.
	if m.screen != screenPick || m.step != stepSection {
		t.Fatalf("screen %v step %v", m.screen, m.step)
	}
	if s := screenText(m); !strings.Contains(s, "What do you want to do?") || !strings.Contains(s, "Tools") ||
		!strings.Contains(s, "✕ quit") || strings.Contains(s, "all") {
		t.Errorf("section step:\n%s", s)
	}

	// → goes on, from the last answers; ← comes back.
	press(m, "right")
	if m.tab != "Run" || m.step != stepPlatform {
		t.Fatalf("tab %q step %v", m.tab, m.step)
	}
	press(m, "right")
	if m.step != stepFlavor || m.curChoices()[m.pickIndex].value != "staging" {
		t.Fatalf("step %v at %d", m.step, m.pickIndex)
	}
	if s := screenText(m); !strings.Contains(s, "Which flavor?") || !strings.Contains(s, "✓ Run") ||
		!strings.Contains(s, "✓ Android") || !strings.Contains(s, "‹ back") {
		t.Errorf("flavor step:\n%s", s)
	}
	press(m, "left")
	if m.step != stepPlatform {
		t.Fatalf("← went to %v", m.step)
	}

	// Web has only dev: its flavor step is skipped.
	press(m, "down", "right")
	if m.menuPlatform != "Web" || m.menuFlavor != "dev" || m.screen != screenMenu || itemNames(m) != "web-dev" {
		t.Fatalf("web: %s %s screen %v items %q", m.menuPlatform, m.menuFlavor, m.screen, itemNames(m))
	}
	press(m, "left")
	if m.screen != screenPick || m.step != stepPlatform {
		t.Fatalf("← from the menu: screen %v step %v", m.screen, m.step)
	}

	// Tools has no platforms or flavors to ask.
	press(m, "left", "2")
	if m.tab != "Tools" || m.screen != screenMenu || itemNames(m) != "clean" {
		t.Errorf("Tools: tab %q screen %v items %q", m.tab, m.screen, itemNames(m))
	}

	// The back button, then esc on the first step quits.
	screenText(m)
	var back zone
	for _, z := range m.zones {
		if z.kind == "back" {
			back = z
		}
	}
	m.Update(tea.MouseClickMsg{X: back.x0, Y: back.y0, Button: tea.MouseLeft})
	if m.screen != screenPick || m.step != stepSection {
		t.Fatalf("back button: screen %v step %v", m.screen, m.step)
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd == nil {
		t.Error("esc on the first step should quit")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("esc on the first step should quit")
	}
}

func TestQuestionsInThePicker(t *testing.T) {
	cfg := load(t, t.TempDir(), `groups:
  - name: Run
    targets:
      - {name: dev, run: "true", flavor: dev, ask: [store, mode]}
asks:
  store:
    title: Store edition
    env: STORE
    options:
      - {label: Google Play, value: "", icon: play, color: "#4285F4"}
      - {label: Myket, value: myket, icon: store}
  mode:
    title: Mode
    env: MODE
    options: [{label: Debug, value: debug}, {label: Profile, value: profile}]
`)
	st := newState(nil)
	st.Answers["store"] = "myket"
	m := New(cfg, st, "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	press(m, "enter")
	if m.screen != screenPick || !m.asking || m.curChoices()[m.pickIndex].title != "Myket" {
		t.Fatalf("screen %v asking %v at %d", m.screen, m.asking, m.pickIndex)
	}
	if s := screenText(m); !strings.Contains(s, "Store edition") || !strings.Contains(s, "STORE=myket · last time") ||
		!strings.Contains(s, "○ Mode") || !strings.Contains(s, "‹ back") {
		t.Errorf("store question:\n%s", s)
	}

	// → answers, ← goes back to it, and ← again to the menu.
	press(m, "right")
	if m.qTitle != "Mode" || m.job.env["STORE"] != "myket" {
		t.Fatalf("after store: %q %v", m.qTitle, m.job.env)
	}
	if s := screenText(m); !strings.Contains(s, "✓ Myket") || !strings.Contains(s, "● Mode") {
		t.Errorf("mode question:\n%s", s)
	}
	press(m, "left")
	if m.qTitle != "Store edition" {
		t.Fatalf("← went to %q", m.qTitle)
	}
	press(m, "left")
	if m.screen != screenMenu || m.asking {
		t.Fatalf("← again: screen %v asking %v", m.screen, m.asking)
	}
}

func TestSavedLogs(t *testing.T) {
	root := t.TempDir()
	cfg := load(t, root, pickYAML)
	dir := filepath.Join(root, ".fdev", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := "2026-10-01_10-00-00_dev"
	os.WriteFile(filepath.Join(dir, base+".raw.log"), []byte(
		"10:00:01.500\tI/flutter (1): ⟪fd⟫{\"l\":\"info\",\"t\":\"Bazaar\",\"m\":\"connected\"}\n"), 0o600)
	os.WriteFile(filepath.Join(dir, base+".log"), []byte("# Demo · dev STORE=myket · started x\n# Pixel 7\n\n"), 0o600)

	m := New(cfg, newState(map[string]string{"platform": "Android", "flavor": "dev", "tab": logsTab}), "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	if !contains(m.sections(), logsTab) || m.tab != logsTab {
		t.Fatalf("sections %v tab %q", m.sections(), m.tab)
	}
	if s := screenText(m); !strings.Contains(s, "dev STORE=myket · Pixel 7") || !strings.Contains(s, "view ›") {
		t.Errorf("saved logs:\n%s", s)
	}

	press(m, "space")
	if got := logview.Sessions(root, ".fdev/logs"); len(got) != 1 || !got[0].Starred || !strings.HasPrefix(itemNames(m), "★") {
		t.Fatalf("not starred: %+v %q", got, itemNames(m))
	}

	// It opens in the viewer, at the time it was logged.
	press(m, "enter")
	if m.screen != screenLogs {
		t.Fatalf("screen %v", m.screen)
	}
	if s := ansi.Strip(m.View().Content); !strings.Contains(s, "connected") || !strings.Contains(s, "10:00:01") || !strings.Contains(s, "saved") {
		t.Errorf("replay:\n%s", s)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m.Update(cmd())
	if m.screen != screenMenu {
		t.Fatalf("back from the replay: %v", m.screen)
	}

	press(m, "x")
	if len(logview.Sessions(root, ".fdev/logs")) != 1 {
		t.Fatal("x once should only ask")
	}
	press(m, "x")
	if got := logview.Sessions(root, ".fdev/logs"); len(got) != 0 {
		t.Errorf("not deleted: %+v", got)
	}
	if contains(m.sections(), logsTab) {
		t.Error("Saved logs should be gone")
	}
}

// Every screen fits the terminal: no line wider, no more lines than it has.
func TestScreensFit(t *testing.T) {
	cfg := load(t, t.TempDir(), pickYAML)
	for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 45}} {
		m := New(cfg, newState(nil), "")
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.Init()
		for _, name := range []string{"platform", "flavor", "section", "menu"} {
			lines := strings.Split(m.View().Content, "\n")
			if len(lines) > size[1] {
				t.Errorf("%dx%d %s: %d lines", size[0], size[1], name, len(lines))
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > size[0] {
					t.Errorf("%dx%d %s: line %d is %d wide: %q", size[0], size[1], name, i, w, ansi.Strip(l))
				}
			}
			if os.Getenv("FDEV_SHOW") != "" {
				t.Logf("%dx%d %s:\n%s", size[0], size[1], name, ansi.Strip(m.View().Content))
			}
			press(m, "right")
		}
	}
}

// Shortcuts work with a Persian layout on; a filter still takes Persian.
func TestPersianKeyboard(t *testing.T) {
	cfg := load(t, t.TempDir(), pickYAML)
	m := New(cfg, newState(nil), "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m.Init()
	persian := func(s string) { m.Update(tea.KeyPressMsg{Code: []rune(s)[0], Text: s}) }

	persian("م") // l: next
	if m.step != stepPlatform {
		t.Fatalf("م didn't go on: step %v", m.step)
	}
	persian("ا") // h: back
	if m.step != stepSection {
		t.Fatalf("ا didn't go back: step %v", m.step)
	}
	persian("۲") // 2: Tools
	if m.tab != "Tools" || m.screen != screenMenu {
		t.Fatalf("۲ picked %q, screen %v", m.tab, m.screen)
	}

	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	persian("ک")
	persian("ل")
	if got := m.list.FilterValue(); got != "کل" {
		t.Errorf("the filter got %q", got)
	}
}

// A recent run on a device runs again at once while the device is there,
// and asks for another when it is not.
func TestRerunChecksTheDevice(t *testing.T) {
	cfg := load(t, t.TempDir(), `groups:
  - name: Run
    targets:
      - {name: dev, run: "true", flavor: dev, platform: Android, logs: true, ask: [store, "device:android"]}
asks:
  store: {title: Store edition, env: STORE, options: [{label: Play, value: ""}, {label: Myket, value: myket}]}
`)
	st := newState(nil)
	st.Recent = []state.Run{{Target: "dev", Env: map[string]string{"DEVICE": "phone-1", "STORE": "myket"}, Note: "Redmi"}}
	m := New(cfg, st, "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	recent := m.allRecent()[0]
	other := devices.Device{ID: "emu-2", Name: "Pixel", TargetPlatform: "android", Emulator: true}

	m.rerun(&recent)
	if m.screen != screenDevices || m.job.env["DEVICE"] != "" || m.job.env["STORE"] != "myket" {
		t.Fatalf("screen %v env %v", m.screen, m.job.env)
	}
	m.Update(devicesMsg{list: []devices.Device{other}})
	if s := screenText(m); !strings.Contains(s, "Redmi is not connected") || !strings.Contains(s, "Pixel") || !strings.Contains(s, "Look again") {
		t.Fatalf("question:\n%s", s)
	}
	if st.Recent[0].Env["DEVICE"] != "phone-1" {
		t.Fatal("the recent run changed")
	}

	// Look again, with the phone connected: it runs on it.
	m.answerChoice(choice{device: &deviceChoice{retry: true}})
	m.Update(devicesMsg{list: []devices.Device{other, {ID: "phone-1", Name: "Redmi", TargetPlatform: "android"}}})
	if m.screen != screenLogs || m.ran.env["DEVICE"] != "phone-1" || m.ran.env["STORE"] != "myket" || m.ran.note != "Redmi" {
		t.Fatalf("screen %v ran %v %q", m.screen, m.ran.env, m.ran.note)
	}
	if st.Answers["device:android"] != "phone-1" {
		t.Errorf("answers %v", st.Answers)
	}
}

// Every screen fits at every size and in full screen, and full screen
// has no buttons or keys.
func TestSizesAndFullScreenFit(t *testing.T) {
	cfg := load(t, t.TempDir(), pickYAML)
	for _, size := range []string{config.SizeAuto, config.SizeSmall, config.SizeMedium, config.SizeLarge} {
		for _, full := range []bool{false, true} {
			for _, win := range [][2]int{{60, 16}, {80, 24}, {160, 45}} {
				st := newState(nil)
				st.UI = config.UI{Size: size}
				m := New(cfg, st, "")
				m.Update(tea.WindowSizeMsg{Width: win[0], Height: win[1]})
				m.Init()
				if full {
					press(m, "z")
				}
				for _, name := range []string{"section", "platform", "flavor", "menu"} {
					s := m.View().Content
					lines := strings.Split(s, "\n")
					if len(lines) > win[1] {
						t.Errorf("%s full=%v %dx%d %s: %d lines", size, full, win[0], win[1], name, len(lines))
					}
					for i, l := range lines {
						if w := ansi.StringWidth(l); w > win[0] {
							t.Errorf("%s full=%v %dx%d %s: line %d is %d wide", size, full, win[0], win[1], name, i, w)
						}
					}
					if plain := ansi.Strip(s); full && (strings.Contains(plain, "q quit") || strings.Contains(plain, "next ›") || strings.Contains(plain, "run ›")) {
						t.Errorf("%s %dx%d %s: full screen shows the keys or buttons:\n%s", size, win[0], win[1], name, plain)
					}
					if os.Getenv("FDEV_SHOW") != "" {
						t.Logf("%s full=%v %dx%d %s:\n%s", size, full, win[0], win[1], name, ansi.Strip(s))
					}
					press(m, "right")
				}
			}
		}
	}
}

// The size sets the menu's rows; auto takes the biggest that fits.
func TestMenuSizes(t *testing.T) {
	cfg := load(t, t.TempDir(), `groups:
  - name: Tools
    targets:
      - {name: clean, run: make clean}
      - {name: get, run: make get}
      - {name: test, run: make test}
      - {name: gen, run: make gen}
`)
	for _, c := range []struct {
		size   string
		height int
		rowH   int
	}{
		{config.SizeSmall, 45, 1}, {config.SizeMedium, 45, 2}, {config.SizeLarge, 16, 3},
		{config.SizeAuto, 45, 3}, {config.SizeAuto, 20, 2}, {config.SizeAuto, 12, 1},
	} {
		st := newState(nil)
		st.UI = config.UI{Size: c.size}
		m := New(cfg, st, "")
		m.Update(tea.WindowSizeMsg{Width: 120, Height: c.height})
		m.Init()
		if m.screen != screenMenu || len(m.list.Items()) != 4 {
			t.Fatalf("screen %v with %d items", m.screen, len(m.list.Items()))
		}
		if m.menuRowH != c.rowH {
			t.Errorf("%s at %d rows: rows of %d lines, want %d", c.size, c.height, m.menuRowH, c.rowH)
		}
		// Hovering the second item still selects it.
		m.Update(tea.MouseMotionMsg{X: 5, Y: m.headerHeight() + 4 + m.menuRowH + m.menuGap})
		if m.list.Index() != 1 {
			t.Errorf("%s at %d rows: hover selected %d:\n%s", c.size, c.height, m.list.Index(), screenText(m))
		}
	}
}

// z and the header's button go in and out of full screen; t opens the
// settings on the theme, which is tried while moving and put back on esc.
func TestFullScreenAndTheme(t *testing.T) {
	cfg := load(t, t.TempDir(), pickYAML)
	m := New(cfg, newState(nil), "")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Init()
	press(m, "z")
	if !m.full || strings.Contains(screenText(m), "q quit") {
		t.Fatalf("z did not go full screen:\n%s", screenText(m))
	}
	f := m.fullChip()
	m.Update(tea.MouseClickMsg{X: f.x + 1, Y: 0, Button: tea.MouseLeft})
	if m.full || !strings.Contains(screenText(m), "q quit") {
		t.Fatalf("the header's button did not leave full screen:\n%s", screenText(m))
	}

	press(m, "t")
	if m.screen != screenAsk || !m.editingLook || !strings.Contains(screenText(m), "Dracula") {
		t.Fatalf("t did not open the themes:\n%s", screenText(m))
	}
	press(m, "down")
	if m.th.Name != "charm-dark" || m.View().BackgroundColor == nil {
		t.Errorf("the theme under the cursor is not tried: %q", m.th.Name)
	}
	press(m, "esc")
	if m.th.Name != "auto" || m.screen == screenAsk || m.View().BackgroundColor != nil {
		t.Errorf("esc kept theme %q on screen %v", m.th.Name, m.screen)
	}
}
