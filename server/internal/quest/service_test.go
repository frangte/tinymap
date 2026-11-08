package quest

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

	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// memStore is a Store in memory that can be told to fail.
type memStore struct {
	mu      sync.Mutex
	saved   map[string]*Saved
	granted map[string]bool
	// balance is each character's wallet; a character without one is not paid
	balance map[string]int64
	fail    bool
}

func newMemStore() *memStore {
	return &memStore{saved: map[string]*Saved{}, granted: map[string]bool{}, balance: map[string]int64{}}
}

func (m *memStore) LoadQuests(_ context.Context, character string) (Saved, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.saved[character]; s != nil {
		return *s, nil
	}
	return Saved{}, nil
}

func (m *memStore) SaveQuests(_ context.Context, character string, c Commit) (Stored, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out Stored
	if m.fail {
		return out, errors.New("disk full")
	}
	s := m.saved[character]
	if s == nil {
		s = &Saved{}
		m.saved[character] = s
	}
	for _, r := range c.Records {
		// a row per quest and cycle, as in the database
		s.Records = slices.DeleteFunc(s.Records, func(o Record) bool { return o.ID == r.ID && o.Cycle == r.Cycle })
		s.Records = append(s.Records, *r.clone())
	}
	for _, g := range c.Grants {
		if m.granted[character+g.Source] {
			continue
		}
		m.granted[character+g.Source] = true
		out.Granted = append(out.Granted, g.Source)
		s.Titles = append(s.Titles, g.Titles...)
		s.Stamps = append(s.Stamps, g.Stamps...)
		if b, ok := m.balance[character]; ok && g.Xu > 0 {
			m.balance[character] = b + g.Xu
			out.Paid, out.Balance = true, b+g.Xu
		}
	}
	if c.Title != nil {
		s.Title = *c.Title
	}
	if c.Streak != nil {
		s.Streak = *c.Streak
	}
	return out, nil
}

// fakeWorld records what the service tells players.
type fakeWorld struct {
	mu     sync.Mutex
	frames map[string][]protocol.Progress
	titles map[string]string
	quests map[string]map[string]bool
	// wallets are the balances sent, in order
	wallets  map[string][]int64
	received chan struct{}
}

func newFakeWorld() *fakeWorld {
	return &fakeWorld{frames: map[string][]protocol.Progress{}, titles: map[string]string{}, quests: map[string]map[string]bool{},
		wallets: map[string][]int64{}, received: make(chan struct{}, 100)}
}

func (w *fakeWorld) Deliver(character string, frame []byte) bool {
	var p protocol.Progress
	if err := json.Unmarshal(frame, &p); err != nil {
		panic(err)
	}
	if p.T == protocol.TypeWallet {
		var wm protocol.Wallet
		_ = json.Unmarshal(frame, &wm)
		w.mu.Lock()
		w.wallets[character] = append(w.wallets[character], wm.Balance)
		w.mu.Unlock()
		return true
	}
	w.mu.Lock()
	w.frames[character] = append(w.frames[character], p)
	w.mu.Unlock()
	w.received <- struct{}{}
	return true
}

func (w *fakeWorld) SetQuests(character string, q map[string]bool) bool {
	w.mu.Lock()
	w.quests[character] = q
	w.mu.Unlock()
	return true
}

func (w *fakeWorld) SetTitle(character, title string) bool {
	w.mu.Lock()
	w.titles[character] = title
	w.mu.Unlock()
	return true
}

