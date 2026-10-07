package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOnlyOpensProjectFiles(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "lib"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "lib", "a.dart"), []byte("x"), 0o644))
	must(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644))
	must(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "lib", "link.dart")))

	loc, err := Resolve(root, "lib/a.dart:12:3")
	if err != nil || loc.Line != 12 || loc.Col != 3 || !strings.HasSuffix(loc.File, "/lib/a.dart") || !filepath.IsAbs(loc.File) {
		t.Fatalf("Resolve = %+v, %v", loc, err)
	}
	if loc, err := Resolve(root, filepath.Join(root, "lib/a.dart")+":4"); err != nil || loc.Col != 1 {
		t.Errorf("absolute path inside: %+v, %v", loc, err)
	}
	for _, at := range []string{
		"../" + filepath.Base(outside) + "/secret.txt:1",
		filepath.Join(outside, "secret.txt") + ":1",
		"lib/link.dart:1",
		"lib/missing.dart:1",
		"lib:1",
		"--install-extension=x:1",
		"lib/a.dart",
	} {
		if _, err := Resolve(root, at); err == nil {
			t.Errorf("Resolve(%q) should fail", at)
		}
	}
}

func TestTemplate(t *testing.T) {
	loc := Location{File: "/p/lib/a.dart", Line: 7, Col: 2}
	cmd, inTerminal, err := fromTemplate(loc, "sh -c {file}:{line}:{col}")
	if err != nil || inTerminal || strings.Join(cmd.Args[1:], " ") != "-c /p/lib/a.dart:7:2" {
		t.Errorf("template = %v %v %v", cmd, inTerminal, err)
	}
	cmd, inTerminal, err = fromTemplate(loc, "vi")
	if err == nil && (!inTerminal || strings.Join(cmd.Args[1:], " ") != "+7 /p/lib/a.dart") {
		t.Errorf("vi = %v %v", cmd.Args, inTerminal)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
