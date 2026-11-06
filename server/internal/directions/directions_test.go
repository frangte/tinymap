package directions_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/directions"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// ---- a world of a few players, enough for the module

type member struct {
	id   uint32
	char string
	name string
	caps capability.Set
	pose protocol.Pose
}

func (m *member) ID() uint32                  { return m.id }
func (m *member) Character() string           { return m.char }
func (m *member) Name() string                { return m.name }
func (m *member) Arch() string                { return "" }
func (m *member) Caps() capability.Set        { return m.caps }
func (m *member) Pose() protocol.Pose         { return m.pose }
func (m *member) Speed() float64              { return 0 }
func (m *member) Riding() (uint32, int, bool) { return 0, 0, false }
func (m *member) MutedUntil() time.Time       { return time.Time{} }

type host struct {
	now     time.Time
	members map[uint32]*member
	inbox   map[uint32][]map[string]any
	pins    map[[2]uint32]bool
	events  []events.Event
}

func (h *host) Member(id uint32) (world.Member, bool) {
	m, ok := h.members[id]
	if !ok {
		return nil, false
	}
	return m, true
}

func (h *host) Each(fn func(world.Member)) {
	for _, m := range h.members {
		fn(m)
	}
}

func (h *host) Seat(uint32, uint32) bool { return false }
func (h *host) Unseat(uint32)            {}

func (h *host) Pin(a, b uint32, on bool) {
	if a > b {
		a, b = b, a
	}
	if on {
		h.pins[[2]uint32{a, b}] = true
	} else {
		delete(h.pins, [2]uint32{a, b})
	}
}

func (h *host) Send(id uint32, frame []byte) {
	var m map[string]any
	if err := json.Unmarshal(frame, &m); err != nil {
		panic(err)
	}
	h.inbox[id] = append(h.inbox[id], m)
}

func (h *host) Do(_ string, job func(context.Context) error, done func(error)) {
	done(job(context.Background()))
}
func (h *host) Publish(ev events.Event)     { h.events = append(h.events, ev) }
func (h *host) Now() time.Time              { return h.now }
func (h *host) Kick(id uint32, code string) { delete(h.members, id) }

// last is the latest message of type t a player got (nil for none), and clears the inbox.
func (h *host) last(id uint32, t string) map[string]any {
	var out map[string]any
	for _, m := range h.inbox[id] {
		if m["t"] == t {
			out = m
		}
	}
	h.inbox[id] = nil
	return out
}

const (
	traveler = 1
	citizenA = 2
	citizenB = 3
	driver   = 4
)

