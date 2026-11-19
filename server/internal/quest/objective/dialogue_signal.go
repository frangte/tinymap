package objective

import (
	"encoding/json"
	"fmt"
	"slices"

	"saigon-noir/server/internal/events"
)

// dialogue_signal: pick an answer that sends a signal, talking to an NPC ({"npc": id, "signal": id}). The NPC's
// dialogue must have such an answer. "also" lists other NPCs whose answer with the same signal counts too (a phrase
// two NPCs teach, such as Cô Ba and Anh Tony teaching "xin chào").
type dialogueSignal struct{}

func init() { Register(dialogueSignal{}) }

func (dialogueSignal) Kind() string { return "dialogue_signal" }

func (dialogueSignal) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		NPC    string   `json:"npc"`
		Also   []string `json:"also"`
		Signal string   `json:"signal"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	npcs := append([]string{p.NPC}, p.Also...)
	for _, n := range npcs {
		if !refs.Emits(n, p.Signal) {
			return nil, fmt.Errorf("npc %q has no answer that sends %q", n, p.Signal)
		}
	}
	if count > 1 {
		return nil, errOnce
	}
	return signalMatcher{npcs: npcs, signal: p.Signal}, nil
}

type signalMatcher struct {
	npcs   []string
	signal string
}

func (signalMatcher) Listens() []events.Kind { return []events.Kind{events.Signal} }

func (m signalMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.Signal == m.signal && slices.Contains(m.npcs, ev.NPC) && p.Count(1, max)
}
