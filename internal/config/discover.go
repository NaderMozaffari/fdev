package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Discover works out a project's config from what it already has, every
// time fdev starts, so it can't go stale and nothing goes in git:
//
//   - targets: the Makefile's rules, described by `make help`, a `## text`
//     or the comment above; the recipe, expanded like make would, tells the
//     group (flutter run/build), flavor (--flavor, -t lib/main_<flavor>.dart),
//     platform (-d chrome, build apk, variables named ANDROID_/IOS_) and the
//     questions: DEVICE for a device, and any other `NAME ?=` the recipe
//     uses, with the values the Makefile tests it against;
//   - flavors: android/app/build.gradle(.kts) productFlavors (ids, labels
//     in strings.xml), ios/Runner.xcodeproj (bundle ids, display names) and
//     a Dart enum in lib/ with a value per flavor (its strings, like the
//     backend, and its doc comment);
//   - without a Makefile, plain flutter commands, one per flavor.
func Discover(root string) *Config {
	cfg := &Config{Root: root, Logs: Look{Layout: LayoutTable, Labels: LabelsBadge, Save: true}}
	fl := discoverFlavors(root)
	cfg.Flavors = fl.info
	cfg.Title = fl.title
	if cfg.Title == "" {
		cfg.Title = pubspecName(root)
	}
	if data, err := os.ReadFile(filepath.Join(root, "Makefile")); err == nil {
		mk := parseMakefile(string(data))
		cfg.Groups, cfg.Asks = makeTargets(root, mk, fl)
		cfg.Source = "Makefile"
	}
	if len(cfg.Groups) == 0 && IsFlutter(root) {
		def := flutterDefaults()
		cfg.Groups = def.Groups
		for i := len(fl.names) - 1; i >= 0; i-- {
			f := fl.names[i]
			cfg.Groups[0].Targets = append([]*Target{{
				Name: "run " + f, Desc: "flutter run --flavor " + f, Flavor: f, Logs: true, Ask: []string{"device"},
				Run: `flutter run --flavor ` + f + ` ${DEVICE:+-d "$DEVICE"} $FDEV_DART_DEFINES`,
			}}, cfg.Groups[0].Targets...)
		}
		cfg.Source = "flutter"
	}
	return cfg
}

// ── Flavors ──────────────────────────────────────────────────────────────────

type flavors struct {
	names    []string           // the base flavors, in build.gradle order
	variants map[string]string  // e.g. devMyket → dev
	info     map[string]*Flavor // what the menu shows about each
	title    string
}

func discoverFlavors(root string) flavors {
	fl := flavors{variants: map[string]string{}, info: map[string]*Flavor{}}
	gradle := readFirst(root, "android/app/build.gradle", "android/app/build.gradle.kts")
	baseID := ""
	if m := regexp.MustCompile(`applicationId\s*=?\s*["']([^"']+)["']`).FindStringSubmatch(gradle); m != nil {
		baseID = m[1]
	}
	android := gradleFlavors(gradle)
	var all []string
	for _, f := range android {
		all = append(all, f.name)
	}
	for _, f := range all {
		if base := baseOf(f, all); base != "" {
			fl.variants[f] = base
		} else {
			fl.names = append(fl.names, f)
		}
	}
	if len(fl.names) == 0 { // no Android flavors: the entry points (lib/main_dev.dart)
		matches, _ := filepath.Glob(filepath.Join(root, "lib", "main_*.dart"))
		for _, p := range matches {
			fl.names = append(fl.names, strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "main_"), ".dart"))
		}
	}
	ios := iosConfigs(readFirst(root, "ios/Runner.xcodeproj/project.pbxproj"))
	dart := dartFlavorFacts(root, fl.names)
	fl.title = androidLabel(root, "main")

	for _, name := range fl.names {
		var info Info
		label := androidLabel(root, name)
		if label == "" {
			label = fl.title
		}
		if label != "" {
			info = append(info, Fact{"App", label})
		}
		ic := ios["Release-"+name]
		if ic.id == "" {
			ic = ios["Debug-"+name]
		}
		if ic.name != "" && ic.name != label {
			info = append(info, Fact{"iOS app", ic.name})
		}
		androidID := ""
		if baseID != "" {
			androidID = baseID
			for _, f := range android {
				if f.name == name {
					if f.id != "" {
						androidID = f.id
					}
					androidID += f.suffix
				}
			}
		}
		switch {
		case androidID != "" && androidID == ic.id:
			info = append(info, Fact{"ID", androidID})
		default:
			if androidID != "" {
				info = append(info, Fact{"Android ID", androidID})
			}
			if ic.id != "" {
				info = append(info, Fact{"iOS ID", ic.id})
			}
		}
		info = append(info, dart[name]...)
		fl.info[name] = &Flavor{Info: info}
	}
	return fl
}

