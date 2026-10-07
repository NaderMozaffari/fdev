package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/NaderMozaffari/fdev/internal/config"
)

// command is one of fdev's own commands, for `fdev help` and for telling a
// mistyped one apart from a target.
type command struct {
	name     string
	aliases  []string
	args     string // as the help shows it: [pair]
	summary  string
	about    string // more, for `fdev help <name>`
	options  []option
	examples []string
}

type option struct{ flag, desc string }

var commands = []command{
	{
		name: "logs", args: "-- <command>",
		summary: "run a command (e.g. flutter run) in the log viewer",
		about: "Runs the command in the log viewer, with FDEV_DART_DEFINES set to\n" +
			"--dart-define=FDEV_LOGS=true for an app that writes fdev's structured logs.",
		examples: []string{"fdev logs -- flutter run -d emulator-5554 --dart-define=FDEV_LOGS=true"},
	},
	{
		name: "wifi", aliases: []string{"wireless"}, args: "[pair]",
		summary: "debug an Android phone over Wi-Fi",
		about:   "Connects the phones paired before, or pairs one with a QR code or its pairing code.",
		options: []option{{"pair", "skip straight to pairing a new phone"}},
	},
	{
		name: "update", aliases: []string{"upgrade", "self-update"}, args: "[version]",
		summary: "replace fdev with the newest release of its channel",
		about: "A beta updates to the newest beta (or a newer stable release), a stable\n" +
			"release to the newest stable one. A version installs that one.",
		options: []option{
			{"--beta", "betas too"},
			{"--stable", "stable releases only"},
			{"--local [folder]", "build fdev from its source (a clone, here by default) and install that"},
		},
		examples: []string{
			"fdev update",
			"fdev update --beta",
			"fdev update v0.2.0-beta.1",
			"go run . update --local            # in a clone of fdev: try your changes everywhere",
			"fdev update --local ~/code/fdev",
		},
	},
	{name: "version", summary: "print fdev's version"},
	{name: "help", args: "[command]", summary: "these commands, or one command's options"},
}

// globalOptions are what `fdev -x` takes.
var globalOptions = []option{
	{"-h, --help", "these commands"},
	{"-v, --version", "fdev's version"},
}

const environment = `  FDEV_EDITOR   command that opens a file at a line, e.g. 'code -g {file}:{line}:{col}'
  FDEV_REPO     owner/name of the repository fdev update downloads from
  FDEV_CHANNEL  stable or beta: the channel fdev update keeps to
  GH_TOKEN      a GitHub token for fdev update, when that repository is private
                (or log in with the GitHub CLI: gh auth login)`

func findCommand(name string) *command {
	for i, c := range commands {
		if c.name == name || slices.Contains(c.aliases, name) {
			return &commands[i]
		}
	}
	return nil
}

var (
	bold    = lipgloss.NewStyle().Bold(true)
	heading = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Magenta)
	name    = lipgloss.NewStyle().Foreground(lipgloss.Cyan)
	faint   = lipgloss.NewStyle().Faint(true)
	red     = lipgloss.NewStyle().Foreground(lipgloss.Red)
)

// columns lines up rows of a name and what it does.
func columns(rows [][2]string) string {
	w := 0
	for _, r := range rows {
		w = max(w, lipgloss.Width(r[0]))
	}
	var b strings.Builder
	for _, r := range rows {
		pad := strings.Repeat(" ", w-lipgloss.Width(r[0])+3)
		b.WriteString("  " + name.Render(r[0]) + pad + r[1] + "\n")
	}
	return b.String()
}

// help prints every command, and the targets of the project fdev is in.
func help(w io.Writer) {
	var b strings.Builder
	b.WriteString(bold.Render("fdev") + ": a launcher and log viewer for Flutter projects\n\n")
	b.WriteString(heading.Render("Usage") + "\n")
	b.WriteString(columns([][2]string{
		{"fdev", "the menu: pick a target and run it"},
		{"fdev <target>", "start that target straight away"},
		{"fdev <command> [options]", "one of the commands below"},
	}))
	b.WriteString(faint.Render("  fdev finds the project by looking up for fdev.yaml, a Makefile or pubspec.yaml.") + "\n")
	b.WriteString("\n" + heading.Render("Commands") + "\n")
	var rows [][2]string
	for _, c := range commands {
		rows = append(rows, [2]string{strings.TrimSpace(c.name + " " + c.args), c.summary})
	}
	b.WriteString(columns(rows))
	b.WriteString("\n" + heading.Render("Options") + "\n")
	rows = nil
	for _, o := range globalOptions {
		rows = append(rows, [2]string{o.flag, o.desc})
	}
	b.WriteString(columns(rows))
	if cwd, err := os.Getwd(); err == nil {
		if cfg, err := config.Load(cwd); err == nil {
			b.WriteString(targets(cfg))
		}
	}
	b.WriteString("\n" + heading.Render("Environment") + "\n" + environment + "\n")
	b.WriteString("\n" + faint.Render("fdev help <command> shows a command's options.") + "\n")
	lipgloss.Fprint(w, b.String())
}

