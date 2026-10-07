// fdev is a launcher and log viewer for Flutter projects.
//
//	fdev                    pick a target from fdev.yaml or the Makefile
//	fdev <target>           start that target straight away
//	fdev logs -- <command>  run a command (e.g. flutter run) in the log viewer
//	fdev wifi [pair]        debug an Android phone over Wi-Fi
//	fdev update [--beta|--stable] [version]
//	                        replace fdev with the newest release (or that one)
//	fdev help [command]     the commands (see cli.go), or one command's options
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
		case "-h", "--help":
			help(os.Stdout)
			return 0
		case "help":
			return runHelp(args[1:])
		case "-v", "--version", "version":
			fmt.Println("fdev", fdevversion.Describe())
			return 0
		case "logs":
			return runLogs(args[1:])
		case "wifi", "wireless":
			return runWifi(args[1:])
		case "update", "upgrade", "self-update":
			return runUpdate(args[1:])
		}
		if strings.HasPrefix(args[0], "-") {
			return unknown("option", args[0], optionFlags(globalOptions), "fdev ")
		}
	}

	cwd, _ := os.Getwd()
	cfg, err := config.Load(cwd)
	if err != nil {
		if len(args) > 0 { // a mistyped command, most likely
			return unknown("command", args[0], commandNames(), "fdev ")
		}
		fmt.Fprintln(os.Stderr, "fdev:", err)
		return 1
	}
	start := strings.Join(args, " ")
	if start != "" && cfg.Target(start) == nil {
		var known []string
		for _, t := range cfg.Targets() {
			known = append(known, t.Name)
		}
		return unknown("command or target", start, append(commandNames(), known...), "fdev ")
	}
	showTitles()
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
		case a == "-h" || a == "--help":
			commandHelp(os.Stdout, findCommand("update"))
			return 0
		case strings.HasPrefix(a, "-"):
			return unknown("option", a, optionFlags(findCommand("update").options), "fdev update ")
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

func runLogs(args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		commandHelp(os.Stdout, findCommand("logs"))
		return 0
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		commandHelp(os.Stderr, findCommand("logs"))
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
		case "-h", "--help":
			commandHelp(os.Stdout, findCommand("wifi"))
			return 0
		default:
			return unknown("option", a, []string{"pair"}, "fdev wifi ")
		}
	}
	return wifi.Run(o)
}
