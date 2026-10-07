# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

fdev is a terminal launcher and log viewer for Flutter projects, written in Go with Charm's v2 libraries (Bubble Tea, Bubbles, Huh, Lip Gloss, imported as `charm.land/...`). It is in beta until `v1.0.0`. A Dart package in `dart/fdev_log` lets apps write structured logs that fdev shows. Contributor workflow and release steps are in [CONTRIBUTING.md](CONTRIBUTING.md); release internals are in [docs/RELEASING.md](docs/RELEASING.md).

## Commands

```sh
go build -o fdev .                      # binary in repo root (gitignored)
go vet ./... && go test ./...           # what CI runs (ubuntu + macOS)
GOOS=windows go vet ./... && GOOS=windows go build -o /dev/null .   # CI also checks Windows
go test ./internal/logview -run TestHTTPRecords      # a single test
FDEV_SHOW=1 go test ./internal/launcher -run TestScreensFit -v   # print the screens launcher tests draw
go build -ldflags "-X main.version=v0.2.0-beta.1" -o fdev .   # build with a release-like version
go run . update --local                  # build this checkout and install it over the fdev on PATH

cd dart/fdev_log && dart pub get && dart analyze && dart test   # the Dart package
```

Run the full UI without Flutter or a device, using the fake `flutter`/`dart`/`adb` in `scripts/demo/bin`:

```sh
go build -o fdev . && cd scripts/demo/project && PATH="$PWD/../bin:$PATH" ../../../fdev
```

`scripts/demo/record.sh [scene...]` re-records the README media in `docs/media` (needs `agg`). Scenes are in `scripts/demo/scenes`.

## Architecture

`main.go` dispatches the subcommands (`logs`, `wifi`, `update`, `version`). Otherwise it runs the main flow:

1. **`internal/config`: what can be run.** `config.Load` walks up from the cwd. If the project has an `fdev.yaml`, its targets are used. If not, `Discover` works them out at every start: it parses the Makefile and expands variables the way make would (`make.go`), reads flavors from Gradle `productFlavors`, the Xcode project and a Dart enum in `lib/`, and falls back to plain flutter commands. The result is written to `.fdev/fdev.yaml` for inspection only. A recipe's `NAME ?=` variables become *asks* (questions), and the values the Makefile tests them against become the answers offered. `fdev.example.yaml` documents the schema.
2. **`internal/launcher`: the Bubble Tea app.** It is a single `Model` with screens (pick → menu → ask → devices → logs). The menu is a Bubbles list, the questions are Huh forms, and the steps before the menu are in `picker.go`. `run()` hands a job to `logview.New(...)`, and the launcher embeds that model while `screen == screenLogs`. `logview.DoneMsg` brings control back. Builtin targets such as `fdev wifi` (`Target.Builtin`) are not run in the viewer. They run in the plain terminal via `tea.Exec`, because they prompt the user interactively.
3. **`internal/logview`: runs the command in a pty and renders its output.** The pty code is `term_unix.go` (creack/pty) or `term_windows.go`, so always keep the Windows build compiling. `parse.go` sorts lines by `Kind` (flutter tool output, app, other native tags) and decodes fdev records. The rest is split by feature: filters (`filter.go`), network request/response pairing and grouping (`group.go`), pinned and tracked rows (`sticky.go`), Gradle-step progress bars timed from earlier runs (`task.go`), the values window (`values.go`, `vault.go`), saving and replaying sessions (`sessions.go`, `.fdev/logs`), and animation (`motion.go`). `sanitize.go` strips escape sequences, control characters and bidi overrides from device text before anything is drawn. Keep that guarantee.
4. **`internal/state`: per-user memory.** It stores recent runs, last answers, viewer toggles and step durations in the OS user cache dir (`<cache>/fdev/<hash of root>.json` plus `ui.json`), never in the project.

Cross-cutting:
- **The log protocol is shared by Go and Dart.** Apps built with `--dart-define=FDEV_LOGS=true` (`config.DartDefine`) print `⟪fd⟫<json>` records, chunked at 300 UTF-16 units. [docs/PROTOCOL.md](docs/PROTOCOL.md) is the spec, `dart/fdev_log/lib/fdev_log.dart` writes the records and `internal/logview/parse.go` reads them. A change to any one of the three must be made in all three.
- `internal/keys` maps keys typed on non-US layouts (Persian, Arabic, Russian, …) to US positions. Compare shortcuts through it, not against raw key strings.
- `internal/theme` holds all palettes. Code resolves colour roles (`Tone`) against the theme instead of hard-coding colours. `internal/sprite` and `internal/icon` draw pixel art with half-block characters.
- **Versioning:** `main.version` and `main.releaseRepo` are set through `-ldflags` by `scripts/release.sh`. A local build falls back to Go's module pseudo-version. `internal/version` decides the channel: any semver pre-release tag (e.g. `-beta.1`) means the beta channel. `internal/update` swaps the running binary after checking the release's `checksums.txt`.

## Branches and releases

- Work branches from `beta` and goes back into `beta` through a PR. `main` only receives `beta` once a beta release has been tested (`git merge --ff-only origin/beta`). Never commit directly to `main`.
- A release is a pushed tag: `vX.Y.Z-beta.N` from `beta` makes a GitHub pre-release, and `vX.Y.Z` from `main` makes a stable release. `.github/workflows/release.yml` runs `scripts/release.sh <tag> --publish`. The release notes are the commit subjects since the previous tag, so write each subject as a user-facing sentence.
- Make one commit per feature or fix, and make sure every commit builds.

## Conventions

- Comments are plain prose that explains why or what the user sees, often with a concrete example (e.g. `// (🟡 fdev • Acme Shop v2.1.39 (439) • dev • ...)`). Every package has a doc comment that describes its role. Match that style.
- The README is in two languages (`README.md`, `README.fa.md`), and so is the contributing guide. Update both when you change user-facing behaviour.
