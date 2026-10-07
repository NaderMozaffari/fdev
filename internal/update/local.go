package update

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NaderMozaffari/fdev/internal/version"
)

// module is the line of go.mod that marks fdev's source.
const module = "module github.com/NaderMozaffari/fdev"

// Local builds fdev from its source in dir (or a folder above it) and puts
// it where fdev is installed, for trying a change in your own projects:
// `go run . update --local` in a clone of fdev, or `fdev update --local
// <clone>` anywhere. `fdev update` goes back to the newest release.
func Local(dir string, out io.Writer) error {
	root, err := sourceRoot(dir)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go isn't on your PATH: install Go (https://go.dev/dl/) to build fdev from source")
	}
	exe, err := installed()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "fdev-local")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	bin := filepath.Join(tmp, exeName())

	fmt.Fprintf(out, "building fdev from %s...\n", version.Home(root))
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir, build.Stdout, build.Stderr = root, out, out
	if err := build.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	built, _ := exec.Command(bin, "version").Output()
	data, err := os.ReadFile(bin)
	if err != nil {
		return err
	}
	if err := replace(exe, data); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ installed %s at %s\n", strings.TrimSpace(string(built)), version.Home(exe))
	if first, err := exec.LookPath(exeName()); err == nil {
		if first, err = filepath.EvalSymlinks(first); err == nil && first != exe {
			fmt.Fprintf(out, "! but `fdev` runs %s, which comes first on your PATH (see: which -a fdev)\n", version.Home(first))
		}
	}
	fmt.Fprintln(out, "  fdev update goes back to the newest release, fdev update --beta to the newest beta")
	return nil
}

// sourceRoot is the folder of fdev's go.mod, in dir or above it.
func sourceRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; d = filepath.Dir(d) {
		if data, err := os.ReadFile(filepath.Join(d, "go.mod")); err == nil {
			if first, _, _ := bytes.Cut(data, []byte("\n")); string(bytes.TrimSpace(first)) == module {
				return d, nil
			}
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("%s isn't fdev's source: run it in your clone of fdev, or give its folder (fdev update --local ~/fdev)", version.Home(abs))
		}
	}
}
