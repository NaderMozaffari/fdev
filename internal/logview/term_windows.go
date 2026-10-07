//go:build windows

package logview

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// shellCommand runs a target's command line with Git for Windows' sh (which
// Flutter needs anyway), so Makefile recipes and quoting work as they do
// elsewhere; cmd.exe when there is none.
func shellCommand(line string) *exec.Cmd {
	if sh := findSh(); sh != "" {
		return exec.Command(sh, "-c", line)
	}
	return exec.Command("cmd", "/C", line)
}

func findSh() string {
	if sh, err := exec.LookPath("sh"); err == nil {
		return sh
	}
	// Git's installer puts only Git\cmd on the PATH; sh is in Git\bin.
	if git, err := exec.LookPath("git"); err == nil {
		sh := filepath.Join(filepath.Dir(filepath.Dir(git)), "bin", "sh.exe")
		if _, err := os.Stat(sh); err == nil {
			return sh
		}
	}
	return ""
}

// pipeTerm stands in for a pty on Windows: output and errors through one
// pipe (so they stay in order), keys through stdin. ctrl+c can't be sent
// down a pipe, so it stops the command.
type pipeTerm struct {
	out *os.File
	in  io.WriteCloser
	cmd *exec.Cmd
}

func startTerm(cmd *exec.Cmd, rows, cols int) (term, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		r.Close()
		w.Close()
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return nil, err
	}
	w.Close() // the child has its copy; reads end when it exits
	return &pipeTerm{out: r, in: in, cmd: cmd}, nil
}

func (t *pipeTerm) Read(p []byte) (int, error) { return t.out.Read(p) }

func (t *pipeTerm) Write(p []byte) (int, error) {
	if bytes.IndexByte(p, 0x03) >= 0 && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		return len(p), nil
	}
	return t.in.Write(p)
}

func (t *pipeTerm) Close() error {
	t.in.Close()
	return t.out.Close()
}

func (t *pipeTerm) Resize(rows, cols int) error { return nil }
