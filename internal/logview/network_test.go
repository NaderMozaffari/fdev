package logview

import (
	"strings"
	"testing"
	"time"
)

func TestQueryParametersApart(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"request","method":"GET","url":"https://a.test/api/v1/app-texts?type=UPDATE_INFORMATION&q=hello%20world"}` + "\n"))
	s := screen(m)
	if strings.Contains(s, "?type=UPDATE") || !strings.Contains(s, "/api/v1/app-texts") {
		t.Errorf("the query is still in the path:\n%s", s)
	}
	for _, want := range []string{"? type = UPDATE_INFORMATION", "& q = hello world"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q:\n%s", want, s)
		}
	}
}

func TestRequestsWaitForTheirResponse(t *testing.T) {
	m := testModel(map[string]bool{})
	req := `I/flutter (1): ⟪fd⟫{"l":"http","p":"request","method":"GET","url":"https://a.test/v1/me","id":"1"}` + "\n"
	m.feed([]byte(req + strings.Replace(req, `"1"`, `"2"`, 1)))
	if m.parser.InFlight() != 2 {
		t.Fatalf("in flight = %d", m.parser.InFlight())
	}
	s := screen(m)
	if !strings.Contains(s, "2 waiting") || !strings.ContainsAny(s, strings.Join(spinFrames, "")) {
		t.Errorf("no spinner for the waiting calls:\n%s", s)
	}
	// The second call answers first: the id pairs it.
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://a.test/v1/me","id":"2","status":201,"ms":9}` + "\n"))
	first, second := m.entries[0], m.entries[1]
	if !first.waiting() || second.waiting() || second.Response.status() != "201" {
		t.Errorf("paired wrong: first waiting %v, second %+v", first.waiting(), second.Response)
	}
	if s := screen(m); !strings.Contains(s, "→ 201 9ms") || !strings.Contains(s, "1 waiting") {
		t.Errorf("the answered request doesn't say so:\n%s", s)
	}
	// A hot restart: the first's answer won't come.
	m.feed([]byte("Restarted application in 1,234ms.\n"))
	if m.parser.InFlight() != 0 || first.waiting() {
		t.Error("the restart left calls waiting")
	}
	if s := screen(m); !strings.Contains(s, "→ ⊘ no reply") {
		t.Errorf("no word of the lost call:\n%s", s)
	}
	_ = time.Second
}

func TestExpandButtonAndAll(t *testing.T) {
	m := testModel(map[string]bool{})
	m.feed([]byte(sample))
	if s := screen(m); !strings.Contains(s, "▸ more") || !strings.Contains(s, "⊞ open all") {
		t.Fatalf("no expand buttons:\n%s", s)
	}
	m.Update(keyText("x"))
	if s := screen(m); !strings.Contains(s, `"a": 1`) || !strings.Contains(s, "▾ less") {
		t.Errorf("x did not open the calls:\n%s", s)
	}
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"GET","url":"https://a.test/v2","status":200,"body":{"later":1}}` + "\n"))
	if s := screen(m); !strings.Contains(s, `"later": 1`) {
		t.Errorf("a new call didn't come open:\n%s", s)
	}
}
