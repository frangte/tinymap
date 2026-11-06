package quest

import (
	"sort"

	"saigon-noir/server/internal/protocol"
)

// News kinds: what just happened, for the game to celebrate.
const (
	NewsQuest       = "quest"
	NewsAchievement = "achievement"
	NewsTitle       = "title"
	NewsStamp       = "stamp"
)

// News is one thing that just happened.
type News = protocol.News

// progress is the whole of a character's quests as the game shows them, with what just happened.
func (e *Engine) progress(b *Book, news []News) []byte {
	msg := protocol.Progress{T: protocol.TypeProgress, Quests: []protocol.QuestState{}, Achievements: []protocol.AchievementState{},
		Titles: keys(b.Titles), Stamps: keys(b.Stamps), Title: b.Title, News: news}
	for _, t := range e.tracks {
		r := b.Records[t.id]
		if r == nil {
			continue
		}
		done := r.State == StateDone
		if !t.quest {
			n := 0
			if p := r.Objectives[""]; p != nil {
				n = p.N
			}
			msg.Achievements = append(msg.Achievements, protocol.AchievementState{ID: t.id[len(achievementPrefix):], N: n, Done: done})
			continue
		}
		q := protocol.QuestState{ID: t.id, Done: done}
		if !done {
			q.N = map[string]int{}
			for _, g := range t.goals {
				if p := r.Objectives[g.id]; p != nil {
					q.N[g.id] = p.N
				}
			}
		}
		msg.Quests = append(msg.Quests, q)
	}
	return protocol.Encode(msg)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
