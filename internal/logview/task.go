package logview

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// A task is a step flutter shows on a line of its own while it works, like
// "Running Gradle task 'assembleDevDebug'..." or "Installing build/...apk...",
// and finishes with the time it took ("36.4s"). fdev shows a progress bar
// for it, measured against how long the same step took last time.
type task struct {
	name     string
	started  time.Time
	expected time.Duration // 0 the first time
}

var (
	taskRunning = regexp.MustCompile(`^(\S.*?)\.\.\.`)
	taskDone    = regexp.MustCompile(`^(\S.*?)\.\.\.\s+(\d+(?:\.\d+)?)(ms|s)$`)
)

// taskName is the step a partial line is running, if it is one.
func taskName(partial string) string {
	plain := strings.TrimSpace(ansi.Strip(overwrite(partial)))
	if m := taskRunning.FindStringSubmatch(plain); m != nil && !taskDone.MatchString(plain) {
		return m[1]
	}
	return ""
}

// finishedTask is the step a complete line reports and how long it took.
func finishedTask(line string) (string, time.Duration, bool) {
	plain := strings.TrimSpace(ansi.Strip(line))
	m := taskDone.FindStringSubmatch(plain)
	if m == nil {
		return "", 0, false
	}
	v, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return "", 0, false
	}
	unit := time.Second
	if m[3] == "ms" {
		unit = time.Millisecond
	}
	return m[1], time.Duration(v * float64(unit)), true
}

// trackTask starts, keeps or ends the task of the unfinished last line.
func (m *Model) trackTask() {
	name := taskName(m.partial)
	switch {
	case name == "":
		m.task = nil
	case m.task == nil || m.task.name != name:
		m.task = &task{name: name, started: time.Now()}
		if secs, ok := m.o.Durations[name]; ok {
			m.task.expected = time.Duration(secs * float64(time.Second))
		}
	}
}

// taskFinished remembers how long a step took, for its next progress bar.
func (m *Model) taskFinished(line string) {
	name, took, ok := finishedTask(line)
	if m.task != nil && m.task.name == name {
		m.task = nil
	}
	// Steps under a second are over before a bar would help.
	if !ok || m.o.Durations == nil || took < time.Second {
		return
	}
	secs := took.Seconds()
	if old, ok := m.o.Durations[name]; ok {
		secs = 0.6*secs + 0.4*old // follow change, but not every outlier
	}
	m.o.Durations[name] = math.Round(secs*10) / 10
}

// progress is how far along the task is, 0 to 1, measured against an
// earlier run; false when there is none to measure against.
func (t *task) progress() (float64, bool) {
	if t.expected <= 0 {
		return 0, false
	}
	return min(time.Since(t.started).Seconds()/t.expected.Seconds(), 0.99), true
}

// taskRow is the task as a row: spinner, name, bar, times. Without an
// earlier time the bar can't fill; it is a loading bar instead.
func (m *Model) taskRow() string {
	t := m.task
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	elapsed := time.Since(t.started).Seconds()
	pct, known := t.progress()
	times := fmt.Sprintf("%.1fs · first time", elapsed)
	if known {
		times = fmt.Sprintf("%3.0f%%  %.1fs / ~%.0fs", pct*100, elapsed, t.expected.Seconds())
	}

	barW := min(max(m.width/3, 12), 40)
	nameW := max(m.width-barW-lipgloss.Width(times)-8, 10)
	name := ansi.Truncate(t.name, nameW, "…")
	bar := loadingBar(barW, elapsed)
	if known {
		m.bar.SetWidth(barW)
		bar = m.bar.ViewAs(pct)
	}
	return m.spin.View() + lipgloss.NewStyle().Bold(true).Render(name) + "  " + bar + " " + muted.Render(times)
}

// loadingBar is a band sweeping back and forth along the track, in the
// progress bar's colors, its pink edge leading.
func loadingBar(width int, elapsed float64) string {
	const sweep = 1.8 // seconds from one end to the other
	band := max(width/4, 3)
	pos := math.Mod(elapsed/sweep, 2)
	forward := pos < 1
	if !forward {
		pos = 2 - pos
	}
	pos = pos * pos * (3 - 2*pos) // slow at the ends
	start := int(math.Round(pos * float64(width-band)))

	colors := lipgloss.Blend1D(band, barStart, barEnd)
	if !forward {
		slices.Reverse(colors)
	}
	empty := lipgloss.NewStyle().Foreground(barEmpty)
	var b strings.Builder
	b.WriteString(empty.Render(strings.Repeat(string(progress.DefaultEmptyCharBlock), start)))
	for _, c := range colors {
		b.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(progress.DefaultFullCharFullBlock)))
	}
	b.WriteString(empty.Render(strings.Repeat(string(progress.DefaultEmptyCharBlock), width-start-band)))
	return b.String()
}

// TerminalProgress is the task for the terminal's own progress bar (in the
// tab or dock, where the terminal supports it).
func (m *Model) TerminalProgress() *tea.ProgressBar {
	if m.task == nil || m.exited {
		return nil
	}
	pct, known := m.task.progress()
	if !known {
		return tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
	}
	return tea.NewProgressBar(tea.ProgressBarDefault, int(pct*100))
}

// The progress bar's colors (bubbles' default blend), for the loading bar.
var (
	barStart = lipgloss.Color("#5A56E0")
	barEnd   = lipgloss.Color("#EE6FF8")
	barEmpty = lipgloss.Color("#606060")
)

func newBar() progress.Model {
	return progress.New(progress.WithColors(barStart, barEnd), progress.WithoutPercentage())
}
