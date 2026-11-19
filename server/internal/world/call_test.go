package world

import (
	"context"
	"errors"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// numbers is who has which numbers, as the store would say.
type numbers map[string][]string

func (n numbers) HasContact(_ context.Context, character, contact string) (bool, error) {
	if character == "char-broken" {
		return false, errors.New("disk full")
	}
	for _, c := range n[character] {
		if c == contact {
			return true, nil
		}
	}
	return false, nil
}

// callHarness is a world with the game's NPCs and voices, and the numbers given; the events published are kept.
func callHarness(t *testing.T, book numbers) (*harness, *[]events.Event) {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	var published []events.Event
	h := newHarnessWith(t, func(cfg *Config) {
		cfg.NPCs, cfg.Voices, cfg.Dialogue, cfg.Contacts = NPCsFrom(c), VoicesFrom(c), dialogue.NewScript(c), book
		cfg.OnEvent = func(ev events.Event) { published = append(published, ev) }
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.w.jobs.start(ctx)
	return h, &published
}

// call sends a call and runs the job's answer, which the loop would do.
func (h *harness) call(p *Peer, contact string, rid uint32) {
	h.t.Helper()
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeCall, Contact: contact, Rid: rid})
	select {
	case e := <-h.w.events:
		h.w.handle(e)
	case <-time.After(150 * time.Millisecond):
		// a call refused before the store is asked has no job: its result is already queued
	}
}

var withPhone = capability.Of(content.CapPhone)

func TestACallStartsWithoutDistance(t *testing.T) {
	h, published := callHarness(t, numbers{"char-Lan": {"chu_sau", "ma"}})
	sau := h.npc("chu_sau")
	// 150 m away from Chú Sáu, on a phone
	p, m := h.join(Arrival{Name: "Lan", Character: "char-Lan", Arch: "newcomer", Caps: withPhone,
		Start: &protocol.Pose{X: sau.pose.X + 150, Z: sau.pose.Z}})
	h.drain(p)
	h.call(p, "chu_sau", 7)
	fs, _ := h.drain(p)
	if ok, code := resultOf(t, fs); !ok {
		t.Fatalf("the call was refused: %s", code)
	}
	in := only(t, fs, protocol.TypeInteraction)
	if in.Call != "chu_sau" || in.Mode != protocol.ModePrivate || !in.Lead || len(in.Choices) != 1 {
		t.Fatalf("interaction %+v", in)
	}
	if want := "Alô, chú Sáu nghe"; len(in.Say.VI) < len(want) || in.Say.VI[:len(want)] != want {
		t.Fatalf("the call's first line is %q, want the call node", in.Say.VI)
	}
	if m.talk == nil || !m.talk.call {
		t.Fatal("the player is not on a call")
	}
	// walking further and further does not end it
	h.w.walkAways()
	if m.talk == nil {
		t.Fatal("distance ended a call")
	}
	// Chú Sáu does not turn toward a caller 150 m away, and a call is not "talking to" him
	if sau.talks != 0 {
		t.Fatalf("a call counts as %d conversations with the NPC", sau.talks)
	}
	for _, ev := range *published {
		if ev.Kind == events.NPCTalked {
			t.Fatalf("a call published %+v: only a face to face talk meets someone", ev)
		}
	}
	// the goodbye ends it
	h.choose(p, in.Sid, 0, 8)
	fs, _ = h.drain(p)
	if bye := only(t, fs, protocol.TypeInteraction); len(bye.Choices) != 0 {
		t.Fatalf("the closing line has answers: %+v", bye)
	}
	// only goodbye is left: the player hangs up
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeInteractLeave, Sid: in.Sid})
	if m.talk != nil || len(h.w.sessions) != 0 {
		t.Fatal("the call did not end")
	}
}

func TestACallIsChecked(t *testing.T) {
	cases := []struct {
		name    string
		who     Arrival
		contact string
		want    string
	}{
		{"without a phone", Arrival{Name: "Lan", Character: "char-Lan"}, "chu_sau", protocol.ResPhone},
		{"a number it does not have", Arrival{Name: "Lan", Character: "char-Lan", Caps: withPhone}, "co_ba", protocol.ResTarget},
		{"nobody by that id", Arrival{Name: "Lan", Character: "char-Lan", Caps: withPhone}, "nobody", protocol.ResTarget},
		{"a guest has no numbers", Arrival{Name: "Khách", Caps: withPhone}, "chu_sau", protocol.ResTarget},
		{"the store is down", Arrival{Name: "Lan", Character: "char-broken", Caps: withPhone}, "chu_sau", protocol.ResBusy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, _ := callHarness(t, numbers{"char-Lan": {"chu_sau"}, "char-broken": {"chu_sau"}})
			p, m := h.join(c.who)
			h.drain(p)
			h.call(p, c.contact, 5)
			fs, _ := h.drain(p)
			ok, code := resultOf(t, fs)
			if ok || code != c.want {
				t.Fatalf("result ok=%v code=%q, want %q", ok, code, c.want)
			}
			if m.talk != nil {
				t.Fatal("a refused call started a conversation")
			}
		})
	}
}