// builds is the Android flavors a variant suffix ("myket"; "" for none)
// makes, when there are variants: devMyket · prodMyket.
func (fl flavors) builds(suffix string) string {
	if len(fl.variants) == 0 {
		return ""
	}
	var out []string
	for _, base := range fl.names {
		if suffix == "" {
			out = append(out, base)
			continue
		}
		for variant, b := range fl.variants {
			if b == base && strings.EqualFold(variant[len(base):], suffix) {
				out = append(out, variant)
			}
		}
	}
	return strings.Join(out, " · ")
}

// baseOf is the flavor f is a variant of (devMyket of dev), or "".
func baseOf(f string, all []string) string {
	best := ""
	for _, b := range all {
		if len(b) < len(f) && strings.HasPrefix(f, b) && unicode.IsUpper(rune(f[len(b)])) && len(b) > len(best) {
			best = b
		}
	}
	return best
}

type gradleFlavor struct{ name, id, suffix string }

var (
	gradleEntry  = regexp.MustCompile(`(?m)(?:create\(\s*"(\w+)"\s*\)|register\(\s*"(\w+)"\s*\)|^\s*(\w+))\s*\{`)
	gradleSuffix = regexp.MustCompile(`applicationIdSuffix\s*=?\s*["']([^"']*)["']`)
	gradleID     = regexp.MustCompile(`applicationId\s*=?\s*["']([^"']+)["']`)
)

// gradleFlavors reads productFlavors { name { ... } ... }.
func gradleFlavors(gradle string) []gradleFlavor {
	start := strings.Index(gradle, "productFlavors")
	if start < 0 {
		return nil
	}
	open := strings.IndexByte(gradle[start:], '{')
	if open < 0 {
		return nil
	}
	open += start
	end := matchParen(gradle, open)
	if end < 0 {
		return nil
	}
	body := gradle[open+1 : end]
	var out []gradleFlavor
	for i := 0; i < len(body); {
		loc := gradleEntry.FindStringSubmatchIndex(body[i:])
		if loc == nil {
			break
		}
		name := ""
		for g := 1; g <= 3; g++ {
			if loc[2*g] >= 0 {
				name = body[i+loc[2*g] : i+loc[2*g+1]]
			}
		}
		brace := i + loc[1] - 1
		close := matchParen(body, brace)
		if close < 0 {
			break
		}
		block := body[brace+1 : close]
		f := gradleFlavor{name: name}
		if m := gradleSuffix.FindStringSubmatch(block); m != nil {
			f.suffix = m[1]
		}
		if m := gradleID.FindStringSubmatch(block); m != nil {
			f.id = m[1]
		}
		if name != "" && !strings.Contains(name, "dimension") {
			out = append(out, f)
		}
		i = close + 1
	}
	return out
}

func androidLabel(root, sourceSet string) string {
	data, err := os.ReadFile(filepath.Join(root, "android/app/src", sourceSet, "res/values/strings.xml"))
	if err != nil {
		return ""
	}
	if m := regexp.MustCompile(`<string name="app_name">([^<]*)</string>`).FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return ""
}

type iosConfig struct{ id, name string }

