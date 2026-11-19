package store

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/shop"
)

func ifoneX(id string) shop.Purchase {
	return shop.Purchase{Character: id, Item: "ifone_x", Price: 400_000, Grants: []string{"phone.basic", "phone.smart"}, At: 10}
}

func TestBuyingAnItemPaysAndGrants(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 500_000, 50_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	b, err := db.Buy(ctx, ifoneX(id))
	if err != nil || b != 100_000 {
		t.Fatalf("buy: %d, %v", b, err)
	}
	owned, err := db.Owned(ctx, id)
	if err != nil || !slices.Equal(owned, []string{"ifone_x"}) {
		t.Fatalf("owned: %v, %v", owned, err)
	}
	// the capabilities are the character's from its next arrival on, like any earned one
	caps, err := db.EarnedCapabilities(ctx, id)
	if err != nil || !slices.Equal(caps, []string{"phone.basic", "phone.smart"}) {
		t.Fatalf("capabilities: %v, %v", caps, err)
	}
	var source string
	if err := db.sql.QueryRow(`select distinct source from capability_grants where character_id = ?`, id).Scan(&source); err != nil || source != "item:ifone_x" {
		t.Fatalf("grant source %q, %v", source, err)
	}
	var delta int64
	var reason string
	if err := db.sql.QueryRow(`select delta, reason from ledger where character_id = ? and reason = 'buy'`, id).Scan(&delta, &reason); err != nil || delta != -400_000 {
		t.Fatalf("ledger line: %d %s, %v", delta, reason, err)
	}
	if n := ledgerSum(t, db, id); n != 100_000 {
		t.Fatalf("the ledger adds up to %d", n)
	}
}

func TestARetriedPurchaseMovesNoMoneyTwice(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 900_000, 50_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Buy(ctx, ifoneX(id)); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := db.Buy(ctx, ifoneX(id)); !errors.Is(err, shop.ErrOwned) {
			t.Fatalf("a retry: %v, want ErrOwned", err)
		}
	}
	if b, _ := db.ArriveWallet(ctx, id, 900_000, 50_000, "2026-11-19", 2); b != 500_000 {
		t.Fatalf("balance %d: it was charged more than once", b)
	}
	if n := ledgerSum(t, db, id); n != 500_000 {
		t.Fatalf("the ledger adds up to %d", n)
	}
	if owned, _ := db.Owned(ctx, id); len(owned) != 1 {
		t.Fatalf("owned %v", owned)
	}
}

func TestAFailedPurchaseKeepsNothing(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 399_999, 50_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Buy(ctx, ifoneX(id)); !errors.Is(err, shop.ErrBalance) {
		t.Fatalf("one xu short: %v, want ErrBalance", err)
	}
	if owned, _ := db.Owned(ctx, id); len(owned) != 0 {
		t.Fatalf("a failed purchase kept %v", owned)
	}
	if caps, _ := db.EarnedCapabilities(ctx, id); len(caps) != 0 {
		t.Fatalf("a failed purchase granted %v", caps)
	}
	if n := ledgerSum(t, db, id); n != 399_999 {
		t.Fatalf("the ledger adds up to %d", n)
	}
	// and the cheaper one still works afterwards: the failed try left no half-bought row behind
	nokla := shop.Purchase{Character: id, Item: "ifone_x", Price: 399_999, Grants: []string{"phone.basic"}, At: 11}
	if b, err := db.Buy(ctx, nokla); err != nil || b != 0 {
		t.Fatalf("buying it at a price the wallet covers: %d, %v", b, err)
	}
}

func TestAnItemNeedsAWallet(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	id := character(t, db)
	if _, err := db.Buy(context.Background(), ifoneX(id)); !errors.Is(err, shop.ErrBalance) {
		t.Fatalf("no wallet: %v, want ErrBalance", err)
	}
	if owned, _ := db.Owned(context.Background(), id); len(owned) != 0 {
		t.Fatalf("owned %v", owned)
	}
}

func TestOwnedItemsComeInTheOrderBought(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 800_000, 50_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	for i, item := range []string{"nokla_1280", "ifone_x"} {
		if _, err := db.Buy(ctx, shop.Purchase{Character: id, Item: item, Price: 100, Grants: []string{"phone.basic"}, At: int64(10 + i)}); err != nil {
			t.Fatal(err)
		}
	}
	if owned, _ := db.Owned(ctx, id); !slices.Equal(owned, []string{"nokla_1280", "ifone_x"}) {
		t.Fatalf("owned %v", owned)
	}
	if owned, err := db.Owned(ctx, "nobody"); err != nil || owned == nil || len(owned) != 0 {
		t.Fatalf("a stranger owns %v, %v: want an empty list", owned, err)
	}
}

// Every character from before items gets an Ifone X for free, with the capabilities of one.
func TestTheBackfillGivesEveryOldCharacterAnIfoneX(t *testing.T) {
	opt := tempOptions(t)
	before := fstest.MapFS{}
	err := fs.WalkDir(migrationFiles, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path >= "migrations/0009" {
			return err
		}
		b, err := fs.ReadFile(migrationFiles, path)
		before[path] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	db := mustOpen(t, opt, before)
	ctx := context.Background()
	look := protocol.Look{Body: "man", Outfit: "thun", Hat: "none"}
	for _, who := range []string{"a", "b"} {
		if err := db.CreateAccount(ctx, "acc-"+who, []byte("hash-"+who), 1); err != nil {
			t.Fatal(err)
		}
		if err := db.CreateCharacter(ctx, identity.Character{ID: "char-" + who, AccountID: "acc-" + who, Name: who, Look: look, CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	db = mustOpen(t, opt, migrationFiles)
	for _, id := range []string{"char-a", "char-b"} {
		owned, err := db.Owned(ctx, id)
		if err != nil || !slices.Equal(owned, []string{"ifone_x"}) {
			t.Fatalf("%s owns %v, %v", id, owned, err)
		}
		caps, err := db.EarnedCapabilities(ctx, id)
		if err != nil || !slices.Equal(caps, []string{"phone.basic", "phone.smart"}) {
			t.Fatalf("%s has %v, %v", id, caps, err)
		}
		var price int64
		if err := db.sql.QueryRow(`select price from owned_items where character_id = ?`, id).Scan(&price); err != nil || price != 0 {
			t.Fatalf("%s: the free phone cost %d, %v", id, price, err)
		}
	}
	// a character made after the migration owns nothing
	if err := db.CreateAccount(ctx, "acc-c", []byte("hash-c"), 2); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateCharacter(ctx, identity.Character{ID: "char-c", AccountID: "acc-c", Name: "c", Look: look, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	if owned, _ := db.Owned(ctx, "char-c"); len(owned) != 0 {
		t.Fatalf("a new character owns %v", owned)
	}
}
