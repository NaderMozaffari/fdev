package logview

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/theme"
	"github.com/NaderMozaffari/fdev/internal/version"
)

func TestCleanRemovesEscapesAndControls(t *testing.T) {
	cases := map[string]string{
		"clipboard \x1b]52;c;ZXZpbA==\x07write":        "clipboard write",
		"\x1b]8;;https://evil\x1b\\link\x1b]8;;\x1b\\": "link",
		"title \x1b]0;pwned\x07!":                      "title !",
		"\x1b[31mred\x1b[0m":                           "red",
		"bell\x07 and \x1b[2Jclear":                    "bell and clear",
		"rtl ‮gnp.exe":                                 "rtl gnp.exe",
		"tab\there":                                    "tab here",
		"پرداخت موفق":                                  "پرداخت موفق",
	}
	for in, want := range cases {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanToolKeepsColorsOnly(t *testing.T) {
	got := cleanTool("\x1b[1mBold\x1b[0m \x1b]0;title\x07 \x1b[2Kok")
	if got != "\x1b[1mBold\x1b[0m  ok" {
		t.Errorf("cleanTool = %q", got)
	}
	if got := cleanTool("Running Gradle task...  ⣽\b⣻\b\b 12.3s"); got != "Running Gradle task...  12.3s" {
		t.Errorf("spinner line = %q", got)
	}
	if got := cleanTool("50%\r100%"); got != "100%" {
		t.Errorf("carriage return = %q", got)
	}
}

func feed(p *Parser, lines ...string) []*Entry {
	var out []*Entry
	for _, l := range lines {
		if r := p.Line(l, time.Unix(0, 0)); r.New != nil {
			out = append(out, r.New)
		}
	}
	return out
}

func TestRecordsAndChunks(t *testing.T) {
	var p Parser
	entries := feed(&p,
		`I/flutter (32096): ⟪fd⟫{"l":"info","t":"Bazaar","m":"connected","at":"lib/a.dart:3:5"}`,
		`I/flutter (32096): ⟪fd 1 1/2⟫{"l":"error","t":"Auth","m":"tok`,
		`I/flutter (32096): ⟪fd 1 2/2⟫en expired","s":"#0 main (package:app/main.dart:3)\n#1 x (dart:async/zone.dart:1)"}`,
		`flutter: ⟪fd⟫{"l":"warning","t":"WARNING","m":"{\"a\": [1, 2]}"}`,
	)
	if len(entries) != 3 {
		t.Fatalf("got %d entries", len(entries))
	}
	if e := entries[0]; e.Category != "info" || e.Tag != "Bazaar" || e.Text != "connected" || e.At != "lib/a.dart:3:5" {
		t.Errorf("info entry = %+v", e)
	}
	if e := entries[1]; e.Text != "token expired" || e.TextTone != ToneRed || len(e.Stack) != 2 || len(e.Raw) != 2 {
		t.Errorf("chunked entry = %+v", e)
	}
	if e := entries[2]; e.Tag != "" || e.Text != "{" || len(e.Lines) != 2 || e.Lines[0].Text != `  "a": [1, 2]` {
		t.Errorf("json entry = %+v", e)
	}
}

func TestHTTPRecords(t *testing.T) {
	var p Parser
	entries := feed(&p,
		`I/flutter ( 1234): ⟪fd⟫{"l":"http","p":"request","method":"GET","url":"https://api.test/v1/me?x=1","headers":{"Authorization":"•••","Accept":"application/json"}}`,
		`I/flutter ( 1234): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://api.test/v1/me?x=1","status":200,"ms":42,"body":{"b":1,"a":[1,2]}}`,
		`I/flutter ( 1234): ⟪fd⟫{"l":"http","p":"error","method":"POST","url":"https://other.host/v1/order","ms":3000,"error":"connectionTimeout"}`,
	)
	if len(entries) != 3 {
		t.Fatalf("got %d entries", len(entries))
	}
	req, res, fail := entries[0], entries[1], entries[2]
	if req.Level != "request" || req.Text != "GET /v1/me" || len(req.Details) != 2 || req.Details[0].Text != "Authorization: •••" {
		t.Errorf("request = %+v", req)
	}
	// The query is a line a parameter, under the request; in the response's details.
	if len(req.Lines) != 1 || req.Lines[0] != (Line{"?x=1", LineQuery}) || res.Details[0] != (Line{"?x=1", LineQuery}) {
		t.Errorf("query: request %+v, response %+v", req.Lines, res.Details)
	}
	if req.Response != res || res.Request != req || req.waiting() {
		t.Errorf("the response did not find its request")
	}
	if res.Level != "response" || fields(res) != "status=200 took=42ms size=17B" || res.Fields[0].Tone != ToneGreen {
		t.Errorf("response = %+v", res)
	}
	// Key order is kept.
	if res.Details[2].Text != `  "b": 1,` {
		t.Errorf("body = %+v", res.Details)
	}
	if fail.Level != "fail" || fields(fail) != "error=connectionTimeout took=3000ms host=other.host" {
		t.Errorf("error = %+v", fail)
	}
}

func fields(e *Entry) string {
	var parts []string
	for _, f := range e.Fields {
		parts = append(parts, f.Key+"="+f.Value)
	}
	return strings.Join(parts, " ")
}

func TestExceptionsNativeAndPlain(t *testing.T) {
	var p Parser
	entries := feed(&p,
		"I/flutter (1): ══╡ EXCEPTION CAUGHT BY WIDGETS LIBRARY ╞═══",
		"I/flutter (1): The following assertion was thrown:",
		"I/flutter (1): #0      Foo.build (package:app/foo.dart:12:3)",
		"I/flutter (1): ════════════════════",
		"E/flutter (1): [ERROR:flutter/runtime/dart_vm_initializer.cc(40)] Unhandled Exception: Bad state",
		"E/flutter (1): #0      List.first (dart:core-patch/growable_array.dart:344:5)",
		"D/EGL_emulation(1): app_time_stats: avg=12ms",
		"I/flutter (1): INFO [Myket] purchase done",
		"I/flutter (1): some print",
		"Flutter run key commands.",
		"h List all available interactive commands.",
	)
	if len(entries) != 7 {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	if e := entries[0]; e.Tag != "Flutter" || len(e.Lines) != 2 || e.Lines[1].Kind != LineFrame {
		t.Errorf("exception = %+v", e)
	}
	if e := entries[1]; e.Text != "Unhandled Exception: Bad state" || len(e.Stack) != 1 {
		t.Errorf("uncaught = %+v", e)
	}
	if e := entries[2]; e.Kind != KindNative || e.Tag != "EGL_emulation" || e.Level != "D" {
		t.Errorf("native = %+v", e)
	}
	if e := entries[3]; e.Category != "info" || e.Tag != "Myket" || e.Text != "purchase done" {
		t.Errorf("plain = %+v", e)
	}
	if e := entries[4]; e.Category != "debug" || e.Level != "print" {
		t.Errorf("print = %+v", e)
	}
	if !p.Ready {
		t.Error("parser not ready after the key commands")
	}
}

func TestCouldBeLog(t *testing.T) {
	for s, want := range map[string]bool{
		"I": true, "I/flu": true, "I/flutter (12": true, "I/flutter (123): ⟪fd": true,
		"Installing build/app.apk...": false, "Running Gradle task": false, "flu": true, "⟪f": true,
	} {
		if got := CouldBeLog(s); got != want {
			t.Errorf("CouldBeLog(%q) = %v", s, got)
		}
	}
}

func testModel(settings map[string]bool) *Model {
	return testModelLook(settings, config.Look{})
}

func testModelLook(settings map[string]bool, look config.Look) *Model {
	m := &Model{on: map[string]bool{}, counts: map[string]int{}, width: 110, height: 20, follow: true, look: look.WithDefaults(), th: theme.New(true),
		o: Options{Settings: settings, Title: "App", Command: "dev"}, started: time.Now(), ui: config.UI{Size: config.SizeMedium},
		keys: newKeyMap(), help: help.New(), dirty: true}
	for _, t := range Toggles {
		m.on[t.Name] = t.Default
		if v, ok := settings[t.Name]; ok {
			m.on[t.Name] = v
		}
	}
	m.vp = viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(m.logHeight()))
	m.spin = spinner.New()
	m.bar = newBar()
	m.input = textinput.New()
	m.SetTheme(theme.New(true))
	m.offKeys()
	m.resize()
	return m
}

func screen(m *Model) string {
	content, _ := m.View()
	return ansi.Strip(content)
}

func TestViewAndToggles(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte("Launching lib/main.dart...\r\n" +
		`I/flutter (1): ⟪fd⟫{"l":"info","t":"Bazaar","m":"connected","at":"lib/a.dart:3"}` + "\r\n" +
		`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://a.test/v1","status":200,"ms":5}` + "\r\n" +
		"D/EGL_emulation(1): noise\r\n" +
		"h List all available interactive commands.\r\n"))
	s := screen(m)
	for _, want := range []string{"INFO ↗ Bazaar: connected", "RESP   GET ← /v1 status=200 took=5ms", "8 native ·1", "fdev  App  dev", "LOGS", "SHOW", "COLUMNS", "⚙ look"} {
		if !strings.Contains(s, want) {
			t.Errorf("default view has no %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "noise") {
		t.Errorf("native line shown:\n%s", s)
	}

	m.toggle("time")
	m.toggle("label")
	m.toggle("native")
	s = screen(m)
	if !strings.Contains(s, "\n↗ Bazaar: connected") || !strings.Contains(s, "EGL_emulation: noise") {
		t.Errorf("without time and label:\n%s", s)
	}

	m.toggle("raw")
	if s = screen(m); !strings.Contains(s, "I/flutter (1): ⟪fd⟫") {
		t.Errorf("raw view:\n%s", s)
	}
}

func TestFilterAndPartialLines(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"info","t":"Bazaar","m":"one"}` + "\n" +
		`I/flutter (1): ⟪fd⟫{"l":"info","t":"Myket","m":"two"}` + "\n" + "Running Gradle task...  ⣽"))
	m.setFilter("myket")
	s := screen(m)
	if strings.Contains(s, "one") || !strings.Contains(s, "two") {
		t.Errorf("filter:\n%s", s)
	}
	if !strings.Contains(s, "Running Gradle task") || !strings.Contains(s, "first time") {
		t.Errorf("the running step has no progress bar:\n%s", s)
	}
	m.feed([]byte("\b 3.1s\n" + "I/flutter (1): ⟪fd"))
	s = screen(m)
	if !strings.Contains(s, "Running Gradle task...   3.1s") || strings.Contains(s, "⟪fd") {
		t.Errorf("finished line / held log line:\n%s", s)
	}
}

const sample = "Launching lib/main.dart...\n" +
	`I/flutter (1): ⟪fd⟫{"l":"info","t":"Bazaar","m":"connected","at":"lib/a.dart:3"}` + "\n" +
	`I/flutter (1): ⟪fd⟫{"l":"warning","t":"Myket","m":"pending"}` + "\n" +
	`I/flutter (1): ⟪fd⟫{"l":"http","p":"request","method":"POST","url":"https://a.test/v1/orders/new"}` + "\n" +
	`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://a.test/v1","status":200,"ms":5,"body":{"a":1}}` + "\n" +
	`I/flutter (1): ⟪fd⟫{"l":"http","p":"error","method":"DELETE","url":"https://a.test/v1/me","status":500,"ms":1234}` + "\n" +
	"h List all available interactive commands.\n"

// column is where s starts on the line that contains it.
func column(t *testing.T, screen, s string) int {
	t.Helper()
	for _, line := range strings.Split(screen, "\n") {
		if i := strings.Index(line, s); i >= 0 {
			return len([]rune(line[:i]))
		}
	}
	t.Fatalf("no %q in:\n%s", s, screen)
	return -1
}

func TestColumnsLineUpNetworkFields(t *testing.T) {
	m := testModelLook(map[string]bool{}, config.Look{Layout: config.LayoutColumns})
	m.feed([]byte(sample))
	s := screen(m)
	// URLs start in one column, and so do the statuses.
	if a, b, c := column(t, s, "/v1/orders/new"), column(t, s, "/v1 "), column(t, s, "/v1/me"); a != b || b != c {
		t.Errorf("URLs at %d %d %d:\n%s", a, b, c, s)
	}
	if a, b := column(t, s, "200"), column(t, s, "500"); a != b {
		t.Errorf("statuses at %d %d:\n%s", a, b, s)
	}
	// Tags have a column too, which the message starts after.
	if a, b := column(t, s, "connected"), column(t, s, "pending"); a != b {
		t.Errorf("messages at %d %d:\n%s", a, b, s)
	}
}

func TestNetworkNameAndDirection(t *testing.T) {
	m := testModelLook(map[string]bool{}, config.Look{Layout: config.LayoutColumns})
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"request","method":"POST","url":"https://a.test/api/v1/payments/verify","name":"verifyPlayPayment"}` + "\n" +
		`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"POST","url":"https://a.test/api/v1/payments/verify","status":200,"ms":5,"name":"verifyPlayPayment"}` + "\n"))
	s := screen(m)
	for _, want := range []string{"→ verifyPlayPayment  /api/v1/payments/verify", "← verifyPlayPayment  /api/v1/payments/verify"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q:\n%s", want, s)
		}
	}
	if !strings.Contains(m.entries[0].Searchable(), "verifyplaypayment") {
		t.Errorf("the filter does not find the name")
	}
	if phaseColor(m.th, "request") == phaseColor(m.th, "response") {
		t.Errorf("requests and responses share a color")
	}
}

func TestTableBadgesAndSpacing(t *testing.T) {
	m := testModelLook(map[string]bool{}, config.Look{Layout: config.LayoutTable, Labels: config.LabelsBadge, Spacing: 1})
	m.feed([]byte(sample))
	s := screen(m)
	for _, want := range []string{"TIME", "LEVEL", "TAG", "MESSAGE", "┼", " │ ", " INFO "} {
		if !strings.Contains(s, want) {
			t.Errorf("table has no %q:\n%s", want, s)
		}
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.Contains(l, "connected") {
			if i+2 >= len(lines) || strings.Contains(lines[i+1], "pending") || !strings.Contains(lines[i+2], "pending") {
				t.Errorf("no spacing row between logs:\n%s", s)
			}
		}
	}
	t.Log("\n" + s)
}

func TestSaveKeepsLogsOutOfGit(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "init", "-q", root).Run(); err != nil {
		t.Skip("no git")
	}
	m := testModelLook(map[string]bool{}, config.Look{Save: true})
	m.o.Root, m.o.Target, m.started = root, "dev STORE=x", time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	if err := m.startRecording(); err != nil {
		t.Fatal(err)
	}
	m.feed([]byte(sample))
	path, err := m.save()
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(".fdev", "logs", "2026-01-02_03-04-05_dev-STORE-x.log") {
		t.Errorf("path = %s", path)
	}
	data, _ := os.ReadFile(filepath.Join(root, path))
	for _, want := range []string{"INFO  Bazaar: connected  (lib/a.dart:3)", "RESP  GET /v1 status=200 took=5ms", `"a": 1`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved log has no %q:\n%s", want, data)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".fdev", "logs", "2026-01-02_03-04-05_dev-STORE-x.raw.log"))
	if !strings.Contains(string(raw), "I/flutter (1): ⟪fd⟫") {
		t.Errorf("raw log:\n%s", raw)
	}
	out, _ := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all").Output()
	if len(strings.TrimSpace(string(out))) != 0 {
		t.Errorf("git sees: %s", out)
	}
}

func TestTaskProgressBars(t *testing.T) {
	gradle := "Running Gradle task 'assembleDevDebug'"
	if got := taskName(gradle + "...  ⣽\b⣻"); got != gradle {
		t.Errorf("taskName = %q", got)
	}
	if got := taskName("Launching lib/main.dart on Pixel 7 in debug mode..."); got == "" {
		t.Error("a line still being written is a task")
	}
	if name, took, ok := finishedTask(gradle + "...                           36.4s"); !ok || name != gradle || took != 36400*time.Millisecond {
		t.Errorf("finishedTask = %q %v %v", name, took, ok)
	}
	if _, took, ok := finishedTask("Syncing files to device Pixel 7...   123ms"); !ok || took != 123*time.Millisecond {
		t.Errorf("ms = %v %v", took, ok)
	}

	durations := map[string]float64{}
	m := testModel(map[string]bool{})
	m.o.Durations = durations
	m.feed([]byte(gradle + "...  ⣽"))
	s := screen(m)
	if m.task == nil || !strings.Contains(s, gradle) || !strings.Contains(s, "first time") ||
		strings.Contains(s, "%") || strings.Contains(s, "~") {
		t.Errorf("first run, no estimate: a loading bar, no percentage:\n%s", s)
	}
	if p := m.TerminalProgress(); p == nil || p.State != tea.ProgressBarIndeterminate {
		t.Errorf("terminal progress without an estimate = %+v", p)
	}
	m.feed([]byte("\b\b 36.4s\n" + "Installing build/app/outputs/flutter-apk/app-dev-debug.apk...  ⣽"))
	if durations[gradle] != 36.4 {
		t.Errorf("durations = %v", durations)
	}
	if m.task == nil || m.task.name != "Installing build/app/outputs/flutter-apk/app-dev-debug.apk" {
		t.Errorf("task = %+v", m.task)
	}
	m.feed([]byte("\b\b 18.1s\n"))
	if m.task != nil || durations["Installing build/app/outputs/flutter-apk/app-dev-debug.apk"] != 18.1 {
		t.Errorf("task %+v durations %v", m.task, durations)
	}
	if s := screen(m); !strings.Contains(s, "36.4s") || !strings.Contains(s, "18.1s") {
		t.Errorf("finished lines missing:\n%s", s)
	}

	// The next run knows how long it takes.
	m2 := testModel(map[string]bool{})
	m2.o.Durations = durations
	m2.feed([]byte(gradle + "...  ⣽"))
	if s := screen(m2); !strings.Contains(s, "/ ~36s") {
		t.Errorf("second run, with an estimate:\n%s", s)
	}
	if p := m2.TerminalProgress(); p == nil || p.State != tea.ProgressBarDefault {
		t.Errorf("terminal progress = %+v", p)
	}
}

func TestLoadingBarSweeps(t *testing.T) {
	band := func(s string) int { return strings.Index(ansi.Strip(s), "█") }
	if w := lipgloss.Width(loadingBar(40, 0.4)); w != 40 {
		t.Errorf("width = %d", w)
	}
	if a, b, c := band(loadingBar(40, 0)), band(loadingBar(40, 0.9)), band(loadingBar(40, 1.8)); !(a == 0 && b > a && c > b) {
		t.Errorf("going right: %d %d %d", a, b, c)
	}
	if a, b := band(loadingBar(40, 1.8)), band(loadingBar(40, 2.7)); b >= a {
		t.Errorf("and back: %d %d", a, b)
	}
}

func TestGroupLines(t *testing.T) {
	rec := func(tag, msg string) string {
		return `I/flutter (1): ⟪fd⟫{"l":"info","t":"` + tag + `","m":"` + msg + `"}`
	}
	start := time.Date(2026, 10, 1, 14, 0, 0, 0, time.Local)
	m := testModel(map[string]bool{})
	feed := func(line string, after time.Duration) { m.addLineAt(line, start.Add(after)) }
	feed(rec("Auth", "one"), 0)
	feed(rec("Auth", "two"), 200*time.Millisecond)
	feed(`I/flutter (1): ⟪fd⟫{"l":"http","p":"request","method":"POST","url":"https://x.co/login"}`, 300*time.Millisecond)
	feed(`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"POST","url":"https://x.co/login","status":200}`, 3*time.Second)
	feed(rec("Home", "three"), 6*time.Second)
	m.dirty = true
	s := screen(m)
	if n := strings.Count(s, "╌╌ +"); n != 1 || !strings.Contains(s, "╌╌ +3.0s") {
		t.Errorf("one line, before three, none before the slow response (%d):\n%s", n, s)
	}
	if strings.Index(s, "╌╌ +") < strings.Index(s, "/login") || strings.Index(s, "╌╌ +") > strings.Index(s, "three") {
		t.Errorf("the line is not between the groups:\n%s", s)
	}

	m.applyLook(config.Look{Group: config.GroupTag})
	s = screen(m)
	for _, want := range []string{"╌╌ network", "╌╌ Home"} {
		if !strings.Contains(s, want) {
			t.Errorf("by tag, %q missing:\n%s", want, s)
		}
	}
	m.applyLook(config.Look{Group: config.GroupOff})
	if s := screen(m); strings.Contains(s, "╌╌") {
		t.Errorf("groups off:\n%s", s)
	}
}

func TestShortcutsTurnedOff(t *testing.T) {
	m := testModelLook(map[string]bool{}, config.Look{Off: []string{"ctrl+l", "ctrl+c"}})
	m.o.Root, m.o.Target = t.TempDir(), "dev"
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"info","m":"kept"}` + "\n"))
	m.key(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	s := screen(m)
	if !strings.Contains(s, "kept") || !strings.Contains(s, "ctrl+l is turned off") {
		t.Errorf("ctrl+l cleared the screen:\n%s", s)
	}
	if strings.Contains(s, "ctrl+c") {
		t.Errorf("the Stop button still names ctrl+c:\n%s", s)
	}
	if m.keys.Wipe.Enabled() {
		t.Error("the help still lists ctrl+l")
	}
	m.applyLook(config.Look{})
	m.key(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	if s := screen(m); strings.Contains(s, "kept") {
		t.Errorf("ctrl+l back on did not clear:\n%s", s)
	}
}

func TestKeysWorkWhileBuilding(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte("Launching lib/main.dart on Pixel 7 in debug mode...\nRunning Gradle task 'assembleDevDebug'...  ⣽"))
	m.key(tea.KeyPressMsg{Code: '8', Text: "8"})
	if !m.on["native"] {
		t.Error("a toggle before flutter is ready did nothing")
	}
	if s := screen(m); !strings.Contains(s, "show/hide logs") {
		t.Errorf("help while building:\n%s", s)
	}

	// flutter asks for a device: the digits are the answer.
	m.feed([]byte("\b 3.0s\nMultiple devices found:\n[1]: Pixel 7 (emulator-5554)\n[2]: macOS (macos)\nPlease choose one (or \"q\" to quit): "))
	if !m.prompting() {
		t.Fatal("the device question is not a prompt")
	}
	m.key(tea.KeyPressMsg{Code: '8', Text: "8"})
	if !m.on["native"] {
		t.Error("an answer to flutter toggled a log category")
	}
	if s := screen(m); !strings.Contains(s, "the command is asking") {
		t.Errorf("no prompt notice:\n%s", s)
	}
	m.feed([]byte("1\n"))
	if m.prompting() {
		t.Error("still prompting after the answer")
	}
}

