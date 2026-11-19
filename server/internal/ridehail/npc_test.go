package ridehail_test

import (
	"context"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/ridehail"
)

// NPC passengers: the module runs against the fake world of ridehail_test.go with guests added to it.

// guestHost is the fake world with NPC guests.
type guestHost struct {
	*host
	guests map[uint32]*fakeGuest
	next   uint32
}

type fakeGuest struct {
	key, name string
	x, z      float64
	rider     uint32 // who it sits behind, 0 for standing
	says      []protocol.Text
}

func (g *guestHost) AddGuest(key, name string, x, z, _ float64) uint32 {
	g.next++
	g.guests[1000+g.next] = &fakeGuest{key: key, name: name, x: x, z: z}
	return 1000 + g.next
}

func (g *guestHost) RemoveGuest(id uint32) { delete(g.guests, id) }

func (g *guestHost) SeatGuest(guest, rider uint32) bool {
	n, r := g.guests[guest], g.members[rider]
	if n == nil || r == nil || r.vehicle == 0 || r.seat != 0 || n.rider != 0 {
		return false
	}
	n.rider = rider
	return true
}

func (g *guestHost) UnseatGuest(guest uint32) {
	if n := g.guests[guest]; n != nil {
		n.rider = 0
	}
}

func (g *guestHost) GuestSeated(guest uint32) bool {
	n := g.guests[guest]
	return n != nil && n.rider != 0
}

func (g *guestHost) GuestSays(guest uint32, line protocol.Text) {
	if n := g.guests[guest]; n != nil {
		n.says = append(n.says, line)
	}
}

// the trip: from the spot at x = 40 to Chợ Bến Thành at x = 100, 60 m: 10.000 + 60 × 60 = 13.600, rounded to 14.000
const (
	npcFare = int64(14_000)
	npcPaid = int64(11_200)
	spotX   = 40.0
)

func passengers() content.Passengers {
	line := []content.Text{{VI: "Chào bác tài", EN: "Hello driver"}}
	return content.Passengers{
		Rules: content.PassengerRules{IdleSeconds: 30, CooldownSeconds: 40, RealRadius: 150, PickupMin: 25, PickupMax: 140, TripMin: 50, TripMax: 200,
			BoardReach: 4, PatienceSeconds: 150, DailyTrips: 2, LingerSeconds: 8,
			Rating: content.Rating{WaitOK: 40, WaitBad: 80, DetourOK: 1.6, DetourBad: 2.4, PaceOK: 5, PaceBad: 2.5, ScarySpeed: 17, ScaryShare: 0.35}},
		Cast: []content.Passenger{{ID: "chi_thao", Name: "Chị Thảo", Greet: line, Thanks: []content.Text{{VI: "Cảm ơn", EN: "Thanks"}},
			Grumble: []content.Text{{VI: "Lâu quá", EN: "So slow"}}, Left: []content.Text{{VI: "Đi xe khác", EN: "Another ride"}}}},
		Spots: []content.PassengerSpot{{ID: "cho", Place: "cho_ben_thanh", At: []float64{spotX, 0}, Face: []float64{41, 0}}},
	}
}

type npcFixture struct {
	*fixture
	gh *guestHost
}

func newNPCFixture(t *testing.T) *npcFixture {
	t.Helper()
	f := newFixture(t, config())
	gh := &guestHost{host: f.host, guests: map[uint32]*fakeGuest{}}
	f.s = ridehail.New(gh, config(), []content.Place{dest}, f.db, quiet)
	f.s.UsePassengers(passengers())
	return &npcFixture{fixture: f, gh: gh}
}

