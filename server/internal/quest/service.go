package quest

import (
	"context"
	"hash/fnv"
	"log/slog"
	"maps"
	"slices"
	"sync/atomic"
	"time"

	"saigon-noir/server/internal/events"
)

// World is how the service reaches players: world.World in production. Each call may block briefly.
type World interface {
	Deliver(character string, frame []byte) bool
	SetQuests(character string, quests map[string]bool) bool
	SetTitle(character, title string) bool
}

// Config tunes the service.
type Config struct {
	// Workers is how many goroutines run characters (each character always on the same one)
	Workers int
	// Queue is each worker's backlog; an event that finds it full is dropped (and counted)
	Queue int
	// StoreWait bounds each load and save
	StoreWait time.Duration
}

func DefaultConfig() Config { return Config{Workers: 4, Queue: 1024, StoreWait: 3 * time.Second} }

// Service runs quests for the characters in the world.
type Service struct {
	engine *Engine
	store  Store
	world  World
	cfg    Config
	log    *slog.Logger
	// OnCapability hears every capability a character earns (the connection updates what it may do)
	OnCapability func(character, capability string)
	now          func() time.Time
	workers      []chan job
	dropped      atomic.Int64
	// stopped is closed when Run returns: nobody takes jobs any more
	stopped chan struct{}
}

type jobKind int

const (
	jobJoin jobKind = iota
	jobLeave
	jobEvent
	jobTitle
)

type job struct {
	kind      jobKind
	character string
	// session tells one visit of a character from the next: a late Leave from a replaced connection must not
	// drop the visit that replaced it
	session uint64
	player  Player
	ev      events.Event
	title   string
}

// player is a character in the world, on its worker.
type player struct {
	session uint64
	who     Player
	book    *Book
}

func NewService(e *Engine, store Store, world World, cfg Config, log *slog.Logger) *Service {
	s := &Service{engine: e, store: store, world: world, cfg: cfg, log: log, now: time.Now, stopped: make(chan struct{})}
	for range max(1, cfg.Workers) {
		s.workers = append(s.workers, make(chan job, max(1, cfg.Queue)))
	}
	return s
}

// Run works until ctx ends.
func (s *Service) Run(ctx context.Context) {
	defer close(s.stopped)
	done := make(chan struct{})
	for _, ch := range s.workers {
		go func() {
			s.work(ctx, ch)
			done <- struct{}{}
		}()
	}
	for range s.workers {
		<-done
	}
}

// Join loads a character that just joined the world and starts its available quests. Call it after the world
// accepted the join, so what the service sends arrives after the welcome.
func (s *Service) Join(character string, session uint64, who Player) {
	s.enqueue(job{kind: jobJoin, character: character, session: session, player: who}, true)
}

// Leave forgets a character whose visit ended.
func (s *Service) Leave(character string, session uint64) {
	s.enqueue(job{kind: jobLeave, character: character, session: session}, true)
}

// ChooseTitle changes the title a character carries to one it earned ("" for none).
func (s *Service) ChooseTitle(character, title string) {
	s.enqueue(job{kind: jobTitle, character: character, title: title}, false)
}

// Submit hands over an event; it never blocks (the world loop calls it). A full queue drops the event.
func (s *Service) Submit(ev events.Event) {
	if ev.Character == "" {
		return
	}
	s.enqueue(job{kind: jobEvent, character: ev.Character, ev: ev}, false)
}

// Dropped is how many events were dropped for a full queue.
func (s *Service) Dropped() int64 { return s.dropped.Load() }

func (s *Service) enqueue(j job, wait bool) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(j.character))
	ch := s.workers[int(h.Sum32()%uint32(len(s.workers)))]
	if wait {
		// joins and leaves come from connection goroutines, which can wait, but not on a service that stopped
		select {
		case ch <- j:
		case <-s.stopped:
		}
		return
	}
	select {
	case ch <- j:
	default:
		if n := s.dropped.Add(1); n == 1 || n%100 == 0 {
			s.log.Warn("quest queue full, event dropped", "character", j.character, "kind", j.ev.Kind, "dropped", n)
		}
	}
}

func (s *Service) work(ctx context.Context, jobs <-chan job) {
	players := map[string]*player{}
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-jobs:
			s.do(ctx, players, j)
		}
	}
}

// do runs one job. A panic (a bug, a corrupt stored row) costs that character this job, not the server.
func (s *Service) do(ctx context.Context, players map[string]*player, j job) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("quest job failed", "character", j.character, "kind", j.kind, "panic", r)
		}
	}()
	switch j.kind {
	case jobJoin:
		if p := s.join(ctx, j); p != nil {
			players[j.character] = p
		}
	case jobLeave:
		if p := players[j.character]; p != nil && p.session == j.session {
			delete(players, j.character)
		}
	case jobEvent:
		if p := players[j.character]; p != nil {
			s.apply(ctx, j.character, p, func(b *Book, now int64) Outcome { return s.engine.Apply(p.who, b, j.ev, now) })
		}
	case jobTitle:
		if p := players[j.character]; p != nil {
			s.chooseTitle(ctx, j.character, p, j.title)
		}
	}
}

