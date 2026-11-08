// Package objective holds the objective types of the quest engine: the one extension point for new kinds of
// quests (docs/design/03-domain-model.md). A type is one file here that registers itself; quests.json names it
// in an objective's type, and its Compile checks the objective's params against the game data when the engine
// starts, so a typo fails the server's tests, not a player's quest.
package objective

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
)

// Progress is how far a character got with one objective.
type Progress struct {
	N int `json:"n"`
	// Seen are the distinct things already counted (places, NPCs, players), so the same one never counts twice
	Seen []string `json:"seen,omitempty"`
}

func (p *Progress) Clone() *Progress {
	c := *p
	c.Seen = slices.Clone(p.Seen)
	return &c
}

// Count adds n, up to max.
func (p *Progress) Count(n, max int) bool {
	if p.N >= max || n <= 0 {
		return false
	}
	p.N = min(max, p.N+n)
	return true
}

// CountDistinct counts key once, up to max.
func (p *Progress) CountDistinct(key string, max int) bool {
	if p.N >= max || key == "" || slices.Contains(p.Seen, key) {
		return false
	}
	p.Seen = append(p.Seen, key)
	p.N++
	return true
}

// Matcher is a compiled objective: it moves its progress on the events it listens to.
type Matcher interface {
	Listens() []events.Kind
	// Advance applies an event; true when the progress changed. max is the objective's count.
	Advance(ev events.Event, p *Progress, max int) bool
}

// Type is a kind of objective.
type Type interface {
	Kind() string
	// Compile checks the params (quests.json) and returns the matcher; count is the objective's count, which a
	// type may bound (no more distinct places than there are).
	Compile(params json.RawMessage, count int, refs Refs) (Matcher, error)
}

// Refs is the game data an objective's params may name.
type Refs interface {
	Place(id string) (content.Place, bool)
	Places() []content.Place
	NPCByID(id string) (content.NPC, bool)
	NPCs() []content.NPC
	Emote(id string) (content.Emote, bool)
	Vehicle(id string) (content.Vehicle, bool)
	Quest(id string) (content.Quest, bool)
	// Emits: some answer of the NPC's dialogue sends the signal
	Emits(npc, signal string) bool
}

// errOnce: an objective that does not count distinct things is done once; a count would only reward doing the
// same thing over (an emote toggled, a bike got on and off).
var errOnce = errors.New("count must be 1: this objective does not count distinct things, so repeating it is not progress")

var registry = map[string]Type{}

// Register adds a type; each type file calls it from init.
func Register(t Type) {
	if _, dup := registry[t.Kind()]; dup {
		panic("objective: type registered twice: " + t.Kind())
	}
	registry[t.Kind()] = t
}

// Lookup finds a type by kind.
func Lookup(kind string) (Type, bool) {
	t, ok := registry[kind]
	return t, ok
}

// Kinds lists the registered types.
func Kinds() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// decode reads params strictly into v.
func decode(params json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("params: %w", err)
	}
	return nil
}