// at is the screen position of the bar item text.
func at(t *testing.T, m *Model, text string) (int, int) {
	t.Helper()
	lines := strings.Split(screen(m), "\n")
	for y, l := range lines {
		if i := strings.Index(l, text); i >= 0 {
			return len([]rune(l[:i])), y
		}
	}
	t.Fatalf("no %q on screen:\n%s", text, screen(m))
	return 0, 0
}

func TestHeaderVersions(t *testing.T) {
	m := testModel(map[string]bool{})
	m.o.Version = "v2.1.39 · build 439"
	version.Current, version.Shown = "v1.4.0", false
	t.Cleanup(func() { version.Current, version.Shown = "dev", false })
	if s := screen(m); !strings.Contains(s, "fdev  App  v2.1.39 · build 439  dev") {
		t.Errorf("no app version in the header:\n%s", s)
	}
	m.Update(tea.MouseClickMsg{X: 1, Y: 0, Button: tea.MouseLeft})
	if s := screen(m); !strings.Contains(s, "fdev v1.4.0  App  v2.1.39") {
		t.Errorf("clicking fdev did not show its version:\n%s", s)
	}
	m.Update(tea.MouseClickMsg{X: 1, Y: 0, Button: tea.MouseLeft})
	if s := screen(m); strings.Contains(s, "v1.4.0") {
		t.Errorf("clicking fdev again did not hide its version:\n%s", s)
	}
}

