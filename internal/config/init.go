package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// GuideURL is the guide to the Makefile and fdev.yaml fdev reads, in
// English and Persian.
const (
	GuideURL   = "https://github.com/NaderMozaffari/fdev/blob/main/docs/CUSTOMIZE.md"
	GuideURLFa = "https://github.com/NaderMozaffari/fdev/blob/main/docs/CUSTOMIZE.fa.md"
)

// ErrNoProject is Load's error outside a Flutter project.
var ErrNoProject = errors.New("no fdev.yaml, Makefile or pubspec.yaml here or in a parent directory")

// NeedsMakefile reports whether fdev made the project's targets up from
// the project itself, with neither an fdev.yaml nor a Makefile to read:
// what `fdev init` is for.
func (c *Config) NeedsMakefile() bool {
	for _, f := range append([]string{"Makefile", "makefile", "GNUmakefile"}, FileNames...) {
		if exists(filepath.Join(c.Root, f)) {
			return false
		}
	}
	return true
}

// FindProjects is the Flutter projects in the folders below dir, a few
// levels down, for when fdev runs above one.
func FindProjects(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			n := d.Name()
			if path != dir && (strings.HasPrefix(n, ".") || slices.Contains([]string{"build", "node_modules", "Pods", "ios", "android", "macos", "windows", "linux", "web", "lib", "test"}, n) ||
				strings.Count(rel, string(filepath.Separator)) >= 3) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "pubspec.yaml" && len(out) < 10 {
			if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), "flutter:") {
				out = append(out, filepath.Dir(path))
			}
		}
		return nil
	})
	return out
}

