package ridehail_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/ridehail"
	"saigon-noir/server/internal/store"
	"saigon-noir/server/internal/world"
)

// The module runs against a fake world (members it can move by hand, jobs it runs when told) and the real store,
// so every path is checked against the ledger.

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

const (
	lan  = uint32(1) // the passenger, at the pickup
	tai  = uint32(2) // drivers on their bikes nearby
	hoa  = uint32(3)
	minh = uint32(4)
)

const (
	start = int64(100_000)
	// the trip: from (0, 0) to Chợ Bến Thành 100 m away, 10.000 + 60 × 100 = 16.000; the driver gets 80 %
	fare = int64(16_000)
	paid = int64(12_800)
)

type member struct {
	id         uint32
	char, name string
	caps       capability.Set
	pose       protocol.Pose
	speed      float64
	vehicle    uint32
	seat       int
	own        bool
}

func (m *member) ID() uint32                  { return m.id }
func (m *member) Character() string           { return m.char }
func (m *member) Name() string                { return m.name }
func (m *member) Arch() string                { return "" }
func (m *member) Caps() capability.Set        { return m.caps }
func (m *member) Pose() protocol.Pose         { return m.pose }
func (m *member) Speed() float64              { return m.speed }
func (m *member) Riding() (uint32, int, bool) { return m.vehicle, m.seat, m.own }
func (m *member) MutedUntil() time.Time       { return time.Time{} }

