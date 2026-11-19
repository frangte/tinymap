package store

import (
	"context"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/shop"
)

// A driver from before the job was earned in play gets what the job is made of (the bike, the uniform, the licence,
// a smartphone if it has none) and becomes a Native Saigonese; everyone else is left as they are (0012).
func TestTheMigrationTurnsDriversIntoOwnersOfTheirJob(t *testing.T) {
	opt := tempOptions(t)
	before := fstest.MapFS{}
	err := fs.WalkDir(migrationFiles, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path >= "migrations/0012" {
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
	look := protocol.Look{Body: "man", Outfit: "crab", Hat: "helmet"}
	for _, c := range []struct{ id, arch string }{{"drv-bare", "driver"}, {"drv-brick", "driver"}, {"cit", "citizen"}, {"trv", "traveler"}} {
		if err := db.CreateAccount(ctx, "acc-"+c.id, []byte("hash-"+c.id), 1); err != nil {
			t.Fatal(err)
		}
		if err := db.CreateCharacter(ctx, identity.Character{ID: c.id, AccountID: "acc-" + c.id, Name: c.id, Look: look, CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		if err := db.ChangeArchetype(ctx, c.id, c.arch, look, 2); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ArriveWallet(ctx, "drv-brick", 1_000_000, 1_000_000, "2026-11-19", 3); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Buy(ctx, shop.Purchase{Character: "drv-brick", Item: "nokla_1280", Price: 150_000, Grants: []string{"phone.basic"}, At: 4}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db = mustOpen(t, opt, migrationFiles)
	wantDriver := []string{"driver.app", "ridehail.drive", "uniform.crab", "vehicle.motorbike.own", "vehicle.motorbike.ride"}
	for id, wantItems := range map[string][]string{
		"drv-bare":  {"xe_so_110", "dong_phuc_crab", "ifone_x"},
		"drv-brick": {"nokla_1280", "xe_so_110", "dong_phuc_crab"},
	} {
		owned, err := db.Owned(ctx, id)
		if err != nil || !slices.Equal(owned, wantItems) {
			t.Errorf("%s owns %v, %v; want %v", id, owned, err, wantItems)
		}
		caps, err := db.EarnedCapabilities(ctx, id)
		want := append([]string{"phone.basic"}, wantDriver...)
		if id == "drv-bare" {
			want = append(want, "phone.smart")
		}
		slices.Sort(want)
		if err != nil || !slices.Equal(caps, want) {
			t.Errorf("%s has %v, %v; want %v", id, caps, err, want)
		}
		c, err := db.CharacterOf(ctx, "acc-"+id)
		if err != nil || c.Archetype != "citizen" {
			t.Errorf("%s is %q, %v: a driver becomes a Native Saigonese", id, c.Archetype, err)
		}
	}
	// the brick's owner paid for it: its price stays, and the free things cost nothing
	var price int64
	if err := db.sql.QueryRow(`select price from owned_items where character_id = 'drv-brick' and item_id = 'dong_phuc_crab'`).Scan(&price); err != nil || price != 0 {
		t.Errorf("the uniform cost %d, %v", price, err)
	}
	// nobody else is touched
	for id, arch := range map[string]string{"cit": "citizen", "trv": "traveler"} {
		if owned, _ := db.Owned(ctx, id); len(owned) != 0 {
			t.Errorf("%s owns %v", id, owned)
		}
		if c, err := db.CharacterOf(ctx, "acc-"+id); err != nil || c.Archetype != arch {
			t.Errorf("%s is %q, %v; want %s", id, c.Archetype, err, arch)
		}
	}
}
