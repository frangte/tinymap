package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// trip: complete Crab Bike trips in a role ({"role": "rider"} for whoever drove, or {"role": "passenger"}); each distinct partner
// counts once, so two friends driving each other around do not farm it.
type trip struct{}

func init() { Register(trip{}) }

func (trip) Kind() string { return "trip" }

func (trip) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Role string `json:"role"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if p.Role != "rider" && p.Role != "passenger" {
		return nil, fmt.Errorf("role must be rider or passenger, not %q", p.Role)
	}
	return tripMatcher{role: p.Role}, nil
}

type tripMatcher struct{ role string }

func (tripMatcher) Listens() []events.Kind { return []events.Kind{events.TripCompleted} }

func (m tripMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	if ev.Role != m.role {
		return false
	}
	changed := false
	for _, who := range ev.Partners {
		changed = p.CountDistinct(who, max) || changed
	}
	return changed
}
