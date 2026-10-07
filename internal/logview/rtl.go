package logview

import (
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Persian and Arabic text in a terminal: most terminals (VS Code's, iTerm2,
// Ghostty, kitty, Windows Terminal) draw it letter by letter, left to
// right, unjoined: "سلام" reads backwards, its letters apart. fdev joins
// the letters (their presentation forms: initial, medial, final) and lays
// out each right-to-left run of a row right to left, numbers in it left to
// right, as the Unicode bidi algorithm does in a left-to-right line. The
// screen gets it so; the logs, the saved files and what is copied keep the
// text as it was.
//
// Terminals that do it themselves (Terminal.app, GNOME's and others on
// VTE, Konsole, mlterm) get the text as it is: the look's RTL setting.

// RTL settings, config.Look.RTL.
const (
	RTLAuto     = "auto"
	RTLFdev     = "fdev"
	RTLTerminal = "terminal"
)

// terminalDoesRTL is whether the terminal fdev runs in lays out and joins
// right-to-left text itself.
func terminalDoesRTL() bool {
	return os.Getenv("TERM_PROGRAM") == "Apple_Terminal" || os.Getenv("VTE_VERSION") != "" ||
		os.Getenv("KONSOLE_VERSION") != "" || strings.HasPrefix(os.Getenv("TERM"), "mlterm")
}

// rtl is whether fdev lays out right-to-left text, for the look.
func (m *Model) rtl() bool {
	switch m.look.RTL {
	case RTLFdev:
		return true
	case RTLTerminal:
		return false
	}
	return !terminalDoesRTL()
}

func isRTL(r rune) bool {
	return r >= 0x0590 && r <= 0x08ff || r >= 0xfb1d && r <= 0xfdff || r >= 0xfe70 && r <= 0xfeff
}

func hasRTL(s string) bool {
	for _, r := range s {
		if isRTL(r) {
			return true
		}
	}
	return false
}

// visualLines lays out the right-to-left text of every line of s.
func visualLines(s string) string {
	if !hasRTL(s) {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if hasRTL(l) {
			lines[i] = visual(l)
		}
	}
	return strings.Join(lines, "\n")
}

// cell is a character on the screen: its base, the marks on it, and the
// colors it is drawn in.
type cell struct {
	style string // the SGR sequences in effect, since the last reset
	text  string
	base  rune
}

// visual is a row, styled with SGR sequences, in the order to draw it.
func visual(row string) string {
	cells, tail := cellsOf(row)
	shape(cells)
	reorder(cells)
	var b strings.Builder
	style := ""
	for _, c := range cells {
		if c.style != style {
			b.WriteString("\x1b[m" + c.style)
			style = c.style
		}
		b.WriteString(c.text)
	}
	if style != "" || tail != "" {
		b.WriteString("\x1b[m")
	}
	return b.String()
}

// cellsOf splits a row into cells; tail is the styles after the last one.
func cellsOf(row string) ([]cell, string) {
	var cells []cell
	style := ""
	for i := 0; i < len(row); {
		if row[i] == 0x1b {
			loc := escapeSeq.FindStringIndex(row[i:])
			n := 1
			if loc != nil && loc[0] == 0 {
				n = loc[1]
			}
			seq := row[i : i+n]
			if resetSeq.MatchString(seq) && len(seq) == len(resetSeq.FindString(seq)) {
				style = ""
			} else {
				style += seq
			}
			i += n
			continue
		}
		r, n := utf8.DecodeRuneInString(row[i:])
		i += n
		joins := r == 0x200c || r == 0x200d || unicode.Is(unicode.Mn, r)
		if joins && len(cells) > 0 {
			cells[len(cells)-1].text += string(r)
			continue
		}
		cells = append(cells, cell{style: style, text: string(r), base: r})
	}
	return cells, style
}

// ── Joining letters ──────────────────────────────────────────────────────────

// forms are a letter's isolated, final, initial and medial forms; a letter
// that joins only the one before it has no initial or medial.
var forms = map[rune][4]rune{
	'ء': {0xFE80}, 'آ': {0xFE81, 0xFE82}, 'أ': {0xFE83, 0xFE84}, 'ؤ': {0xFE85, 0xFE86},
	'إ': {0xFE87, 0xFE88}, 'ئ': {0xFE89, 0xFE8A, 0xFE8B, 0xFE8C}, 'ا': {0xFE8D, 0xFE8E},
	'ب': {0xFE8F, 0xFE90, 0xFE91, 0xFE92}, 'ة': {0xFE93, 0xFE94}, 'ت': {0xFE95, 0xFE96, 0xFE97, 0xFE98},
	'ث': {0xFE99, 0xFE9A, 0xFE9B, 0xFE9C}, 'ج': {0xFE9D, 0xFE9E, 0xFE9F, 0xFEA0},
	'ح': {0xFEA1, 0xFEA2, 0xFEA3, 0xFEA4}, 'خ': {0xFEA5, 0xFEA6, 0xFEA7, 0xFEA8},
	'د': {0xFEA9, 0xFEAA}, 'ذ': {0xFEAB, 0xFEAC}, 'ر': {0xFEAD, 0xFEAE}, 'ز': {0xFEAF, 0xFEB0},
	'س': {0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4}, 'ش': {0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8},
	'ص': {0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC}, 'ض': {0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0},
	'ط': {0xFEC1, 0xFEC2, 0xFEC3, 0xFEC4}, 'ظ': {0xFEC5, 0xFEC6, 0xFEC7, 0xFEC8},
	'ع': {0xFEC9, 0xFECA, 0xFECB, 0xFECC}, 'غ': {0xFECD, 0xFECE, 0xFECF, 0xFED0},
	'ف': {0xFED1, 0xFED2, 0xFED3, 0xFED4}, 'ق': {0xFED5, 0xFED6, 0xFED7, 0xFED8},
	'ك': {0xFED9, 0xFEDA, 0xFEDB, 0xFEDC}, 'ل': {0xFEDD, 0xFEDE, 0xFEDF, 0xFEE0},
	'م': {0xFEE1, 0xFEE2, 0xFEE3, 0xFEE4}, 'ن': {0xFEE5, 0xFEE6, 0xFEE7, 0xFEE8},
	'ه': {0xFEE9, 0xFEEA, 0xFEEB, 0xFEEC}, 'و': {0xFEED, 0xFEEE}, 'ى': {0xFEEF, 0xFEF0},
	'ي': {0xFEF1, 0xFEF2, 0xFEF3, 0xFEF4},
	// Persian.
	'پ': {0xFB56, 0xFB57, 0xFB58, 0xFB59}, 'چ': {0xFB7A, 0xFB7B, 0xFB7C, 0xFB7D},
	'ژ': {0xFB8A, 0xFB8B}, 'ک': {0xFB8E, 0xFB8F, 0xFB90, 0xFB91}, 'گ': {0xFB92, 0xFB93, 0xFB94, 0xFB95},
	'ی': {0xFBFC, 0xFBFD, 0xFBFE, 0xFBFF},
}

// joinsNext is whether a letter joins the one after it.
func joinsNext(c cell) bool {
	if c.base == 'ـ' {
		return true
	}
	f, ok := forms[c.base]
	return ok && f[2] != 0 && !strings.ContainsRune(c.text, 0x200c)
}

// joinsPrev is whether a letter joins the one before it.
func joinsPrev(c cell) bool {
	f, ok := forms[c.base]
	return c.base == 'ـ' || ok && f[1] != 0
}

// shape gives each letter the form for its neighbours.
func shape(cells []cell) {
	for i := range cells {
		f, ok := forms[cells[i].base]
		if !ok {
			continue
		}
		prev := i > 0 && joinsNext(cells[i-1]) && joinsPrev(cells[i])
		next := i+1 < len(cells) && joinsNext(cells[i]) && joinsPrev(cells[i+1])
		form := f[0]
		switch {
		case prev && next:
			form = f[3]
		case prev:
			form = f[1]
		case next:
			form = f[2]
		}
		if form != 0 {
			cells[i].text = string(form) + cells[i].text[len(string(cells[i].base)):]
		}
	}
}

// ── Ordering ─────────────────────────────────────────────────────────────────

type bidiClass int

const (
	bidiNeutral bidiClass = iota
	bidiL
	bidiR
	bidiNumber
)

func classOf(r rune) bidiClass {
	switch {
	case r >= '0' && r <= '9', r >= '۰' && r <= '۹', r >= '٠' && r <= '٩':
		return bidiNumber
	case isRTL(r):
		return bidiR
	case unicode.IsLetter(r):
		return bidiL
	}
	return bidiNeutral
}

var mirrors = map[rune]rune{'(': ')', ')': '(', '[': ']', ']': '[', '{': '}', '}': '{', '<': '>', '>': '<', '«': '»', '»': '«'}

// reorder lays out each right-to-left run right to left: from its first
// right-to-left letter to its last, or a number after it, before a
// left-to-right letter. Numbers in a run stay left to right.
func reorder(cells []cell) {
	start, last := -1, -1
	strong := bidiL
	flush := func() {
		if start >= 0 && last >= start {
			run(cells[start : last+1])
		}
		start, last = -1, -1
	}
	for i, c := range cells {
		switch classOf(c.base) {
		case bidiR:
			if start < 0 {
				start = i
			}
			last, strong = i, bidiR
		case bidiL:
			flush()
			strong = bidiL
		case bidiNumber:
			if strong == bidiR && start >= 0 {
				last = i
			}
		}
	}
	flush()
}

func run(cells []cell) {
	for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
		cells[i], cells[j] = cells[j], cells[i]
	}
	for i := range cells {
		if m, ok := mirrors[cells[i].base]; ok {
			cells[i].text, cells[i].base = string(m), m
		}
	}
	// Numbers read left to right: put each back, with the . , : / in it.
	for i := 0; i < len(cells); {
		if classOf(cells[i].base) != bidiNumber {
			i++
			continue
		}
		j := i
		for j+1 < len(cells) && (classOf(cells[j+1].base) == bidiNumber ||
			strings.ContainsRune(".,:/٫٬", cells[j+1].base) && j+2 < len(cells) && classOf(cells[j+2].base) == bidiNumber) {
			j++
		}
		for a, b := i, j; a < b; a, b = a+1, b-1 {
			cells[a], cells[b] = cells[b], cells[a]
		}
		i = j + 1
	}
}

// ── Matching it ──────────────────────────────────────────────

// fold makes text easy to match: lower case, Persian and Arabic letters of
// one sound the same (ي ی, ك ک), no harakat or zero-width non-joiners, and
// Persian and Arabic digits as 0-9.
func fold(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == 'ي' || r == 'ى':
			return 'ی'
		case r == 'ك':
			return 'ک'
		case r == 'ة':
			return 'ه'
		case r == 'أ' || r == 'إ' || r == 'آ':
			return 'ا'
		case r >= '۰' && r <= '۹':
			return '0' + (r - '۰')
		case r >= '٠' && r <= '٩':
			return '0' + (r - '٠')
		case r == 0x200c || r == 0x200d || r == 0x0640: // ZWNJ, ZWJ, tatweel
			return -1
		case r >= 0x064b && r <= 0x065f, r == 0x0670: // harakat
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
