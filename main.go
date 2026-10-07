// fdev is a launcher and log viewer for Flutter projects.
//
//	fdev                    pick a target from fdev.yaml or the Makefile
//	fdev <target>           start that target straight away
//	fdev logs -- <command>  run a command (e.g. flutter run) in the log viewer
//	fdev wifi [pair]        debug an Android phone over Wi-Fi
//	fdev update [--beta|--stable] [version]
//	                        replace fdev with the newest release (or that one)
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/launcher"
	"github.com/NaderMozaffari/fdev/internal/state"
	"github.com/NaderMozaffari/fdev/internal/update"
	fdevversion "github.com/NaderMozaffari/fdev/internal/version"
	"github.com/NaderMozaffari/fdev/internal/vscode"
	"github.com/NaderMozaffari/fdev/internal/wifi"
)

// version is set with -ldflags "-X main.version=...". Without it, the module
// version Go stamps into the binary is used (a tag for go install @version, a
// pseudo-version for a local build).
var version = ""

// releaseRepo is where `fdev update` looks for releases; the release script
// sets it to the repository it publishes to. FDEV_REPO overrides it.
var releaseRepo = "NaderMozaffari/fdev"

func appVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

const usage = `fdev: a launcher and log viewer for Flutter projects

Usage:
  fdev                    pick a target from fdev.yaml or the Makefile
  fdev <target>           start that target straight away
  fdev logs -- <command>  run a command (e.g. flutter run) in the log viewer
  fdev wifi [pair]        debug an Android phone over Wi-Fi: connect the phones
                          paired before, or pair one with a QR code or its
                          pairing code (pair: skip straight to pairing)
  fdev update [version]   replace fdev with the newest release of its channel
                          (or that version): stable, or beta when fdev is a
                          beta; --beta or --stable picks the channel
  fdev version

fdev works the targets out of the project at every start (its Makefile,
android/, ios/, lib/) and writes what it found to .fdev/fdev.yaml, which git
ignores. An fdev.yaml in the project root replaces that (see fdev.example.yaml).

Environment:
  FDEV_EDITOR  command that opens a file at a line, e.g. 'code -g {file}:{line}:{col}'
  FDEV_REPO    owner/name of the repository fdev update downloads from
  FDEV_CHANNEL stable or beta: the channel fdev update keeps to
  GH_TOKEN     a GitHub token for fdev update, when that repository is private
               (or log in with the GitHub CLI: gh auth login)
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	update.Cleanup()
	fdevversion.Current, fdevversion.Repo = appVersion(), releaseRepo
	if r := os.Getenv("FDEV_REPO"); r != "" {
		fdevversion.Repo = r
	}
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			fmt.Print(usage)
			return 0
		case "-v", "--version", "version":
			fmt.Println("fdev", appVersion()+channelNote(appVersion()))
			return 0
		case "logs":
			return runLogs(args[1:])
		case "wifi", "wireless":
			return runWifi(args[1:])
		case "update", "upgrade", "self-update":
			return runUpdate(args[1:])
		}
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
	}

	cwd, _ := os.Getwd()
	cfg, err := config.Load(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fdev:", err)
		return 1
	}
	showTitles()
	start := strings.Join(args, " ")
	m := launcher.New(cfg, state.Load(cfg.Root), start)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "fdev:", err)
		return 1
	}
	return 0
}

func runUpdate(args []string) int {
	repo := releaseRepo
	if r := os.Getenv("FDEV_REPO"); r != "" {
		repo = r
	}
	// A beta updates to the newest beta (or a newer release), a release to
	// the newest release.
	beta := fdevversion.Channel(appVersion()) == fdevversion.Beta
	switch strings.ToLower(os.Getenv("FDEV_CHANNEL")) {
	case fdevversion.Beta:
		beta = true
	case fdevversion.Stable:
		beta = false
	}
	want := ""
	for _, a := range args {
		switch {
		case a == "--beta":
			beta = true
		case a == "--stable":
			beta = false
		case strings.HasPrefix(a, "-"):
			fmt.Fprintln(os.Stderr, "usage: fdev update [--beta|--stable] [version]")
			return 2
		default:
			want = a
			if !strings.HasPrefix(want, "v") {
				want = "v" + want
			}
		}
	}
	err := update.Run(update.Options{Repo: repo, Current: appVersion(), Version: want, Beta: beta, Out: os.Stdout})
	if err != nil {
		fmt.Fprintln(os.Stderr, "fdev update:", err)
		return 1
	}
	return 0
}

// channelNote marks a beta: " (beta)".
func channelNote(v string) string {
	if fdevversion.Channel(v) == fdevversion.Beta {
		return " (beta)"
	}
	return ""
}

func runLogs(args []string) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fdev logs -- <command> [args...]")
		return 2
	}
	cwd, _ := os.Getwd()
	cfg, err := config.Load(cwd)
	if err != nil {
		cfg = &config.Config{Root: cwd}
	}
	if cfg.Title == "" {
		cfg.Title = "fdev"
	}
	showTitles()
	m := launcher.NewLogsOnly(cfg, args, state.Load(cfg.Root))
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "fdev:", err)
		return 1
	}
	return m.Code
}

// showTitles has VS Code name the terminal's tab after fdev's title
// (🟡 fdev • Acme Shop v2.1.39 (439) • dev • SM-S918B • bazaar), not
// "fdev". The note stays on the screen fdev comes back to.
func showTitles() {
	if path, err := vscode.ShowTitles(); err != nil {
		fmt.Fprintln(os.Stderr, "fdev: can't name VS Code's terminal tabs:", err)
	} else if path != "" {
		fmt.Fprintf(os.Stderr, "fdev: VS Code names its terminal tabs after their titles now (\"terminal.integrated.tabs.title\": \"${sequence}\" in %s)\n", path)
	}
}

func runWifi(args []string) int {
	o := wifi.Options{In: os.Stdin, Out: os.Stdout}
	for _, a := range args {
		switch a {
		case "pair":
			o.Pair = true
		case "--wait": // the launcher runs it so
			o.Wait = true
		default:
			fmt.Fprintln(os.Stderr, "usage: fdev wifi [pair]")
			return 2
		}
	}
	return wifi.Run(o)
}
