package launcher

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
)

// The small looks fit a small window and say what was picked so far: on
// one line, or on the rail.
func TestLauncherLooks(t *testing.T) {
	cfg := load(t, t.TempDir(), pickYAML)
	for _, tc := range []struct {
		launcher string
		w, h     int
		want     []string // after two steps
	}{
		{config.LauncherLine, 60, 14, []string{" › Which flavor?", " ❯ ", "↑↓ choose"}},
		{config.LauncherList, 60, 14, []string{"┌  fdev", "◇  What do you want to do?", "◆  Which flavor?", "│  ● ", "└  ↑↓ choose"}},
		{config.LauncherAuto, 60, 14, []string{" › Which flavor?"}}, // auto in a small window: one line
	} {
		st := newState(nil)
		st.UI.Launcher = tc.launcher
		m := New(cfg, st, "")
		m.Update(tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
		m.Init()
		press(m, "right", "right")
		s := ansi.Strip(m.View().Content)
		for _, w := range tc.want {
			if !strings.Contains(s, w) {
				t.Errorf("%s has no %q:\n%s", tc.launcher, w, s)
			}
		}
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) != tc.h {
			t.Errorf("%s: %d lines on a %d-line screen", tc.launcher, len(lines), tc.h)
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w > tc.w {
				t.Errorf("%s: line %d is %d wide", tc.launcher, i, w)
			}
		}
		if os.Getenv("FDEV_SHOW") != "" {
			t.Logf("%s:\n%s", tc.launcher, s)
		}
		// A click on a choice picks it, as in the full look.
		for _, z := range m.zones {
			if z.kind == "choice" && z.index == 1 {
				m.Update(tea.MouseClickMsg{X: z.x0 + 2, Y: z.y0, Button: tea.MouseLeft})
				break
			}
		}
		if m.pickIndex != 1 {
			t.Errorf("%s: clicking the second choice left %d", tc.launcher, m.pickIndex)
		}
	}
}
