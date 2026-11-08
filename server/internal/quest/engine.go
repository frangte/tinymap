// Package quest runs quests and achievements, with the server in charge (docs/design/02-architecture.md, "How a
// quest step completes"). Progress only ever comes from events the world observed (server/internal/events):
// a client cannot say "objective done", it can only move, talk, pick answers and play emotes, each checked.
//
// The Engine is the rules: pure, the same for everyone, compiled from quests.json. The Service runs them for
// the characters in the world on a few worker goroutines (each character always on the same one, so its events
// apply in order), stores each event's changes in one transaction, and tells the world what to show.
package quest

import (
	"fmt"
	"hash/fnv"
	"slices"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/gameday"
	"saigon-noir/server/internal/quest/objective"
)

// Engine is the compiled quests and achievements.
type Engine struct {
	tracks []*track
	byID   map[string]*track
	// pools are the dailies that take turns, by pool, in file order
	pools map[string][]string
}

// track is a quest or an achievement: objectives to do, in a flow, for a reward.
type track struct {
	// id is the record id: the quest id, or achievementPrefix + the achievement id
	id         string
	quest      bool
	version    int
	tagsAny    []string
	requires   []string
	flow       string
	goals      []goal
	rewards    content.Rewards
	listens    map[events.Kind]bool
	rewardFrom string
	// daily: it starts over every day; pool, when set, is the pool it takes turns in
	daily bool
	pool  string
}

type goal struct {
	id    string
	count int
	match objective.Matcher
}

// catalogRefs lets objective types look up the game data.
type catalogRefs struct{ *content.Catalog }

func (r catalogRefs) Places() []content.Place { return r.Catalog.Places }
func (r catalogRefs) NPCs() []content.NPC     { return r.Catalog.NPCs }
func (r catalogRefs) Emote(id string) (content.Emote, bool) {
	return r.Catalog.Emote(id)
}
func (r catalogRefs) Vehicle(id string) (content.Vehicle, bool) { return r.Catalog.Vehicle(id) }

// NewEngine compiles the game data's quests and achievements; an objective whose params do not check out is an
// error naming it.
func NewEngine(c *content.Catalog) (*Engine, error) {
	e := &Engine{byID: map[string]*track{}, pools: map[string][]string{}}
	refs := catalogRefs{c}
	for _, q := range c.Quests {
		t := &track{id: q.ID, quest: true, version: q.Version, tagsAny: q.TagsAny, requires: q.Requires, flow: q.Flow,
			rewards: q.Rewards, rewardFrom: "quest:" + q.ID + ":", daily: q.Repeat == content.RepeatDaily, pool: q.Pool}
		if t.pool != "" {
			e.pools[t.pool] = append(e.pools[t.pool], t.id)
		}
		for _, o := range q.Objectives {
			if err := t.add(o.ID, o, refs); err != nil {
				return nil, fmt.Errorf("quest %s: objective %s: %w", q.ID, o.ID, err)
			}
		}
		e.add(t)
	}
	for _, a := range c.Achievements {
		t := &track{id: achievementPrefix + a.ID, version: 1, tagsAny: a.TagsAny, flow: content.FlowSequence, rewards: a.Rewards,
			rewardFrom: achievementPrefix + a.ID}
		if err := t.add("", a.Objective, refs); err != nil {
			return nil, fmt.Errorf("achievement %s: %w", a.ID, err)
		}
		e.add(t)
	}
	return e, nil
}

func (e *Engine) add(t *track) {
	e.tracks = append(e.tracks, t)
	e.byID[t.id] = t
}

// onToday says whether a daily is on that day: one of each pool takes its turn, picked by the date (so everyone
// gets the same one); a daily without a pool is on every day.
func (e *Engine) onToday(t *track, day string) bool {
	ids := e.pools[t.pool]
	if t.pool == "" || len(ids) == 0 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(day + "/" + t.pool))
	return ids[int(h.Sum32()%uint32(len(ids)))] == t.id
}

func (t *track) add(id string, o content.Objective, refs objective.Refs) error {
	typ, ok := objective.Lookup(o.Type)
	if !ok {
		return fmt.Errorf("unknown type %q (known: %v)", o.Type, objective.Kinds())
	}
	m, err := typ.Compile(o.Params, o.Times(), refs)
	if err != nil {
		return fmt.Errorf("%s: %w", o.Type, err)
	}
	if t.listens == nil {
		t.listens = map[events.Kind]bool{}
	}
	for _, k := range m.Listens() {
		t.listens[k] = true
	}
	t.goals = append(t.goals, goal{id: id, count: o.Times(), match: m})
	return nil
}

// Player is what the engine needs to know about a character: the tags of its archetype.
type Player struct {
	Tags []string
}

// Book is a character's progress, in memory.
type Book struct {
	Records map[string]*Record
	Titles  map[string]bool
	Stamps  map[string]bool
	// Title is the title carried, "" for none
	Title string
	// Streak is the days in a row with a daily done
	Streak Streak
}

