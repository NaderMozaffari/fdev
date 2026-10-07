package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/NaderMozaffari/fdev/internal/config"
	"github.com/NaderMozaffari/fdev/internal/state"
	fdevversion "github.com/NaderMozaffari/fdev/internal/version"
)

// runInit is `fdev init`: a Makefile for a project that has none.
func runInit(args []string) int {
	show, force := false, false
	for _, a := range args {
		switch a {
		case "--print":
			show = true
		case "--force":
			force = true
		case "-h", "--help":
			commandHelp(os.Stdout, findCommand("init"))
			return 0
		default:
			return unknown("option", a, optionFlags(findCommand("init").options), "fdev init ")
		}
	}
	cwd, _ := os.Getwd()
	cfg, err := config.Load(cwd)
	if err != nil {
		if err == config.ErrNoProject {
			return noProject(cwd)
		}
		fmt.Fprintln(os.Stderr, "fdev:", err)
		return 1
	}
	if !cfg.Flutter {
		lipgloss.Fprintln(os.Stderr, red.Render("fdev init: "+fdevversion.Home(cfg.Root)+" isn't a Flutter project"))
		lipgloss.Fprintln(os.Stderr, "fdev init writes Makefiles for Flutter projects. This one has a Makefile, and fdev shows its targets.")
		return 1
	}
	text, _ := config.Makefile(cfg.Root)
	if show {
		fmt.Print(text)
		return 0
	}
	path := filepath.Join(cfg.Root, "Makefile")
	if _, err := os.Stat(path); err == nil && !force {
		lipgloss.Fprintln(os.Stderr, red.Render("fdev init: "+fdevversion.Home(path)+" is there already, and fdev reads its targets."))
		lipgloss.Fprintln(os.Stderr, faint.Render("  fdev init --print shows the one fdev would write; --force replaces it."))
		return 1
	}
	if err := writeMakefile(cfg.Root); err != nil {
		fmt.Fprintln(os.Stderr, "fdev init:", err)
		return 1
	}
	for _, f := range config.FileNames {
		if _, err := os.Stat(filepath.Join(cfg.Root, f)); err == nil {
			lipgloss.Println(faint.Render("  " + f + " is there too, and fdev uses it instead of the Makefile."))
		}
	}
	return 0
}

// writeMakefile writes the project's Makefile and says what's in it.
func writeMakefile(root string) error {
	text, targets := config.Makefile(root)
	path := filepath.Join(root, "Makefile")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	lipgloss.Println(lipgloss.NewStyle().Foreground(lipgloss.Green).Render("✓ wrote "+fdevversion.Home(path)) +
		faint.Render(fmt.Sprintf(" (%d targets)", len(targets))))
	list := strings.Join(targets[:min(len(targets), 8)], " · ")
	if len(targets) > 8 {
		list += " · …"
	}
	lipgloss.Println("  " + list)
	lipgloss.Println("\n  Make it yours, add targets and questions: " + name.Render(config.GuideURL))
	lipgloss.Println(faint.Render("  (فارسی: " + config.GuideURLFa + ")"))
	return nil
}

