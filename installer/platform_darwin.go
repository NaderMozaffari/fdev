package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NaderMozaffari/fdev/internal/update"
)

func destinations() ([]Dest, error) {
	user, err := update.InstallDir()
	if err != nil {
		return nil, err
	}
	return []Dest{
		{ID: "user", Path: user, Shown: tilde(user), Note: "just for me · no password"},
		// On every Mac's PATH already (/etc/paths).
		{ID: "all", Path: "/usr/local/bin", Shown: "/usr/local/bin", Note: "everyone · needs your password", Admin: true},
	}, nil
}

// installAdmin copies bin into dir as root, asking for the password with
// the dialog macOS shows for any app.
func installAdmin(bin []byte, dir string, _ bool) error {
	tmp, err := os.CreateTemp("", "fdev-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	exe := filepath.Join(dir, "fdev")
	sh := fmt.Sprintf("mkdir -p %s && cp -f %s %s && chmod 755 %s", quote(dir), quote(tmp.Name()), quote(exe), quote(exe))
	script := `do shell script "` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(sh) +
		`" with prompt "fdev wants to install itself in ` + dir + `." with administrator privileges`
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "-128") { // the user pressed Cancel
			return errors.New("the password dialog was cancelled, so nothing was installed")
		}
		return fmt.Errorf("couldn't install in %s: %s", dir, strings.TrimSpace(string(out)))
	}
	return nil
}

// onPath puts d on the PATH of each shell the user has, the way
// scripts/install.sh does: a line in ~/.zshrc (zsh is the Mac's shell), and
// in ~/.bash_profile and fish's conf.d when those shells are in use. It
// says where it added it; nothing when it was there already.
func onPath(d Dest) ([]string, error) {
	if d.Admin {
		return nil, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	// $HOME stays as it is, so the files still work if the home folder moves.
	shown := d.Path
	if rest, ok := strings.CutPrefix(d.Path, home+"/"); ok {
		shown = "$HOME/" + rest
	}
	line := `export PATH="` + shown + `:$PATH"`
	zdot := os.Getenv("ZDOTDIR")
	if zdot == "" {
		zdot = home
	}
	type rc struct{ file, line string }
	files := []rc{{filepath.Join(zdot, ".zshrc"), line}}
	shell := filepath.Base(os.Getenv("SHELL"))
	if bash := filepath.Join(home, ".bash_profile"); shell == "bash" || exists(bash) {
		files = append(files, rc{bash, line})
	}
	if fish := filepath.Join(home, ".config", "fish"); shell == "fish" || exists(fish) {
		files = append(files, rc{filepath.Join(fish, "conf.d", "fdev.fish"), "contains " + d.Path + " $PATH; or set -gx PATH " + d.Path + " $PATH"})
	}
	var added []string
	var errs []error
	for _, f := range files {
		if data, err := os.ReadFile(f.file); err == nil && strings.Contains(string(data), f.line) {
			continue
		}
		if err := appendLine(f.file, f.line); err != nil {
			errs = append(errs, err)
			continue
		}
		added = append(added, "in "+tilde(f.file))
	}
	return added, errors.Join(errs...)
}

func appendLine(file, line string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString("\n# fdev\n" + line + "\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func openTerminal() error {
	home, _ := os.UserHomeDir()
	return exec.Command("open", "-a", "Terminal", home).Start()
}

// runElevated is for Windows, which installs for everyone through a second,
// elevated copy of the installer.
func runElevated([]string) (int, bool) { return 0, false }

// tilde writes a path in the home folder as ~/...
func tilde(p string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rest, ok := strings.CutPrefix(p, home+"/"); ok {
			return "~/" + rest
		}
	}
	return p
}

// quote is s as one word for sh.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
