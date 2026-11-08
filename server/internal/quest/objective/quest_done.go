package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// quest_done: complete a quest ({"quest": id}); for achievements that crown a questline.
type questDone struct{}

func init() { Register(questDone{}) }

func (questDone) Kind() string { return "quest_done" }

func (questDone) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Quest string `json:"quest"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.Quest(p.Quest); !ok {
		return nil, fmt.Errorf("unknown quest %q", p.Quest)
	}
	if count > 1 {
		return nil, errOnce
	}
	return questDoneMatcher{quest: p.Quest}, nil
}

type questDoneMatcher struct{ quest string }

func (questDoneMatcher) Listens() []events.Kind { return []events.Kind{events.QuestDone} }

func (m questDoneMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.Quest == m.quest && p.Count(1, max)
}
