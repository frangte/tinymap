package quest

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"testing"
)

// grantKeys lists the grants as "source xu titles stamps capability", sorted, to compare two runs.
func grantKeys(gs []Grant) []string {
	var out []string
	for _, g := range gs {
		out = append(out, fmt.Sprintf("%s|%d|%v|%v|%s", g.Source, g.Xu, g.Titles, g.Stamps, g.Capability))
	}
	sort.Strings(out)
	return out
}

func messageKeys(ts []Message) []string {
	var out []string
	for _, m := range ts {
		out = append(out, m.Key+"|"+m.From)
	}
	sort.Strings(out)
	return out
}

// Completing a quest names the same state playing it leaves: the record and its outcome, the flags, the rewards,
// the texts, and the quests that open.
func TestCompleteMatchesPlaying(t *testing.T) {
	for _, outcome := range []string{"keep", "hand_over", "ask"} {
		t.Run(outcome, func(t *testing.T) {
			played := forkRun(t)
			played.do(entered("cho_ben_thanh", "market"))
			playedOut := played.do(signal("test_npc", outcome))

			made := forkRun(t)
			madeOut, err := made.e.Complete(made.who, made.book, "c1", "test.fork", outcome, made.now+1)
			if err != nil {
				t.Fatal(err)
			}

			pr, mr := played.book.Records["test.fork"], made.book.Records["test.fork"]
			if pr.State != mr.State || pr.Outcome != mr.Outcome || pr.Version != mr.Version {
				t.Fatalf("record played %+v, completed %+v", pr, mr)
			}
			if pr.Objectives["go"].N != mr.Objectives["go"].N {
				t.Fatalf("objective progress played %+v, completed %+v", pr.Objectives["go"], mr.Objectives["go"])
			}
			if !maps.Equal(played.book.Flags, made.book.Flags) {
				t.Fatalf("flags played %v, completed %v", played.flags(), made.flags())
			}
			if !maps.Equal(played.book.Stamps, made.book.Stamps) || !maps.Equal(played.book.Titles, made.book.Titles) || played.book.Title != made.book.Title {
				t.Fatalf("earned differs: played %v %v, completed %v %v", played.book.Stamps, played.book.Titles, made.book.Stamps, made.book.Titles)
			}
			if !slices.Equal(grantKeys(playedOut.Grants), grantKeys(madeOut.Grants)) {
				t.Fatalf("grants played %v, completed %v", grantKeys(playedOut.Grants), grantKeys(madeOut.Grants))
			}
			if !slices.Equal(messageKeys(playedOut.Texts), messageKeys(madeOut.Texts)) {
				t.Fatalf("texts played %v, completed %v", messageKeys(playedOut.Texts), messageKeys(madeOut.Texts))
			}
			if !maps.Equal(played.book.Active(), made.book.Active()) {
				t.Fatalf("open quests played %v, completed %v", played.book.Active(), made.book.Active())
			}
			if !slices.Equal(playedOut.Completed, madeOut.Completed) {
				t.Fatalf("completed played %v, completed %v", playedOut.Completed, madeOut.Completed)
			}
		})
	}
}

func TestCompleteWithoutObjectives(t *testing.T) {
	played := forkRun(t)
	played.do(entered("ho_con_rua", "lake"))
	made := forkRun(t)
	if _, err := made.e.Complete(made.who, made.book, "c1", "test.pure_choice", "left", made.now+1); err != nil {
		t.Fatal(err)
	}
	if pr, mr := played.book.Records["test.pure_choice"], made.book.Records["test.pure_choice"]; pr.State != mr.State || pr.Outcome != mr.Outcome {
		t.Fatalf("played %+v, completed %+v", pr, mr)
	}
}

func TestCompleteRefuses(t *testing.T) {
	cases := []struct {
		name, quest, outcome string
		notOpen              bool
	}{
		{"a quest that is not open yet", "test.after_hand_over", "", true},
		{"an outcome the quest does not have", "test.fork", "burn_it", false},
		{"no outcome for a quest that has them", "test.fork", "", false},
		{"an unknown quest", "test.nothing", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := forkRun(t)
			before := len(r.book.Records)
			_, err := r.e.Complete(r.who, r.book, "c1", c.quest, c.outcome, r.now+1)
			if err == nil {
				t.Fatal("want an error")
			}
			if c.notOpen != errors.Is(err, ErrNotOpen) {
				t.Fatalf("error %v, not open %v", err, c.notOpen)
			}
			if r.state("test.fork") == StateDone || len(r.book.Records) != before {
				t.Fatalf("a refused completion changed the book: %v", r.book.Records)
			}
		})
	}
}

func TestCompleteOpensWhatFollows(t *testing.T) {
	r := forkRun(t)
	if _, err := r.e.Complete(r.who, r.book, "c1", "test.fork", "hand_over", r.now+1); err != nil {
		t.Fatal(err)
	}
	if r.state("test.after_hand_over") != StateActive {
		t.Fatal("the quest that reads the flag should be open")
	}
	if _, err := r.e.Complete(r.who, r.book, "c1", "test.fork", "keep", r.now+2); err == nil {
		t.Fatal("a quest done cannot be completed again")
	}
}
