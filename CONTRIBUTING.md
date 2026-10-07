# Contributing to fdev

**English** · [فارسی](CONTRIBUTING.fa.md)

Thanks for helping make fdev better. This guide covers everything from
cloning the code to shipping a release:

1. [What you need](#1-what-you-need)
2. [Get the code](#2-get-the-code)
3. [Build and run it](#3-build-and-run-it)
4. [Run the checks](#4-run-the-checks)
5. [How branches work: beta first, then main](#5-how-branches-work-beta-first-then-main)
6. [Make a change, step by step](#6-make-a-change-step-by-step)
7. [Releasing (maintainers)](#7-releasing-maintainers): **the commands to copy**
8. [Where things are](#8-where-things-are)

---

## 1. What you need

| Tool | Why | Check |
|---|---|---|
| [Go](https://go.dev/dl/) **1.27.1** or newer | fdev is written in Go | `go version` |
| Git | | `git --version` |
| [Dart](https://dart.dev/get-dart) or Flutter SDK | only if you change `dart/fdev_log` | `dart --version` |
| [GitHub CLI](https://cli.github.com) (`gh`) | optional: opening PRs, watching releases | `gh --version` |

You do **not** need Flutter, a phone or an emulator to try fdev. The repo
has a demo project with a fake `flutter` and `adb` (see [step 3](#3-build-and-run-it)).

## 2. Get the code

If you can't push to this repository, first **fork** it on GitHub (Fork
button, top right), then clone your fork:

```sh
git clone https://github.com/<your-username>/fdev.git
cd fdev
git remote add upstream https://github.com/NaderMozaffari/fdev.git
git checkout beta
```

If you can push here, clone it directly:

```sh
git clone https://github.com/NaderMozaffari/fdev.git
cd fdev
git checkout beta
```

> [!IMPORTANT]
> Work starts from the **`beta`** branch, not `main`. [Section 5](#5-how-branches-work-beta-first-then-main) explains why.

## 3. Build and run it

Build the binary in the repo root (it's in `.gitignore`, so it won't be
committed):

```sh
go build -o fdev .
./fdev version        # fdev dev·07c1a2b (built from source, with uncommitted changes)
```

### Try it without Flutter (demo project)

`scripts/demo/project` is a small Flutter project, and `scripts/demo/bin`
holds a fake `flutter`, `dart` and `adb` that print realistic output. Put
them first on your `PATH` and run fdev there:

```sh
go build -o fdev .
cd scripts/demo/project
PATH="$PWD/../bin:$PATH" ../../../fdev
```

Pick a target and you get the full launcher and log viewer, with network
requests, colours and everything, and nothing real runs. Press `q` or
`ctrl+c` to quit.

### Use your build in your own projects

To try your change in your real Flutter projects, put your build where
`fdev` is installed, so typing `fdev` anywhere runs it. Pick one of these:

**1. One line, from your clone (recommended)**

```sh
cd ~/path/to/fdev
go run . update --local
```

```
building fdev from ~/path/to/fdev...
✓ installed fdev dev·07c1a2b (built from source, with uncommitted changes) at ~/.fdev/bin/fdev
  fdev update goes back to the newest release, fdev update --beta to the newest beta
```

This builds the code in your clone and replaces the `fdev` on your PATH.
It works even if the `fdev` you have installed is an older release.
After the first time, `fdev update --local` from your clone (or
`fdev update --local ~/path/to/fdev` from anywhere) does the same.

**2. With `go build`, to a folder of your choice**

```sh
cd ~/path/to/fdev && go build -o ~/bin/fdev .
```

`go build` prints nothing when it works. Check with `fdev version`, which
should say `dev·<commit>`. `~/bin` has to be on your PATH, and **before**
any other `fdev`: `which -a fdev` lists them all, and the first one is
the one that runs.

**3. With `go install`**

```sh
cd ~/path/to/fdev && go install .
```

Go puts it in `$(go env GOPATH)/bin` (usually `~/go/bin`). The same PATH
note applies.

**4. Without installing anything**

Call the build by its full path:

```sh
cd ~/path/to/fdev && go build -o fdev .
cd ~/path/to/your/flutter_app
~/path/to/fdev/fdev
```

**Back to a release** when you're done:

```sh
fdev update            # the newest stable release
fdev update --beta     # or the newest beta
```

### Handy settings while developing

| Variable | What it does |
|---|---|
| `FDEV_REPO=owner/name` | where `fdev update` looks for releases (test updates against your fork) |
| `FDEV_CHANNEL=beta` | make `fdev update` and the install scripts use betas |
| `FDEV_EDITOR` | the editor clicked log links open in |
| `FDEV_SHOW=1` | with `go test`, prints the launcher screens the tests draw |

fdev keeps its state (recent runs, settings) in your user cache folder
(`~/Library/Caches/fdev` on macOS, `~/.cache/fdev` on Linux). Delete that
folder to start again as a first-time user.

To build with a real-looking version number (for example to test the
About page or `fdev update`):

```sh
go build -ldflags "-X main.version=v0.2.0-beta.1" -o fdev .
```

### Try project detection and `fdev init`

What fdev does depends on the folder you run it in. To try each case:

| Folder | What fdev does | Try it in |
|---|---|---|
| a Flutter project with no Makefile | offers once to write one, then opens the menu | a new `flutter create` app, or a copy of yours without its Makefile |
| a folder with a Makefile, of any kind of project | shows its targets | any folder with a small Makefile |
| a Dart package, or a folder with neither | says it recognizes no project here, and why | `dart/fdev_log`, or `/tmp` |

- `fdev init --print` shows the Makefile fdev would write, without writing it.
- Said **No** to the offer and want it again? Delete fdev's cache folder
  (above).
- In a project with the written Makefile, `make` on its own lists its commands.

## 4. Run the checks

CI runs the same commands on every push and pull request
([.github/workflows/ci.yml](.github/workflows/ci.yml)). Run them before you push:

```sh
go vet ./...
go test ./...
go build .
GOOS=windows go vet ./... && GOOS=windows go build -o /dev/null .   # does it still build for Windows?
```

If you changed the Dart package:

```sh
cd dart/fdev_log
dart pub get
dart analyze
dart test
```

If you changed what the README screenshots show, re-record them (needs
[agg](https://github.com/asciinema/agg/releases)):

```sh
scripts/demo/record.sh              # all of them
scripts/demo/record.sh hero themes  # just these
```

## 5. How branches work: beta first, then main

```
 your-branch ──PR──▶  beta  ──tested, no problems──▶  main
                       │                               │
                 v0.3.0-beta.1                       v0.3.0
                 (pre-release)                  (stable release)
```

| Branch | What's on it | Who gets it |
|---|---|---|
| `beta` | the newest work, still being tested | beta users (`fdev update --beta`) |
| `main` | only what has been tested on beta | everyone; the install scripts in the README come from `main` |

The rules:

1. **Every change starts as a branch from `beta`** and goes back into
   `beta` with a pull request. Nobody commits straight to `main`.
2. From `beta` the maintainer releases a **beta version**
   (`v0.3.0-beta.1`), and people use it for a while.
3. If something is broken, the fix goes into `beta` too, and the next beta
   ships (`v0.3.0-beta.2`).
4. When the beta has no known problems, **`beta` is merged into `main`**
   and the **stable version** (`v0.3.0`) is released from `main`.

Since `main` only ever gets what `beta` already has, the two branches
never drift apart.

## 6. Make a change, step by step

**1. Start from the newest `beta`:**

```sh
git checkout beta
git pull                       # with a fork: git pull upstream beta
git checkout -b fix/wifi-timeout
```

Name the branch after what it does: `feature/…` for something new,
`fix/…` for a bug, `docs/…` for docs.

**2. Write the code**, then build, try it ([step 3](#3-build-and-run-it))
and run the checks ([step 4](#4-run-the-checks)).

A new command or option goes in two places: `main.go` runs it, and the
table in `cli.go` gives its name, options and help, so that `fdev help`
lists it and a typo of it gets a suggestion. Then add it to the command
list in both READMEs.

**3. Commit.** Keep each feature or fix in its own commit, and make sure
every commit builds on its own. Write the subject as a short sentence about
what changes for the user, because **the release notes are made from the
commit subjects**:

```sh
git add -A
git commit -m "wifi: give up pairing after 30 seconds"
```

**4. Push and open a pull request into `beta`:**

```sh
git push -u origin fix/wifi-timeout
gh pr create --base beta --fill
```

Or open it on GitHub. In both cases, **set the base branch to `beta`**, not
`main`. In the description, say what changed, why, and how you tested it
(a screenshot or GIF helps for anything you can see).

**5. Review.** CI has to pass. Push more commits to the same branch if
changes are asked for, and the PR updates itself.

**6. Done.** Once it's merged, your change ships in the next beta.

### Keep your branch up to date

If `beta` moved on while you were working:

```sh
git fetch origin               # with a fork: git fetch upstream
git rebase origin/beta         # with a fork: git rebase upstream/beta
git push --force-with-lease
```

### Reporting bugs and ideas

Open an [issue](https://github.com/NaderMozaffari/fdev/issues) with your
OS, `fdev version`, what you did, what you expected and what happened.
For a bigger feature, open an issue first so we can agree on the approach
before you write it.

---

## 7. Releasing (maintainers)

> **Shortcut:** a release is just **a git tag pushed to GitHub**. The
> [Release workflow](.github/workflows/release.yml) does the rest: it runs
> the tests, builds fdev for macOS, Linux and Windows (amd64 and arm64),
> and publishes the GitHub release with the archives, checksums and install
> scripts. The release notes are the commit subjects since the last tag.
> More detail: [docs/RELEASING.md](docs/RELEASING.md).

### Picking the version number

Versions follow `vMAJOR.MINOR.PATCH` ([semver](https://semver.org)):

| What changed | Example | Next version |
|---|---|---|
| only bug fixes | `v0.3.0` → | `v0.3.1` |
| new features | `v0.3.0` → | `v0.4.0` |
| something that breaks how people use it (after 1.0) | `v1.4.2` → | `v2.0.0` |

A beta adds `-beta.N` to the version it leads to: `v0.4.0-beta.1`,
`v0.4.0-beta.2`, …, and then `v0.4.0`. To see the last tag:

```sh
git fetch --tags
git describe --tags --abbrev=0
```

### A. Release a beta (from `beta`)

```sh
git checkout beta
git pull
go test ./...                          # one last check

git tag v0.4.0-beta.1
git push origin v0.4.0-beta.1
```

Watch it build and check the result:

```sh
gh run watch                           # pick the "Release" run
gh release view v0.4.0-beta.1
```

Install it on your own machine and use it:

```sh
fdev update v0.4.0-beta.1              # or: fdev update --beta
fdev version                           # fdev v0.4.0-beta.1 (beta)
```

Found a problem? Fix it with a PR into `beta` (section 6), then release the
next beta: `v0.4.0-beta.2`.

### B. Release a stable version (merge `beta` into `main`)

Once the last beta has run for a while with no problems:

```sh
# 1. bring main up to beta
git checkout main
git pull
git merge --ff-only origin/beta
git push origin main

# 2. tag the stable version on main
git tag v0.4.0
git push origin v0.4.0
```

`--ff-only` just moves `main` forward to where `beta` is. If Git refuses,
`main` has a commit that `beta` doesn't (someone committed straight to
`main`). Run `git merge origin/beta` instead, push, and then bring `beta`
back in line (`git checkout beta && git merge main && git push`).

If you prefer to do it through a pull request on GitHub:

```sh
gh pr create --base main --head beta --title "Release v0.4.0" --fill
# merge it on GitHub, then:
git checkout main && git pull
git tag v0.4.0 && git push origin v0.4.0
```

> [!NOTE]
> Until `v1.0.0`, fdev ships as betas only (`v0.x.y-beta.n`), so step **A**
> is the usual release. Still merge `beta` into `main` (step B, part 1)
> once a beta has proved itself, because the install scripts in the README
> are read from `main`.

### C. An urgent fix for a stable version (hotfix)

When a stable release has a bug that can't wait for the next beta:

```sh
git checkout main && git pull
git checkout -b fix/crash-on-start
# fix, commit, then a PR into main:  gh pr create --base main --fill
# after it's merged:
git checkout main && git pull
git tag v0.4.1 && git push origin v0.4.1

# and bring the fix back into beta
git checkout beta && git pull
git merge main
git push origin beta
```

### If a release goes wrong

If the Release workflow failed, or you tagged the wrong commit, remove the
tag (and the release, if one was made), fix the problem and tag again:

```sh
gh release delete v0.4.0-beta.1 --yes --cleanup-tag   # if a release was published
git tag -d v0.4.0-beta.1                              # the local tag
git push origin :refs/tags/v0.4.0-beta.1              # the tag on GitHub, if still there
```

Do this only for a release nobody has installed yet. Otherwise, release
the next version instead.

To change a release's channel later:

```sh
gh release edit v0.3.2 --prerelease                     # make it a beta
gh release edit v1.0.0 --prerelease=false --latest      # make it stable
```

### Cheat sheet

| I want to… | Commands |
|---|---|
| release a beta | `git checkout beta && git pull` → `git tag vX.Y.Z-beta.N` → `git push origin vX.Y.Z-beta.N` |
| release stable | `git checkout main && git pull && git merge --ff-only origin/beta && git push origin main` → `git tag vX.Y.Z` → `git push origin vX.Y.Z` |
| see the last version | `git fetch --tags && git describe --tags --abbrev=0` |
| watch the release build | `gh run watch` |
| see all releases | `gh release list` |
| build the archives locally, without publishing | `scripts/release.sh vX.Y.Z-beta.N` (they land in `dist/`) |

---

## 8. Where things are

| Path | What's in it |
|---|---|
| `main.go` | runs the commands (`fdev`, `logs`, `wifi`, `init`, `update`, `channel`, `version`, `help`) |
| `cli.go` | the commands' names, options and help: `fdev help`, and the suggestions for typos |
| `init.go` | `fdev init`, the offer to write a Makefile, and the messages when there's no project |
| `internal/launcher` | the menu: targets, flavors, questions, progress bar |
| `internal/logview` | the log viewer: parsing `flutter run` output, filters, the network view |
| `internal/config` | finding the project, reading `fdev.yaml` and the Makefile, and writing one (`init.go`; its test checks fdev reads back what it writes) |
| `docs/CUSTOMIZE.md` | the guide for users to the Makefile and `fdev.yaml`: update it when what fdev reads changes |
| `internal/theme`, `internal/sprite`, `internal/icon` | colours and themes, the animations, app icons |
| `internal/devices`, `internal/wifi` | finding devices, Wi-Fi debugging |
| `internal/update`, `internal/version` | `fdev update` and the version and channel logic |
| `internal/state` | what fdev remembers between runs |
| `internal/editor`, `internal/vscode`, `internal/keys` | opening log links in an editor, VS Code integration, key bindings |
| `dart/fdev_log` | the Dart package apps log with ([format](docs/PROTOCOL.md)) |
| `scripts/install.sh`, `scripts/install.ps1` | the install scripts |
| `scripts/release.sh` | builds and publishes a release |
| `scripts/demo` | the demo project, fake tools, and README recordings |
| `fdev.example.yaml` | an annotated example config |

By contributing, you agree that your work is released under the project's
[MIT license](LICENSE).