var (
	pbxConfig  = regexp.MustCompile(`(?s)isa = XCBuildConfiguration;.*?buildSettings = \{(.*?)\n\t\t\t\};\s*name = "?([^";]+)"?;`)
	pbxSetting = func(key string) *regexp.Regexp { return regexp.MustCompile(`\b` + key + ` = "?([^";]+)"?;`) }
	pbxID      = pbxSetting("PRODUCT_BUNDLE_IDENTIFIER")
	pbxNames   = []*regexp.Regexp{pbxSetting("APP_DISPLAY_NAME"), pbxSetting("INFOPLIST_KEY_CFBundleDisplayName")}
)

// iosConfigs is the app's build configurations (not its tests'): name →
// bundle id and display name.
func iosConfigs(pbxproj string) map[string]iosConfig {
	out := map[string]iosConfig{}
	for _, m := range pbxConfig.FindAllStringSubmatch(pbxproj, -1) {
		settings := m[1]
		id := pbxID.FindStringSubmatch(settings)
		if id == nil || strings.Contains(strings.ToLower(id[1]), "tests") {
			continue
		}
		c := iosConfig{id: id[1]}
		for _, re := range pbxNames {
			if n := re.FindStringSubmatch(settings); n != nil {
				c.name = n[1]
				break
			}
		}
		out[m[2]] = c
	}
	return out
}

var (
	dartDoc   = regexp.MustCompile(`^\s*/// ?(.*)$`)
	dartValue = regexp.MustCompile(`^\s*(\w+)\((.*)\)\s*[,;]\s*$`)
	dartArg   = regexp.MustCompile(`(\w+)\s*:\s*(?:'([^']*)'|"([^"]*)")`)
	dartRef   = regexp.MustCompile(`\[(\w+)\]`)
)

// dartFlavorFacts finds an enum in lib/ with a value per flavor, like
// `dev(apiHost: 'test.example.com')`, and returns each one's strings
// (a host or URL as Backend) and doc comment (as About).
func dartFlavorFacts(root string, names []string) map[string]Info {
	if len(names) < 2 {
		return nil
	}
	var best map[string]Info
	_ = filepath.WalkDir(filepath.Join(root, "lib"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || best != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "gen" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		n := d.Name()
		if !strings.HasSuffix(n, ".dart") || strings.HasSuffix(n, ".g.dart") || strings.HasSuffix(n, ".freezed.dart") {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > 200<<10 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "enum ") {
			return nil
		}
		found := map[string]Info{}
		var doc []string
		for _, line := range strings.Split(string(data), "\n") {
			if m := dartDoc.FindStringSubmatch(line); m != nil {
				doc = append(doc, m[1])
				continue
			}
			m := dartValue.FindStringSubmatch(line)
			if m == nil || !contains(names, m[1]) {
				doc = nil
				continue
			}
			var info Info
			backend := false
			for _, a := range dartArg.FindAllStringSubmatch(m[2], -1) {
				key, value := a[1], a[2]+a[3]
				if !backend && regexp.MustCompile(`(?i)host|url|domain|server|endpoint`).MatchString(key) {
					key, backend = "Backend", true
				} else {
					key = humanize(key)
				}
				info = append(info, Fact{key, value})
			}
			if len(doc) > 0 {
				about := dartRef.ReplaceAllString(strings.Join(doc, " "), "$1")
				info = append(info, Fact{"About", strings.ReplaceAll(about, "`", "")})
			}
			found[m[1]] = info
			doc = nil
		}
		if len(found) == len(names) {
			best = found
		}
		return nil
	})
	return best
}

// humanize is apiHost → Api host.
func humanize(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case i == 0:
			b.WriteRune(unicode.ToUpper(r))
		case unicode.IsUpper(r):
			b.WriteRune(' ')
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

var flutterSDK = regexp.MustCompile(`(?m)^\s+sdk:\s*["']?flutter["']?\s*$`)

// IsFlutter reports whether dir is a Flutter project: its pubspec.yaml
// depends on the Flutter SDK. A plain Dart package isn't one.
func IsFlutter(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "pubspec.yaml"))
	return err == nil && flutterSDK.Match(data)
}

