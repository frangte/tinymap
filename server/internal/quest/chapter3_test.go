package quest

import (
	"encoding/json"
	"slices"
	"testing"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/quest/objective"
)

// ---- chapter 3: becoming a Crab driver (docs/design/13-driver.md)

// gain gives the character a capability, as a purchase or a quest reward would, and lets the quests open.
func (r *run) gain(caps ...string) {
	r.t.Helper()
	for _, k := range caps {
		r.who.Caps[k] = struct{}{}
	}
	r.now++
	r.collect(r.e.Refresh(r.who, r.book, r.now))
}

func (r *run) granted(capability string) bool {
	for _, g := range r.grants {
		if g.Capability == capability {
			return true
		}
	}
	return false
}

func chapter3(r *run) []string {
	var out []string
	for _, id := range []string{"driver.thi_bang_a1", "driver.mua_xe", "driver.dang_ky_crab", "driver.nhan_xe", "driver.thuoc_duong", "driver.chuyen_dau_tien"} {
		if r.state(id) == StateActive {
			out = append(out, id)
		}
	}
	return out
}

// Nobody is a driver at the start, and nobody is offered the chapter before hearing it is the way: the quests wait
// for chapter.crab, which Chú Sáu's story (or Tony's suggestion) gives.
func TestChapter3WaitsForTheStory(t *testing.T) {
	for _, tags := range [][]string{{"citizen", "newcomer"}, {"citizen", "native"}, {"traveler"}} {
		r := newRun(t, tags...)
		if got := chapter3(r); len(got) != 0 {
			t.Errorf("%v: chapter 3 quests offered at the start: %v", tags, got)
		}
	}
}

func TestANativeHearsChuSauAndIsOfferedTheLicence(t *testing.T) {
	r := newRun(t, "citizen", "native")
	if r.state("citizen.chu_sau_ke_chuyen") != StateActive {
		t.Fatalf("a native is offered Chú Sáu's story at once: %v", r.book.Active())
	}
	r.do(signal("chu_sau", "chu_sau_khuyen_crab"))
	if r.state("citizen.chu_sau_ke_chuyen") != StateDone || !r.granted("chapter.crab") {
		t.Fatalf("the story: %v, grants %+v", r.state("citizen.chu_sau_ke_chuyen"), r.grants)
	}
	r.gain("chapter.crab")
	if got := chapter3(r); !slices.Equal(got, []string{"driver.thi_bang_a1"}) {
		t.Fatalf("after the story only the licence is open: %v", got)
	}
}

func TestANewcomerHearsTheStoryAfterMeetingChuSau(t *testing.T) {
	r := newRun(t, "citizen", "newcomer")
	if r.state("story.chu_sau_ke_chuyen") != "" {
		t.Fatal("the story must follow story.gap_chu_sau")
	}
	r.do(signal("ma", "ma_gui_tien"))
	r.do(talked("chu_sau"))
	if r.state("story.gap_chu_sau") != StateDone || r.state("story.chu_sau_ke_chuyen") != StateActive {
		t.Fatalf("after meeting him: gap %q, story %q", r.state("story.gap_chu_sau"), r.state("story.chu_sau_ke_chuyen"))
	}
	r.do(signal("chu_sau", "chu_sau_khuyen_crab"))
	if !r.granted("chapter.crab") {
		t.Fatalf("grants %+v", r.grants)
	}
}

// A foreigner is not pushed: Tony's suggestion is an optional quest after the hostel, and it opens the same chapter.
func TestATravelerReachesTheChapterThroughTony(t *testing.T) {
	r := newRun(t, "traveler")
	if r.state("traveler.tony_goi_y") != "" {
		t.Fatal("Tony's suggestion must wait for the hostel")
	}
	r.do(signal("anh_tony", "phrase_xin_chao"))
	r.do(signal("anh_tony", "phrase_cam_on"))
	if r.state("traveler.tony_goi_y") != StateActive {
		t.Fatalf("after the hostel: %v", r.book.Active())
	}
	r.do(signal("anh_tony", "tony_goi_y_crab"))
	if !r.granted("chapter.crab") {
		t.Fatalf("grants %+v", r.grants)
	}
	r.gain("chapter.crab")
	if got := chapter3(r); !slices.Equal(got, []string{"driver.thi_bang_a1"}) {
		t.Fatalf("chapter 3 for a traveler: %v", got)
	}
}

// The chapter follows what the character has, not who it is: licence, then bike, then the Crab office, then
// the bike quests and the first trip.
func TestTheDriverQuestsOpenByCapabilities(t *testing.T) {
	r := newRun(t, "citizen", "native")
	r.gain("chapter.crab")
	r.do(signal("thay_muoi", "ly_thuyet_dau"))
	if r.state("driver.thi_bang_a1") != StateActive {
		t.Fatalf("the practical is next: %v", r.book.Records["driver.thi_bang_a1"])
	}
	r.do(events.Event{Kind: events.TestPassed, Test: content.TestA1})
	if r.state("driver.thi_bang_a1") != StateDone || !r.granted(content.CapRide) {
		t.Fatalf("the licence: %v, grants %+v", r.state("driver.thi_bang_a1"), r.grants)
	}
	r.gain(content.CapRide)
	if got := chapter3(r); !slices.Equal(got, []string{"driver.mua_xe"}) {
		t.Fatalf("with the licence the bike is next: %v", got)
	}
	r.do(bought("xe_so_110", "bike"))
	r.gain("vehicle.motorbike.own")
	if got := chapter3(r); !slices.Equal(got, []string{"driver.dang_ky_crab", "driver.nhan_xe"}) {
		t.Fatalf("with a bike: %v", got)
	}
	r.do(bought("dong_phuc_crab", "uniform"))
	r.gain("uniform.crab", "driver.app", "ridehail.drive")
	r.do(events.Event{Kind: events.Mounted, Vehicle: "xe_so_110"})
	if got := chapter3(r); !slices.Equal(got, []string{"driver.thuoc_duong", "driver.chuyen_dau_tien"}) {
		t.Fatalf("as a driver: %v", got)
	}
	r.do(events.Event{Kind: events.TripCompleted, Role: "rider", Partners: []string{"lan"}})
	if r.state("driver.chuyen_dau_tien") != StateDone {
		t.Fatal("the first trip did not count")
	}
}