// request has a driver on duty wait for the NPC's request, and returns its offer and the NPC's id.
func (f *npcFixture) request(driver uint32) (offer string, npc uint32) {
	f.t.Helper()
	f.clear()
	if !f.dutyOn(driver) {
		f.onDuty(driver)
	}
	for range 40 {
		f.wait(time.Second)
		if o := f.last(driver, "ride_offer"); o != nil {
			p := o["passenger"].(map[string]any)
			if p["npc"] != true || p["name"] != "Chị Thảo" {
				f.t.Fatalf("the offer is not from the NPC: %v", o)
			}
			return o["offer"].(string), uint32(p["id"].(float64))
		}
	}
	f.t.Fatal("no NPC request came")
	return "", 0
}

func (f *npcFixture) dutyOn(id uint32) bool {
	d := f.last(id, "duty")
	return d != nil && d["on"] == true
}

// board accepts the offer, brings the bike to the NPC and waits for it to get on.
func (f *npcFixture) board(driver uint32, offer string) {
	f.t.Helper()
	f.accept(driver, offer)
	f.members[driver].pose, f.members[driver].speed = protocol.Pose{X: spotX - 2}, 0
	f.wait(time.Second)
	if r := f.last(driver, "ride"); r["state"] != ridehail.StateOnTrip {
		f.t.Fatalf("the NPC did not get on: %v", r)
	}
}

// ride takes the NPC to the destination in a few reports and completes the trip.
func (f *npcFixture) ride(driver uint32) {
	f.t.Helper()
	for _, x := range []float64{60, 80, 98} {
		f.members[driver].pose = protocol.Pose{X: x}
		f.members[driver].speed = 10
		f.wait(time.Second)
	}
	f.members[driver].speed = 0
	f.wait(time.Second)
	f.send(driver, protocol.ClientMsg{T: protocol.TypeRideComplete})
}

func (f *npcFixture) npcTrips(driver uint32) (state string, end string) {
	r := f.last(driver, "ride")
	if r == nil {
		return "", ""
	}
	s, _ := r["state"].(string)
	e, _ := r["end"].(string)
	return s, e
}

func (f *npcFixture) ledgerReasons(character string) map[string]int {
	f.t.Helper()
	rows, err := f.raw.Query(`select reason, count(*) from ledger where character_id = ? group by reason`, character)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var r string
		var n int
		if err := rows.Scan(&r, &n); err != nil {
			f.t.Fatal(err)
		}
		out[r] = n
	}
	return out
}

func TestAnIdleDriverGetsOneNPCRequestAtATime(t *testing.T) {
	f := newNPCFixture(t)
	f.onDuty(tai)
	f.wait(20 * time.Second)
	if len(f.gh.guests) != 0 || f.last(tai, "ride_offer") != nil {
		t.Fatal("no request before the driver has been idle for a while")
	}
	offer, npc := f.request(tai)
	g := f.gh.guests[npc]
	if g == nil || g.key != "chi_thao" || g.x != spotX {
		t.Fatalf("the NPC waits at its spot: %+v", g)
	}
	if o := f.last(tai, "ride_offer"); int64(o["fare"].(float64)) != npcFare || o["dest"] != dest.ID {
		t.Fatalf("the offer: %v", o)
	}
	// unanswered, the offer lapses and the NPC leaves; the next waits for the cooldown
	f.wait(16 * time.Second)
	if f.last(tai, "ride_offer_gone") == nil || len(f.gh.guests) != 0 {
		t.Fatalf("the offer lapsed with its NPC: %v, %d guests", f.last(tai, "ride_offer_gone"), len(f.gh.guests))
	}
	f.wait(20 * time.Second)
	if len(f.gh.guests) != 0 {
		t.Fatal("no new request inside the cooldown")
	}
	f.wait(30 * time.Second)
	if len(f.gh.guests) != 1 {
		t.Fatalf("the next request came: %d guests", len(f.gh.guests))
	}
	// one at a time: a driver with an NPC request open, or accepted, gets no second one
	f.accept(tai, f.last(tai, "ride_offer")["offer"].(string))
	f.wait(2 * time.Minute)
	if len(f.gh.guests) != 1 {
		t.Fatalf("one NPC passenger per driver: %d", len(f.gh.guests))
	}
	_ = offer
}