func pubspecName(root string) string {
	data, _ := os.ReadFile(filepath.Join(root, "pubspec.yaml"))
	if m := regexp.MustCompile(`(?m)^name:\s*(\S+)`).FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return ""
}

func readFirst(root string, paths ...string) string {
	for _, p := range paths {
		if data, err := os.ReadFile(filepath.Join(root, p)); err == nil {
			return string(data)
		}
	}
	return ""
}

// ── Targets ──────────────────────────────────────────────────────────────────

var (
	flagFlavor = regexp.MustCompile(`--flavor[= ](\S+)`)
	flagMain   = regexp.MustCompile(`-t\s+\S*main_(\w+)\.dart`)
	flagDevice = regexp.MustCompile(`-d\s+(\S+)`)
	buildKind  = regexp.MustCompile(`flutter build (\w+)`)
	iosWord    = regexp.MustCompile(`(?i)\bios\b|xcode|xcworkspace|simulator|\bpod\b|\bipa\b`)
	fdevCmd    = regexp.MustCompile(`\$[({]FDEV[)}]|(^|[;&|]\s*)fdev\b`)
)

// makeTargets turns the Makefile's rules into targets, in its order.
func makeTargets(root string, mk *makefile, fl flavors) ([]Group, map[string]*Ask) {
	groups := map[string]*Group{}
	order := []string{"Run", "Build", "Tools"}
	for _, g := range order {
		groups[g] = &Group{Name: g}
	}
	asks := map[string]*Ask{}
	hasAndroid := exists(filepath.Join(root, "android"))
	for _, r := range mk.rules {
		raw := strings.Join(r.recipe, "\n")
		if r.name == "help" || fdevCmd.MatchString(raw) {
			continue
		}
		cmd := mk.expand(raw)
		refs := mk.refs(raw)
		t := &Target{Name: r.name, Run: "make " + r.name}
		t.Desc = mk.expand(mk.help[r.name])
		if t.Desc == "" {
			t.Desc = r.desc
		}
		if t.Desc == "" {
			t.Desc = strings.TrimLeft(strings.TrimSpace(r.recipe[0]), "@-")
		}
		group := "Tools"
		switch {
		case strings.Contains(cmd, "flutter run"):
			group, t.Logs = "Run", true
		case strings.Contains(cmd, "flutter build"):
			group = "Build"
		}
		t.Flavor = flavorOf(cmd, fl)
		t.Platform = platformOf(r.name, raw, cmd, refs, hasAndroid, t.Flavor != "")
		if refs["DEVICE"] && mk.optional["DEVICE"] {
			switch t.Platform {
			case "Android", "iOS":
				t.Ask = append(t.Ask, "device:"+strings.ToLower(t.Platform))
			case "":
				t.Ask = append(t.Ask, "device")
			}
		}
		for _, v := range sortedKeys(refs) {
			if v == "DEVICE" || !mk.optional[v] || len(mk.tested[v]) == 0 {
				continue
			}
			if asks[strings.ToLower(v)] == nil {
				asks[strings.ToLower(v)] = makeAsk(v, mk.tested[v], fl)
			}
			t.Ask = append([]string{strings.ToLower(v)}, t.Ask...)
		}
		switch t.Platform {
		case "iOS", "macOS":
			t.OS = []string{"darwin"}
		case "Windows":
			t.OS = []string{"windows"}
		}
		groups[group].Targets = append(groups[group].Targets, t)
	}
	var out []Group
	for _, g := range order {
		if len(groups[g].Targets) > 0 {
			out = append(out, *groups[g])
		}
	}
	return out, asks
}

func flavorOf(cmd string, fl flavors) string {
	if m := flagFlavor.FindStringSubmatch(cmd); m != nil {
		if base, ok := fl.variants[m[1]]; ok {
			return base
		}
		if contains(fl.names, m[1]) {
			return m[1]
		}
	}
	if m := flagMain.FindStringSubmatch(cmd); m != nil && (len(fl.names) == 0 || contains(fl.names, m[1])) {
		return m[1]
	}
	return ""
}

