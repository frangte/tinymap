// Package phone keeps what a character's phone holds: the numbers it has (contacts, docs/design/12-phone.md) and
// the text messages it got. A number comes from talking to an NPC face to face (the npc.talked event), from the
// archetype it started as (contacts.json start_for) or from a text; texts are written by the quest service in the
// same transaction as the quest that sends them (store.SaveQuests). This service reads them back to the game when
// it joins and when it gets a phone, marks them read, and adds the numbers. Calls are conversations, which the
// world runs (package world); the world asks the store whether the character has the number.
package phone

import (
	"context"
	"log/slog"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// Store keeps the phone's data (store.DB in production).
type Store interface {
	// AddContacts keeps numbers; it returns the ones that were new
	AddContacts(ctx context.Context, character string, ids []string, at int64) ([]string, error)
	Contacts(ctx context.Context, character string) ([]string, error)
	Texts(ctx context.Context, character string) ([]protocol.PhoneText, error)
	ReadTexts(ctx context.Context, character string, at int64) error
}

// World is how the service reaches a player: world.World in production.
type World interface {
	Deliver(character string, frame []byte) bool
}

// Config tunes the service.
type Config struct {
	// Queue is the backlog of work; an event that finds it full is dropped
	Queue int
	// StoreWait bounds each store call
	StoreWait time.Duration
}

func DefaultConfig() Config { return Config{Queue: 1024, StoreWait: 3 * time.Second} }

// Service runs the phone's contacts and texts for the characters in the world.
type Service struct {
	catalog *content.Catalog
	store   Store
	world   World
	cfg     Config
	log     *slog.Logger
	now     func() time.Time
	jobs    chan job
	// stopped is closed when Run returns: nobody takes jobs any more
	stopped chan struct{}
}

type jobKind int

const (
	// jobArrive: a character joined (or changed archetype): it gets its starting numbers, and hears of all
	jobArrive jobKind = iota
	// jobSync: a character just got its first phone: the texts that waited are due
	jobSync
	// jobTalked: a character talked to an NPC face to face
	jobTalked
	// jobRead: a character opened its messages
	jobRead
)

type job struct {
	kind      jobKind
	character string
	tags      []string
	// phone: the character has a phone, so its texts are sent too
	phone bool
	npc   string
}

func New(c *content.Catalog, store Store, world World, cfg Config, log *slog.Logger) *Service {
	return &Service{catalog: c, store: store, world: world, cfg: cfg, log: log, now: time.Now,
		jobs: make(chan job, max(1, cfg.Queue)), stopped: make(chan struct{})}
}

// Run works until ctx ends.
func (s *Service) Run(ctx context.Context) {
	defer close(s.stopped)
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-s.jobs:
			s.do(ctx, j)
		}
	}
}

// Arrive tells the service a character joined the world, or changed archetype (tags are its archetype's): it
// gets the numbers it starts with, then hears of every number it has and, when it has a phone, every text. Call
// it after the world accepted the join, so what the service sends arrives after the welcome.
func (s *Service) Arrive(character string, tags []string, hasPhone bool) {
	s.enqueue(job{kind: jobArrive, character: character, tags: tags, phone: hasPhone}, true)
}

// Sync sends a character that just got a phone the texts that waited for one.
func (s *Service) Sync(character string) { s.enqueue(job{kind: jobSync, character: character}, false) }

// Read marks all of a character's texts as read.
func (s *Service) Read(character string) { s.enqueue(job{kind: jobRead, character: character}, false) }

// Submit hands over an event; it never blocks (the world loop calls it). Only a face to face talk with an NPC
// that has a number counts, and it gives the number once.
func (s *Service) Submit(ev events.Event) {
	if ev.Kind != events.NPCTalked || ev.Character == "" {
		return
	}
	if _, ok := s.catalog.Contact(ev.NPC); !ok {
		return
	}
	s.enqueue(job{kind: jobTalked, character: ev.Character, npc: ev.NPC}, false)
}

func (s *Service) enqueue(j job, wait bool) {
	if wait {
		select {
		case s.jobs <- j:
		case <-s.stopped:
		}
		return
	}
	select {
	case s.jobs <- j:
	default:
		s.log.Warn("phone queue full, work dropped", "character", j.character)
	}
}

// do runs one job. A panic costs that job, not the server.
func (s *Service) do(ctx context.Context, j job) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("phone job failed", "character", j.character, "panic", r)
		}
	}()
	switch j.kind {
	case jobArrive:
		s.arrive(ctx, j)
	case jobSync:
		s.send(ctx, j.character, true)
	case jobTalked:
		s.talked(ctx, j)
	case jobRead:
		s.withStore(ctx, "marking texts read", j.character, func(c context.Context) error {
			return s.store.ReadTexts(c, j.character, s.now().UnixMilli())
		})
	}
}

func (s *Service) arrive(ctx context.Context, j job) {
	if start := s.catalog.StartContacts(j.tags); len(start) > 0 {
		if !s.withStore(ctx, "adding starting contacts", j.character, func(c context.Context) error {
			_, err := s.store.AddContacts(c, j.character, start, s.now().UnixMilli())
			return err
		}) {
			return
		}
	}
	// without a phone the texts wait: the quest service has them sent (Sync) once the character has one
	s.send(ctx, j.character, j.phone)
}

func (s *Service) talked(ctx context.Context, j job) {
	var added []string
	ok := s.withStore(ctx, "adding a contact", j.character, func(c context.Context) error {
		var err error
		added, err = s.store.AddContacts(c, j.character, []string{j.npc}, s.now().UnixMilli())
		return err
	})
	if !ok {
		return
	}
	for _, id := range added {
		s.world.Deliver(j.character, protocol.Encode(protocol.Contact{T: protocol.TypeContact, Contact: id}))
	}
}

// send tells a character every number it has and, when withTexts, every text it got. The game replaces what it
// had with this.
func (s *Service) send(ctx context.Context, character string, withTexts bool) {
	msg := protocol.Phone{T: protocol.TypePhone, Contacts: []string{}, Texts: []protocol.PhoneText{}}
	ok := s.withStore(ctx, "loading the phone", character, func(c context.Context) error {
		var err error
		if msg.Contacts, err = s.store.Contacts(c, character); err != nil {
			return err
		}
		if withTexts {
			msg.Texts, err = s.store.Texts(c, character)
		}
		return err
	})
	if ok {
		s.world.Deliver(character, protocol.Encode(msg))
	}
}

// saveTries and saveBackoff: a store call that fails (the database busy) is tried again a little later.
const saveTries = 3

var saveBackoff = []time.Duration{250 * time.Millisecond, time.Second}

// withStore runs a store call, retrying a failure; false when it never worked (logged).
func (s *Service) withStore(ctx context.Context, what, character string, call func(context.Context) error) bool {
	var err error
	for try := range saveTries {
		if try > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(saveBackoff[min(try-1, len(saveBackoff)-1)]):
			}
		}
		sctx, cancel := context.WithTimeout(ctx, s.cfg.StoreWait)
		err = call(sctx)
		cancel()
		if err == nil {
			return true
		}
		s.log.Warn("phone store call failed", "what", what, "character", character, "try", try+1, "err", err)
	}
	s.log.Error("phone store call gave up", "what", what, "character", character, "err", err)
	return false
}