// NewBook makes a book from what the store kept.
func NewBook(s Saved) *Book {
	b := &Book{Records: map[string]*Record{}, Titles: map[string]bool{}, Stamps: map[string]bool{}, Title: s.Title, Streak: s.Streak}
	for i := range s.Records {
		r := s.Records[i]
		// a daily has a row a day: the latest is the one that counts (days sort by date, and "" first)
		if had := b.Records[r.ID]; had != nil && had.Cycle > r.Cycle {
			continue
		}
		if r.Objectives == nil {
			r.Objectives = map[string]*objective.Progress{}
		}
		for k, p := range r.Objectives {
			if p == nil {
				r.Objectives[k] = &objective.Progress{} // a damaged row: start that objective over
			}
		}
		b.Records[r.ID] = &r
	}
	for _, t := range s.Titles {
		b.Titles[t] = true
	}
	for _, st := range s.Stamps {
		b.Stamps[st] = true
	}
	return b
}

func (b *Book) clone() *Book {
	c := &Book{Records: make(map[string]*Record, len(b.Records)), Titles: make(map[string]bool, len(b.Titles)),
		Stamps: make(map[string]bool, len(b.Stamps)), Title: b.Title, Streak: b.Streak}
	for k, r := range b.Records {
		c.Records[k] = r.clone()
	}
	for k := range b.Titles {
		c.Titles[k] = true
	}
	for k := range b.Stamps {
		c.Stamps[k] = true
	}
	return c
}

// Active is the ids of the quests running (achievements aside).
func (b *Book) Active() map[string]bool {
	out := map[string]bool{}
	for id, r := range b.Records {
		if r.State == StateActive && !isAchievement(id) {
			out[id] = true
		}
	}
	return out
}

func isAchievement(id string) bool {
	return len(id) > len(achievementPrefix) && id[:len(achievementPrefix)] == achievementPrefix
}

// Outcome is what a step changed.
type Outcome struct {
	// Changed are the records to store, by id
	Changed map[string]*Record
	// Grants are the rewards earned (each given once, by its source)
	Grants []Grant
	// Completed are the ids of the records completed, in order
	Completed []string
	// Title is set when the character now carries another title (its first one earned)
	Title *string
	// Dropped are yesterday's dailies, gone with their day (their rows stay, under that day)
	Dropped []string
	// Streak is set when the streak of dailies changed
	Streak *Streak
}

// Empty: nothing changed.
func (o Outcome) Empty() bool {
	return len(o.Changed) == 0 && len(o.Grants) == 0 && o.Title == nil && len(o.Dropped) == 0 && o.Streak == nil
}

func (o *Outcome) changed(r *Record) {
	if o.Changed == nil {
		o.Changed = map[string]*Record{}
	}
	o.Changed[r.ID] = r
}

// maxChain bounds completions that set off more completions within one step (a quest completing an achievement):
// content cannot loop (requires has no cycles), so it is never reached; it only keeps a bug from spinning.
const maxChain = 256

// Refresh starts every track the character may now take and has not started.
func (e *Engine) Refresh(p Player, b *Book, now int64) Outcome {
	var out Outcome
	e.refresh(p, b, now, &out)
	return out
}

func (e *Engine) refresh(p Player, b *Book, now int64, out *Outcome) {
	day := gameday.OfMilli(now)
	elsewhere := e.dealtElsewhere(p, b, day)
	for _, t := range e.tracks {
		r, started := b.Records[t.id]
		if t.daily && started && r.Cycle != day {
			delete(b.Records, t.id)
			out.Dropped = append(out.Dropped, t.id)
			started = false
		}
		if started || !t.availableTo(p, b) || (t.daily && !e.onToday(t, day)) {
			continue
		}
		if t.daily && len(t.tagsAny) > 0 && elsewhere {
			continue
		}
		r = &Record{ID: t.id, Version: t.version, State: StateActive, Objectives: map[string]*objective.Progress{}, StartedAt: now}
		if t.daily {
			r.Cycle = day
		}
		for _, g := range t.goals {
			r.Objectives[g.id] = &objective.Progress{}
		}
		b.Records[t.id] = r
		out.changed(r)
	}
}

// dealtElsewhere: today the character already has the dailies of another archetype (it changed archetype since
// they were dealt). A character gets one archetype's dailies a day, the first it plays that day, so changing
// archetype does not deal another set; the dailies open to everyone are dealt either way.
func (e *Engine) dealtElsewhere(p Player, b *Book, day string) bool {
	for _, t := range e.tracks {
		if r := b.Records[t.id]; r != nil && t.daily && r.Cycle == day && len(t.tagsAny) > 0 && !t.taggedFor(p) {
			return true
		}
	}
	return false
}

