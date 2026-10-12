package logview

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/NaderMozaffari/fdev/internal/config"
)

func TestCheckpoints(t *testing.T) {
	root := t.TempDir()
	m := testModelLook(map[string]bool{}, config.Look{Save: true})
	m.o.Root, m.o.Target = root, "dev"
	if err := m.startRecording(); err != nil {
		t.Fatal(err)
	}
	m.feed([]byte(sample))

	// space puts one, with how many logs came before it.
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	s := screen(m)
	if !strings.Contains(s, "━ ◆ checkpoint 1 · ") || !strings.Contains(s, "logs since the start ━") {
		t.Fatalf("no checkpoint:\n%s", s)
	}

	// By itself: once the logs were quiet a while, and only when logs came
	// since the last one.
	m.lastLog = time.Now().Add(-time.Minute)
	m.autoCheckpoint()
	if n := strings.Count(screen(m), "◇"); n != 0 {
		t.Fatalf("a checkpoint without logs since the last: %d", n)
	}
	m.feed([]byte(`I/flutter (1): ⟪fd⟫{"l":"info","m":"later"}` + "\n"))
	if m.autoCheckpoint(); strings.Contains(screen(m), "◇") {
		t.Fatal("a checkpoint while logs still come")
	}
	m.lastLog = time.Now().Add(-11 * time.Second)
	m.autoCheckpoint()
	if s := screen(m); !strings.Contains(s, "◇ ") || !strings.Contains(s, " · 1 log since the last") {
		t.Fatalf("no checkpoint by itself:\n%s", s)
	}
	m.autoCheckpoint()
	if n := strings.Count(screen(m), "◇"); n != 1 {
		t.Errorf("%d checkpoints by themselves, want 1", n)
	}
	off := testModelLook(nil, config.Look{Checkpoint: config.CheckpointOff})
	off.feed([]byte(sample))
	off.lastLog = time.Now().Add(-time.Hour)
	if off.autoCheckpoint(); off.marks != 0 {
		t.Error("a checkpoint with them off")
	}

	// [ comes back to it.
	m.Update(keyText("["))
	if m.lit == nil || m.lit.Level != "checkpoint" {
		t.Errorf("[ lit %+v", m.lit)
	}

	// Saved, and back when the session is replayed.
	m.stopRecording()
	raws, _ := filepath.Glob(filepath.Join(root, ".fdev", "logs", "*.raw.log"))
	if len(raws) != 1 {
		t.Fatalf("raw logs: %v", raws)
	}
	r := testModel(map[string]bool{})
	r.o.Replay = raws[0]
	r.replay()
	var kinds []string
	for _, e := range r.entries {
		if e.Level == "checkpoint" {
			kinds = append(kinds, e.Text[:len("◆")])
		}
	}
	if strings.Join(kinds, "") != "◆◇" {
		t.Errorf("replayed checkpoints: %q", kinds)
	}
}
