package objective

import (
	"encoding/json"
	"fmt"
	"slices"

	"saigon-noir/server/internal/events"
)

// ask_way: get the way from a Citizen player ({"npc": id, "signal": s}). When nobody answers an ask in time (or
// nobody could), the fallback opens and the NPC's signal counts instead; it stays open from then on.
type askWay struct{}

func init() { Register(askWay{}) }

func (askWay) Kind() string { return "ask_way" }

func (askWay) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		NPC    string `json:"npc"`
		Signal string `json:"signal"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.NPCByID(p.NPC); !ok {
		return nil, fmt.Errorf("unknown npc %q", p.NPC)
	}
	if !refs.Emits(p.NPC, p.Signal) {
		return nil, fmt.Errorf("no answer of %s's dialogue sends %q", p.NPC, p.Signal)
	}
	if count > 1 {
		return nil, errOnce
	}
	return askWayMatcher{npc: p.NPC, signal: p.Signal}, nil
}

// fallbackOpen marks, in Seen, that an ask went unanswered.
const fallbackOpen = "fallback"

type askWayMatcher struct{ npc, signal string }

func (askWayMatcher) Listens() []events.Kind {
	return []events.Kind{events.DirectionsAnswered, events.DirectionsUnanswered, events.Signal}
}

func (m askWayMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	switch ev.Kind {
	case events.DirectionsAnswered:
		return p.Count(1, max)
	case events.DirectionsUnanswered:
		if slices.Contains(p.Seen, fallbackOpen) {
			return false
		}
		p.Seen = append(p.Seen, fallbackOpen)
		return true
	case events.Signal:
		return ev.NPC == m.npc && ev.Signal == m.signal && slices.Contains(p.Seen, fallbackOpen) && p.Count(1, max)
	}
	return false
}
