package objective

import (
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/events"
)

// dialogue_signal: pick an answer that sends a signal, talking to an NPC ({"npc": id, "signal": id}). The NPC's
// dialogue must have such an answer.
type dialogueSignal struct{}

func init() { Register(dialogueSignal{}) }

func (dialogueSignal) Kind() string { return "dialogue_signal" }

func (dialogueSignal) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		NPC    string `json:"npc"`
		Signal string `json:"signal"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if !refs.Emits(p.NPC, p.Signal) {
		return nil, fmt.Errorf("npc %q has no answer that sends %q", p.NPC, p.Signal)
	}
	if count > 1 {
		return nil, errOnce
	}
	return signalMatcher{npc: p.NPC, signal: p.Signal}, nil
}

type signalMatcher struct{ npc, signal string }

func (signalMatcher) Listens() []events.Kind { return []events.Kind{events.Signal} }

func (m signalMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.NPC == m.npc && ev.Signal == m.signal && p.Count(1, max)
}
