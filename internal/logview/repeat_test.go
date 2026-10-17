package logview

import (
	"os"
	"strings"
	"testing"

	"github.com/NaderMozaffari/fdev/internal/config"
)

func TestRepeats(t *testing.T) {
	m := testModelLook(map[string]bool{}, config.Look{Save: true})
	m.o.Root, m.o.Target = t.TempDir(), "dev"
	if err := m.startRecording(); err != nil {
		t.Fatal(err)
	}
	retry := `I/flutter (1): ⟪fd⟫{"l":"info","t":"Billing","m":"retrying"}` + "\n"
	other := `I/flutter (1): ⟪fd⟫{"l":"info","t":"Billing","m":"gave up"}` + "\n"
	m.feed([]byte(retry + retry + retry + other + retry + retry))
	if len(m.entries) != 3 || m.entries[0].times() != 3 || m.entries[1].times() != 1 || m.entries[2].times() != 2 {
		t.Fatalf("%d rows", len(m.entries))
	}
	s := screen(m)
	if strings.Count(s, "retrying") != 2 || !strings.Contains(s, "retrying  ×3 · ") || !strings.Contains(s, "retrying  ×2 · ") {
		t.Errorf("screen:\n%s", s)
	}
	if m.counts["info"] != 6 {
		t.Errorf("counted %d info logs, want 6", m.counts["info"])
	}

	// An exception is folded once it is whole: the same header with another
	// stack is not the same log.
	crash := func(frame string) string {
		return "E/flutter (1): [ERROR:flutter/runtime] Unhandled Exception: boom\nE/flutter (1): #0      " + frame + "\n"
	}
	m.feed([]byte(crash("main (a.dart:1)") + crash("main (a.dart:1)") + crash("other (b.dart:2)") + other))
	var errs []int
	for _, e := range m.entries {
		if e.Level == "error" {
			errs = append(errs, e.times())
		}
	}
	if len(errs) != 2 || errs[0] != 2 || errs[1] != 1 {
		t.Errorf("exceptions folded as %v, want [2 1]", errs)
	}

	// Every one is in the file, and read back the same way.
	m.checkpoint(false)
	data, _ := os.ReadFile(m.file)
	if n := strings.Count(string(data), "retrying"); n != 5 {
		t.Errorf("%d retries in the file, want 5", n)
	}
	back, err := readEntries(m.file, 0, m.pages[0].to, m.started, m.started, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != m.pages[0].entries || back[0].times() != 3 {
		t.Errorf("read back %d logs (want %d), the first %d times", len(back), m.pages[0].entries, back[0].times())
	}
	var b strings.Builder
	writeEntry(&b, back[0])
	if !strings.Contains(b.String(), "retrying  ×3 · ") {
		t.Errorf("saved as %q", b.String())
	}

	// A checkpoint in between starts over.
	m.feed([]byte(other))
	if last := m.entries[len(m.entries)-1]; last.Level == "checkpoint" || last.times() != 1 {
		t.Errorf("folded across a checkpoint: %+v", last)
	}
}

func TestUpdateKey(t *testing.T) {
	m := testModelLook(map[string]bool{}, config.Look{})
	line := func(left, key string) string {
		return `I/flutter (1): ⟪fd⟫{"l":"info","t":"Mission","m":"` + left + `s left","u":"` + key + `"}` + "\n"
	}
	m.feed([]byte(line("3", "a") + line("2", "a") + `I/flutter (1): ⟪fd⟫{"l":"info","m":"other"}` + "\n" + line("1", "a") + line("9", "b")))
	if len(m.entries) != 3 {
		t.Fatalf("%d rows, want 3", len(m.entries))
	}
	if got := m.entries[0].Text; got != "1s left" {
		t.Errorf("first row says %q, want the last update", got)
	}
}