func platformOf(name, raw, cmd string, refs map[string]bool, hasAndroid, flavored bool) string {
	if m := flagDevice.FindStringSubmatch(cmd); m != nil {
		switch strings.ToLower(m[1]) {
		case "chrome", "edge", "web-server":
			return "Web"
		case "macos":
			return "macOS"
		case "windows":
			return "Windows"
		case "linux":
			return "Linux"
		}
	}
	if m := buildKind.FindStringSubmatch(cmd); m != nil {
		switch m[1] {
		case "apk", "appbundle", "aar":
			return "Android"
		case "ios", "ipa", "ios-framework":
			return "iOS"
		case "web":
			return "Web"
		case "macos":
			return "macOS"
		case "windows":
			return "Windows"
		case "linux":
			return "Linux"
		}
	}
	for v := range refs {
		switch {
		case strings.Contains(v, "IOS"):
			return "iOS"
		case strings.Contains(v, "ANDROID"):
			return "Android"
		}
	}
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "ios") || iosWord.MatchString(raw):
		return "iOS"
	case strings.Contains(lower, "android") || strings.Contains(raw, "gradlew") || strings.Contains(raw, "adb "):
		return "Android"
	case strings.Contains(lower, "web"):
		return "Web"
	case strings.Contains(lower, "macos"):
		return "macOS"
	}
	if flavored && hasAndroid && strings.Contains(cmd, "flutter run") {
		return "Android" // --flavor without a device: flutter's usual pick
	}
	return ""
}

// knownStores dress the usual store editions: label, icon, color.
var knownStores = map[string][3]string{
	"":           {"Google Play", "play", "#4285F4"},
	"play":       {"Google Play", "play", "#4285F4"},
	"google":     {"Google Play", "play", "#4285F4"},
	"bazaar":     {"Cafe Bazaar", "bag", "#3DB54A"},
	"cafebazaar": {"Cafe Bazaar", "bag", "#3DB54A"},
	"myket":      {"Myket", "store", "#00A0E3"},
	"huawei":     {"AppGallery", "bag", "#CF0A2C"},
	"appgallery": {"AppGallery", "bag", "#CF0A2C"},
	"amazon":     {"Amazon Appstore", "store", "#FF9900"},
	"samsung":    {"Galaxy Store", "bag", "#1428A0"},
	"galaxy":     {"Galaxy Store", "bag", "#1428A0"},
	"xiaomi":     {"Xiaomi GetApps", "store", "#FF6900"},
	"fdroid":     {"F-Droid", "store", "#1976D2"},
}

