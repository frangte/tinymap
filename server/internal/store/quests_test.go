package store

import (
	"context"
	"slices"
	"testing"

	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/quest"
	"saigon-noir/server/internal/quest/objective"
)

// character makes an account with one character and returns the character's id.
func character(t *testing.T, db *DB) string {
	t.Helper()
	ctx := context.Background()
	if err := db.CreateAccount(ctx, "acc1", []byte("hash1"), 1); err != nil {
		t.Fatal(err)
	}
	c := identity.Character{ID: "char1", AccountID: "acc1", Name: "Lan", Look: protocol.Look{Body: "woman", Outfit: "aodai", Hat: "nonla"}, CreatedAt: 1}
	if err := db.CreateCharacter(ctx, c); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func TestQuestProgressIsKept(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	rec := quest.Record{ID: "traveler.kien_truc_phap", Version: 1, State: quest.StateActive, StartedAt: 10,
		Objectives: map[string]*objective.Progress{"nha_tho": {N: 1}, "nha_hat": {}}}
	ach := quest.Record{ID: "achievement:kham_pha_quan_1", Version: 1, State: quest.StateActive, StartedAt: 10,
		Objectives: map[string]*objective.Progress{"": {N: 2, Seen: []string{"cho_ben_thanh", "bitexco"}}}}
	if _, err := db.SaveQuests(ctx, id, quest.Commit{Records: []quest.Record{rec, ach}, At: 10}); err != nil {
		t.Fatal(err)
	}
	// later the quest completes: the row is updated, not added
	rec.State, rec.CompletedAt = quest.StateDone, 20
	rec.Objectives["nha_hat"].N = 1
	if _, err := db.SaveQuests(ctx, id, quest.Commit{Records: []quest.Record{rec}, At: 20}); err != nil {
		t.Fatal(err)
	}
	s, err := db.LoadQuests(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Records) != 2 {
		t.Fatalf("records %+v", s.Records)
	}
	for _, r := range s.Records {
		switch r.ID {
		case rec.ID:
			if r.State != quest.StateDone || r.CompletedAt != 20 || r.Objectives["nha_hat"].N != 1 || r.StartedAt != 10 {
				t.Fatalf("quest %+v", r)
			}
		case ach.ID:
			if r.Objectives[""].N != 2 || !slices.Equal(r.Objectives[""].Seen, []string{"cho_ben_thanh", "bitexco"}) {
				t.Fatalf("achievement %+v", r.Objectives[""])
			}
		}
	}
}

func TestARewardIsGivenOnce(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	g := quest.Grant{Source: "quest:traveler.bai_kiem_tra_cuoi:", Titles: []string{"nguoi_sai_gon_chinh_hieu"},
		Stamps: []string{"chung_nhan"}, Capability: "local.certified"}
	title := "nguoi_sai_gon_chinh_hieu"
	first, err := db.SaveQuests(ctx, id, quest.Commit{Grants: []quest.Grant{g}, Title: &title, At: 30})
	if err != nil || !slices.Equal(first, []string{g.Source}) {
		t.Fatalf("first grant: %v, %v", first, err)
	}
	// the same grant again (a retry, a replay): skipped, nothing doubled
	again, err := db.SaveQuests(ctx, id, quest.Commit{Grants: []quest.Grant{g, {Source: "quest:other:", Stamps: []string{"hoi_duong"}}}, At: 40})
	if err != nil || !slices.Equal(again, []string{"quest:other:"}) {
		t.Fatalf("second commit granted %v, %v", again, err)
	}
	s, err := db.LoadQuests(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Titles, []string{"nguoi_sai_gon_chinh_hieu"}) || !slices.Equal(s.Stamps, []string{"chung_nhan", "hoi_duong"}) || s.Title != title {
		t.Fatalf("saved %+v", s)
	}
	// the next arrival carries the title and the earned capability
	c, err := db.CharacterOf(ctx, "acc1")
	if err != nil || c.Title != title || !slices.Equal(c.Caps, []string{"local.certified"}) {
		t.Fatalf("character %+v, %v", c, err)
	}
	// putting the title away
	none := ""
	if _, err := db.SaveQuests(ctx, id, quest.Commit{Title: &none, At: 50}); err != nil {
		t.Fatal(err)
	}
	if s, _ := db.LoadQuests(ctx, id); s.Title != "" {
		t.Fatalf("title after clearing %q", s.Title)
	}
}
