package logview

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Folding JSON: in a log's body, the line that opens an object or an array
// has a ▾ at its left, and a click on the row closes that part to one line
// (▸ "user": {…} 4 keys) or opens it again. A middle click does the same
// for everything inside it too. The rest of the body stays as it is, so a
// long response can be read part by part.
//
//	▾ "user": {            ▸ "user": {…} 4 keys,
//	    "id": 7,             "items": [
//	    ...
//	  },

// detailsFold is where the folds of a log's details start, so they don't
// share a number with its lines'.
const detailsFold = 1 << 20

// foldRow is a line to draw, or one in a folded part (not drawn).
type foldRow struct {
	line   Line
	key    int  // the opener's number, to fold it; -1 for other lines
	folded bool // an opener that is closed
	note   string
}

// jsonBlocks finds the lines that open a part of the JSON and the line
// that closes each: {open index: close index}. Text the app cut short has
// openers with no closer, which can't be folded.
func jsonBlocks(lines []Line) map[int]int {
	var stack []int
	blocks := map[int]int{}
	for i, l := range lines {
		if l.Kind != LineJSON {
			stack = nil // another kind of line ends the JSON
			continue
		}
		t := strings.TrimSpace(l.Text)
		if t == "" {
			continue
		}
		if (t[0] == '}' || t[0] == ']') && len(stack) > 0 {
			blocks[stack[len(stack)-1]] = i
			stack = stack[:len(stack)-1]
		}
		if last := t[len(t)-1]; last == '{' || last == '[' {
			stack = append(stack, i)
		}
	}
	return blocks
}

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// foldLines is the lines to draw: the ones inside a folded part left out,
// and its opener closed to one line. base numbers the openers.
func foldLines(lines []Line, folds map[int]bool, base int) []foldRow {
	blocks := jsonBlocks(lines)
	var out []foldRow
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		end, opener := blocks[i]
		if !opener {
			out = append(out, foldRow{line: l, key: -1})
			continue
		}
		key := base + i
		if !folds[key] {
			out = append(out, foldRow{line: l, key: key})
			continue
		}
		closer := strings.TrimSpace(lines[end].Text)
		n := 0
		for _, c := range lines[i+1 : end] {
			if t := strings.TrimSpace(c.Text); indentOf(c.Text) == indentOf(l.Text)+2 && t != "" && t[0] != '}' && t[0] != ']' {
				n++
			}
		}
		unit := "items"
		if strings.HasSuffix(l.Text, "{") {
			unit = "keys"
		}
		if n == 1 {
			unit = strings.TrimSuffix(unit, "s")
		}
		out = append(out, foldRow{line: Line{l.Text + "…" + closer, LineJSON}, key: key, folded: true,
			note: fmt.Sprintf("%d %s", n, unit)})
		i = end
	}
	return out
}

// toggleFold closes or opens the part of e's JSON that opens at key; deep
// does the same to everything inside it, to match.
func (m *Model) toggleFold(e *Entry, key int, deep bool) {
	if e.folds == nil {
		e.folds = map[int]bool{}
	}
	now := !e.folds[key]
	e.folds[key] = now
	if deep {
		for _, g := range []struct {
			lines []Line
			base  int
		}{{e.Lines, 0}, {e.detailLines(), detailsFold}} {
			blocks := jsonBlocks(g.lines)
			if end, ok := blocks[key-g.base]; ok && key >= g.base {
				for open, close := range blocks {
					if open > key-g.base && close < end {
						e.folds[g.base+open] = now
					}
				}
			}
		}
	}
	e.cached = cacheKey{}
	m.dirty = true
}

// detailLines are the details as they are drawn: a request's query, then
// its headers and body.
func (e *Entry) detailLines() []Line {
	if e.Level == "request" {
		return append(queryOf(e.Lines), e.Details...)
	}
	return e.Details
}

// foldClick is a click at x, y on a row that opens a part of some JSON.
func (m *Model) foldClick(x, y int, deep bool) tea.Cmd {
	r, ok := m.rowAt(x, y)
	if !ok || r.fold == 0 || r.entry.hidden {
		return nil
	}
	m.toggleFold(r.entry, r.fold-1, deep)
	return nil
}
