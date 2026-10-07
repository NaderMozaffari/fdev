# Set up fdev for your project

**English** · [فارسی](CUSTOMIZE.fa.md)

fdev works in any Flutter project with nothing set up. This guide is for
when you want to choose what its menu offers: your own targets, their
descriptions, and the questions they ask before they run.

- [Where fdev gets its targets](#where-fdev-gets-its-targets)
- [Start with `fdev init`](#start-with-fdev-init)
- [The Makefile: what fdev reads](#the-makefile-what-fdev-reads)
- [Examples](#examples)
- [fdev.yaml: full control](#fdevyaml-full-control)
- [Check what fdev read](#check-what-fdev-read)

## Where fdev gets its targets

fdev looks in the folder you run it in, then the ones above it, and uses
the first of these it finds:

| | What fdev does |
|---|---|
| **`fdev.yaml`** | uses it exactly as written ([below](#fdevyaml-full-control)) |
| **`Makefile`** | reads its targets, and works out their group, flavor, platform and questions |
| **neither**, in a Flutter project | makes up `flutter run` for each flavor, the usual builds, `pub get`, `test`, … |

Without either file, fdev offers once to write a Makefile for a Flutter
project. A Makefile works in a project of any kind (Go, Node, a Dart
package, …): fdev shows its targets, all under **Tools** unless they run
`flutter run` or `flutter build`.

## Start with `fdev init`

```sh
cd path/to/your_app
fdev init
```

This writes a `Makefile` made for your project:

- **With flavors** (Android `productFlavors`, or `lib/main_<flavor>.dart`
  entry points): `dev`, `prod`, … to run each flavor, `ios-dev` when iOS
  has that flavor too, `web-dev` when there's a `web/` folder, and
  `build-apk-dev`, `build-aab-dev`, `build-ipa-dev`, `build-web-dev`, …
- **Without flavors**: `run`, `web`, `build-apk`, `build-aab`, `build-ipa`,
  `build-web`.
- **Tools**, either way: `devices`, `get`, `test`, `analyze`, `clean`, and
  `build-runner` when the project uses it.

It uses `--flavor` only where your project has that flavor (Android
`productFlavors`, iOS schemes), so the targets work as they are. Edit them
however you like: the Makefile is yours, and `make dev` works without fdev
too.

| | |
|---|---|
| `fdev init --print` | print it instead, to compare with a Makefile you have |
| `fdev init --force` | replace the Makefile that's there |

## The Makefile: what fdev reads

Each rule with a recipe is a target. fdev reads the recipe the way make
would expand it (variables, `$(if …)`, `$(filter …)`, …), so it can tell
what each target does.

**The description** shown in the menu is `## text` after the target:

```make
dev: ## Run the test app
```

or a comment right above it:

```make
# Run the test app
dev:
```

or the target's line in a `make help` rule, which wins over both:

```make
help:
	@echo "  dev        Run the test app"
```

**The group** comes from what the recipe runs:

| Recipe has | Group | |
|---|---|---|
| `flutter run` | **Run** | opens in the log viewer |
| `flutter build` | **Build** | |
| anything else | **Tools** | |

**The flavor** comes from `--flavor dev` or `-t lib/main_dev.dart`. The menu
then shows the flavor's icon and facts next to the target.

**The platform** comes from `-d chrome` / `-d macos` / …, from
`build apk` / `build ipa` / `build web` / …, from variables named
`ANDROID_…` or `IOS_…`, or from the target's name (`ios-dev`, `web-prod`).
A `flutter run` with `--flavor` and no device counts as Android. iOS and
macOS targets are only offered on a Mac.

**Asking for a device.** Declare an empty `DEVICE ?=` and use it in the
recipe. fdev then lists the connected devices (only Android or only iOS
ones, when the target has that platform) and runs the target with
`DEVICE=<id>`:

```make
DEVICE ?=

dev: ## Run the test app
	flutter run --flavor dev $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

**Asking anything else.** Any other empty `NAME ?=` that a recipe uses
becomes a question. Its answers are the values the Makefile compares it to,
with `$(filter value,$(NAME))` or `ifeq ($(NAME),value)`, plus *Default*
(left unset). A name with *store*, *market*, *shop* or *edition* in it is
titled *Store edition*, and well-known stores (`bazaar`, `myket`, `huawei`,
`amazon`, `samsung`, …) get their names and icons.

**fdev's structured logs.** fdev sets `FDEV_DART_DEFINES` to
`--dart-define=FDEV_LOGS=true` when it runs a target. Put
`$(FDEV_DART_DEFINES)` in your `flutter run` lines so an app using
[`fdev_log`](../dart/fdev_log) writes its logs for fdev. Without fdev it is
empty and changes nothing.

fdev leaves out the `help` rule and rules that run fdev itself.

> [!NOTE]
> On Windows, Makefile targets need `make` on the PATH
> (`winget install ezwinports.make`).

## Examples

### Another flavor or variant

```make
staging: ## The store app's id on the test backend
	flutter run --flavor staging -t lib/main_staging.dart $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

### A question: which backend

```make
# Which backend; empty for the one the flavor uses.
API ?=
API_DEFINE = $(if $(filter local,$(API)),--dart-define=API_URL=http://localhost:8080) \
             $(if $(filter staging,$(API)),--dart-define=API_URL=https://staging.example.com)

dev: ## Run the test app
	flutter run --flavor dev $(API_DEFINE) $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

Before `dev` runs, fdev asks **Api** with *Default*, *Local* and *Staging*,
then asks for the device. `make dev API=local` does the same without fdev.

### A store edition

```make
STORE ?=
FLAVOR = dev$(if $(filter myket,$(STORE)),Myket)$(if $(filter bazaar,$(STORE)),Bazaar)

dev: ## Run the test app
	flutter run --flavor $(FLAVOR) $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

fdev asks **Store edition**: *Google Play*, *Myket* or *Cafe Bazaar*, each
with its icon. This needs Android flavors with those names (`devMyket`,
`devBazaar`).

### A tool

```make
icons: ## Make the launcher icons
	dart run flutter_launcher_icons
```

It shows under **Tools**, and its output shows as plain text.

## fdev.yaml: full control

Use an `fdev.yaml` in the project root when the Makefile can't say what
you want: targets that aren't make rules, your own labels and icons for the
answers, details for the menu's flavor panel, or a default look for the log
viewer. When it's there, fdev uses only it.

The easiest start is what fdev already reads from your project: it's in
`.fdev/fdev.yaml` (git ignores it). Copy it to the project root and edit
it:

```sh
cp .fdev/fdev.yaml fdev.yaml
```

Every field is explained in [fdev.example.yaml](../fdev.example.yaml). A short
example:

```yaml
title: Acme Shop

groups:
  - name: Run
    targets:
      - name: dev
        desc: Test app
        run: make dev          # any shell command
        flavor: dev            # the flavor's icon and facts in the menu
        logs: true             # open the log viewer; sets FDEV_DART_DEFINES
        ask: [store, "device:android"]

asks:
  store:
    title: Store edition
    env: STORE                 # the answer arrives as $STORE
    options:
      - {label: Google Play, value: ""}
      - {label: Myket, value: myket, icon: store, color: "#00A0E3"}

flavors:
  dev:
    info:
      App: Acme Shop Dev
      Backend: test.example.com
```

The device questions are built in: `device`, `device:android` and
`device:ios` list the connected devices and set `DEVICE`.

## Check what fdev read

```sh
fdev help              # lists this project's targets, by group
cat .fdev/fdev.yaml    # everything fdev worked out: targets, questions, flavors
```

`.fdev/fdev.yaml` is written at every start, unless the project has its
own `fdev.yaml`, and fdev never reads it back. If a target lands in the
wrong group or is missing its question, it shows you what fdev understood.
