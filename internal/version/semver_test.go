package version

import "testing"

func TestCompare(t *testing.T) {
	// In order, each before the next (semver.org's own example, and ours).
	order := []string{
		"v0.1.3",
		"v0.2.0-alpha", "v0.2.0-alpha.1", "v0.2.0-alpha.beta", "v0.2.0-beta",
		"v0.2.0-beta.2", "v0.2.0-beta.11", "v0.2.0-rc.1", "v0.2.0",
		"v1.0.0", "v1.10.0",
	}
	for i := range order {
		for j := range order {
			want := cmpInt(i, j)
			if got := Compare(order[i], order[j]); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", order[i], order[j], got, want)
			}
		}
	}
	if Compare("v1.0.0+build.5", "1.0.0") != 0 {
		t.Error("build metadata counted")
	}
	if Compare("dev", "v0.0.1") != -1 || Compare("v0.0.1", "dev") != 1 {
		t.Error("a local build isn't older than every release")
	}
}

func TestChannel(t *testing.T) {
	for v, want := range map[string]string{
		"v1.0.0": Stable, "v0.2.0-beta.1": Beta, "v1.0.0-rc.1": Beta, "v1.0.0+meta": Stable,
		"dev": "", "v1.0": "", "v01.0.0": "", "v1.0.0-": "", "v0.0.0-20261007120000-1a2b3c4d5e6f": "",
	} {
		if got := Channel(v); got != want {
			t.Errorf("Channel(%q) = %q, want %q", v, got, want)
		}
	}
}
