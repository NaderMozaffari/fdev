// Package rail prints a command's steps down a line on the left, the way
// npm create's tools do, for fdev's commands that print as they go
// (update, channel, init):
//
//	┌  fdev update
//	│
//	◇  Found v0.3.0-beta.2, a beta
//	│
//	◇  Checksum matches
//	│
//	└  Done: v0.2.1 → v0.3.0-beta.2
package rail

import (
	"io"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	line  = lipgloss.NewStyle().Foreground(lipgloss.BrightBlack)
	faint = lipgloss.NewStyle().Foreground(lipgloss.BrightBlack)
	green = lipgloss.NewStyle().Foreground(lipgloss.Green)
	warn  = lipgloss.NewStyle().Foreground(lipgloss.Yellow)
	red   = lipgloss.NewStyle().Foreground(lipgloss.Red)
	badge = lipgloss.NewStyle().Background(lipgloss.Magenta).Foreground(lipgloss.Black).Bold(true).Padding(0, 1)
)

// Start is the rail's top, with the command's name: ┌  fdev update.
func Start(w io.Writer, title string) {
	lipgloss.Fprintln(w, line.Render("┌")+" "+badge.Render(title))
}

// Step is a step done: ◇  text.
func Step(w io.Writer, text string) { mark(w, green.Render("◇"), text) }

// Warn is something to look at: ▲  text.
func Warn(w io.Writer, text string) { mark(w, warn.Render("▲"), text) }

// Note is more about the step above, on the rail and faint; one line each.
func Note(w io.Writer, text string) {
	for _, l := range strings.Split(text, "\n") {
		lipgloss.Fprintln(w, line.Render("│")+"  "+faint.Render(l))
	}
}

// End is the rail's bottom: └  text.
func End(w io.Writer, text string) {
	lipgloss.Fprintln(w, line.Render("│"))
	lipgloss.Fprintln(w, line.Render("└")+"  "+text)
}

// Fail ends the rail with what went wrong, in red.
func Fail(w io.Writer, text string) { End(w, red.Render("■ "+text)) }

// Done is End's text when it went well, the first words green: Done: …
func Done(head, rest string) string { return green.Render(head) + " " + rest }

func mark(w io.Writer, m, text string) {
	lipgloss.Fprintln(w, line.Render("│"))
	lipgloss.Fprintln(w, m+"  "+text)
}
