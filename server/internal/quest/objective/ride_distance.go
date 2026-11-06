package objective

import (
	"encoding/json"
	"errors"

	"saigon-noir/server/internal/events"
)

// ride_distance: ride count metres in all ({}); only moves the server accepted count.
type rideDistance struct{}

func init() { Register(rideDistance{}) }

func (rideDistance) Kind() string { return "ride_distance" }

func (rideDistance) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct{}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if count < 100 {
		return nil, errors.New("count is metres, at least 100")
	}
	return rideDistanceMatcher{}, nil
}

type rideDistanceMatcher struct{}

func (rideDistanceMatcher) Listens() []events.Kind { return []events.Kind{events.Rode} }

func (rideDistanceMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return p.Count(ev.Metres, max)
}