func TestBarWithTheMouse(t *testing.T) {
	m := testModel(map[string]bool{})
	m.width = 180 // the whole bar on one row
	m.resize()
	m.feed([]byte(sample + "D/EGL(1): x\n"))

	x, y := at(t, m, "8 native")
	m.Update(tea.MouseMotionMsg{X: x + 1, Y: y})
	if m.hover != "native" {
		t.Errorf("hover = %q", m.hover)
	}
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if !m.on["native"] {
		t.Error("clicking the native chip did not turn it on")
	}

	// The section title turns its whole section off, then on again.
	x, y = at(t, m, "LOGS")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	for _, n := range []string{"info", "success", "warning", "error", "debug", "network"} {
		if m.on[n] {
			t.Errorf("%s still on", n)
		}
	}
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if !m.on["info"] || !m.on["network"] {
		t.Error("the second click did not turn them back on")
	}

	x, y = at(t, m, "? help")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if !m.help.ShowAll {
		t.Error("the help chip did not open the help")
	}
	t.Log("\n" + screen(m))
}

func TestSessionsStarsAndPrune(t *testing.T) {
	root := t.TempDir()
	dir, err := logsDir(root, ".fdev/logs")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < keepSessions+3; i++ {
		base := fmt.Sprintf("2026-01-%02d_10-00-00_dev", i+1)
		os.WriteFile(filepath.Join(dir, base+".log"), []byte("x"), 0o600)
	}
	oldest := "2026-01-01_10-00-00_dev"
	if err := Star(root, ".fdev/logs", oldest, true); err != nil {
		t.Fatal(err)
	}
	prune(dir)
	got := Sessions(root, ".fdev/logs")
	if len(got) != keepSessions+1 || got[0].Base != oldest || !got[0].Starred || got[0].Target != "dev" {
		t.Fatalf("%d sessions, first %+v", len(got), got[0])
	}
	if got[1].Base != fmt.Sprintf("2026-01-%02d_10-00-00_dev", keepSessions+3) {
		t.Errorf("not newest first: %s", got[1].Base)
	}

	day := time.Date(2026, 1, 1, 23, 59, 0, 0, time.Local)
	at, line := splitRaw(rawLine(day.Add(2*time.Minute), "hello"), day, day)
	if line != "hello" || !at.Equal(day.Add(2*time.Minute)) {
		t.Errorf("splitRaw = %v %q", at, line)
	}
	if at, line := splitRaw("no time", day, day); line != "no time" || !at.Equal(day) {
		t.Errorf("splitRaw without a time = %v %q", at, line)
	}
}

