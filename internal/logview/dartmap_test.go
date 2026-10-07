package logview

import (
	"strings"
	"testing"
)

func TestDartMapsAreShownAsJSON(t *testing.T) {
	var p Parser
	entries := feed(&p,
		`I/flutter (1): {id: 7, name: Ali Rezaei, tags: [a, b], address: {city: Tehran, zip: null}, ok: true}`,
		`I/flutter (1): user: {id: 7, note: hello, world}`,
		`I/flutter (1): roles {admin: {since: 2024}, editor}`,
		`I/flutter (1): Navigator push [/home]`,
	)
	got := func(e *Entry) string {
		lines := []string{e.Text}
		for _, l := range e.Lines {
			lines = append(lines, l.Text)
		}
		return strings.Join(lines, "\n")
	}
	want := `{
  "id": 7,
  "name": "Ali Rezaei",
  "tags": ["a", "b"],
  "address": {
    "city": "Tehran",
    "zip": null
  },
  "ok": true
}`
	if s := got(entries[0]); s != want {
		t.Errorf("map:\n%s", s)
	}
	// A value with ", " in it stays whole; the text before stays first.
	if s := got(entries[1]); s != "user: {\n  \"id\": 7,\n  \"note\": \"hello, world\"\n}" {
		t.Errorf("map after text:\n%s", s)
	}
	if s := got(entries[2]); s != "roles {admin: {since: 2024}, editor}" {
		t.Errorf("not a map, not a set: as it is:\n%s", s)
	}
	if e := entries[3]; len(e.Lines) != 0 || e.Text != "Navigator push [/home]" {
		t.Errorf("a short list stays as it is: %+v", e)
	}
	// In a record's message too.
	e := feed(&p, `I/flutter (1): ⟪fd⟫{"l":"info","t":"Auth","m":"session {user: {id: 1}}"}`)[0]
	if e.Text != "session {" || len(e.Lines) != 4 || e.Lines[0].Kind != LineJSON {
		t.Errorf("record = %q %+v", e.Text, e.Lines)
	}
}