func TestNoNPCRequestForADriverOffDuty(t *testing.T) {
	f := newNPCFixture(t)
	f.wait(3 * time.Minute)
	if len(f.gh.guests) != 0 {
		t.Fatal("nobody is on duty")
	}
	f.onDuty(tai)
	f.send(tai, protocol.ClientMsg{T: protocol.TypeDuty, On: false})
	f.wait(3 * time.Minute)
	if len(f.gh.guests) != 0 {
		t.Fatal("off duty again")
	}
}

func TestARealRequestNearStopsNPCRequests(t *testing.T) {
	f := newNPCFixture(t)
	f.onDuty(tai, hoa)
	offers := f.book() // Lan books at once: both drivers are offered the real trip
	if len(offers) != 2 {
		t.Fatalf("real offers: %v", offers)
	}
	f.wait(31 * time.Second)
	f.wait(10 * time.Second)
	if len(f.gh.guests) != 0 {
		t.Fatalf("a real request is open nearby: %d NPC", len(f.gh.guests))
	}
}

func TestARealRequestTakesTheOfferFromAnNPC(t *testing.T) {
	f := newNPCFixture(t)
	_, npc := f.request(tai)
	f.clear()
	offers := f.book()
	if offers[tai] == "" {
		t.Fatal("the driver is offered the real trip")
	}
	if gone := f.last(tai, "ride_offer_gone"); gone == nil || gone["why"] != "cancelled" {
		t.Fatalf("the NPC's offer is withdrawn: %v", f.inbox[tai])
	}
	if f.gh.guests[npc] != nil {
		t.Fatal("the NPC left")
	}
	f.accept(tai, offers[tai]) // the real passenger is the driver's
	if r := f.last(tai, "ride"); r["passenger"].(map[string]any)["name"] != "Lan" {
		t.Fatalf("the trip is Lan's: %v", r)
	}
}

func TestAnNPCBoardsRidesAndPaysTheDriverOnce(t *testing.T) {
	f := newNPCFixture(t)
	offer, npc := f.request(tai)
	f.accept(tai, offer)
	if p := f.last(tai, "ride")["passenger"].(map[string]any); p["npc"] != true || uint32(p["id"].(float64)) != npc {
		t.Fatalf("the ride names the NPC: %v", p)
	}
	// far from it: it waits; close and stopped: it gets on behind the driver
	f.wait(2 * time.Second)
	if f.gh.guests[npc].rider != 0 {
		t.Fatal("the bike is not near yet")
	}
	f.members[tai].pose, f.members[tai].speed = protocol.Pose{X: spotX - 2}, 4
	f.wait(time.Second)
	if f.gh.guests[npc].rider != 0 {
		t.Fatal("a moving bike is not boarded")
	}
	f.members[tai].speed = 0
	f.wait(time.Second)
	if g := f.gh.guests[npc]; g.rider != tai || len(g.says) != 1 || g.says[0].VI != "Chào bác tài" {
		t.Fatalf("it got on behind Tài and greeted: %+v", g)
	}
	if r := f.last(tai, "ride"); r["state"] != ridehail.StateOnTrip {
		t.Fatalf("the driver hears the trip began: %v", r)
	}
	// not there yet, then there
	f.members[tai].pose = protocol.Pose{X: 60}
	f.wait(time.Second)
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideComplete})
	if e := f.last(tai, "ride_error"); e == nil || e["code"] != ridehail.ErrNotThere {
		t.Fatalf("completing halfway: %v", e)
	}
	f.ride(tai)
	r := f.last(tai, "ride")
	if r["state"] != ridehail.StateCompleted || int64(r["paid"].(float64)) != npcPaid || r["stars"] != float64(5) {
		t.Fatalf("the trip completed, paid and rated: %v", r)
	}
	if f.balance(f.char(tai)) != start+npcPaid {
		t.Fatalf("Tài got the driver's share (80 %%) of the fare: %d", f.balance(f.char(tai)))
	}
	if got := f.ledgerReasons(f.char(tai))["ride_npc"]; got != 1 {
		t.Fatalf("paid %d times", got)
	}
	g := f.gh.guests[npc]
	if g == nil || g.rider != 0 || g.says[len(g.says)-1].VI != "Cảm ơn" {
		t.Fatalf("it got off and thanked: %+v", g)
	}
	// no rating window and no tip: the NPC does not tip, and there is nothing to rate
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideRate, Stars: 5, Tip: 5000})
	if f.balance(f.char(tai)) != start+npcPaid {
		t.Fatal("no tip from an NPC")
	}
	// it leaves a few seconds after
	f.wait(9 * time.Second)
	if len(f.gh.guests) != 0 {
		t.Fatal("the NPC left the world")
	}
	var done int
	for _, ev := range f.events {
		if ev.Kind == events.TripCompleted {
			done++
			if ev.Character != f.char(tai) || ev.Role != ridehail.RoleRider || len(ev.Partners) != 1 || ev.Partners[0] != "npc:chi_thao" {
				t.Fatalf("trip.completed: %+v", ev)
			}
		}
	}
	if done != 1 {
		t.Fatalf("%d trip.completed events, want 1 (the driver's)", done)
	}
	trips, stars, err := f.db.DriverStats(context.Background(), f.char(tai))
	if err != nil || trips != 1 || stars != 5 {
		t.Fatalf("the rating counts in the driver's stats: %d trips, %.1f stars, %v", trips, stars, err)
	}
	// completing again moves nothing
	f.send(tai, protocol.ClientMsg{T: protocol.TypeRideComplete})
	if f.balance(f.char(tai)) != start+npcPaid {
		t.Fatal("paid twice")
	}
}