// taggedFor: the track is offered to the player's archetype (or to everyone).
func (t *track) taggedFor(p Player) bool {
	return len(t.tagsAny) == 0 || slices.ContainsFunc(t.tagsAny, func(tag string) bool { return slices.Contains(p.Tags, tag) })
}

func (t *track) availableTo(p Player, b *Book) bool {
	if !t.taggedFor(p) {
		return false
	}
	for _, id := range t.requires {
		if r := b.Records[id]; r == nil || r.State != StateDone {
			return false
		}
	}
	return true
}

// Apply runs one event through the character's running tracks: progress, completions and their rewards, and the
// tracks that completions open up.
func (e *Engine) Apply(p Player, b *Book, ev events.Event, now int64) Outcome {
	var out Outcome
	day := gameday.OfMilli(now)
	queue := []events.Event{ev}
	for steps := 0; len(queue) > 0 && steps < maxChain; steps++ {
		ev, queue = queue[0], queue[1:]
		for _, id := range e.step(b, ev, now, &out) {
			if isAchievement(id) {
				continue
			}
			queue = append(queue, events.Event{Kind: events.QuestDone, Character: ev.Character, Quest: id})
			if e.byID[id].daily && b.streakUp(day) {
				s := b.Streak
				out.Streak = &s
				queue = append(queue, events.Event{Kind: events.DailyStreak, Character: ev.Character, Days: s.Days})
			}
		}
		e.refresh(p, b, now, &out)
	}
	return out
}

// streakUp counts a daily done on day: the streak goes on from yesterday, or starts again. False when a daily
// was already done that day (the streak has counted it).
func (b *Book) streakUp(day string) bool {
	switch b.Streak.Last {
	case day:
		return false
	case gameday.Before(day):
		b.Streak.Days++
	default:
		b.Streak.Days = 1
	}
	b.Streak.Last = day
	return true
}

// StreakNow is the streak as it stands on day: 0 once a whole day went by without a daily.
func (b *Book) StreakNow(day string) int {
	if b.Streak.Last == day || b.Streak.Last == gameday.Before(day) {
		return b.Streak.Days
	}
	return 0
}

// step applies one event and returns the tracks it completed.
func (e *Engine) step(b *Book, ev events.Event, now int64, out *Outcome) []string {
	var completed []string
	day := gameday.OfMilli(now)
	for _, t := range e.tracks {
		r := b.Records[t.id]
		// yesterday's daily takes nothing more: the refresh after this step replaces it with today's
		if r == nil || r.State != StateActive || !t.listens[ev.Kind] || (t.daily && r.Cycle != day) {
			continue
		}
		if !t.advance(r, ev) {
			continue
		}
		out.changed(r)
		if !t.complete(r) {
			continue
		}
		r.State, r.CompletedAt = StateDone, now
		completed = append(completed, t.id)
		out.Completed = append(out.Completed, t.id)
		e.reward(t, r, b, out)
	}
	return completed
}

// advance moves the objectives whose turn it is; true when any moved.
func (t *track) advance(r *Record, ev events.Event) bool {
	moved := false
	for _, g := range t.goals {
		p := r.Objectives[g.id]
		if p == nil {
			p = &objective.Progress{}
			r.Objectives[g.id] = p
		}
		if p.N >= g.count {
			continue
		}
		moved = g.match.Advance(ev, p, g.count) || moved
		if t.flow == content.FlowSequence {
			break // later objectives wait their turn
		}
	}
	if moved {
		r.Version = t.version
	}
	return moved
}

func (t *track) complete(r *Record) bool {
	done := 0
	for _, g := range t.goals {
		if p := r.Objectives[g.id]; p != nil && p.N >= g.count {
			done++
		}
	}
	if t.flow == content.FlowAny {
		return done > 0
	}
	return done == len(t.goals)
}

func (e *Engine) reward(t *track, r *Record, b *Book, out *Outcome) {
	w := t.rewards
	if len(w.Titles) == 0 && len(w.Stamps) == 0 && w.Capability == "" && w.Xu == 0 {
		return
	}
	g := Grant{Source: t.rewardFrom + r.Cycle, Titles: w.Titles, Stamps: w.Stamps, Capability: w.Capability, Xu: w.Xu}
	out.Grants = append(out.Grants, g)
	for _, s := range w.Stamps {
		b.Stamps[s] = true
	}
	// the first title ever earned is carried at once; later ones wait to be picked in the phone (and a player who
	// put theirs away keeps it that way)
	first := len(b.Titles) == 0
	for _, title := range w.Titles {
		b.Titles[title] = true
		if first && b.Title == "" {
			b.Title = title
			out.Title = &b.Title
		}
	}
}

// Track ids, for tests and the wire: quests in file order, then achievements.
func (e *Engine) trackIDs() []string {
	out := make([]string, 0, len(e.tracks))
	for _, t := range e.tracks {
		out = append(out, t.id)
	}
	return out
}
