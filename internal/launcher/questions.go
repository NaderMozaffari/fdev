package launcher

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/devices"
	"github.com/NaderMozaffari/fdev/internal/sprite"
)

// A target's questions (asks in fdev.yaml, and the devices) open in the
// picker too: icons, a card, ← and →. Its stepper is the target and its
// questions, answered ones first.

// optionColors is for options without a color.
var optionColors = []color.Color{
	lipgloss.Color("#7571F9"), lipgloss.Color("#02BF87"), lipgloss.Color("#F780E2"),
	lipgloss.Color("#E8B730"), lipgloss.Color("#4285F4"), lipgloss.Color("#ED567A"),
}

// openQuestion shows choices in the picker, from the one with value
// selected.
func (m *Model) openQuestion(title, hint string, choices []choice, selected string) tea.Cmd {
	m.asking, m.qTitle, m.qHint, m.qChoices = true, title, hint, choices
	m.screen, m.pickIndex = screenPick, 0
	for i, c := range choices {
		if c.value == selected {
			m.pickIndex = i
		}
	}
	m.enterStart, m.animStart = time.Now(), time.Now()
	return animTick()
}

// askQuestion is one of fdev.yaml's asks.
func (m *Model) askQuestion(name string) tea.Cmd {
	j, ask := m.job, m.cfg.Asks[name]
	last, answered := m.state.Answers[name]
	var choices []choice
	for i, o := range ask.Options {
		c := choice{value: o.Value, title: o.Label, desc: o.Desc, sprite: "option",
			color: optionColors[i%len(optionColors)]}
		if o.Icon != "" {
			c.sprite = sprite.For(o.Icon)
		}
		if o.Color != "" {
			c.color = lipgloss.Color(o.Color)
		}
		sets := ask.Env + "=" + o.Value
		if o.Value == "" {
			sets = ask.Env + " unset"
		}
		if c.desc == "" {
			c.desc = sets
		}
		c.meta = sets
		if answered && o.Value == last {
			c.meta += " · last time"
		}
		c.facts = [][2]string{{"Sets", sets}, {"For", j.target.Name}, {"Runs", j.target.Run}}
		choices = append(choices, c)
	}
	return m.openQuestion(m.askTitle(name), "for "+j.target.Name, choices, last)
}

// deviceQuestion lists the devices found, or what to do when there are none.
func (m *Model) deviceQuestion(list []devices.Device, err error) tea.Cmd {
	j := m.job
	last := m.state.Answers[j.asks[0]]
	var choices []choice
	for _, d := range list {
		d := d
		platform, sp := platformOfDevice(d.TargetPlatform)
		kind := "phone"
		switch {
		case d.Emulator && platform == "iOS":
			kind = "simulator"
		case d.Emulator:
			kind = "emulator"
		case sp != "phone":
			kind = strings.ToLower(platform)
		}
		facts := [][2]string{{"Name", d.Name}, {"ID", d.ID}, {"Platform", d.TargetPlatform}}
		if d.SDK != "" {
			facts = append(facts, [2]string{"SDK", d.SDK})
		}
		facts = append(facts, [2]string{"Kind", kind}, [2]string{"For", j.target.Name})
		meta := kind
		if d.ID == last {
			meta += " · last time"
		}
		choices = append(choices, choice{value: d.ID, title: d.Name, desc: d.Desc(), meta: meta,
			sprite: sp, color: platformColor(platform), facts: facts, device: &deviceChoice{id: d.ID, name: d.Name}})
	}
	title, hint := devicesWord(m.platform, "device"), "for "+j.target.Name
	if len(choices) == 0 {
		title = "No " + devicesWord(m.platform, "device") + " found"
		hint = "start an emulator or plug in a phone"
		if err != nil {
			hint = err.Error()
		}
		choices = append(choices, choice{value: "retry", title: "Look again", desc: "list the devices again",
			sprite: "refresh", color: m.th.Indigo, device: &deviceChoice{retry: true}})
		if m.platform == "ios" && runtime.GOOS == "darwin" {
			choices = append(choices, choice{value: "sim", title: "Open the Simulator", desc: "and look again",
				sprite: "ios", color: platformColor("iOS"), device: &deviceChoice{sim: true}})
		}
		choices = append(choices, choice{value: "any", title: "Run anyway", desc: "let flutter pick",
			sprite: "auto", color: m.th.Fuchsia, device: &deviceChoice{}})
	} else if j.want != "" {
		choices = append(choices, choice{value: "retry", title: "Look again", desc: "after connecting " + wantName(j),
			sprite: "refresh", color: m.th.Indigo, device: &deviceChoice{retry: true}})
	}
	if j.want != "" { // a recent run's device that is gone
		title = wantName(j) + " is not connected"
		hint = "pick another " + devicesWord(m.platform, "device")
		if len(list) == 0 {
			hint = "connect it and look again"
		}
	}
	return m.openQuestion(title, hint, choices, last)
}

