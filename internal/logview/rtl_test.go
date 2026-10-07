package logview

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPersianIsJoinedRightToLeft(t *testing.T) {
	cases := map[string]string{
		// سلام: س initial, ل medial, ا final, م isolated; drawn right to left.
		"سلام":          "\uFEE1\uFE8E\uFEE0\uFEB3",
		"ok سلام done":  "ok \uFEE1\uFE8E\uFEE0\uFEB3 done",
		"سفارش 123 ثبت": "\uFE96\uFE92\uFE9B 123 \uFEB5\uFEAD\uFE8E\uFED4\uFEB3",
		"id 42":         "id 42",
		"(کد)":          "(\uFEAA\uFB90)",
	}
	for in, want := range cases {
		if got := visual(in); ansi.Strip(got) != want {
			t.Errorf("visual(%q) = %q, want %q", in, ansi.Strip(got), want)
		}
	}
	// Colors stay on their letters, and the width is the same.
	in := "\x1b[1mبا\x1b[m x"
	if got := visual(in); ansi.StringWidth(got) != ansi.StringWidth(in) || ansi.Strip(got) != "\uFE8E\uFE91 x" {
		t.Errorf("styled: %q", got)
	}
}
