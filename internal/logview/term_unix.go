//go:build !windows

package logview

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// shellCommand runs a target's command line the way make does.
func shellCommand(line string) *exec.Cmd {
	return exec.Command("sh", "-c", line)
}

// ptyTerm is the command's terminal: a pty, so it sees a tty (colors,
// flutter's single-key commands) and ctrl+c reaches it as a signal.
type ptyTerm struct{ *os.File }

func startTerm(cmd *exec.Cmd, rows, cols int) (term, error) {
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, err
	}
	return ptyTerm{f}, nil
}

func (t ptyTerm) Resize(rows, cols int) error {
	return pty.Setsize(t.File, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}