func setup(t *testing.T) (*host, *directions.Service) {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	h := &host{now: time.Date(2026, 11, 7, 5, 0, 0, 0, time.UTC), members: map[uint32]*member{}, inbox: map[uint32][]map[string]any{},
		pins: map[[2]uint32]bool{}}
	h.members[traveler] = &member{id: traveler, char: "t1", name: "Lan", caps: capability.Of(content.CapAskWay)}
	h.members[citizenA] = &member{id: citizenA, char: "c1", name: "Minh", caps: capability.Of(content.CapGiveWay), pose: protocol.Pose{X: 30}}
	h.members[citizenB] = &member{id: citizenB, char: "c2", name: "Tú", caps: capability.Of(content.CapGiveWay), pose: protocol.Pose{X: -40}}
	h.members[driver] = &member{id: driver, char: "d1", name: "Tài", caps: capability.Of("ridehail.drive")}
	return h, directions.New(h, c, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func send(s *directions.Service, h *host, id uint32, msg protocol.ClientMsg) {
	if !s.Handle(h.members[id], msg) {
		panic("not the module's message: " + msg.T)
	}
}

func ask(s *directions.Service, h *host, dest string) map[string]any {
	send(s, h, traveler, protocol.ClientMsg{T: protocol.TypeWayAsk, Dest: dest})
	return h.last(traveler, "way")
}

func kinds(evs []events.Event) map[events.Kind]events.Event {
	out := map[events.Kind]events.Event{}
	for _, e := range evs {
		out[e.Kind] = e
	}
	return out
}

func TestTheFirstCitizenToWalkUpAnswersAndBothGetCredit(t *testing.T) {
	h, s := setup(t)
	way := ask(s, h, "nha_hat_thanh_pho")
	if way["state"] != directions.StateAsking || way["id"] == "" {
		t.Fatalf("the asker heard %v", way)
	}
	id := way["id"].(string)
	callA, callB := h.last(citizenA, "way_call"), h.last(citizenB, "way_call")
	if callA == nil || callB == nil || h.last(driver, "way_call") != nil {
		t.Fatalf("calls: A %v, B %v (the driver must get none)", callA, callB)
	}
	if !h.pins[[2]uint32{traveler, citizenA}] || !h.pins[[2]uint32{traveler, citizenB}] {
		t.Fatalf("the asker is not kept in the Citizens' view: %v", h.pins)
	}
	// from 30 m away: too far
	send(s, h, citizenA, protocol.ClientMsg{T: protocol.TypeWayAnswer, Ask: id})
	if e := h.last(citizenA, "way_error"); e == nil || e["code"] != directions.ErrTooFar {
		t.Fatalf("answering from afar: %v", e)
	}
	h.members[citizenA].pose = protocol.Pose{X: 2.5}
	send(s, h, citizenA, protocol.ClientMsg{T: protocol.TypeWayAnswer, Ask: id})
	if d := h.last(citizenA, "way_done"); d == nil {
		t.Fatal("the helper got no thanks")
	}
	if w := h.last(traveler, "way"); w["state"] != directions.StateAnswered || w["helper"].(map[string]any)["name"] != "Minh" {
		t.Fatalf("the asker heard %v", w)
	}
	if g := h.last(citizenB, "way_gone"); g == nil || g["why"] != "taken" {
		t.Fatalf("the other Citizen's call: %v", g)
	}
	if len(h.pins) != 0 {
		t.Fatalf("pins left: %v", h.pins)
	}
	ev := kinds(h.events)
	if a := ev[events.DirectionsAnswered]; a.Character != "t1" || a.Partners[0] != "c1" || a.Place != "nha_hat_thanh_pho" {
		t.Fatalf("answered event %+v", a)
	}
	if g := ev[events.DirectionsGiven]; g.Character != "c1" || g.Partners[0] != "t1" {
		t.Fatalf("given event %+v", g)
	}
	// the ask is over: answering again finds nothing
	send(s, h, citizenB, protocol.ClientMsg{T: protocol.TypeWayAnswer, Ask: id})
	if e := h.last(citizenB, "way_error"); e == nil || e["code"] != directions.ErrGone {
		t.Fatalf("a late answer: %v", e)
	}
}

func TestWithNobodyToAskTheFallbackOpensAtOnce(t *testing.T) {
	h, s := setup(t)
	delete(h.members, citizenA)
	delete(h.members, citizenB)
	way := ask(s, h, "bitexco")
	if way["state"] != directions.StateUnanswered || way["why"] != "nobody" {
		t.Fatalf("asking an empty town: %v", way)
	}
	if e := kinds(h.events)[events.DirectionsUnanswered]; e.Character != "t1" {
		t.Fatalf("no unanswered event: %+v", h.events)
	}
}

func TestAnAskNobodyAnswersRunsOut(t *testing.T) {
	h, s := setup(t)
	ask(s, h, "bitexco")
	h.now = h.now.Add(179 * time.Second)
	s.Tick(h.now)
	if len(h.events) != 0 {
		t.Fatal("ran out early")
	}
	h.now = h.now.Add(2 * time.Second)
	s.Tick(h.now)
	if w := h.last(traveler, "way"); w["state"] != directions.StateUnanswered || w["why"] != "no_answer" {
		t.Fatalf("the asker heard %v", w)
	}
	if g := h.last(citizenA, "way_gone"); g == nil || g["why"] != "expired" {
		t.Fatalf("the call: %v", g)
	}
	if kinds(h.events)[events.DirectionsUnanswered].Character != "t1" || len(h.pins) != 0 {
		t.Fatalf("events %+v, pins %v", h.events, h.pins)
	}
}

func TestWhoMayAskAndAnswer(t *testing.T) {
	h, s := setup(t)
	// a Citizen cannot ask, a guest cannot either, and only landmarks are asked for
	send(s, h, citizenA, protocol.ClientMsg{T: protocol.TypeWayAsk, Dest: "bitexco"})
	if e := h.last(citizenA, "way_error"); e == nil || e["code"] != directions.ErrNotAllowed {
		t.Fatalf("a Citizen asking: %v", e)
	}
	send(s, h, traveler, protocol.ClientMsg{T: protocol.TypeWayAsk, Dest: "duong_le_loi"})
	if e := h.last(traveler, "way_error"); e == nil || e["code"] != directions.ErrUnknownPlace {
		t.Fatalf("asking for a street: %v", e)
	}
	way := ask(s, h, "bitexco")
	send(s, h, traveler, protocol.ClientMsg{T: protocol.TypeWayAsk, Dest: "bitexco"})
	if e := h.last(traveler, "way_error"); e == nil || e["code"] != directions.ErrBusy {
		t.Fatalf("asking twice: %v", e)
	}
	// the driver is near but cannot answer; the asker cannot answer themselves
	h.members[driver].pose = protocol.Pose{X: 1}
	send(s, h, driver, protocol.ClientMsg{T: protocol.TypeWayAnswer, Ask: way["id"].(string)})
	if e := h.last(driver, "way_error"); e == nil || e["code"] != directions.ErrNotAllowed {
		t.Fatalf("a driver answering: %v", e)
	}
	// cancelling withdraws the calls, and asking again straight away is too soon
	send(s, h, traveler, protocol.ClientMsg{T: protocol.TypeWayCancel})
	if w := h.last(traveler, "way"); w["state"] != directions.StateCancelled {
		t.Fatalf("cancel: %v", w)
	}
	if g := h.last(citizenA, "way_gone"); g == nil || g["why"] != "cancelled" {
		t.Fatalf("the call after a cancel: %v", g)
	}
	send(s, h, traveler, protocol.ClientMsg{T: protocol.TypeWayAsk, Dest: "bitexco"})
	if e := h.last(traveler, "way_error"); e == nil || e["code"] != directions.ErrTooSoon {
		t.Fatalf("asking again at once: %v", e)
	}
}

func TestAGuestCannotAsk(t *testing.T) {
	h, s := setup(t)
	h.members[traveler].char = ""
	send(s, h, traveler, protocol.ClientMsg{T: protocol.TypeWayAsk, Dest: "bitexco"})
	if e := h.last(traveler, "way_error"); e == nil || e["code"] != directions.ErrNoAccount {
		t.Fatalf("a guest asking: %v", e)
	}
}

func TestAPairIsCreditedOnceADay(t *testing.T) {
	h, s := setup(t)
	h.members[citizenA].pose = protocol.Pose{X: 1}
	answer := func() map[string]any {
		way := ask(s, h, "bitexco")
		send(s, h, citizenA, protocol.ClientMsg{T: protocol.TypeWayAnswer, Ask: way["id"].(string)})
		return h.last(citizenA, "way_error")
	}
	if e := answer(); e != nil {
		t.Fatalf("first answer: %v", e)
	}
	h.now = h.now.Add(time.Minute)
	if e := answer(); e == nil || e["code"] != directions.ErrAlreadyHelped {
		t.Fatalf("the same pair again today: %v", e)
	}
	send(s, h, traveler, protocol.ClientMsg{T: protocol.TypeWayCancel})
	h.now = h.now.Add(24 * time.Hour)
	if e := answer(); e != nil {
		t.Fatalf("the next day: %v", e)
	}
}

func TestLeavingEndsTheAskAndLateCitizensAreCalled(t *testing.T) {
	h, s := setup(t)
	newcomer := &member{id: 9, char: "c9", name: "Hoa", caps: capability.Of(content.CapGiveWay)}
	ask(s, h, "bitexco")
	h.members[9] = newcomer
	h.now = h.now.Add(1500 * time.Millisecond)
	s.Tick(h.now)
	if h.last(9, "way_call") == nil {
		t.Fatal("a Citizen who came online during the ask was not called")
	}
	s.Left(h.members[traveler])
	for _, id := range []uint32{citizenA, citizenB, 9} {
		if g := h.last(id, "way_gone"); g == nil || g["why"] != "left" {
			t.Fatalf("player %d after the asker left: %v", id, g)
		}
	}
	if len(h.pins) != 0 {
		t.Fatalf("pins left: %v", h.pins)
	}
}
