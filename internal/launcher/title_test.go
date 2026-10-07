package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/version"
)

func TestColorDot(t *testing.T) {
	for hex, want := range map[string]string{
		"#E8A400": "🟡", // dev
		"#2196F3": "🔵", // test
		"#E53935": "🔴", // prod
		"#7D56F4": "🟣",
		"#00C853": "🟢",
	} {
		if got := colorDot(lipgloss.Color(hex)); got != want {
			t.Errorf("colorDot(%s) = %s, want %s", hex, got, want)
		}
	}
}

func TestVersions(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte("name: demo\nversion: 2.1.39+439\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := load(t, root, `title: Demo
groups:
  - name: Run
    targets:
      - {name: dev, run: make dev}
  - name: Build
    targets:
      - {name: apk, run: make apk}
`)
	version.Current, version.Shown = "v1.4.0", false
	t.Cleanup(func() { version.Current, version.Shown = "dev", false })
	m := New(cfg, newState(nil), "")
	m.Update(tea.WindowSizeMsg{Width: 130, Height: 30})
	m.Init()

	lines := strings.Split(screenText(m), "\n")
	if !strings.Contains(lines[0], " fdev  Demo  v2.1.39 · build 439 ") {
		t.Errorf("header without the app's version, or with fdev's:\n%s", lines[0])
	}
	if got, want := m.windowTitle(), "fdev • Demo v2.1.39 (439)"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if !strings.Contains(strings.Join(lines[:4], "\n"), "Demo v2.1.39 (439)  ·  fdev v1.4.0 · ") {
		t.Errorf("no versions at the picker's top:\n%s", strings.Join(lines[:4], "\n"))
	}

	m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	if lines := strings.Split(screenText(m), "\n"); !strings.Contains(lines[0], " fdev v1.4.0  Demo  v2.1.39") {
		t.Errorf("clicking fdev did not show its version:\n%s", lines[0])
	}
	if got, want := m.windowTitle(), "fdev v1.4.0 • Demo v2.1.39 (439)"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	if version.Shown {
		t.Error("clicking fdev again did not hide its version")
	}

	about := m.about()
	if about.App != "Demo v2.1.39 (439)" || len(about.Facts) == 0 || about.Facts[0] != (config.Fact{Key: "Version", Value: "2.1.39"}) {
		t.Errorf("about = %+v", about)
	}
}
