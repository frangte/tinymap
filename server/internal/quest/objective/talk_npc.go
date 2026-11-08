package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// talk_npc: start a conversation with an NPC ({"npc": id}), or with count distinct NPCs ({}).
type talkNPC struct{}

func init() { Register(talkNPC{}) }

func (talkNPC) Kind() string { return "talk_npc" }

func (talkNPC) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		NPC string `json:"npc"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if p.NPC != "" {
		if _, ok := refs.NPCByID(p.NPC); !ok {
			return nil, fmt.Errorf("unknown npc %q", p.NPC)
		}
		if count > 1 {
			return nil, errOnce
		}
		return talkMatcher{npc: p.NPC}, nil
	}
	if n := len(refs.NPCs()); count > n {
		return nil, fmt.Errorf("count %d: there are only %d NPCs", count, len(refs.NPCs()))
	}
	return talkMatcher{}, nil
}

type talkMatcher struct{ npc string }

func (talkMatcher) Listens() []events.Kind { return []events.Kind{events.NPCTalked} }

func (m talkMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	if m.npc != "" {
		return ev.NPC == m.npc && p.Count(1, max)
	}
	return p.CountDistinct(ev.NPC, max)
}
