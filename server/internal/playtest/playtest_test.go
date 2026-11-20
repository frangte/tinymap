package playtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/quest"
	"saigon-noir/server/internal/store"
)

// counting is the identity service that counts the accounts it makes.
type counting struct {
	*identity.Service
	guests int
}

func (c *counting) NewGuest(ctx context.Context) (string, error) {
	c.guests++
	return c.Service.NewGuest(ctx)
}

type rig struct {
	t        *testing.T
	maker    *Maker
	db       *store.DB
	accounts *counting
	catalog  *content.Catalog
}

// newRig is the game data plus the branching-story fixture of the quest package (a quest with outcomes), a
// real database in a temporary folder, and a maker over them.
func newRig(t *testing.T) *rig {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "quest", "testdata", "branching_story.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Stamps    []content.Stamp             `json:"stamps"`
		Quests    []content.Quest             `json:"quests"`
		NPC       content.NPC                 `json:"npc"`
		Dialogues map[string]content.Dialogue `json:"dialogues"`
	}
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&fx); err != nil {
		t.Fatal(err)
	}
	c.Stamps = append(c.Stamps, fx.Stamps...)
	c.Quests = append(slices.Clone(c.Quests), fx.Quests...)
	c.NPCs = append(slices.Clone(c.NPCs), fx.NPC)
	for id, d := range fx.Dialogues {
		c.Dialogues[id] = d
	}
	engine, err := quest.NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), store.Options{Path: filepath.Join(t.TempDir(), "game.db")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	acc := &counting{Service: identity.New(db)}
	return &rig{t: t, db: db, accounts: acc, catalog: c,
		maker: &Maker{Catalog: c, Engine: engine, Accounts: acc, Store: db, Now: time.Now}}
}

func (r *rig) make(req Request) Result {
	r.t.Helper()
	res, err := r.maker.Make(context.Background(), req)
	if err != nil {
		r.t.Fatal(err)
	}
	return res
}

func (r *rig) saved(res Result) *quest.Book {
	r.t.Helper()
	s, err := r.db.LoadQuests(context.Background(), res.Character)
	if err != nil {
		r.t.Fatal(err)
	}
	return quest.NewBook(s)
}

func TestMakeChapterThree(t *testing.T) {
	r := newRig(t)
	res := r.make(Request{
		Archetype: "newcomer",
		Done:      []DoneQuest{{Quest: "story.chu_sau_ke_chuyen"}, {Quest: "story.goi_ve_cho_ma"}, {Quest: "story.gap_chu_sau"}},
		Items:     []string{"ifone_x"},
		Xu:        1000,
		Stand:     "thay_muoi",
	})
	book := r.saved(res)
	for _, id := range []string{"story.goi_ve_cho_ma", "story.gap_chu_sau", "story.chu_sau_ke_chuyen"} {
		if rec := book.Records[id]; rec == nil || rec.State != quest.StateDone || rec.CompletedAt == 0 {
			t.Errorf("%s: %+v", id, rec)
		}
	}
	// the quest the story goes on with is running, and the character holds what its rewards gave
	if rec := book.Records["driver.thi_bang_a1"]; rec == nil || rec.State != quest.StateActive {
		t.Fatalf("driver.thi_bang_a1: %+v", rec)
	}
	caps, err := r.db.EarnedCapabilities(context.Background(), res.Character)
	if err != nil || !slices.Contains(caps, "chapter.crab") || !slices.Contains(caps, "phone.smart") {
		t.Fatalf("capabilities %v, %v", caps, err)
	}
	owned, _ := r.db.Owned(context.Background(), res.Character)
	if !slices.Equal(owned, []string{"ifone_x"}) {
		t.Fatalf("owned %v", owned)
	}
	// the wallet: the starting xu, the quest's reward and the extra
	want := r.catalog.Ridehail.StartBalance + 100000 + 1000
	if res.Balance != want {
		t.Fatalf("balance %d, want %d", res.Balance, want)
	}
	if !slices.Equal(res.Active, sortedActive(book)) || !slices.Contains(res.Active, "driver.thi_bang_a1") {
		t.Fatalf("active %v", res.Active)
	}
	// marked as a test, the account is new, and the character stands beside Thầy Mười, facing him
	if !strings.HasPrefix(res.Name, NamePrefix) || r.accounts.guests != 1 {
		t.Fatalf("name %q, %d accounts", res.Name, r.accounts.guests)
	}
	npc, _ := r.catalog.NPCByID("thay_muoi")
	nx, nz, _ := npc.ThreePose()
	if res.Pose == nil || math.Hypot(res.Pose.X-nx, res.Pose.Z-nz) > 2 || math.Hypot(res.Pose.X-nx, res.Pose.Z-nz) < 0.5 {
		t.Fatalf("pose %+v, NPC at (%v, %v)", res.Pose, nx, nz)
	}
	got, err := r.accounts.Arrive(context.Background(), res.Token, identity.Profile{})
	if err != nil || got.ID != res.Character || got.Archetype != "newcomer" || got.Pose == nil || *got.Pose != *res.Pose {
		t.Fatalf("logging in with the token: %+v, %v", got, err)
	}
}