type host struct {
	now     time.Time
	members map[uint32]*member
	inbox   map[uint32][]map[string]any
	jobs    []func()
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

func (h *host) Seat(p, r uint32) bool {
	pm, rm := h.members[p], h.members[r]
	if pm == nil || rm == nil || rm.vehicle == 0 || rm.seat != 0 || pm.vehicle != 0 {
		return false
	}
	pm.vehicle, pm.seat, pm.own = rm.vehicle, 1, false
	return true
}

func (h *host) Unseat(p uint32) {
	if pm := h.members[p]; pm != nil && pm.seat == 1 {
		pm.vehicle, pm.seat = 0, 0
	}
}

func pairOf(a, b uint32) [2]uint32 { return [2]uint32{min(a, b), max(a, b)} }

func (h *host) Pin(a, b uint32, on bool) {
	if on {
		h.pins[pairOf(a, b)] = true
	} else {
		delete(h.pins, pairOf(a, b))
	}
}

func (h *host) Send(id uint32, frame []byte) {
	if _, ok := h.members[id]; !ok {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(frame, &m); err != nil {
		panic(err)
	}
	h.inbox[id] = append(h.inbox[id], m)
}

func (h *host) Do(_ string, job func(context.Context) error, done func(error)) {
	h.jobs = append(h.jobs, func() { done(job(context.Background())) })
}

func (h *host) Publish(ev events.Event)     { h.events = append(h.events, ev) }
func (h *host) Now() time.Time              { return h.now }
func (h *host) Kick(id uint32, code string) { delete(h.members, id) }

type fixture struct {
	*host
	t   *testing.T
	s   *ridehail.Service
	db  *store.DB
	raw *sql.DB
}

var dest = content.Place{ID: "cho_ben_thanh", Name: "Chợ Bến Thành", Kind: content.Landmark, Pts: [][]float64{{100, 0}}, R: 10}

func config() ridehail.Config {
	return ridehail.ConfigFrom(content.Ridehail{StartBalance: start, TopupTo: 50_000, Fare: content.Fare{Base: 10_000, PerMetre: 60, Round: 1000},
		CutPercent: 20, Tips: []int64{0, 5000, 10_000}, PairLimit: 5, OfferRadius: 150, OfferSeconds: 15, RequestSecs: 60,
		PickupSeconds: 300, TripSeconds: 300, RatingSeconds: 120, BoardReach: 3, StoppedSpeed: 0.5})
}

func newFixture(t *testing.T, cfg ridehail.Config) *fixture {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "game.db")
	db, err := store.Open(ctx, store.Options{Path: path}, quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	h := &host{now: time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC), members: map[uint32]*member{}, inbox: map[uint32][]map[string]any{},
		pins: map[[2]uint32]bool{}}
	purse := ridehail.Purse{Wallets: db, Cfg: cfg.Ridehail, Now: func() time.Time { return h.now }}
	drive := capability.Set{ridehail.DriveCapability: {}}
	for i, name := range []string{"Lan", "Tài", "Hoa", "Minh"} {
		id := uint32(i + 1)
		c := identity.Character{ID: "char" + name, AccountID: "acc" + name, Name: name, CreatedAt: 1}
		if err := db.CreateAccount(ctx, c.AccountID, []byte("hash"+name), 1); err != nil {
			t.Fatal(err)
		}
		if err := db.CreateCharacter(ctx, c); err != nil {
			t.Fatal(err)
		}
		if _, err := purse.Arrive(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
		m := &member{id: id, char: c.ID, name: name}
		if id != lan {
			// a driver a few metres away, on their own bike
			m.caps, m.pose, m.vehicle, m.own = drive, protocol.Pose{X: float64(5 * i)}, 100+id, true
		}
		h.members[id] = m
	}
	f := &fixture{host: h, t: t, db: db, raw: raw, s: ridehail.New(h, cfg, []content.Place{dest}, db, quiet)}
	return f
}

func (f *fixture) send(id uint32, msg protocol.ClientMsg) {
	f.t.Helper()
	if !f.s.Handle(f.members[id], msg) {
		f.t.Fatalf("%s not handled", msg.T)
	}
	f.settle()
}

// settle runs the queued jobs, and the ones their answers queue, as the world loop would.
func (f *fixture) settle() {
	for len(f.jobs) > 0 {
		j := f.jobs[0]
		f.jobs = f.jobs[1:]
		j()
	}
}

func (f *fixture) wait(d time.Duration) {
	f.now = f.now.Add(d)
	f.s.Tick(f.now)
	f.settle()
}

func (f *fixture) leave(id uint32) {
	f.s.Left(f.members[id])
	delete(f.members, id)
	f.settle()
}

// last is the newest message of type t a member got (nil for none); clear forgets what it got.
func (f *fixture) last(id uint32, t string) map[string]any {
	in := f.inbox[id]
	for i := len(in) - 1; i >= 0; i-- {
		if in[i]["t"] == t {
			return in[i]
		}
	}
	return nil
}

func (f *fixture) clear() { clear(f.inbox) }

func (f *fixture) onDuty(ids ...uint32) {
	f.t.Helper()
	for _, id := range ids {
		f.send(id, protocol.ClientMsg{T: protocol.TypeDuty, On: true})
		if d := f.last(id, "duty"); d == nil || d["on"] != true {
			f.t.Fatalf("driver %d is not on duty: %v", id, f.inbox[id])
		}
	}
}

// book requests a ride for Lan and returns the offer each driver got.
func (f *fixture) book() map[uint32]string {
	f.t.Helper()
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
	r := f.last(lan, "ride")
	if r == nil || r["state"] != ridehail.StateRequested || int64(r["fare"].(float64)) != fare {
		f.t.Fatalf("booking: %v", f.inbox[lan])
	}
	offers := map[uint32]string{}
	for id := range f.members {
		if o := f.last(id, "ride_offer"); o != nil {
			offers[id] = o["offer"].(string)
		}
	}
	return offers
}

func (f *fixture) accept(driver uint32, offer string) {
	f.t.Helper()
	f.send(driver, protocol.ClientMsg{T: protocol.TypeRideAccept, Offer: offer})
	if r := f.last(driver, "ride"); r == nil || r["state"] != ridehail.StateAccepted {
		f.t.Fatalf("accept: %v", f.inbox[driver])
	}
}

// pickUp has the driver come to Lan, stop, and Lan get on.
func (f *fixture) pickUp(driver uint32) {
	f.t.Helper()
	d := f.members[driver]
	d.pose, d.speed = protocol.Pose{X: 1}, 0
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideBoard})
	if r := f.last(lan, "ride"); r["state"] != ridehail.StateOnTrip {
		f.t.Fatalf("boarding: %v", f.inbox[lan])
	}
}

