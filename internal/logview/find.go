package logview

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Find looks for a word in the logs without hiding the others, unlike the
// filter: ctrl+f asks for the word, then n and N jump to the next and the
// previous log that has it, the way a browser's find does. The filter's
// folding applies, so a Persian ي finds a ی.

// startFind asks for the word to find.
func (m *Model) startFind() tea.Cmd {
	m.finding = true
	m.input.Prompt = "Find: "
	m.input.Placeholder = "a word in the logs · enter, then n and N jump between them"
	m.input.SetValue(m.find)
	m.input.CursorEnd()
	m.filtering = true
	m.resize()
	return m.input.Focus()
}

// endFind closes the question, and searches for what was typed when ok.
func (m *Model) endFind(ok bool) tea.Cmd {
	m.finding, m.filtering = false, false
	m.input.Prompt = "Filter: "
	m.input.Placeholder = filterHint
	var cmd tea.Cmd
	if ok {
		m.find = fold(strings.TrimSpace(m.input.Value()))
		m.jumped = nil
		if m.find != "" {
			cmd = m.nextFound(1)
		}
	}
	m.resize()
	return cmd
}

// nextFound shows the next log down (dir 1) or up (-1) that has the word,
// from the one it showed last, wrapping around at the ends.
func (m *Model) nextFound(dir int) tea.Cmd {
	shown := m.shownEntries()
	var hits []*Entry
	for _, e := range shown {
		if !e.isRule() && e.Level != "paged" && strings.Contains(e.Searchable(), m.find) {
			hits = append(hits, e)
		}
	}
	if len(hits) == 0 {
		m.setStatus(`⊘ "` + m.find + `" is not in the logs shown · ctrl+f finds another word`)
		return nil
	}
	at := -1
	if i := indexOf(shown, m.jumped); i >= 0 {
		// The first hit past the one shown last.
		for j, h := range hits {
			if k := indexOf(shown, h); dir > 0 && k > i || dir < 0 && k < i {
				at = j
				if dir > 0 {
					break
				}
			}
		}
	}
	if at < 0 { // wrap
		at = 0
		if dir < 0 {
			at = len(hits) - 1
		}
	}
	m.setStatus(fmt.Sprintf(`⌕ "%s" %d of %d · n next, N previous, esc stops`, m.find, at+1, len(hits)))
	return m.jumpTo(hits[at])
}
