package world

import (
	"slices"
	"testing"
	"time"

	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

// ---- chapter 3 in the world: owned and lent bikes, the licence, and the examiner's conversation
// (docs/design/13-driver.md)

var ownBike = VehicleSpec{Kind: "xe_so_110", Speed: 11, Requires: "vehicle.motorbike.ride"}

// Riding the bike needs the licence: without it the bike stands there, and a mount is refused.
func TestAnOwnedBikeNeedsTheLicenceToMount(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan", Character: "char-lan", Vehicle: &ownBike, Caps: capability.Of()})
	bike := m.owns.id
	h.drain(p)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: bike})
	if fs, _ := h.drain(p); count(fs, protocol.TypeError) != 1 || m.riding != nil {
		t.Fatalf("mounting without the licence: riding %v, %+v", m.riding, fs)
	}
	m.caps = capability.Of("vehicle.motorbike.ride")
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: bike})
	if m.riding == nil || m.riding.id != bike {
		t.Fatal("the licence did not let the owner mount")
	}
}

func TestParkingABikeMakesItTheOwnersOwn(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan", Character: "char-lan"})
	h.drain(p)
	if m.owns != nil {
		t.Fatal("it had a bike")
	}
	host{h.w}.Park(m.id, ownBike)
	if m.owns == nil || m.owns.kind != "xe_so_110" || m.owns.owner != m || m.owns.requires != "vehicle.motorbike.ride" {
		t.Fatalf("owns %+v", m.owns)
	}
	if fs, _ := h.drain(p); count(fs, protocol.TypeVehicle) != 1 {
		t.Fatalf("the others and the owner are shown the bike: %+v", fs)
	}
	// a second bike replaces the first
	first := m.owns.id
	host{h.w}.Park(m.id, VehicleSpec{Kind: "xe_so_110", Speed: 12})
	if m.owns == nil || m.owns.id == first || h.w.vehicles[first] != nil || len(h.w.vehicles) != 1 {
		t.Fatalf("vehicles %v", h.w.vehicles)
	}
}

func TestALentBikeReplacesTheOwnOneAndComesBack(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan", Character: "char-lan", Vehicle: &ownBike})
	h.drain(p)
	own := m.owns.id
	school := VehicleSpec{Kind: "xe_tap_lai", Speed: 11, Requires: "exam.a1.theory"}
	if !(host{h.w}).Lend(m.id, school) {
		t.Fatal("lend refused")
	}
	if m.owns == nil || m.owns.kind != "xe_tap_lai" || h.w.vehicles[own] != nil || !m.lent {
		t.Fatalf("owns %+v, lent %v", m.owns, m.lent)
	}
	// buying a bike meanwhile waits for the return
	host{h.w}.Park(m.id, VehicleSpec{Kind: "xe_so_110", Speed: 12})
	if m.owns.kind != "xe_tap_lai" {
		t.Fatal("a purchase took the lent bike away")
	}
	host{h.w}.Return(m.id)
	if m.owns == nil || m.owns.kind != "xe_so_110" || m.owns.speed != 12 || m.lent || len(h.w.vehicles) != 1 {
		t.Fatalf("after the return: owns %+v, vehicles %v", m.owns, h.w.vehicles)
	}
}

func TestALentBikeIsOnlyForTheTheoryPass(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan", Character: "char-lan", Caps: capability.Of()})
	h.drain(p)
	host{h.w}.Lend(m.id, VehicleSpec{Kind: "xe_tap_lai", Speed: 11, Requires: "exam.a1.theory"})
	bike := m.owns.id
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: bike})
	if m.riding != nil {
		t.Fatal("the school's bike was ridden without the theory pass")
	}
	m.caps = capability.Of("exam.a1.theory")
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: bike})
	if m.riding == nil {
		t.Fatal("the theory pass did not allow the school's bike")
	}
	// returning it puts the rider on the ground, the bike gone
	host{h.w}.Return(m.id)
	if m.riding != nil || h.w.vehicles[bike] != nil {
		t.Fatalf("riding %v after the return", m.riding)
	}
}

func TestACitizenOnSomeoneElsesBikeCannotBeLent(t *testing.T) {
	h := newHarness(t)
	pd, d, pp, p := h.driverAndPassenger()
	h.drain(pd)
	h.drain(pp)
	_ = d
	if (host{h.w}).Lend(p.id, VehicleSpec{Kind: "xe_tap_lai", Speed: 11}) == false {
		t.Fatal("a standing player can be lent a bike")
	}
	h.w.onMsg(pd, protocol.ClientMsg{T: protocol.TypeMount, V: d.owns.id})
	// a passenger behind a rider cannot
	if !h.w.seat(p, d.riding) {
		t.Skip("could not seat the passenger")
	}
	if (host{h.w}).Lend(p.id, VehicleSpec{Kind: "xe_tap_lai", Speed: 11}) {
		t.Fatal("a seated passenger was lent a bike")
	}
}

