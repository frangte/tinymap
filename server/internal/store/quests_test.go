package store

import (
	"context"
	"slices"
	"testing"

	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/moderation"
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
	if err != nil || !slices.Equal(first.Granted, []string{g.Source}) {
		t.Fatalf("first grant: %v, %v", first, err)
	}
	// the same grant again (a retry, a replay): skipped, nothing doubled
	again, err := db.SaveQuests(ctx, id, quest.Commit{Grants: []quest.Grant{g, {Source: "quest:other:", Stamps: []string{"hoi_duong"}}}, At: 40})
	if err != nil || !slices.Equal(again.Granted, []string{"quest:other:"}) {
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

func TestADailyPaysXuOnceADayAndKeepsItsStreak(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.ArriveWallet(ctx, id, 100_000, 50_000, "2026-11-06", 1); err != nil {
		t.Fatal(err)
	}
	day6 := quest.Grant{Source: "quest:daily.ghe_nguoi_quen:2026-11-06", Xu: 2_000}
	streak := quest.Streak{Days: 1, Last: "2026-11-06"}
	rec := quest.Record{ID: "daily.ghe_nguoi_quen", Cycle: "2026-11-06", Version: 1, State: quest.StateDone, StartedAt: 2, CompletedAt: 3}
	for range 2 { // a retried save pays once
		got, err := db.SaveQuests(ctx, id, quest.Commit{Records: []quest.Record{rec}, Grants: []quest.Grant{day6}, Streak: &streak, At: 3})
		if err != nil {
			t.Fatal(err)
		}
		if got.Paid && got.Balance != 102_000 {
			t.Fatalf("balance after the reward %d", got.Balance)
		}
	}
	// the next day: another row, another payment
	rec7 := rec
	rec7.Cycle = "2026-11-07"
	day7 := quest.Grant{Source: "quest:daily.ghe_nguoi_quen:2026-11-07", Xu: 2_000}
	streak = quest.Streak{Days: 2, Last: "2026-11-07"}
	got, err := db.SaveQuests(ctx, id, quest.Commit{Records: []quest.Record{rec7}, Grants: []quest.Grant{day7}, Streak: &streak, At: 4})
	if err != nil || !got.Paid || got.Balance != 104_000 || !slices.Equal(got.Granted, []string{day7.Source}) {
		t.Fatalf("second day: %+v, %v", got, err)
	}
	if n := ledgerSum(t, db, id); n != 104_000 {
		t.Fatalf("the ledger adds up to %d", n)
	}
	var reasons int
	if err := db.sql.QueryRow(`select count(*) from ledger where character_id = ? and reason = 'quest_reward'`, id).Scan(&reasons); err != nil || reasons != 2 {
		t.Fatalf("%d quest_reward lines, %v", reasons, err)
	}
	s, err := db.LoadQuests(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if s.Streak != streak {
		t.Fatalf("streak loaded %+v", s.Streak)
	}
	cycles := 0
	for _, r := range s.Records {
		if r.ID == "daily.ghe_nguoi_quen" {
			cycles++
		}
	}
	if cycles != 2 {
		t.Fatalf("%d rows of the daily, want one a day", cycles)
	}
}

func TestARewardWithoutAWalletIsGrantedButNotPaid(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	got, err := db.SaveQuests(ctx, id, quest.Commit{Grants: []quest.Grant{{Source: "quest:daily.x:2026-11-06", Xu: 5_000}}, At: 1})
	if err != nil || got.Paid || len(got.Granted) != 1 {
		t.Fatalf("%+v, %v", got, err)
	}
}

func TestModerationIsStoredOnTheAccount(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	p, d := passengerAndDriver(t, db)
	if err := db.SetMuted(ctx, p, 5_000, "mod", 1); err != nil {
		t.Fatal(err)
	}
	if err := db.SetBanned(ctx, d, 9_000, "mod", 1); err != nil {
		t.Fatal(err)
	}
	c, err := db.CharacterOf(ctx, "acc1")
	if err != nil || c.Standing.MutedUntil != 5_000 || c.Standing.BannedUntil != 0 || c.Standing.Moderator {
		t.Fatalf("standing %+v, %v", c.Standing, err)
	}
	if n, err := db.UnbanNamed(ctx, "TÀI", "mod", 2); err != nil || n != 1 {
		t.Fatalf("unban by name: %d, %v", n, err)
	}
	c2, _ := db.CharacterOf(ctx, "acc2")
	if c2.Standing.BannedUntil != 0 {
		t.Fatalf("still banned: %+v", c2.Standing)
	}
	// a report and its closing
	id, err := db.SaveReport(ctx, moderation.Report{Reporter: p, Target: d, Context: []moderation.Line{{Name: "Tài", Ch: "global", Text: "xấu", At: 3}}, At: 3})
	if err != nil {
		t.Fatal(err)
	}
	open, err := db.OpenReports(ctx, 10)
	if err != nil || len(open) != 1 || open[0].TargetName != "Tài" || open[0].Context[0].Text != "xấu" {
		t.Fatalf("open reports %+v, %v", open, err)
	}
	if ok, err := db.CloseReport(ctx, id, "mod", 4); err != nil || !ok {
		t.Fatalf("close %v %v", ok, err)
	}
	if ok, _ := db.CloseReport(ctx, id, "mod", 5); ok {
		t.Fatal("closed twice")
	}
	// moderators, from the server command
	found, err := db.CharactersNamed(ctx, "tài")
	if err != nil || len(found) != 1 {
		t.Fatalf("named: %+v %v", found, err)
	}
	if err := db.SetModerator(ctx, found[0].AccountID, true); err != nil {
		t.Fatal(err)
	}
	if mods, _ := db.Moderators(ctx); len(mods) != 1 || mods[0].Name != "Tài" {
		t.Fatalf("moderators %+v", mods)
	}
	var log int
	if err := db.sql.QueryRow(`select count(*) from moderation_log`).Scan(&log); err != nil || log != 4 {
		t.Fatalf("%d log rows, %v", log, err)
	}
}
