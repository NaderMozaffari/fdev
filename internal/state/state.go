// Package state remembers, per project, the recent runs, the last answers
// and the log viewer settings, and for every project the theme and size. It
// lives in the user's cache directory, never in the project.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/NaderMozaffari/fdev/internal/config"
)

type Run struct {
	Target string            `json:"target"`
	Env    map[string]string `json:"env"`
	Note   string            `json:"note,omitempty"` // e.g. the device name
	At     time.Time         `json:"at"`
}

type State struct {
	Recent  []Run             `json:"recent"`
	Answers map[string]string `json:"answers"` // ask name -> last value
	Logs    map[string]bool   `json:"logs"`    // log viewer toggles
	// Durations is how long flutter's steps took, for progress bars.
	Durations map[string]float64 `json:"durations,omitempty"`
	// Menu is the menu's last tab, platform and flavor.
	Menu map[string]string `json:"menu,omitempty"`
	// Look is the user's log viewer settings; nil until they change them.
	Look *config.Look `json:"look,omitempty"`
	// UI is the theme and the size of menus, for every project: it is kept
	// in a file of its own (SaveUI).
	UI     config.UI `json:"-"`
	path   string
	uiPath string
}

func Load(root string) *State {
	s := &State{Answers: map[string]string{}, Logs: map[string]bool{}}
	dir, err := os.UserCacheDir()
	if err != nil {
		return s
	}
	sum := sha256.Sum256([]byte(root))
	s.path = filepath.Join(dir, "fdev", hex.EncodeToString(sum[:8])+".json")
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, s)
	}
	s.uiPath = filepath.Join(dir, "fdev", "ui.json")
	if data, err := os.ReadFile(s.uiPath); err == nil {
		_ = json.Unmarshal(data, &s.UI)
	}
	s.UI = s.UI.WithDefaults()
	if s.Answers == nil {
		s.Answers = map[string]string{}
	}
	if s.Logs == nil {
		s.Logs = map[string]bool{}
	}
	if s.Durations == nil {
		s.Durations = map[string]float64{}
	}
	return s
}

func (s *State) Save() { write(s.path, s) }

// SaveUI keeps the theme and size for every project.
func (s *State) SaveUI() { write(s.uiPath, s.UI) }

func write(path string, v any) {
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
}

// Remember puts run first in the recent list (at most five, no repeats).
func (s *State) Remember(run Run) {
	run.At = time.Now()
	recent := []Run{run}
	for _, r := range s.Recent {
		if r.Target == run.Target && sameEnv(r.Env, run.Env) {
			continue
		}
		if len(recent) < 5 {
			recent = append(recent, r)
		}
	}
	s.Recent = recent
	s.Save()
}

func sameEnv(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
