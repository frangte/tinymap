package ridehail

import (
	"context"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/ids"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// NPC passengers (docs/design/08-ride-hailing.md, "NPC passengers"): an on-duty driver nobody has asked for a while
// gets a request from an NPC waiting at a pavement. It goes through the same states and messages as a real
// passenger's, with these differences: the request is offered to that driver only, nobody's wallet holds the fare
// (the game pays the driver's share when the trip completes), the NPC boards by itself when the stopped bike is
// close, rates by how the trip went, never tips, and leaves the world a few seconds after it gets off.

// guest is the NPC of a ride, which is a ride whose npc is not nil. Its id (the ride's passenger) is the NPC's id
// in the world, not a player's.
type guest struct {
	who  content.Passenger
	spot content.PassengerSpot
	// driver is who it was offered to
	driver uint32
	// x, z is where it waits
	x, z float64
	// the ride measured, while on the bike: the way ridden (m), the reports of the driver's speed and how many were
	// over the scary speed, and where the driver was last
	path    float64
	last    protocol.Pose
	samples int
	scary   int
	// trip is the settlement while it is being written; rated the stars it holds
	trip  *NPCTrip
	rated int
}

// leaver is an NPC that got off (or gave up) and leaves the world at when.
type leaver struct {
	id uint32
	at time.Time
}

// dailyCount is how many paid NPC trips a driver did on a Saigon day.
type dailyCount struct {
	day string
	n   int
}

// UsePassengers turns NPC passengers on with their content; without it (or on a host without guests) there are
// none.
func (s *Service) UsePassengers(p content.Passengers) {
	g, ok := s.h.(world.GuestHost)
	if !ok {
		return
	}
	s.guests, s.pas = g, p
	s.npcFor, s.nextNPC, s.npcToday, s.npcLoading = map[uint32]*ride{}, map[uint32]time.Time{}, map[string]dailyCount{}, map[string]bool{}
	s.pickN = rand.IntN
	for _, pl := range s.places {
		if pl.Kind == content.Landmark {
			s.landmarks = append(s.landmarks, pl)
		}
	}
	slices.SortFunc(s.landmarks, func(a, b content.Place) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
}

func (s *Service) npcOn() bool { return s.guests != nil }

func (s *Service) rules() content.PassengerRules { return s.pas.Rules }

func secs(n int) time.Duration { return time.Duration(n) * time.Second }

// npcRideOf is a driver's NPC trip, asked for or on its way, nil for none.
func (s *Service) npcRideOf(driver uint32) *ride {
	if r := s.of[driver]; r != nil && r.npc != nil {
		return r
	}
	return s.npcFor[driver]
}

// postpone makes an NPC request wait at least d from now for a driver.
func (s *Service) postpone(driver uint32, d time.Duration) {
	if s.npcOn() {
		s.nextNPC[driver] = s.h.Now().Add(d)
	}
}

// ---- asking

// npcSpawn gives each driver nobody has asked for a while one NPC request. Real passengers come first: a driver
// with a real request open near them waits, and a real request that arrives while an NPC one is offered takes its
// place (offer).
func (s *Service) npcSpawn(now time.Time) {
	for id := range s.duty {
		d, ok := s.h.Member(id)
		if !ok || now.Before(s.nextNPC[id]) || !onOwnBike(d) || s.npcRideOf(id) != nil || s.of[id] != nil || s.realRequestNear(d) {
			continue
		}
		day := Today(now)
		c, known := s.npcToday[d.Character()]
		if !known || c.day != day {
			s.loadNPCCount(d.Character(), day)
			continue
		}
		if c.n >= s.rules().DailyTrips {
			s.nextNPC[id] = now.Add(time.Minute) // looks again later; the day may roll over
			continue
		}
		if !s.askFor(d, now) {
			s.nextNPC[id] = now.Add(secs(s.rules().CooldownSeconds))
		}
	}
}

func (s *Service) loadNPCCount(driver, day string) {
	if s.npcLoading[driver] {
		return
	}
	s.npcLoading[driver] = true
	var n int
	s.h.Do(driver, func(ctx context.Context) error {
		var err error
		n, err = s.bank.NPCTripsToday(ctx, driver, day)
		return err
	}, func(err error) {
		delete(s.npcLoading, driver)
		if err != nil {
			s.log.Warn("npc trips today", "character", driver, "err", err)
			return
		}
		s.npcToday[driver] = dailyCount{day: day, n: n}
	})
}

// realRequestNear: a request of a real passenger is open within the rules' radius of the driver.
func (s *Service) realRequestNear(d world.Member) bool {
	p := d.Pose()
	for _, r := range s.rides {
		if r.npc == nil && (r.state == stateHolding || r.state == StateRequested) &&
			math.Hypot(p.X-r.pickupX, p.Z-r.pickupZ) <= s.rules().RealRadius {
			return true
		}
	}
	return false
}

// askFor makes an NPC passenger ask the driver for a ride; false when no spot or destination fits.
func (s *Service) askFor(d world.Member, now time.Time) bool {
	rl := s.rules()
	dp := d.Pose()
	var spots []content.PassengerSpot
	for _, sp := range s.pas.Spots {
		x, z, _ := sp.ThreePose()
		if dist := math.Hypot(x-dp.X, z-dp.Z); dist >= rl.PickupMin && dist <= rl.PickupMax {
			spots = append(spots, sp)
		}
	}
	if len(spots) == 0 {
		return false
	}
	spot := spots[s.pickN(len(spots))]
	x, z, face := spot.ThreePose()
	var dests []content.Place
	for _, pl := range s.landmarks {
		if dist := distanceTo(pl, x, z); dist >= rl.TripMin && dist <= rl.TripMax {
			dests = append(dests, pl)
		}
	}
	if len(dests) == 0 {
		return false
	}
	dest := dests[s.pickN(len(dests))]
	who := s.pas.Cast[s.pickN(len(s.pas.Cast))]
	id := s.guests.AddGuest(who.ID, who.Name, x, z, face)
	g := &guest{who: who, spot: spot, driver: d.ID(), x: x, z: z}
	r := &ride{id: ids.New(), passenger: id, pName: who.Name, dest: dest.ID, pickupX: x, pickupZ: z, fare: s.cfg.Fare.Of(distanceTo(dest, x, z)),
		state: StateRequested, deadline: now.Add(s.cfg.Offer), offers: map[uint32]offer{}, declined: map[uint32]bool{}, npc: g}
	o := offer{id: ids.New(), expires: r.deadline}
	r.offers[d.ID()] = o
	s.rides[r.id] = r
	s.npcFor[d.ID()] = r
	s.h.Send(d.ID(), encode(offerMsg{T: "ride_offer", Offer: o.id, Dest: r.dest, Fare: r.fare, Pass: Person{ID: id, Name: who.Name, NPC: true},
		Pickup: [2]float64{x, z}, Expires: o.expires.UnixMilli()}))
	return true
}

// ---- the trip

// npcTick follows an NPC trip on every tick: the request lapsing, the NPC giving up or boarding, the ride's
// measures and its limits.
func (s *Service) npcTick(r *ride, now time.Time) {
	d, here := s.h.Member(r.driver)
	switch r.state {
	case StateRequested:
		if now.After(r.deadline) {
			s.npcEnd(r, StateExpired, "no_driver", nil)
		}
	case StateAccepted:
		switch {
		case now.After(r.deadline):
			s.npcEnd(r, StateCancelled, "pickup_timeout", r.npc.who.Left)
		case here && onOwnBike(d):
			s.npcBoard(r, d, now)
		}
	case StateOnTrip:
		switch {
		case !here || !onOwnBike(d) || !s.guests.GuestSeated(r.passenger):
			s.npcEnd(r, StateEnded, "driver_got_off", r.npc.who.Grumble)
		case now.After(r.deadline):
			s.npcEnd(r, StateEnded, "trip_timeout", r.npc.who.Grumble)
		default:
			s.npcMeasure(r, d)
		}
	case stateSettling:
		if r.npc.trip != nil && !r.retryAt.IsZero() && !now.Before(r.retryAt) {
			s.payNPC(r)
		}
	}
}

// npcBoard seats the NPC behind the driver once the stopped bike is close to it.
func (s *Service) npcBoard(r *ride, d world.Member, now time.Time) {
	g := r.npc
	p := d.Pose()
	if math.Hypot(p.X-g.x, p.Z-g.z) > s.rules().BoardReach || d.Speed() > s.cfg.StoppedSpeed || s.arrivedAt(r, p.X, p.Z) {
		return
	}
	if !s.guests.SeatGuest(r.passenger, r.driver) {
		return
	}
	r.state, r.boarded = StateOnTrip, now
	r.boardX, r.boardZ = p.X, p.Z
	r.deadline = now.Add(s.cfg.Trip)
	g.last = p
	s.sendRide(r)
	s.guests.GuestSays(r.passenger, s.pickLine(g.who.Greet))
}

// npcMeasure adds what the ride is like to the facts the rating uses: the way ridden and how fast.
func (s *Service) npcMeasure(r *ride, d world.Member) {
	g := r.npc
	p := d.Pose()
	g.path += math.Hypot(p.X-g.last.X, p.Z-g.last.Z)
	g.last = p
	g.samples++
	if d.Speed() > s.rules().Rating.ScarySpeed {
		g.scary++
	}
}

// settleNPC ends an NPC trip. One that reached its destination is paid and rated; any other end pays nothing.
func (s *Service) settleNPC(r *ride, state, end string) {
	if r.state == stateSettling {
		return
	}
	if end != StateCompleted {
		s.npcEnd(r, state, end, r.npc.who.Grumble)
		return
	}
	g := r.npc
	if d, ok := s.h.Member(r.driver); ok {
		p := d.Pose()
		r.endX, r.endZ = p.X, p.Z
		s.npcMeasure(r, d)
	}
	s.guests.UnseatGuest(r.passenger)
	now := s.h.Now()
	g.rated, _ = s.rate(r, now)
	r.state = stateSettling
	r.deadline = time.Time{}
	g.trip = &NPCTrip{Ride: r.id, Driver: r.dChar, Passenger: g.who.ID, Dest: r.dest, PickupX: r.pickupX, PickupZ: r.pickupZ, Fare: r.fare,
		Paid: s.cfg.Paid(r.fare), Stars: g.rated, Day: Today(now), DailyTrips: s.rules().DailyTrips, AcceptedAt: milli(r.accepted),
		BoardedAt: milli(r.boarded), FinishedAt: now.UnixMilli()}
	s.payNPC(r)
}

// payNPC writes the trip. A failed write is tried again from Tick, as a real trip's settlement is.
func (s *Service) payNPC(r *ride) {
	t := *r.npc.trip
	r.retryAt = time.Time{}
	var paid, bal int64
	s.h.Do(r.dChar, func(ctx context.Context) error {
		var err error
		paid, bal, err = s.bank.SettleNPC(ctx, t)
		return err
	}, func(err error) {
		if err != nil {
			r.tries++
			wait := min(retryFirst<<min(r.tries-1, 6), retryMax)
			r.retryAt = s.h.Now().Add(wait)
			s.log.Error("npc trip settlement failed, trying again", "ride", r.id, "tries", r.tries, "in", wait, "err", err)
			return
		}
		s.npcPaid(r, paid, bal)
	})
}

// npcPaid ends a completed NPC trip once its money moved.
func (s *Service) npcPaid(r *ride, paid, balance int64) {
	g := r.npc
	g.trip = nil
	s.h.Send(r.driver, encode(walletMsg{T: "wallet", Balance: balance}))
	m := s.rideMsg(r, RoleRider, StateCompleted)
	m.State, m.Paid, m.Stars = StateCompleted, paid, g.rated
	s.h.Send(r.driver, encode(m))
	if c := s.npcToday[r.dChar]; paid > 0 {
		c.day, c.n = Today(s.h.Now()), c.n+1
		s.npcToday[r.dChar] = c
	}
	metres := int(math.Hypot(r.endX-r.boardX, r.endZ-r.boardZ))
	s.h.Publish(events.Event{Kind: events.TripCompleted, Character: r.dChar, Role: RoleRider, Partners: []string{"npc:" + g.who.ID}, Metres: metres})
	line := g.who.Thanks
	if g.rated <= 3 {
		line = g.who.Grumble
	}
	s.guests.GuestSays(r.passenger, s.pickLine(line))
	s.loadStats(r.dChar)
	s.npcFinished(r, s.rules().CooldownSeconds)
}

// npcEnd ends an NPC trip that paid nothing: the driver hears how it ended, the NPC says its line and leaves.
func (s *Service) npcEnd(r *ride, state, end string, lines []content.Text) {
	if r.state == StateOnTrip {
		s.guests.UnseatGuest(r.passenger)
	}
	if r.driver != 0 {
		m := s.rideMsg(r, RoleRider, end)
		m.State = state
		s.h.Send(r.driver, encode(m))
	}
	if len(lines) > 0 {
		s.guests.GuestSays(r.passenger, s.pickLine(lines))
	}
	s.dropOffers(r, "cancelled")
	s.npcFinished(r, s.rules().CooldownSeconds)
}

// npcFinished forgets the trip, and makes the NPC leave after a few seconds; the driver waits for the next one.
func (s *Service) npcFinished(r *ride, cooldown int) {
	driver := r.npc.driver
	s.postpone(driver, secs(cooldown))
	if s.npcFor[driver] == r {
		delete(s.npcFor, driver)
	}
	asked := r.state == StateRequested // nobody came for it: no one saw it get on or off, so it simply goes
	r.state = StateEnded
	s.forget(r)
	if asked {
		s.guests.RemoveGuest(r.passenger)
		return
	}
	s.leaving = append(s.leaving, leaver{id: r.passenger, at: s.h.Now().Add(secs(s.rules().LingerSeconds))})
}

// npcLeavers takes NPCs that got off out of the world once they have stood there a while.
func (s *Service) npcLeavers(now time.Time) {
	s.leaving = slices.DeleteFunc(s.leaving, func(l leaver) bool {
		if now.Before(l.at) {
			return false
		}
		s.guests.RemoveGuest(l.id)
		return true
	})
}

// ---- lines and rating

func (s *Service) pickLine(lines []content.Text) content.Text {
	if len(lines) == 0 {
		return content.Text{}
	}
	return lines[s.pickN(len(lines))]
}

// TripFacts are what the server knows about a trip: the time between accepting and boarding, the straight line
// from the pickup to the destination, the way ridden over it, the time on the bike, and the share of it spent
// faster than the scary speed.
type TripFacts struct {
	Wait, Straight, Ridden, Ride float64
	ScaryShare                   float64
}

// Stars is the rating of a trip: five, less a star or two for each of a long wait, a long way or a slow one (the
// worse of the two), and fast riding; at least one. The second result says what cost stars.
func Stars(r content.Rating, f TripFacts) (int, []string) {
	stars := 5
	var why []string
	cost := func(n int, reason string) {
		stars -= n
		why = append(why, reason)
	}
	switch {
	case f.Wait > r.WaitBad:
		cost(2, "wait")
	case f.Wait > r.WaitOK:
		cost(1, "wait")
	}
	route := 0
	if f.Straight > 0 {
		switch ratio := f.Ridden / f.Straight; {
		case ratio > r.DetourBad:
			route = 2
		case ratio > r.DetourOK:
			route = 1
		}
	}
	if f.Ride > 0 && f.Straight > 0 {
		switch pace := f.Straight / f.Ride; {
		case pace < r.PaceBad:
			route = max(route, 2)
		case pace < r.PaceOK:
			route = max(route, 1)
		}
	}
	if route > 0 {
		cost(route, "route")
	}
	if f.ScaryShare > r.ScaryShare {
		cost(1, "scary")
	}
	return max(stars, 1), why
}

// rate works out the stars an NPC gives a completed trip.
func (s *Service) rate(r *ride, now time.Time) (int, []string) {
	g := r.npc
	place := s.places[r.dest]
	f := TripFacts{Wait: r.boarded.Sub(r.accepted).Seconds(), Straight: distanceTo(place, r.boardX, r.boardZ), Ridden: g.path,
		Ride: now.Sub(r.boarded).Seconds()}
	if g.samples > 0 {
		f.ScaryShare = float64(g.scary) / float64(g.samples)
	}
	return Stars(s.rules().Rating, f)
}
