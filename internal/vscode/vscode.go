// Package vscode lets VS Code's terminal (and Cursor's, and the other
// editors built on it) name its tab after fdev's title.
//
// Other terminals show the title a program sets. VS Code names a tab after
// its process ("fdev") unless terminal.integrated.tabs.title has
// ${sequence}, the title the program sets; with it, a tab whose program
// sets none is still named after its process.
package vscode

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	key   = "terminal.integrated.tabs.title"
	value = "${sequence}"
)

// ShowTitles adds terminal.integrated.tabs.title to the user settings of the
// editor fdev runs in, once: a value there already, even in a comment, is
// left alone. It returns the settings file when it changed it.
func ShowTitles() (string, error) {
	if os.Getenv("TERM_PROGRAM") != "vscode" {
		return "", nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, product(), "User")
	if _, err := os.Stat(dir); err != nil { // e.g. a remote: its settings are on the other side
		return "", nil
	}
	path := filepath.Join(dir, "settings.json")
	mode := fs.FileMode(0o644)
	data, err := os.ReadFile(path)
	if err == nil {
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	out, ok := withTitle(data)
	if !ok {
		return "", nil
	}
	if err := os.WriteFile(path, out, mode); err != nil {
		return "", err
	}
	return path, nil
}

// product is the folder the editor keeps its settings in, worked out from
// what it puts in the terminal's environment.
func product() string {
	hint := strings.ToLower(os.Getenv("__CFBundleIdentifier") + " " + os.Getenv("VSCODE_GIT_ASKPASS_NODE") + " " + os.Getenv("TERM_PROGRAM_VERSION"))
	switch {
	case strings.Contains(hint, "todesktop"), strings.Contains(hint, "cursor"):
		return "Cursor"
	case strings.Contains(hint, "windsurf"):
		return "Windsurf"
	case strings.Contains(hint, "codium"):
		return "VSCodium"
	case strings.Contains(hint, "insider"):
		return "Code - Insiders"
	}
	return "Code"
}

// withTitle adds the setting at the top of settings.json, keeping the rest
// (comments, order, indentation) as it is. ok is false when the file names
// the setting already, or isn't an object.
func withTitle(data []byte) (out []byte, ok bool) {
	if bytes.Contains(data, []byte(`"`+key+`"`)) {
		return nil, false
	}
	open := skipJunk(data, 0)
	if open == len(data) {
		return []byte("{\n    \"" + key + "\": \"" + value + "\"\n}\n"), true
	}
	if data[open] != '{' {
		return nil, false
	}
	entry := "\n" + indent(data[open+1:]) + `"` + key + `": "` + value + `"`
	if next := skipJunk(data, open+1); next < len(data) && data[next] == '}' {
		entry += "\n" // {} or { /* nothing */ }
	} else {
		entry += ","
	}
	out = append(out, data[:open+1]...)
	out = append(out, entry...)
	return append(out, data[open+1:]...), true
}

// skipJunk is where the first thing after i that isn't space or a comment is.
func skipJunk(data []byte, i int) int {
	for i < len(data) {
		switch {
		case strings.ContainsRune(" \t\r\n", rune(data[i])):
			i++
		case bytes.HasPrefix(data[i:], []byte("//")):
			end := bytes.IndexByte(data[i:], '\n')
			if end < 0 {
				return len(data)
			}
			i += end + 1
		case bytes.HasPrefix(data[i:], []byte("/*")):
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				return len(data)
			}
			i += 2 + end + 2
		default:
			return i
		}
	}
	return i
}

// indent is the one the file's first entry has, or four spaces.
func indent(body []byte) string {
	for _, line := range strings.Split(string(body), "\n")[1:] {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != "" && trimmed != "\r" {
			if n := len(line) - len(trimmed); n > 0 {
				return line[:n]
			}
			break
		}
	}
	return "    "
}