func (w *fakeWorld) last(t *testing.T, character string) protocol.Progress {
	t.Helper()
	select {
	case <-w.received:
	case <-time.After(2 * time.Second):
		t.Fatal("no progress delivered")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	f := w.frames[character]
	return f[len(f)-1]
}

// startService runs a service; the returned func lists the capabilities it gave so far.
func startService(t *testing.T, store Store) (*Service, *fakeWorld, func() []string) {
	t.Helper()
	w := newFakeWorld()
	s := NewService(engine(t), store, w, Config{Workers: 2, Queue: 64, StoreWait: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var mu sync.Mutex
	var caps []string
	s.OnCapability = func(character, capability string) {
		mu.Lock()
		caps = append(caps, character+":"+capability)
		mu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Run(ctx)
	return s, w, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(caps)
	}
}

func questState(p protocol.Progress, id string) (protocol.QuestState, bool) {
	for _, q := range p.Quests {
		if q.ID == id {
			return q, true
		}
	}
	return protocol.QuestState{}, false
}

func TestAJoinShowsWhereTheCharacterStands(t *testing.T) {
	store := newMemStore()
	s, w, _ := startService(t, store)
	s.Join("c1", 1, Player{Tags: []string{"traveler"}})
	p := w.last(t, "c1")
	if q, ok := questState(p, "traveler.chao_sai_gon"); !ok || q.Done || q.N["learn"] != 0 {
		t.Fatalf("first visit %+v", p)
	}
	if len(store.saved["c1"].Records) == 0 {
		t.Fatal("the quests started on arrival should be stored")
	}
	// a later visit with nothing new still gets its progress
	s.Leave("c1", 1)
	s.Join("c1", 2, Player{Tags: []string{"traveler"}})
	if p := w.last(t, "c1"); len(p.Quests) == 0 {
		t.Fatalf("second visit %+v", p)
	}
}

func TestEventsMoveQuestsAndTheNewsSaysSo(t *testing.T) {
	s, w, _ := startService(t, newMemStore())
	s.Join("c1", 1, Player{Tags: []string{"traveler"}})
	w.last(t, "c1")
	s.Submit(events.Event{Kind: events.Signal, Character: "c1", NPC: "co_ba", Signal: "greetings_learned"})
	p := w.last(t, "c1")
	if q, _ := questState(p, "traveler.chao_sai_gon"); !q.Done {
		t.Fatalf("after the lesson %+v", p)
	}
	news := map[protocol.News]bool{}
	for _, n := range p.News {
		news[n] = true
	}
	if !news[protocol.News{Kind: NewsQuest, ID: "traveler.chao_sai_gon"}] || !news[protocol.News{Kind: NewsStamp, ID: "chao_sai_gon"}] {
		t.Fatalf("news %+v", p.News)
	}
	if !slices.Equal(p.Stamps, []string{"chao_sai_gon"}) {
		t.Fatalf("stamps %v", p.Stamps)
	}
	w.mu.Lock()
	running := w.quests["c1"]
	w.mu.Unlock()
	if !running["traveler.hoi_duong"] || running["traveler.chao_sai_gon"] {
		t.Fatalf("the world was told %v", running)
	}
}

func TestAFailedSaveKeepsNothing(t *testing.T) {
	store := newMemStore()
	s, w, _ := startService(t, store)
	s.Join("c1", 1, Player{Tags: []string{"traveler"}})
	w.last(t, "c1")
	store.mu.Lock()
	store.fail = true
	store.mu.Unlock()
	s.Submit(events.Event{Kind: events.Signal, Character: "c1", NPC: "co_ba", Signal: "greetings_learned"})
	// once the database is back, the same lesson counts: the failed one left nothing half done
	time.Sleep(50 * time.Millisecond)
	store.mu.Lock()
	store.fail = false
	store.mu.Unlock()
	s.Submit(events.Event{Kind: events.Signal, Character: "c1", NPC: "co_ba", Signal: "greetings_learned"})
	if q, _ := questState(w.last(t, "c1"), "traveler.chao_sai_gon"); !q.Done {
		t.Fatal("the lesson after the failed save should count")
	}
}

func TestALateLeaveKeepsTheNewVisit(t *testing.T) {
	s, w, _ := startService(t, newMemStore())
	s.Join("c1", 1, Player{Tags: []string{"traveler"}})
	w.last(t, "c1")
	s.Join("c1", 2, Player{Tags: []string{"traveler"}}) // a second tab replaced the first
	w.last(t, "c1")
	s.Leave("c1", 1) // the first one's connection closes after
	s.Submit(events.Event{Kind: events.Signal, Character: "c1", NPC: "co_ba", Signal: "greetings_learned"})
	if q, _ := questState(w.last(t, "c1"), "traveler.chao_sai_gon"); !q.Done {
		t.Fatal("the visit that replaced the first one stopped getting progress")
	}
}

func TestOnlyAnEarnedTitleCanBeCarried(t *testing.T) {
	store := newMemStore()
	s, w, _ := startService(t, store)
	s.Join("c1", 1, Player{Tags: []string{"citizen"}})
	w.last(t, "c1")
	s.ChooseTitle("c1", "nguoi_sai_gon_chinh_hieu") // not earned: nothing happens
	s.Submit(events.Event{Kind: events.Emote, Character: "c1", Emote: "exercise", Place: "cong_vien_23_9"})
	if p := w.last(t, "c1"); p.Title != "nguoi_day_som" {
		t.Fatalf("carried %q after earning the first title", p.Title)
	}
	s.ChooseTitle("c1", "")
	if p := w.last(t, "c1"); p.Title != "" {
		t.Fatalf("carried %q after putting it away", p.Title)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.titles["c1"] != "" {
		t.Fatalf("the world shows %q", w.titles["c1"])
	}
}

func TestTheCertificateCapabilityIsGivenOnce(t *testing.T) {
	store := newMemStore()
	// a character who has done everything but the final test
	done := func(id string) Record { return Record{ID: id, Version: 1, State: StateDone} }
	store.saved["c1"] = &Saved{Records: []Record{done("traveler.chao_sai_gon"), done("traveler.ca_phe_sua_da"),
		done("traveler.kien_truc_phap"), done("traveler.banh_trang_tron"), done("traveler.hoi_duong")}}
	s, w, caps := startService(t, store)
	s.Join("c1", 1, Player{Tags: []string{"traveler"}})
	w.last(t, "c1")
	for range 2 {
		s.Submit(events.Event{Kind: events.Signal, Character: "c1", NPC: "co_ba", Signal: "final_quiz_passed"})
	}
	p := w.last(t, "c1")
	time.Sleep(50 * time.Millisecond)
	if p.Title != "nguoi_sai_gon_chinh_hieu" || !slices.Contains(p.Stamps, "chung_nhan") {
		t.Fatalf("after the test %+v", p)
	}
	if got := caps(); !slices.Equal(got, []string{"c1:local.certified"}) {
		t.Fatalf("capabilities given %v", got)
	}
}
