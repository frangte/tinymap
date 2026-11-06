package quest

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/gameday"
)

// noon in Saigon on a day of November 2026 (05:00 UTC)
func noon(day int) int64 { return time.Date(2026, 11, day, 5, 0, 0, 0, time.UTC).UnixMilli() }

// dailyOn is the daily of the pool that is on that day.
func dailyOn(t *testing.T, e *Engine, pool string, now int64) string {
	t.Helper()
	var on []string
	for _, id := range e.pools[pool] {
		if e.onToday(e.byID[id], gameday.OfMilli(now)) {
			on = append(on, id)
		}
	}
	if len(on) != 1 {
		t.Fatalf("pool %s on %s: %v are on, want exactly one", pool, gameday.OfMilli(now), on)
	}
	return on[0]
}

func TestEachPoolHasOneDailyADayAndEveryOneGetsATurn(t *testing.T) {
	e := engine(t)
	if len(e.pools) == 0 {
		t.Fatal("quests.json has no daily pools")
	}
	for pool, ids := range e.pools {
		turns := map[string]int{}
		for d := 0; d < 60; d++ {
			turns[dailyOn(t, e, pool, noon(1)+int64(d)*24*3600*1000)]++
		}
		for _, id := range ids {
			if turns[id] == 0 {
				t.Errorf("pool %s: %s never had a turn in 60 days (%v)", pool, id, turns)
			}
		}
	}
}

// citizenDaily runs a citizen through days: it finds which citizen daily is on and does it.
type citizenDaily struct {
	*run
}

func newCitizen(t *testing.T, now int64) citizenDaily {
	r := &run{t: t, e: engine(t), who: Player{Tags: []string{"citizen"}}, book: NewBook(Saved{}), now: now}
	r.collect(r.e.Refresh(r.who, r.book, r.now))
	return citizenDaily{r}
}

// at moves the clock to now and lets the engine start that day's dailies.
func (c citizenDaily) at(now int64) Outcome {
	c.now = now
	return c.collect(c.e.Refresh(c.who, c.book, c.now))
}

// finish does the daily "ghé thăm người quen" (on every day for everyone): talk to two NPCs.
func (c citizenDaily) finish() {
	c.t.Helper()
	for _, npc := range []string{"co_ba", "chu_tu"} {
		ev := events.Event{Kind: events.NPCTalked, Character: "c1", NPC: npc}
		c.collect(c.e.Apply(c.who, c.book, ev, c.now))
	}
	if c.state("daily.ghe_nguoi_quen") != StateDone {
		c.t.Fatalf("the daily is %q after talking to two NPCs", c.state("daily.ghe_nguoi_quen"))
	}
}

func TestADailyStartsOverEveryDayAndPaysEachDay(t *testing.T) {
	c := newCitizen(t, noon(6))
	rec := c.book.Records["daily.ghe_nguoi_quen"]
	if rec == nil || rec.Cycle != "2026-11-06" {
		t.Fatalf("the shared daily on the 6th: %+v", rec)
	}
	pooled := dailyOn(t, c.e, "citizen_daily", noon(6))
	if c.book.Records[pooled] == nil {
		t.Fatalf("today's citizen daily %s was not started", pooled)
	}
	for _, id := range c.e.pools["citizen_daily"] {
		if id != pooled && c.book.Records[id] != nil {
			t.Fatalf("%s started on a day that is not its turn", id)
		}
	}
	c.finish()
	// a second day: yesterday's goes, a fresh one starts, and doing it pays again under that day's source
	out := c.at(noon(7))
	if !slices.Contains(out.Dropped, "daily.ghe_nguoi_quen") {
		t.Fatalf("yesterday's daily was not dropped: %v", out.Dropped)
	}
	if rec := c.book.Records["daily.ghe_nguoi_quen"]; rec == nil || rec.State != StateActive || rec.Cycle != "2026-11-07" {
		t.Fatalf("the daily on the 7th: %+v", rec)
	}
	c.finish()
	var sources []string
	for _, g := range c.grants {
		if g.Xu > 0 {
			sources = append(sources, g.Source)
		}
	}
	want := []string{"quest:daily.ghe_nguoi_quen:2026-11-06", "quest:daily.ghe_nguoi_quen:2026-11-07"}
	if !slices.Equal(sources, want) {
		t.Fatalf("xu grants %v, want %v", sources, want)
	}
}

func TestYesterdaysDailyTakesNoMoreProgress(t *testing.T) {
	c := newCitizen(t, noon(6))
	// the 7th has begun but nothing refreshed yet (the event arrives first): it must not count for the 6th
	c.now = noon(7)
	out := c.collect(c.e.Apply(c.who, c.book, events.Event{Kind: events.NPCTalked, Character: "c1", NPC: "co_ba"}, c.now))
	rec := c.book.Records["daily.ghe_nguoi_quen"]
	if rec == nil || rec.Cycle != "2026-11-07" {
		t.Fatalf("after the first event of the 7th: %+v (dropped %v)", rec, out.Dropped)
	}
	if rec.Objectives["talk"].N != 0 {
		t.Fatalf("the event before the day's daily started counted for it: %d", rec.Objectives["talk"].N)
	}
}