// rideTo moves the driver (and Lan behind them) to x on the way.
func (f *fixture) rideTo(driver uint32, x float64) {
	f.members[driver].pose = protocol.Pose{X: x}
	f.members[lan].pose = protocol.Pose{X: x}
}

func (f *fixture) balance(id string) int64 {
	f.t.Helper()
	var b int64
	if err := f.raw.QueryRow(`select balance from wallets where character_id = ?`, id).Scan(&b); err != nil {
		f.t.Fatal(err)
	}
	var sum int64
	if err := f.raw.QueryRow(`select coalesce(sum(delta), 0) from ledger where character_id = ?`, id).Scan(&sum); err != nil {
		f.t.Fatal(err)
	}
	if sum != b {
		f.t.Fatalf("%s: the wallet holds %d, the ledger adds up to %d", id, b, sum)
	}
	return b
}

func (f *fixture) rideRow() (state, end string, paid int64) {
	f.t.Helper()
	if err := f.raw.QueryRow(`select state, coalesce(end_reason, ''), coalesce(paid, 0) from rides`).Scan(&state, &end, &paid); err != nil {
		f.t.Fatal(err)
	}
	return state, end, paid
}

func (f *fixture) char(id uint32) string {
	return "char" + map[uint32]string{lan: "Lan", tai: "Tài", hoa: "Hoa", minh: "Minh"}[id]
}

func TestATripMovesTheFareOnceAndCountsForBoth(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai)
	offers := f.book()
	if f.balance(f.char(lan)) != start-fare {
		t.Fatalf("the fare is held: %d", f.balance(f.char(lan)))
	}
	f.accept(tai, offers[tai])
	if !f.pins[pairOf(lan, tai)] {
		t.Fatal("the passenger and the driver are pinned in each other's view")
	}
	if r := f.last(lan, "ride"); r["driver"].(map[string]any)["name"] != "Tài" {
		t.Fatalf("the passenger sees the driver: %v", r)
	}

	// too far, then not stopped, then on
	f.members[tai].pose = protocol.Pose{X: 10}
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideBoard})
	if e := f.last(lan, "ride_error"); e == nil || e["code"] != ridehail.ErrTooFar {
		t.Fatalf("boarding from 10 m: %v", e)
	}
	f.members[tai].pose, f.members[tai].speed = protocol.Pose{X: 1}, 3
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideBoard})
	if e := f.last(lan, "ride_error"); e["code"] != ridehail.ErrNotStopped {
		t.Fatalf("boarding a moving bike: %v", e)
	}
	f.pickUp(tai)
	if v, seat, _ := f.members[lan].Riding(); v != f.members[tai].vehicle || seat != 1 {
		t.Fatalf("Lan sits behind Tài: vehicle %d seat %d", v, seat)
	}

	// not there yet, then there
	f.rideTo(tai, 50)
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideComplete})
	if e := f.last(tai, "ride_error"); e["code"] != ridehail.ErrNotThere {
		t.Fatalf("completing halfway: %v", e)
	}
	f.rideTo(tai, 95)
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideComplete})
	if r := f.last(tai, "ride"); r["state"] != ridehail.StateCompleted || int64(r["paid"].(float64)) != paid {
		t.Fatalf("the driver hears the trip completed: %v", r)
	}
	if v, _, _ := f.members[lan].Riding(); v != 0 {
		t.Fatal("Lan got off at the end")
	}
	if f.pins[pairOf(lan, tai)] {
		t.Fatal("the pin ends with the trip")
	}
	if f.balance(f.char(lan)) != start-fare || f.balance(f.char(tai)) != start+paid {
		t.Fatalf("after the trip: Lan %d, Tài %d", f.balance(f.char(lan)), f.balance(f.char(tai)))
	}
	if state, end, p := f.rideRow(); state != ridehail.StateCompleted || end != ridehail.StateCompleted || p != paid {
		t.Fatalf("the ride row: %s %s %d", state, end, p)
	}
	roles := map[string]string{}
	for _, ev := range f.events {
		if ev.Kind == events.TripCompleted {
			roles[ev.Character] = ev.Role
		}
	}
	if roles[f.char(tai)] != ridehail.RoleRider || roles[f.char(lan)] != ridehail.RolePassenger || len(roles) != 2 {
		t.Fatalf("trip.completed: %v", roles)
	}

	// five stars and a tip, once
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRate, Stars: 5, Tip: 5000})
	if f.last(lan, "ride_rated") == nil {
		t.Fatalf("rating: %v", f.inbox[lan])
	}
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRate, Stars: 1, Tip: 10_000})
	if f.balance(f.char(lan)) != start-fare-5000 || f.balance(f.char(tai)) != start+paid+5000 {
		t.Fatalf("after the tip: Lan %d, Tài %d", f.balance(f.char(lan)), f.balance(f.char(tai)))
	}
	trips, stars, err := f.db.DriverStats(context.Background(), f.char(tai))
	if err != nil || trips != 1 || stars != 5 {
		t.Fatalf("Tài's stats: %d trips, %.1f stars, %v", trips, stars, err)
	}
}

