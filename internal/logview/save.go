package logview

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/NaderMozaffari/fdev/internal/theme"
)

// themeless is for label names, which don't depend on the colors.
var themeless = theme.New(true)

func stripANSI(s string) string { return ansi.Strip(s) }

// keepSessions is how many sessions stay in the logs directory.
const keepSessions = 30

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// logsDir makes the directory for saved logs, with a .gitignore of `*` in
// it: git then sees neither the logs nor the .gitignore, and no tracked
// file of the project changes.
func logsDir(root, dir string) (string, error) {
	path := dir
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, dir)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	ignore := filepath.Join(path, ".gitignore")
	if _, err := os.Stat(ignore); os.IsNotExist(err) {
		note := "# Logs saved by fdev; nothing here is committed.\n*\n"
		if err := os.WriteFile(ignore, []byte(note), 0o600); err != nil {
			return "", err
		}
	}
	return path, nil
}

// sessionBase is the file name, without extension, of this run's logs.
func (m *Model) sessionBase() string {
	name := unsafeName.ReplaceAllString(m.o.Target, "-")
	if name == "" {
		name = "logs"
	}
	return m.started.Format("2006-01-02_15-04-05") + "_" + strings.Trim(name, "-")
}

// startRecording opens the raw log, which gets every output line as it
// arrives (so it survives a crash of fdev).
func (m *Model) startRecording() error {
	if m.raw != nil {
		return nil
	}
	dir, err := logsDir(m.o.Root, m.look.Dir)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, m.sessionBase()+".raw.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	m.raw = f
	for _, e := range m.entries { // what came before recording started
		for _, r := range e.Raw {
			fmt.Fprintln(f, rawLine(e.Time, r))
		}
	}
	prune(dir)
	return nil
}

// save writes the readable log: every entry, whatever is hidden, with its
// stack trace, headers and body. It returns the path, relative to the root.
func (m *Model) save() (string, error) {
	dir, err := logsDir(m.o.Root, m.look.Dir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, m.sessionBase()+".log")
	var b strings.Builder
	fmt.Fprintf(&b, "# %s · %s · started %s\n", m.o.Title, m.o.Command, m.started.Format(time.RFC3339))
	if m.o.Note != "" {
		fmt.Fprintf(&b, "# %s\n", m.o.Note)
	}
	b.WriteString("\n")
	for _, e := range m.entries {
		writeEntry(&b, e)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	prune(dir)
	if rel, err := filepath.Rel(m.o.Root, path); err == nil {
		return rel, nil
	}
	return path, nil
}

func writeEntry(b *strings.Builder, e *Entry) {
	stamp := e.Time.Format("15:04:05.000")
	if e.Kind == KindTool {
		fmt.Fprintf(b, "%s        %s\n", stamp, stripANSI(e.Text))
		return
	}
	name, _ := label(themeless, e.Level)
	line := e.Text
	if e.Tag != "" {
		line = e.Tag + ": " + line
	}
	for _, f := range e.Fields {
		line += " " + f.Key + "=" + f.Value
	}
	if e.At != "" {
		line += "  (" + e.At + ")"
	}
	fmt.Fprintf(b, "%s %-4s  %s\n", stamp, name, line)
	for _, group := range [][]Line{e.Lines, e.Stack, e.Details} {
		for _, l := range group {
			fmt.Fprintf(b, "                    %s\n", l.Text)
		}
	}
}

// prune keeps the newest keepSessions sessions, and every starred one.
func prune(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	stars := readStars(dir)
	sessions := map[string][]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".log") {
			continue
		}
		base := strings.TrimSuffix(strings.TrimSuffix(name, ".log"), ".raw")
		if !stars[base] {
			sessions[base] = append(sessions[base], name)
		}
	}
	var bases []string
	for base := range sessions {
		bases = append(bases, base)
	}
	sort.Strings(bases) // names start with the time, so this is oldest first
	for len(bases) > keepSessions {
		for _, name := range sessions[bases[0]] {
			_ = os.Remove(filepath.Join(dir, name))
		}
		bases = bases[1:]
	}
}
