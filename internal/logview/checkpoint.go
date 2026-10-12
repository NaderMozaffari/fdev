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
//	──────── ◇ 16:09:12 · 112 logs since the last ──────────────────────
//
// space (or the bar's ◆ checkpoint) puts one; fdev puts one by itself when
// the logs go quiet for a while (10 seconds, or as the settings say) or a
// page grows to pageSize logs. Each one closes a page (page.go): the logs
// before it go into the session's file. [ and ] jump between them and the
// bookmarks. They are saved with the session and come back when it is
// replayed.

// checkpointRaw starts a checkpoint's line in a saved session.
const checkpointRaw = "──── fdev: "

// isRule is whether e is a line across the logs, not a log: a clear, a
// checkpoint, or a page out of memory.
func (e *Entry) isRule() bool {
	return e.Level == "cleared" || e.Level == "checkpoint" || e.Level == "paged"
}

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
		text = "◇ " + now.Format("15:04:05") + " · " + text
		tone = ToneDim
	} else {
		m.checkpoints++
		text = fmt.Sprintf("◆ checkpoint %d · %s · %s", m.checkpoints, now.Format("15:04:05"), text)
	}
	m.closePage()
	m.saveRule(m.addCheckpoint(text, tone, now))
	m.pageOut()
	if !auto {
		m.setStatus(fmt.Sprintf("◆ checkpoint %d · [ and ] come back to it", m.checkpoints))
	}
}

func (m *Model) addCheckpoint(text string, tone Tone, at time.Time) *Entry {
	e := checkpointEntry(text, tone, at)
	m.entries = append(m.entries, e)
	m.sinceMark = 0
	m.marks++
	m.dirty = true
	return e
}

func checkpointEntry(text string, tone Tone, at time.Time) *Entry {
	return &Entry{Kind: KindTool, Level: "checkpoint", Text: text, TextTone: tone, Time: at, page: -1,
		Raw: []string{checkpointRaw + text + " ────"}}
}

// clearEntry is the line where the screen was cleared (motion.go); what
// says why and when: "cleared at 16:04:12".
func clearEntry(what string, at time.Time) *Entry {
	return &Entry{Kind: KindTool, Level: "cleared", Time: at, page: -1,
		Text: "──── " + what + " · what came before is in the saved log ────",
		Raw:  []string{checkpointRaw + what + " ────"}}
}

// ruleEntry takes a saved session's checkpoint or clear back as one; nil
// for any other line.
func ruleEntry(line string, at time.Time) *Entry {
	text, ok := strings.CutPrefix(line, checkpointRaw)
	if !ok {
		return nil
	}
	text = strings.TrimSuffix(text, " ────")
	switch {
	case strings.HasPrefix(text, "◆"):
		return checkpointEntry(text, ToneCyan, at)
	case strings.HasPrefix(text, "◇"):
		return checkpointEntry(text, ToneDim, at)
	case strings.Contains(text, "cleared at "):
		return clearEntry(text, at)
	}
	return nil
}

// autoCheckpoint puts a checkpoint once no log came for the settings' time,
// when logs came since the last one.
func (m *Model) autoCheckpoint() {
	quiet := m.look.CheckpointQuiet()
	if quiet > 0 && m.sinceMark > 0 && !m.exited && time.Since(m.lastLog) >= quiet {
		m.checkpoint(true)
	}
}