func TestCommandBar(t *testing.T) {
	// Before flutter lists its keys, and for builds: just stop.
	m := testModel(nil)
	m.width = 160
	if s := ansi.Strip(m.commandBar()); !strings.Contains(s, "ctrl+c  Stop") || strings.Contains(s, "Reload") {
		t.Errorf("before the keys:\n%s", s)
	}

	m.feed([]byte("Flutter run key commands.\nr Hot reload. 🔥🔥🔥\nR Hot restart.\nh List all available interactive commands.\n"))
	s := ansi.Strip(m.commandBar())
	for _, want := range []string{"FLUTTER", " r  Reload", " R  Restart", " v  DevTools", " q  Quit", " ⋯  more", " ctrl+c  Stop"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "Widget tree") {
		t.Errorf("more is open:\n%s", s)
	}
	x, y := at(t, m, "more")
	m.click(x, y, false)
	if s := ansi.Strip(m.commandBar()); !strings.Contains(s, "Widget tree") || !strings.Contains(s, "less") ||
		!strings.Contains(s, "Stop") || !strings.Contains(s, "Flutter help") {
		t.Errorf("more didn't open:\n%s", s)
	}
	if name := m.commandAt(at(t, m, "Inspector")); name == "" {
		t.Error("Inspector is not a button")
	}

	// After it exits: back.
	m.exited = true
	if s := ansi.Strip(m.commandBar()); !strings.Contains(s, "enter  Back") || strings.Contains(s, "Reload") {
		t.Errorf("after exit:\n%s", s)
	}
}

