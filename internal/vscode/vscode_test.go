package vscode

import (
	"encoding/json"
	"testing"
)

func TestWithTitle(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"", "{\n    \"terminal.integrated.tabs.title\": \"${sequence}\"\n}\n"},
		{"{}", "{\n    \"terminal.integrated.tabs.title\": \"${sequence}\"\n}"},
		{"{\n\t\"a\": 1\n}", "{\n\t\"terminal.integrated.tabs.title\": \"${sequence}\",\n\t\"a\": 1\n}"},
		{"// mine\n{\n  // b\n  \"b\": {\"c\": 2}\n}\n", "// mine\n{\n  \"terminal.integrated.tabs.title\": \"${sequence}\",\n  // b\n  \"b\": {\"c\": 2}\n}\n"},
		{"/* { */ {\"a\": 1}", "/* { */ {\n    \"terminal.integrated.tabs.title\": \"${sequence}\",\"a\": 1}"},
	} {
		out, ok := withTitle([]byte(c.in))
		if !ok || string(out) != c.want {
			t.Errorf("withTitle(%q) = %q, %v; want %q", c.in, out, ok, c.want)
		}
	}
	for _, in := range []string{
		"{\"terminal.integrated.tabs.title\": \"${process}\"}",
		"{\n  // \"terminal.integrated.tabs.title\": \"${process}\"\n}",
		"[]",
	} {
		if out, ok := withTitle([]byte(in)); ok {
			t.Errorf("withTitle(%q) = %q, want it left alone", in, out)
		}
	}
	out, _ := withTitle([]byte("{\n    \"a\": 1\n}"))
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil || v["terminal.integrated.tabs.title"] != "${sequence}" || v["a"] != 1.0 {
		t.Errorf("not valid JSON with both settings: %s (%v)", out, err)
	}
}
