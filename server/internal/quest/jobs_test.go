package quest

import (
	"encoding/json"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
)

// The odd jobs (docs/design/13-jobs.md) and the story quest that opens the coffee run: each walked through, the
// time limit failing and retrying, and the daily limit.

const secs = int64(time.Second / time.Millisecond)

// at moves the character's clock to unix milliseconds and starts what that day brings.
func (r *run) at(ms int64) {
	r.now = ms
	r.collect(r.e.Refresh(r.who, r.book, r.now))
}

// after moves the clock on by seconds, without an event.
func (r *run) after(seconds int64) { r.now += seconds * secs }

func (r *run) progressMsg() map[string]map[string]int {
	r.t.Helper()
	var p struct {
		Quests []struct {
			ID   string
			Left map[string]int
		}
	}
	if err := json.Unmarshal(r.e.progress(r.book, nil, r.now), &p); err != nil {
		r.t.Fatal(err)
	}
	out := map[string]map[string]int{}
	for _, q := range p.Quests {
		out[q.ID] = q.Left
	}
	return out
}

func (r *run) jobStarted(id string) bool {
	rec := r.book.Records[id]
	return rec != nil && rec.Objectives["run"] != nil && rec.Objectives["run"].T != 0
}

func newJobRun(t *testing.T, tags ...string) *run {
	r := newRun(t, tags...)
	r.at(noon(19))
	return r
}

var coffeeStops = [][2]string{{"bung_binh_quach_thi_trang", "street"}, {"duong_ham_nghi", "street"}, {"duong_le_loi", "street"}}

func TestTheCoffeeRunPaysWhenTheStopsAreReachedInTime(t *testing.T) {
	const job = "job.bung_ca_phe"
	r := newJobRun(t, "traveler")
	if r.state(job) != StateActive || r.jobStarted(job) {
		t.Fatalf("the job is on offer to everyone with a phone, not taken: %+v", r.book.Records[job])
	}
	if left, ok := r.progressMsg()[job]["run"]; ok {
		t.Fatalf("a job not taken has no clock: %d", left)
	}
	// the stops mean nothing before the job is taken
	r.do(entered("bung_binh_quach_thi_trang", "street"))
	if r.n(job, "run") != 0 {
		t.Fatal("a stop counted before the job was taken")
	}
	r.do(signal("chu_tu", "nhan_viec_bung_ca_phe"))
	if !r.jobStarted(job) {
		t.Fatal("picking the answer did not take the job")
	}
	if left := r.progressMsg()[job]["run"]; left != 120 {
		t.Fatalf("the clock reads %d s, want 120", left)
	}
	// the stops come in order: Hàm Nghi first is not the roundabout
	r.do(entered("duong_ham_nghi", "street"))
	if r.n(job, "run") != 0 {
		t.Fatal("a stop out of order counted")
	}
	// asking again does not buy more time
	r.after(30)
	r.do(signal("chu_tu", "nhan_viec_bung_ca_phe"))
	if left := r.progressMsg()[job]["run"]; left != 90 {
		t.Fatalf("taking it again moved the clock: %d s left, want 90", left)
	}
	for i, s := range coffeeStops {
		r.after(10)
		r.do(entered(s[0], s[1]))
		if want := i + 1; r.n(job, "run") != want && r.state(job) != StateDone {
			t.Fatalf("after stop %d: n = %d", want, r.n(job, "run"))
		}
	}
	if r.state(job) != StateDone || r.xuGiven() != 40_000 {
		t.Fatalf("the run is %s and paid %d, want done and 40000", r.state(job), r.xuGiven())
	}
	// being done with a job does not count as a day of the daily streak
	if r.book.Streak.Days != 0 {
		t.Fatalf("a job counted toward the streak: %+v", r.book.Streak)
	}
}

