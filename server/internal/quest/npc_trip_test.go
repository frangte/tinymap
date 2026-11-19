package quest

import (
	"testing"

	"saigon-noir/server/internal/events"
)

// A trip with an NPC passenger publishes the same trip.completed as a real one (partner "npc:<passenger id>"), so a
// new driver can finish the first-trip quest and the daily with nobody else online (ridehail/npc.go).
func TestAnNPCTripCountsLikeARealOne(t *testing.T) {
	d := newRun(t, "driver")
	d.do(events.Event{Kind: events.Mounted, Vehicle: "xe_so_110"})
	d.do(events.Event{Kind: events.TripCompleted, Role: "rider", Partners: []string{"npc:chi_thao"}, Metres: 300})
	if d.state("driver.chuyen_dau_tien") != StateDone || !d.book.Titles["tai_xe_crab"] {
		t.Fatalf("first trip %+v, titles %v", d.book.Records["driver.chuyen_dau_tien"], d.book.Titles)
	}
}
