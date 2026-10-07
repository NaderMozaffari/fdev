package logview

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/config"
)

// Related logs are drawn as groups, with a dashed line between them (the
// look's Group): a burst of logs and the pause after it, or a run of one
// tag. A response stays with its request however long it took.
//
//	╌╌ +2.4s ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌

// groupPause is the pause that ends a burst of logs.
const groupPause = time.Second

// grouper decides, entry by entry, where a group starts.
type grouper struct {
	mode string
	prev *Entry          // the entry shown before, nil at the start
	open map[string]bool // the group's requests still waiting for a response
}

// grouping is the grouper for the current settings, nil when groups are off.
func (m *Model) grouping() *grouper {
	if m.o.Plain || m.on["raw"] || m.look.Group == config.GroupOff {
		return nil
	}
	return &grouper{mode: m.look.Group, open: map[string]bool{}}
}

// next takes the next shown entry and says whether a line goes before it,
// with the line's label.
func (g *grouper) next(e *Entry) (string, bool) {
	prev := g.prev
	g.prev = e
	// flutter's own lines and screen clears already set logs apart.
	if prev == nil || e.Kind == KindTool || prev.Kind == KindTool {
		g.open = map[string]bool{}
		g.track(e)
		return "", false
	}
	label, starts := "", false
	switch g.mode {
	case config.GroupTag:
		if k := groupTag(e); k != groupTag(prev) {
			label, starts = k, true
		}
	default:
		gap := e.Time.Sub(prev.Time)
		if gap >= groupPause && !(g.open[e.Text] && (e.Level == "response" || e.Level == "fail")) {
			label, starts = "+"+gapText(gap), true
		}
	}
	if starts {
		g.open = map[string]bool{}
	}
	g.track(e)
	return label, starts
}

func (g *grouper) track(e *Entry) {
	switch e.Level {
	case "request":
		g.open[e.Text] = true
	case "response", "fail":
		delete(g.open, e.Text)
	}
}

// groupTag is what a run of one tag shares: the tag, or the kind of log.
func groupTag(e *Entry) string {
	switch {
	case e.Category == "network":
		return "network"
	case e.Tag != "":
		return e.Tag
	case e.Kind == KindNative:
		return "native"
	}
	return "app"
}

func gapText(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return short(d)
}

// groupRule is the line between two groups, its label near the left.
func (m *Model) groupRule(label string) string {
	rule := lipgloss.NewStyle().Foreground(m.th.Subtle)
	text := lipgloss.NewStyle().Foreground(m.th.Muted).Render(" " + label + " ")
	if label == "" {
		text = ""
	}
	rest := max(m.width-2-lipgloss.Width(text), 0)
	return ansi.Truncate(rule.Render("╌╌")+text+rule.Render(strings.Repeat("╌", rest)), m.width, "")
}
