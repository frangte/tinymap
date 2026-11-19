package driving_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/driving"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type member struct {
	id     uint32
	caps   capability.Set
	pose   protocol.Pose
	riding uint32
	own    bool
}

func (m *member) ID() uint32                  { return m.id }
func (m *member) Character() string           { return "charLan" }
func (m *member) Name() string                { return "Lan" }
func (m *member) Arch() string                { return "" }
func (m *member) Caps() capability.Set        { return m.caps }
func (m *member) Pose() protocol.Pose         { return m.pose }
func (m *member) Speed() float64              { return 0 }
func (m *member) Riding() (uint32, int, bool) { return m.riding, 0, m.own }
func (m *member) MutedUntil() time.Time       { return time.Time{} }

type host struct {
	m       *member
	now     time.Time
	sent    []protocol.DriveTest
	events  []events.Event
	lent    []world.VehicleSpec
	returns int
	lendOK  bool
	jobs    []func()
}

func (h *host) Member(id uint32) (world.Member, bool) { return h.m, id == h.m.id }
func (h *host) Each(fn func(world.Member))            { fn(h.m) }
func (h *host) Seat(uint32, uint32) bool              { return false }
func (h *host) Unseat(uint32)                         {}
func (h *host) Pin(uint32, uint32, bool)              {}
func (h *host) Send(_ uint32, frame []byte) {
	var d protocol.DriveTest
	if json.Unmarshal(frame, &d) == nil && d.T == protocol.TypeDriveTest {
		h.sent = append(h.sent, d)
	}
}
func (h *host) Do(_ string, run func(context.Context) error, done func(error)) {
	h.jobs = append(h.jobs, func() { done(run(context.Background())) })
}
func (h *host) Publish(ev events.Event)        { h.events = append(h.events, ev) }
func (h *host) Now() time.Time                 { return h.now }
func (h *host) Kick(uint32, string)            {}
func (h *host) Park(uint32, world.VehicleSpec) {}
func (h *host) Return(uint32)                  { h.returns++ }
func (h *host) Lend(_ uint32, v world.VehicleSpec) bool {
	if h.lendOK {
		h.lent = append(h.lent, v)
	}
	return h.lendOK
}

// settle runs the queued jobs, as the world's workers would.
func (h *host) settle() {
	for len(h.jobs) > 0 {
		j := h.jobs[0]
		h.jobs = h.jobs[1:]
		j()
	}
}

// bank keeps the capabilities as the store would.
type bank struct{ held map[string]string }

func (b *bank) GrantCapability(_ context.Context, _, k, source string, _ int64) error {
	b.held[k] = source
	return nil
}
func (b *bank) RevokeCapability(_ context.Context, _, k string) error {
	delete(b.held, k)
	return nil
}

type fixture struct {
	t    *testing.T
	h    *host
	b    *bank
	s    *driving.Service
	cat  *content.Catalog
	drv  content.Driving
	prop capability.Set
}

func newFixture(t *testing.T, caps ...string) *fixture {
	t.Helper()
	cat, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	m := &member{id: 1, caps: capability.Of(caps...)}
	h := &host{m: m, now: time.Unix(1_000, 0), lendOK: true}
	b := &bank{held: map[string]string{}}
	f := &fixture{t: t, h: h, b: b, cat: cat, drv: cat.Driving}
	f.s = driving.New(h, cat, b, quiet)
	// the connection: what the module grants or revokes reaches the member's capabilities
	f.s.OnGrant = func(_, k string) { m.caps[k] = struct{}{} }
	f.s.OnRevoke = func(_, k string) { delete(m.caps, k) }
	return f
}

func (f *fixture) signal(sig string) {
	f.t.Helper()
	f.s.Signalled(f.h.m, f.drv.NPC, sig)
	f.h.settle()
}

func (f *fixture) has(k string) bool { return f.h.m.caps.Has(k) }

func (f *fixture) tick(d time.Duration) {
	f.h.now = f.h.now.Add(d)
	f.s.Tick(f.h.now)
}

