package logview

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Pages keep a long session light. The logs between two checkpoints are a
// page: when a checkpoint is put, the page before it goes into the
// session's file in one write (not line by line as they come), and once
// more than keepLoaded logs are in memory the oldest pages leave it. Each
// one left is a row in its place:
//
//	┄┄┄┄┄┄┄┄ ⋯ 112 logs here, in the session's file · click to load ┄┄┄┄┄┄┄┄
//	━━━━━━━━ ◆ checkpoint 2 · 16:04:12 · 38 logs since the last ━━━━━━━━━━
//
// A click on it, or [ and ] reaching the checkpoint next to it, reads the
// page back from the file (the row says "loading" meanwhile). Pages leave
// memory only while the logs follow the output, so the ones you went back
// to stay until you come back down. A replayed session is read the same
// way: its newest pages first, the rest when you go to them.
type page struct {
	from, to int64     // where its lines are in the file
	at       time.Time // the time before its first line, for lines without one
	entries  int       // how many logs it had, to put the kept ones back in place
	lines    int       // for a replay, which counts lines, not logs
	loaded   bool
	loading  bool
	counted  bool // in the counts already (a replay's pages are counted as they load)
	gone     bool // before a clear: not offered again
	// kept is the logs with something done to them (bookmarked, pinned,
	// hidden, open, selected), by their place in the page: they come back
	// as they were, not read again.
	kept map[int]*Entry
	row  *Entry // the row in its place while it is out of memory
}

const (
	keepLoaded = 2000 // logs in memory, at most, while following the output
	pageSize   = 1000 // a page this long closes with a checkpoint by itself
)

type pagedMsg struct {
	page    int
	entries []*Entry
	err     error
}

// closePage writes the logs not in the file yet to it, and starts a new
// page. It does nothing without a file: everything stays in memory.
func (m *Model) closePage() {
	m.live = 0
	if m.raw == nil {
		return
	}
	k := len(m.pages)
	p := &page{from: m.size, loaded: true, counted: true}
	var b strings.Builder
	for _, e := range m.entries {
		if e.saved || e.Level == "paged" {
			continue
		}
		e.saved = true
		for _, r := range e.Raw {
			b.WriteString(rawLine(e.Time, r) + "\n")
		}
		if e.page == k {
			if p.entries == 0 {
				p.at = e.Time
			}
			p.entries++
		}
	}
	m.writeRaw(b.String())
	p.to = m.size
	if p.entries > 0 {
		m.pages = append(m.pages, p)
	}
}

// writeRaw adds to the session's file.
func (m *Model) writeRaw(s string) {
	if m.raw == nil || s == "" {
		return
	}
	n, err := m.raw.WriteString(s)
	m.size += int64(n)
	if err != nil {
		m.setStatus("✘ can't save logs: " + err.Error())
	}
}

// saveRule writes a checkpoint or a clear to the file, after the page
// before it.
func (m *Model) saveRule(e *Entry) {
	if m.raw == nil {
		return
	}
	e.saved = true
	var b strings.Builder
	for _, r := range e.Raw {
		b.WriteString(rawLine(e.Time, r) + "\n")
	}
	m.writeRaw(b.String())
}

// pageOut takes the oldest pages out of memory, while the logs follow the
// output and more than keepLoaded are in it. The newest page stays.
func (m *Model) pageOut() {
	if !m.follow || m.file == "" {
		return
	}
	n := 0
	for _, e := range m.entries {
		if e.page >= 0 && e.Level != "paged" {
			n++
		}
	}
	for k := 0; k < len(m.pages)-1 && n > keepLoaded; k++ {
		if p := m.pages[k]; p.loaded && !p.gone {
			n -= m.unload(k)
		}
	}
}

// unload puts page k's row in place of its logs; it is how many left.
func (m *Model) unload(k int) int {
	p := m.pages[k]
	p.kept = map[int]*Entry{}
	out := m.entries[:0]
	n := 0
	for _, e := range m.entries {
		if e.page != k {
			out = append(out, e)
			continue
		}
		if e.bookmarked || e.pinned || e.hidden || e.Expanded || e.Request != nil || e.Response != nil ||
			e == m.sel || e == m.anchor || e == m.jumped {
			p.kept[n] = e
		}
		if n == 0 {
			p.row = &Entry{Kind: KindTool, Level: "paged", Time: e.Time, page: k, saved: true}
			m.pageRow(p, "")
			out = append(out, p.row)
		}
		n++
	}
	clear(m.entries[len(out):])
	m.entries = out
	p.loaded = false
	m.dirty = true
	return n
}

