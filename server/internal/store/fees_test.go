package store

import (
	"context"
	"errors"
	"slices"
	"testing"

	"saigon-noir/server/internal/shop"
)

func testFee(id string) shop.Payment {
	return shop.Payment{Character: id, Fee: "thi_a1", Amount: 200_000, Grants: []string{"exam.a1.ticket"}, At: 10}
}

func TestPayingAFeeTakesItGrantsTheTicketAndWritesTheLedger(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 500_000, 50_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	b, err := db.Pay(ctx, testFee(id))
	if err != nil || b != 300_000 {
		t.Fatalf("pay: %d, %v", b, err)
	}
	var delta int64
	if err := db.sql.QueryRow(`select delta from ledger where character_id = ? and reason = 'fee'`, id).Scan(&delta); err != nil || delta != -200_000 {
		t.Fatalf("ledger line: %d, %v", delta, err)
	}
	var source string
	if err := db.sql.QueryRow(`select source from capability_grants where character_id = ? and capability = 'exam.a1.ticket'`, id).Scan(&source); err != nil || source != "fee:thi_a1" {
		t.Fatalf("grant source %q, %v", source, err)
	}
	// holding the ticket, paying again moves no money
	if _, err := db.Pay(ctx, testFee(id)); !errors.Is(err, shop.ErrOwned) {
		t.Fatalf("a second payment: %v, want ErrOwned", err)
	}
	if n := ledgerSum(t, db, id); n != 300_000 {
		t.Fatalf("the ledger adds up to %d", n)
	}
}

// A fee is paid each time: spending the ticket makes the next payment possible, and it is charged again.
func TestAFeeIsPaidEachTime(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 500_000, 50_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pay(ctx, testFee(id)); err != nil {
		t.Fatal(err)
	}
	if err := db.RevokeCapability(ctx, id, "exam.a1.ticket"); err != nil {
		t.Fatal(err)
	}
	if caps, _ := db.EarnedCapabilities(ctx, id); len(caps) != 0 {
		t.Fatalf("a spent ticket is still held: %v", caps)
	}
	retake := shop.Payment{Character: id, Fee: "thi_lai_a1", Amount: 50_000, Grants: []string{"exam.a1.ticket"}, At: 11}
	if b, err := db.Pay(ctx, retake); err != nil || b != 250_000 {
		t.Fatalf("retake: %d, %v", b, err)
	}
	var n int
	if err := db.sql.QueryRow(`select count(*) from ledger where character_id = ? and reason = 'fee'`, id).Scan(&n); err != nil || n != 2 {
		t.Fatalf("%d fee lines, %v: each payment has its own", n, err)
	}
}

func TestAFeeTheWalletCannotCoverKeepsNothing(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 199_999, 50_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pay(ctx, testFee(id)); !errors.Is(err, shop.ErrBalance) {
		t.Fatalf("one xu short: %v, want ErrBalance", err)
	}
	if caps, _ := db.EarnedCapabilities(ctx, id); len(caps) != 0 {
		t.Fatalf("a refused fee granted %v", caps)
	}
	if n := ledgerSum(t, db, id); n != 199_999 {
		t.Fatalf("the ledger adds up to %d", n)
	}
}

func TestGrantingAndRevokingACapability(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	for range 2 { // granting twice is the same as once
		if err := db.GrantCapability(ctx, id, "exam.a1.theory", "test:a1", 5); err != nil {
			t.Fatal(err)
		}
	}
	if caps, _ := db.EarnedCapabilities(ctx, id); !slices.Equal(caps, []string{"exam.a1.theory"}) {
		t.Fatalf("caps %v", caps)
	}
	if err := db.RevokeCapability(ctx, id, "exam.a1.theory"); err != nil {
		t.Fatal(err)
	}
	if err := db.RevokeCapability(ctx, id, "exam.a1.theory"); err != nil {
		t.Fatalf("revoking what is not held is no error: %v", err)
	}
	if caps, _ := db.EarnedCapabilities(ctx, id); len(caps) != 0 {
		t.Fatalf("caps %v", caps)
	}
}
