package logview

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
)

// Each log style draws the same logs its own way; FDEV_SHOW=1 prints them.
func TestLogStyles(t *testing.T) {
	calls := `I/flutter (1): ⟪fd⟫{"l":"http","p":"request","id":"9","method":"POST","url":"https://a.test/v1/cart"}` + "\n" +
		`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","id":"9","method":"POST","url":"https://a.test/v1/cart","status":402}` + "\n"
	for _, tc := range []struct {
		style string
		want  []string
	}{
		{config.StyleStatus, []string{"INFO", "? more"}},
		{config.StyleDots, []string{"● ↗ Bazaar: connected", "● info", "● network"}},
		{config.StyleRail, []string{"◇ ", "▲ ", "↔ ", "└  fdev"}},
		{config.StyleBlocks, []string{"╭─", "│ ", "╰─", "POST ← /v1/cart"}},
		{config.StyleTabs, []string{" All ", "Network 5", "Warnings 1"}},
	} {
		m := testModelLook(map[string]bool{}, config.Look{Style: tc.style})
		m.feed([]byte(sample + calls))
		s := screen(m)
		for _, w := range tc.want {
			if !strings.Contains(s, w) {
				t.Errorf("%s has no %q:\n%s", tc.style, w, s)
			}
		}
		if os.Getenv("FDEV_SHOW") != "" {
			t.Logf("%s:\n%s", tc.style, s)
		}
		switch tc.style {
		case config.StyleDots: // the time once a minute
			if strings.Count(s, m.entries[1].Time.Format("15:04")) != 1 {
				t.Errorf("dots repeat the minute:\n%s", s)
			}
		case config.StyleBlocks: // a call with its response is one box
			if strings.Contains(s, "POST → /v1/cart") {
				t.Errorf("blocks show the request of an answered call:\n%s", s)
			}
		case config.StyleTabs:
			m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			if m.filter != "level:network" || strings.Contains(screen(m), "Bazaar") {
				t.Errorf("tab did not pick Network: %q\n%s", m.filter, screen(m))
			}
			x, y := at(t, m, " All ")
			m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
			if m.filter != "" {
				t.Errorf("clicking All left %q", m.filter)
			}
		}
	}
}