func TestTheStreakCountsDaysInARow(t *testing.T) {
	c := newCitizen(t, noon(1))
	streak := func(want int, now int64) {
		t.Helper()
		if got := c.book.StreakNow(gameday.OfMilli(now)); got != want {
			t.Fatalf("streak on %s: %d, want %d (%+v)", gameday.OfMilli(now), got, want, c.book.Streak)
		}
	}
	c.finish()
	streak(1, noon(1))
	for day := 2; day <= 3; day++ {
		c.at(noon(day))
		c.finish()
	}
	streak(3, noon(3))
	if c.state(achievementPrefix+"chuoi_3_ngay") != StateDone {
		t.Fatal("three days running did not earn its achievement")
	}
	if !slices.ContainsFunc(c.grants, func(g Grant) bool { return slices.Contains(g.Stamps, "ba_ngay_lien") }) {
		t.Fatal("no streak stamp")
	}
	// the streak still shows the next day before a daily is done, and is gone a day later
	streak(3, noon(4))
	streak(0, noon(5))
	c.at(noon(5))
	c.finish()
	streak(1, noon(5))
	// the seven-day achievement kept its best (3): a broken streak does not take progress back
	if n := c.n(achievementPrefix+"chuoi_7_ngay", ""); n != 3 {
		t.Fatalf("seven-day achievement at %d, want 3", n)
	}
}

func TestTwoDailiesOnOneDayCountOneDayOfStreak(t *testing.T) {
	c := newCitizen(t, noon(1))
	c.finish()
	// the pool's daily done the same day (the engine calls streakUp for every daily completed)
	if c.book.streakUp("2026-11-01") {
		t.Fatal("a second daily on the same day moved the streak")
	}
	if c.book.Streak.Days != 1 {
		t.Fatalf("streak %+v", c.book.Streak)
	}
}

func TestTheLatestDayOfADailyIsTheOneLoaded(t *testing.T) {
	b := NewBook(Saved{Records: []Record{
		{ID: "daily.ghe_nguoi_quen", Cycle: "2026-11-07", State: StateActive},
		{ID: "daily.ghe_nguoi_quen", Cycle: "2026-11-06", State: StateDone},
	}})
	if r := b.Records["daily.ghe_nguoi_quen"]; r.Cycle != "2026-11-07" {
		t.Fatalf("loaded the %s row", r.Cycle)
	}
}

func TestDailiesAreCheckedWhenTheContentLoads(t *testing.T) {
	c := catalog(t)
	for _, q := range c.Quests {
		if q.Repeat != content.RepeatDaily {
			continue
		}
		if q.Rewards.Xu <= 0 {
			t.Errorf("daily %s pays nothing", q.ID)
		}
	}
}

// ---- the service: online across midnight, and the wallet

// clock is a time a test moves forward.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

func TestAPlayerOnlineAtMidnightGetsTheNewDaysDailies(t *testing.T) {
	store := newMemStore()
	store.balance["c1"] = 10_000
	w := newFakeWorld()
	s := NewService(engine(t), store, w, Config{Workers: 2, Queue: 64, StoreWait: time.Second, DayCheck: 10 * time.Millisecond},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	clk := &clock{t: time.UnixMilli(noon(6))}
	s.now = clk.now
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Run(ctx)
	s.Join("c1", 1, Player{Tags: []string{"citizen"}})
	w.last(t, "c1")
	for _, npc := range []string{"co_ba", "chu_tu"} {
		s.Submit(events.Event{Kind: events.NPCTalked, Character: "c1", NPC: npc})
		w.last(t, "c1")
	}
	w.mu.Lock()
	paid := slices.Clone(w.wallets["c1"])
	w.mu.Unlock()
	if !slices.Equal(paid, []int64{12_000}) {
		t.Fatalf("wallet after the daily: %v, want [12000]", paid)
	}
	w.mu.Lock()
	shown := w.frames["c1"][len(w.frames["c1"])-1].Streak
	w.mu.Unlock()
	if shown != 1 {
		t.Fatalf("streak shown %d", shown)
	}
	// midnight passes while the player stays: the 7th's daily arrives by itself, not yet done
	clk.set(time.UnixMilli(noon(7)))
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-w.received:
		case <-deadline:
			t.Fatal("no progress for the new day")
		}
		w.mu.Lock()
		f := w.frames["c1"]
		p := f[len(f)-1]
		w.mu.Unlock()
		if q, ok := questState(p, "daily.ghe_nguoi_quen"); ok && !q.Done {
			return
		}
	}
}