func TestACoffeeRunTooSlowFailsAndCanBeTakenAgain(t *testing.T) {
	const job = "job.bung_ca_phe"
	r := newJobRun(t, "citizen", "native")
	r.do(signal("chu_tu", "nhan_viec_bung_ca_phe"))
	r.do(entered("bung_binh_quach_thi_trang", "street"))
	r.do(entered("duong_ham_nghi", "street"))
	if r.n(job, "run") != 2 {
		t.Fatalf("n = %d", r.n(job, "run"))
	}
	// the ice melts: the clock is up before the last stop
	r.after(121)
	if left := r.progressMsg()[job]["run"]; left != 0 {
		t.Fatalf("the clock reads %d s after the time was up", left)
	}
	r.do(entered("duong_le_loi", "street"))
	if r.state(job) != StateActive || r.n(job, "run") != 0 || r.jobStarted(job) {
		t.Fatalf("a late stop finished or kept the run: %+v", r.book.Records[job])
	}
	if r.xuGiven() != 0 {
		t.Fatalf("a failed run paid %d", r.xuGiven())
	}
	if _, ok := r.progressMsg()[job]["run"]; ok {
		t.Fatal("the failed run still has a clock")
	}
	// taking it again starts over, and this time it is in time
	r.do(signal("chu_tu", "nhan_viec_bung_ca_phe"))
	for _, s := range coffeeStops {
		r.after(20)
		r.do(entered(s[0], s[1]))
	}
	if r.state(job) != StateDone || r.xuGiven() != 40_000 {
		t.Fatalf("the second run is %s and paid %d", r.state(job), r.xuGiven())
	}
}

func TestTakingAJobAgainAfterTheTimeIsUpRestartsIt(t *testing.T) {
	const job = "job.giao_banh_trang"
	r := newJobRun(t, "citizen")
	r.do(signal("di_nam", "nhan_viec_giao_banh_trang"))
	r.do(entered("cong_vien_30_4", "area"))
	r.after(241)
	r.do(signal("di_nam", "nhan_viec_giao_banh_trang"))
	if !r.jobStarted(job) || r.n(job, "run") != 0 {
		t.Fatalf("a late job taken again: %+v", r.book.Records[job])
	}
	if left := r.progressMsg()[job]["run"]; left != 240 {
		t.Fatalf("the new clock reads %d s", left)
	}
	r.do(entered("cong_vien_30_4", "area"))
	r.do(entered("nha_tho_duc_ba", "landmark"))
	if r.state(job) != StateDone || r.xuGiven() != 30_000 {
		t.Fatalf("the delivery is %s and paid %d, want done and 30000", r.state(job), r.xuGiven())
	}
}

func TestTheFlyersGoToFiveDifferentPeopleInAnyOrder(t *testing.T) {
	const job = "job.phat_to_roi"
	r := newJobRun(t, "traveler")
	hand := func(npc string) { r.do(signal(npc, "nhan_to_roi")) }
	hand("anh_khoi") // before taking the job it is only a chat
	if r.jobStarted(job) || r.n(job, "run") != 0 {
		t.Fatal("a flyer counted before the job was taken")
	}
	r.do(signal("chi_hanh", "nhan_viec_phat_to_roi"))
	if left := r.progressMsg()[job]["run"]; left != -1 {
		t.Fatalf("a job without a time limit reads %d, want -1", left)
	}
	hand("ban_minh")
	hand("ban_minh") // the same person twice is one flyer
	r.do(talked("chu_hung"))
	r.do(signal("co_ba", "nhan_to_roi")) // not a passer-by
	if r.n(job, "run") != 1 {
		t.Fatalf("n = %d after one flyer, a repeat, a chat and the wrong NPC", r.n(job, "run"))
	}
	r.after(3600) // no limit: an hour later it still counts
	for _, n := range []string{"anh_khoi", "chu_hung", "chi_lan", "co_hoa"} {
		hand(n)
	}
	if r.state(job) != StateDone || r.xuGiven() != 30_000 {
		t.Fatalf("the flyers are %s and paid %d, want done and 30000", r.state(job), r.xuGiven())
	}
}