func TestCommandsOfOtherTools(t *testing.T) {
	vite := testModel(nil)
	vite.width = 160
	for _, l := range []string{
		"  VITE v5.0.0  ready in 300 ms",
		"  ➜  press h + enter to show help",
		"  press r + enter to restart the server",
		"  press o + enter to open in browser",
	} {
		vite.detectCommands(l)
	}
	got := map[string]string{}
	for _, c := range vite.commands {
		got[c.key] = c.label + "|" + c.send
	}
	if got["r ⏎"] != "Restart the server|r\r" || got["h ⏎"] != "Show help|h\r" || got["o ⏎"] != "Open in browser|o\r" {
		t.Errorf("vite: %v", got)
	}

	expo := testModel(nil)
	for _, l := range []string{"› Press a │ open Android", "› Press w │ open web", "› Press r │ reload app"} {
		expo.detectCommands(l)
	}
	if len(expo.commands) != 3 || expo.commands[2].key != "r" || expo.commands[2].label != "Reload app" || expo.commands[2].send != "r" {
		t.Errorf("expo: %+v", expo.commands)
	}

	list := testModel(nil)
	for _, l := range []string{"Keyboard shortcuts:", "  b  Build again", "  t  Run the tests", "Watching for changes"} {
		list.detectCommands(l)
	}
	if len(list.commands) != 2 || list.commands[1].label != "Run the tests" || list.inKeyList {
		t.Errorf("key list: %+v", list.commands)
	}
}