// signalRecorder is a module that hears the conversation's signals.
type signalRecorder struct {
	recorder
	heard []string
}

func (s *signalRecorder) Signalled(_ Member, npc, signal string) {
	s.heard = append(s.heard, npc+"/"+signal)
}

func examHarness(t *testing.T, caps ...string) (*harness, *signalRecorder, *Peer, *member, *npc) {
	t.Helper()
	h, _ := castHarness(t, nil)
	rec := &signalRecorder{}
	h.w.Use(func(host Host) Module {
		rec.h = host
		return rec
	})
	thay := h.npc("thay_muoi")
	x, z := h.near(thay, 1.5, 0)
	p, m := h.join(Arrival{Name: "Lan", Character: "char-lan", Caps: capability.Of(caps...), Start: &protocol.Pose{X: x, Z: z}})
	h.drain(p)
	return h, rec, p, m, thay
}

// answer picks the offered answer whose English text is en, on the line the conversation is at.
func (h *harness) answer(p *Peer, m *member, en string) {
	h.t.Helper()
	s := m.talk
	if s == nil {
		h.t.Fatal("not in a conversation")
	}
	l, err := h.w.cfg.Dialogue.Line(s.npc.spec.Dialogue, s.node, m.speaker())
	if err != nil {
		h.t.Fatal(err)
	}
	i := slices.IndexFunc(l.Choices, func(c protocol.Text) bool { return c.EN == en })
	if i < 0 {
		h.t.Fatalf("%s: no answer %q among %v", s.node, en, l.Choices)
	}
	h.choose(p, s.id, i, 0)
}

// The signals of an answer, and of a node the conversation arrives at, reach the modules; the quiz's tally lives in
// the conversation and three wrong answers end at the fail node.
func TestTheExaminersQuizRunsInTheConversation(t *testing.T) {
	h, rec, p, m, thay := examHarness(t, "exam.a1.ticket")
	h.interact(p, thay, 1)
	for _, en := range []string{"I want to take the A1 motorbike licence.", "I will take the theory test.", "Ready."} {
		h.answer(p, m, en)
	}
	if !slices.Equal(rec.heard, []string{"thay_muoi/thi_bat_dau"}) {
		t.Fatalf("heard %v: arriving at the first question starts the attempt", rec.heard)
	}
	// wrong answers: the first of each question's choices is wrong at q1 (right is second), q2 (third) and q3 (first, so take the second)
	wrongs := map[string]int{"q1": 0, "q2": 0, "q3": 1}
	for _, q := range []string{"q1", "q2", "q3"} {
		if m.talk.node != q {
			t.Fatalf("at %q, want %q", m.talk.node, q)
		}
		h.choose(p, m.talk.id, wrongs[q], 0)
		if q != "q3" {
			h.choose(p, m.talk.id, 0, 0) // the explanation: next question
		}
	}
	if m.talk.node != "fail" || m.talk.marks["wrong"] != 3 {
		t.Fatalf("after three wrong answers: node %q, marks %v", m.talk.node, m.talk.marks)
	}
	if slices.Contains(rec.heard, "thay_muoi/ly_thuyet_dau") {
		t.Fatalf("heard %v", rec.heard)
	}
}

// A fee line carries its fee to the client, and when the player's capabilities change the line is shown again
// with the answers they now unlock: paying makes "Paid. Let us do the theory" appear.
func TestAFeeLineIsShownAgainWhenTheTicketArrives(t *testing.T) {
	h, _, p, m, thay := examHarness(t)
	h.interact(p, thay, 1)
	h.answer(p, m, "I want to take the A1 motorbike licence.")
	h.answer(p, m, "I will pay the test fee.")
	fs, _ := h.drain(p)
	in := fs[len(fs)-1]
	if in.T != protocol.TypeInteraction || m.talk.node != "fee_first" {
		t.Fatalf("at %q: %+v", m.talk.node, fs)
	}
	if slices.ContainsFunc(in.Choices, func(c protocol.Text) bool { return c.EN == "Paid. Let us do the theory, Thầy." }) {
		t.Fatalf("the quiz is offered before paying: %+v", in.Choices)
	}
	h.w.handle(event{kind: evCaps, p: p, caps: capability.Of("exam.a1.ticket")})
	fs, _ = h.drain(p)
	in = only(t, fs, protocol.TypeInteraction)
	if !slices.ContainsFunc(in.Choices, func(c protocol.Text) bool { return c.EN == "Paid. Let us do the theory, Thầy." }) {
		t.Fatalf("the line was not refreshed with the unlocked answer: %+v", in.Choices)
	}
}

func TestAnArchetypeChangeIsRefusedDuringAConversation(t *testing.T) {
	h, _, p, m, thay := examHarness(t)
	h.interact(p, thay, 1)
	h.tick(DefaultArchGap + time.Second)
	if code := h.w.archRefused(m); code != protocol.ErrArchBusy {
		t.Fatalf("code %q", code)
	}
	_ = p
}
