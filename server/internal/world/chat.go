package world

import (
	"strings"
	"time"

	"saigon-noir/server/internal/protocol"
)

// ChatHook is a module that sees every chat line before the world sends it (moderation). Chat returns true when it
// took the line: the world then sends it nowhere.
type ChatHook interface {
	Chat(m Member, msg protocol.ClientMsg) bool
}

// chatRefused applies the chat rules to a line: "" when it may go out, else the error code telling its sender why
// not. A line that goes out is counted (the last line, global lines for slow mode).
func (w *World) chatRefused(m *member, msg protocol.ClientMsg) string {
	r := w.cfg.Chat
	now := w.now()
	same := strings.ToLower(strings.TrimSpace(msg.Text))
	if r.RepeatWithin > 0 && same == m.lastLine && now.Sub(m.lastLineAt) < r.RepeatWithin {
		return protocol.ErrRepeat
	}
	if msg.Ch != protocol.ChannelNearby {
		if r.GlobalAfter > 0 && (m.character == "" || now.Sub(m.since) < r.GlobalAfter) {
			return protocol.ErrNewPlayer
		}
		if w.slowMode(now) && now.Sub(m.lastGlobalAt) < r.SlowGap {
			return protocol.ErrSlow
		}
		m.lastGlobalAt = now
		w.globalAt = append(w.globalAt, now)
	}
	m.lastLine, m.lastLineAt = same, now
	return ""
}

// slowMode: more global lines than SlowLines within SlowWindow. Old times are dropped as it looks.
func (w *World) slowMode(now time.Time) bool {
	r := w.cfg.Chat
	if r.SlowLines <= 0 {
		return false
	}
	keep := 0
	for _, t := range w.globalAt {
		if now.Sub(t) < r.SlowWindow {
			w.globalAt[keep] = t
			keep++
		}
	}
	w.globalAt = w.globalAt[:keep]
	return len(w.globalAt) >= r.SlowLines
}