func TestEveryCancelAndTimeoutReturnsTheHold(t *testing.T) {
	cfg := config()
	cases := []struct {
		name, state, end string
		run              func(f *fixture, offers map[uint32]string)
	}{
		{"the passenger cancels the request", ridehail.StateCancelled, "cancelled_by_passenger", func(f *fixture, _ map[uint32]string) {
			f.send(lan, protocol.ClientMsg{T: protocol.TypeRideCancel})
		}},
		{"the passenger cancels with the driver on the way", ridehail.StateCancelled, "cancelled_by_passenger", func(f *fixture, o map[uint32]string) {
			f.accept(tai, o[tai])
			f.send(lan, protocol.ClientMsg{T: protocol.TypeRideCancel})
		}},
		{"no driver accepts", ridehail.StateExpired, "no_driver", func(f *fixture, _ map[uint32]string) {
			f.wait(cfg.Request + time.Second)
		}},
		{"the driver never comes", ridehail.StateCancelled, "pickup_timeout", func(f *fixture, o map[uint32]string) {
			f.accept(tai, o[tai])
			f.wait(cfg.Pickup + time.Second)
		}},
		{"the passenger leaves before pickup", ridehail.StateCancelled, "passenger_left", func(f *fixture, o map[uint32]string) {
			f.accept(tai, o[tai])
			f.leave(lan)
		}},
		{"the driver leaves on the way", ridehail.StateEnded, "driver_left", func(f *fixture, o map[uint32]string) {
			f.accept(tai, o[tai])
			f.pickUp(tai)
			f.rideTo(tai, 60)
			f.leave(tai)
		}},
		{"the driver gets off on the way", ridehail.StateEnded, "driver_got_off", func(f *fixture, o map[uint32]string) {
			f.accept(tai, o[tai])
			f.pickUp(tai)
			f.rideTo(tai, 60)
			f.members[tai].vehicle = 0
			f.members[lan].vehicle, f.members[lan].seat = 0, 0 // the world puts the passenger down with the bike
			f.wait(100 * time.Millisecond)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, cfg)
			f.onDuty(tai)
			c.run(f, f.book())
			if state, end, p := f.rideRow(); state != c.state || end != c.end || p != 0 {
				t.Fatalf("the ride ended %s (%s), paid %d", state, end, p)
			}
			if f.balance(f.char(lan)) != start || f.balance(f.char(tai)) != start {
				t.Fatalf("Lan %d, Tài %d", f.balance(f.char(lan)), f.balance(f.char(tai)))
			}
			if len(f.pins) != 0 {
				t.Fatalf("pins left: %v", f.pins)
			}
			if _, ok := f.members[lan]; ok {
				if v, _, _ := f.members[lan].Riding(); v != 0 {
					t.Fatal("Lan is still on the bike")
				}
				// free to book again
				f.now = f.now.Add(cfg.RequestGap)
				f.clear()
				f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
				if r := f.last(lan, "ride"); r == nil || r["state"] != ridehail.StateRequested {
					t.Fatalf("booking again: %v", f.inbox[lan])
				}
			}
		})
	}
}

