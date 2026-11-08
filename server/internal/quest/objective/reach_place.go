package objective

import (
	"encoding/json"
	"errors"
	"fmt"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
)

// reach_place: stand in a place for a moment ({"place": id}), or in count distinct places of a kind
// ({"kind": "landmark"}).
type reachPlace struct{}

func init() { Register(reachPlace{}) }

func (reachPlace) Kind() string { return "reach_place" }

func (reachPlace) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Place string `json:"place"`
		Kind  string `json:"kind"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	switch {
	case (p.Place == "") == (p.Kind == ""):
		return nil, errors.New("name exactly one of place or kind")
	case p.Place != "":
		if _, ok := refs.Place(p.Place); !ok {
			return nil, fmt.Errorf("unknown place %q", p.Place)
		}
		if count > 1 {
			return nil, errOnce
		}
		return reachMatcher{place: p.Place}, nil
	}
	n := 0
	for _, pl := range refs.Places() {
		if string(pl.Kind) == p.Kind {
			n++
		}
	}
	if n == 0 {
		return nil, fmt.Errorf("no place is a %q", p.Kind)
	}
	if count > n {
		return nil, fmt.Errorf("count %d: there are only %d places of kind %q", count, n, p.Kind)
	}
	return reachMatcher{kind: content.PlaceKind(p.Kind)}, nil
}

type reachMatcher struct {
	place string
	kind  content.PlaceKind
}

func (reachMatcher) Listens() []events.Kind { return []events.Kind{events.PlaceEntered} }

func (m reachMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	if m.place != "" {
		return ev.Place == m.place && p.Count(1, max)
	}
	return ev.PlaceKind == string(m.kind) && p.CountDistinct(ev.Place, max)
}