func TestTheLicenceQuestIsNotOfferedToSomeoneWhoHasOne(t *testing.T) {
	r := newRunWith(t, driverCaps(), "citizen", "native")
	r.gain("chapter.crab")
	for _, id := range []string{"driver.thi_bang_a1", "driver.mua_xe", "driver.dang_ky_crab"} {
		if r.state(id) != "" {
			t.Errorf("%s is offered to a driver who has it all", id)
		}
	}
}

func TestADailyForDriversOpensWithTheUniformNotATag(t *testing.T) {
	plain := newRun(t, "citizen", "native")
	rides := newRunWith(t, driverCaps(), "citizen", "native")
	for _, id := range []string{"daily.chay_mot_vong", "daily.mot_cuoc_xe", "daily.duong_ven_bien"} {
		if plain.state(id) != "" {
			t.Errorf("%s offered without driving", id)
		}
	}
	// one daily of the pool is on each day: a driver gets one, and it is a driver's
	var active []string
	for id, rec := range rides.book.Records {
		if rec.State == StateActive && slices.Contains([]string{"daily.chay_mot_vong", "daily.mot_cuoc_xe", "daily.duong_ven_bien"}, id) {
			active = append(active, id)
		}
	}
	if len(active) != 1 {
		t.Fatalf("driver dailies on: %v", active)
	}
}

// The end of the chapter: a newcomer tells Má, a native tells Chú Sáu, who texts the newcomer too.
func TestTheChapterEndsWithMaOrChuSau(t *testing.T) {
	tripped := func(r *run) {
		r.gain("vehicle.motorbike.ride", "vehicle.motorbike.own", "uniform.crab", "driver.app", "ridehail.drive")
		r.do(events.Event{Kind: events.Mounted, Vehicle: "xe_so_110"})
		r.do(events.Event{Kind: events.TripCompleted, Role: "rider", Partners: []string{"lan"}})
	}
	n := newRun(t, "citizen", "newcomer")
	tripped(n)
	if n.state("story.khoe_voi_ma") != StateActive || n.state("citizen.khoe_voi_chu_sau") != "" {
		t.Fatalf("a newcomer calls Má: %v", n.book.Active())
	}
	out := n.do(signal("ma", "ma_khoe_crab"))
	if n.state("story.khoe_voi_ma") != StateDone {
		t.Fatalf("the call home did not end the chapter: %+v", n.book.Records["story.khoe_voi_ma"])
	}
	_ = out
	c := newRun(t, "citizen", "native")
	tripped(c)
	if c.state("citizen.khoe_voi_chu_sau") != StateActive || c.state("story.khoe_voi_ma") != "" {
		t.Fatalf("a native tells Chú Sáu: %v", c.book.Active())
	}
	c.do(signal("chu_sau", "khoe_crab"))
	if c.state("citizen.khoe_voi_chu_sau") != StateDone {
		t.Fatal("telling Chú Sáu did not end the chapter")
	}
}

// pay_fee and pass_test are objectives of their own, checked against the game data when compiled.
func TestFeesAndTestsAreObjectives(t *testing.T) {
	cat := catalog(t)
	for _, c := range []struct {
		kind, params string
		ok           bool
	}{
		{"pay_fee", `{"fee": "thi_a1"}`, true},
		{"pay_fee", `{"fee": "nope"}`, false},
		{"pass_test", `{"test": "a1"}`, true},
		{"pass_test", `{"test": "b2"}`, false},
	} {
		typ, found := objective.Lookup(c.kind)
		if !found {
			t.Fatalf("objective type %q is not registered", c.kind)
		}
		if _, err := typ.Compile(json.RawMessage(c.params), 1, catalogRefs{cat}); (err == nil) != c.ok {
			t.Errorf("%s %s: err %v, want ok %v", c.kind, c.params, err, c.ok)
		}
	}
	// a fee is one payment: a count would only reward paying again
	typ, _ := objective.Lookup("pay_fee")
	if _, err := typ.Compile(json.RawMessage(`{"fee": "thi_a1"}`), 2, catalogRefs{cat}); err == nil {
		t.Error("a count of 2 for a fee compiled")
	}
	// and the event counts once
	m, _ := typ.Compile(json.RawMessage(`{"fee": "thi_a1"}`), 1, catalogRefs{cat})
	p := &objective.Progress{}
	if m.Advance(events.Event{Kind: events.FeePaid, Fee: "thi_lai_a1"}, p, 1) || p.N != 0 {
		t.Error("another fee counted")
	}
	if !m.Advance(events.Event{Kind: events.FeePaid, Fee: "thi_a1"}, p, 1) || p.N != 1 {
		t.Error("the fee did not count")
	}
}
