package config

import (
	"errors"
	"fmt"
	"io/fs"
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

// ErrNoProject is Load's error where fdev knows no project: no fdev.yaml,
// Makefile or Flutter project here or in a folder above.
var ErrNoProject = errors.New("no fdev.yaml, Makefile or Flutter project here or in a parent directory")

// NeedsMakefile reports whether fdev made a Flutter project's targets up
// from the project itself, with neither an fdev.yaml nor a Makefile to
// read: what `fdev init` is for.
func (c *Config) NeedsMakefile() bool {
	if !c.Flutter {
		return false
	}
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
		if d.Name() == "pubspec.yaml" && len(out) < 10 && IsFlutter(filepath.Dir(path)) {
			out = append(out, filepath.Dir(path))
		}
		return nil
	})
	return out
}

// Makefile is a Makefile for a Flutter project without one: running and
// building each of its flavors (or the app, without flavors), and the
// usual tools, written so that Discover reads it back as the same targets.
// It is meant for anyone to edit, programmer or not: a header on how a
// command looks, a settings section, one plain command line per target,
// and make's machinery at the bottom. targets is their names, in order.
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
		if f == "" {
			return ""
		}
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
	// Each flavor's run and builds, or the app's without flavors.
	flavors := slices.Clone(fl.names)
	if len(flavors) == 0 {
		flavors = []string{""}
	}
	for _, f := range flavors {
		iosFlavor := f != "" && (ios["Debug-"+f].id != "" || ios["Release-"+f].id != "")
		name := func(prefix string) string { // dev, ios-dev, build-apk-dev; run, ios, build-apk
			switch {
			case f == "" && prefix == "":
				return "run"
			case f == "":
				return strings.TrimSuffix(prefix, "-")
			}
			return prefix + f
		}
		what, of := "the app", ""
		if f != "" {
			what, of = "the "+f+" flavor", " of "+f
			if info := fl.info[f]; info != nil {
				for _, fact := range info.Info {
					if fact.Key == "App" {
						what += " (" + fact.Value + ")"
						break
					}
				}
			}
		}
		run = append(run, rule{name(""), "Run " + what, cmd("flutter run", args(f, androidFlavors || iosFlavor), "$(RUN)")})
		if android && iphone && iosFlavor {
			run = append(run, rule{name("ios-"), "Run " + what + " on an iPhone or the iOS simulator", cmd("flutter run", args(f, true), "$(RUN)")})
		}
		if web {
			run = append(run, rule{name("web-"), "Run " + what + " in Chrome", cmd("flutter run -d chrome", args(f, false), "$(RUN_WEB)")})
		}
		if android {
			build = append(build,
				rule{name("build-apk-"), "Android app (APK)" + of + ", to install or share", cmd("flutter build apk", args(f, androidFlavors), "$(BUILD)")},
				rule{name("build-aab-"), "Android app bundle" + of + ", for Google Play", cmd("flutter build appbundle", args(f, androidFlavors), "$(BUILD)")})
		}
		if iphone {
			build = append(build, rule{name("build-ipa-"), "iOS app (IPA)" + of + ", for the App Store", cmd("flutter build ipa", args(f, iosFlavor), "$(BUILD)")})
		}
		if web {
			build = append(build, rule{name("build-web-"), "Website" + of + ", in build/web", cmd("flutter build web", args(f, false), "$(BUILD)")})
		}
	}
	tools = append(tools,
		rule{"devices", "List the phones, simulators and browsers to run on", "flutter devices"},
		rule{"get", "Download the packages in pubspec.yaml", "flutter pub get"})
	if strings.Contains(readFirst(root, "pubspec.yaml"), "build_runner") {
		tools = append(tools, rule{"build-runner", "Generate code (build_runner)", "dart run build_runner build --delete-conflicting-outputs"})
	}
	tools = append(tools,
		rule{"test", "Run the tests", "flutter test"},
		rule{"analyze", "Check the code for problems", "flutter analyze"},
		rule{"clean", "Delete the build files, to start fresh", "flutter clean"})

	for _, rs := range [][]rule{run, build, tools} {
		for _, r := range rs {
			targets = append(targets, r.name)
		}
	}
	first := targets[0]
	heading := func(title string) string {
		return "# ── " + title + " " + strings.Repeat("─", 72-len([]rune(title))) + "\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s: the commands fdev's menu offers.\n", title)
	b.WriteString(`#
# fdev (https://github.com/NaderMozaffari/fdev) reads this file every time it
# starts, and shows each command below in its menu. They work without fdev
# too: "make ` + first + `" runs ` + first + `, and "make" alone lists them all.
#
# Each command is two lines:
#
#     ` + first + `: ## ` + run[0].desc + `
#     →  ` + run[0].recipe + `
#
#   - The first line is its name, a colon, then ## and what the menu says.
#   - The second line is what it runs. → is a TAB (the Tab key), not spaces.
#
# To add a command, copy one, then change its name, text and command line.
# To remove one, delete its two lines. Save, and fdev shows the change.
#
# More, with examples: ` + GuideURL + `

`)
	b.WriteString(heading("Settings"))
	b.WriteString(`
# Added to every run below. For example:
#   RUN_OPTIONS = --dart-define=API_URL=https://test.example.com
RUN_OPTIONS =

# Added to every build below. For example:
#   BUILD_OPTIONS = --obfuscate --split-debug-info=build/symbols
BUILD_OPTIONS =

`)
	for _, section := range []struct {
		name  string
		rules []rule
	}{{"Run", run}, {"Build", build}, {"Tools", tools}} {
		if len(section.rules) == 0 {
			continue
		}
		b.WriteString(heading(section.name))
		for _, r := range section.rules {
			fmt.Fprintf(&b, "\n%s: ## %s\n\t%s\n", r.name, r.desc, r.recipe)
		}
		b.WriteString("\n")
	}
	b.WriteString(heading("fdev's part: nothing to change below"))
	b.WriteString(`#
# RUN is what each run adds: the device to run on (fdev asks; with make,
# "make ` + first + ` DEVICE=emulator-5554", and "make devices" lists them), your
# RUN_OPTIONS, and FDEV_DART_DEFINES, which fdev sets to turn on its
# structured logs. RUN_WEB is the same for Chrome, and BUILD your BUILD_OPTIONS.

DEVICE ?=
RUN = $(if $(DEVICE),-d $(DEVICE)) $(RUN_OPTIONS) $(FDEV_DART_DEFINES)
RUN_WEB = $(RUN_OPTIONS) $(FDEV_DART_DEFINES)
BUILD = $(BUILD_OPTIONS)

# "make" alone lists the commands.
.DEFAULT_GOAL := help
help:
	@awk -F ':.*## ' '/^[A-Za-z0-9_.-]+:.*## / { printf "  make %-22s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# A command runs even when a file or folder has its name (test/, web/).
.PHONY: $(MAKECMDGOALS)
`)
	return b.String(), targets
}