// makeAsk is the question for a Makefile variable: leaving it unset, or one
// of the values the Makefile tests it against.
func makeAsk(name string, values []string, fl flavors) *Ask {
	storeLike := regexp.MustCompile(`(?i)store|market|shop|edition`).MatchString(name)
	ask := &Ask{Title: humanize(strings.ToLower(name)), Env: name}
	if storeLike {
		ask.Title = "Store edition"
	}
	option := func(value string) Option {
		o := Option{Value: value, Label: humanize(value)}
		if value == "" {
			o.Label, o.Desc = "Default", name+" unset"
		}
		if s, ok := knownStores[strings.ToLower(value)]; ok && (storeLike || value != "") {
			o.Label, o.Icon, o.Color = s[0], s[1], s[2]
		}
		if builds := fl.builds(value); builds != "" { // the Android flavors it builds
			o.Desc = "builds " + builds
		}
		return o
	}
	ask.Options = append(ask.Options, option(""))
	for _, v := range values {
		ask.Options = append(ask.Options, option(v))
	}
	return ask
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ── The snapshot in .fdev ────────────────────────────────────────────────────

// SnapshotPath is where fdev writes what it worked out, to look at; it
// never reads it back.
const SnapshotPath = ".fdev/fdev.yaml"

// writeSnapshot writes cfg to .fdev/fdev.yaml when it changed, with a
// .gitignore of `*` in .fdev so git sees nothing there.
func writeSnapshot(cfg *Config) {
	dir := filepath.Join(cfg.Root, ".fdev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	ignore := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(ignore); os.IsNotExist(err) {
		_ = os.WriteFile(ignore, []byte("# fdev's files (its config and saved logs); nothing here is committed.\n*\n"), 0o644)
	}
	text := cfg.YAML()
	path := filepath.Join(cfg.Root, SnapshotPath)
	if old, err := os.ReadFile(path); err == nil && string(old) == text {
		return
	}
	_ = os.WriteFile(path, []byte(text), 0o644)
}

// YAML is the config in fdev.yaml's format.
func (c *Config) YAML() string {
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	var b strings.Builder
	b.WriteString("# What fdev worked out from this project (" + c.Source + ", android/, ios/, lib/).\n")
	b.WriteString("# fdev makes it again at every start and never reads it: it's here to\n")
	b.WriteString("# look at, or to copy to fdev.yaml in the project root to take over.\n")
	fmt.Fprintf(&b, "title: %s\n\nlogs:\n  layout: %s\n  labels: %s\n  save: %v\n", q(c.Title), c.Logs.Layout, c.Logs.Labels, c.Logs.Save)
	if len(c.Flavors) > 0 {
		b.WriteString("\nflavors:\n")
		var names []string
		for n := range c.Flavors {
			names = append(names, n)
		}
		sort.SliceStable(names, func(i, j int) bool { return c.flavorIndex(names[i]) < c.flavorIndex(names[j]) })
		for _, n := range names {
			fmt.Fprintf(&b, "  %s:\n    info:\n", n)
			for _, f := range c.Flavors[n].Info {
				fmt.Fprintf(&b, "      %s: %s\n", q(f.Key), q(f.Value))
			}
		}
	}
	b.WriteString("\ngroups:\n")
	for _, g := range c.Groups {
		fmt.Fprintf(&b, "  - name: %s\n    targets:\n", g.Name)
		for _, t := range g.Targets {
			parts := []string{"name: " + q(t.Name)}
			if t.Flavor != "" {
				parts = append(parts, "flavor: "+q(t.Flavor))
			}
			if t.Platform != "" {
				parts = append(parts, "platform: "+q(t.Platform))
			}
			parts = append(parts, "desc: "+q(t.Desc), "run: "+q(t.Run))
			if t.Logs {
				parts = append(parts, "logs: true")
			}
			if len(t.Ask) > 0 {
				var a []string
				for _, x := range t.Ask {
					a = append(a, q(x))
				}
				parts = append(parts, "ask: ["+strings.Join(a, ", ")+"]")
			}
			if len(t.OS) > 0 {
				parts = append(parts, "os: ["+strings.Join(t.OS, ", ")+"]")
			}
			fmt.Fprintf(&b, "      - {%s}\n", strings.Join(parts, ", "))
		}
	}
	if len(c.Asks) > 0 {
		b.WriteString("\nasks:\n")
		var names []string
		for n := range c.Asks {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			a := c.Asks[n]
			fmt.Fprintf(&b, "  %s:\n    title: %s\n    env: %s\n    options:\n", n, q(a.Title), a.Env)
			for _, o := range a.Options {
				parts := []string{"label: " + q(o.Label), "value: " + q(o.Value)}
				for _, kv := range [][2]string{{"desc", o.Desc}, {"icon", o.Icon}, {"color", o.Color}} {
					if kv[1] != "" {
						parts = append(parts, kv[0]+": "+q(kv[1]))
					}
				}
				fmt.Fprintf(&b, "      - {%s}\n", strings.Join(parts, ", "))
			}
		}
	}
	return b.String()
}

// flavorIndex orders flavors as the targets first use them.
func (c *Config) flavorIndex(name string) int {
	for i, t := range c.Targets() {
		if t.Flavor == name {
			return i
		}
	}
	return 1 << 20
}
