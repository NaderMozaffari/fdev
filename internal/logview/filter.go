package logview

import (
	"strings"
	"unicode"
)

// The filter is words that a log must all match. A word looks anywhere in
// the log; key:value looks in one part of it; a - before a word keeps the
// logs that don't match it.
//
//	tag:Billing  level:error  url:/v1/me  status:4  method:post  name:getProfile
//	is:bookmarked  is:pinned  is:tracked  is:hidden  is:waiting  "two words"  -tag:Firebase

type filterTerm struct {
	key, value string // key "" looks anywhere
	not        bool
}

var termKeys = map[string]string{
	"tag": "tag", "t": "tag", "level": "level", "l": "level", "url": "url", "path": "url",
	"status": "status", "s": "status", "method": "method", "name": "name", "is": "is",
}

// parseFilter reads a filter, already folded.
func parseFilter(s string) []filterTerm {
	var out []filterTerm
	for _, word := range splitWords(s) {
		t := filterTerm{}
		if strings.HasPrefix(word, "-") && len(word) > 1 {
			t.not, word = true, word[1:]
		}
		if k, v, ok := strings.Cut(word, ":"); ok && termKeys[k] != "" && v != "" {
			t.key, t.value = termKeys[k], strings.Trim(v, `"`)
		} else {
			t.value = strings.Trim(word, `"`)
		}
		if t.value != "" {
			out = append(out, t)
		}
	}
	return out
}

// splitWords splits on spaces, keeping "quoted words" together.
func splitWords(s string) []string {
	var out []string
	var cur strings.Builder
	quoted := false
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			cur.WriteRune(r)
		case unicode.IsSpace(r) && !quoted:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// matches is whether e passes every term.
func (m *Model) matches(e *Entry, terms []filterTerm) bool {
	for _, t := range terms {
		if m.matchTerm(e, t) == t.not {
			return false
		}
	}
	return true
}

func (m *Model) matchTerm(e *Entry, t filterTerm) bool {
	switch t.key {
	case "tag":
		return strings.Contains(fold(e.Tag), t.value)
	case "level":
		name, _ := label(themeless, e.Level)
		return strings.HasPrefix(e.Level, t.value) || strings.HasPrefix(strings.ToLower(name), t.value) ||
			e.Category == t.value
	case "url":
		return e.Category == "network" && strings.Contains(fold(e.URL+" "+e.path()), t.value)
	case "status":
		return strings.HasPrefix(e.status(), t.value)
	case "method":
		return e.Category == "network" && strings.EqualFold(e.method(), t.value)
	case "name":
		return strings.Contains(fold(e.Name), t.value)
	case "is":
		switch {
		case strings.HasPrefix(t.value, "book"):
			return e.bookmarked
		case strings.HasPrefix(t.value, "hid"):
			return e.hidden
		case strings.HasPrefix(t.value, "pin"):
			return e.pinned
		case strings.HasPrefix(t.value, "track"):
			return m.trackOf(e) != nil
		case strings.HasPrefix(t.value, "wait"), strings.HasPrefix(t.value, "pend"):
			return e.waiting() && !m.exited
		case t.value == "error":
			return e.Category == "error" || e.Level == "fail"
		}
		return false
	}
	return strings.Contains(e.Searchable(), t.value)
}

// setFilter filters by s, or shows everything for "".
func (m *Model) setFilter(s string) {
	m.filter = fold(strings.TrimSpace(s))
	m.terms = parseFilter(m.filter)
	m.changed()
}

// likeThis is a filter word for logs like e: its call for a network log,
// its tag, else the start of its message.
func likeThis(e *Entry) string {
	switch {
	case e.Category == "network":
		return "url:" + quote(fold(e.path()))
	case e.Tag != "":
		return "tag:" + quote(fold(e.Tag))
	case e.Kind == KindTool:
		return quote(fold(firstWords(stripANSI(e.Text), 4)))
	}
	return quote(fold(firstWords(e.Text, 4)))
}

func quote(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + strings.ReplaceAll(s, `"`, "") + `"`
	}
	return s
}

func firstWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ")
}
