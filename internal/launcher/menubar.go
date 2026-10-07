package launcher

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/logview"
)

// What the menu shows is picked at start, a step at a time (see picker.go):
// the section (a target group, Recent or Saved logs), then the platform and
// the flavor among its targets. There is no "all": a target without a
// platform or flavor shows under every one, and a section with fewer than
// two platforms (or flavors) isn't narrowed by them.

const (
	recentTab = "Recent"
	logsTab   = "Saved logs"
)

// matches reports whether t shows for a platform and flavor; a target
// without one shows for all.
func (m *Model) matches(t *config.Target, platform, flavor string) bool {
	p := t.PlatformName()
	return (platform == "" || p == "" || p == platform) && (flavor == "" || t.Flavor == "" || t.Flavor == flavor)
}

func (m *Model) shown(t *config.Target) bool {
	return m.matches(t, m.activePlatform(), m.activeFlavor())
}

// sections is Recent (when there are recent runs) first, then every
// target group, then Saved logs when there are any.
func (m *Model) sections() []string {
	var out []string
	if len(m.allRecent()) > 0 {
		out = append(out, recentTab)
	}
	for _, t := range m.cfg.Targets() {
		if !contains(out, t.Group) {
			out = append(out, t.Group)
		}
	}
	if len(m.sessions) > 0 {
		out = append(out, logsTab)
	}
	return out
}

// sectionTargets is the targets a section holds, whatever their platform
// and flavor.
func (m *Model) sectionTargets(tab string) []*config.Target {
	var out []*config.Target
	add := func(t *config.Target) {
		for _, o := range out {
			if o == t {
				return
			}
		}
		if t != nil {
			out = append(out, t)
		}
	}
	switch tab {
	case recentTab:
		for _, r := range m.state.Recent {
			add(m.cfg.Target(r.Target))
		}
	case logsTab:
		for _, s := range m.sessions {
			add(m.cfg.Target(s.Target))
		}
	default:
		for _, t := range m.cfg.Targets() {
			if t.Group == tab {
				add(t)
			}
		}
	}
	return out
}

// platforms is the platforms of the section's targets.
func (m *Model) platforms() []string {
	return values(m.sectionTargets(m.tab), platformOf, platformOrder)
}

// flavors is the flavors of the section's targets on platform ("" for
// any), in the order the config has them.
func (m *Model) flavors(platform string) []string {
	var order []string
	for _, t := range m.cfg.Targets() {
		order = append(order, t.Flavor)
	}
	var targets []*config.Target
	for _, t := range m.sectionTargets(m.tab) {
		if platform == "" || t.PlatformName() == platform {
			targets = append(targets, t)
		}
	}
	return values(targets, flavorOf, order)
}

// activePlatform is the platform the section is narrowed to: none when it
// has fewer than two.
func (m *Model) activePlatform() string {
	if len(m.platforms()) > 1 {
		return m.menuPlatform
	}
	return ""
}

func (m *Model) activeFlavor() string {
	if len(m.flavors(m.activePlatform())) > 1 {
		return m.menuFlavor
	}
	return ""
}

// fixFilters makes the section, platform and flavor ones the project has:
// the last ones when they still exist, else the first.
func (m *Model) fixFilters() {
	m.tab = pickValue(m.sections(), m.tab)
	m.menuPlatform = pickValue(m.platforms(), m.menuPlatform)
	m.menuFlavor = pickValue(m.flavors(m.activePlatform()), m.menuFlavor)
}

func pickValue(options []string, current string) string {
	if len(options) == 0 || contains(options, current) {
		return current
	}
	return options[0]
}

// tabTargets is the targets of a group that show.
func (m *Model) tabTargets(tab string) []*config.Target {
	var out []*config.Target
	for _, t := range m.cfg.Targets() {
		if t.Group == tab && m.shown(t) {
			out = append(out, t)
		}
	}
	return out
}

// allRecent is the recent runs of targets the config still has.
func (m *Model) allRecent() []job {
	var jobs []job
	for _, r := range m.state.Recent {
		if t := m.cfg.Target(r.Target); t != nil {
			jobs = append(jobs, job{target: t, env: r.Env, note: r.Note, at: r.At})
		}
	}
	return jobs
}

// recentJobs is the recent runs that show.
func (m *Model) recentJobs() []job {
	var jobs []job
	for _, j := range m.allRecent() {
		if m.shown(j.target) {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// savedLogs is the saved sessions of targets that show (and of ones the
// config no longer has), starred first.
func (m *Model) savedLogs() []logview.Session {
	var out []logview.Session
	for _, s := range m.sessions {
		if t := m.cfg.Target(s.Target); t == nil || m.shown(t) {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) loadSessions() {
	m.sessions = logview.Sessions(m.cfg.Root, m.look().Dir)
}

var platformOrder = []string{"Android", "iOS", "Web", "macOS", "Windows", "Linux"}

// values is the platforms or flavors among targets, in a stable order.
func values(targets []*config.Target, of func(*config.Target) string, order []string) []string {
	have := map[string]bool{}
	var seen []string
	for _, t := range targets {
		if v := of(t); v != "" && !have[v] {
			have[v] = true
			seen = append(seen, v)
		}
	}
	var out []string
	for _, v := range order {
		if have[v] {
			out = append(out, v)
			delete(have, v)
		}
	}
	for _, v := range seen {
		if have[v] {
			out = append(out, v)
		}
	}
	return out
}

func platformOf(t *config.Target) string { return t.PlatformName() }
func flavorOf(t *config.Target) string   { return t.Flavor }

// flavorColor is a flavor's color: its config, or one by its name that
// matches the usual icon bands (dev yellow, test blue, prod red).
func (m *Model) flavorColor(name string) color.Color {
	if f := m.cfg.Flavors[name]; f != nil && f.Color != "" {
		return lipgloss.Color(f.Color)
	}
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "dev"):
		return lipgloss.Color("#E8A400")
	case strings.Contains(n, "stag"), strings.Contains(n, "test"), strings.Contains(n, "qa"), strings.Contains(n, "beta"):
		return lipgloss.Color("#2196F3")
	case strings.Contains(n, "prod"), strings.Contains(n, "release"), strings.Contains(n, "live"):
		return lipgloss.Color("#E53935")
	}
	return lipgloss.Color("#7D56F4")
}

func platformColor(name string) color.Color {
	switch name {
	case "Android":
		return lipgloss.Color("#2DB36F")
	case "iOS":
		return lipgloss.Color("#8E8EF0")
	case "Web":
		return lipgloss.Color("#4285F4")
	case "Windows":
		return lipgloss.Color("#0078D4")
	}
	return lipgloss.Color("#9A9AA0")
}

func sectionColor(name string) color.Color {
	switch strings.ToLower(name) {
	case "run":
		return lipgloss.Color("#02BF87")
	case "build":
		return lipgloss.Color("#7571F9")
	case "tools", "other":
		return lipgloss.Color("#B48EF7")
	case "recent":
		return lipgloss.Color("#F780E2")
	case strings.ToLower(logsTab):
		return lipgloss.Color("#E8B730")
	}
	return lipgloss.Color("#9A9AA0")
}

func (m *Model) flavorFact(flavor, key string) string {
	if f := m.cfg.Flavors[flavor]; f != nil {
		for _, fact := range f.Info {
			if fact.Key == key {
				return fact.Value
			}
		}
	}
	return ""
}

func (m *Model) saveMenu() {
	m.state.Menu = map[string]string{"tab": m.tab, "platform": m.menuPlatform, "flavor": m.menuFlavor}
	m.state.Save()
}