func (f *fixture) at(i int) {
	x, z := f.drv.Checkpoints[i].ThreeXZ()
	f.h.m.pose = protocol.Pose{X: x, Z: z}
}

func (f *fixture) last() protocol.DriveTest {
	f.t.Helper()
	if len(f.h.sent) == 0 {
		f.t.Fatal("nothing was sent to the player")
	}
	return f.h.sent[len(f.h.sent)-1]
}

// ---- the theory: signals become tickets

func TestStartingTheTheorySpendsTheTicketAndMarksTheAttempt(t *testing.T) {
	f := newFixture(t, content.CapExamTicket)
	f.signal(content.SignalTheoryStart)
	if f.has(content.CapExamTicket) || !f.has(content.CapExamRetake) {
		t.Fatalf("caps %v: the ticket is spent and the next fee is the retake one", f.h.m.caps)
	}
	if _, kept := f.b.held[content.CapExamTicket]; kept {
		t.Fatal("the spent ticket is still stored")
	}
	if f.b.held[content.CapExamRetake] == "" {
		t.Fatal("the retake mark was not stored")
	}
}

func TestStartingTheTheoryWithoutATicketDoesNothing(t *testing.T) {
	f := newFixture(t)
	f.signal(content.SignalTheoryStart)
	if f.has(content.CapExamRetake) || len(f.b.held) != 0 {
		t.Fatalf("a signal without a ticket changed %v", f.b.held)
	}
}

func TestPassingTheTheoryGivesThePracticalItsTicket(t *testing.T) {
	f := newFixture(t, content.CapExamTicket)
	f.signal(content.SignalTheoryStart)
	f.signal(content.SignalTheoryPass)
	if !f.has(content.CapExamTheory) || !f.has(content.CapExamTicket) {
		t.Fatalf("caps %v: want the theory pass and a ticket for the practical", f.h.m.caps)
	}
}

func TestAPassSignalWithoutAnAttemptIsIgnored(t *testing.T) {
	// a tampered conversation cannot reach the pass node without starting; if it somehow did, nothing is given
	f := newFixture(t, content.CapExamTicket)
	f.signal(content.SignalTheoryPass)
	if f.has(content.CapExamTheory) {
		t.Fatal("a theory pass without an attempt was granted")
	}
}

// ---- the practical

func (f *fixture) readyForPractical() {
	f.t.Helper()
	f.h.m.caps = capability.Of(content.CapExamTicket, content.CapExamTheory, content.CapExamRetake)
}

func TestThePracticalNeedsTheTheoryAndATicketAndALendableBike(t *testing.T) {
	cases := []struct {
		name   string
		caps   []string
		lendOK bool
	}{
		{"no ticket", []string{content.CapExamTheory}, true},
		{"no theory pass", []string{content.CapExamTicket}, true},
		{"the player cannot be lent a bike", []string{content.CapExamTicket, content.CapExamTheory}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, c.caps...)
			f.h.lendOK = c.lendOK
			f.signal(content.SignalPracticalGo)
			if len(f.h.lent) != 0 || len(f.h.sent) != 0 {
				t.Fatalf("lent %v, sent %v", f.h.lent, f.h.sent)
			}
			if slices.Contains(c.caps, content.CapExamTicket) && !f.has(content.CapExamTicket) {
				t.Fatal("the ticket was spent for a practical that did not start")
			}
		})
	}
}

func TestThePracticalLendsTheSchoolsBikeAndSpendsTheTicket(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	if len(f.h.lent) != 1 || f.h.lent[0].Kind != f.drv.Vehicle || f.h.lent[0].Requires != content.CapExamTheory {
		t.Fatalf("lent %+v", f.h.lent)
	}
	if f.has(content.CapExamTicket) {
		t.Fatal("the ticket stays after the practical started")
	}
	if got := f.last(); got.State != "lent" {
		t.Fatalf("sent %+v", got)
	}
	// a second signal while it runs starts nothing more
	f.h.m.caps[content.CapExamTicket] = struct{}{}
	f.signal(content.SignalPracticalGo)
	if len(f.h.lent) != 1 {
		t.Fatalf("lent twice: %+v", f.h.lent)
	}
}