func TestAVoiceWithNoBodyCanBeCalled(t *testing.T) {
	h, published := callHarness(t, numbers{"char-Lan": {"ma"}})
	for _, n := range h.w.npcs {
		if n.spec.Key == "ma" {
			t.Fatal("Má is on the island")
		}
	}
	if list := h.w.npcList(); len(list) != len(h.w.npcs) {
		t.Fatalf("npcList has %d, want only the bodies", len(list))
	}
	p, _ := h.join(Arrival{Name: "Lan", Character: "char-Lan", Arch: "newcomer", Caps: withPhone})
	h.drain(p)
	h.call(p, "ma", 3)
	fs, _ := h.drain(p)
	in := only(t, fs, protocol.TypeInteraction)
	if in.Call != "ma" || in.N == 0 {
		t.Fatalf("interaction %+v", in)
	}
	// Má's first answer sends the signal that completes "Gọi về cho má"
	h.choose(p, in.Sid, 0, 4)
	h.drain(p)
	var sent []events.Event
	for _, ev := range *published {
		if ev.Kind == events.Signal {
			sent = append(sent, ev)
		}
	}
	if len(sent) != 1 || sent[0].NPC != "ma" || sent[0].Signal != "ma_gui_tien" || sent[0].Character != "char-Lan" {
		t.Fatalf("signals %+v", sent)
	}
	// a voice cannot be walked up to
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeInteract, N: in.N, Rid: 9})
	if ok, code := resultOf(t, mustDrain(h, p)); ok || code != protocol.ResTarget {
		t.Fatalf("interacting with a voice: ok=%v code=%q", ok, code)
	}
}

func TestOnlyOneConversationAtATimeWithCalls(t *testing.T) {
	h, _ := callHarness(t, numbers{"char-Lan": {"chu_sau", "co_ba"}})
	sau := h.npc("chu_sau")
	p, m := h.join(Arrival{Name: "Lan", Character: "char-Lan", Caps: withPhone, Start: &protocol.Pose{X: sau.pose.X + 1, Z: sau.pose.Z}})
	h.drain(p)
	h.call(p, "co_ba", 1)
	fs, _ := h.drain(p)
	first := only(t, fs, protocol.TypeInteraction)
	// a second call ends the first
	h.call(p, "chu_sau", 2)
	fs, _ = h.drain(p)
	if end := only(t, fs, protocol.TypeInteractionEnd); end.Sid != first.Sid || end.Reason != protocol.EndOther {
		t.Fatalf("end %+v", end)
	}
	second := only(t, fs, protocol.TypeInteraction)
	// so does walking up to someone and talking
	h.interact(p, sau, 3)
	fs, _ = h.drain(p)
	if end := only(t, fs, protocol.TypeInteractionEnd); end.Sid != second.Sid || end.Reason != protocol.EndOther {
		t.Fatalf("end %+v", end)
	}
	if in := only(t, fs, protocol.TypeInteraction); in.Call != "" || in.N != sau.id {
		t.Fatalf("the face to face talk is %+v", in)
	}
	if len(h.w.sessions) != 1 || m.talk == nil || m.talk.call {
		t.Fatalf("sessions %d, talk %+v", len(h.w.sessions), m.talk)
	}
}

func TestACallToAnNPCSomeoneElseTalksTo(t *testing.T) {
	// the NPC is shared: a caller and someone standing next to it each have a conversation of their own
	h, _ := callHarness(t, numbers{"char-Lan": {"chu_tu"}})
	tu := h.npc("chu_tu")
	pa, _ := h.joinNear("Minh", "citizen", tu, 1.5, 0)
	h.interact(pa, tu, 1)
	h.drain(pa)
	pb, _ := h.join(Arrival{Name: "Lan", Character: "char-Lan", Caps: withPhone})
	h.drain(pb)
	h.call(pb, "chu_tu", 2)
	fs, _ := h.drain(pb)
	if ok, code := resultOf(t, fs); !ok {
		t.Fatalf("refused: %s", code)
	}
	if len(h.w.sessions) != 2 {
		t.Fatalf("sessions %d, want 2", len(h.w.sessions))
	}
}
