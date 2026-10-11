package logview

import (
	"fmt"
	"strings"
	"time"
)

// Checkpoints mark where you were in the logs: a line across them with the
// time and how many logs came since the one before, to find your place
// again after looking away.
//
//	━━━━━━━━ ◆ checkpoint 2 · 16:04:12 · 38 logs since the last ━━━━━━━━
//	──────── ◇ 16:09:12 · every 5m · 112 logs since the last ────────────
//
// space (or the bar's ◆ checkpoint) puts one; the settings put one every few
// minutes by themselves, counted from the last one of either kind, when logs
// came since. [ and ] jump between them and the bookmarks. They are saved
// with the session and come back when it is replayed.

// checkpointRaw starts a checkpoint's line in a saved session.
const checkpointRaw = "──── fdev: "

// isRule is whether e is a line across the logs, not a log: a clear or a
// checkpoint.
func (e *Entry) isRule() bool { return e.Level == "cleared" || e.Level == "checkpoint" }

// checkpoint puts a checkpoint after the logs so far.
func (m *Model) checkpoint(auto bool) {
	now := time.Now()
	since := "since the start"
	if m.marks > 0 {
		since = "since the last"
	}
	text := plural(m.sinceMark, "log") + " " + since
	tone := ToneCyan
	if auto {
		text = "◇ " + now.Format("15:04:05") + " · every " + m.look.Checkpoint + " · " + text
		tone = ToneDim
	} else {
		m.checkpoints++
		text = fmt.Sprintf("◆ checkpoint %d · %s · %s", m.checkpoints, now.Format("15:04:05"), text)
	}
	m.addCheckpoint(text, tone, now)
	if m.raw != nil {
		_, _ = m.raw.WriteString(rawLine(now, checkpointRaw+text+" ────") + "\n")
	}
	if !auto {
		m.setStatus(fmt.Sprintf("◆ checkpoint %d · [ and ] come back to it", m.checkpoints))
	}
}

func (m *Model) addCheckpoint(text string, tone Tone, at time.Time) {
	e := &Entry{Kind: KindTool, Level: "checkpoint", Text: text, TextTone: tone, Time: at}
	e.Raw = []string{checkpointRaw + text + " ────"}
	m.entries = append(m.entries, e)
	m.sinceMark, m.lastMark = 0, at
	m.marks++
	m.dirty = true
}

// replayCheckpoint takes a saved session's checkpoint line back as one.
func (m *Model) replayCheckpoint(line string, at time.Time) bool {
	text, ok := strings.CutPrefix(line, checkpointRaw)
	if !ok || !(strings.HasPrefix(text, "◆") || strings.HasPrefix(text, "◇")) {
		return false
	}
	tone := ToneCyan
	if strings.HasPrefix(text, "◇") {
		tone = ToneDim
	}
	m.addCheckpoint(strings.TrimSuffix(text, " ────"), tone, at)
	return true
}

// autoCheckpoint puts a checkpoint when the settings' time has passed since
// the last one and logs came since.
func (m *Model) autoCheckpoint() {
	every := m.look.CheckpointEvery()
	last := m.lastMark
	if last.IsZero() {
		last = m.started
	}
	if every > 0 && m.sinceMark > 0 && !m.exited && time.Since(last) >= every {
		m.checkpoint(true)
	}
}