func TestSmoothScroll(t *testing.T) {
	m := testModel(nil)
	for i := range 200 {
		m.feed([]byte(fmt.Sprintf("line %d\n", i)))
	}
	m.refresh()
	bottom := m.vp.YOffset()
	m.wheel(-1)
	first := m.scrollLeft
	m.wheel(-1) // a quick second notch goes further
	if first != -wheelLines || m.scrollLeft >= 2*first {
		t.Fatalf("scroll left %d after %d", m.scrollLeft, first)
	}
	steps := 0
	for m.motion() != nil {
		steps++
	}
	if steps < 3 || m.scrollLeft != 0 || m.vp.YOffset() >= bottom-2*wheelLines || m.follow {
		t.Errorf("%d steps, offset %d (bottom %d), follow %v", steps, m.vp.YOffset(), bottom, m.follow)
	}
	m.wheel(1) // the other way drops what was left
	if m.scrollLeft != wheelLines {
		t.Errorf("turning back: %d", m.scrollLeft)
	}
}

func TestDetailsUnfold(t *testing.T) {
	m := testModel(nil)
	m.width = 110
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://a.test/v1","status":200,"ms":5,"body":{"a":1,"b":2,"c":3,"d":4}}` + "\n"))
	m.refresh()
	closed := len(m.rows)
	e := m.rows[0].entry
	m.toggleExpanded(e)
	m.unfolding.start = time.Now().Add(-unfoldFor / 3)
	m.dirty = true
	m.refresh()
	partway := len(m.rows)
	m.unfolding.start = time.Now().Add(-2 * unfoldFor)
	m.motion()
	m.dirty = true
	m.refresh()
	open := len(m.rows)
	if !(closed < partway && partway < open) {
		t.Errorf("rows closed %d, partway %d, open %d", closed, partway, open)
	}
	if !strings.HasPrefix(ansi.Strip(m.rows[1].text), "▌") {
		t.Errorf("what opened is not marked: %q", ansi.Strip(m.rows[1].text))
	}
	m.litAt = time.Now().Add(-2 * highlightOn)
	m.motion()
	m.refresh()
	if strings.HasPrefix(ansi.Strip(m.rows[1].text), "▌") {
		t.Error("the mark doesn't fade")
	}
}