func TestCancellingWhileTheFareIsTakenRefundsIt(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai)
	f.s.Handle(f.members[lan], protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
	f.s.Handle(f.members[lan], protocol.ClientMsg{T: protocol.TypeRideCancel}) // before the hold came back
	f.settle()
	if f.last(tai, "ride_offer") != nil {
		t.Fatal("a cancelled request is not offered")
	}
	if state, end, _ := f.rideRow(); state != ridehail.StateCancelled || end != "cancelled_by_passenger" || f.balance(f.char(lan)) != start {
		t.Fatalf("%s %s, Lan %d", state, end, f.balance(f.char(lan)))
	}
}

func TestTheFirstAcceptWins(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai, hoa, minh)
	offers := f.book()
	if len(offers) != 3 {
		t.Fatalf("every driver nearby gets the offer: %v", offers)
	}
	// all three press accept; the loop reads them one at a time
	for _, d := range []uint32{hoa, tai, minh} {
		f.s.Handle(f.members[d], protocol.ClientMsg{T: protocol.TypeRideAccept, Offer: offers[d]})
	}
	f.settle()
	if r := f.last(hoa, "ride"); r == nil || r["state"] != ridehail.StateAccepted {
		t.Fatalf("Hoa was first: %v", f.inbox[hoa])
	}
	for _, d := range []uint32{tai, minh} {
		if g := f.last(d, "ride_offer_gone"); g == nil || g["why"] != "taken" {
			t.Fatalf("driver %d: the offer is gone: %v", d, f.inbox[d])
		}
		if e := f.last(d, "ride_error"); e == nil || e["code"] != ridehail.ErrGone {
			t.Fatalf("driver %d: a late accept fails: %v", d, f.inbox[d])
		}
		if f.last(d, "ride") != nil {
			t.Fatalf("driver %d got the trip too", d)
		}
	}
	if r := f.last(lan, "ride"); r["driver"].(map[string]any)["name"] != "Hoa" {
		t.Fatalf("Lan's driver: %v", r)
	}
}

func TestADriverWhoCancelsLetsTheOthersTakeIt(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai)
	offers := f.book()
	f.accept(tai, offers[tai])
	f.onDuty(hoa) // on duty while the trip is taken: no offer yet
	if f.last(hoa, "ride_offer") != nil {
		t.Fatal("a taken trip is not offered")
	}
	f.clear()
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideCancel})
	if r := f.last(tai, "ride"); r == nil || r["end"] != "cancelled_by_driver" {
		t.Fatalf("Tài dropped the trip: %v", f.inbox[tai])
	}
	if r := f.last(lan, "ride"); r["state"] != ridehail.StateRequested || r["driver"] != nil {
		t.Fatalf("Lan is looking again: %v", r)
	}
	o := f.last(hoa, "ride_offer")
	if o == nil || f.last(tai, "ride_offer") != nil {
		t.Fatalf("offered to Hoa, not to Tài again: Hoa %v, Tài %v", o, f.last(tai, "ride_offer"))
	}
	f.accept(hoa, o["offer"].(string))
	if f.balance(f.char(lan)) != start-fare {
		t.Fatal("the fare stays held")
	}
}

