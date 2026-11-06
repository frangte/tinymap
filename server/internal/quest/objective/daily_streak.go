package objective

import (
	"encoding/json"
	"errors"

	"saigon-noir/server/internal/events"
)

// daily_streak: do a daily quest on count days in a row ({}). A missed day starts the streak over, but progress
// already made stays: the best streak counts.
type dailyStreak struct{}

func init() { Register(dailyStreak{}) }

func (dailyStreak) Kind() string { return "daily_streak" }

func (dailyStreak) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct{}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if count < 2 {
		return nil, errors.New("count is days in a row, at least 2")
	}
	return dailyStreakMatcher{}, nil
}

type dailyStreakMatcher struct{}

func (dailyStreakMatcher) Listens() []events.Kind { return []events.Kind{events.DailyStreak} }

func (dailyStreakMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return p.Count(min(ev.Days, max)-p.N, max)
}
