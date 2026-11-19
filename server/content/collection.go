package content

import (
	_ "embed"
	"errors"
	"fmt"
	"unicode/utf8"
)

// The Sổ sưu tầm (collection.json, docs/design/10-story.md, "Nhiệm vụ ngầm: Sổ sưu tầm"): a hidden collection
// every player can fill. The server runs each entry as a quest-engine track with one objective (record id
// "collection:<id>"); the game reads the same file for the text (src/game/collection.ts). This file checks its
// shape; the objective's params are checked with the quests' (checkQuests) and by the quest engine.

//go:embed collection.json
var collectionJSON []byte

// Collection kinds, in the order the Sổ tay shows them.
const (
	CollectionPhrase = "phrase"
	CollectionPlace  = "place"
	CollectionFood   = "food"
)

var collectionKinds = map[string]bool{CollectionPhrase: true, CollectionPlace: true, CollectionFood: true}

// CollectionTextMax keeps an entry's text readable on a page of the Sổ tay (characters)
const CollectionTextMax = 400

// CollectionEntry is one thing to collect.
type CollectionEntry struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Title is the phrase itself for a phrase ("Xin chào")
	Title Text `json:"title"`
	// Text is what the player reads once found: the meaning, when to use it, how to say it (in the English)
	Text Text `json:"text"`
	// Hint is shown while the entry is still "???"
	Hint Text `json:"hint"`
	// Objective is what finds it, the same shape as a quest objective (one, no id)
	Objective Objective `json:"objective"`
}

func parseCollection(f files, c *Catalog) error {
	var col struct {
		About   string            `json:"about"`
		Entries []CollectionEntry `json:"entries"`
	}
	if err := decodeStrict(f.collection, &col); err != nil {
		return fmt.Errorf("collection.json: %w", err)
	}
	seen := map[string]bool{}
	for i, e := range col.Entries {
		if err := e.check(seen); err != nil {
			return fmt.Errorf("collection.json: entry %d (%q): %w", i, e.ID, err)
		}
		seen[e.ID] = true
	}
	c.Collection = col.Entries
	return nil
}

func (e CollectionEntry) check(seen map[string]bool) error {
	switch {
	case !idPattern.MatchString(e.ID):
		return errors.New("id must be lower_snake_case ascii")
	case seen[e.ID]:
		return errors.New("duplicate id")
	case !collectionKinds[e.Kind]:
		return fmt.Errorf("kind must be %s, %s or %s", CollectionPhrase, CollectionPlace, CollectionFood)
	case e.Objective.ID != "":
		return errors.New("its objective has no id")
	}
	for _, f := range []struct {
		name string
		t    Text
	}{{"title", e.Title}, {"text", e.Text}, {"hint", e.Hint}} {
		if !bilingual(f.t) {
			return fmt.Errorf("%s needs Vietnamese and English", f.name)
		}
		if utf8.RuneCountInString(f.t.VI) > CollectionTextMax || utf8.RuneCountInString(f.t.EN) > CollectionTextMax {
			return fmt.Errorf("%s is longer than %d characters", f.name, CollectionTextMax)
		}
	}
	return nil
}

// collection checks the entries' objectives with the quests' references: an objective may point at places and NPCs.
func (r questRefs) collection() error {
	for _, e := range r.c.Collection {
		if err := r.objective(e.Objective, false); err != nil {
			return fmt.Errorf("collection.json: entry %q: objective: %w", e.ID, err)
		}
	}
	return nil
}
