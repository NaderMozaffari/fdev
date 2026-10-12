package logview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
)

func appLogs(from, to int) []byte {
	var b strings.Builder
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, `I/flutter (1): ⟪fd⟫{"l":"info","m":"log %d"}`+"\n", i)
	}
	return []byte(b.String())
}

// readPages hands the model the pages a command reads, as the program
// would; other messages (the motion's ticks) are left.
func readPages(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			readPages(m, c)
		}
	case pagedMsg:
		m.Update(msg)
	}
}

func TestPages(t *testing.T) {
	root := t.TempDir()
	m := testModelLook(map[string]bool{}, config.Look{Save: true})
	m.o.Root, m.o.Target = root, "dev"
	if err := m.startRecording(); err != nil {
		t.Fatal(err)
	}
	m.feed(appLogs(0, 10))
	if data, _ := os.ReadFile(m.file); len(data) != 0 {
		t.Fatalf("logs went into the file before a checkpoint:\n%s", data)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if data, _ := os.ReadFile(m.file); !strings.Contains(string(data), "log 9") || !strings.Contains(string(data), "◆ checkpoint 1") {
		t.Fatalf("the checkpoint didn't put the logs before it in the file:\n%s", data)
	}
	m.entries[3].bookmarked = true // "log 3"

	// Long pages close by themselves, and the oldest leave memory.
	m.feed(appLogs(10, 3500))
	if len(m.pages) != 4 || m.pages[0].loaded {
		t.Fatalf("%d pages, the first loaded: %v", len(m.pages), m.pages[0].loaded)
	}
	if n := len(m.entries); n > keepLoaded+pageSize {
		t.Errorf("%d entries in memory", n)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	if s := screen(m); !strings.Contains(s, "⋯ 10 logs here, in the session's file · click to load") {
		t.Fatalf("no row for the page out of memory:\n%s", s)
	}

	// [ goes up to the first checkpoint, and reads the page before it back.
	for i := 0; i < 8 && (m.lit == nil || !strings.Contains(m.lit.Text, "checkpoint 1")); i++ {
		readPages(m, m.Update(keyText("[")))
	}
	if m.lit == nil || !strings.Contains(m.lit.Text, "checkpoint 1") || !m.pages[0].loaded {
		t.Fatalf("[ didn't come back to the first checkpoint: %+v", m.lit)
	}
	if !strings.Contains(m.entries[0].Text, "log 0") || !m.entries[3].bookmarked {
		t.Errorf("the page read back: %q, bookmarked %v", m.entries[0].Text, m.entries[3].bookmarked)
	}

	// The whole session is saved, whatever is in memory.
	path, err := m.save()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, path))
	if !strings.Contains(string(data), "log 0\n") || !strings.Contains(string(data), "log 3499\n") {
		t.Error("the saved log misses logs")
	}

	// A replay reads its newest pages, and the rest when asked.
	m.stopRecording()
	r := testModel(map[string]bool{})
	r.o.Replay = m.file
	r.replay()
	if len(r.pages) != 5 || r.pages[0].loaded || !r.pages[4].loaded {
		t.Fatalf("replayed pages: %d", len(r.pages))
	}
	readPages(r, r.pageIn(0))
	if !r.pages[0].loaded || !strings.Contains(r.entries[0].Text, "log 0") {
		t.Errorf("the replay's first page: %q", r.entries[0].Text)
	}
}

// With saving off the file is only for the pages: it goes at the end,
// unless the screen was cleared.
func TestPagesFileGoes(t *testing.T) {
	m := testModel(nil)
	m.o.Root, m.o.Target = t.TempDir(), "dev"
	if err := m.startRecording(); err != nil {
		t.Fatal(err)
	}
	m.feed(appLogs(0, 3))
	m.checkpoint(false)
	file := m.file
	m.stopRecording()
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("the file stayed: %v", err)
	}
}