func TestAnNPCRatesTheWaitAndTheWay(t *testing.T) {
	f := newNPCFixture(t)
	offer, npc := f.request(tai)
	f.accept(tai, offer)
	f.wait(100 * time.Second) // the driver is slow to come: two stars off
	f.members[tai].pose, f.members[tai].speed = protocol.Pose{X: spotX - 2}, 0
	f.wait(time.Second)
	f.ride(tai)
	r := f.last(tai, "ride")
	if r["stars"] != float64(3) {
		t.Fatalf("a long wait costs two stars: %v", r)
	}
	if g := f.gh.guests[npc]; g.says[len(g.says)-1].VI != "Lâu quá" {
		t.Fatalf("it grumbles: %+v", g.says)
	}
	// still paid in full
	if f.balance(f.char(tai)) != start+npcPaid {
		t.Fatalf("paid %d", f.balance(f.char(tai)))
	}
}

func TestAnNPCGivesUpWaiting(t *testing.T) {
	f := newNPCFixture(t)
	offer, npc := f.request(tai)
	f.accept(tai, offer)
	f.wait(151 * time.Second)
	if s, end := f.npcTrips(tai); s != ridehail.StateCancelled || end != "pickup_timeout" {
		t.Fatalf("the driver hears it gave up: %s %s", s, end)
	}
	if g := f.gh.guests[npc]; g == nil || g.says[0].VI != "Đi xe khác" {
		t.Fatalf("it says it will go: %+v", g)
	}
	f.wait(9 * time.Second)
	if f.gh.guests[npc] != nil || f.balance(f.char(tai)) != start {
		t.Fatal("it left and nobody was paid")
	}
}

