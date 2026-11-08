package store

import (
	"context"
	"errors"
	"testing"

	"saigon-noir/server/internal/ridehail"
)

func TestAWalletStartsAndIsToppedUpOnceADay(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if b, err := db.ArriveWallet(ctx, id, 100_000, 50_000, "2026-11-06", 1); err != nil || b != 100_000 {
		t.Fatalf("first visit: %d, %v", b, err)
	}
	// spend down to 20.000, then come back later the same day: no top-up
	if _, err := db.Hold(ctx, ridehail.Hold{Ride: "r1", Passenger: id, Dest: "cho_ben_thanh", Fare: 80_000, At: 2}); err != nil {
		t.Fatal(err)
	}
	if b, _ := db.ArriveWallet(ctx, id, 100_000, 50_000, "2026-11-06", 3); b != 20_000 {
		t.Fatalf("same day: %d", b)
	}
	// the next day: back up to 50.000, once
	if b, _ := db.ArriveWallet(ctx, id, 100_000, 50_000, "2026-11-07", 4); b != 50_000 {
		t.Fatalf("next day: %d", b)
	}
	if b, _ := db.ArriveWallet(ctx, id, 100_000, 50_000, "2026-11-07", 5); b != 50_000 {
		t.Fatalf("next day again: %d", b)
	}
	if n := ledgerSum(t, db, id); n != 50_000 {
		t.Fatalf("the ledger adds up to %d", n)
	}
}

func ledgerSum(t *testing.T, db *DB, character string) int64 {
	t.Helper()
	var sum int64
	if err := db.sql.QueryRow(`select coalesce(sum(delta), 0) from ledger where character_id = ?`, character).Scan(&sum); err != nil {
		t.Fatal(err)
	}
	return sum
}

