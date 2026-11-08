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
	"saigon-noir/server/internal/gameday"
	"saigon-noir/server/internal/protocol"
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
	// DayCheck is how often the service looks whether a new day began, to start the dailies of everyone online
	DayCheck time.Duration
}

func DefaultConfig() Config {
	// 16 workers: a character whose save is retried (up to a few seconds) holds up only those sharing its worker
	return Config{Workers: 16, Queue: 1024, StoreWait: 3 * time.Second, DayCheck: 30 * time.Second}
}

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
	// jobNewDay: a day began; every character on the worker gets that day's dailies
	jobNewDay
	// jobRetag: the character changed archetype; the quests of the new one open
	jobRetag
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
	go s.watchDays(ctx)
	for range s.workers {
		<-done
	}
}

// watchDays tells every worker when a new day begins (midnight in Saigon), so players online across midnight get
// the new day's dailies without coming back.
func (s *Service) watchDays(ctx context.Context) {
	every := s.cfg.DayCheck
	if every <= 0 {
		every = DefaultConfig().DayCheck
	}
	t := time.NewTicker(every)
	defer t.Stop()
	day := gameday.Of(s.now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		today := gameday.Of(s.now())
		if today == day {
			continue
		}
		day = today
		for _, ch := range s.workers {
			select {
			case ch <- job{kind: jobNewDay}:
			case <-ctx.Done():
				return
			}
		}
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

// Retag tells the service a playing character changed archetype: the quests the new one is offered open, the
// ones already started keep going, and the game hears where the character stands. A visit that ended (another
// session) is left alone.
func (s *Service) Retag(character string, session uint64, who Player) {
	s.enqueue(job{kind: jobRetag, character: character, session: session, player: who}, true)
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
	case jobNewDay:
		for character, p := range players {
			s.apply(ctx, character, p, func(b *Book, now int64) Outcome { return s.engine.Refresh(p.who, b, now) })
		}
	case jobRetag:
		if p := players[j.character]; p != nil && p.session == j.session {
			p.who = j.player
			// the game shows the new archetype's quests and achievements even when nothing opened
			if !s.apply(ctx, j.character, p, func(b *Book, now int64) Outcome { return s.engine.Refresh(p.who, b, now) }) {
				s.send(j.character, p.book, nil, s.now().UnixMilli())
			}
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
		s.send(j.character, p.book, nil, s.now().UnixMilli())
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
	c := Commit{Grants: out.Grants, Title: out.Title, Streak: out.Streak, At: now}
	for _, r := range out.Changed {
		c.Records = append(c.Records, *r)
	}
	stored, err := s.save(ctx, character, c)
	if err != nil {
		s.log.Error("saving quests failed, progress not kept", "character", character, "err", err)
		return false
	}
	before := p.book
	p.book = b
	// the balance first, so the game already shows it when the progress celebrates the reward
	if stored.Paid {
		s.world.Deliver(character, protocol.Encode(protocol.Wallet{T: protocol.TypeWallet, Balance: stored.Balance}))
	}
	s.send(character, b, s.news(out, stored.Granted), now)
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

func (s *Service) save(ctx context.Context, character string, c Commit) (Stored, error) {
	var err error
	for try := 0; try < saveTries; try++ {
		if try > 0 {
			select {
			case <-ctx.Done():
				return Stored{}, err
			case <-time.After(saveBackoff[min(try-1, len(saveBackoff)-1)]):
			}
		}
		sctx, cancel := context.WithTimeout(ctx, s.cfg.StoreWait)
		var stored Stored
		stored, err = s.store.SaveQuests(sctx, character, c)
		cancel()
		if err == nil {
			return stored, nil
		}
		s.log.Warn("saving quests failed", "character", character, "try", try+1, "err", err)
	}
	return Stored{}, err
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

func (s *Service) send(character string, b *Book, news []News, now int64) {
	s.world.Deliver(character, s.engine.progress(b, news, gameday.OfMilli(now)))
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
