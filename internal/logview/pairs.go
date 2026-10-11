package logview

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// A request and its response are drawn as a pair: a line in a gutter on the
// left joins them, in the call's color, and both rows have a tint of it,
// stronger while the mouse is over either one:
//
//	╭── 12:04:51 REQU   GET → getProfile  /v1/me → 200 142ms
//	│╭─ 12:04:51 REQU   POST → addToCart  /v1/cart → 201 80ms
//	││  12:04:51 INFO   cart: 3 items
//	╰┼─ 12:04:52 RESP   GET ← getProfile  /v1/me status=200
//	 ╰─ 12:04:52 RESP   POST ← addToCart  /v1/cart status=201
//
// Calls waiting at once each take a column of the gutter, up to maxLanes;
// a pair beyond them has its tint only. The gutter is there while a pair
// is shown.

const maxLanes = 4

// partner is the other half of e's call, when both are shown.
func (m *Model) partner(e *Entry) *Entry {
	if m.o.Plain || m.on["raw"] || e.Category != "network" {
		return nil
	}
	o := e.Request
	if e.Level == "request" {
		o = e.Response
	}
	if o == nil || e.hidden || o.hidden || !m.visible(o) {
		return nil
	}
	return o
}

// planLanes gives each pair shown a column of the gutter, from its request
// to its response, and is the width left for the logs.
func (m *Model) planLanes() (map[*Entry]int, int) {
	lane := map[*Entry]int{}
	var busy [maxLanes]*Entry // the request holding each column
	used := 0
	for _, e := range m.entries {
		if !m.visible(e) {
			continue
		}
		o := m.partner(e)
		switch {
		case o == nil:
		case e.Level == "request":
			for j := range busy {
				if busy[j] == nil {
					busy[j], lane[e], lane[o] = e, j, j
					used = max(used, j+1)
					break
				}
			}
		default:
			if j, ok := lane[e]; ok && busy[j] == o {
				busy[j] = nil
			}
		}
	}
	// The gutter only grows while pairs are shown, so the logs don't shift
	// back and forth as calls come and go.
	if used == 0 {
		m.lanes = 0
	} else {
		m.lanes = max(m.lanes, used)
	}
	m.gutterW = 0
	if m.lanes > 0 {
		m.gutterW = m.lanes + 2
	}
	return lane, m.width - m.gutterW
}

// callColor is a call's color: one of a few, in turn, so calls next to each
// other differ.
func (m *Model) callColor(e *Entry) color.Color {
	th := m.th
	palette := []color.Color{th.Info, th.Warn, th.Green, th.Fuchsia, th.Indigo}
	return palette[max(e.call-1, 0)%len(palette)]
}

// tintRows paints the first row of a pair's request or response in its
// call's color, faintly; stronger while the mouse is over the pair.
func (m *Model) tintRows(rows []row, e *Entry, width int) []row {
	if len(rows) == 0 || !rows[0].first {
		return rows
	}
	base := m.th.TermBg
	if base == nil {
		base = m.th.BarBg
	}
	step := 2
	if h := m.hoverEntry; h != nil && (h == e || h == m.partner(e)) {
		step = 4
	}
	out := append([]row(nil), rows...)
	out[0].text = paintBg(out[0].text, width, lipgloss.Blend1D(10, base, m.callColor(e))[step])
	return out
}

// withGutter puts the gutter before each row: the pairs' lines.
func (m *Model) withGutter(rows []row, lane map[*Entry]int) []row {
	if m.gutterW == 0 {
		return rows
	}
	var active [maxLanes]*Entry // the request whose line goes through each column
	pad := strings.Repeat(" ", m.gutterW)
	for i, r := range rows {
		end := -1
		if j, ok := lane[r.entry]; ok && r.first {
			end = j
		}
		if end < 0 {
			if !hasActive(active[:m.lanes]) {
				rows[i].text = pad + r.text
			} else {
				var b strings.Builder
				for _, a := range active[:m.lanes] {
					if a == nil {
						b.WriteString(" ")
					} else {
						b.WriteString(lipgloss.NewStyle().Foreground(m.callColor(a)).Render("│"))
					}
				}
				rows[i].text = b.String() + "  " + r.text
			}
		} else {
			e := r.entry
			line := lipgloss.NewStyle().Foreground(m.callColor(e))
			var b strings.Builder
			for j, a := range active[:m.lanes] {
				switch {
				case j < end && a != nil:
					b.WriteString(lipgloss.NewStyle().Foreground(m.callColor(a)).Render("│"))
				case j < end:
					b.WriteString(" ")
				case j == end && e.Level == "request":
					b.WriteString(line.Render("╭"))
				case j == end:
					b.WriteString(line.Render("╰"))
				case a != nil:
					b.WriteString(line.Render("┼"))
				default:
					b.WriteString(line.Render("─"))
				}
			}
			rows[i].text = b.String() + line.Render("─") + " " + r.text
			if e.Level == "request" {
				active[end] = e
			} else {
				active[end] = nil
			}
		}
		if r.entry == nil {
			rows[i].text = ansi.Truncate(rows[i].text, m.width, "")
		}
	}
	return rows
}

func hasActive(lanes []*Entry) bool {
	for _, a := range lanes {
		if a != nil {
			return true
		}
	}
	return false
}