func TestClearKeepsTheLog(t *testing.T) {
	root := t.TempDir()
	m := testModel(nil) // saving is off
	m.o.Root, m.o.Target, m.started = root, "dev", time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	m.feed([]byte("before one\nbefore two\n"))
	m.clearScreen("cleared")
	m.feed([]byte("after\nnot yet\x1b[2Jafter flutter's clear\n"))
	s := screen(m)
	if strings.Contains(s, "before one") || strings.Contains(s, "not yet") || !strings.Contains(s, "after flutter's clear") || !strings.Contains(s, "screen cleared at") {
		t.Errorf("screen:\n%s", s)
	}
	path, err := m.save()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, path))
	log := string(data)
	order := []string{"before one", "before two", "──── cleared at", "after", "not yet", "──── screen cleared at", "after flutter's clear"}
	last := -1
	for _, w := range order {
		i := strings.Index(log, w)
		if i <= last {
			t.Fatalf("saved log is missing %q in order:\n%s", w, log)
		}
		last = i
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".fdev", "logs", "2026-01-02_03-04-05_dev.raw.log"))
	if !strings.Contains(string(raw), "before one") || !strings.Contains(string(raw), "fdev: cleared at") || !strings.Contains(string(raw), "after flutter's clear") {
		t.Errorf("raw log:\n%s", raw)
	}
}

// Full screen keeps the header and the logs: z or the header's button go
// in and out, and esc comes out.
func TestFullScreen(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(sample + "h List all available interactive commands.\n"))
	logs := m.logHeight()

	m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if !m.Full() {
		t.Fatal("z did not go full screen")
	}
	if s := screen(m); !strings.Contains(s, "z or ⤡") {
		t.Errorf("no word on how to leave full screen:\n%s", s)
	}
	m.statusUntil = time.Now() // the hint is gone: the title is back
	s := screen(m)
	for _, gone := range []string{"LOGS", "1 info", "Reload", "Stop", "↑/↓ scroll"} {
		if strings.Contains(s, gone) {
			t.Errorf("full screen shows %q:\n%s", gone, s)
		}
	}
	if !strings.Contains(s, "fdev  App  dev") || !strings.Contains(s, "Bazaar: connected") {
		t.Errorf("full screen lost the title or the logs:\n%s", s)
	}
	if m.logHeight() != m.height-1 || m.logHeight() <= logs {
		t.Errorf("logs get %d rows of %d (were %d)", m.logHeight(), m.height, logs)
	}
	if lines := strings.Split(s, "\n"); len(lines) != m.height {
		t.Errorf("%d lines on a %d-line screen", len(lines), m.height)
	}

	// The header's button comes out.
	x, y := at(t, m, "⤡ exit")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if m.Full() {
		t.Fatal("the header's button did not leave full screen")
	}
	// The bar's button goes in; esc comes out.
	x, y = at(t, m, "⤢ full")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if !m.Full() {
		t.Fatal("the bar's button did not go full screen")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.Full() || !strings.Contains(screen(m), "LOGS") {
		t.Errorf("esc did not leave full screen:\n%s", screen(m))
	}
}

// Small, the bars have no rule or section titles, and the command's
// buttons take one row.
func TestCompactBars(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(sample + "Flutter run key commands.\n"))
	big := m.toggleRows() + m.commandRows()
	m.ui.Size = config.SizeSmall
	m.resize()
	s := screen(m)
	if strings.Contains(s, "LOGS") || strings.Contains(s, "────") {
		t.Errorf("compact bar has titles or a rule:\n%s", s)
	}
	if m.commandRows() != 1 || m.toggleRows()+m.commandRows() >= big {
		t.Errorf("compact bars take %d+%d rows (were %d)", m.toggleRows(), m.commandRows(), big)
	}
	x, y := at(t, m, "8 native")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if !m.on["native"] {
		t.Error("clicking a compact chip did not toggle it")
	}

	m.ui.Size = config.SizeAuto // auto is compact in a short window
	m.height = 40
	if m.compact() {
		t.Error("auto is compact in a tall window")
	}
	m.height = 24
	if !m.compact() {
		t.Error("auto is not compact in a short window")
	}
}

