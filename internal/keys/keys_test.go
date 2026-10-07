package keys

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLatin(t *testing.T) {
	typed := func(s string) tea.KeyPressMsg {
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: s}
	}
	for _, c := range []struct {
		msg  tea.KeyPressMsg
		want string
	}{
		{typed("ق"), "r"}, // Persian: hot reload
		{typed("ً"), "R"}, // Persian shift+r: hot restart
		{typed("ض"), "q"}, // quit
		{typed("۳"), "3"}, // Persian digit
		{typed("٣"), "3"}, // Arabic digit
		{typed("؟"), "?"}, // help
		{typed("و"), ","}, // the time toggle
		{typed("ک"), ";"}, // the tag toggle
		{typed("ك"), ";"}, // Arabic kaf
		{typed("م"), "l"}, // log look
		{typed("к"), "r"}, // Russian
		{typed("К"), "R"}, //
		{typed("ר"), "r"}, // Hebrew
		{typed("ρ"), "r"}, // Greek
		{typed("r"), "r"}, // already Latin
		{typed("/"), "/"}, //
		{typed("中"), "中"}, // no layout to map it from
		{tea.KeyPressMsg{Code: 'ش', Mod: tea.ModCtrl}, "ctrl+a"},
		{tea.KeyPressMsg{Code: 'س', Mod: tea.ModCtrl}, "ctrl+s"},
		{tea.KeyPressMsg{Code: 'ไ', Mod: tea.ModCtrl, BaseCode: 'w'}, "ctrl+w"}, // Thai, from the terminal's report
		{tea.KeyPressMsg{Code: 'ไ', Text: "ไ", BaseCode: 'w', Mod: tea.ModShift}, "W"},
		{tea.KeyPressMsg{Code: tea.KeyEnter}, "enter"},
	} {
		if got := Latin(c.msg).String(); got != c.want {
			t.Errorf("Latin(%q) = %q, want %q", c.msg.String(), got, c.want)
		}
	}
}
