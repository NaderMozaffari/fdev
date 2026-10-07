package version

import (
	"strconv"
	"strings"
)

// Releases are tagged with semantic versions (semver.org): v1.2.0 is a
// stable release, and a version with a pre-release part, like
// v0.2.0-beta.1 or v1.0.0-rc.2, is a beta, published as a GitHub
// pre-release. Install scripts and fdev update keep to one channel: stable
// (the default), or beta, which has the betas as well.
const (
	Stable = "stable"
	Beta   = "beta"
)

// Channel is the channel version v belongs to: Beta for a pre-release,
// Stable for a release, "" for what isn't a release (a local build, or a
// Go pseudo-version).
func Channel(v string) string {
	s, ok := parse(v)
	switch {
	case !ok || pseudo.MatchString(v):
		return ""
	case s.pre != nil:
		return Beta
	}
	return Stable
}

// Valid reports whether v is a semantic version, with or without its v.
func Valid(v string) bool {
	_, ok := parse(v)
	return ok
}

// Compare orders two semantic versions as semver.org does: -1 when a comes
// before b, 0 when they are equal (build metadata aside), 1 after. A
// version that isn't one comes before every version.
func Compare(a, b string) int {
	x, okA := parse(a)
	y, okB := parse(b)
	switch {
	case !okA && !okB:
		return 0
	case !okA:
		return -1
	case !okB:
		return 1
	}
	for i := range x.core {
		if c := cmpInt(x.core[i], y.core[i]); c != 0 {
			return c
		}
	}
	// A pre-release comes before its release: v1.0.0-rc.1 < v1.0.0.
	switch {
	case x.pre == nil && y.pre == nil:
		return 0
	case x.pre == nil:
		return 1
	case y.pre == nil:
		return -1
	}
	for i := 0; i < len(x.pre) && i < len(y.pre); i++ {
		if c := cmpIdent(x.pre[i], y.pre[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(x.pre), len(y.pre))
}

type semver struct {
	core [3]int
	pre  []string // nil for a release
}

func parse(v string) (semver, bool) {
	var s semver
	v = strings.TrimPrefix(v, "v")
	v, _, _ = strings.Cut(v, "+") // build metadata doesn't count
	v, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return s, false
	}
	for i, p := range parts {
		n, ok := number(p)
		if !ok {
			return s, false
		}
		s.core[i] = n
	}
	if hasPre {
		s.pre = strings.Split(pre, ".")
		for _, id := range s.pre {
			if id == "" || strings.Trim(id, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-") != "" {
				return s, false
			}
		}
	}
	return s, true
}

// number is a numeric identifier: digits, without leading zeros.
func number(s string) (int, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// cmpIdent orders pre-release identifiers: numbers by value, before words,
// which go in ASCII order (alpha < beta < rc).
func cmpIdent(a, b string) int {
	x, numA := number(a)
	y, numB := number(b)
	switch {
	case numA && numB:
		return cmpInt(x, y)
	case numA:
		return -1
	case numB:
		return 1
	}
	return strings.Compare(a, b)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