// The settings try a theme as the cursor moves over it, and cancelling
// goes back to the one before.
func TestSettingsTryTheTheme(t *testing.T) {
	m := testModel(map[string]bool{})
	m.ui = config.UI{Theme: theme.Auto, Size: config.SizeMedium}
	var tried []config.UI
	m.o.OnUI = func(ui config.UI, save bool) {
		tried = append(tried, ui)
		if save {
			t.Error("saved before the form was")
		}
		m.SetTheme(theme.Named(ui.Theme, true))
	}
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.lookForm == nil {
		t.Fatal("l did not open the settings")
	}
	if s := screen(m); !strings.Contains(s, "Theme") || !strings.Contains(s, "Dracula") {
		t.Fatalf("no themes in the settings:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if len(tried) == 0 || m.th.Name != "charm-dark" || m.th.TermBg == nil {
		t.Fatalf("moving to a theme did not try it: %v, shown %q", tried, m.th.Name)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.lookForm != nil || m.th.Name != theme.Auto || m.ui.Theme != theme.Auto {
		t.Errorf("cancel left theme %q (ui %+v)", m.th.Name, m.ui)
	}
}

func TestLinesGluedToFlutterSpinner(t *testing.T) {
	var p Parser
	entries := feed(&p,
		"Syncing files to device Pixel...                       ⣽I/flutter (13500): ⟪fd 5 1/2⟫{\"l\":\"info\",\"t\":\"Sync\",",
		"Syncing files to device Pixel...                       ⣾I/flutter (13500): ⟪fd 5 2/2⟫\"m\":\"done\"}\b\b⣷",
		"Running Gradle task... ⣽D/EGL_emulation(1): noise",
		"Error: see E/flutter (1): logs", // no spinner: flutter's own line
	)
	if len(entries) != 3 {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	if e := entries[0]; e.Kind != KindApp || e.Tag != "Sync" || e.Text != "done" {
		t.Errorf("record after the spinner = %+v", e)
	}
	if e := entries[1]; e.Kind != KindNative || e.Text != "noise" {
		t.Errorf("native line after the spinner = %+v", e)
	}
	if e := entries[2]; e.Kind != KindTool {
		t.Errorf("tool line taken for a log: %+v", e)
	}
}

func TestChunksAfterHotRestart(t *testing.T) {
	var p Parser
	entries := feed(&p,
		`I/flutter (1): ⟪fd 3 1/2⟫{"l":"info","m":"lost`, // its second part never came
		`I/flutter (1): ⟪fd 3 1/2⟫{"l":"info",`,
		`I/flutter (1): ⟪fd 3 2/2⟫"m":"after restart"}`,
	)
	if len(entries) != 1 || entries[0].Text != "after restart" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestBodyCutShortIsIndented(t *testing.T) {
	var p Parser
	body := `{\"data\":[{\"key\":\"LINKS\",\"value\":{\"a\":\"x, y\",\"b\":[1,2]}},{\"key\":\"FA` + strings.Repeat("Q", 200) + `… (44352 chars)`
	entries := feed(&p, `I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://a.test/v1/settings","status":200,"headers":{"report-to":{"group":"cf"}},"body":"`+body+`"}`)
	e := entries[0]
	if e.Level != "response" || len(e.Details) < 8 {
		t.Fatalf("entry = %+v", e)
	}
	if e.Details[0].Text != `report-to: { "group": "cf" }` {
		t.Errorf("object header = %q", e.Details[0].Text)
	}
	want := []string{"{", `  "data": [`, "    {", `      "key": "LINKS",`, `      "value": {`, `        "a": "x, y",`}
	for i, w := range want {
		if got := e.Details[1+i]; got.Text != w || got.Kind != LineJSON {
			t.Errorf("body line %d = %q (%v), want %q", i, got.Text, got.Kind, w)
		}
	}
}

func TestLongEntryFoldsAndOpensInAWindow(t *testing.T) {
	m := testModel(map[string]bool{"details": true})
	m.width = 100
	items := make([]string, 300)
	for i := range items {
		items[i] = fmt.Sprintf(`{"id":%d}`, i)
	}
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://a.test/v1/settings","status":200,"body":[` + strings.Join(items, ",") + `]}` + "\n" +
		`I/flutter (1): ⟪fd⟫{"l":"info","m":"` + strings.Repeat("long text ", 200) + `"}` + "\n"))
	m.refresh()
	var hints []row
	for _, r := range m.rows {
		if r.more {
			hints = append(hints, r)
		}
	}
	if len(hints) != 2 || len(m.rows) > 2*(1+maxBody+1)+maxHead+2 {
		t.Fatalf("%d rows, hints %+v", len(m.rows), hints)
	}
	if s := ansi.Strip(hints[0].text); !strings.Contains(s, "more lines") || !strings.Contains(s, "⤢ open") {
		t.Errorf("hint = %q", s)
	}

	m.openDialog(hints[0].entry)
	s := screen(m)
	if !strings.Contains(s, "GET /v1/settings") || !strings.Contains(s, "Body") || !strings.Contains(s, "esc close") {
		t.Errorf("window:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if s := screen(m); !strings.Contains(s, `"id": 299`) {
		t.Errorf("end of the window:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.dialog != nil {
		t.Error("esc doesn't close the window")
	}

	m.Update(tea.KeyPressMsg{Text: "e", Code: 'e'})
	if m.dialog == nil || m.dialog.entry.Category != "info" {
		t.Errorf("e opens %+v", m.dialog)
	}
}

func keyText(s string) tea.KeyPressMsg {
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}
