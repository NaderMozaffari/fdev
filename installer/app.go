package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/NaderMozaffari/fdev/internal/state"
	"github.com/NaderMozaffari/fdev/internal/update"
	fdevversion "github.com/NaderMozaffari/fdev/internal/version"
)

// App is what the page calls: Prepare when it opens (no network), Check
// for newer releases on GitHub, Install, then OpenTerminal or Quit.
//
// The installer carries the fdev of the release it was built for (bundle/),
// so it installs without the internet. When it can reach GitHub and the
// channel picked has a newer release, it downloads that one instead: run
// it again later and it updates fdev.
type App struct {
	emit   func(event string, data ...any) // to the page
	bundle fs.FS                           // the release archives the installer carries
	latest func() (stable, beta *update.Release, err error)
	fetch  func(r *update.Release, read func(done, total int64)) ([]byte, string, error)
	online map[string]*update.Release // Check's answer, by channel
}

func newApp() *App {
	b, _ := fs.Sub(bundled, "bundle")
	return &App{
		bundle: b,
		latest: func() (*update.Release, *update.Release, error) { return update.Latest(repo, runtime.GOOS, runtime.GOARCH) },
		fetch:  func(r *update.Release, read func(int64, int64)) ([]byte, string, error) { return r.Fetch(read) },
	}
}

func (a *App) startup(ctx context.Context) {
	a.emit = func(event string, data ...any) { wr.EventsEmit(ctx, event, data...) }
}

// Info is what the screens show before anything happens.
type Info struct {
	OS        string   `json:"os"`       // darwin or windows
	Platform  string   `json:"platform"` // e.g. darwin/arm64
	Included  *Version `json:"included"` // the fdev in the installer; nil in a build without one
	Installed *Version `json:"installed"`
	Choices   []Choice `json:"choices"`
	Dests     []Dest   `json:"dests"`
	Theme     string   `json:"theme"` // the theme fdev uses now, as the page names it
}

// Version is an fdev: the one in the installer, or the one installed.
type Version struct {
	Tag     string `json:"tag"`
	Channel string `json:"channel"`
	Size    int64  `json:"size,omitempty"`
	Dest    string `json:"dest,omitempty"`  // installed: the Dest it is in, if one of them
	Where   string `json:"where,omitempty"` // installed: its folder as the screen writes it
}

// Choice is what installing a channel means: which version, and whether it
// comes from GitHub or from the installer. Tag is "" when the channel has
// nothing to install, as stable until its first release.
type Choice struct {
	Channel  string `json:"channel"`
	Tag      string `json:"tag"`
	Download bool   `json:"download"`
	Size     int64  `json:"size"`
}

// Dest is a folder fdev can be installed in.
type Dest struct {
	ID    string `json:"id"`
	Path  string `json:"path"`  // the real folder
	Shown string `json:"shown"` // the folder as the screen writes it: ~/.fdev/bin
	Note  string `json:"note"`
	Admin bool   `json:"admin"` // needs a password or admin rights
}

// themes maps the page's theme names to fdev's (internal/theme).
var themes = map[string]string{
	"charm": "auto", "dracula": "dracula", "tokyo": "tokyo-night",
	"catppuccin": "catppuccin", "gruvbox": "gruvbox", "latte": "latte",
}

var channels = []string{fdevversion.Stable, fdevversion.Beta}

// Prepare says what can be installed with no network: the fdev in the
// installer, and the fdev already on this computer.
func (a *App) Prepare() (Info, error) {
	dests, err := destinations()
	if err != nil {
		return Info{}, err
	}
	theme := "charm"
	current := state.Load("").UI.Theme
	for page, name := range themes {
		if name == current {
			theme = page
		}
	}
	return Info{
		OS: runtime.GOOS, Platform: runtime.GOOS + "/" + runtime.GOARCH,
		Included: a.included(), Installed: installed(dests),
		Choices: a.choices(), Dests: dests, Theme: theme,
	}, nil
}

// Check asks GitHub for the newest releases, and gives the choices again
// with them. Offline, it fails, and the installer's own fdev still installs.
func (a *App) Check() ([]Choice, error) {
	stable, beta, err := a.latest()
	if err != nil {
		return nil, fmt.Errorf("can't reach github.com/%s: %w", repo, err)
	}
	a.online = map[string]*update.Release{fdevversion.Stable: stable, fdevversion.Beta: beta}
	return a.choices(), nil
}

// included is the fdev the installer carries for this OS and CPU.
func (a *App) included() *Version {
	if !fdevversion.Valid(version) {
		return nil
	}
	info, err := fs.Stat(a.bundle, update.Asset(runtime.GOOS, runtime.GOARCH))
	if err != nil {
		return nil
	}
	return &Version{Tag: version, Channel: fdevversion.Channel(version), Size: info.Size()}
}

// fits is whether v belongs on channel: a beta on beta, a stable release
// on either (the beta channel takes stable releases too).
func fits(v *Version, channel string) bool {
	return v != nil && (v.Channel == channel || channel == fdevversion.Beta)
}

// choices picks, for each channel, the newest of the installer's fdev and
// GitHub's release.
func (a *App) choices() []Choice {
	inc := a.included()
	var out []Choice
	for _, ch := range channels {
		c := Choice{Channel: ch}
		rel := a.online[ch]
		switch {
		case rel != nil && (!fits(inc, ch) || fdevversion.Compare(rel.Tag, inc.Tag) > 0):
			c.Tag, c.Download, c.Size = rel.Tag, true, rel.Size
		case fits(inc, ch):
			c.Tag, c.Size = inc.Tag, inc.Size
		}
		out = append(out, c)
	}
	return out
}

