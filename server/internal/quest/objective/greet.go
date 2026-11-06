package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// greet: exchange an emote with other players close by, each playing it at the other ({"emote": "wave"}); each
// distinct player counts once, so the same friend cannot be greeted over and over for progress.
type greet struct{}

func init() { Register(greet{}) }

func (greet) Kind() string { return "greet" }

func (greet) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Emote string `json:"emote"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.Emote(p.Emote); !ok {
		return nil, fmt.Errorf("unknown emote %q", p.Emote)
	}
	return greetMatcher{emote: p.Emote}, nil
}

type greetMatcher struct{ emote string }

func (greetMatcher) Listens() []events.Kind { return []events.Kind{events.Greeted} }

func (m greetMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	if ev.Emote != m.emote {
		return false
	}
	changed := false
	for _, who := range ev.Partners {
		changed = p.CountDistinct(who, max) || changed
	}
	return changed
}
