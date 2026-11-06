package world

import (
	"strings"
	"testing"
	"time"

	"saigon-noir/server/internal/protocol"
)

// chatter joins a character made long ago (old enough for global chat).
func (h *harness) chatter(name string) (*Peer, *member) {
	return h.join(Arrival{Character: "c-" + name, Name: name, Since: h.clock.Add(-time.Hour)})
}

func (h *harness) say(p *Peer, ch, text string) {
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeChat, Ch: ch, Text: text})
}

// heard is the chat lines and error codes a peer got since the last call.
func (h *harness) heard(p *Peer) (lines []string, codes []string) {
	fs, _ := h.drain(p)
	for _, f := range fs {
		switch f.T {
		case protocol.TypeChat:
			lines = append(lines, f.Text)
		case protocol.TypeError:
			codes = append(codes, f.Code)
		}
	}
	return lines, codes
}

func TestChatIsFilteredAndARepeatDropped(t *testing.T) {
	h := newHarnessWith(t, func(c *Config) { c.Chat.Filter = func(s string) string { return strings.ReplaceAll(s, "bad", "***") } })
	a, _ := h.chatter("Lan")
	b, _ := h.chatter("Minh")
	h.heard(a)
	h.heard(b)
	h.say(a, protocol.ChannelGlobal, "a bad day")
	if lines, _ := h.heard(b); len(lines) != 1 || lines[0] != "a *** day" {
		t.Fatalf("filtered line: %v", lines)
	}
	h.tick(5 * time.Second)
	h.say(a, protocol.ChannelGlobal, "A BAD DAY ")
	if lines, _ := h.heard(b); len(lines) != 0 {
		t.Fatalf("the same line again went out: %v", lines)
	}
	if _, codes := h.heard(a); len(codes) != 1 || codes[0] != protocol.ErrRepeat {
		t.Fatalf("the sender heard %v", codes)
	}
	h.tick(31 * time.Second)
	h.say(a, protocol.ChannelGlobal, "a bad day")
	if lines, _ := h.heard(b); len(lines) != 1 {
		t.Fatalf("after a while the line may go again: %v", lines)
	}
}

func TestGlobalChatWaitsForNewCharactersAndGuests(t *testing.T) {
	h := newHarness(t)
	guest, _ := h.join(Arrival{Name: "Khách"})
	fresh, _ := h.join(Arrival{Character: "c-new", Name: "Mới", Since: h.clock})
	old, _ := h.chatter("Cũ")
	h.heard(guest)
	h.heard(fresh)
	h.heard(old)
	for _, p := range []*Peer{guest, fresh} {
		h.say(p, protocol.ChannelGlobal, "chào cả phố")
		if _, codes := h.heard(p); len(codes) != 1 || codes[0] != protocol.ErrNewPlayer {
			t.Fatalf("a new player posting in global: %v", codes)
		}
	}
	if lines, _ := h.heard(old); len(lines) != 0 {
		t.Fatalf("their lines went out: %v", lines)
	}
	// nearby is open to everyone (they all start in the same spot)
	h.say(guest, protocol.ChannelNearby, "chào")
	if lines, _ := h.heard(old); len(lines) != 1 {
		t.Fatalf("a guest's nearby line: %v", lines)
	}
	h.tick(3 * time.Minute)
	h.say(fresh, protocol.ChannelGlobal, "giờ được rồi")
	if lines, _ := h.heard(old); len(lines) != 1 {
		t.Fatalf("three minutes on: %v", lines)
	}
}

func TestBusyGlobalChatSlowsDown(t *testing.T) {
	h := newHarness(t)
	var peers []*Peer
	for i := range 13 {
		p, _ := h.chatter(string(rune('A' + i)))
		peers = append(peers, p)
	}
	listener, _ := h.chatter("Nghe")
	for _, p := range peers {
		h.heard(p)
	}
	h.heard(listener)
	// twelve lines in a few seconds: slow mode is on
	for i, p := range peers[:12] {
		h.say(p, protocol.ChannelGlobal, "dòng "+string(rune('a'+i)))
		h.tick(200 * time.Millisecond)
	}
	h.say(peers[0], protocol.ChannelGlobal, "lại nữa")
	if _, codes := h.heard(peers[0]); len(codes) != 1 || codes[0] != protocol.ErrSlow {
		t.Fatalf("a second line in slow mode: %v", codes)
	}
	// one who has not posted yet still may, once
	h.say(peers[12], protocol.ChannelGlobal, "lần đầu")
	if _, codes := h.heard(peers[12]); len(codes) != 0 {
		t.Fatalf("a first line in slow mode: %v", codes)
	}
	// it calms down: lines go freely again
	h.tick(11 * time.Second)
	h.say(peers[0], protocol.ChannelGlobal, "yên rồi")
	if _, codes := h.heard(peers[0]); len(codes) != 0 {
		t.Fatalf("after the rush: %v", codes)
	}
	if lines, _ := h.heard(listener); len(lines) != 14 {
		t.Fatalf("the listener heard %d lines, want 14", len(lines))
	}
}

// takeAll is a chat hook that takes every line starting with "/".
type takeAll struct{ seen []string }

func (t *takeAll) Handle(Member, protocol.ClientMsg) bool { return false }
func (t *takeAll) Tick(time.Time)                         {}
func (t *takeAll) Left(Member)                            {}
func (t *takeAll) Chat(m Member, msg protocol.ClientMsg) bool {
	t.seen = append(t.seen, msg.Text)
	return strings.HasPrefix(msg.Text, "/")
}

func TestAChatHookSeesLinesFirstAndMayTakeThem(t *testing.T) {
	h := newHarness(t)
	hook := &takeAll{}
	h.w.Use(func(Host) Module { return hook })
	a, _ := h.chatter("Lan")
	b, _ := h.chatter("Minh")
	h.heard(a)
	h.heard(b)
	h.say(a, protocol.ChannelGlobal, "/mute Minh")
	h.say(a, protocol.ChannelGlobal, "chào")
	if lines, _ := h.heard(b); len(lines) != 1 || lines[0] != "chào" {
		t.Fatalf("Minh heard %v", lines)
	}
	if len(hook.seen) != 2 {
		t.Fatalf("the hook saw %v", hook.seen)
	}
}