func TestTheParcelsAreCarriedThereAndBackThreeTimes(t *testing.T) {
	const job = "job.khuan_hang"
	r := newJobRun(t, "citizen", "newcomer")
	take := func() { r.do(signal("anh_hai", "lay_kien")) }
	give := func() { r.do(signal("co_ba", "giao_kien")) }
	r.do(signal("co_ba", "nhan_viec_khuan_hang"))
	give() // no parcel in hand
	if r.n(job, "run") != 0 {
		t.Fatal("delivered a parcel that was never picked up")
	}
	for i := 1; i <= 3; i++ {
		take()
		take() // two parcels cannot be picked up in a row
		give()
		if want := 2 * i; r.n(job, "run") != want && r.state(job) != StateDone {
			t.Fatalf("after parcel %d: n = %d, want %d", i, r.n(job, "run"), want)
		}
	}
	if r.state(job) != StateDone || r.xuGiven() != 40_000 {
		t.Fatalf("the hauling is %s and paid %d, want done and 40000", r.state(job), r.xuGiven())
	}
}

// Each job is done once a day: the day after, it is on again.
func TestAJobIsDoneOnceADayAndComesBackTomorrow(t *testing.T) {
	const job = "job.bung_ca_phe"
	r := newJobRun(t, "traveler")
	run := func() {
		r.do(signal("chu_tu", "nhan_viec_bung_ca_phe"))
		for _, s := range coffeeStops {
			r.do(entered(s[0], s[1]))
		}
	}
	run()
	if r.state(job) != StateDone || r.xuGiven() != 40_000 {
		t.Fatalf("first run: %s, paid %d", r.state(job), r.xuGiven())
	}
	// the same day: nothing to take, nothing more is paid, however many times
	for range 2 {
		run()
	}
	if r.state(job) != StateDone || r.xuGiven() != 40_000 {
		t.Fatalf("second run the same day: %s, paid %d", r.state(job), r.xuGiven())
	}
	if _, ok := r.book.Active()[job]; ok {
		t.Fatal("a job done today still shows as running, so Chú Tư would still offer it")
	}
	// tomorrow: on again, not taken
	r.at(noon(20))
	if r.state(job) != StateActive || r.jobStarted(job) {
		t.Fatalf("the next day: %+v", r.book.Records[job])
	}
	run()
	if r.state(job) != StateDone || r.xuGiven() != 80_000 {
		t.Fatalf("the next day's run: %s, paid %d in all", r.state(job), r.xuGiven())
	}
}

// A job half done when the day ends is gone with it.
func TestAJobLeftHalfDoneIsDroppedWithItsDay(t *testing.T) {
	const job = "job.khuan_hang"
	r := newJobRun(t, "traveler")
	r.do(signal("co_ba", "nhan_viec_khuan_hang"))
	r.do(signal("anh_hai", "lay_kien"))
	r.at(noon(20))
	if r.jobStarted(job) || r.n(job, "run") != 0 {
		t.Fatalf("yesterday's run carried over: %+v", r.book.Records[job])
	}
}

func TestEveryoneWithAPhoneIsOfferedTheJobsAndNobodyWithoutOne(t *testing.T) {
	jobs := []string{"job.bung_ca_phe", "job.giao_banh_trang", "job.phat_to_roi", "job.khuan_hang"}
	for _, tags := range [][]string{{"citizen", "newcomer"}, {"citizen", "native"}, {"traveler"}} {
		r := newRunWith(t, nil, tags...)
		for _, id := range jobs {
			if r.state(id) != "" {
				t.Fatalf("%v: %s offered with no phone", tags, id)
			}
		}
		r = newRunWith(t, capability.Of(content.CapPhone), tags...)
		for _, id := range jobs {
			if r.state(id) != StateActive {
				t.Fatalf("%v: %s not offered with a basic phone", tags, id)
			}
		}
	}
}

