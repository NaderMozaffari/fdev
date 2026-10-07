package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMakefileReadsBack writes the Makefile fdev init would and checks
// that fdev reads the same targets out of it.
func TestMakefileReadsBack(t *testing.T) {
	for _, tc := range []struct {
		name   string
		files  map[string]string
		want   map[string]string // name: group|flavor|platform|asks
		recipe map[string]string
	}{
		{
			name: "flavors",
			files: map[string]string{
				"pubspec.yaml": "name: acme\ndependencies:\n  flutter:\n    sdk: flutter\ndev_dependencies:\n  build_runner: any\n",
				"android/app/build.gradle": `android {
    productFlavors {
        dev { dimension "env" }
        prod { dimension "env" }
    }
}
`,
				"lib/main_dev.dart":     "",
				"lib/main_prod.dart":    "",
				"ios/Runner/Info.plist": "",
				"web/index.html":        "",
			},
			want: map[string]string{
				"dev":            "Run|dev|Android|device:android",
				"web-dev":        "Run|dev|Web|",
				"build-apk-prod": "Build|prod|Android|",
				"build-aab-dev":  "Build|dev|Android|",
				"build-ipa-prod": "Build|prod|iOS|",
				"build-web-dev":  "Build|dev|Web|",
				"build-runner":   "Tools|||",
				"clean":          "Tools|||",
			},
			recipe: map[string]string{
				"dev":            "flutter run --flavor dev -t lib/main_dev.dart $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)",
				"build-ipa-prod": "flutter build ipa --release -t lib/main_prod.dart", // iOS has no prod scheme
			},
		},
		{
			name: "no flavors",
			files: map[string]string{
				"pubspec.yaml":             "name: plain\ndependencies:\n  flutter:\n    sdk: flutter\n",
				"android/app/build.gradle": "android {}\n",
			},
			want: map[string]string{
				"run":       "Run|||device",
				"build-apk": "Build||Android|",
				"build-aab": "Build||Android|",
				"get":       "Tools|||",
			},
			recipe: map[string]string{"run": "flutter run $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for path, text := range tc.files {
				must(t, os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755))
				must(t, os.WriteFile(filepath.Join(root, path), []byte(text), 0o644))
			}
			text, names := Makefile(root)
			must(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte(text), 0o644))
			cfg, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.NeedsMakefile() {
				t.Error("NeedsMakefile with a Makefile")
			}
			got := map[string]*Target{}
			for _, g := range cfg.Groups {
				for _, tg := range g.Targets {
					tg.Group = g.Name
					got[tg.Name] = tg
				}
			}
			for _, n := range names {
				tg := got[n]
				if tg == nil {
					t.Errorf("target %s isn't read back", n)
					continue
				}
				if strings.HasPrefix(tg.Desc, "flutter ") || strings.HasPrefix(tg.Desc, "dart ") {
					t.Errorf("%s has no description: %q", n, tg.Desc)
				}
			}
			for n, want := range tc.want {
				tg := got[n]
				if tg == nil {
					t.Errorf("no target %s in %v", n, names)
					continue
				}
				if s := strings.Join([]string{tg.Group, tg.Flavor, tg.PlatformName(), strings.Join(tg.Ask, "+")}, "|"); s != want {
					t.Errorf("%s = %q, want %q", n, s, want)
				}
			}
			for n, want := range tc.recipe {
				if !strings.Contains(text, "\n"+n+": ") || !strings.Contains(text, "\t"+want+"\n") {
					t.Errorf("%s: want recipe %q in\n%s", n, want, text)
				}
			}
			if runtime.GOOS != "windows" {
				if _, ok := got["build-ipa-prod"]; tc.name == "flavors" && !ok && runtime.GOOS == "darwin" {
					t.Error("no build-ipa-prod")
				}
			}
		})
	}
}

func TestNeedsMakefileAndFindProjects(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "apps", "shop")
	must(t, os.MkdirAll(filepath.Join(app, "build", "x"), 0o755))
	must(t, os.WriteFile(filepath.Join(app, "pubspec.yaml"), []byte("name: shop\ndependencies:\n  flutter:\n    sdk: flutter\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(app, "build", "x", "pubspec.yaml"), []byte("flutter:\n"), 0o644))
	if got := FindProjects(dir); len(got) != 1 || got[0] != app {
		t.Errorf("FindProjects = %v, want [%s]", got, app)
	}
	if _, err := Load(dir); err != ErrNoProject {
		t.Errorf("Load above a project: %v", err)
	}
	cfg, err := Load(app)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.NeedsMakefile() {
		t.Error("a project with neither Makefile nor fdev.yaml should need one")
	}
}

func TestWhatIsAProject(t *testing.T) {
	write := func(dir, path, text string) {
		t.Helper()
		must(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755))
		must(t, os.WriteFile(filepath.Join(dir, path), []byte(text), 0o644))
	}

	// A Dart package isn't a Flutter project: no menu of flutter commands.
	dart := t.TempDir()
	write(dart, "pubspec.yaml", "name: tool\ndescription: Works with flutter: and dart.\ndependencies:\n  path: any\n")
	if IsFlutter(dart) {
		t.Error("a Dart package counts as Flutter")
	}
	if _, err := Load(dart); err != ErrNoProject {
		t.Errorf("Load(Dart package) = %v, want ErrNoProject", err)
	}

	// A Makefile is a project of any kind, with only its own targets.
	mk := t.TempDir()
	write(mk, "Makefile", "# Build the binary\nbuild:\n\tgo build ./...\n")
	cfg, err := Load(mk)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Flutter || cfg.NeedsMakefile() {
		t.Errorf("Flutter %v, NeedsMakefile %v for a Go project", cfg.Flutter, cfg.NeedsMakefile())
	}
	if ts := cfg.Targets(); len(ts) != 1 || ts[0].Name != "build" || ts[0].Desc != "Build the binary" {
		t.Errorf("targets %+v", ts)
	}

	// A Makefile without targets has nothing to run, rather than flutter's.
	empty := t.TempDir()
	write(empty, "Makefile", "X = 1\n")
	if cfg, err := Load(empty); err != nil || len(cfg.Targets()) != 0 {
		t.Errorf("Load(empty Makefile) = %v targets, %v", len(cfg.Targets()), err)
	}

	// A Flutter project needs no Makefile, and is offered one.
	app := t.TempDir()
	write(app, "pubspec.yaml", "name: app\ndependencies:\n  flutter:\n    sdk: flutter\n")
	cfg, err = Load(app)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Flutter || !cfg.NeedsMakefile() || len(cfg.Targets()) == 0 {
		t.Errorf("Flutter %v, NeedsMakefile %v, %d targets", cfg.Flutter, cfg.NeedsMakefile(), len(cfg.Targets()))
	}
}
