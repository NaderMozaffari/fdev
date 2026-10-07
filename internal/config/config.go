// Package config finds the project and the targets fdev offers: from
// fdev.yaml when the project has one, otherwise worked out from the project
// itself at every start (see Discover), so nothing goes stale or in git.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileNames are the config files fdev looks for, in order.
var FileNames = []string{"fdev.yaml", ".fdev.yaml"}

// DartDefine turns on the structured log records (see docs/PROTOCOL.md).
const DartDefine = "--dart-define=FDEV_LOGS=true"

type Config struct {
	Root   string            `yaml:"-"`
	Source string            `yaml:"-"` // what the targets came from, for the banner
	Title  string            `yaml:"title"`
	Env    map[string]string `yaml:"env"`
	Groups []Group           `yaml:"groups"`
	Asks   map[string]*Ask   `yaml:"asks"`
	Editor string            `yaml:"editor"` // overrides the editor command, see editor.Command
	// Logs is the default look of the log viewer; each user's changes in
	// its settings (key l) are kept on top of it.
	Logs Look `yaml:"logs"`
	// Flavors describe the app's flavors for the menu's details panel.
	Flavors map[string]*Flavor `yaml:"flavors"`

	// Version and Build are the app's, from pubspec.yaml.
	Version string `yaml:"-"`
	Build   string `yaml:"-"`
	// Warning is something fdev couldn't work out.
	Warning string `yaml:"-"`
}

// Flavor is what the menu shows about a flavor: its icon and some facts.
type Flavor struct {
	Icon string `yaml:"icon"` // a PNG, relative to the project; found when empty
	Info Info   `yaml:"info"` // e.g. App, ID, Backend, in the order written
	// Color of the flavor's chip in the menu; by its name when empty
	// (dev yellow, staging/test blue, prod red).
	Color string `yaml:"color"`
}

// Info is an ordered list of facts, written as a YAML mapping.
type Info []Fact

type Fact struct{ Key, Value string }

func (i *Info) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: info must be a mapping", node.Line)
	}
	for j := 0; j+1 < len(node.Content); j += 2 {
		*i = append(*i, Fact{node.Content[j].Value, node.Content[j+1].Value})
	}
	return nil
}

// Look is how the log viewer draws and keeps logs.
type Look struct {
	Layout  string `yaml:"layout" json:"layout"`   // lines, columns or table
	Labels  string `yaml:"labels" json:"labels"`   // text or badge
	Spacing int    `yaml:"spacing" json:"spacing"` // blank lines between logs
	Time    string `yaml:"time" json:"time"`       // clock or millis
	Save    bool   `yaml:"save" json:"save"`       // save every session under Dir
	Dir     string `yaml:"dir" json:"dir"`         // relative to the project; git-ignored
	// Group draws a line between groups of related logs: pause (a burst of
	// logs, a pause before the next), tag (a run of one tag) or off.
	Group string `yaml:"group" json:"group"`
	// Off is the key combinations turned off, for when they get hit by
	// mistake: ctrl+c, ctrl+l, ...
	Off []string `yaml:"off" json:"off,omitempty"`
	// RTL is who lays out Persian and Arabic text: auto, fdev (joins the
	// letters, right to left) or terminal (it does it itself).
	RTL string `yaml:"rtl" json:"rtl,omitempty"`
	// Values are keys, besides tokens, whose values in network headers and
	// bodies the viewer keeps at hand (k): userId, deviceId, ...
	Values []string `yaml:"values" json:"-"`
}

// Layouts, label styles and time formats a Look can have.
const (
	LayoutLines   = "lines"
	LayoutColumns = "columns"
	LayoutTable   = "table"
	LabelsText    = "text"
	LabelsBadge   = "badge"
	TimeClock     = "clock"
	TimeMillis    = "millis"
	GroupPause    = "pause"
	GroupTag      = "tag"
	GroupOff      = "off"
)

// WithDefaults fills what l leaves empty.
func (l Look) WithDefaults() Look {
	if l.Layout != LayoutColumns && l.Layout != LayoutTable {
		l.Layout = LayoutLines
	}
	if l.Labels != LabelsBadge {
		l.Labels = LabelsText
	}
	if l.Time != TimeMillis {
		l.Time = TimeClock
	}
	if l.Group != GroupTag && l.Group != GroupOff {
		l.Group = GroupPause
	}
	if l.RTL != "fdev" && l.RTL != "terminal" {
		l.RTL = "auto"
	}
	l.Spacing = min(max(l.Spacing, 0), 2)
	if l.Dir == "" {
		l.Dir = ".fdev/logs"
	}
	return l
}

// KeyOff reports whether a key combination (as Bubble Tea names it,
// "ctrl+l") is turned off.
func (l Look) KeyOff(key string) bool {
	for _, k := range l.Off {
		if k == key {
			return true
		}
	}
	return false
}

// UI is how fdev's screens look, for every project: the theme and how big
// menus and their items are.
type UI struct {
	Theme string `json:"theme,omitempty"` // auto (the terminal's dark or light) or a theme's name
	Size  string `json:"size,omitempty"`  // auto, small, medium or large
}

