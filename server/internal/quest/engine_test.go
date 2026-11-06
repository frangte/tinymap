package quest

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/quest/objective"
)

func catalog(t *testing.T) *content.Catalog {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func engine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine(catalog(t))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestTheGameDataCompiles(t *testing.T) {
	c := catalog(t)
	e := engine(t)
	if len(e.tracks) != len(c.Quests)+len(c.Achievements) {
		t.Fatalf("%d tracks for %d quests and %d achievements", len(e.tracks), len(c.Quests), len(c.Achievements))
	}
	for _, kind := range []string{"reach_place", "stay_place", "talk_npc", "dialogue_signal", "emote_at", "greet", "mount", "ride_through", "ride_distance", "quest_done"} {
		if _, ok := objective.Lookup(kind); !ok {
			t.Errorf("objective type %s is not registered", kind)
		}
	}
}

func TestObjectiveParamsAreChecked(t *testing.T) {
	c := catalog(t)
	cases := []struct {
		name, typ, params string
		count             int
		want              string
	}{
		{"an unknown type", "teleport", `{}`, 1, "unknown type"},
		{"an unknown place", "reach_place", `{"place": "nowhere"}`, 1, "unknown place"},
		{"a place and a kind", "reach_place", `{"place": "cho_ben_thanh", "kind": "landmark"}`, 1, "exactly one"},
		{"more landmarks than there are", "reach_place", `{"kind": "landmark"}`, 99, "only"},
		{"a param nobody reads", "reach_place", `{"place": "cho_ben_thanh", "radius": 3}`, 1, "unknown field"},
		{"a signal the NPC never sends", "dialogue_signal", `{"npc": "co_ba", "signal": "free_money"}`, 1, "no answer"},
		{"an unknown emote", "emote_at", `{"emote": "backflip"}`, 1, "unknown emote"},
		{"a stay shorter than a time step", "stay_place", `{"place": "ho_con_rua"}`, 2, "at least 5"},
		{"more NPCs than there are", "talk_npc", `{}`, 50, "only"},
		{"an unknown quest", "quest_done", `{"quest": "no.such"}`, 1, "unknown quest"},
		{"a ride shorter than a step", "ride_distance", `{}`, 10, "at least 100"},
		{"an emote counted five times", "emote_at", `{"emote": "wave"}`, 5, "count must be 1"},
		{"getting on a bike three times", "mount", `{}`, 3, "count must be 1"},
		{"one place entered twice", "reach_place", `{"place": "cho_ben_thanh"}`, 2, "count must be 1"},
		{"one NPC talked to twice", "talk_npc", `{"npc": "co_ba"}`, 2, "count must be 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := *c
			bad.Quests = []content.Quest{{ID: "test.q", Version: 1, Flow: content.FlowSequence,
				Objectives: []content.Objective{{ID: "o", Type: tc.typ, Params: json.RawMessage(tc.params), Count: tc.count}}}}
			bad.Achievements = nil
			_, err := NewEngine(&bad)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// run is a character and its book, stepping through events like the service would (minus the store).
type run struct {
	t    *testing.T
	e    *Engine
	who  Player
	book *Book
	now  int64
	// everything granted and completed so far
	grants    []Grant
	completed []string
}

func newRun(t *testing.T, tags ...string) *run {
	r := &run{t: t, e: engine(t), who: Player{Tags: tags}, book: NewBook(Saved{}), now: 1}
	r.collect(r.e.Refresh(r.who, r.book, r.now))
	return r
}

func (r *run) collect(o Outcome) Outcome {
	r.grants = append(r.grants, o.Grants...)
	r.completed = append(r.completed, o.Completed...)
	return o
}

func (r *run) do(ev events.Event) Outcome {
	r.now++
	ev.Character = "c1"
	return r.collect(r.e.Apply(r.who, r.book, ev, r.now))
}

func (r *run) state(id string) string {
	if rec := r.book.Records[id]; rec != nil {
		return rec.State
	}
	return ""
}

func (r *run) n(id, obj string) int {
	if rec := r.book.Records[id]; rec != nil && rec.Objectives[obj] != nil {
		return rec.Objectives[obj].N
	}
	return -1
}

func signal(npc, s string) events.Event {
	return events.Event{Kind: events.Signal, NPC: npc, Signal: s}
}

func entered(place, kind string) events.Event {
	return events.Event{Kind: events.PlaceEntered, Place: place, PlaceKind: kind}
}

func stayed(place string, seconds int) events.Event {
	return events.Event{Kind: events.PlaceTime, Place: place, Seconds: seconds}
}

func TestTheTravelerQuestline(t *testing.T) {
	r := newRun(t, "traveler")
	if r.state("traveler.chao_sai_gon") != StateActive || r.state("traveler.ca_phe_sua_da") != "" {
		t.Fatalf("at the start: %v", r.book.Active())
	}
	if r.state("citizen.ca_phe_sang") != "" || r.state("driver.nhan_xe") != "" {
		t.Fatal("other archetypes' quests offered to a traveler")
	}

	// 1. the greeting lesson opens the four middle quests
	r.do(signal("co_ba", "greetings_learned"))
	if r.state("traveler.chao_sai_gon") != StateDone {
		t.Fatal("the lesson did not complete the first quest")
	}
	for _, id := range []string{"traveler.ca_phe_sua_da", "traveler.kien_truc_phap", "traveler.banh_trang_tron", "traveler.hoi_duong"} {
		if r.state(id) != StateActive {
			t.Fatalf("%s should be running after the lesson", id)
		}
	}
	if r.state("traveler.bai_kiem_tra_cuoi") != "" {
		t.Fatal("the final test opened too early")
	}

	// 2. a sequence: sitting before ordering does not count; ordering, then 20 s at the café does
	r.do(stayed("day_nha_pho_le_lai", 5))
	if r.n("traveler.ca_phe_sua_da", "sit") != 0 {
		t.Fatal("sat before ordering and it counted")
	}
	r.do(signal("chu_tu", "order_ca_phe_sua_da"))
	for range 4 {
		r.do(stayed("day_nha_pho_le_lai", 5))
	}
	if r.state("traveler.ca_phe_sua_da") != StateDone {
		t.Fatalf("coffee quest: %+v", r.book.Records["traveler.ca_phe_sua_da"])
	}

	// 3. all three buildings, any order; the same one twice counts once
	r.do(entered("nha_hat_thanh_pho", "landmark"))
	r.do(entered("nha_hat_thanh_pho", "landmark"))
	r.do(entered("nha_tho_duc_ba", "landmark"))
	if r.state("traveler.kien_truc_phap") != StateActive {
		t.Fatal("two buildings finished a quest of three")
	}
	r.do(entered("buu_dien_thanh_pho", "landmark"))

	// 4 and 5
	r.do(signal("di_nam", "buy_banh_trang_tron"))
	r.do(stayed("ho_con_rua", 15))
	r.do(signal("chu_sau", "asked_directions"))
	if r.state("traveler.bai_kiem_tra_cuoi") != StateActive {
		t.Fatalf("the final test should open after the four quests: %v", r.book.Active())
	}

	// 6. the final test: the certificate, the title (carried at once: the first one) and the achievement
	out := r.do(signal("co_ba", "final_quiz_passed"))
	if r.state("traveler.bai_kiem_tra_cuoi") != StateDone || r.state("achievement:nguoi_sai_gon_chinh_hieu") != StateDone {
		t.Fatalf("after the test: %v, completed %v", r.book.Active(), out.Completed)
	}
	if out.Title == nil || *out.Title != "nguoi_sai_gon_chinh_hieu" || r.book.Title != "nguoi_sai_gon_chinh_hieu" {
		t.Fatalf("title %v", out.Title)
	}
	caps := 0
	for _, g := range r.grants {
		if g.Capability == "local.certified" {
			caps++
		}
	}
	stamps := []string{"banh_trang_tron", "ca_phe_sua_da", "chao_sai_gon", "chung_nhan", "hoi_duong", "kien_truc_phap"}
	if caps != 1 || !slices.Equal(keys(r.book.Stamps), stamps) {
		t.Fatalf("certificate %d times, stamps %v", caps, keys(r.book.Stamps))
	}
	// the same signals again change nothing: rewards are given once
	if out := r.do(signal("co_ba", "final_quiz_passed")); !out.Empty() {
		t.Fatalf("a finished quest moved again: %+v", out)
	}
}

func TestProgressOnlyFromTheRightEvents(t *testing.T) {
	r := newRun(t, "traveler")
	for _, ev := range []events.Event{
		signal("chu_tu", "greetings_learned"),               // the right signal from the wrong NPC
		signal("co_ba", "order_ca_phe_sua_da"),              // a signal Cô Ba does not send
		{Kind: events.PlaceEntered, Place: "cho_ben_thanh"}, // nothing listens for it yet
	} {
		r.do(ev)
	}
	if r.state("traveler.chao_sai_gon") != StateActive {
		t.Fatal("the wrong events completed the lesson")
	}
}

func TestDistinctThingsCountOnce(t *testing.T) {
	r := newRun(t, "citizen")
	wave := func(partners ...string) events.Event {
		return events.Event{Kind: events.Greeted, Emote: "wave", Partners: partners}
	}
	r.do(wave("minh"))
	r.do(wave("minh"))
	if r.n("citizen.chao_hang_xom", "wave") != 1 {
		t.Fatalf("the same friend twice: %d", r.n("citizen.chao_hang_xom", "wave"))
	}
	r.do(wave("minh", "hoa"))
	if r.state("citizen.chao_hang_xom") != StateDone {
		t.Fatal("two different friends should finish it")
	}
	if r.book.Title != "hang_xom_than_thien" {
		t.Fatalf("the first title earned is carried: %q", r.book.Title)
	}
	// a later title is earned but not carried
	r.do(events.Event{Kind: events.Emote, Emote: "exercise", Place: "cong_vien_23_9"})
	if !r.book.Titles["nguoi_day_som"] || r.book.Title != "hang_xom_than_thien" {
		t.Fatalf("titles %v, carried %q", r.book.Titles, r.book.Title)
	}
	// landmarks for the achievement: distinct ones only
	for range 3 {
		r.do(entered("cho_ben_thanh", "landmark"))
	}
	r.do(entered("duong_le_loi", "street"))
	if n := r.n("achievement:kham_pha_quan_1", ""); n != 1 {
		t.Fatalf("landmarks counted %d", n)
	}
}

func TestADriverLearnsTheStreets(t *testing.T) {
	r := newRun(t, "driver")
	ride := func(place string, riding bool) events.Event {
		return events.Event{Kind: events.PlaceEntered, Place: place, PlaceKind: "street", Riding: riding}
	}
	if r.state("driver.thuoc_duong") != "" {
		t.Fatal("the route quest needs the bike first")
	}
	r.do(events.Event{Kind: events.Mounted, Vehicle: "xe_so_110"})
	r.do(ride("duong_le_loi", false)) // on foot: no
	r.do(ride("duong_dong_khoi", true))
	if r.n("driver.thuoc_duong", "le_loi") != 0 || r.n("driver.thuoc_duong", "dong_khoi") != 0 {
		t.Fatal("walking, or riding out of order, counted")
	}
	r.do(ride("duong_le_loi", true))
	r.do(events.Event{Kind: events.PlaceTime, Place: "duong_dong_khoi", Riding: true, Seconds: 5})
	r.do(ride("duong_ham_nghi", true))
	if r.state("driver.thuoc_duong") != StateDone {
		t.Fatalf("route quest %+v", r.book.Records["driver.thuoc_duong"])
	}
	for range 50 {
		r.do(events.Event{Kind: events.Rode, Metres: 100})
	}
	if r.state("achievement:tay_lai_lua") != StateDone {
		t.Fatal("5 km ridden should give the achievement")
	}
}
