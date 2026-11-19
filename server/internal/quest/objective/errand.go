package objective

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"saigon-noir/server/internal/events"
)

// Limits of an errand's time limit (seconds).
const (
	errandSecondsMin = 30
	errandSecondsMax = 1800
)

// errand: a job taken from an NPC and then done stop by stop, perhaps against the clock (odd jobs,
// docs/design/13-jobs.md).
//
//	{"npc": "chu_tu", "signal": "nhan_viec", "stops": [{"place": "duong_ham_nghi"}, ...], "seconds": 120, "order": "fixed"}
//
// Picking the answer that sends signal when talking to npc takes the job and starts the clock. Then each stop
// is reached by standing in its place, starting a conversation with its NPC, or picking an answer that sends
// the stop's signal in a conversation with its NPC. order "fixed" (the default) wants the stops one after
// the other, and a stop may come twice (a parcel carried to the stall and back to the truck); "free" takes
// them in any order, each once. Count is the number of stops. With seconds the run fails when the time is
// up: nothing says so at that moment, the next thing the player does for the job (reach a stop, or take it
// again) finds the run over, wipes it (a stop reached too late counts for nothing) and the job waits to be
// taken again. Without seconds there is no limit.
type errand struct{}

func init() { Register(errand{}) }

func (errand) Kind() string { return "errand" }

type stop struct {
	Place  string `json:"place"`
	NPC    string `json:"npc"`
	Signal string `json:"signal"`
}

func (s stop) key() string {
	if s.Place != "" {
		return "place:" + s.Place
	}
	return "npc:" + s.NPC + ":" + s.Signal
}

// Errand orders.
const (
	orderFixed = "fixed"
	orderFree  = "free"
)

func (errand) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		NPC     string `json:"npc"`
		Signal  string `json:"signal"`
		Stops   []stop `json:"stops"`
		Seconds int    `json:"seconds"`
		Order   string `json:"order"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if p.Signal == "" || !refs.Emits(p.NPC, p.Signal) {
		return nil, fmt.Errorf("npc %q has no answer that sends %q: the job is taken there", p.NPC, p.Signal)
	}
	if p.Order == "" {
		p.Order = orderFixed
	}
	switch {
	case p.Order != orderFixed && p.Order != orderFree:
		return nil, fmt.Errorf("order must be %q or %q", orderFixed, orderFree)
	case len(p.Stops) < 2:
		return nil, errors.New("needs at least 2 stops")
	case count != len(p.Stops):
		return nil, fmt.Errorf("count is %d but there are %d stops: count is the number of stops", count, len(p.Stops))
	case p.Seconds != 0 && (p.Seconds < errandSecondsMin || p.Seconds > errandSecondsMax):
		return nil, fmt.Errorf("seconds must be 0 (no limit) or %d to %d", errandSecondsMin, errandSecondsMax)
	}
	seen := map[string]bool{}
	for i, s := range p.Stops {
		if err := checkStop(s, refs); err != nil {
			return nil, fmt.Errorf("stop %d: %w", i, err)
		}
		if p.Order == orderFree && seen[s.key()] {
			return nil, fmt.Errorf("stop %d is a duplicate: in free order each stop counts once", i)
		}
		seen[s.key()] = true
	}
	return errandMatcher{giver: p.NPC, signal: p.Signal, stops: p.Stops, limit: int64(p.Seconds) * 1000, free: p.Order == orderFree}, nil
}

func checkStop(s stop, refs Refs) error {
	switch {
	case (s.Place == "") == (s.NPC == ""):
		return errors.New("name exactly one of place or npc")
	case s.Place != "" && s.Signal != "":
		return errors.New("a place has no signal")
	case s.Place != "":
		if _, ok := refs.Place(s.Place); !ok {
			return fmt.Errorf("unknown place %q", s.Place)
		}
	default:
		if _, ok := refs.NPCByID(s.NPC); !ok {
			return fmt.Errorf("unknown npc %q", s.NPC)
		}
		if s.Signal != "" && !refs.Emits(s.NPC, s.Signal) {
			return fmt.Errorf("npc %q has no answer that sends %q", s.NPC, s.Signal)
		}
	}
	return nil
}

type errandMatcher struct {
	giver, signal string
	stops         []stop
	// limit is the time limit in milliseconds, 0 for none
	limit int64
	free  bool
}

func (errandMatcher) Listens() []events.Kind {
	return []events.Kind{events.Signal, events.PlaceEntered, events.NPCTalked}
}

// over: the run began at p.T and its time is up at now.
func (m errandMatcher) over(p *Progress, now int64) bool { return m.limit > 0 && now-p.T > m.limit }

func (m errandMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	if ev.Kind == events.Signal && ev.NPC == m.giver && ev.Signal == m.signal {
		if p.T != 0 && !m.over(p, ev.At) {
			return false // already on its way: asking again does not buy more time
		}
		*p = Progress{T: milli(ev.At)}
		return true
	}
	if p.T == 0 {
		return false // not taken: the stops mean nothing yet
	}
	i := m.match(ev, p)
	if i < 0 {
		return false
	}
	if m.over(p, ev.At) {
		*p = Progress{}
		return true
	}
	if m.free {
		return p.CountDistinct(m.stops[i].key(), max)
	}
	return p.Count(1, max)
}

// milli keeps a run's start from being 0, which means "not taken" (a test clock may start there).
func milli(at int64) int64 { return max(at, 1) }

// match is the stop the event reaches (the one whose turn it is, or any not yet reached in free order), or -1.
func (m errandMatcher) match(ev events.Event, p *Progress) int {
	if !m.free {
		if p.N < len(m.stops) && m.stops[p.N].reachedBy(ev) {
			return p.N
		}
		return -1
	}
	for i, s := range m.stops {
		if s.reachedBy(ev) && !slices.Contains(p.Seen, s.key()) {
			return i
		}
	}
	return -1
}

func (s stop) reachedBy(ev events.Event) bool {
	switch {
	case s.Place != "":
		return ev.Kind == events.PlaceEntered && ev.Place == s.Place
	case s.Signal != "":
		return ev.Kind == events.Signal && ev.NPC == s.NPC && ev.Signal == s.Signal
	}
	return ev.Kind == events.NPCTalked && ev.NPC == s.NPC
}

// Left implements Runner.
func (m errandMatcher) Left(p *Progress, now int64) (int, bool) {
	if p.T == 0 {
		return 0, false
	}
	if m.limit == 0 {
		return -1, true
	}
	return int(max((m.limit-(now-p.T)+999)/1000, 0)), true
}
