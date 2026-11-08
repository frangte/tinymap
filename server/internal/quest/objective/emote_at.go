package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// emote_at: play an emote, anywhere or in a place ({"emote": id, "place": id}).
type emoteAt struct{}

func init() { Register(emoteAt{}) }

func (emoteAt) Kind() string { return "emote_at" }

func (emoteAt) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Emote string `json:"emote"`
		Place string `json:"place"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.Emote(p.Emote); !ok {
		return nil, fmt.Errorf("unknown emote %q", p.Emote)
	}
	if _, ok := refs.Place(p.Place); p.Place != "" && !ok {
		return nil, fmt.Errorf("unknown place %q", p.Place)
	}
	if count > 1 {
		return nil, errOnce
	}
	return emoteMatcher{emote: p.Emote, place: p.Place}, nil
}

type emoteMatcher struct{ emote, place string }

func (emoteMatcher) Listens() []events.Kind { return []events.Kind{events.Emote} }

func (m emoteMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.Emote == m.emote && (m.place == "" || ev.Place == m.place) && p.Count(1, max)
}
