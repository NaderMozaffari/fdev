package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerify(t *testing.T) {
	data := []byte("archive")
	sum := sha256.Sum256(data)
	list := []byte("0000  other.zip\n" + hex.EncodeToString(sum[:]) + "  fdev_linux_amd64.tar.gz\n")
	if err := verify(data, list, "fdev_linux_amd64.tar.gz"); err != nil {
		t.Fatal(err)
	}
	if err := verify([]byte("tampered"), list, "fdev_linux_amd64.tar.gz"); err == nil {
		t.Fatal("a wrong checksum passed")
	}
	if err := verify(data, list, "fdev_darwin_arm64.tar.gz"); err == nil {
		t.Fatal("a missing checksum passed")
	}
}

func TestExtract(t *testing.T) {
	var tgz bytes.Buffer
	gz := gzip.NewWriter(&tgz)
	tw := tar.NewWriter(gz)
	for _, f := range []struct{ name, body string }{{"README.md", "readme"}, {"fdev", "binary"}} {
		_ = tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(f.body))
	}
	tw.Close()
	gz.Close()
	if got, err := extract(tgz.Bytes(), "fdev_linux_amd64.tar.gz"); err != nil || string(got) != "binary" {
		t.Fatalf("tar.gz: %q, %v", got, err)
	}

	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	w, _ := zw.Create("fdev.exe")
	_, _ = w.Write([]byte("exe"))
	zw.Close()
	if got, err := extract(zbuf.Bytes(), "fdev_windows_amd64.zip"); err != nil || string(got) != "exe" {
		t.Fatalf("zip: %q, %v", got, err)
	}
}

func TestNewest(t *testing.T) {
	all := []release{ // as GitHub lists them: newest made first
		{Tag: "v1.0.1"},
		{Tag: "v1.1.0-beta.2", Prerelease: true},
		{Tag: "v1.2.0-beta.1", Prerelease: true, Draft: true},
		{Tag: "v1.1.0-beta.10", Prerelease: true},
		{Tag: "v1.0.0"},
		{Tag: "nightly", Prerelease: true},
		{Tag: "v1.1.0-rc.1"}, // a beta, though not marked as one
	}
	for _, c := range []struct {
		beta bool
		want string
	}{{false, "v1.0.1"}, {true, "v1.1.0-rc.1"}} {
		if got, ok := Newest(all, c.beta); !ok || got.Tag != c.want {
			t.Errorf("Newest(beta %v) = %s, %v; want %s", c.beta, got.Tag, ok, c.want)
		}
	}
	betas := []release{{Tag: "v0.2.0-beta.1", Prerelease: true}, {Tag: "v0.1.3", Prerelease: true}}
	if got, ok := Newest(betas, false); ok {
		t.Errorf("a stable release among betas: %s", got.Tag)
	}
	if got, _ := Newest(betas, true); got.Tag != "v0.2.0-beta.1" {
		t.Errorf("newest beta: %s", got.Tag)
	}
}