func (f *fixture) mount() {
	f.h.m.riding, f.h.m.own = 7, true
	f.tick(time.Second)
}

func TestRidingThroughEveryCheckpointPassesTheTest(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	f.mount()
	if got := f.last(); got.State != "run" || got.Index != 0 || got.Total != len(f.drv.Checkpoints) || got.Left <= 0 || len(got.Next) != 2 {
		t.Fatalf("sent %+v", got)
	}
	for i := range f.drv.Checkpoints {
		// out of order is not a checkpoint: only the next one counts
		f.at(i)
		f.tick(time.Second)
	}
	if got := f.last(); got.State != "pass" {
		t.Fatalf("sent %+v", got)
	}
	if len(f.h.events) != 1 || f.h.events[0].Kind != events.TestPassed || f.h.events[0].Test != content.TestA1 || f.h.events[0].Character != "charLan" {
		t.Fatalf("events %+v", f.h.events)
	}
	if f.h.returns != 1 {
		t.Fatalf("the bike was returned %d times", f.h.returns)
	}
	f.h.settle()
	if f.has(content.CapExamTheory) || f.has(content.CapExamRetake) {
		t.Fatalf("caps %v: the attempt's marks go with a pass", f.h.m.caps)
	}
}

func TestACheckpointOutOfOrderDoesNotCount(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	f.mount()
	f.at(len(f.drv.Checkpoints) - 1)
	f.tick(time.Second)
	if f.last().State != "run" || f.last().Index != 0 {
		t.Fatalf("sent %+v: the last checkpoint first must not pass", f.last())
	}
}

func TestGettingOffFailsTheTestAndKeepsTheTheoryPass(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	f.mount()
	f.h.m.riding = 0
	f.tick(time.Second)
	if got := f.last(); got.State != "fail" || got.Reason != "off" {
		t.Fatalf("sent %+v", got)
	}
	if len(f.h.events) != 0 || f.h.returns != 1 {
		t.Fatalf("events %v, returns %d", f.h.events, f.h.returns)
	}
	if !f.has(content.CapExamTheory) || !f.has(content.CapExamRetake) || f.has(content.CapExamTicket) {
		t.Fatalf("caps %v: the theory stays, the ticket is spent, the retake fee is next", f.h.m.caps)
	}
}

func TestRunningOutOfTimeFails(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	f.mount()
	f.tick(time.Duration(f.drv.TimeLimit+1) * time.Second)
	if got := f.last(); got.State != "fail" || got.Reason != "time" {
		t.Fatalf("sent %+v", got)
	}
}

func TestABikeLeftUnriddenIsTakenBack(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	f.tick(time.Duration(f.drv.MountWithin+1) * time.Second)
	if got := f.last(); got.State != "fail" || got.Reason != "idle" || f.h.returns != 1 {
		t.Fatalf("sent %+v, returns %d", got, f.h.returns)
	}
}

func TestLeavingTheWorldEndsTheRunWithoutAnAnswer(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	f.s.Left(f.h.m)
	f.tick(time.Hour)
	if len(f.h.events) != 0 {
		t.Fatalf("events %+v", f.h.events)
	}
	if f.s.ArchBusy(f.h.m) {
		t.Fatal("a run that ended still holds the player")
	}
}

func TestAnArchetypeChangeWaitsForThePractical(t *testing.T) {
	f := newFixture(t)
	f.readyForPractical()
	f.signal(content.SignalPracticalGo)
	if !f.s.ArchBusy(f.h.m) {
		t.Fatal("a change of archetype must wait for the test")
	}
}

func TestOtherNPCsSignalsDoNothing(t *testing.T) {
	f := newFixture(t, content.CapExamTicket)
	f.s.Signalled(f.h.m, "co_ba", content.SignalTheoryStart)
	f.h.settle()
	if !f.has(content.CapExamTicket) || f.has(content.CapExamRetake) {
		t.Fatalf("caps %v", f.h.m.caps)
	}
}
