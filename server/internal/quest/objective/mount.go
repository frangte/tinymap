package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// mount: get on a vehicle, any or of a kind ({"vehicle": id}).
type mount struct{}

func init() { Register(mount{}) }

func (mount) Kind() string { return "mount" }

func (mount) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Vehicle string `json:"vehicle"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.Vehicle(p.Vehicle); p.Vehicle != "" && !ok {
		return nil, fmt.Errorf("unknown vehicle %q", p.Vehicle)
	}
	if count > 1 {
		return nil, errOnce
	}
	return mountMatcher{vehicle: p.Vehicle}, nil
}

type mountMatcher struct{ vehicle string }

func (mountMatcher) Listens() []events.Kind { return []events.Kind{events.Mounted} }

func (m mountMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return (m.vehicle == "" || ev.Vehicle == m.vehicle) && p.Count(1, max)
}
