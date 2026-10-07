// Package editor opens a source location from a log line in the user's editor.
//
// It runs the editor's own command line directly (no shell, no URL handler)
// and only for files that exist inside the project, so a log line can't make
// it open anything else.
package editor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type Location struct {
	File      string // absolute, symlinks resolved
	Line, Col int
}

var locationPattern = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?$`)

// Resolve checks `path:line[:col]` (relative to root, or absolute) and
// returns it only if the file exists inside root.
func Resolve(root, at string) (Location, error) {
	m := locationPattern.FindStringSubmatch(at)
	if m == nil {
		return Location{}, fmt.Errorf("not a location: %q", at)
	}
	path := m[1]
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Location{}, err
	}
	realPath, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return Location{}, fmt.Errorf("%s is not in this project", m[1])
	}
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return Location{}, fmt.Errorf("%s is outside the project", m[1])
	}
	if info, err := os.Stat(realPath); err != nil || !info.Mode().IsRegular() {
		return Location{}, fmt.Errorf("%s is not a file", m[1])
	}
	loc := Location{File: realPath, Col: 1}
	loc.Line, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		loc.Col, _ = strconv.Atoi(m[3])
	}
	return loc, nil
}

// Command returns the command that opens loc, and whether it runs in the
// terminal (so the caller has to hand the terminal over while it runs).
//
// template, from FDEV_EDITOR or the config's editor field, is split on spaces
// and may use {file}, {line} and {col}, e.g. "zed {file}:{line}:{col}".
// Without one it picks the editor the terminal runs in (VS Code, Cursor,
// Android Studio / IntelliJ), then `code`, then $VISUAL / $EDITOR.
func Command(loc Location, template string) (cmd *exec.Cmd, inTerminal bool, err error) {
	if env := os.Getenv("FDEV_EDITOR"); env != "" {
		template = env
	}
	if template != "" {
		return fromTemplate(loc, template)
	}
	goTo := fmt.Sprintf("%s:%d:%d", loc.File, loc.Line, loc.Col)

	if os.Getenv("TERM_PROGRAM") == "vscode" {
		name := "code"
		if strings.Contains(os.Getenv("__CFBundleIdentifier"), "todesktop") {
			name = "cursor"
		}
		if bin := find(name); bin != "" {
			return exec.Command(bin, "-g", goTo), false, nil
		}
	}
	if os.Getenv("TERMINAL_EMULATOR") == "JetBrains-JediTerm" {
		for _, name := range []string{"studio", "idea"} {
			if bin := find(name); bin != "" {
				return exec.Command(bin, "--line", strconv.Itoa(loc.Line), "--column", strconv.Itoa(loc.Col), loc.File), false, nil
			}
		}
	}
	if bin := find("code"); bin != "" {
		return exec.Command(bin, "-g", goTo), false, nil
	}
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if value := os.Getenv(env); value != "" {
			return fromTemplate(loc, value)
		}
	}
	return nil, false, errors.New("no editor found: set FDEV_EDITOR, e.g. FDEV_EDITOR='code -g {file}:{line}:{col}'")
}

// Editors that take over the terminal, and take the line as +N.
var terminalEditors = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "nano": true, "hx": true, "helix": true,
	"micro": true, "kak": true, "emacs": true,
}

func fromTemplate(loc Location, template string) (*exec.Cmd, bool, error) {
	fields := strings.Fields(template)
	if len(fields) == 0 {
		return nil, false, errors.New("empty editor command")
	}
	bin := find(fields[0])
	if bin == "" {
		return nil, false, fmt.Errorf("editor %q not found", fields[0])
	}
	inTerminal := terminalEditors[filepath.Base(fields[0])]
	placeholders := strings.NewReplacer(
		"{file}", loc.File, "{line}", strconv.Itoa(loc.Line), "{col}", strconv.Itoa(loc.Col))
	args, hasFile := []string{}, false
	for _, f := range fields[1:] {
		if strings.Contains(f, "{file}") {
			hasFile = true
		}
		args = append(args, placeholders.Replace(f))
	}
	if !hasFile {
		if inTerminal {
			args = append(args, "+"+strconv.Itoa(loc.Line))
		}
		args = append(args, loc.File)
	}
	return exec.Command(bin, args...), inTerminal, nil
}

// find looks a command up on PATH, then where the macOS apps keep their CLIs.
func find(name string) string {
	if bin, err := exec.LookPath(name); err == nil {
		return bin
	}
	if runtime.GOOS != "darwin" {
		return ""
	}
	bundled := map[string][]string{
		"code":   {"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"},
		"cursor": {"/Applications/Cursor.app/Contents/Resources/app/bin/cursor"},
		"studio": {"/Applications/Android Studio.app/Contents/MacOS/studio"},
		"idea": {"/Applications/IntelliJ IDEA.app/Contents/MacOS/idea",
			"/Applications/IntelliJ IDEA CE.app/Contents/MacOS/idea"},
	}
	home, _ := os.UserHomeDir()
	for _, path := range bundled[name] {
		for _, candidate := range []string{path, filepath.Join(home, path)} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}
