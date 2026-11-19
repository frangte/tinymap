package objective

import (
	"encoding/json"
	"errors"
	"fmt"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
)

// buy_item: buy one item ({"item": id}) or an item of a kind ({"kind": "phone"}): exactly one of the two. With a
// kind, a count above 1 asks for that many distinct items.
type buyItem struct{}

func init() { Register(buyItem{}) }

func (buyItem) Kind() string { return "buy_item" }

func (buyItem) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Item string `json:"item"`
		Kind string `json:"kind"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if (p.Item == "") == (p.Kind == "") {
		return nil, errors.New("params: name exactly one of item or kind")
	}
	if p.Item != "" {
		if _, ok := refs.Item(p.Item); !ok {
			return nil, fmt.Errorf("unknown item %q", p.Item)
		}
		if count > 1 {
			return nil, errOnce
		}
		return buyMatcher{item: p.Item}, nil
	}
	if !content.ItemKindKnown(p.Kind) {
		return nil, fmt.Errorf("unknown item kind %q", p.Kind)
	}
	if n := refs.ItemsOfKind(p.Kind); count > n {
		return nil, fmt.Errorf("count %d: there are only %d items of kind %q", count, n, p.Kind)
	}
	return buyMatcher{kind: p.Kind}, nil
}

type buyMatcher struct{ item, kind string }

func (buyMatcher) Listens() []events.Kind { return []events.Kind{events.ItemBought} }

func (m buyMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	if m.item != "" {
		return ev.Item == m.item && p.Count(1, max)
	}
	return ev.ItemKind == m.kind && p.CountDistinct(ev.Item, max)
}