// Install puts the channel's fdev into the folder dest names, puts that
// folder on the PATH when addPath, and saves theme for fdev. The page
// follows it through "log" and "progress" events.
func (a *App) Install(dest string, addPath bool, theme, channel string) error {
	dests, err := destinations()
	if err != nil {
		return err
	}
	var d Dest
	for _, x := range dests {
		if x.ID == dest {
			d = x
		}
	}
	if d.ID == "" {
		return fmt.Errorf("no folder %q", dest)
	}
	bin, err := a.binary(channel)
	if err != nil {
		return err
	}

	exe := filepath.Join(d.Path, exeName())
	if d.Admin {
		err = installAdmin(bin, d.Path, addPath)
	} else if err = os.MkdirAll(d.Path, 0o755); err == nil {
		err = update.Replace(exe, bin)
	}
	if err != nil {
		return err
	}
	a.log("INFO", "copied to "+filepath.Join(d.Shown, exeName()))

	if addPath {
		where, err := onPath(d)
		if err != nil {
			a.log("WARN", "couldn't add it to your PATH: "+err.Error())
		}
		for _, w := range where {
			a.log("INFO", "added "+d.Shown+" to PATH "+w)
		}
		if len(where) == 0 && err == nil {
			a.log("INFO", d.Shown+" is on your PATH already")
		}
	}
	// So the terminal OpenTerminal starts finds fdev straight away.
	os.Setenv("PATH", d.Path+string(os.PathListSeparator)+os.Getenv("PATH"))

	if name, ok := themes[theme]; ok {
		s := state.Load("")
		s.UI.Theme = name
		s.SaveUI()
	}
	if v := versionOf(exe); v != "" {
		a.log("INFO", "installed fdev "+v)
	}
	return nil
}

// binary is the channel's fdev: downloaded and checked when GitHub has a
// newer one than the installer, else unpacked from the installer.
func (a *App) binary(channel string) ([]byte, error) {
	var pick *Choice
	for _, c := range a.choices() {
		if c.Channel == channel && c.Tag != "" {
			pick = &c
		}
	}
	if pick == nil {
		if channel == fdevversion.Stable {
			return nil, errors.New("there is no stable release yet: pick beta")
		}
		return nil, fmt.Errorf("this installer has no fdev for the %s channel, and GitHub can't be reached to download it", channel)
	}
	name := update.Asset(runtime.GOOS, runtime.GOARCH)
	if pick.Download {
		rel := a.online[channel]
		a.log("INFO", fmt.Sprintf("%s %s from GitHub", rel.Archive, size(rel.Size)))
		a.log("STEP", "download")
		last := -1
		bin, sum, err := a.fetch(rel, func(done, total int64) {
			if total <= 0 {
				return
			}
			// One event a percent is plenty for the bar.
			if pct := int(done * 100 / total); pct != last {
				last = pct
				a.emit("progress", done, total)
			}
		})
		if err != nil {
			return nil, err
		}
		a.log("DEBUG", fmt.Sprintf("checksums.txt sha256 %s…%s ok", sum[:4], sum[len(sum)-4:]))
		return bin, nil
	}
	data, err := fs.ReadFile(a.bundle, name)
	if err != nil {
		return nil, err
	}
	a.log("INFO", fmt.Sprintf("fdev %s is in the installer: nothing to download", pick.Tag))
	a.log("STEP", "unpack")
	a.emit("progress", int64(len(data)), int64(len(data)))
	return update.Extract(data, name)
}

// installed is the fdev already on this computer: in one of the folders
// the installer offers, or else wherever the PATH finds one.
func installed(dests []Dest) *Version {
	for _, d := range dests {
		if v := versionOf(filepath.Join(d.Path, exeName())); v != "" {
			return &Version{Tag: v, Channel: fdevversion.Channel(v), Dest: d.ID, Where: d.Shown}
		}
	}
	if exe, err := exec.LookPath(exeName()); err == nil {
		if v := versionOf(exe); v != "" {
			return &Version{Tag: v, Channel: fdevversion.Channel(v), Where: tilde(filepath.Dir(exe))}
		}
	}
	return nil
}

// versionOf asks the fdev at exe for its version: `fdev version` prints
// "fdev v0.2.0-beta.1 (beta)".
func versionOf(exe string) string {
	if _, err := os.Stat(exe); err != nil {
		return ""
	}
	out, err := exec.Command(exe, "version").Output()
	if err != nil {
		return ""
	}
	if f := strings.Fields(string(out)); len(f) >= 2 && f[0] == "fdev" {
		return f[1]
	}
	return ""
}

// OpenTerminal opens a terminal in the home folder, for the first fdev.
func (a *App) OpenTerminal() error { return openTerminal() }

func (a *App) log(level, text string) {
	a.emit("log", map[string]string{"level": level, "text": text, "time": time.Now().Format("15:04:05")})
}

// size is a byte count the way the screens write it: 6.8 MB.
func size(n int64) string {
	return fmt.Sprintf("%.1f MB", float64(n)/1e6)
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "fdev.exe"
	}
	return "fdev"
}
