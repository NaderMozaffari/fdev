package logview

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Repeats: a log that comes again right after itself, the same in all but
// its time, is one row that says how many times it came and from when to
// when:
//
//	12:04:01 INFO  Billing: retrying  ×12 · 12:04:01 – 12:04:30
//
// Any other log in between starts a new row, which counts again. Every one
// of them is still in the session's file, at its own time; a page read
// back is folded the same way. Network calls are never folded: each is a
// call of its own.

// sameLog is whether b says again what a said.
func sameLog(a, b *Entry) bool {
	if a.Kind == KindTool || a.Category == "network" || a.isRule() || a.Level == "reload" {
		return false
	}
	return a.Kind == b.Kind && a.Category == b.Category && a.Level == b.Level && a.Tag == b.Tag &&
		a.Text == b.Text && a.At == b.At && a.Name == b.Name && a.URL == b.URL &&
		slices.Equal(a.Fields, b.Fields) && slices.Equal(a.Lines, b.Lines) &&
		slices.Equal(a.Stack, b.Stack) && slices.Equal(a.Details, b.Details)
}

// fold puts e on prev's row when it repeats it, and is whether it did.
func (m *Model) fold(prev, e *Entry) bool {
	if prev.saved || e.saved || prev.page != e.page || prev.hidden ||
		e.bookmarked || e.pinned || e.hidden || e.Expanded || e == m.sel || !sameLog(prev, e) {
		return false
	}
	prev.repeats = append(prev.repeats, e)
	prev.cached = cacheKey{}
	m.dirty = true
	return true
}

// foldNew folds a log that just came onto the last row, unless it may
// still grow (an exception, a stack trace): then it is left for settle.
func (m *Model) foldNew(e *Entry) bool {
	if n := len(m.entries); n > 0 && !m.parser.Growing() {
		return m.fold(m.entries[n-1], e)
	}
	return false
}

// settle folds the last log, which may have grown since it came, once
// another log comes after it or a page closes.
func (m *Model) settle() {
	e := m.unsettled
	m.unsettled = nil
	if n := len(m.entries); e != nil && n >= 2 && m.entries[n-1] == e && m.fold(m.entries[n-2], e) {
		m.entries = m.entries[:n-1]
	}
}

// foldAll folds the repeats in logs read back from a file.
func foldAll(entries []*Entry) []*Entry {
	out := entries[:0]
	for _, e := range entries {
		if n := len(out); n > 0 && sameLog(out[n-1], e) {
			out[n-1].repeats = append(out[n-1].repeats, e)
			continue
		}
		out = append(out, e)
	}
	clear(entries[len(out):])
	return out
}

// times is how many times e came: 1, and its repeats.
func (e *Entry) times() int { return 1 + len(e.repeats) }

// until is when e came the last time.
func (e *Entry) until() time.Time {
	if n := len(e.repeats); n > 0 {
		return e.repeats[n-1].Time
	}
	return e.Time
}

// repeatText is how a repeated log says it came again: "×12 · 12:04:01 – 12:04:30".
func (e *Entry) repeatText(layout string) string {
	return fmt.Sprintf("×%d · %s – %s", e.times(), e.Time.Format(layout), e.until().Format(layout))
}

// rawLines is e's original lines with the time each came, its repeats' too.
func (e *Entry) rawLines() string {
	var b strings.Builder
	for _, x := range append([]*Entry{e}, e.repeats...) {
		for _, r := range x.Raw {
			b.WriteString(rawLine(x.Time, r) + "\n")
		}
	}
	return b.String()
}
