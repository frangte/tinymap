package quest

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"saigon-noir/server/internal/quest/objective"
)

// ErrNotOpen: Complete was asked for a quest the character could not have started: it needs a quest, flag or
// capability the character does not have, or is not offered to its archetype.
var ErrNotOpen = errors.New("quest: not open to this character")

// Commit is what the outcome changed, as the store keeps it, at now (unix milliseconds).
func (o Outcome) Commit(now int64) Commit {
	c := Commit{Grants: o.Grants, Title: o.Title, Streak: o.Streak, Texts: o.Texts, Flags: o.Flags, At: now}
	for _, r := range o.Changed {
		c.Records = append(c.Records, *r)
	}
	return c
}

// Complete finishes a quest the way playing it would have: its objectives (and the chosen outcome's) count as
// done, and the quest ends through the same code that ends it in play, so the record, its rewards, flags and
// texts, and the quests and achievements the completion opens, are what the character would have stored. outcome
// is "" for a quest without outcomes; character is the id the events carry. The quest must be open to the character (Refresh starts it); a daily or
// an achievement cannot be completed this way. When it errs, the Outcome still holds the quests the refresh
// started (already in the book): store it all the same.
func (e *Engine) Complete(p Player, b *Book, character, id, outcome string, now int64) (Outcome, error) {
	var out Outcome
	t := e.byID[id]
	if t == nil || !t.quest || t.daily {
		return out, fmt.Errorf("quest %q: only a quest done once can be completed", id)
	}
	e.refresh(p, b, now, &out)
	r := b.Records[id]
	switch {
	case r == nil:
		return out, fmt.Errorf("%w: %s (%s)", ErrNotOpen, id, t.whyNot(p, b))
	case r.State == StateDone:
		return out, fmt.Errorf("quest %q is done already", id)
	}
	chosen, err := t.pick(outcome)
	if err != nil {
		return out, err
	}
	for _, g := range t.goals {
		r.Objectives[g.id] = fullProgress(g.count)
	}
	if chosen.id != "" {
		r.Objectives[chosen.id] = fullProgress(chosen.count)
	}
	out.changed(r)
	if t.ended(r) == nil {
		return Outcome{}, fmt.Errorf("quest %q did not end with its objectives done", id)
	}
	e.finish(t, r, chosen, now, b, &out)
	queue := e.followUp(b, character, id, "", nil, &out)
	e.drain(p, b, queue, now, &out)
	return out, nil
}

// whyNot says what keeps the quest from being offered to the character (the checks of availableTo, in its order).
func (t *track) whyNot(p Player, b *Book) string {
	var why []string
	if !t.taggedFor(p) {
		why = append(why, fmt.Sprintf("not offered to this origin (tags %v)", t.tagsAny))
	}
	for _, k := range t.unlessCaps {
		if p.Caps.Has(k) {
			why = append(why, "the character already has capability "+k)
		}
	}
	for _, k := range t.requiresCaps {
		if !p.Caps.Has(k) {
			why = append(why, "needs capability "+k)
		}
	}
	for _, f := range t.requiresFlag {
		if !b.Flags[f] {
			why = append(why, "needs flag "+f)
		}
	}
	for _, f := range t.unlessFlags {
		if b.Flags[f] {
			why = append(why, "flag "+f+" closes it")
		}
	}
	for _, id := range t.requires {
		if r := b.Records[id]; r == nil || r.State != StateDone {
			why = append(why, "needs quest "+id+" done")
		}
	}
	if len(why) == 0 {
		return "not open"
	}
	return strings.Join(why, "; ")
}

// pick is the outcome named, or {} for a quest without outcomes.
func (t *track) pick(id string) (*outcome, error) {
	if len(t.outcomes) == 0 {
		if id != "" {
			return nil, fmt.Errorf("quest %q has no outcomes, not %q", t.id, id)
		}
		return &outcome{}, nil
	}
	i := slices.IndexFunc(t.outcomes, func(o outcome) bool { return o.id == id })
	if i < 0 {
		return nil, fmt.Errorf("quest %q has no outcome %q", t.id, id)
	}
	return &t.outcomes[i], nil
}

func fullProgress(n int) *objective.Progress { return &objective.Progress{N: n} }