// Makefile is a Makefile for a project without one: running and building
// each of its flavors (or the app, without flavors), and the usual tools,
// written so that Discover reads it back as the same targets. targets is
// their names, in order.
func Makefile(root string) (text string, targets []string) {
	fl := discoverFlavors(root)
	gradle := readFirst(root, "android/app/build.gradle", "android/app/build.gradle.kts")
	androidFlavors := len(gradleFlavors(gradle)) > 0
	ios := iosConfigs(readFirst(root, "ios/Runner.xcodeproj/project.pbxproj"))
	has := func(dir string) bool { return exists(filepath.Join(root, dir)) }
	android, iphone, web := has("android"), has("ios"), has("web")

	title := fl.title
	if title == "" {
		title = pubspecName(root)
	}
	if title == "" {
		title = filepath.Base(root)
	}

	type rule struct{ name, desc, recipe string }
	var run, build, tools []rule
	// args is a flavor's flags: --flavor where the platform has it, and its
	// entry point when it has one of its own.
	args := func(f string, flavorFlag bool) string {
		var a []string
		if flavorFlag {
			a = append(a, "--flavor "+f)
		}
		if main := "lib/main_" + f + ".dart"; exists(filepath.Join(root, main)) {
			a = append(a, "-t "+main)
		}
		return strings.Join(a, " ")
	}
	cmd := func(parts ...string) string {
		var out []string
		for _, p := range parts {
			if p != "" {
				out = append(out, p)
			}
		}
		return strings.Join(out, " ")
	}
	const device, logs = "$(if $(DEVICE),-d $(DEVICE))", "$(FDEV_DART_DEFINES)"

	if len(fl.names) == 0 {
		run = append(run, rule{"run", "Run the app", cmd("flutter run", device, logs)})
		if web {
			run = append(run, rule{"web", "Run the app in Chrome", cmd("flutter run -d chrome", logs)})
		}
		if android {
			build = append(build,
				rule{"build-apk", "A release APK", "flutter build apk --release"},
				rule{"build-aab", "A release app bundle, for Google Play", "flutter build appbundle --release"})
		}
		if iphone {
			build = append(build, rule{"build-ipa", "A release IPA, for the App Store", "flutter build ipa --release"})
		}
		if web {
			build = append(build, rule{"build-web", "A release web build", "flutter build web --release"})
		}
	}
	for _, f := range fl.names {
		iosFlavor := ios["Debug-"+f].id != "" || ios["Release-"+f].id != ""
		about := "the " + f + " flavor"
		if info := fl.info[f]; info != nil {
			for _, fact := range info.Info {
				if fact.Key == "App" {
					about += " (" + fact.Value + ")"
					break
				}
			}
		}
		run = append(run, rule{f, "Run " + about, cmd("flutter run", args(f, androidFlavors || iosFlavor), device, logs)})
		if android && iphone && iosFlavor {
			run = append(run, rule{"ios-" + f, "Run " + about + " on an iPhone or the simulator", cmd("flutter run", args(f, true), device, logs)})
		}
		if web {
			run = append(run, rule{"web-" + f, "Run " + about + " in Chrome", cmd("flutter run -d chrome", args(f, false), logs)})
		}
		if android {
			build = append(build,
				rule{"build-apk-" + f, "A release APK of " + f, cmd("flutter build apk --release", args(f, androidFlavors))},
				rule{"build-aab-" + f, "A release app bundle of " + f + ", for Google Play", cmd("flutter build appbundle --release", args(f, androidFlavors))})
		}
		if iphone {
			build = append(build, rule{"build-ipa-" + f, "A release IPA of " + f + ", for the App Store", cmd("flutter build ipa --release", args(f, iosFlavor))})
		}
		if web {
			build = append(build, rule{"build-web-" + f, "A release web build of " + f, cmd("flutter build web --release", args(f, false))})
		}
	}
	tools = append(tools,
		rule{"devices", "List the devices to run on", "flutter devices"},
		rule{"get", "Get the packages", "flutter pub get"})
	if strings.Contains(readFirst(root, "pubspec.yaml"), "build_runner") {
		tools = append(tools, rule{"build-runner", "Generate code with build_runner", "dart run build_runner build --delete-conflicting-outputs"})
	}
	tools = append(tools,
		rule{"test", "Run the tests", "flutter test"},
		rule{"analyze", "Analyze the code", "flutter analyze"},
		rule{"clean", "Delete the build files", "flutter clean"})

	for _, rs := range [][]rule{run, build, tools} {
		for _, r := range rs {
			targets = append(targets, r.name)
		}
	}
	first := targets[0]

	var b strings.Builder
	fmt.Fprintf(&b, "# %s: the targets fdev offers, written by `fdev init`.\n", title)
	b.WriteString("#\n# fdev (https://github.com/NaderMozaffari/fdev) shows them in its menu, and\n# make runs them without it too:\n#\n")
	fmt.Fprintf(&b, "#   make %-22s %s\n", first, "run it; flutter picks the device")
	fmt.Fprintf(&b, "#   make %-22s %s\n", first+" DEVICE=<id>", "on that device (make devices lists them)")
	b.WriteString("#\n# Change them and add your own: fdev shows the ## text after a target as\n" +
		"# its description, asks for a device when the recipe uses DEVICE, and\n" +
		"# asks the other questions you give it. How:\n# " + GuideURL + "\n\n")
	b.WriteString("# The device to run on, from `make devices`. Empty lets flutter pick.\nDEVICE ?=\n\n")
	b.WriteString("# fdev sets FDEV_DART_DEFINES to --dart-define=FDEV_LOGS=true, which turns\n" +
		"# on its structured logs (see its README); without fdev it is empty.\n\n")
	line := ".PHONY:"
	for _, t := range targets {
		if len(line)+1+len(t) > 76 {
			b.WriteString(line + " \\\n")
			line = "\t"
		} else {
			line += " "
		}
		line += t
	}
	b.WriteString(line + "\n")
	for _, section := range []struct {
		name  string
		rules []rule
	}{{"Run", run}, {"Build", build}, {"Tools", tools}} {
		if len(section.rules) == 0 {
			continue
		}
		b.WriteString("\n# ── " + section.name + " " + strings.Repeat("─", 70-len(section.name)) + "\n")
		for _, r := range section.rules {
			fmt.Fprintf(&b, "\n%s: ## %s\n\t%s\n", r.name, r.desc, r.recipe)
		}
	}
	return b.String(), targets
}
