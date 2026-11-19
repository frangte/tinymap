package store

import (
	"context"
	"slices"
	"testing"

	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/quest"
)

func TestContactsAreKeptOnceInOrder(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)

	if got, err := db.Contacts(ctx, id); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("a new character has %v, %v: want an empty list", got, err)
	}
	added, err := db.AddContacts(ctx, id, []string{"ma", "chu_sau"}, 10)
	if err != nil || !slices.Equal(added, []string{"ma", "chu_sau"}) {
		t.Fatalf("added %v, %v", added, err)
	}
	// a number it has is skipped, and not reported as new
	added, err = db.AddContacts(ctx, id, []string{"chu_sau", "co_ba"}, 20)
	if err != nil || !slices.Equal(added, []string{"co_ba"}) {
		t.Fatalf("added again %v, %v", added, err)
	}
	if got, _ := db.Contacts(ctx, id); !slices.Equal(got, []string{"ma", "chu_sau", "co_ba"}) {
		t.Fatalf("contacts %v", got)
	}
	for contact, want := range map[string]bool{"ma": true, "co_ba": true, "chu_tu": false} {
		if has, err := db.HasContact(ctx, id, contact); err != nil || has != want {
			t.Errorf("HasContact(%s) = %v, %v, want %v", contact, has, err, want)
		}
	}
	if has, _ := db.HasContact(ctx, "stranger", "ma"); has {
		t.Fatal("a stranger has Má's number")
	}
}

func textKeysOf(texts []protocol.PhoneText) []string {
	var keys []string
	for _, t := range texts {
		keys = append(keys, t.ID)
	}
	return keys
}

// A quest's texts are written in the transaction of its commit: kept, with the sender's number, and a retried
// commit changes nothing.
func TestTextsAreKeptWithTheirCommitAndNeverTwice(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	c := quest.Commit{At: 100, Texts: []quest.Message{
		{Key: "q.one//start/0", From: "chu_tu", Say: protocol.Text{VI: "Mai ghé nha", EN: "Come tomorrow"}},
		{Key: "q.one//start/1", From: "ma", Say: protocol.Text{VI: "Ăn cơm chưa con", EN: "Have you eaten"}},
	}}
	stored, err := db.SaveQuests(ctx, id, c)
	if err != nil || len(stored.Texts) != 2 || !slices.Equal(stored.Contacts, []string{"chu_tu", "ma"}) {
		t.Fatalf("stored %+v, %v", stored, err)
	}
	// the retry: a save that failed after the commit went through is tried again
	again, err := db.SaveQuests(ctx, id, c)
	if err != nil || len(again.Texts) != 0 || len(again.Contacts) != 0 {
		t.Fatalf("the retry stored %+v, %v: want nothing new", again, err)
	}
	texts, err := db.Texts(ctx, id)
	if err != nil || !slices.Equal(textKeysOf(texts), []string{"q.one//start/0", "q.one//start/1"}) {
		t.Fatalf("texts %+v, %v", texts, err)
	}
	if texts[0].From != "chu_tu" || texts[0].Say.VI != "Mai ghé nha" || texts[0].Say.EN != "Come tomorrow" || texts[0].At != 100 || texts[0].Read {
		t.Fatalf("text %+v", texts[0])
	}
	if got, _ := db.Contacts(ctx, id); !slices.Equal(got, []string{"chu_tu", "ma"}) {
		t.Fatalf("senders' numbers: %v", got)
	}
	// reading marks them all, and later ones arrive unread
	if err := db.ReadTexts(ctx, id, 200); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SaveQuests(ctx, id, quest.Commit{At: 300, Texts: []quest.Message{{Key: "q.two//done/0", From: "ma", Say: protocol.Text{VI: "Giỏi", EN: "Well done"}}}}); err != nil {
		t.Fatal(err)
	}
	texts, _ = db.Texts(ctx, id)
	if len(texts) != 3 || !texts[0].Read || !texts[1].Read || texts[2].Read {
		t.Fatalf("read flags %+v", texts)
	}
}

// A texting NPC that was never met has its number saved only once; a number given by talking is not added again.
func TestATextFromSomeoneYouHaveNumberOfAddsNothing(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	ctx := context.Background()
	id := character(t, db)
	if _, err := db.AddContacts(ctx, id, []string{"chu_tu"}, 1); err != nil {
		t.Fatal(err)
	}
	stored, err := db.SaveQuests(ctx, id, quest.Commit{At: 2, Texts: []quest.Message{{Key: "k", From: "chu_tu", Say: protocol.Text{VI: "a", EN: "b"}}}})
	if err != nil || len(stored.Texts) != 1 || len(stored.Contacts) != 0 {
		t.Fatalf("stored %+v, %v", stored, err)
	}
}
