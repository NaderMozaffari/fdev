package logview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestValuesAtHand(t *testing.T) {
	m := testModel(map[string]bool{})
	m.parser.ValueKeys = []string{"userId"}
	m.feed([]byte(
		`I/flutter (1): ⟪fd⟫{"l":"http","p":"request","method":"GET","url":"https://a.test/v1/me","headers":{"Authorization":"Bearer abc.def"}}` + "\n" +
			`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"POST","url":"https://a.test/auth","status":200,"body":{"data":{"access_token":"tok1","refresh_token":"••• (12 chars)","user_id":42}}}` + "\n" +
			`I/flutter (1): ⟪fd⟫{"l":"value","k":"pushToken","v":"fcm-123"}` + "\n" +
			`I/flutter (1): ⟪fd⟫{"l":"http","p":"response","method":"POST","url":"https://a.test/auth","status":200,"body":"{\"access_token\":\"tok2\"}"}` + "\n"))
	got := map[string]string{}
	for _, k := range m.values {
		got[k.Name] = k.Value.Value
	}
	want := map[string]string{"Authorization": "Bearer abc.def", "access_token": "tok2", "user_id": "42", "pushToken": "fcm-123"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	if _, ok := got["refresh_token"]; ok {
		t.Error("a masked value was kept")
	}
	m.Update(keyText("k"))
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	s := screen(m)
	for _, w := range []string{"Values", "Authorization", "fcm-123", "changed 1 time", "access_token · POST /auth · response body"} {
		if !strings.Contains(s, w) {
			t.Errorf("values window has no %q:\n%s", w, s)
		}
	}
	m.Update(keyText("k"))
	if m.vault != nil {
		t.Error("k did not close the values")
	}

	// From More's chip, and closed again by the ✕ in its border.
	m.toggleHelp()
	x, y := at(t, m, "k values")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if m.vault == nil {
		t.Fatalf("the k values chip did not open the values:\n%s", screen(m))
	}
	x, y = at(t, m, " ✕ ")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if m.vault != nil {
		t.Errorf("the ✕ did not close the values:\n%s", screen(m))
	}
}