// offerMakefile asks, once per project, to write a Makefile for a Flutter
// project that has none, so its menu can be changed (and run with make).
// It returns the config to go on with: read again from the Makefile, if
// one was written.
func offerMakefile(cfg *config.Config, st *state.State) *config.Config {
	if st.NoMakefile || !cfg.NeedsMakefile() || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return cfg
	}
	what := "a run and a build of the app"
	var flavors []string
	for _, t := range cfg.Targets() {
		if t.Flavor != "" && !slices.Contains(flavors, t.Flavor) {
			flavors = append(flavors, t.Flavor)
		}
	}
	if len(flavors) > 0 {
		what = "a run and a build of each flavor (" + strings.Join(flavors, ", ") + ")"
	}
	write := true
	err := huh.NewConfirm().
		Title("This Flutter project has no Makefile").
		Description("fdev works its menu out of the project at every start. It can write a\n" +
			"Makefile in " + fdevversion.Home(cfg.Root) + " instead: " + what + ",\n" +
			"and the usual tools. It's a plain file anyone can edit to change the menu\n" +
			"or add commands, and make runs them too.\n\n" +
			"How to edit it: " + config.GuideURL + "\n\n" +
			"Write the Makefile?").
		Affirmative("Yes, write it").
		Negative("No, don't ask again").
		Value(&write).
		Run()
	if err != nil { // ctrl+c: ask next time
		return cfg
	}
	if !write {
		st.NoMakefile = true
		st.Save()
		lipgloss.Println(faint.Render("fdev init writes it any time. How to set fdev up: " + config.GuideURL))
		return cfg
	}
	if err := writeMakefile(cfg.Root); err != nil {
		fmt.Fprintln(os.Stderr, "fdev:", err)
		return cfg
	}
	fmt.Print("\n  enter opens the menu ")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	if again, err := config.Load(cfg.Root); err == nil {
		return again
	}
	return cfg
}

// noProject says fdev knows no project at dir, what it looked for, and
// where to go: the Flutter projects below dir, if any.
func noProject(dir string) int {
	e := os.Stderr
	lipgloss.Fprintln(e, red.Render("fdev: no project recognized here"))
	lipgloss.Fprintln(e, "\nfdev looked in "+fdevversion.Home(dir)+" and the folders above it for")
	lipgloss.Fprintln(e, "  • a Flutter project: pubspec.yaml with the Flutter SDK, or")
	lipgloss.Fprintln(e, "  • a Makefile, in a project of any kind: fdev shows its targets in a menu,")
	lipgloss.Fprintln(e, "and found neither.")
	found := config.FindProjects(dir)
	switch pub := nearestPubspec(dir); {
	case pub != "":
		lipgloss.Fprintln(e, "\nThis is a Dart package, not a Flutter project ("+fdevversion.Home(pub)+").")
		lipgloss.Fprintln(e, "fdev runs Flutter projects for now. To use it here, add a Makefile with the")
		lipgloss.Fprintln(e, "package's commands (dart test, dart analyze, …): fdev shows them in its menu.")
	case len(found) > 0:
		lipgloss.Fprintln(e, "\nFlutter projects below this folder:")
		for _, p := range found {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				rel = p
			}
			if strings.ContainsAny(rel, " '\"") {
				rel = `"` + rel + `"`
			}
			lipgloss.Fprintln(e, "  "+name.Render("cd "+rel+" && fdev"))
		}
	default:
		lipgloss.Fprintln(e, "\nRun fdev in your project's folder.")
	}
	lipgloss.Fprintln(e, "\n"+faint.Render("How to set fdev up for a project: "+config.GuideURL))
	return 1
}

// nearestPubspec is the pubspec.yaml in dir or above it, when it is a
// Dart package's rather than a Flutter project's.
func nearestPubspec(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if p := filepath.Join(d, "pubspec.yaml"); fileExists(p) {
			if config.IsFlutter(d) {
				return ""
			}
			return p
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// noTargets says the project's Makefile (or fdev.yaml) has nothing to run.
func noTargets(cfg *config.Config) int {
	file := filepath.Join(cfg.Root, "Makefile")
	for _, f := range config.FileNames {
		if p := filepath.Join(cfg.Root, f); fileExists(p) {
			file = p
		}
	}
	lipgloss.Fprintln(os.Stderr, red.Render("fdev: nothing to run in "+fdevversion.Home(file)))
	if filepath.Base(file) == "Makefile" {
		lipgloss.Fprintln(os.Stderr, "\nfdev shows a Makefile's targets: a name and a colon, and on the next line,")
		lipgloss.Fprintln(os.Stderr, "after a TAB, the command it runs. This one has none yet:")
		lipgloss.Fprintln(os.Stderr, "\n  "+name.Render("test: ## Run the tests")+"\n  "+name.Render("\tgo test ./..."))
	}
	lipgloss.Fprintln(os.Stderr, "\n"+faint.Render("How to write them, with examples: "+config.GuideURL))
	return 1
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
