// Package keys makes fdev's shortcuts work whatever keyboard layout is on:
// a key typed in Persian, Arabic, Russian, Hebrew or Greek counts as the
// key in the same place on a US keyboard (ق is r, ً is R, ۱ is 1).
package keys

import (
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// layouts pair US keys with what other layouts type in their place.
var layouts = [][2]string{
	// Persian (ISIRI 9147: Windows, macOS "Persian - Standard", Linux).
	{"qwertyuiop[]asdfghjkl;'zxcvbnm,", "ضصثقفغعهخحجچشسیبلاتنمکگظطزرذدپو"},
	{"QWERTYUIASFGHJKL\"XCVNM?", "\u0652\u064c\u064d\u064b\u064f\u0650\u064e\u0651" + // ْ ٌ ٍ ً ُ ِ َ ّ
		"\u0624\u0626\u0625\u0623\u0622\u0629\u00bb\u00ab\u061b\u0653\u0698\u0670\u0654\u0621\u061f"}, // ؤ ئ إ أ آ ة » « ؛ ٓ ژ ٰ ٔ ء ؟
	// Arabic (PC 101), where it differs from Persian.
	{"zxcnm", "ئءؤىة"},
	{"d;", "يك"}, // also older Persian layouts' yeh and kaf
	// Russian (ЙЦУКЕН) and Ukrainian.
	{"qwertyuiop[]asdfghjkl;'zxcvbnm,.`", "йцукенгшщзхъфывапролджэячсмитьбюё"},
	{"QWERTYUIOP{}ASDFGHJKL:\"ZXCVBNM<>~", "ЙЦУКЕНГШЩЗХЪФЫВАПРОЛДЖЭЯЧСМИТЬБЮЁ"},
	{"s]'`S}\"", "іїєґІЇЄ"},
	// Hebrew.
	{"ertyuiopasdfghjkl;zxcvbnm,.", "קראטוןםפשדגכעיחלךףזסבהנמצתץ"},
	// Greek.
	{"wertyuiopasdfghjklzxcvbnm", "ςερτυθιοπασδφγηξκλζχψωβνμ"},
	{"ERTYUIOPASDFGHJKLZXCVBNM", "ΕΡΤΥΘΙΟΠΑΣΔΦΓΗΞΚΛΖΧΨΩΒΝΜ"},
}

const usLower, usUpper = "qwertyuiop[]asdfghjkl;'zxcvbnm,./`", "QWERTYUIOP{}ASDFGHJKL:\"ZXCVBNM<>?~"

// table maps a character to the US key in its place; the first layout
// that has it wins.
var table = func() map[rune]rune {
	t := map[rune]rune{}
	for _, l := range layouts {
		us, chars := []rune(l[0]), []rune(l[1])
		if len(us) != len(chars) {
			panic("keys: layout " + l[0] + " is not as long as its keys")
		}
		for i, r := range chars {
			if _, ok := t[r]; !ok {
				t[r] = us[i]
			}
		}
	}
	for i := range 10 { // Persian and Arabic digits
		t['۰'+rune(i)] = '0' + rune(i)
		t['٠'+rune(i)] = '0' + rune(i)
	}
	return t
}()

// shifted is a US key with shift: r → R, / → ?.
func shifted(r rune) rune {
	if unicode.IsLetter(r) {
		return unicode.ToUpper(r)
	}
	if i := indexRune(usLower, r); i >= 0 {
		return []rune(usUpper)[i]
	}
	if i := indexRune("1234567890-=", r); i >= 0 {
		return []rune("!@#$%^&*()_+")[i]
	}
	return r
}

func indexRune(s string, r rune) int {
	for i, c := range []rune(s) {
		if c == r {
			return i
		}
	}
	return -1
}

// Latin is msg as the US key in the same place, for shortcuts: from the
// terminal's own report of it when it gives one (kitty's keyboard protocol),
// else from the layouts above. Other keys come back as they are.
func Latin(msg tea.KeyPressMsg) tea.KeyPressMsg {
	from := msg.Code
	if t := []rune(msg.Text); len(t) == 1 {
		from = t[0]
	}
	if from < utf8.RuneSelf {
		return msg
	}
	to, ok := table[from]
	if !ok && msg.BaseCode > 0 && msg.BaseCode < utf8.RuneSelf {
		to, ok = msg.BaseCode, true
		if msg.Mod.Contains(tea.ModShift) {
			to = shifted(to)
		}
	}
	if !ok {
		return msg
	}
	msg.Code, msg.ShiftedCode = unicode.ToLower(to), 0
	if unicode.IsUpper(to) {
		msg.ShiftedCode = to
	}
	if msg.Mod&^tea.ModShift == 0 { // typed text; with ctrl or alt it's a combination
		msg.Text = string(to)
	} else {
		msg.Text = ""
	}
	return msg
}
