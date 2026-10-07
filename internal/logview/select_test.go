package logview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSelectPinTrackBookmarkHide(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(sample))
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.sel == nil || m.sel.Kind != KindTool {
		t.Fatalf("tab selected %+v", m.sel)
	}
	if s := screen(m); !strings.Contains(s, "1 log selected") || !strings.Contains(s, "c copy") {
		t.Errorf("no actions for the selection:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.sel.Level != "fail" {
		t.Fatalf("↑ selected %+v", m.sel)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	if n := len(m.selected()); n != 2 {
		t.Errorf("shift+↑ selected %d logs", n)
	}
	m.Update(keyText("b"))
	if !m.entries[4].bookmarked || !m.entries[5].bookmarked {
		t.Error("b did not bookmark the range")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // one again: the failed call
	m.Update(keyText("p"))
	m.Update(keyText("t"))
	s := screen(m)
	if !strings.Contains(s, "⚑ ") || !strings.Contains(s, "◉ ") || !strings.Contains(s, "×1") || !strings.Contains(s, "★") {
		t.Errorf("no pin, track or bookmark:\n%s", s)
	}
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"error","method":"DELETE","url":"https://a.test/v1/me","status":502,"ms":1}` + "\n"))
	if s := screen(m); !strings.Contains(s, "×2") || !strings.Contains(s, "502") {
		t.Errorf("the track didn't count the new call:\n%s", s)
	}
	// Hiding leaves an empty row; a click on it brings the log back.
	m.Update(keyText("h"))
	hid := m.entries[5]
	if !hid.hidden || m.sel != nil {
		t.Fatalf("h did not hide: %+v", hid)
	}
	s = screen(m)
	if !strings.Contains(s, "hidden") || !strings.Contains(s, "◌ show") {
		t.Errorf("no row for the hidden log:\n%s", s)
	}
	row := -1
	for i, r := range m.rows {
		if r.entry == hid {
			row = i
		}
	}
	m.click(20, m.logTop()+row-m.vp.YOffset(), false)
	if hid.hidden {
		t.Error("a click did not show the hidden log again")
	}
	// [ jumps to a bookmark.
	m.unselect()
	m.Update(keyText("["))
	if m.sel == nil || !m.sel.bookmarked {
		t.Errorf("[ selected %+v", m.sel)
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
