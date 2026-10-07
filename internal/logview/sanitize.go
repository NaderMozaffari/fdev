package logview

import (
	"regexp"
	"strings"
)

var (
	// Any escape sequence: CSI, OSC (ended by BEL or ST), and two-byte ones.
	escapeSeq = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?|\x1b[@-Z\\-_]|\x1b`)
	sgrSeq    = regexp.MustCompile(`^\x1b\[[0-9;:]*m$`)
	resetSeq  = regexp.MustCompile(`\x1b\[0?m`) // a reset of the colors
)

// clean makes text from the device safe to show: no escape sequences (a log
// line could otherwise set the window title, write the clipboard or fake a
// link), no control characters, and no bidi overrides that reorder text.
func clean(s string) string {
	s = escapeSeq.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			return -1
		case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
			return -1
		}
		return r
	}, s)
}

// cleanTool is clean for flutter's own output, which keeps its colors.
// Carriage returns and backspaces (progress spinners) are applied, the way a
// terminal would show the line.
func cleanTool(s string) string {
	if strings.ContainsAny(s, "\r\b") {
		return overwrite(s)
	}
	s = escapeSeq.ReplaceAllStringFunc(s, func(seq string) string {
		if sgrSeq.MatchString(seq) {
			return seq
		}
		return ""
	})
	return strings.Map(func(r rune) rune {
		if r == 0x1b { // what is left of them starts a color
			return r
		}
		if r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f || r >= 0x80 && r <= 0x9f || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 {
			return -1
		}
		return r
	}, s)
}

// overwrite replays \r and \b over the plain text of raw.
func overwrite(raw string) string {
	plain := escapeSeq.ReplaceAllString(raw, "")
	var buf []rune
	cur := 0
	for _, r := range plain {
		switch {
		case r == '\r':
			cur = 0
		case r == '\b':
			if cur > 0 {
				cur--
			}
		case r == '\t':
			r = ' '
			fallthrough
		case r >= 0x20 && r != 0x7f && !(r >= 0x80 && r <= 0x9f) &&
			!(r >= 0x202a && r <= 0x202e) && !(r >= 0x2066 && r <= 0x2069):
			if cur < len(buf) {
				buf[cur] = r
			} else {
				buf = append(buf, r)
			}
			cur++
		}
	}
	return strings.TrimRight(string(buf), " ")
}
