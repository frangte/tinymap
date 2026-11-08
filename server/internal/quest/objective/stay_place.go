package objective

import (
	"encoding/json"
	"errors"
	"fmt"

	"saigon-noir/server/internal/events"
)

// stay_place: spend count seconds in a place ({"place": id}); the world counts time there in steps of 5 s, from
// when the objective's turn comes.
type stayPlace struct{}

func init() { Register(stayPlace{}) }

func (stayPlace) Kind() string { return "stay_place" }

func (stayPlace) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Place string `json:"place"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.Place(p.Place); !ok {
		return nil, fmt.Errorf("unknown place %q", p.Place)
	}
	if count < 5 {
		return nil, errors.New("count is seconds, at least 5")
	}
	return stayMatcher{place: p.Place}, nil
}

type stayMatcher struct{ place string }

func (stayMatcher) Listens() []events.Kind { return []events.Kind{events.PlaceTime} }

func (m stayMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.Place == m.place && p.Count(ev.Seconds, max)
}
