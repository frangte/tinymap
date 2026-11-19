package phone

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// memStore is the phone's store in memory; failures lets a test make the first calls fail.
type memStore struct {
	mu       sync.Mutex
	contacts map[string][]string
	texts    map[string][]protocol.PhoneText
	read     map[string]bool
	failures int
}

func newMemStore() *memStore {
	return &memStore{contacts: map[string][]string{}, texts: map[string][]protocol.PhoneText{}, read: map[string]bool{}}
}

func (m *memStore) fail() error {
	if m.failures > 0 {
		m.failures--
		return errors.New("database is locked")
	}
	return nil
}

func (m *memStore) AddContacts(_ context.Context, character string, ids []string, _ int64) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail(); err != nil {
		return nil, err
	}
	var added []string
	for _, id := range ids {
		if !slices.Contains(m.contacts[character], id) {
			m.contacts[character] = append(m.contacts[character], id)
			added = append(added, id)
		}
	}
	return added, nil
}

func (m *memStore) Contacts(_ context.Context, character string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail(); err != nil {
		return nil, err
	}
	return slices.Clone(m.contacts[character]), nil
}

func (m *memStore) Texts(_ context.Context, character string) ([]protocol.PhoneText, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.texts[character]), nil
}

func (m *memStore) ReadTexts(_ context.Context, character string, _ int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail(); err != nil {
		return err
	}
	m.read[character] = true
	return nil
}

// world records the frames the service delivers.
type world struct {
	mu     sync.Mutex
	frames map[string][][]byte
}

func (w *world) Deliver(character string, frame []byte) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.frames[character] = append(w.frames[character], frame)
	return true
}

func (w *world) got(character string) []map[string]json.RawMessage {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []map[string]json.RawMessage
	for _, f := range w.frames[character] {
		var m map[string]json.RawMessage
		_ = json.Unmarshal(f, &m)
		out = append(out, m)
	}
	return out
}

func start(t *testing.T) (*Service, *memStore, *world) {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	store, w := newMemStore(), &world{frames: map[string][][]byte{}}
	s := New(c, store, w, Config{Queue: 16, StoreWait: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Run(ctx)
	return s, store, w
}

func waitFrames(t *testing.T, w *world, character string, n int) []map[string]json.RawMessage {
	t.Helper()
	for range 300 {
		if f := w.got(character); len(f) >= n {
			return f
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s got %d frames, want %d", character, len(w.got(character)), n)
	return nil
}

func TestANewcomerStartsWithMasNumber(t *testing.T) {
	s, store, w := start(t)
	store.texts["c1"] = []protocol.PhoneText{{ID: "k", From: "ma", Say: protocol.Text{VI: "a", EN: "b"}}}
	s.Arrive("c1", []string{"citizen", "newcomer"}, true)
	f := waitFrames(t, w, "c1", 1)
	var p protocol.Phone
	b, _ := json.Marshal(f[0])
	_ = json.Unmarshal(b, &p)
	if p.T != protocol.TypePhone || !slices.Equal(p.Contacts, []string{"ma"}) || len(p.Texts) != 1 {
		t.Fatalf("phone %+v", p)
	}
	// a native has no one's number to begin with, and a character with no phone is sent no texts
	s.Arrive("c2", []string{"citizen", "native"}, false)
	store.texts["c3"] = []protocol.PhoneText{{ID: "k"}}
	s.Arrive("c3", []string{"native"}, false)
	f = waitFrames(t, w, "c3", 1)
	b, _ = json.Marshal(f[0])
	p = protocol.Phone{}
	_ = json.Unmarshal(b, &p)
	if len(p.Texts) != 0 || len(p.Contacts) != 0 {
		t.Fatalf("a phone-less native got %+v", p)
	}
}

func TestTalkingToAnNPCGivesItsNumberOnce(t *testing.T) {
	s, store, w := start(t)
	talk := func(npc string) {
		s.Submit(events.Event{Kind: events.NPCTalked, Character: "c1", NPC: npc})
	}
	talk("chu_sau")
	waitFrames(t, w, "c1", 1)
	talk("chu_sau") // again: nothing new
	talk("nobody")  // not a contact
	// a signal or a guest is no talk either
	s.Submit(events.Event{Kind: events.Signal, Character: "c1", NPC: "co_ba"})
	s.Submit(events.Event{Kind: events.NPCTalked, NPC: "co_ba"})
	talk("co_ba")
	f := waitFrames(t, w, "c1", 2)
	time.Sleep(50 * time.Millisecond)
	f = w.got("c1")
	if len(f) != 2 {
		t.Fatalf("%d frames: %v", len(f), f)
	}
	var got []string
	for _, m := range f {
		var c string
		_ = json.Unmarshal(m["contact"], &c)
		got = append(got, c)
	}
	if !slices.Equal(got, []string{"chu_sau", "co_ba"}) || !slices.Equal(store.contacts["c1"], []string{"chu_sau", "co_ba"}) {
		t.Fatalf("told %v, kept %v", got, store.contacts["c1"])
	}
}

func TestAStoreThatFailsIsTriedAgain(t *testing.T) {
	saveBackoffSaved := saveBackoff
	saveBackoff = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { saveBackoff = saveBackoffSaved }()
	s, store, w := start(t)
	store.failures = 2
	s.Submit(events.Event{Kind: events.NPCTalked, Character: "c1", NPC: "chu_tu"})
	waitFrames(t, w, "c1", 1)
	if !slices.Equal(store.contacts["c1"], []string{"chu_tu"}) {
		t.Fatalf("kept %v", store.contacts["c1"])
	}
}

func TestReadingMarksTheTexts(t *testing.T) {
	s, store, _ := start(t)
	s.Read("c1")
	for range 100 {
		store.mu.Lock()
		done := store.read["c1"]
		store.mu.Unlock()
		if done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the texts were not marked read")
}
