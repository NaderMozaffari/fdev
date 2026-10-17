package logview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFoldJSON(t *testing.T) {
	m := testModel(map[string]bool{"details": true})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	url := "https://a.test/v1/me"
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"` + url +
		`","status":200,"body":{"user":{"id":7,"name":"Ann","tags":["a","b"]},"items":[{"sku":"x1"},{"sku":"x2"}]}}` + "\n"))
	if s := allRows(m); !strings.Contains(s, `"sku": "x2"`) || !strings.Contains(s, "▾") {
		t.Fatalf("the body is open, with a ▾ on its parts:\n%s", s)
	}
	row := func(want string) int {
		m.refresh()
		for i, r := range m.rows {
			if strings.Contains(stripANSI(r.text), want) {
				return m.logTop() + i - m.vp.YOffset()
			}
		}
		t.Fatalf("no row with %q:\n%s", want, screen(m))
		return 0
	}

	// Each object says who it is, open or closed; the ones without those
	// fields say nothing.
	if s := allRows(m); !strings.Contains(s, `"user": { id=7 name=Ann`) || strings.Contains(s, "sku=") {
		t.Fatalf("no id and name by the objects:\n%s", s)
	}

	// A click closes one part: its lines go, a count stays, the rest is there.
	m.Update(tea.MouseClickMsg{X: 20, Y: row(`"user": {`), Button: tea.MouseLeft})
	s := allRows(m)
	if strings.Contains(s, `"name": "Ann"`) || !strings.Contains(s, `▸`) || !strings.Contains(s, "3 keys · id=7 name=Ann") || !strings.Contains(s, `"sku": "x2"`) {
		t.Fatalf("user not folded alone:\n%s", s)
	}
	// Again, and it opens.
	m.Update(tea.MouseClickMsg{X: 20, Y: row(`"user": {…}`), Button: tea.MouseLeft})
	if s := allRows(m); !strings.Contains(s, `"name": "Ann"`) {
		t.Fatalf("user not open again:\n%s", s)
	}
	// A middle click on the whole body closes what is inside it too.
	m.Update(tea.MouseClickMsg{X: 20, Y: row(`"items": [`), Button: tea.MouseMiddle})
	if s := allRows(m); strings.Contains(s, `"sku": "x1"`) || !strings.Contains(s, "2 items") {
		t.Fatalf("items not folded:\n%s", s)
	}
}

// allRows is every row of the logs, also those above the screen.
func allRows(m *Model) string {
	m.refresh()
	var b strings.Builder
	for _, r := range m.rows {
		b.WriteString(stripANSI(r.text) + "\n")
	}
	return b.String()
}

func TestKeysThatSayWhatItIsComeFirst(t *testing.T) {
	n, _ := parseJSON([]byte(`{"updated_at":"u","size":1,"status":"ACTIVE","deleted_at":null,"name":"x","created_at":"c","meta":{"b":1,"id":2},"id":38,"type":"T"}`))
	got := strings.Join(prettyLines(n), "\n")
	want := []string{`"id"`, `"name"`, `"type"`, `"status"`, `"size"`, `"meta"`, `"created_at"`, `"updated_at"`, `"deleted_at"`}
	last := -1
	for _, w := range want {
		i := strings.Index(got, "  "+w+":")
		if i < last {
			t.Fatalf("%s is out of order:\n%s", w, got)
		}
		last = i
	}
	if i, j := strings.Index(got, `"id": 2`), strings.Index(got, `"b": 1`); i > j {
		t.Fatalf("the object inside is not ordered too:\n%s", got)
	}
}