// pageRow sets what a page's row says.
func (m *Model) pageRow(p *page, err string) {
	what := plural(p.entries, "log")
	if p.entries == 0 {
		what = plural(p.lines, "line")
	}
	switch {
	case err != "":
		p.row.Text, p.row.TextTone = "✘ can't load "+what+": "+err, ToneRed
	case p.loading:
		p.row.Text, p.row.TextTone = spinFrame()+" loading "+what+"…", ToneCyan
	default:
		p.row.Text, p.row.TextTone = "⋯ "+what+" here, in the session's file · click to load", ToneDim
	}
	p.row.cached = cacheKey{}
	m.dirty = true
}

// pageIn reads page k back from the file, in the background.
func (m *Model) pageIn(k int) tea.Cmd {
	if k < 0 || k >= len(m.pages) {
		return nil
	}
	p := m.pages[k]
	if p.loaded || p.loading || p.gone {
		return nil
	}
	p.loading = true
	m.pageRow(p, "")
	path, from, to, at, day, values := m.file, p.from, p.to, p.at, m.started, m.o.Values
	return func() tea.Msg {
		entries, err := readEntries(path, from, to, day, at, values)
		return pagedMsg{page: k, entries: entries, err: err}
	}
}

// paged puts a page that was read back in its place.
func (m *Model) paged(msg pagedMsg) tea.Cmd {
	p := m.pages[msg.page]
	p.loading = false
	if msg.err != nil {
		m.pageRow(p, msg.err.Error())
		m.after = nil
		return nil
	}
	i := indexOf(m.entries, p.row)
	if i < 0 { // cleared meanwhile
		return nil
	}
	m.refresh()
	before := len(m.rows)
	m.place(msg.page, i, msg.entries)
	// What was on the screen stays where it was: the page opens above it.
	if m.after == nil && !m.follow {
		offset := m.vp.YOffset()
		m.refresh()
		m.vp.SetYOffset(offset + len(m.rows) - before)
	}
	if m.after != nil && !m.loadingPages() {
		e := m.after
		m.after = nil
		return m.jumpTo(e)
	}
	return nil
}

// place puts page k's logs at m.entries[i], in place of its row.
func (m *Model) place(k, i int, entries []*Entry) {
	p := m.pages[k]
	if len(entries) == p.entries {
		for n, e := range p.kept {
			entries[n] = e
		}
	}
	for _, e := range entries {
		e.page, e.saved = k, true
		m.growTag(tagWidth(e))
		if !p.counted {
			m.counts[e.Category]++
			if e.Category == "error" {
				m.errors++
			}
		}
	}
	if p.entries == 0 {
		p.entries = len(entries)
	}
	p.loaded, p.counted, p.kept, p.row = true, true, nil, nil
	m.entries = append(m.entries[:i], append(entries, m.entries[i+1:]...)...)
	m.dirty = true
}

func (m *Model) loadingPages() bool {
	for _, p := range m.pages {
		if p.loading {
			return true
		}
	}
	return false
}

// reach jumps to e, a log or a checkpoint; the pages next to a checkpoint
// are read back first, so that what came before and after it is there.
func (m *Model) reach(e *Entry) tea.Cmd {
	var cmds []tea.Cmd
	if e.Level == "checkpoint" {
		if i := indexOf(m.entries, e); i >= 0 {
			for _, j := range []int{i - 1, i + 1} {
				if j >= 0 && j < len(m.entries) && m.entries[j].Level == "paged" {
					cmds = append(cmds, m.pageIn(m.entries[j].page))
				}
			}
		}
	}
	if len(cmds) == 0 {
		return m.jumpTo(e)
	}
	m.after, m.jumped = e, e
	return tea.Batch(cmds...)
}

// sessionSize is how many logs the session has, in memory or not.
func (m *Model) sessionSize() int {
	n := 0
	for _, e := range m.entries {
		if e.Level == "paged" {
			n += m.pages[e.page].entries
		} else {
			n++
		}
	}
	return n
}

// allEntries is the whole session, read back from its file; the logs in
// memory when there is no file.
func (m *Model) allEntries() []*Entry {
	if m.file == "" {
		return m.entries
	}
	m.closePage()
	entries, err := readEntries(m.file, 0, -1, m.started, m.started, m.o.Values)
	if err != nil {
		return m.entries
	}
	return entries
}

