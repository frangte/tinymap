package gameday

import (
	"testing"
	"time"
)

func TestADayTurnsAtMidnightInSaigon(t *testing.T) {
	// 16:59 UTC is 23:59 in Saigon; a minute later it is the next day there
	before := time.Date(2026, 11, 6, 16, 59, 0, 0, time.UTC)
	if got := Of(before); got != "2026-11-06" {
		t.Fatalf("23:59 in Saigon: %s", got)
	}
	if got := Of(before.Add(time.Minute)); got != "2026-11-07" {
		t.Fatalf("00:00 in Saigon: %s", got)
	}
	if got := OfMilli(before.UnixMilli()); got != "2026-11-06" {
		t.Fatalf("in milliseconds: %s", got)
	}
}

func TestTheDayBefore(t *testing.T) {
	for day, want := range map[string]string{"2026-11-07": "2026-11-06", "2026-03-01": "2026-02-28", "2027-01-01": "2026-12-31", "nope": ""} {
		if got := Before(day); got != want {
			t.Errorf("before %s: %q, want %q", day, got, want)
		}
	}
}
