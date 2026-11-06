package quest

import (
	"context"

	"saigon-noir/server/internal/quest/objective"
)

// States of a record.
const (
	StateActive = "active"
	StateDone   = "done"
)

// achievementPrefix marks an achievement's record among quests' (they share the engine and the table).
const achievementPrefix = "achievement:"

// Record is one quest's (or achievement's) progress for one character, as it is stored.
type Record struct {
	// ID is the quest id, or achievementPrefix + the achievement id
	ID string
	// Cycle is "" for a quest done once (dailies, later, use the day)
	Cycle string
	// Version is the quest's version when the record started
	Version     int
	State       string
	Objectives  map[string]*objective.Progress
	StartedAt   int64
	CompletedAt int64
}

func (r *Record) clone() *Record {
	c := *r
	c.Objectives = make(map[string]*objective.Progress, len(r.Objectives))
	for k, p := range r.Objectives {
		c.Objectives[k] = p.Clone()
	}
	return &c
}

// Grant is a reward to give once: Source is its key ("quest:<id>:<cycle>", "achievement:<id>"); the store gives
// it only if that key was never given before, in the same transaction as the progress that earned it.
type Grant struct {
	Source     string
	Titles     []string
	Stamps     []string
	Capability string
}

// Saved is what the store keeps of a character's quests.
type Saved struct {
	Records []Record
	// Titles and Stamps are the ones earned; Title the one carried ("" for none)
	Titles []string
	Stamps []string
	Title  string
}

// Commit is everything one event changed for one character, stored in one transaction.
type Commit struct {
	Records []Record
	Grants  []Grant
	// Title, when set, is the title the character now carries
	Title *string
	// At is when (unix milliseconds)
	At int64
}

// Store keeps quest state (store.DB in production).
type Store interface {
	LoadQuests(ctx context.Context, character string) (Saved, error)
	// SaveQuests stores a commit in one transaction and returns the sources of the grants it gave (a grant given
	// before is skipped).
	SaveQuests(ctx context.Context, character string, c Commit) (granted []string, err error)
}