// two characters with wallets: a passenger and a driver
func passengerAndDriver(t *testing.T, db *DB) (string, string) {
	t.Helper()
	ctx := context.Background()
	p := character(t, db)
	if err := db.CreateAccount(ctx, "acc2", []byte("hash2"), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec(`insert into characters (id, account_id, name, appearance, created_at, updated_at) values ('char2', 'acc2', 'Tài', '{}', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{p, "char2"} {
		if _, err := db.ArriveWallet(ctx, c, 100_000, 50_000, "2026-11-06", 1); err != nil {
			t.Fatal(err)
		}
	}
	return p, "char2"
}

func TestATripMovesTheFareOnce(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	p, d := passengerAndDriver(t, db)
	h := ridehail.Hold{Ride: "r1", Passenger: p, Dest: "cho_ben_thanh", PickupX: 1, PickupZ: 2, Fare: 16_000, At: 10}
	for range 2 { // a retried hold takes the fare once
		if b, err := db.Hold(ctx, h); err != nil || b != 84_000 {
			t.Fatalf("hold: %d, %v", b, err)
		}
	}
	s := ridehail.Settlement{Ride: "r1", Passenger: p, Driver: d, Paid: 12_800, State: "completed", EndReason: "completed", AcceptedAt: 11, BoardedAt: 12, FinishedAt: 20}
	for range 2 { // and a retried payout pays once
		pb, db2, err := db.Settle(ctx, s)
		if err != nil || pb != 84_000 || db2 != 112_800 {
			t.Fatalf("settle: %d %d, %v", pb, db2, err)
		}
	}
	// the rating and a tip, once
	for range 2 {
		pb, dbal, err := db.Rate(ctx, ridehail.Rating{Ride: "r1", Passenger: p, Driver: d, Stars: 5, Tip: 5_000, At: 30})
		if err != nil || pb != 79_000 || dbal != 117_800 {
			t.Fatalf("rate: %d %d, %v", pb, dbal, err)
		}
	}
	trips, stars, err := db.DriverStats(ctx, d)
	if err != nil || trips != 1 || stars != 5 {
		t.Fatalf("stats: %d trips, %.1f stars, %v", trips, stars, err)
	}
	// the ledgers add up to the balances
	if ledgerSum(t, db, p) != 79_000 || ledgerSum(t, db, d) != 117_800 {
		t.Fatalf("ledgers %d / %d", ledgerSum(t, db, p), ledgerSum(t, db, d))
	}
}

func TestACancelledRideComesBack(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	p, _ := passengerAndDriver(t, db)
	if _, err := db.Hold(ctx, ridehail.Hold{Ride: "r1", Passenger: p, Dest: "bitexco", Fare: 20_000, At: 10}); err != nil {
		t.Fatal(err)
	}
	pb, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "r1", Passenger: p, Refund: 20_000, State: "expired", EndReason: "no_driver", FinishedAt: 70})
	if err != nil || pb != 100_000 {
		t.Fatalf("refund: %d, %v", pb, err)
	}
	var state string
	if err := db.sql.QueryRow(`select state from rides where id = 'r1'`).Scan(&state); err != nil || state != "expired" {
		t.Fatalf("ride state %q, %v", state, err)
	}
}

func TestAFareTooDearIsRefused(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	p, d := passengerAndDriver(t, db)
	if _, err := db.Hold(ctx, ridehail.Hold{Ride: "r1", Passenger: p, Dest: "bitexco", Fare: 150_000, At: 10}); !errors.Is(err, ridehail.ErrBalance) {
		t.Fatalf("hold of 150.000 with 100.000: %v", err)
	}
	if ledgerSum(t, db, p) != 100_000 {
		t.Fatal("a refused hold must leave nothing behind")
	}
	// a ride not completed takes no rating, and so no tip
	if _, err := db.Hold(ctx, ridehail.Hold{Ride: "r2", Passenger: p, Dest: "bitexco", Fare: 98_000, At: 11}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Rate(ctx, ridehail.Rating{Ride: "r2", Passenger: p, Driver: d, Stars: 4, Tip: 1_000, At: 12}); err != nil || ledgerSum(t, db, p) != 2_000 {
		t.Fatalf("a tip on a ride still open: %v, the passenger has %d", err, ledgerSum(t, db, p))
	}
	// a tip the passenger cannot pay is refused, with the rating
	if _, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "r2", Passenger: p, Driver: d, Paid: 78_400, State: "completed", EndReason: "completed", FinishedAt: 13}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Rate(ctx, ridehail.Rating{Ride: "r2", Passenger: p, Driver: d, Stars: 4, Tip: 5_000, At: 14}); !errors.Is(err, ridehail.ErrBalance) {
		t.Fatalf("tip of 5.000 with 2.000: %v", err)
	}
}

func TestSettlingIsOnlyForOpenRidesAndNeverMoreThanTheFare(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	p, d := passengerAndDriver(t, db)
	// never held: nothing moves
	if _, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "ghost", Passenger: p, Refund: 16_000, State: "cancelled", FinishedAt: 1}); err != nil || ledgerSum(t, db, p) != 100_000 {
		t.Fatalf("a refund of a ride never held: %v, %d", err, ledgerSum(t, db, p))
	}
	if _, err := db.Hold(ctx, ridehail.Hold{Ride: "r1", Passenger: p, Dest: "bitexco", Fare: 16_000, At: 2}); err != nil {
		t.Fatal(err)
	}
	// more than the fare: refused, nothing moves
	if _, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "r1", Passenger: p, Driver: d, Refund: 16_000, Paid: 12_800, State: "ended", FinishedAt: 3}); err == nil {
		t.Fatal("paying out more than the fare went through")
	}
	if ledgerSum(t, db, p) != 84_000 || ledgerSum(t, db, d) != 100_000 {
		t.Fatal("a refused settlement moved money")
	}
	// settled once, then again with other sums (a stale retry): the second moves nothing
	if _, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "r1", Passenger: p, Driver: d, Paid: 12_800, State: "completed", EndReason: "completed", FinishedAt: 4}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "r1", Passenger: p, Driver: d, Refund: 16_000, State: "cancelled", FinishedAt: 5}); err != nil {
		t.Fatal(err)
	}
	if ledgerSum(t, db, p) != 84_000 || ledgerSum(t, db, d) != 112_800 {
		t.Fatalf("after a second settlement: passenger %d, driver %d", ledgerSum(t, db, p), ledgerSum(t, db, d))
	}
}

func TestAStoppedServerGivesOpenHoldsBack(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	p, d := passengerAndDriver(t, db)
	for _, ride := range []string{"open", "done"} {
		if _, err := db.Hold(ctx, ridehail.Hold{Ride: ride, Passenger: p, Dest: "cho_ben_thanh", Fare: 16_000, At: 2}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := db.Settle(ctx, ridehail.Settlement{Ride: "done", Passenger: p, Driver: d, Paid: 12_800, State: "completed", EndReason: "completed", FinishedAt: 3}); err != nil {
		t.Fatal(err)
	}
	for range 2 { // a second start finds nothing left to give back
		if _, err := db.ReturnOpenHolds(ctx, 4); err != nil {
			t.Fatal(err)
		}
	}
	if n := ledgerSum(t, db, p); n != 100_000-16_000 {
		t.Fatalf("the passenger has %d: only the completed trip is paid", n)
	}
	var state, end string
	if err := db.sql.QueryRow(`select state, end_reason from rides where id = 'open'`).Scan(&state, &end); err != nil || state != "ended" || end != "server_stopped" {
		t.Fatalf("the open ride: %s %s %v", state, end, err)
	}
	if n, err := db.ReturnOpenHolds(ctx, 5); err != nil || n != 0 {
		t.Fatalf("nothing more to give back: %d %v", n, err)
	}
}