// readEntries reads the logs in path's bytes [from, to) (to the end when to
// is -1); day is the session's start, at the time of the line before.
func readEntries(path string, from, to int64, day, at time.Time, values []string) ([]*Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if to < 0 {
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		to = info.Size()
	}
	buf := make([]byte, max(to-from, 0))
	if _, err := f.ReadAt(buf, from); err != nil && len(buf) > 0 {
		return nil, err
	}
	p := Parser{ValueKeys: values}
	var out []*Entry
	last := at
	for _, line := range strings.Split(strings.TrimRight(string(buf), "\n"), "\n") {
		var t time.Time
		t, line = splitRaw(strings.TrimSuffix(line, "\r"), day, last)
		last = t
		if line == "" {
			continue
		}
		if rule := ruleEntry(line, t); rule != nil {
			out = append(out, rule)
			continue
		}
		if e := p.Line(line, t).New; e != nil {
			out = append(out, e)
		}
	}
	p.Abandon()
	return out, nil
}

// startRecording opens the session's file, which the pages go into. With
// saving off it goes when the viewer closes, unless the screen was cleared.
func (m *Model) startRecording() error {
	if m.raw != nil || m.o.Replay != "" {
		return nil
	}
	dir, err := logsDir(m.o.Root, m.look.Dir)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, m.sessionBase()+".raw.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	m.raw, m.file, m.size = f, f.Name(), info.Size()
	prune(dir)
	return nil
}

// recording is whether the session's file is kept: saving is on, or the
// screen was cleared.
func (m *Model) recording() bool { return m.raw != nil && (m.look.Save || m.keepRaw) }

// stopRecording writes what is left and closes the file; with saving off,
// the file goes.
func (m *Model) stopRecording() {
	if m.raw == nil {
		return
	}
	m.closePage()
	_ = m.raw.Close()
	if !m.recording() {
		_ = os.Remove(m.file)
	}
	m.raw = nil
}

// replay shows a saved session: its checkpoints mark its pages, the newest
// are read now, and the rest when you go to them.
func (m *Model) replay() {
	m.exited = true
	if !strings.HasSuffix(m.o.Replay, ".raw.log") {
		m.o.Plain = true // already drawn: just text
	}
	if info, err := os.Stat(m.o.Replay); err == nil {
		m.started = info.ModTime()
	}
	if base := filepath.Base(m.o.Replay); len(base) > len(timeLayout) {
		if t, err := time.ParseInLocation(timeLayout, base[:len(timeLayout)], time.Local); err == nil {
			m.started = t
		}
	}
	if err := m.scanPages(m.o.Replay); err != nil {
		m.err, m.code = err, -1
		m.entries = append(m.entries, &Entry{Kind: KindTool, Text: "can't read it: " + err.Error(), TextTone: ToneRed, Time: time.Now(), page: -1})
		return
	}
	m.file = m.o.Replay
	// The newest pages, up to keepLoaded logs.
	n := 0
	for k := len(m.pages) - 1; k >= 0 && (n < keepLoaded || k == len(m.pages)-1); k-- {
		p := m.pages[k]
		entries, err := readEntries(m.file, p.from, p.to, m.started, p.at, m.o.Values)
		if err != nil {
			m.pageRow(p, err.Error())
			break
		}
		m.place(k, indexOf(m.entries, p.row), entries)
		n += len(entries)
	}
	for i := len(m.entries) - 1; i >= 0; i-- {
		if e := m.entries[i]; e.Level != "paged" {
			m.took = e.Time.Sub(m.started)
			break
		}
	}
	m.dirty = true
}

// scanPages finds a saved session's pages, between its checkpoints, without
// reading its logs: a row for each page, and its checkpoints.
func (m *Model) scanPages(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64*1024)
	var off, from int64
	lines, at := 0, m.started
	add := func(to int64) {
		if lines == 0 {
			return
		}
		p := &page{from: from, to: to, at: at, lines: lines}
		p.row = &Entry{Kind: KindTool, Level: "paged", Time: at, page: len(m.pages), saved: true}
		m.pages = append(m.pages, p)
		m.pageRow(p, "")
		m.entries = append(m.entries, p.row)
	}
	for {
		line, err := r.ReadString('\n')
		start := off
		off += int64(len(line))
		var rule *Entry
		if strings.Contains(line, checkpointRaw) {
			t, text := splitRaw(strings.TrimRight(line, "\r\n"), m.started, at)
			if rule = ruleEntry(text, t); rule != nil && rule.Level != "checkpoint" {
				rule = nil // a clear stays in its page
			}
		}
		switch {
		case rule != nil:
			add(start)
			m.entries = append(m.entries, rule)
			m.marks++
			from, lines, at = off, 0, rule.Time
		case line != "":
			lines++
		}
		if err != nil {
			break
		}
	}
	add(off)
	return nil
}

// bookmark is the page's last bookmarked log going up (dir < 0), or its
// first going down, while the page is out of memory.
func (p *page) bookmark(dir int) *Entry {
	at := -1
	for n, e := range p.kept {
		if e.bookmarked && (at < 0 || (dir < 0) == (n > at)) {
			at = n
		}
	}
	return p.kept[at]
}
