package update

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("fdev.exe")
	_, _ = w.Write([]byte("binary"))
	zw.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	sums := hex.EncodeToString(sum[:]) + "  fdev_windows_amd64.zip\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			_, _ = w.Write(archive)
		case "/s":
			_, _ = w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rel := release{Tag: "v1.0.0", Assets: []asset{
		{Name: "fdev_windows_amd64.zip", URL: srv.URL + "/a", Size: int64(len(archive))},
		{Name: "checksums.txt", URL: srv.URL + "/s"},
	}}
	r := Release{Tag: rel.Tag, Archive: "fdev_windows_amd64.zip", rel: rel, c: &client{http: srv.Client()}}
	var last, total int64
	bin, got, err := r.Fetch(func(done, all int64) { last, total = done, all })
	if err != nil || string(bin) != "binary" {
		t.Fatalf("Fetch: %q, %v", bin, err)
	}
	if got != hex.EncodeToString(sum[:]) {
		t.Errorf("sum %s", got)
	}
	if last != int64(len(archive)) || total != int64(len(archive)) {
		t.Errorf("progress ended at %d of %d, want %d", last, total, len(archive))
	}

	// A tampered archive fails the checksum.
	sums = "0000  fdev_windows_amd64.zip\n"
	if _, _, err := r.Fetch(nil); err == nil {
		t.Error("a wrong checksum passed")
	}
}