func TestAllTheJobsInADayStayWithinTheMoneyTarget(t *testing.T) {
	c := catalog(t)
	var total int64
	for _, q := range c.Quests {
		if !q.Job {
			continue
		}
		if q.Repeat != content.RepeatDaily || q.Rewards.Xu > content.XuJobRewardMax || q.Rewards.Xu < 20_000 {
			t.Errorf("%s: repeat %q, pays %d", q.ID, q.Repeat, q.Rewards.Xu)
		}
		total += q.Rewards.Xu
	}
	// 12 to 15 jobs are enough for chapter 3's 1.500.000đ together with the story quests: a few days of four
	if total > 150_000 {
		t.Fatalf("a day of jobs pays %d: too fast a way to the licence and the bike", total)
	}
}

func TestTheStoryJobAtChuTuIsTheNativesAndNewcomersToDoOnce(t *testing.T) {
	const story = "story.xin_viec_chu_tu"
	r := newJobRun(t, "citizen", "native")
	if r.state(story) != StateActive {
		t.Fatalf("not offered to a native with a phone: %v", r.book.Active())
	}
	if f := newRunWith(t, nil, "citizen", "native"); f.state(story) != "" {
		t.Fatal("offered without a phone")
	}
	if tr := newJobRun(t, "traveler"); tr.state(story) != "" {
		t.Fatal("offered to a foreigner: the job is the Vietnamese's story")
	}
	// the shift: take it, 23/9 Park, Bùi Viện, then report back
	r.do(signal("chu_tu", "xin_viec_ca_sang"))
	r.do(entered("cong_vien_23_9", "area"))
	r.do(entered("pho_di_bo_bui_vien", "street"))
	r.do(signal("chu_tu", "het_ca")) // a stop of the shift: it counts as the third
	if r.state(story) != StateDone {
		t.Fatalf("the shift: %+v", r.book.Records[story])
	}
	if r.xuGiven() != 100_000 || !r.book.Titles["phuc_vu_chu_tu"] {
		t.Fatalf("paid %d, titles %v", r.xuGiven(), r.book.Titles)
	}
	// done for good: tomorrow it is not on again, and the coffee run is still there
	r.at(noon(20))
	if r.state(story) != StateDone || r.state("job.bung_ca_phe") != StateActive {
		t.Fatalf("the next day: story %s, run %s", r.state(story), r.state("job.bung_ca_phe"))
	}
}

func TestTheStoryJobFailsWhenTheShiftRunsOutAndCanBeAskedForAgain(t *testing.T) {
	const story = "story.xin_viec_chu_tu"
	r := newJobRun(t, "citizen", "newcomer")
	r.do(signal("chu_tu", "xin_viec_ca_sang"))
	r.do(entered("cong_vien_23_9", "area"))
	r.after(301)
	r.do(entered("pho_di_bo_bui_vien", "street"))
	if r.state(story) != StateActive || r.jobStarted(story) {
		t.Fatalf("a late shift: %+v", r.book.Records[story])
	}
	r.do(signal("chu_tu", "xin_viec_ca_sang"))
	r.do(entered("cong_vien_23_9", "area"))
	r.do(entered("pho_di_bo_bui_vien", "street"))
	r.do(signal("chu_tu", "het_ca"))
	if r.state(story) != StateDone {
		t.Fatalf("the second shift: %+v", r.book.Records[story])
	}
}

// The quests the story job was written beside keep working for those who have them running.
func TestTheMorningCoffeeAndNeighbourhoodQuestsAreUntouched(t *testing.T) {
	r := newJobRun(t, "citizen", "native")
	for _, id := range []string{"citizen.ca_phe_sang", "citizen.chuyen_khu_pho"} {
		if r.state(id) != StateActive {
			t.Fatalf("%s: %q", id, r.state(id))
		}
	}
	r.do(signal("chu_tu", "order_ca_phe_sua_da"))
	if r.n("citizen.ca_phe_sang", "order") != 1 {
		t.Fatal("ordering a coffee no longer counts for the morning coffee")
	}
}