func TestTheTripEndsWithNoPayWhenTheDriverGetsOffOrLeaves(t *testing.T) {
	cases := []struct {
		name string
		end  string
		run  func(f *npcFixture)
	}{
		{"the driver gets off the bike", "driver_got_off", func(f *npcFixture) {
			f.members[tai].vehicle = 0
			f.gh.UnseatGuest(f.nextGuestID())
			f.wait(time.Second)
		}},
		{"the driver leaves", "driver_left", func(f *npcFixture) { f.leave(tai) }},
		{"the ride takes too long", "trip_timeout", func(f *npcFixture) { f.wait(config().Trip + time.Second) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newNPCFixture(t)
			offer, npc := f.request(tai)
			f.board(tai, offer)
			_ = npc
			c.run(f)
			if c.end != "driver_left" {
				if s, end := f.npcTrips(tai); s != ridehail.StateEnded || end != c.end {
					t.Fatalf("the driver hears %s %s", s, end)
				}
			}
			f.wait(10 * time.Second)
			if len(f.gh.guests) != 0 || f.balance(f.char(tai)) != start {
				t.Fatalf("no pay, and the NPC left: %d guests, %d", len(f.gh.guests), f.balance(f.char(tai)))
			}
			if got := f.ledgerReasons(f.char(tai))["ride_npc"]; got != 0 {
				t.Fatalf("ride_npc entries: %d", got)
			}
		})
	}
}

func (f *npcFixture) nextGuestID() uint32 {
	for id := range f.gh.guests {
		return id
	}
	return 0
}

func TestTheDailyCapStopsNPCTripsAndTheirPay(t *testing.T) {
	f := newNPCFixture(t) // two paid trips a day
	for i := 1; i <= 2; i++ {
		offer, _ := f.request(tai)
		f.board(tai, offer)
		f.ride(tai)
		if got := f.balance(f.char(tai)); got != start+int64(i)*npcPaid {
			t.Fatalf("after trip %d: %d", i, got)
		}
		f.members[tai].pose, f.members[tai].speed = protocol.Pose{X: 5}, 0
		f.wait(10 * time.Second)
	}
	f.clear()
	f.wait(5 * time.Minute)
	if f.last(tai, "ride_offer") != nil || len(f.gh.guests) != 0 {
		t.Fatal("no third NPC request in a day")
	}
	// the next day it starts again
	f.wait(24 * time.Hour)
	f.wait(time.Minute)
	f.wait(time.Minute)
	if f.last(tai, "ride_offer") == nil {
		t.Fatalf("the cap is per day: %v", f.inbox[tai])
	}
}

func TestStarsFromWhatTheServerCanKnow(t *testing.T) {
	r := passengers().Rules.Rating
	good := ridehail.TripFacts{Wait: 10, Straight: 100, Ridden: 105, Ride: 10}
	cases := []struct {
		name string
		f    func(f ridehail.TripFacts) ridehail.TripFacts
		want int
	}{
		{"a good trip", func(f ridehail.TripFacts) ridehail.TripFacts { return f }, 5},
		{"a longish wait", func(f ridehail.TripFacts) ridehail.TripFacts { f.Wait = 50; return f }, 4},
		{"a long wait", func(f ridehail.TripFacts) ridehail.TripFacts { f.Wait = 90; return f }, 3},
		{"a detour", func(f ridehail.TripFacts) ridehail.TripFacts { f.Ridden = 180; return f }, 4},
		{"a long detour", func(f ridehail.TripFacts) ridehail.TripFacts { f.Ridden = 300; return f }, 3},
		{"slow", func(f ridehail.TripFacts) ridehail.TripFacts { f.Ride = 30; return f }, 4},
		{"very slow", func(f ridehail.TripFacts) ridehail.TripFacts { f.Ride = 60; return f }, 3},
		{"a detour that is also slow costs the worse, not both", func(f ridehail.TripFacts) ridehail.TripFacts { f.Ridden, f.Ride = 300, 60; return f }, 3},
		{"fast riding", func(f ridehail.TripFacts) ridehail.TripFacts { f.ScaryShare = 0.5; return f }, 4},
		{"everything wrong, never under one", func(f ridehail.TripFacts) ridehail.TripFacts {
			f.Wait, f.Ridden, f.Ride, f.ScaryShare = 200, 500, 100, 1
			return f
		}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := ridehail.Stars(r, c.f(good)); got != c.want {
				t.Fatalf("stars = %d, want %d", got, c.want)
			}
		})
	}
}