func sortedActive(b *quest.Book) []string {
	out := []string{}
	for id := range b.Active() {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func TestMakeKeepsTheOutcomeAndItsFlags(t *testing.T) {
	r := newRig(t)
	res := r.make(Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "test.fork", Outcome: "hand_over"}}, Flags: []string{"kept_it"}})
	book := r.saved(res)
	rec := book.Records["test.fork"]
	if rec == nil || rec.State != quest.StateDone || rec.Outcome != "hand_over" {
		t.Fatalf("record %+v", rec)
	}
	for _, f := range []string{"test.fork:hand_over", "gave_it", "good_name", "fork_done", "kept_it"} {
		if !book.Flags[f] {
			t.Errorf("flag %s missing from %v", f, res.Flags)
		}
	}
	if !book.Stamps["test_stamp"] {
		t.Error("the outcome's stamp was not earned")
	}
	// the quest's reward (1000) and the outcome's (5000) on top of the start
	if want := r.catalog.Ridehail.StartBalance + 6000; res.Balance != want {
		t.Fatalf("balance %d, want %d", res.Balance, want)
	}
	if book.Records["test.after_hand_over"] == nil {
		t.Error("the quest that reads the outcome's flag did not open")
	}
}

func TestMakeRefuses(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want string
	}{
		{"unknown archetype", Request{Archetype: "king"}, "archetype"},
		{"no archetype", Request{}, "archetype"},
		{"unknown quest", Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "no.such"}}}, "unknown quest"},
		{"outcome of another quest", Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "test.fork", Outcome: "left"}}}, "no outcome"},
		{"no outcome for a quest that has them", Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "test.fork"}}}, "no outcome"},
		{"an outcome for a quest that has none", Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "story.goi_ve_cho_ma", Outcome: "x"}}}, "no outcomes"},
		{"skips a required quest", Request{Archetype: "newcomer", Done: []DoneQuest{{Quest: "story.gap_chu_sau"}}}, "needs"},
		{"the same quest twice", Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "test.pure_choice", Outcome: "left"}, {Quest: "test.pure_choice", Outcome: "right"}}}, "twice"},
		{"a daily", Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "daily.sang_o_pho"}}}, "quest"},
		{"unknown item", Request{Archetype: "citizen", Items: []string{"gold_bar"}}, "item"},
		{"unknown flag", Request{Archetype: "citizen", Flags: []string{"made_up"}}, "flag"},
		{"unknown place", Request{Archetype: "citizen", Stand: "the_moon"}, "neither a place nor an NPC"},
		{"negative money", Request{Archetype: "citizen", Xu: -5}, "xu"},
		{"too much money", Request{Archetype: "citizen", Xu: xuMax + 1}, "xu"},
		{"a quest the origin is not offered", Request{Archetype: "traveler", Done: []DoneQuest{{Quest: "story.goi_ve_cho_ma"}}}, "capability"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			_, err := r.maker.Make(context.Background(), c.req)
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want invalid mentioning %q", err, c.want)
			}
		})
	}
}

func TestRefusedRequestsMakeNoAccount(t *testing.T) {
	r := newRig(t)
	for _, req := range []Request{{Archetype: "king"}, {Archetype: "citizen", Xu: -1}, {Archetype: "citizen", Done: []DoneQuest{{Quest: "test.fork"}}}} {
		if _, err := r.maker.Make(context.Background(), req); err == nil {
			t.Fatal("want a refusal")
		}
	}
	if r.accounts.guests != 0 {
		t.Fatalf("%d accounts made for refused requests", r.accounts.guests)
	}
}

func TestEveryTestCharacterIsNew(t *testing.T) {
	r := newRig(t)
	a := r.make(Request{Archetype: "citizen"})
	b := r.make(Request{Archetype: "citizen"})
	if a.Token == b.Token || a.Character == b.Character {
		t.Fatalf("two requests shared an account: %+v %+v", a, b)
	}
}

// A quest may wait for a capability that a quest listed after it gives: the order asked for does not matter.
func TestMakeFindsTheOrderQuestsOpenIn(t *testing.T) {
	r := newRig(t)
	res := r.make(Request{
		Archetype: "citizen",
		Done:      []DoneQuest{{Quest: "driver.dang_ky_crab"}, {Quest: "driver.thi_bang_a1"}, {Quest: "citizen.chu_sau_ke_chuyen"}},
		Items:     []string{"ifone_x", "xe_so_110"},
	})
	book := r.saved(res)
	for _, id := range []string{"driver.dang_ky_crab", "driver.thi_bang_a1", "citizen.chu_sau_ke_chuyen"} {
		if rec := book.Records[id]; rec == nil || rec.State != quest.StateDone {
			t.Errorf("%s: %+v", id, rec)
		}
	}
}

func TestMakeSaysWhatIsMissing(t *testing.T) {
	r := newRig(t)
	_, err := r.maker.Make(context.Background(), Request{Archetype: "citizen", Done: []DoneQuest{{Quest: "driver.thi_bang_a1"}}})
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "needs capability chapter.crab") {
		t.Fatalf("error %v", err)
	}
}

func TestTestNameFitsTheGame(t *testing.T) {
	for _, label := range []string{"", "   ", "một cái tên rất là dài dòng văn tự", "a\x00b\n"} {
		got := testName(label)
		if n := len([]rune(got)); n > nameMax || !strings.HasPrefix(got, NamePrefix) || strings.ContainsAny(got, "\x00\n") {
			t.Errorf("testName(%q) = %q", label, got)
		}
	}
}
