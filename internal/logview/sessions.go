package logview

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Session is a saved run in the logs directory: its readable log (.log),
// its raw output (.raw.log), or both.
type Session struct {
	Base    string // the file name without extension: <time>_<target>
	Target  string
	Time    time.Time
	Command string // what ran, from the readable log's header
	Note    string // e.g. the device
	Subject string // what a hand-picked save is about
	Tags    []string
	Raw     string // absolute paths; "" when missing
	Log     string
	Size    int64
	Starred bool
}

// Path is the file to replay: the raw output when there is one, since it
// keeps everything the viewer can show.
func (s Session) Path() string {
	if s.Raw != "" {
		return s.Raw
	}
	return s.Log
}

const (
	starFile   = ".starred"
	timeLayout = "2006-01-02_15-04-05"
)

func sessionsDir(root, dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(root, dir)
}

// Sessions lists the saved runs, starred first, then newest first.
func Sessions(root, dir string) []Session {
	path := sessionsDir(root, dir)
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	stars := readStars(path)
	byBase := map[string]*Session{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".log") {
			continue
		}
		raw := strings.HasSuffix(name, ".raw.log")
		base := strings.TrimSuffix(strings.TrimSuffix(name, ".log"), ".raw")
		s := byBase[base]
		if s == nil {
			s = &Session{Base: base, Starred: stars[base]}
			if len(base) > len(timeLayout) {
				s.Time, _ = time.ParseInLocation(timeLayout, base[:len(timeLayout)], time.Local)
				s.Target = base[len(timeLayout)+1:]
			}
			byBase[base] = s
		}
		full := filepath.Join(path, name)
		if info, err := e.Info(); err == nil {
			s.Size += info.Size()
		}
		if raw {
			s.Raw = full
		} else {
			s.Log = full
			h := readHeader(full)
			s.Command, s.Note, s.Subject, s.Tags = h.command, h.note, h.subject, h.tags
		}
	}
	var out []Session
	for _, s := range byBase {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Starred != out[j].Starred {
			return out[i].Starred
		}
		return out[i].Base > out[j].Base
	})
	return out
}

type header struct {
	command, note, subject string
	tags                   []string
}

// readHeader is what save writes first: "# title · command · started ...",
// then "# note", "# subject: ...", "# tags: a, b" when there are.
func readHeader(path string) header {
	var h header
	f, err := os.Open(path)
	if err != nil {
		return h
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for i := 0; i < 6 && sc.Scan(); i++ {
		line, ok := strings.CutPrefix(sc.Text(), "# ")
		if !ok {
			break
		}
		switch {
		case i == 0:
			if parts := strings.Split(line, " · "); len(parts) >= 3 {
				h.command = parts[1]
			}
		case strings.HasPrefix(line, "subject: "):
			h.subject = strings.TrimPrefix(line, "subject: ")
		case strings.HasPrefix(line, "tags: "):
			h.tags = splitTags(strings.TrimPrefix(line, "tags: "))
		case strings.HasPrefix(line, "logs: "):
		case h.note == "":
			h.note = line
		}
	}
	return h
}

func readStars(dir string) map[string]bool {
	stars := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(dir, starFile))
	if err != nil {
		return stars
	}
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			stars[l] = true
		}
	}
	return stars
}

// Star marks a session (starred ones are never pruned) or unmarks it.
func Star(root, dir, base string, on bool) error {
	path, err := logsDir(root, dir)
	if err != nil {
		return err
	}
	stars := readStars(path)
	if on {
		stars[base] = true
	} else {
		delete(stars, base)
	}
	var lines []string
	for b := range stars {
		lines = append(lines, b)
	}
	sort.Strings(lines)
	return os.WriteFile(filepath.Join(path, starFile), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// Delete removes a session's files and its star.
func Delete(root, dir string, s Session) error {
	for _, p := range []string{s.Raw, s.Log} {
		if p != "" {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	if s.Starred {
		return Star(root, dir, s.Base, false)
	}
	return nil
}

// Raw lines are saved after the time they arrived, so a replay shows it.
const rawTime = "15:04:05.000"

var rawTimePrefix = regexp.MustCompile(`^(\d\d:\d\d:\d\d\.\d{3})\t`)

// rawLine is how a line goes in the raw log.
func rawLine(t time.Time, line string) string {
	return t.Format(rawTime) + "\t" + line
}

// splitRaw takes the time off a raw log line, on day (the session's start);
// lines saved without one get the time before.
func splitRaw(line string, day, before time.Time) (time.Time, string) {
	m := rawTimePrefix.FindStringSubmatch(line)
	if m == nil {
		return before, line
	}
	clock, err := time.ParseInLocation(rawTime, m[1], time.Local)
	if err != nil {
		return before, line
	}
	t := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), clock.Second(),
		clock.Nanosecond(), time.Local)
	if t.Before(day.Add(-time.Minute)) { // after midnight
		t = t.AddDate(0, 0, 1)
	}
	return t, line[len(m[0]):]
}
