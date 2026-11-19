package store

import (
	"context"
	"testing"

	"saigon-noir/server/internal/ridehail"
)

func npcTrip(ride, driver string, stars int) ridehail.NPCTrip {
	return ridehail.NPCTrip{Ride: ride, Driver: driver, Passenger: "chi_thao", Dest: "cho_ben_thanh", PickupX: 1, PickupZ: 2, Fare: 16_000, Paid: 12_800,
		Stars: stars, Day: "2026-11-06", DailyTrips: 2, AcceptedAt: 11, BoardedAt: 12, FinishedAt: 20}
}

func TestAnNPCTripPaysTheDriverOnceFromNobody(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	_, d := passengerAndDriver(t, db)
	for range 2 { // a retried settlement pays once and answers the same
		paid, bal, err := db.SettleNPC(ctx, npcTrip("n1", d, 4))
		if err != nil || paid != 12_800 || bal != 112_800 {
			t.Fatalf("settle: %d %d, %v", paid, bal, err)
		}
	}
	if ledgerSum(t, db, d) != 112_800 {
		t.Fatalf("the ledger adds up to %d", ledgerSum(t, db, d))
	}
	var reason string
	if err := db.sql.QueryRow(`select reason from ledger where ride_id = 'n1'`).Scan(&reason); err != nil || reason != "ride_npc" {
		t.Fatalf("the ledger reason: %q, %v", reason, err)
	}
	// the stars count in the driver's stats, with the real trips
	if _, err := db.Hold(ctx, ridehail.Hold{Ride: "r1", Passenger: "char1", Dest: "cho_ben_thanh", Fare: 16_000, At: 10}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "r1", Passenger: "char1", Driver: d, Paid: 12_800, State: "completed", EndReason: "completed", FinishedAt: 20}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Rate(ctx, ridehail.Rating{Ride: "r1", Passenger: "char1", Driver: d, Stars: 2, At: 30}); err != nil {
		t.Fatal(err)
	}
	trips, stars, err := db.DriverStats(ctx, d)
	if err != nil || trips != 2 || stars != 3 {
		t.Fatalf("stats: %d trips, %.1f stars, %v", trips, stars, err)
	}
}

func TestTheDailyCapPaysNothingMore(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	_, d := passengerAndDriver(t, db)
	for i, want := range []int64{12_800, 12_800, 0} {
		paid, _, err := db.SettleNPC(ctx, npcTrip("n"+string(rune('1'+i)), d, 5))
		if err != nil || paid != want {
			t.Fatalf("trip %d paid %d, %v, want %d", i+1, paid, err, want)
		}
	}
	if n, err := db.NPCTripsToday(ctx, d, "2026-11-06"); err != nil || n != 2 {
		t.Fatalf("paid trips today: %d, %v", n, err)
	}
	// another day, a fresh cap
	later := npcTrip("n4", d, 5)
	later.Day = "2026-11-07"
	if paid, _, err := db.SettleNPC(ctx, later); err != nil || paid != 12_800 {
		t.Fatalf("next day: %d, %v", paid, err)
	}
	if ledgerSum(t, db, d) != 100_000+3*12_800 {
		t.Fatalf("the ledger adds up to %d", ledgerSum(t, db, d))
	}
}