func wantName(j *job) string {
	if j.wantName != "" {
		return j.wantName
	}
	return j.want
}

// platformOfDevice is the platform and icon of a flutter target platform
// (android-arm64, ios, web-javascript, darwin, ...).
func platformOfDevice(target string) (platform, sprite string) {
	switch {
	case strings.HasPrefix(target, "android"):
		return "Android", "phone"
	case strings.HasPrefix(target, "ios"):
		return "iOS", "phone"
	case strings.HasPrefix(target, "web"):
		return "Web", "web"
	case target == "darwin":
		return "macOS", "desktop"
	case strings.HasPrefix(target, "windows"):
		return "Windows", "windows"
	}
	return "Linux", "desktop"
}

func (m *Model) answerChoice(c choice) tea.Cmd {
	if c.device != nil {
		return m.answerDevice(*c.device, false)
	}
	return m.answerAsk(c.value)
}

func (m *Model) askTitle(name string) string {
	if p, ok := config.DeviceAsk(name); ok {
		return devicesWord(p, "device")
	}
	if ask := m.cfg.Asks[name]; ask != nil && ask.Title != "" {
		return ask.Title
	}
	return name
}

// answerLabel is what was answered to a question of the job.
func (m *Model) answerLabel(name string) string {
	j := m.job
	if _, ok := config.DeviceAsk(name); ok {
		if j.note != "" {
			return j.note
		}
		return "flutter picks"
	}
	ask := m.cfg.Asks[name]
	for _, o := range ask.Options {
		if o.Value == j.env[ask.Env] {
			return o.Label
		}
	}
	return j.env[ask.Env]
}

// goToAsk goes back to the job's i-th answered question.
func (m *Model) goToAsk(i int) tea.Cmd {
	j := m.job
	if j == nil || i >= len(j.asked) {
		return nil
	}
	j.asks = append(append([]string(nil), j.asked[i:]...), j.asks...)
	j.asked = j.asked[:i]
	return m.nextAsk()
}

// jobStepper is the target, then its questions: answered ones (click to
// change them), the one asked, the ones to come.
func (m *Model) jobStepper(y int) string {
	th := m.th
	j := m.job
	if j == nil {
		return ""
	}
	c := sectionColor(j.target.Group)
	if j.target.Flavor != "" {
		c = m.flavorColor(j.target.Flavor)
	}
	line := "  " + lipgloss.NewStyle().Foreground(th.Cream).Background(c).Bold(true).Padding(0, 1).
		Render(tabSign(j.target.Group)+" "+j.target.Name)
	x := lipgloss.Width(line)
	add := func(text string) {
		sep := m.fg(th.Subtle).Render(" ─── ")
		line += sep + text
		x += lipgloss.Width(sep) + lipgloss.Width(text)
	}
	for i, name := range j.asked {
		if j.auto[name] {
			continue
		}
		st := lipgloss.NewStyle().Foreground(th.Cream).Background(th.Purple).Padding(0, 1)
		if m.hover == fmt.Sprintf("ask:%d", i) {
			st = st.Underline(true)
		}
		text := st.Render("✓ " + m.answerLabel(name))
		sepW := lipgloss.Width(m.fg(th.Subtle).Render(" ─── "))
		m.addZone(zone{kind: "ask", value: fmt.Sprint(i), index: i, x0: x + sepW, y0: y, x1: x + sepW + lipgloss.Width(text), y1: y + 1})
		add(text)
	}
	for i, name := range j.asks {
		if i == 0 {
			add(m.fg(th.Pink).Bold(true).Render("● " + m.askTitle(name)))
		} else {
			add(m.fg(th.Muted).Render("○ " + m.askTitle(name)))
		}
	}
	return line
}
