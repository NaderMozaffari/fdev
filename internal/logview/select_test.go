package logview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// rowOf is the screen row of e's first row.
func rowOf(t *testing.T, m *Model, e *Entry) int {
	t.Helper()
	m.fitHeight()
	m.refresh()
	for i, r := range m.rows {
		if r.entry == e {
			return m.logTop() + i - m.vp.YOffset()
		}
	}
	t.Fatalf("no row for %+v", e)
	return 0
}

// menuItem is where the menu's item for act is on the screen.
func menuItem(t *testing.T, m *Model, act string) (int, int) {
	t.Helper()
	x, y, _, _ := m.menuRect(m.menuBox())
	for i, a := range m.selActions() {
		if a.act == act {
			return x + 2, y + 1 + i
		}
	}
	t.Fatalf("no %q in the menu", act)
	return 0, 0
}

func ctrlClick(x, y int, mod tea.KeyMod) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: tea.ModCtrl | mod}
}

func TestSelectPinTrackBookmarkHide(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(sample))
	fail, tool := m.entries[5], m.entries[4]

	// A plain click doesn't select; tab and the actions' old keys do nothing.
	m.Update(tea.MouseClickMsg{X: 20, Y: rowOf(t, m, fail), Button: tea.MouseLeft})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	if m.sel != nil {
		t.Fatalf("selected without ctrl: %+v", m.sel)
	}

	// ctrl+click selects and opens the menu by the mouse; the bar at the
	// bottom stays as it was.
	m.Update(ctrlClick(20, rowOf(t, m, fail), 0))
	if m.sel != fail || m.follow {
		t.Fatalf("ctrl+click selected %+v (following %v)", m.sel, m.follow)
	}
	s := screen(m)
	if !strings.Contains(s, "1 log") || !strings.Contains(s, "pin to the top") || !strings.Contains(s, "track logs like it") {
		t.Errorf("no menu:\n%s", s)
	}
	if foot := strings.Split(s, "\n"); strings.Contains(foot[len(foot)-1], "copy") {
		t.Errorf("the actions are in the bar:\n%s", s)
	}
	m.Update(keyText("p"))
	if fail.pinned || m.sel != nil {
		t.Fatal("p pinned, or kept the menu open")
	}

	// shift with the click stretches it.
	m.Update(ctrlClick(20, rowOf(t, m, fail), 0))
	m.Update(ctrlClick(20, rowOf(t, m, tool), tea.ModShift))
	if n := len(m.selected()); n != 2 {
		t.Errorf("shift+ctrl+click selected %d logs", n)
	}
	x, y := menuItem(t, m, "bookmark")
	m.Update(tea.MouseMotionMsg{X: x, Y: y})
	if m.hover != "menu:bookmark" {
		t.Errorf("hover = %q", m.hover)
	}
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if !tool.bookmarked || !fail.bookmarked || m.sel != nil || !m.follow {
		t.Error("bookmark did not bookmark the range and close the menu")
	}

	for _, act := range []string{"pin", "track"} {
		m.Update(tea.MouseClickMsg{X: 20, Y: rowOf(t, m, fail), Button: tea.MouseRight})
		m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft}) // elsewhere: just closes
		if m.sel != nil {
			t.Fatal("a click elsewhere left the menu open")
		}
		m.Update(tea.MouseClickMsg{X: 20, Y: rowOf(t, m, fail), Button: tea.MouseRight})
		x, y := menuItem(t, m, act)
		m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	}
	s = screen(m)
	if !strings.Contains(s, "⚑ ") || !strings.Contains(s, "◉ ") || !strings.Contains(s, "×1") || !strings.Contains(s, "★") {
		t.Errorf("no pin, track or bookmark:\n%s", s)
	}
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"error","method":"DELETE","url":"https://a.test/v1/me","status":502,"ms":1}` + "\n"))
	if s := screen(m); !strings.Contains(s, "×2") || !strings.Contains(s, "502") {
		t.Errorf("the track didn't count the new call:\n%s", s)
	}

	// Hiding leaves an empty row; a click on it brings the log back.
	m.Update(ctrlClick(20, rowOf(t, m, fail), 0))
	x, y = menuItem(t, m, "hide1")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if !fail.hidden || m.sel != nil {
		t.Fatalf("hide did not hide: %+v", fail)
	}
	s = screen(m)
	if !strings.Contains(s, "hidden") || !strings.Contains(s, "◌ show") {
		t.Errorf("no row for the hidden log:\n%s", s)
	}
	m.click(20, rowOf(t, m, fail))
	if fail.hidden {
		t.Error("a click did not show the hidden log again")
	}
	// [ jumps to a bookmark, without selecting it.
	m.Update(keyText("["))
	if m.sel != nil || m.lit == nil || !m.lit.bookmarked {
		t.Errorf("[ lit %+v, selected %+v", m.lit, m.sel)
	}
}

func TestCurlAndBody(t *testing.T) {
	var p Parser
	entries := feed(&p,
		`I/flutter (1): ⟪fd⟫{"l":"http","p":"request","method":"POST","url":"https://a.test/v1/orders?x=1","headers":{"Content-Type":"application/json","Authorization":"•••"},"body":{"id":1}}`,
		`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"POST","url":"https://a.test/v1/orders?x=1","status":200,"body":{"ok":true}}`,
		`I/flutter (1): ⟪fd⟫{"l":"info","m":"state: {a: 1}"}`,
	)
	want := "curl -X POST 'https://a.test/v1/orders?x=1' \\\n  -H 'Content-Type: application/json' \\\n  --data-raw '{\"id\":1}'"
	if got := curl(entries[1]); got != want {
		t.Errorf("curl:\n%s", got)
	}
	if got := linesText(bodyOf(entries[1])); got != "{\n  \"ok\": true\n}" {
		t.Errorf("response body: %q", got)
	}
	if got := linesText(bodyOf(entries[2])); got != "{\n  \"a\": 1\n}" {
		t.Errorf("printed map: %q", got)
	}
}
