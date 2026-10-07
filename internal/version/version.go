// Package version is fdev's own version, for the headers and the
// terminal's title. main sets it at start.
package version

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/NaderMozaffari/fdev/internal/config"
)

// Current is the version fdev was built as: a tag (v1.4.0), a Go
// pseudo-version for a local build, or "dev".
var Current = "dev"

// Repo is where fdev update looks for releases; main sets it.
var Repo = ""

// Shown is whether fdev's name in the headers and the terminal's title
// has its version by it. Clicking the name turns it on and off.
var Shown bool

var pseudo = regexp.MustCompile(`-(?:\d+\.)?\d{14}-([0-9a-f]{12})(?:\+dirty)?$`)

// Short is Current as headers show it: a tag as it is, a pseudo-version
// as dev and its commit (dev·1a2b3c4).
func Short() string {
	if m := pseudo.FindStringSubmatch(Current); m != nil {
		return "dev·" + m[1][:7]
	}
	return Current
}

// Describe is Current as `fdev version` prints it: a release with its
// channel (v0.2.0-beta.1 (beta)), a build from source as Short and what it
// is (dev·07c1a2b (built from source, with uncommitted changes)).
func Describe() string {
	switch Channel(Current) {
	case Beta:
		return Current + " (beta)"
	case Stable:
		return Current
	}
	note := "built from source"
	if strings.HasSuffix(Current, "+dirty") || build().dirty {
		note += ", with uncommitted changes"
	}
	return Short() + " (" + note + ")"
}

// Name is fdev's name as the headers show it: "fdev", or "fdev v1.4.0"
// once it was clicked.
func Name() string {
	if Shown {
		return "fdev " + Short()
	}
	return "fdev"
}

// Line is fdev's version with the commit and platform it was built for,
// on one line: fdev v1.4.0 · 5177840 · darwin/arm64.
func Line() string {
	parts := []string{"fdev " + Short()}
	if b := build(); b.commit != "" && !strings.Contains(Short(), b.commit[:7]) {
		parts = append(parts, b.commit[:7])
	}
	return strings.Join(append(parts, runtime.GOOS+"/"+runtime.GOARCH), " · ")
}

// Facts is what fdev knows about itself, for the settings: its version,
// the commit and Go it was built from, and where it is installed.
func Facts() []config.Fact {
	facts := []config.Fact{{Key: "Version", Value: Short()}}
	switch Channel(Current) {
	case Beta:
		facts = append(facts, config.Fact{Key: "Channel", Value: "beta: a pre-release, it may have bugs"})
	case Stable:
		facts = append(facts, config.Fact{Key: "Channel", Value: "stable"})
	}
	b := build()
	if b.commit != "" {
		commit := b.commit[:min(len(b.commit), 12)]
		if b.dirty {
			commit += " · with uncommitted changes"
		}
		facts = append(facts, config.Fact{Key: "Commit", Value: commit})
	}
	if !b.at.IsZero() {
		facts = append(facts, config.Fact{Key: "Committed", Value: b.at.Local().Format("Mon 2 Jan 2006 15:04")})
	}
	facts = append(facts,
		config.Fact{Key: "Go", Value: runtime.Version()},
		config.Fact{Key: "Platform", Value: runtime.GOOS + "/" + runtime.GOARCH},
	)
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		facts = append(facts, config.Fact{Key: "Installed", Value: Home(exe)})
	}
	if Repo != "" {
		how := "fdev update"
		switch Channel(Current) {
		case Beta:
			how += " (the newest beta; fdev channel stable for releases only)"
		case Stable:
			how += " (fdev channel beta for betas too)"
		}
		facts = append(facts, config.Fact{Key: "Updates", Value: how + ", from github.com/" + Repo})
	}
	return facts
}

// Home is path with the home folder as ~.
func Home(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return path
}

type buildInfo struct {
	commit string
	dirty  bool
	at     time.Time
}

// build is the commit Go stamped into the binary, when it was built in
// fdev's repository.
func build() buildInfo {
	var b buildInfo
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return b
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				b.commit = s.Value
			}
		case "vcs.modified":
			b.dirty = s.Value == "true"
		case "vcs.time":
			b.at, _ = time.Parse(time.RFC3339, s.Value)
		}
	}
	return b
}
