package objective

import (
	"encoding/json"

	"saigon-noir/server/internal/events"
)

// give_way: answer count distinct players' asks for the way ({}).
type giveWay struct{}

func init() { Register(giveWay{}) }

func (giveWay) Kind() string { return "give_way" }

func (giveWay) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct{}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	return giveWayMatcher{}, nil
}

type giveWayMatcher struct{}

func (giveWayMatcher) Listens() []events.Kind { return []events.Kind{events.DirectionsGiven} }

func (giveWayMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	if len(ev.Partners) == 0 {
		return false
	}
	return p.CountDistinct(ev.Partners[0], max)
}
