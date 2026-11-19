package quest

import (
	"slices"
	"testing"

	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/quest/objective"
)

// xuGiven is what the grants so far add up to.
func (r *run) xuGiven() (total int64) {
	for _, g := range r.grants {
		total += g.Xu
	}
	return total
}

// The newcomer's chapter 0: the slip, then Bà Chín. The quest to call Má belongs to the phone's own tests.
func TestTheNewcomersChapterZero(t *testing.T) {
	r := newRunWith(t, nil, "citizen", "newcomer")
	const slip, room = "newcomer.to_giay_dia_chi", "newcomer.nha_tro_ba_chin"
	if r.state(slip) != StateActive || r.state(room) != "" {
		t.Fatalf("at the start: %v", r.book.Active())
	}
	// Bà Chín's answer means nothing before the walk is done: the room quest has not started
	r.do(signal("ba_chin", "nhan_phong"))
	if r.state(room) != "" {
		t.Fatal("took the room before the walk")
	}
	r.do(entered("ben_xe_cho_lon", "landmark"))
	if r.state(slip) != StateActive {
		t.Fatal("the bus station finished the walk")
	}
	r.do(entered("day_nha_pho_le_lai", "landmark"))
	if r.state(slip) != StateDone || r.state(room) != StateActive {
		t.Fatalf("after the walk: %v", r.book.Active())
	}
	// someone else's signal does not take the room
	r.do(signal("anh_tony", "nhan_phong"))
	if r.state(room) != StateActive {
		t.Fatal("the wrong NPC took the room")
	}
	r.do(signal("ba_chin", "nhan_phong"))
	if r.state(room) != StateDone {
		t.Fatalf("room quest: %+v", r.book.Records[room])
	}
	if got := r.xuGiven(); got != 150_000 {
		t.Fatalf("the two quests paid %d, want 150000", got)
	}
	// nothing of the foreigner's, and the phone quest stays everyone's
	if r.state("traveler.hostel_anh_tony") != "" || r.state(phoneQuest) != StateActive {
		t.Fatalf("running: %v", r.book.Active())
	}
}

// The foreigner's chapter 0, step by step; each quest opens only after the one before.
func TestTheForeignersChapterZero(t *testing.T) {
	r := newRun(t, "traveler")
	chain := []string{"traveler.hostel_anh_tony", "traveler.tien_viet", "traveler.tra_gia_cho", "traveler.qua_duong", "traveler.an_nhu_nguoi_sai_gon"}
	if r.state(chain[0]) != StateActive {
		t.Fatalf("at the start: %v", r.book.Active())
	}
	for _, id := range chain[1:] {
		if r.state(id) != "" {
			t.Fatalf("%s offered too early", id)
		}
	}
	if r.state("newcomer.to_giay_dia_chi") != "" {
		t.Fatal("the newcomer's quest offered to a foreigner")
	}
	// the Crab ride waits for the money lesson, then opens with it
	if r.state("traveler.di_crab") != "" {
		t.Fatal("the Crab ride offered before the money lesson")
	}

	// 1. Tony: both phrases, any order
	r.do(signal("anh_tony", "phrase_cam_on"))
	if r.state(chain[0]) != StateActive {
		t.Fatal("one phrase finished the quest")
	}
	r.do(signal("anh_tony", "phrase_xin_chao"))
	if r.state(chain[0]) != StateDone || r.state(chain[1]) != StateActive {
		t.Fatalf("after Tony: %v", r.book.Active())
	}

	// 2. Cô Kim's quiz: the second answer counts only after the first
	r.do(signal("co_kim", "biet_tien_viet"))
	if r.state(chain[1]) != StateActive {
		t.Fatal("the second answer counted before the first")
	}
	r.do(signal("co_kim", "quiz_tien_20k"))
	r.do(signal("co_kim", "biet_tien_viet"))
	if r.state(chain[1]) != StateDone {
		t.Fatalf("quiz: %+v", r.book.Records[chain[1]])
	}
	if !slices.Equal(known(r.book.Stamps, r.e.stamps), []string{"biet_tien_viet"}) {
		t.Fatalf("stamps %v", known(r.book.Stamps, r.e.stamps))
	}
	if r.state(chain[2]) != StateActive || r.state("traveler.di_crab") != StateActive {
		t.Fatalf("after the quiz: %v", r.book.Active())
	}

	// 3. Cô Ba: ask, haggle, thank, in that order
	r.do(signal("co_ba", "thanked_after_bargain"))
	r.do(signal("co_ba", "phrase_mac_qua"))
	if r.state(chain[2]) != StateActive {
		t.Fatal("bargaining counted out of order")
	}
	r.do(signal("co_ba", "asked_price_hat"))
	r.do(signal("co_ba", "phrase_mac_qua"))
	r.do(signal("co_ba", "thanked_after_bargain"))
	if r.state(chain[2]) != StateDone {
		t.Fatalf("bargaining: %+v", r.book.Records[chain[2]])
	}

	// 4. Chú Sáu
	r.do(signal("chu_sau", "crossing_advice"))
	if r.state(chain[3]) != StateDone || r.state(chain[4]) != StateActive {
		t.Fatalf("after Chú Sáu: %v", r.book.Active())
	}

	// 5. the coffee and the snack, any order
	r.do(signal("di_nam", "hoi_gia_banh_trang"))
	if r.state(chain[4]) != StateActive {
		t.Fatal("one price finished the quest")
	}
	r.do(signal("chu_tu", "hoi_gia_ca_phe"))
	if r.state(chain[4]) != StateDone {
		t.Fatalf("eating: %+v", r.book.Records[chain[4]])
	}
	// four quests of 50.000 and the eating quest of 100.000
	if got := r.xuGiven(); got != 300_000 {
		t.Fatalf("the chain paid %d, want 300000", got)
	}
	// the Crab ride is still there, untouched, for whenever there is a phone
	if r.state("traveler.di_crab") != StateActive {
		t.Fatal("the Crab ride was dropped")
	}
}

// A foreigner who started the Crab ride before it waited for the money lesson keeps it running.
func TestACrabRideAlreadyRunningKeepsRunning(t *testing.T) {
	r := newRun(t, "traveler")
	// started before the ride waited for the money lesson: put the record in by hand, then refresh as a join does
	r.book.Records["traveler.di_crab"] = &Record{ID: "traveler.di_crab", Version: 1, State: StateActive,
		Objectives: map[string]*objective.Progress{"ride": {}}, StartedAt: 1}
	r.collect(r.e.Refresh(r.who, r.book, r.now+1))
	if r.state("traveler.di_crab") != StateActive {
		t.Fatal("a running Crab ride was dropped")
	}
	r.do(events.Event{Kind: events.TripCompleted, Role: "passenger", Partners: []string{"d1"}})
	if r.state("traveler.di_crab") != StateDone {
		t.Fatal("a running Crab ride cannot be finished")
	}
}

// Cô Ba and Anh Tony both teach xin chào and cảm ơn: either one fills the Sổ sưu tầm.
func TestEitherTeacherFillsThePhraseEntries(t *testing.T) {
	for _, npc := range []string{"co_ba", "anh_tony"} {
		r := newRun(t)
		r.do(signal(npc, "phrase_xin_chao"))
		r.do(signal(npc, "phrase_cam_on"))
		for _, id := range []string{"xin_chao", "cam_on"} {
			if r.state(collectionPrefix+id) != StateDone {
				t.Errorf("%s did not fill %s", npc, id)
			}
		}
	}
}
