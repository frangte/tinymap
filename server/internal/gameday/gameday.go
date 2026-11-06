// Package gameday is the game's calendar: a day runs midnight to midnight in Saigon (UTC+7, no daylight saving),
// for the wallet's daily top-up and the daily quests. Days are "2006-01-02" strings, which sort by date.
package gameday

import "time"

var saigon = time.FixedZone("ICT", 7*60*60)

const layout = "2006-01-02"

// Of is the day of t.
func Of(t time.Time) string { return t.In(saigon).Format(layout) }

// OfMilli is the day of a unix time in milliseconds.
func OfMilli(ms int64) string { return Of(time.UnixMilli(ms)) }

// Before is the day before day ("" for a day that does not parse).
func Before(day string) string {
	t, err := time.ParseInLocation(layout, day, saigon)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -1).Format(layout)
}
