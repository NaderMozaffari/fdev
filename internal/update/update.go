// Package update replaces the running fdev with a release from GitHub: the
// archive for this OS and CPU, checked against the release's checksums.txt.
// It keeps to a channel: stable releases, or betas too (GitHub
// pre-releases; see version.Channel). A token (GH_TOKEN, GITHUB_TOKEN, or
// the GitHub CLI's login) raises GitHub's rate limit, and reads a private
// repository.
package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NaderMozaffari/fdev/internal/version"
)

type Options struct {
	Repo    string // owner/name, where the releases are
	Current string // the running version
	Version string // a tag, or "" for the newest of the channel
	Beta    bool   // the beta channel: pre-releases too
	Out     io.Writer
}

type release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"url"` // the API's URL; with octet-stream it serves the file
}

// Asset is the archive name for an OS and CPU, as the release script names it.
func Asset(goos, goarch string) string {
	if goos == "windows" {
		return "fdev_" + goos + "_" + goarch + ".zip"
	}
	return "fdev_" + goos + "_" + goarch + ".tar.gz"
}

func Run(o Options) error {
	c := &client{http: &http.Client{Timeout: 5 * time.Minute}, token: token()}
	api := "https://api.github.com/repos/" + o.Repo + "/releases"
	var rel release
	if o.Version != "" {
		if err := c.json(api+"/tags/"+o.Version, &rel); err != nil {
			return err
		}
	} else {
		var all []release
		if err := c.json(api+"?per_page=100", &all); err != nil {
			return err
		}
		newest, ok := Newest(all, o.Beta)
		if !ok {
			if beta, ok := Newest(all, true); ok && !o.Beta {
				return fmt.Errorf("there is no stable release yet; `fdev update --beta` installs the newest beta, %s", beta.Tag)
			}
			return errors.New("there are no releases yet")
		}
		channel := version.Stable
		if o.Beta {
			channel = version.Beta
		}
		switch cmp := version.Compare(newest.Tag, o.Current); {
		case cmp == 0:
			fmt.Fprintf(o.Out, "fdev %s is the newest %s\n", o.Current, channel)
			return nil
		case cmp < 0:
			fmt.Fprintf(o.Out, "fdev %s is newer than the newest %s, %s; `fdev update %s` goes back to it\n",
				o.Current, channel, newest.Tag, newest.Tag)
			return nil
		}
		rel = newest
	}
	name := Asset(runtime.GOOS, runtime.GOARCH)
	archive, sums := find(rel.Assets, name), find(rel.Assets, "checksums.txt")
	if archive == nil {
		return fmt.Errorf("release %s has no %s", rel.Tag, name)
	}
	note := ""
	if version.Channel(rel.Tag) == version.Beta {
		note = ", a beta"
	}
	fmt.Fprintf(o.Out, "downloading fdev %s (%s%s)...\n", rel.Tag, name, note)
	data, err := c.download(archive.URL)
	if err != nil {
		return err
	}
	if sums == nil {
		return fmt.Errorf("release %s has no checksums.txt", rel.Tag)
	}
	list, err := c.download(sums.URL)
	if err != nil {
		return err
	}
	if err := verify(data, list, name); err != nil {
		return err
	}
	bin, err := extract(data, name)
	if err != nil {
		return err
	}
	exe, err := installed()
	if err != nil {
		return err
	}
	if err := replace(exe, bin); err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "fdev %s → %s (%s)\n", version.Short(), rel.Tag, version.Home(exe))
	return nil
}

// Newest is the newest release by version, a stable one unless beta (then
// betas count too). Drafts and tags that aren't versions don't count.
func Newest(all []release, beta bool) (release, bool) {
	var best release
	found := false
	for _, r := range all {
		if r.Draft || (r.Prerelease && !beta) || !version.Valid(r.Tag) {
			continue
		}
		// A beta marked as a full release on GitHub still is one.
		if !beta && version.Channel(r.Tag) == version.Beta {
			continue
		}
		if !found || version.Compare(r.Tag, best.Tag) > 0 {
			best, found = r, true
		}
	}
	return best, found
}

// Cleanup removes what an update on Windows leaves: the old binary, which
// can't be deleted while it runs.
func Cleanup() {
	if runtime.GOOS != "windows" {
		return
	}
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}
}

// token is the GitHub token, for a private repository (and higher rate
// limits): from the environment, or the GitHub CLI's login.
func token() string {
	for _, k := range []string{"FDEV_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

type client struct {
	http  *http.Client
	token string
}

func (c *client) get(url, accept string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		// Go drops it when GitHub redirects the download to its storage.
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusNotFound && c.token == "":
			return nil, errors.New("release not found; if the repository is private, run `gh auth login` or set GITHUB_TOKEN")
		case resp.StatusCode == http.StatusNotFound:
			return nil, errors.New("release not found (or the token can't read the repository)")
		case resp.StatusCode == http.StatusUnauthorized:
			return nil, errors.New("GitHub refused the token (401)")
		}
		return nil, fmt.Errorf("GitHub: %s", resp.Status)
	}
	return resp, nil
}

func (c *client) json(url string, v any) error {
	resp, err := c.get(url, "application/vnd.github+json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *client) download(url string) ([]byte, error) {
	resp, err := c.get(url, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func find(assets []asset, name string) *asset {
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}

// verify checks data against its line in a sha256sum list.
func verify(data, list []byte, name string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	sc := bufio.NewScanner(bytes.NewReader(list))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			if !strings.EqualFold(f[0], got) {
				return fmt.Errorf("%s: checksum mismatch", name)
			}
			return nil
		}
	}
	return fmt.Errorf("checksums.txt has no %s", name)
}

// extract takes the fdev binary out of a release archive.
func extract(data []byte, name string) ([]byte, error) {
	if strings.HasSuffix(name, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == "fdev.exe" {
				r, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer r.Close()
				return io.ReadAll(r)
			}
		}
		return nil, fmt.Errorf("%s has no fdev.exe", name)
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s has no fdev", name)
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == "fdev" {
			return io.ReadAll(tr)
		}
	}
}

// installed is the fdev to replace: the running one, or under `go run`,
// which runs a build in a temporary folder, the fdev on the PATH.
func installed() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if strings.Contains(exe, string(filepath.Separator)+"go-build") {
		if exe, err = exec.LookPath(exeName()); err != nil {
			return "", errors.New("there is no fdev on your PATH to replace: install a release first (see the README), or build one where you want it: go build -o ~/bin/fdev .")
		}
	}
	return filepath.EvalSymlinks(exe)
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "fdev.exe"
	}
	return "fdev"
}

// replace puts bin at exe. Windows won't overwrite a running program but
// lets it be renamed, so it moves aside first.
func replace(exe string, bin []byte) error {
	tmp := exe + ".new"
	if err := os.WriteFile(tmp, bin, 0o755); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("can't write to %s: run it with the rights to change it (sudo), or reinstall", filepath.Dir(exe))
		}
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			os.Remove(tmp)
			return err
		}
		if err := os.Rename(tmp, exe); err != nil {
			_ = os.Rename(old, exe)
			return err
		}
		return nil
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
