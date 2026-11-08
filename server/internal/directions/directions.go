// Package directions is asking the way, the co-op quest (docs/design/README.md, D18): a Traveler asks the way to
// a landmark; every Citizen online gets a call and keeps the Traveler in view; the first to walk up and answer
// gives the way, and both get credit (events the quests count). Nobody answering in time, or nobody online to
// ask, is published too: the Traveler's quest then takes an NPC's directions instead. A world module: everything
// here runs on the world goroutine, and nothing touches the database.
package directions

import (
	"log/slog"
	"math"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/gameday"
	"saigon-noir/server/internal/ids"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// sweepEvery is how often open asks call Citizens who came online after them.
const sweepEvery = time.Second

// Service runs the asks.
type Service struct {
	h         world.Host
	cfg       content.Directions
	log       *slog.Logger
	landmarks map[string]bool
	// asks by asker (one each), and by id
	asks map[uint32]*ask
	byID map[string]*ask
	// asked is when each character last asked (the gap between asks)
	asked map[string]time.Time
	// helped are the pairs (helper, asker characters) credited today; helpedDay is that day
	helped    map[[2]string]bool
	helpedDay string
	sweptAt   time.Time
}

type ask struct {
	id        string
	asker     uint32
	character string
	name      string
	dest      string
	until     time.Time
	// called are the Citizens called, each pinned to the asker while the call stands
	called map[uint32]bool
}

// New makes the module; pass it to world.Use with a closure over the catalog.
func New(h world.Host, c *content.Catalog, log *slog.Logger) *Service {
	s := &Service{h: h, cfg: c.Directions, log: log, landmarks: map[string]bool{}, asks: map[uint32]*ask{}, byID: map[string]*ask{},
		asked: map[string]time.Time{}, helped: map[[2]string]bool{}}
	for _, p := range c.Places {
		if p.Kind == content.Landmark {
			s.landmarks[p.ID] = true
		}
	}
	return s
}

// Handle takes the asking messages.
func (s *Service) Handle(m world.Member, msg protocol.ClientMsg) bool {
	switch msg.T {
	case protocol.TypeWayAsk:
		s.onAsk(m, msg.Dest)
	case protocol.TypeWayCancel:
		if a := s.asks[m.ID()]; a != nil {
			s.close(a, StateCancelled, "cancelled")
		}
	case protocol.TypeWayAnswer:
		s.onAnswer(m, msg.Ask)
	default:
		return false
	}
	return true
}

func (s *Service) fail(id uint32, code string) {
	s.h.Send(id, encode(errorMsg{T: typeError, Code: code}))
}

func (s *Service) onAsk(m world.Member, dest string) {
	now := s.h.Now()
	switch {
	case m.Character() == "":
		s.fail(m.ID(), ErrNoAccount)
		return
	case !m.Caps().Has(content.CapAskWay):
		s.fail(m.ID(), ErrNotAllowed)
		return
	case s.asks[m.ID()] != nil:
		s.fail(m.ID(), ErrBusy)
		return
	case !s.landmarks[dest]:
		s.fail(m.ID(), ErrUnknownPlace)
		return
	case now.Sub(s.asked[m.Character()]) < time.Duration(s.cfg.AskGapSeconds)*time.Second:
		s.fail(m.ID(), ErrTooSoon)
		return
	}
	s.asked[m.Character()] = now
	a := &ask{id: ids.New(), asker: m.ID(), character: m.Character(), name: m.Name(), dest: dest,
		until: now.Add(time.Duration(s.cfg.WaitSeconds) * time.Second), called: map[uint32]bool{}}
	s.callAll(a)
	if len(a.called) == 0 {
		// nobody to ask: the fallback opens at once rather than after a wait for no one
		s.h.Send(a.asker, encode(wayMsg{T: typeWay, State: StateUnanswered, Dest: dest, Why: "nobody"}))
		s.h.Publish(events.Event{Kind: events.DirectionsUnanswered, Character: a.character, Place: dest})
		return
	}
	s.asks[a.asker] = a
	s.byID[a.id] = a
	s.h.Send(a.asker, encode(wayMsg{T: typeWay, ID: a.id, State: StateAsking, Dest: dest, Until: a.until.UnixMilli()}))
}

// callAll calls every Citizen online the ask has not called yet.
func (s *Service) callAll(a *ask) {
	s.h.Each(func(o world.Member) {
		if o.ID() == a.asker || a.called[o.ID()] || o.Character() == "" || !o.Caps().Has(content.CapGiveWay) {
			return
		}
		a.called[o.ID()] = true
		s.h.Pin(a.asker, o.ID(), true)
		s.h.Send(o.ID(), encode(callMsg{T: typeCall, ID: a.id, From: Person{ID: a.asker, Name: a.name}, Dest: a.dest,
			Until: a.until.UnixMilli()}))
	})
}

func (s *Service) onAnswer(m world.Member, id string) {
	a := s.byID[id]
	if a == nil {
		s.fail(m.ID(), ErrGone)
		return
	}
	asker, ok := s.h.Member(a.asker)
	switch {
	case !ok:
		s.fail(m.ID(), ErrGone)
		return
	case m.Character() == "" || !m.Caps().Has(content.CapGiveWay) || m.ID() == a.asker:
		s.fail(m.ID(), ErrNotAllowed)
		return
	case ground(m.Pose(), asker.Pose()) > s.cfg.Reach:
		s.fail(m.ID(), ErrTooFar)
		return
	}
	s.newDay()
	pair := [2]string{m.Character(), a.character}
	if s.helped[pair] {
		s.fail(m.ID(), ErrAlreadyHelped)
		return
	}
	s.helped[pair] = true
	s.h.Publish(events.Event{Kind: events.DirectionsAnswered, Character: a.character, Place: a.dest, Partners: []string{m.Character()}})
	s.h.Publish(events.Event{Kind: events.DirectionsGiven, Character: m.Character(), Place: a.dest, Partners: []string{a.character}})
	s.h.Send(m.ID(), encode(doneMsg{T: typeDone, ID: a.id, To: Person{ID: a.asker, Name: a.name}, Dest: a.dest}))
	delete(a.called, m.ID())
	s.h.Pin(a.asker, m.ID(), false)
	s.h.Send(a.asker, encode(wayMsg{T: typeWay, ID: a.id, State: StateAnswered, Dest: a.dest, Helper: &Person{ID: m.ID(), Name: m.Name()}}))
	s.close(a, "", "taken")
}

// close ends an ask: its calls are withdrawn (why) and its pins dropped; state, when set, is told to the asker.
func (s *Service) close(a *ask, state, why string) {
	for id := range a.called {
		s.h.Send(id, encode(goneMsg{T: typeGone, ID: a.id, Why: why}))
		s.h.Pin(a.asker, id, false)
	}
	delete(s.asks, a.asker)
	delete(s.byID, a.id)
	if state != "" {
		s.h.Send(a.asker, encode(wayMsg{T: typeWay, ID: a.id, State: state, Dest: a.dest}))
	}
}

// newDay forgets yesterday's pairs.
func (s *Service) newDay() {
	if day := gameday.Of(s.h.Now()); day != s.helpedDay {
		s.helpedDay = day
		clear(s.helped)
	}
}

// Tick runs out the asks nobody answered, and calls Citizens who came online since an ask began.
func (s *Service) Tick(now time.Time) {
	for _, a := range s.asks {
		if now.Before(a.until) {
			continue
		}
		s.close(a, "", "expired")
		s.h.Send(a.asker, encode(wayMsg{T: typeWay, ID: a.id, State: StateUnanswered, Dest: a.dest, Why: "no_answer"}))
		s.h.Publish(events.Event{Kind: events.DirectionsUnanswered, Character: a.character, Place: a.dest})
	}
	if now.Sub(s.sweptAt) < sweepEvery {
		return
	}
	s.sweptAt = now
	for _, a := range s.asks {
		s.callAll(a)
	}
	// characters who have not asked for longer than the gap need no entry
	for c, at := range s.asked {
		if now.Sub(at) > time.Duration(s.cfg.AskGapSeconds)*time.Second {
			delete(s.asked, c)
		}
	}
}

var _ world.ArchHook = (*Service)(nil)

// ArchBusy: no archetype change while asking the way (the ask stands until it is answered, cancelled or runs out).
func (s *Service) ArchBusy(m world.Member) bool { return s.asks[m.ID()] != nil }

// ArchChanged: a Citizen who is no longer one is not called to answer asks any more (a new Citizen is called by
// the next sweep).
func (s *Service) ArchChanged(m world.Member) {
	if m.Caps().Has(content.CapGiveWay) {
		return
	}
	for _, a := range s.asks {
		if !a.called[m.ID()] {
			continue
		}
		delete(a.called, m.ID())
		s.h.Pin(a.asker, m.ID(), false)
		s.h.Send(m.ID(), encode(goneMsg{T: typeGone, ID: a.id, Why: "changed"}))
	}
}

// Left: an asker who leaves takes their ask along; a Citizen who leaves is no longer called (the world drops
// their pins).
func (s *Service) Left(m world.Member) {
	if a := s.asks[m.ID()]; a != nil {
		s.close(a, "", "left")
	}
	for _, a := range s.asks {
		delete(a.called, m.ID())
	}
}

// ground is the distance between two poses on the ground (m).
func ground(a, b protocol.Pose) float64 { return math.Hypot(a.X-b.X, a.Z-b.Z) }
