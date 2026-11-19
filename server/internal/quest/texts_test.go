package quest

import (
	"slices"
	"sync"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/events"
)

const day1 = int64(1_700_000_000_000)

func textKeys(out Outcome) []string {
	var keys []string
	for _, m := range out.Texts {
		keys = append(keys, m.Key+"<"+m.From)
	}
	return keys
}

// A quest's texts go out when it starts and when it is done, once each.
func TestAQuestSendsItsTextsWhenItStartsAndWhenItIsDone(t *testing.T) {
	e := engine(t)
	b := NewBook(Saved{})
	who := Player{Tags: []string{"citizen", "native"}}
	out := e.Refresh(who, b, day1)
	if got := textKeys(out); !slices.Equal(got, []string{"citizen.ca_phe_sang//start/0<chu_tu"}) {
		t.Fatalf("on start: %v", got)
	}
	if out.Texts[0].Say.VI == "" || out.Texts[0].Say.EN == "" {
		t.Fatalf("the text has no both languages: %+v", out.Texts[0])
	}
	// nothing is sent again by the next refresh
	if out = e.Refresh(who, b, day1+1); len(out.Texts) != 0 {
		t.Fatalf("a refresh sent %v again", textKeys(out))
	}
	out = e.Apply(who, b, events.Event{Kind: events.Signal, Character: "c1", NPC: "chu_tu", Signal: "order_ca_phe_sua_da"}, day1+2)
	if len(out.Texts) != 0 {
		t.Fatalf("half a quest sent %v", textKeys(out))
	}
	out = e.Apply(who, b, events.Event{Kind: events.PlaceTime, Character: "c1", Place: "day_nha_pho_le_lai", Seconds: 25}, day1+3)
	if got := textKeys(out); !slices.Equal(got, []string{"citizen.ca_phe_sang//done/0<chu_tu"}) {
		t.Fatalf("on done: %v (completed %v)", got, out.Completed)
	}
}

// The newcomer's call home: it opens with a phone alone (not with the phone quest done), Má's call pays 100.000đ,
// she texts, and the quest to meet Chú Sáu follows.
func TestTheCallHomeChain(t *testing.T) {
	e := engine(t)
	newcomer := []string{"citizen", "newcomer"}
	b := NewBook(Saved{})

	// no phone: the call waits
	out := e.Refresh(Player{Tags: newcomer}, b, day1)
	if _, started := b.Records["story.goi_ve_cho_ma"]; started {
		t.Fatalf("the call home opened without a phone: %+v", out.Changed)
	}
	// a phone: it opens, though the phone quest was never done (a phone from elsewhere is enough)
	who := Player{Tags: newcomer, Caps: capability.Of(content.CapPhone)}
	e.Refresh(who, b, day1+1)
	if r := b.Records["story.goi_ve_cho_ma"]; r == nil || r.State != StateActive {
		t.Fatalf("with a phone the call home is %+v", r)
	}
	if _, started := b.Records["story.gap_chu_sau"]; started {
		t.Fatal("Chú Sáu's quest opened before the call")
	}

	// Má's call: the answer that sends ma_gui_tien completes it and pays
	out = e.Apply(who, b, events.Event{Kind: events.Signal, Character: "c1", NPC: "ma", Signal: "ma_gui_tien"}, day1+2)
	if !slices.Contains(out.Completed, "story.goi_ve_cho_ma") {
		t.Fatalf("completed %v", out.Completed)
	}
	if len(out.Grants) != 1 || out.Grants[0].Xu != 100_000 {
		t.Fatalf("grants %+v: Má sends 100.000đ", out.Grants)
	}
	if got := textKeys(out); !slices.Equal(got, []string{"story.goi_ve_cho_ma//done/0<ma"}) {
		t.Fatalf("Má's text: %v", got)
	}
	if r := b.Records["story.gap_chu_sau"]; r == nil || r.State != StateActive {
		t.Fatalf("after the call, Chú Sáu's quest is %+v", r)
	}
	// meeting Chú Sáu completes it
	out = e.Apply(who, b, events.Event{Kind: events.NPCTalked, Character: "c1", NPC: "chu_sau"}, day1+3)
	if !slices.Contains(out.Completed, "story.gap_chu_sau") {
		t.Fatalf("completed %v", out.Completed)
	}
	// and a native never sees any of it
	nb := NewBook(Saved{})
	e.Refresh(Player{Tags: []string{"citizen", "native"}, Caps: capability.Of(content.CapPhone)}, nb, day1)
	if nb.Records["story.goi_ve_cho_ma"] != nil || nb.Records["story.gap_chu_sau"] != nil {
		t.Fatal("a native was offered the newcomer's quests")
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for range 200 {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (w *fakeWorld) phoneFrames(character string) []phoneFrame {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.phone[character])
}

// A text reaches a character that has a phone as it arrives; without one it waits, and the quest service says
// when the phone came.
func TestTextsReachOnlyAPhone(t *testing.T) {
	store := newMemStore()
	s, w, _ := startService(t, store)
	var mu sync.Mutex
	var got []string
	s.OnPhone = func(character string) {
		mu.Lock()
		got = append(got, character)
		mu.Unlock()
	}
	native := []string{"citizen", "native"}

	// with a phone: the number and the text come
	s.Join("withphone", 1, Player{Tags: native, Caps: capability.Of(content.CapPhone)})
	waitFor(t, "the text", func() bool { return len(w.phoneFrames("withphone")) == 2 })
	if f := w.phoneFrames("withphone"); f[0].T != "contact" || f[0].Contact != "chu_tu" || f[1].T != "text" || f[1].Text.From != "chu_tu" {
		t.Fatalf("frames %+v", f)
	}

	// without: the number is told, the text waits (it is stored: store.Texts sends it with the phone)
	s.Join("nophone", 1, Player{Tags: native})
	waitFor(t, "the number", func() bool { return len(w.phoneFrames("nophone")) == 1 })
	time.Sleep(50 * time.Millisecond)
	if f := w.phoneFrames("nophone"); len(f) != 1 || f[0].T != "contact" {
		t.Fatalf("a text reached a character with no phone: %+v", f)
	}
	if !store.texts["nophone"+"citizen.ca_phe_sang//start/0"] {
		t.Fatal("the waiting text was not kept")
	}
	s.Capabilities("nophone", capability.Of(content.CapPhone))
	waitFor(t, "the phone hook", func() bool { mu.Lock(); defer mu.Unlock(); return slices.Contains(got, "nophone") })
	// more caps later is not a first phone again
	s.Capabilities("nophone", capability.Of(content.CapPhone, content.CapSmartphone))
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if n := len(got); n != 1 {
		t.Fatalf("OnPhone ran %d times: %v", n, got)
	}
}
