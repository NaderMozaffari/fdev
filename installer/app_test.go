//go:build darwin

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/NaderMozaffari/fdev/internal/state"
	"github.com/NaderMozaffari/fdev/internal/update"
)

// archive is a release archive holding an fdev that prints tag.
func archive(t *testing.T, tag string) []byte {
	t.Helper()
	body := []byte("#!/bin/sh\necho fdev " + tag + "\n")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "fdev", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	tw.Write(body)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// testApp is an installer carrying fdev v1.2.3-beta.1, in a home folder
// of its own, offline unless online is given.
func testApp(t *testing.T, online func() (*update.Release, *update.Release, error)) (*App, *[]string) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin") // no fdev of this machine's
	old := version
	version = "v1.2.3-beta.1"
	t.Cleanup(func() { version = old })

	var logs []string
	if online == nil {
		online = func() (*update.Release, *update.Release, error) { return nil, nil, errors.New("offline") }
	}
	a := &App{
		emit: func(event string, data ...any) {
			if event == "log" {
				logs = append(logs, data[0].(map[string]string)["text"])
			}
		},
		bundle: fstest.MapFS{update.Asset(runtime.GOOS, runtime.GOARCH): {Data: archive(t, "v1.2.3-beta.1")}},
		latest: online,
	}
	return a, &logs
}

// TestInstallOffline installs the fdev the installer carries, with no
// network: into ~/.fdev/bin, on the PATH once, with the theme.
func TestInstallOffline(t *testing.T) {
	a, logs := testApp(t, nil)
	if _, err := a.Check(); err == nil {
		t.Fatal("Check worked offline")
	}
	info, err := a.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	if info.Installed != nil {
		t.Fatalf("fdev is installed already: %+v", info.Installed)
	}
	want := []Choice{{Channel: "stable"}, {Channel: "beta", Tag: "v1.2.3-beta.1", Size: info.Choices[1].Size}}
	if len(info.Choices) != 2 || info.Choices[0] != want[0] || info.Choices[1] != want[1] {
		t.Fatalf("choices %+v", info.Choices)
	}
	if err := a.Install("user", true, "dracula", "stable"); err == nil || !strings.Contains(err.Error(), "no stable release") {
		t.Fatalf("Install(stable) = %v", err)
	}

	for i := range 2 {
		*logs = nil
		if err := a.Install("user", true, "dracula", "beta"); err != nil {
			t.Fatal(err)
		}
		all := strings.Join(*logs, "\n")
		path := "added ~/.fdev/bin to PATH in ~/.zshrc"
		if i == 1 {
			path = "~/.fdev/bin is on your PATH already"
		}
		for _, w := range []string{"is in the installer: nothing to download", "copied to ~/.fdev/bin/fdev", path, "installed fdev v1.2.3-beta.1"} {
			if !strings.Contains(all, w) {
				t.Errorf("run %d: no %q in the log:\n%s", i, w, all)
			}
		}
	}
	home := os.Getenv("HOME")
	rc, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
	if n := strings.Count(string(rc), `export PATH="$HOME/.fdev/bin:$PATH"`); n != 1 {
		t.Errorf("~/.zshrc has the PATH line %d times:\n%s", n, rc)
	}
	if got := state.Load("").UI.Theme; got != "dracula" {
		t.Errorf("fdev's theme is %q", got)
	}
	// Run again, the installer sees the fdev it put there.
	if info, _ := a.Prepare(); info.Installed == nil || info.Installed.Tag != "v1.2.3-beta.1" || info.Installed.Dest != "user" {
		t.Errorf("installed: %+v", info.Installed)
	}
}

// TestInstallUpdate downloads the channel's newer release when GitHub has
// one, and keeps the installer's own fdev when GitHub's is older.
func TestInstallUpdate(t *testing.T) {
	newer := &update.Release{Tag: "v1.2.4-beta.1", Archive: update.Asset(runtime.GOOS, runtime.GOARCH), Size: 100}
	a, logs := testApp(t, func() (*update.Release, *update.Release, error) { return nil, newer, nil })
	a.fetch = func(r *update.Release, read func(int64, int64)) ([]byte, string, error) {
		read(100, 100)
		bin, err := update.Extract(archive(t, r.Tag), r.Archive)
		return bin, "9f2c0000e41a", err
	}
	choices, err := a.Check()
	if err != nil {
		t.Fatal(err)
	}
	if c := choices[1]; c.Tag != "v1.2.4-beta.1" || !c.Download {
		t.Fatalf("beta: %+v", c)
	}
	if err := a.Install("user", false, "", "beta"); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(*logs, "\n")
	for _, w := range []string{"from GitHub", "checksums.txt sha256 9f2c…e41a ok", "installed fdev v1.2.4-beta.1"} {
		if !strings.Contains(all, w) {
			t.Errorf("no %q in the log:\n%s", w, all)
		}
	}

	// An older release on GitHub than the installer's: the installer's wins.
	a.online["beta"] = &update.Release{Tag: "v1.2.2-beta.1"}
	if c := a.choices()[1]; c.Tag != "v1.2.3-beta.1" || c.Download {
		t.Errorf("beta with an older release online: %+v", c)
	}
}

func TestInstallUnknownFolder(t *testing.T) {
	a, _ := testApp(t, nil)
	if err := a.Install("nowhere", false, "", "beta"); err == nil || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("Install(nowhere) = %v", err)
	}
}