func TestGettingOffEarlyPaysTheWholeFare(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai)
	f.accept(tai, f.book()[tai])
	f.pickUp(tai)
	f.rideTo(tai, 30)
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideAlight})
	if state, end, p := f.rideRow(); state != ridehail.StateCompleted || end != "alighted" || p != paid {
		t.Fatalf("%s %s %d", state, end, p)
	}
	if f.balance(f.char(lan)) != start-fare || f.balance(f.char(tai)) != start+paid {
		t.Fatalf("Lan %d, Tài %d", f.balance(f.char(lan)), f.balance(f.char(tai)))
	}
	for _, ev := range f.events {
		if ev.Kind == events.TripCompleted {
			t.Fatalf("a trip cut short does not count for quests: %v", ev)
		}
	}
}

func TestLeavingOnTheTripPaysTheWayCovered(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai)
	f.accept(tai, f.book()[tai])
	f.pickUp(tai) // boards at x = 1, 99 m from the market
	f.rideTo(tai, 50)
	f.leave(lan)
	// half the way: half of 16.000, rounded to 1.000, at least the base fare
	charged := int64(10_000)
	if state, end, p := f.rideRow(); state != ridehail.StateEnded || end != "passenger_left" || p != charged*80/100 {
		t.Fatalf("%s %s %d", state, end, p)
	}
	if f.balance(f.char(lan)) != start-charged || f.balance(f.char(tai)) != start+charged*80/100 {
		t.Fatalf("Lan %d, Tài %d", f.balance(f.char(lan)), f.balance(f.char(tai)))
	}
}

func TestATooDearFareIsRefused(t *testing.T) {
	f := newFixture(t, config())
	if _, err := f.raw.Exec(`update wallets set balance = 5000 where character_id = ?`, f.char(lan)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.raw.Exec(`update ledger set delta = 5000 where character_id = ?`, f.char(lan)); err != nil {
		t.Fatal(err)
	}
	f.onDuty(tai)
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
	if e := f.last(lan, "ride_error"); e == nil || e["code"] != ridehail.ErrBalanceLow {
		t.Fatalf("%v", f.inbox[lan])
	}
	if f.last(tai, "ride_offer") != nil || f.balance(f.char(lan)) != 5000 {
		t.Fatal("nothing was offered or taken")
	}
}

func TestBookingsAreSpacedOut(t *testing.T) {
	cfg := config()
	f := newFixture(t, cfg)
	f.book()
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideCancel})
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
	if e := f.last(lan, "ride_error"); e == nil || e["code"] != ridehail.ErrTooSoon {
		t.Fatalf("booking again at once: %v", f.inbox[lan])
	}
	f.wait(cfg.RequestGap)
	f.clear()
	f.book()
}

func TestDutyNeedsTheCapabilityAndTheBike(t *testing.T) {
	f := newFixture(t, config())
	f.members[hoa].caps = nil
	f.send(hoa, protocol.ClientMsg{T: protocol.TypeDuty, On: true})
	if e := f.last(hoa, "ride_error"); e == nil || e["code"] != ridehail.ErrNotAllowed {
		t.Fatalf("without the capability: %v", f.inbox[hoa])
	}
	f.members[tai].vehicle = 0
	f.send(tai, protocol.ClientMsg{T: protocol.TypeDuty, On: true})
	if e := f.last(tai, "ride_error"); e == nil || e["code"] != ridehail.ErrNoBike {
		t.Fatalf("on foot: %v", f.inbox[tai])
	}
	f.members[tai].vehicle = 102
	f.onDuty(tai)
	// getting off the bike ends duty, and a request is no longer offered
	f.members[tai].vehicle = 0
	f.wait(100 * time.Millisecond)
	if d := f.last(tai, "duty"); d["on"] != false {
		t.Fatalf("off the bike: %v", d)
	}
	f.book()
	if f.last(tai, "ride_offer") != nil {
		t.Fatal("a driver off duty gets no offer")
	}
}

