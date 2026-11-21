package update

import (
	"net/http"
	"time"
)

// Release is a release that an installer takes fdev from, to put it
// somewhere other than over the running fdev (see installer/).
type Release struct {
	Tag     string
	Archive string // e.g. fdev_darwin_arm64.tar.gz
	Size    int64  // the archive's size in bytes
	rel     release
	c       *client
}

// Latest is repo's newest stable release and the newest release of the
// beta channel (a beta, or a stable release newer than every beta), each
// with its archive for goos and goarch. Either is nil when there is none,
// as for stable until the first stable release.
func Latest(repo, goos, goarch string) (stable, beta *Release, err error) {
	c := &client{http: &http.Client{Timeout: 5 * time.Minute}, token: token()}
	var all []release
	if err := c.json("https://api.github.com/repos/"+repo+"/releases?per_page=100", &all); err != nil {
		return nil, nil, err
	}
	pick := func(beta bool) *Release {
		r, ok := Newest(all, beta)
		if !ok {
			return nil
		}
		name := Asset(goos, goarch)
		a := find(r.Assets, name)
		if a == nil {
			return nil
		}
		return &Release{Tag: r.Tag, Archive: name, Size: a.Size, rel: r, c: c}
	}
	return pick(false), pick(true), nil
}

// Fetch downloads the archive, checks it against the release's
// checksums.txt and gives fdev's binary and the archive's sha256. read
// hears how many bytes have come so far (it may be nil).
func (r *Release) Fetch(read func(done, total int64)) ([]byte, string, error) {
	return r.c.binary(r.rel, *find(r.rel.Assets, r.Archive), find(r.rel.Assets, "checksums.txt"), read)
}
