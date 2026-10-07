package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const testMakefile = `.PHONY: dev build-dev clean
# Run the test app
dev:
	flutter run --flavor dev -t lib/main_dev.dart $(RUN_ARGS)

build-dev: ## APK, dev flavor
	flutter build apk --flavor dev

VAR := x
clean:
	flutter clean
`

func TestMakefileDiscovery(t *testing.T) {
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte(testMakefile), 0o644))
	must(t, os.MkdirAll(filepath.Join(root, "lib", "src"), 0o755))

	cfg, err := Load(filepath.Join(root, "lib", "src"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Root != root || cfg.Source != "auto · Makefile" {
		t.Errorf("root %q source %q", cfg.Root, cfg.Source)
	}
	targets := cfg.Targets()
	if len(targets) != 3 {
		t.Fatalf("targets = %+v", targets)
	}
	dev, build, clean := targets[0], targets[1], targets[2]
	if dev.Name != "dev" || !dev.Logs || dev.Group != "Run" || dev.Desc != "Run the test app" || dev.Run != "make dev" {
		t.Errorf("dev = %+v", dev)
	}
	if build.Group != "Build" || build.Desc != "APK, dev flavor" || build.Logs || build.PlatformName() != "Android" {
		t.Errorf("build = %+v", build)
	}
	if clean.Group != "Tools" {
		t.Errorf("clean = %+v", clean)
	}

	// What it worked out is in .fdev, which git ignores.
	snapshot, err := os.ReadFile(filepath.Join(root, ".fdev", "fdev.yaml"))
	if err != nil || !strings.Contains(string(snapshot), `name: "build-dev"`) {
		t.Errorf("snapshot: %v\n%s", err, snapshot)
	}
	if ignore, _ := os.ReadFile(filepath.Join(root, ".fdev", ".gitignore")); !strings.Contains(string(ignore), "*") {
		t.Errorf(".fdev/.gitignore = %q", ignore)
	}
}

// A Flutter project like most: flavors with store editions, an entry point
// per flavor, a Makefile that takes DEVICE= and STORE=.
func TestDiscoverFlutterProject(t *testing.T) {
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		must(t, os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755))
		must(t, os.WriteFile(filepath.Join(root, path), []byte(text), 0o644))
	}
	write("Makefile", `MAIN_DEV := lib/main_dev.dart
MAIN_PROD := lib/main_prod.dart
RUN_ARGS ?= $(FDEV_DART_DEFINES)
DEVICE ?=
DEVICE_FLAG = $(if $(DEVICE),-d $(DEVICE),)
STORE ?=
STORE_SUFFIX = $(if $(filter myket,$(STORE)),Myket,)
ANDROID_DEV = dev$(STORE_SUFFIX)
ANDROID_PROD = prod$(STORE_SUFFIX)
IOS_DEVICE = $(or $(DEVICE),$(shell flutter devices | head -1))
FDEV = $(shell command -v fdev)
WEB_PORT ?= 5050

help:
	@echo "  dev        Run the test app"
	@echo "  web-dev    In Chrome on localhost:$(WEB_PORT)"

ui:
	@$(FDEV)

dev:
	flutter run --flavor $(ANDROID_DEV) -t $(MAIN_DEV) $(DEVICE_FLAG) $(RUN_ARGS)

ios-prod:
	flutter run --flavor prod -t $(MAIN_PROD) -d $(IOS_DEVICE) $(RUN_ARGS)

web-dev:
	flutter run -d chrome -t $(MAIN_DEV) --web-port $(WEB_PORT)

build-prod:
	flutter build apk --flavor $(ANDROID_PROD) -t $(MAIN_PROD)

pod-install:
	cd ios && pod install
`)
	write("android/app/build.gradle", `android {
    defaultConfig {
        applicationId "com.example.app"
    }
    productFlavors {
        // The test app
        dev {
            dimension "env"
            applicationIdSuffix ".dev"
        }
        prod { dimension "env" }
        devMyket {
            dimension "env"
            applicationIdSuffix ".dev"
        }
        prodMyket { dimension "env" }
    }
}
`)
	write("android/app/src/main/res/values/strings.xml", `<resources><string name="app_name">Example</string></resources>`)
	write("android/app/src/dev/res/values/strings.xml", `<resources><string name="app_name">Example Dev</string></resources>`)
	write("lib/core/flavor.dart", `enum Flavor {
  /// Test app, on [prod]'s side.
  dev(apiHost: 'test.example.com'),

  /// Store app.
  prod(apiHost: 'example.com');

  const Flavor({required this.apiHost});
  final String apiHost;
}
`)

	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "Example" {
		t.Errorf("title %q", cfg.Title)
	}
	got := map[string]string{}
	for _, tg := range cfg.Targets() {
		got[tg.Name] = strings.Join([]string{tg.Group, tg.Flavor, tg.PlatformName(), strings.Join(tg.Ask, "+"), tg.Desc}, "|")
	}
	want := map[string]string{
		"dev":        "Run|dev|Android|store+device:android|Run the test app",
		"web-dev":    "Run|dev|Web||In Chrome on localhost:5050",
		"build-prod": "Build|prod|Android|store|flutter build apk --flavor $(ANDROID_PROD) -t $(MAIN_PROD)",
		WifiTarget:   "Tools||||Debug an Android phone over Wi-Fi: pair with a QR code or pairing code", // fdev's own
	}
	if runtime.GOOS == "darwin" { // iOS targets are macOS-only
		want["pod-install"] = "Tools||iOS||cd ios && pod install"
		want["ios-prod"] = "Run|prod|iOS|device:ios|flutter run --flavor prod -t $(MAIN_PROD) -d $(IOS_DEVICE) $(RUN_ARGS)"
	}
	if len(got) != len(want) {
		t.Errorf("targets %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}

	store := cfg.Asks["store"]
	if store == nil || store.Title != "Store edition" || store.Env != "STORE" || len(store.Options) != 2 {
		t.Fatalf("store = %+v", store)
	}
	if o := store.Options[1]; o.Label != "Myket" || o.Value != "myket" || o.Icon != "store" || o.Desc != "builds devMyket · prodMyket" {
		t.Errorf("myket = %+v", o)
	}
	if o := store.Options[0]; o.Label != "Google Play" || o.Desc != "builds dev · prod" {
		t.Errorf("default = %+v", o)
	}

	dev := cfg.Flavors["dev"].Info
	wantDev := Info{{"App", "Example Dev"}, {"Android ID", "com.example.app.dev"}, {"Backend", "test.example.com"}, {"About", "Test app, on prod's side."}}
	if len(dev) != len(wantDev) {
		t.Fatalf("dev = %+v", dev)
	}
	for i := range dev {
		if dev[i] != wantDev[i] {
			t.Errorf("dev[%d] = %+v, want %+v", i, dev[i], wantDev[i])
		}
	}
	if _, ok := cfg.Flavors["devMyket"]; ok {
		t.Error("store editions are not flavors of their own")
	}
}

func TestMakeExpand(t *testing.T) {
	mk := parseMakefile("A := x\nB = $(A)y\nS ?=\nC = $(if $(filter b,$(S)),yes,no) $(or $(S),$(B))\n")
	if got := mk.expand("$(C) $$HOME $(shell date)"); got != "no xy $HOME " {
		t.Errorf("expand = %q", got)
	}
	if !mk.optional["S"] || len(mk.tested["S"]) != 1 || !mk.refs("$(C)")["S"] {
		t.Errorf("optional %v tested %v", mk.optional, mk.tested)
	}
}

func TestConfigFile(t *testing.T) {
	root := t.TempDir()
	yaml := `title: Demo
groups:
  - name: Run
    targets:
      - {name: dev, run: make dev, logs: true, ask: [store, "device:android"]}
      - {name: mac, run: make mac, os: [plan9]}
asks:
  store:
    title: Store
    env: STORE
    options: [{label: Play, value: ""}, {label: Other, value: other}]
`
	must(t, os.WriteFile(filepath.Join(root, "fdev.yaml"), []byte(yaml), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte(testMakefile), 0o644))
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "Demo" || cfg.Source != "fdev.yaml" || len(cfg.Targets()) != 1 {
		t.Errorf("cfg = %+v targets %+v", cfg, cfg.Targets())
	}
	if p, ok := DeviceAsk("device:android"); !ok || p != "android" {
		t.Errorf("DeviceAsk = %q %v", p, ok)
	}

	bad := `groups: [{name: Run, targets: [{name: dev, run: x, ask: [nope]}]}]`
	must(t, os.WriteFile(filepath.Join(root, "fdev.yaml"), []byte(bad), 0o644))
	if _, err := Load(root); err == nil {
		t.Error("an unknown ask should fail")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestFlavorsVersionAndIcons(t *testing.T) {
	root := t.TempDir()
	yaml := `groups: [{name: Run, targets: [{name: dev, run: make dev, flavor: dev, ask: ["device:android"]}, {name: web, run: make web-dev}]}]
flavors:
  dev:
    info: {App: My App Dev, ID: com.example.dev, Backend: test.example.com}
`
	must(t, os.WriteFile(filepath.Join(root, "fdev.yaml"), []byte(yaml), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "pubspec.yaml"), []byte("name: x\nversion: 2.1.39+439\n"), 0o644))
	icon := filepath.Join(root, "android/app/src/dev/res/mipmap-xxxhdpi/launcher_icon.png")
	must(t, os.MkdirAll(filepath.Dir(icon), 0o755))
	must(t, os.WriteFile(icon, []byte("png"), 0o644))

	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != "2.1.39" || cfg.Build != "439" {
		t.Errorf("version %q build %q", cfg.Version, cfg.Build)
	}
	info := cfg.Flavors["dev"].Info
	if len(info) != 3 || info[0] != (Fact{"App", "My App Dev"}) || info[2].Key != "Backend" {
		t.Errorf("info = %+v", info)
	}
	if got := cfg.FlavorIcon("dev"); got != icon {
		t.Errorf("icon = %q", got)
	}
	dev, web := cfg.Targets()[0], cfg.Targets()[1]
	if dev.PlatformName() != "Android" || web.PlatformName() != "Web" {
		t.Errorf("platforms %q %q", dev.PlatformName(), web.PlatformName())
	}
}