func TestOffersStopAtTheRadiusAndThePairLimit(t *testing.T) {
	cfg := config()
	cfg.PairLimit = 1
	f := newFixture(t, cfg)
	f.members[minh].pose = protocol.Pose{X: 200} // past 150 m
	f.onDuty(tai, minh)
	offers := f.book()
	if _, ok := offers[minh]; ok || offers[tai] == "" {
		t.Fatalf("offers: %v", offers)
	}
	f.accept(tai, offers[tai])
	f.pickUp(tai)
	f.rideTo(tai, 95)
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideComplete})
	f.members[tai].pose, f.members[lan].pose = protocol.Pose{X: 5}, protocol.Pose{}
	f.now = f.now.Add(cfg.RequestGap)
	f.clear()
	f.book()
	if f.last(tai, "ride_offer") != nil {
		t.Fatal("past the pair limit, Tài is not offered Lan again today")
	}
	f.wait(24 * time.Hour) // the next day (the request expires meanwhile)
	f.clear()
	f.book()
	if f.last(tai, "ride_offer") == nil {
		t.Fatal("the pair limit is per day")
	}
}

// flakyBank fails the first few settlements, as a busy database would.
type flakyBank struct {
	*store.DB
	fails int
}

func (b *flakyBank) Settle(ctx context.Context, s ridehail.Settlement) (int64, int64, error) {
	if b.fails > 0 {
		b.fails--
		return 0, 0, errors.New("database is locked")
	}
	return b.DB.Settle(ctx, s)
}

func TestAFailedSettlementIsTriedAgainUntilItIsWritten(t *testing.T) {
	f := newFixture(t, config())
	f.s = ridehail.New(f.host, config(), []content.Place{dest}, &flakyBank{DB: f.db, fails: 2}, quiet)
	f.onDuty(tai)
	f.accept(tai, f.book()[tai])
	f.pickUp(tai)
	f.rideTo(tai, 95)
	f.clear()
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideComplete})
	if f.last(tai, "ride") != nil || f.balance(f.char(tai)) != start {
		t.Fatal("the driver heard of a settlement that was not written")
	}
	f.now = f.now.Add(config().RequestGap)
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
	if e := f.last(lan, "ride_error"); e == nil || e["code"] != ridehail.ErrBusy {
		t.Fatalf("booking again before the last trip is paid: %v", f.inbox[lan])
	}
	f.wait(3 * time.Second) // the second try fails too
	f.wait(5 * time.Second) // the third goes through
	if r := f.last(tai, "ride"); r == nil || r["state"] != ridehail.StateCompleted {
		t.Fatalf("the driver hears once it is written: %v", f.inbox[tai])
	}
	if f.balance(f.char(lan)) != start-fare || f.balance(f.char(tai)) != start+paid {
		t.Fatalf("Lan %d, Tài %d", f.balance(f.char(lan)), f.balance(f.char(tai)))
	}
}

func TestNoTripToWhereYouAre(t *testing.T) {
	f := newFixture(t, config())
	f.members[lan].pose = protocol.Pose{X: 95}
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
	if e := f.last(lan, "ride_error"); e == nil || e["code"] != ridehail.ErrHere {
		t.Fatalf("booking the market from inside it: %v", f.inbox[lan])
	}
	// booked from outside, then walked there while waiting: boarding there is refused
	f.members[lan].pose = protocol.Pose{}
	f.onDuty(tai)
	f.accept(tai, f.book()[tai])
	f.members[lan].pose, f.members[tai].pose = protocol.Pose{X: 95}, protocol.Pose{X: 96}
	f.send(lan, protocol.ClientMsg{T: protocol.TypeRideBoard})
	if e := f.last(lan, "ride_error"); e == nil || e["code"] != ridehail.ErrHere {
		t.Fatalf("boarding at the destination: %v", f.inbox[lan])
	}
}