// Sizes a UI can have. Auto picks one by the window: the biggest that fits.
const (
	SizeAuto   = "auto"
	SizeSmall  = "small"
	SizeMedium = "medium"
	SizeLarge  = "large"
)

// WithDefaults fills what u leaves empty.
func (u UI) WithDefaults() UI {
	if u.Theme == "" {
		u.Theme = "auto"
	}
	if u.Size != SizeSmall && u.Size != SizeMedium && u.Size != SizeLarge {
		u.Size = SizeAuto
	}
	return u
}

type Group struct {
	Name    string    `yaml:"name"`
	Targets []*Target `yaml:"targets"`
}

type Target struct {
	Name string   `yaml:"name"`
	Desc string   `yaml:"desc"`
	Run  string   `yaml:"run"`  // run by `sh -c`; asks arrive as environment variables
	Logs bool     `yaml:"logs"` // a `flutter run`: show it in the log viewer
	Ask  []string `yaml:"ask"`  // names from Config.Asks, or device, device:android, device:ios
	OS   []string `yaml:"os"`   // only on these GOOS values (darwin, linux, windows)
	// Flavor links the target to a flavor (its icon and facts); Platform
	// names what it runs on. Both are shown in the menu; Platform is
	// guessed from the asks and name when empty.
	Flavor   string `yaml:"flavor"`
	Platform string `yaml:"platform"`
	Group    string `yaml:"-"`
	// Builtin is one of fdev's own tools (WifiTarget), run in the terminal
	// instead of the log viewer.
	Builtin string `yaml:"-"`
}

// WifiTarget is fdev wifi in the Tools menu, for projects with an Android app.
const WifiTarget = "wifi-debug"

// PlatformName is Target.Platform, or a guess: device:ios is iOS, a name
// with web or macos is that, a device:android ask is Android.
func (t *Target) PlatformName() string {
	if t.Platform != "" {
		return t.Platform
	}
	for _, a := range t.Ask {
		switch a {
		case "device:ios":
			return "iOS"
		case "device:android":
			return "Android"
		}
	}
	name := strings.ToLower(t.Name + " " + t.Run)
	switch {
	case strings.Contains(name, "web"):
		return "Web"
	case strings.Contains(name, "macos"):
		return "macOS"
	case strings.Contains(name, "windows"):
		return "Windows"
	case strings.Contains(name, "ios") || strings.Contains(name, "ipa"):
		return "iOS"
	case strings.Contains(name, "apk") || strings.Contains(name, "aab") || strings.Contains(name, "android"):
		return "Android"
	}
	return ""
}

// FlavorIcon is the icon of a flavor: Flavors[name].Icon, or the launcher
// icon of android/app/src/<name> (then main), or the iOS AppIcon-<name>.
func (c *Config) FlavorIcon(name string) string {
	if name == "" {
		return ""
	}
	if f := c.Flavors[name]; f != nil && f.Icon != "" {
		return filepath.Join(c.Root, f.Icon)
	}
	for _, dir := range []string{name, "main"} {
		for _, dpi := range []string{"xxxhdpi", "xxhdpi", "xhdpi", "hdpi"} {
			for _, file := range []string{"launcher_icon.png", "ic_launcher.png"} {
				path := filepath.Join(c.Root, "android/app/src", dir, "res", "mipmap-"+dpi, file)
				if exists(path) {
					return path
				}
			}
		}
	}
	for _, set := range []string{"AppIcon-" + name, "AppIcon"} {
		path := filepath.Join(c.Root, "ios/Runner/Assets.xcassets", set+".appiconset", "Icon-App-1024x1024@1x.png")
		if exists(path) {
			return path
		}
	}
	return ""
}

var pubspecVersion = regexp.MustCompile(`(?m)^version:\s*["']?([^+\s"']+)(?:\+(\S+?))?["']?\s*$`)

func (c *Config) readVersion() {
	data, err := os.ReadFile(filepath.Join(c.Root, "pubspec.yaml"))
	if err != nil {
		return
	}
	if m := pubspecVersion.FindSubmatch(data); m != nil {
		c.Version, c.Build = string(m[1]), string(m[2])
	}
}

// AppVersion is the app's version as the headers show it, "" when
// pubspec.yaml has none: v2.1.39 · build 439.
func (c *Config) AppVersion() string {
	if c.Version == "" {
		return ""
	}
	if c.Build != "" {
		return "v" + c.Version + " · build " + c.Build
	}
	return "v" + c.Version
}

// ShortVersion is AppVersion where room is short: v2.1.39 (439).
func (c *Config) ShortVersion() string {
	if c.Version == "" || c.Build == "" {
		return c.AppVersion()
	}
	return "v" + c.Version + " (" + c.Build + ")"
}

type Ask struct {
	Title   string   `yaml:"title"`
	Env     string   `yaml:"env"`
	Options []Option `yaml:"options"`
}

type Option struct {
	Label string `yaml:"label"`
	Value string `yaml:"value"`
	Desc  string `yaml:"desc"`
	// Icon is one of fdev's icons for it (play, bag, store, phone, star,
	// android, ios, web, ...), and Color its color, e.g. "#4285F4".
	Icon  string `yaml:"icon"`
	Color string `yaml:"color"`
}