func (s *Service) join(ctx context.Context, j job) *player {
	lctx, cancel := context.WithTimeout(ctx, s.cfg.StoreWait)
	saved, err := s.store.LoadQuests(lctx, j.character)
	cancel()
	if err != nil {
		// the database is down or slow: the visit goes on without quests rather than not at all
		s.log.Error("loading quests failed, playing without them", "character", j.character, "err", err)
		return nil
	}
	p := &player{session: j.session, who: j.player, book: NewBook(saved)}
	// a new visit always tells the game where it stands, even when nothing changed
	if !s.apply(ctx, j.character, p, func(b *Book, now int64) Outcome { return s.engine.Refresh(p.who, b, now) }) {
		s.send(j.character, p.book, nil)
	}
	return p
}

// apply runs a step on a copy of the book, stores what changed, and only then keeps the copy and tells the world;
// a failed save leaves everything as it was. True when something changed and was stored.
func (s *Service) apply(ctx context.Context, character string, p *player, step func(*Book, int64) Outcome) bool {
	now := s.now().UnixMilli()
	b := p.book.clone()
	out := step(b, now)
	if out.Empty() {
		return false
	}
	c := Commit{Grants: out.Grants, Title: out.Title, At: now}
	for _, r := range out.Changed {
		c.Records = append(c.Records, *r)
	}
	granted, err := s.save(ctx, character, c)
	if err != nil {
		s.log.Error("saving quests failed, progress not kept", "character", character, "err", err)
		return false
	}
	before := p.book
	p.book = b
	s.send(character, b, s.news(out, granted))
	if !maps.Equal(before.Active(), b.Active()) {
		s.world.SetQuests(character, b.Active())
	}
	if out.Title != nil {
		s.world.SetTitle(character, *out.Title)
	}
	// every capability earned goes to the connection, even one the store had given before (a save that went
	// through but answered with an error): giving it again changes nothing
	for _, g := range out.Grants {
		if g.Capability != "" && s.OnCapability != nil {
			s.OnCapability(character, g.Capability)
		}
	}
	return true
}

// saveTries and saveBackoff: a save that fails (the database busy) is tried again a little later before the
// event is given up, since some events (a quiz passed) do not happen again by themselves.
const saveTries = 3

var saveBackoff = []time.Duration{250 * time.Millisecond, time.Second}

func (s *Service) save(ctx context.Context, character string, c Commit) ([]string, error) {
	var err error
	for try := 0; try < saveTries; try++ {
		if try > 0 {
			select {
			case <-ctx.Done():
				return nil, err
			case <-time.After(saveBackoff[min(try-1, len(saveBackoff)-1)]):
			}
		}
		sctx, cancel := context.WithTimeout(ctx, s.cfg.StoreWait)
		var granted []string
		granted, err = s.store.SaveQuests(sctx, character, c)
		cancel()
		if err == nil {
			return granted, nil
		}
		s.log.Warn("saving quests failed", "character", character, "try", try+1, "err", err)
	}
	return nil, err
}

func (s *Service) chooseTitle(ctx context.Context, character string, p *player, title string) {
	if title == p.book.Title || (title != "" && !p.book.Titles[title]) {
		return // the same one, or one it has not earned (a tampered client): nothing happens
	}
	s.apply(ctx, character, p, func(b *Book, now int64) Outcome {
		b.Title = title
		return Outcome{Title: &b.Title}
	})
}

func (s *Service) send(character string, b *Book, news []News) {
	s.world.Deliver(character, s.engine.progress(b, news))
	s.world.SetQuests(character, b.Active())
}

// news is what the game celebrates: quests and achievements completed, and the rewards actually given (a grant
// the store had already given is not news).
func (s *Service) news(out Outcome, granted []string) []News {
	var n []News
	for _, id := range out.Completed {
		if isAchievement(id) {
			n = append(n, News{Kind: NewsAchievement, ID: id[len(achievementPrefix):]})
		} else {
			n = append(n, News{Kind: NewsQuest, ID: id})
		}
	}
	for _, g := range out.Grants {
		if !slices.Contains(granted, g.Source) {
			continue
		}
		for _, t := range g.Titles {
			n = append(n, News{Kind: NewsTitle, ID: t})
		}
		for _, st := range g.Stamps {
			n = append(n, News{Kind: NewsStamp, ID: st})
		}
	}
	return n
}
