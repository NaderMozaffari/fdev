package version

import (
	"strings"
	"testing"
)

func TestDescribe(t *testing.T) {
	defer func(c string) { Current = c }(Current)
	for v, want := range map[string]string{
		"v0.2.0-beta.1": "v0.2.0-beta.1 (beta)",
		"v1.0.0":        "v1.0.0",
		"v0.0.0-20261007203013-07c1a2b1fb8f+dirty": "dev·07c1a2b (built from source, with uncommitted changes)",
		"v0.2.1-0.20261007203013-07c1a2b1fb8f":     "dev·07c1a2b (built from source",
		"dev":                                      "dev (built from source",
	} {
		Current = v
		if got := Describe(); !strings.HasPrefix(got, want) {
			t.Errorf("Describe() for %s = %q, want %q", v, got, want)
		}
	}
}