// targets lists the project's targets, a row of names for each group.
func targets(cfg *config.Config) string {
	var groups []string
	byGroup := map[string][]string{}
	for _, t := range cfg.Targets() {
		if _, ok := byGroup[t.Group]; !ok {
			groups = append(groups, t.Group)
		}
		n := t.Name
		if strings.Contains(n, " ") {
			n = `"` + n + `"`
		}
		byGroup[t.Group] = append(byGroup[t.Group], n)
	}
	if len(groups) == 0 {
		return ""
	}
	width := 80
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 20 {
		width = w
	}
	gw := 0
	for _, g := range groups {
		gw = max(gw, len(g))
	}
	var b strings.Builder
	b.WriteString("\n" + heading.Render("Targets in "+cfg.Title) + faint.Render(" ("+cfg.Source+")") + "\n")
	for _, g := range groups {
		indent := 2 + gw + 3
		line, col := "  "+g+strings.Repeat(" ", gw-len(g)+3), indent
		for i, n := range byGroup[g] {
			if i > 0 && col+len(n) > width {
				b.WriteString(strings.TrimRight(line, " ") + "\n")
				line, col = strings.Repeat(" ", indent), indent
			}
			line += name.Render(n) + "  "
			col += len(n) + 2
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	return b.String()
}

// commandHelp prints one command: what it takes and does.
func commandHelp(w io.Writer, c *command) {
	var b strings.Builder
	b.WriteString(heading.Render("Usage") + "\n  " + name.Render(strings.TrimSpace("fdev "+c.name+" "+c.args)) + "\n\n")
	b.WriteString(strings.ToUpper(c.summary[:1]) + c.summary[1:] + ".\n")
	if c.about != "" {
		b.WriteString(c.about + "\n")
	}
	if len(c.aliases) > 0 {
		b.WriteString(faint.Render("Also: "+strings.Join(c.aliases, ", ")) + "\n")
	}
	if len(c.options) > 0 {
		var rows [][2]string
		for _, o := range c.options {
			rows = append(rows, [2]string{o.flag, o.desc})
		}
		b.WriteString("\n" + heading.Render("Options") + "\n" + columns(rows))
	}
	if len(c.examples) > 0 {
		b.WriteString("\n" + heading.Render("Examples") + "\n")
		for _, e := range c.examples {
			b.WriteString("  " + e + "\n")
		}
	}
	lipgloss.Fprint(w, b.String())
}

// runHelp is `fdev help [command]`.
func runHelp(args []string) int {
	if len(args) == 0 {
		help(os.Stdout)
		return 0
	}
	if c := findCommand(args[0]); c != nil {
		commandHelp(os.Stdout, c)
		return 0
	}
	return unknown("command", args[0], commandNames(), "fdev help ")
}

func commandNames() []string {
	var out []string
	for _, c := range commands {
		out = append(out, c.name)
		out = append(out, c.aliases...)
	}
	return out
}

// optionFlags is the flags opts name: "-h, --help" is -h and --help,
// "--local [folder]" is --local.
func optionFlags(opts []option) []string {
	var out []string
	for _, o := range opts {
		for f := range strings.SplitSeq(o.flag, ",") {
			out = append(out, strings.Fields(f)[0])
		}
	}
	return out
}

// unknown says what wasn't a kind ("command", "option", ...), suggests the
// closest of known with prefix before it, and returns the exit code.
func unknown(kind, what string, known []string, prefix string) int {
	lipgloss.Fprintln(os.Stderr, red.Render(fmt.Sprintf("fdev: no %s %q", kind, what)))
	if s := suggest(what, known); len(s) > 0 {
		lipgloss.Fprintln(os.Stderr, "\nDid you mean?")
		for _, v := range s {
			if strings.Contains(v, " ") {
				v = `"` + v + `"`
			}
			lipgloss.Fprintln(os.Stderr, "  "+name.Render(prefix+v))
		}
	}
	lipgloss.Fprintln(os.Stderr, "\n"+faint.Render("fdev help lists the commands and this project's targets."))
	return 2
}

// suggest is the known words closest to word: a few typos away, or
// starting with it.
func suggest(word string, known []string) []string {
	type hit struct {
		s string
		d int
	}
	var hits []hit
	low := strings.ToLower(word)
	limit := 1 + len([]rune(word))/4
	for _, k := range known {
		d := distance(low, strings.ToLower(k))
		switch {
		case d <= limit:
		case len(low) >= 2 && strings.HasPrefix(strings.ToLower(k), low):
			d = limit + 1
		default:
			continue
		}
		if !slices.ContainsFunc(hits, func(h hit) bool { return h.s == k }) {
			hits = append(hits, hit{k, d})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return a.d - b.d })
	var out []string
	for i, h := range hits {
		if i == 3 {
			break
		}
		out = append(out, h.s)
	}
	return out
}

// distance is the edit distance between a and b, a swap of two letters
// counting as one edit.
func distance(a, b string) int {
	x, y := []rune(a), []rune(b)
	d := make([][]int, len(x)+1)
	for i := range d {
		d[i] = make([]int, len(y)+1)
		d[i][0] = i
	}
	for j := range y {
		d[0][j+1] = j + 1
	}
	for i := 1; i <= len(x); i++ {
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && x[i-1] == y[j-2] && x[i-2] == y[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(x)][len(y)]
}
