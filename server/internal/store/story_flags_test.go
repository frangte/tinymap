package store

import (
	"context"
	"slices"
	"testing"

	"saigon-noir/server/internal/quest"
	"saigon-noir/server/internal/quest/objective"
)

func TestFlagsAndTheOutcomeAreKeptWithTheCompletion(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	rec := quest.Record{ID: "test.fork", Version: 1, State: quest.StateDone, Outcome: "hand_over", StartedAt: 10, CompletedAt: 20,
		Objectives: map[string]*objective.Progress{"go": {N: 1}, "hand_over": {N: 1}}}
	commit := quest.Commit{Records: []quest.Record{rec}, At: 20,
		Flags:  []string{"test.fork:hand_over", "gave_it", "fork_done"},
		Grants: []quest.Grant{{Source: "quest:test.fork:outcome:hand_over", Stamps: []string{"biet_tien_viet"}}}}
	// storing the same completion again (a retry after a save that went through) changes nothing and gives nothing twice
	for try := range 2 {
		stored, err := db.SaveQuests(ctx, id, commit)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(stored.Granted); got != 1-try {
			t.Fatalf("try %d granted %v", try, stored.Granted)
		}
	}
	s, err := db.LoadQuests(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(s.Flags)
	if want := []string{"fork_done", "gave_it", "test.fork:hand_over"}; !slices.Equal(s.Flags, want) {
		t.Fatalf("flags %v, want %v", s.Flags, want)
	}
	if len(s.Records) != 1 || s.Records[0].Outcome != "hand_over" || s.Records[0].State != quest.StateDone {
		t.Fatalf("records %+v", s.Records)
	}
	// a flag keeps the time of its first setting
	if _, err := db.SaveQuests(ctx, id, quest.Commit{At: 99, Flags: []string{"gave_it", "another"}}); err != nil {
		t.Fatal(err)
	}
	var at int64
	if err := db.sql.QueryRowContext(ctx, `select set_at from story_flags where character_id = ? and flag = 'gave_it'`, id).Scan(&at); err != nil || at != 20 {
		t.Fatalf("set_at %d, %v", at, err)
	}
	if s, _ = db.LoadQuests(ctx, id); len(s.Flags) != 4 {
		t.Fatalf("flags %v", s.Flags)
	}
}

func TestAFailedSaveSetsNoFlag(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	// a record the table refuses (an unknown state) rolls the flags back with it
	bad := quest.Record{ID: "test.fork", Version: 1, State: "gone", StartedAt: 1, Objectives: map[string]*objective.Progress{}}
	if _, err := db.SaveQuests(ctx, id, quest.Commit{Records: []quest.Record{bad}, Flags: []string{"gave_it"}, At: 1}); err == nil {
		t.Fatal("saved a record with an unknown state")
	}
	if s, _ := db.LoadQuests(ctx, id); len(s.Flags) != 0 {
		t.Fatalf("flags %v after a failed save", s.Flags)
	}
}