func TestATripThatGoesOnForeverIsRefunded(t *testing.T) {
	cfg := config()
	f := newFixture(t, cfg)
	f.onDuty(tai)
	f.accept(tai, f.book()[tai])
	f.pickUp(tai)
	f.rideTo(tai, 40)
	f.wait(cfg.Trip + time.Second)
	if state, end, p := f.rideRow(); state != ridehail.StateEnded || end != "trip_timeout" || p != 0 {
		t.Fatalf("%s %s %d", state, end, p)
	}
	if f.balance(f.char(lan)) != start || f.balance(f.char(tai)) != start {
		t.Fatalf("Lan %d, Tài %d", f.balance(f.char(lan)), f.balance(f.char(tai)))
	}
	if v, _, _ := f.members[lan].Riding(); v != 0 {
		t.Fatal("Lan is still on the bike")
	}
}

func TestADriverWithATripGetsNoOtherOffers(t *testing.T) {
	f := newFixture(t, config())
	f.members[hoa].vehicle = 0 // Hoa books a ride too, on foot
	f.onDuty(tai)
	offers := f.book()
	f.send(hoa, protocol.ClientMsg{T: protocol.TypeRideRequest, Dest: dest.ID})
	other := f.last(tai, "ride_offer")["offer"].(string)
	if other == offers[tai] {
		t.Fatal("Tài got Hoa's request as a second offer")
	}
	f.accept(tai, offers[tai])
	if g := f.last(tai, "ride_offer_gone"); g == nil || g["offer"] != other || g["why"] != "busy" {
		t.Fatalf("Hoa's offer goes from Tài: %v", g)
	}
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideAccept, Offer: other})
	if e := f.last(tai, "ride_error"); e == nil || e["code"] != ridehail.ErrGone {
		t.Fatalf("taking a second trip: %v", f.inbox[tai])
	}
}

func TestLeavingTheSeatSomeOtherWayIsGettingOffEarly(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai)
	f.accept(tai, f.book()[tai])
	f.pickUp(tai)
	f.rideTo(tai, 30)
	// a respawn: the world puts the passenger down and sends them home
	f.members[lan].vehicle, f.members[lan].seat, f.members[lan].pose = 0, 0, protocol.Pose{X: -20}
	f.wait(100 * time.Millisecond)
	if state, end, p := f.rideRow(); state != ridehail.StateEnded || end != "passenger_got_off" || p != paid {
		t.Fatalf("%s %s %d", state, end, p)
	}
	if f.balance(f.char(lan)) != start-fare {
		t.Fatalf("Lan pays the whole fare: %d", f.balance(f.char(lan)))
	}
}

func TestATripHoldsBothArchetypesUntilItsMoneyMoved(t *testing.T) {
	f := newFixture(t, config())
	f.onDuty(tai, hoa)
	if f.s.ArchBusy(f.members[lan]) || f.s.ArchBusy(f.members[tai]) {
		t.Fatal("nobody on a trip yet")
	}
	offers := f.book()
	if !f.s.ArchBusy(f.members[lan]) {
		t.Fatal("a passenger who booked must not change archetype")
	}
	f.accept(tai, offers[tai])
	if !f.s.ArchBusy(f.members[tai]) || f.s.ArchBusy(f.members[hoa]) {
		t.Fatal("only the driver who accepted is held")
	}
	f.pickUp(tai)
	f.rideTo(tai, 100)
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideComplete})
	if f.s.ArchBusy(f.members[lan]) || f.s.ArchBusy(f.members[tai]) {
		t.Fatalf("a completed trip holds nobody: %v / %v", f.inbox[lan], f.inbox[tai])
	}
	// a driver who changed archetype is off duty, and told so
	f.clear()
	f.s.ArchChanged(f.members[hoa])
	if d := f.last(hoa, "duty"); d == nil || d["on"] != false {
		t.Fatalf("after the change: %v", f.inbox[hoa])
	}
}
