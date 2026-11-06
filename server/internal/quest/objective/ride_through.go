package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// ride_through: be in a place while riding ({"place": id}): riding into it, or getting on and riding while there.
type rideThrough struct{}

func init() { Register(rideThrough{}) }

func (rideThrough) Kind() string { return "ride_through" }

func (rideThrough) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Place string `json:"place"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.Place(p.Place); !ok {
		return nil, fmt.Errorf("unknown place %q", p.Place)
	}
	if count > 1 {
		return nil, errOnce
	}
	return rideThroughMatcher{place: p.Place}, nil
}

type rideThroughMatcher struct{ place string }

func (rideThroughMatcher) Listens() []events.Kind {
	return []events.Kind{events.PlaceEntered, events.PlaceTime}
}

func (m rideThroughMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.Riding && ev.Place == m.place && p.Count(1, max)
}
