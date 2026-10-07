package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module+"\n\ngo 1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(root, "internal", "update")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := sourceRoot(inner); err != nil || got != root {
		t.Errorf("sourceRoot(inner) = %q, %v; want %q", got, err, root)
	}

	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "go.mod"), []byte(module+"-fork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := sourceRoot(other); err == nil {
		t.Error("sourceRoot found fdev's source in another module")
	}
}
