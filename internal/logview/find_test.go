package logview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFind(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(sample))
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"info","m":"needle one"}` + "\n" +
		`I/flutter (1): ⟪fd⟫{"l":"info","m":"hay"}` + "\n" +
		`I/flutter (1): ⟪fd⟫{"l":"info","m":"needle two"}` + "\n"))
	shown := len(m.shownEntries())

	m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if !m.finding || !m.filtering {
		t.Fatal("ctrl+f did not ask for a word")
	}
	for _, r := range "NEEDLE" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.find != "needle" || m.filtering || m.jumped == nil || !strings.Contains(m.jumped.Text, "needle one") {
		t.Fatalf("enter did not show the first: find=%q jumped=%v", m.find, m.jumped)
	}
	if len(m.shownEntries()) != shown {
		t.Fatal("find hid logs")
	}
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if !strings.Contains(m.jumped.Text, "needle two") {
		t.Fatalf("n: %q", m.jumped.Text)
	}
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"}) // wraps
	if !strings.Contains(m.jumped.Text, "needle one") {
		t.Fatalf("wrap: %q", m.jumped.Text)
	}
	m.Update(tea.KeyPressMsg{Code: 'N', Text: "N"})
	if !strings.Contains(m.jumped.Text, "needle two") {
		t.Fatalf("N: %q", m.jumped.Text)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.find != "" {
		t.Fatal("esc kept the find")
	}
}
