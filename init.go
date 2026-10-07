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

// offerMakefile asks, once per project, to write a Makefile for a project
// that has none, so its targets can be changed (and run with make). It
// returns the config to go on with: read again from the Makefile, if one
// was written.
func offerMakefile(cfg *config.Config, st *state.State) *config.Config {
	if st.NoMakefile || !cfg.NeedsMakefile() || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return cfg
	}
	flavors := "the app"
	if len(cfg.Flavors) > 0 {
		var names []string
		for _, t := range cfg.Targets() {
			if t.Flavor != "" && !slices.Contains(names, t.Flavor) {
				names = append(names, t.Flavor)
			}
		}
		flavors = "the flavors " + strings.Join(names, ", ")
	}
	write := true
	err := huh.NewConfirm().
		Title("Write a Makefile for " + cfg.Title + "?").
		Description("This project has no Makefile, so fdev works its targets out each time.\n" +
			"A Makefile in " + fdevversion.Home(cfg.Root) + " lets you change them and add your own:\n" +
			"running and building " + flavors + ", and the usual tools. make runs them too.\n" +
			"fdev init writes it any time.").
		Affirmative("Write it").
		Negative("No, don't ask again").
		Value(&write).
		Run()
	if err != nil { // ctrl+c: ask next time
		return cfg
	}
	if !write {
		st.NoMakefile = true
		st.Save()
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

// noProject explains that fdev runs in a Flutter project, and points to
// the ones below dir, if any.
func noProject(dir string) int {
	lipgloss.Fprintln(os.Stderr, red.Render("fdev: no Flutter project here"))
	lipgloss.Fprintln(os.Stderr, "fdev runs in a Flutter project: the folder with pubspec.yaml, or one inside it.")
	if found := config.FindProjects(dir); len(found) > 0 {
		lipgloss.Fprintln(os.Stderr, "\nFlutter projects below this folder:")
		for _, p := range found {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				rel = p
			}
			if strings.ContainsAny(rel, " '\"") {
				rel = `"` + rel + `"`
			}
			lipgloss.Fprintln(os.Stderr, "  "+name.Render("cd "+rel+" && fdev"))
		}
	} else {
		lipgloss.Fprintln(os.Stderr, "Go to your project's folder (cd path/to/your_app) and run fdev there.")
	}
	lipgloss.Fprintln(os.Stderr, "\n"+faint.Render("How fdev reads a project, and how to set it up: "+config.GuideURL))
	return 1
}