// DeviceAsk reports whether name is a built-in device question, and for
// which platform ("" for any).
func DeviceAsk(name string) (platform string, ok bool) {
	if name == "device" {
		return "", true
	}
	if p, found := strings.CutPrefix(name, "device:"); found {
		return p, true
	}
	return "", false
}

// DeviceEnv is the variable a device question sets.
const DeviceEnv = "DEVICE"

// Load finds the project around dir and its targets: its fdev.yaml, or
// what Discover works out (also written to .fdev/fdev.yaml, to look at).
func Load(dir string) (*Config, error) {
	root, file := findRoot(dir)
	if root == "" {
		return nil, errors.New("no fdev.yaml, Makefile or pubspec.yaml here or in a parent directory")
	}
	var cfg *Config
	if file != "" {
		var err error
		if cfg, err = fromFile(filepath.Join(root, file)); err != nil {
			return nil, err
		}
		cfg.Source = file
	} else {
		cfg = Discover(root)
		writeSnapshot(cfg)
		cfg.Source = "auto · " + cfg.Source
	}
	cfg.Root = root
	if cfg.Title == "" {
		cfg.Title = filepath.Base(root)
	}
	cfg.readVersion()
	cfg.addWifi()
	return cfg, cfg.check()
}

// addWifi offers fdev wifi under Tools when the project has an Android app
// and no target of that name.
func (c *Config) addWifi() {
	if !exists(filepath.Join(c.Root, "android")) || c.Target(WifiTarget) != nil {
		return
	}
	t := &Target{Name: WifiTarget, Desc: "Debug an Android phone over Wi-Fi: pair with a QR code or pairing code", Run: "fdev wifi", Builtin: WifiTarget}
	for i := range c.Groups {
		if c.Groups[i].Name == "Tools" {
			c.Groups[i].Targets = append(c.Groups[i].Targets, t)
			return
		}
	}
	c.Groups = append(c.Groups, Group{Name: "Tools", Targets: []*Target{t}})
}

// Targets lists the targets for this OS, in menu order.
func (c *Config) Targets() []*Target {
	var out []*Target
	for _, g := range c.Groups {
		for _, t := range g.Targets {
			if len(t.OS) > 0 && !contains(t.OS, runtime.GOOS) {
				continue
			}
			t.Group = g.Name
			out = append(out, t)
		}
	}
	return out
}

func (c *Config) Target(name string) *Target {
	for _, t := range c.Targets() {
		if t.Name == name {
			return t
		}
	}
	return nil
}

func (c *Config) check() error {
	for _, t := range c.Targets() {
		if t.Name == "" || t.Run == "" {
			return fmt.Errorf("%s: every target needs a name and run", c.Source)
		}
		for _, a := range t.Ask {
			if _, ok := DeviceAsk(a); ok {
				continue
			}
			ask := c.Asks[a]
			if ask == nil || ask.Env == "" || len(ask.Options) == 0 {
				return fmt.Errorf("%s: target %q asks %q, which needs env and options under asks", c.Source, t.Name, a)
			}
		}
	}
	return nil
}

func findRoot(dir string) (root, file string) {
	fallback := ""
	for d := dir; ; d = filepath.Dir(d) {
		for _, name := range FileNames {
			if exists(filepath.Join(d, name)) {
				return d, name
			}
		}
		if fallback == "" && (exists(filepath.Join(d, "Makefile")) || exists(filepath.Join(d, "pubspec.yaml"))) {
			fallback = d
		}
		if parent := filepath.Dir(d); parent == d {
			return fallback, ""
		}
	}
}

func fromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return &cfg, nil
}

func flutterDefaults() *Config {
	run := `flutter run ${DEVICE:+-d "$DEVICE"} ` + DartDefine
	return &Config{Groups: []Group{
		{Name: "Run", Targets: []*Target{
			{Name: "run", Desc: "flutter run", Run: run, Logs: true, Ask: []string{"device"}},
		}},
		{Name: "Build", Targets: []*Target{
			{Name: "build apk", Desc: "flutter build apk", Run: "flutter build apk"},
			{Name: "build appbundle", Desc: "flutter build appbundle", Run: "flutter build appbundle"},
			{Name: "build ios", Desc: "flutter build ios", Run: "flutter build ios", OS: []string{"darwin"}},
			{Name: "build web", Desc: "flutter build web", Run: "flutter build web"},
		}},
		{Name: "Other", Targets: []*Target{
			{Name: "pub get", Desc: "flutter pub get", Run: "flutter pub get"},
			{Name: "test", Desc: "flutter test", Run: "flutter test"},
			{Name: "analyze", Desc: "flutter analyze", Run: "flutter analyze"},
			{Name: "build_runner", Desc: "dart run build_runner build -d", Run: "dart run build_runner build --delete-conflicting-outputs"},
			{Name: "devices", Desc: "flutter devices", Run: "flutter devices"},
			{Name: "clean", Desc: "flutter clean", Run: "flutter clean"},
		}},
	}}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
