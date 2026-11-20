package quest

import (
	"sort"

	"saigon-noir/server/internal/gameday"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/quest/objective"
)

// News kinds: what just happened, for the game to celebrate.
const (
	NewsQuest       = "quest"
	NewsAchievement = "achievement"
	NewsTitle       = "title"
	NewsStamp       = "stamp"
	NewsCollection  = "collection"
)

// News is one thing that just happened.
type News = protocol.News

// progress is the whole of a character's quests as the game shows them on day, with what just happened.
func (e *Engine) progress(b *Book, news []News, now int64) []byte {
	day := gameday.OfMilli(now)
	title := b.Title
	if !e.titles[title] {
		title = ""
	}
	msg := protocol.Progress{T: protocol.TypeProgress, Quests: []protocol.QuestState{}, Achievements: []protocol.AchievementState{},
		Titles: known(b.Titles, e.titles), Stamps: known(b.Stamps, e.stamps), Title: title, Streak: b.StreakNow(day), Collection: []string{}, News: news}
	for _, t := range e.tracks {
		r := b.Records[t.id]
		if r == nil {
			continue
		}
		done := r.State == StateDone
		if isCollection(t.id) {
			if id := t.id[len(collectionPrefix):]; done && e.collection[id] {
				msg.Collection = append(msg.Collection, id)
			}
			continue
		}
		if !t.quest {
			n := 0
			if p := r.Objectives[""]; p != nil {
				n = p.N
			}
			msg.Achievements = append(msg.Achievements, protocol.AchievementState{ID: t.id[len(achievementPrefix):], N: n, Done: done})
			continue
		}
		q := protocol.QuestState{ID: t.id, Done: done, Outcome: r.Outcome}
		if !done && len(t.outcomes) > 0 && t.goalsDone(r) {
			for _, o := range t.outcomes {
				n := 0
				if p := r.Objectives[o.id]; p != nil {
					n = p.N
				}
				q.Outcomes = append(q.Outcomes, protocol.OutcomeState{ID: o.id, N: n})
			}
		}
		if !done {
			q.N = map[string]int{}
			for _, g := range t.goals {
				p := r.Objectives[g.id]
				if p == nil {
					continue
				}
				q.N[g.id] = p.N
				if run, ok := g.match.(objective.Runner); ok {
					if left, on := run.Left(p, now); on {
						if q.Left == nil {
							q.Left = map[string]int{}
						}
						q.Left[g.id] = left
					}
				}
			}
		}
		msg.Quests = append(msg.Quests, q)
	}
	return protocol.Encode(msg)
}

// known is the ids in m that are still in the catalog (in), sorted.
func known(m, in map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if in[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
