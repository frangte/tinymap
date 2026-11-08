// Package ridehail is booking a Crab Bike (docs/design/08-ride-hailing.md): a passenger books, every on-duty
// driver nearby gets the offer at once and the first to accept wins (D12), the driver picks the passenger up and
// drops them off, and the fare moves between their wallets (D11).
//
// It runs as a world module, on the world goroutine: the trips are plain state, and since the world reads one
// message at a time, the first accept it reads is the one that wins, with no lock. Money moves off the loop, in
// jobs against a Bank (one transaction each, idempotent by the ledger's key), whose results come back to the loop.
package ridehail

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/ids"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// DriveCapability lets a character go on duty.
const DriveCapability = "ridehail.drive"

// Config is ride-hailing's settings (content.Ridehail), with its timeouts as durations so tests can make them
// short.
type Config struct {
	content.Ridehail
	Offer, Request, Pickup, Trip, Rating time.Duration
	// RequestGap is how long a character waits between two bookings
	RequestGap time.Duration
}

// ConfigFrom reads the game data.
func ConfigFrom(r content.Ridehail) Config {
	s := func(n int) time.Duration { return time.Duration(n) * time.Second }
	return Config{Ridehail: r, Offer: s(r.OfferSeconds), Request: s(r.RequestSecs), Pickup: s(r.PickupSeconds), Trip: s(r.TripSeconds), Rating: s(r.RatingSeconds),
		RequestGap: requestGap}
}

// requestGap: one booking per 10 s per character (docs/design/08-ride-hailing.md, rate limits).
const requestGap = 10 * time.Second

// Trip states (the `state` of a ride message; the last three end a trip).
const (
	stateHolding   = "holding"  // the fare is being taken from the wallet
	stateSettling  = "settling" // the money of its end is moving
	StateRequested = "requested"
	StateAccepted  = "accepted"
	StateOnTrip    = "on_trip"
	StateCompleted = "completed"
	StateCancelled = "cancelled"
	StateExpired   = "expired"
	StateEnded     = "ended"
)

// Roles on a trip.
const (
	RolePassenger = "passenger"
	RoleRider     = "rider"
)

type ride struct {
	id                string
	passenger, driver uint32
	pChar, dChar      string
	pName, dName      string
	dest              string
	pickupX, pickupZ  float64
	// board is where the passenger got on, end where the bike was when the trip ended
	boardX, boardZ, endX, endZ float64
	fare                       int64
	state                      string
	deadline                   time.Time
	accepted, boarded          time.Time
	offers                     map[uint32]offer
	// declined are the drivers who dropped this trip: it is not offered to them again
	declined map[uint32]bool
	// cancelled: the passenger cancelled (or left) while the hold was being taken
	cancelled bool
	// rating: a rating is being stored
	rating bool
	// settlement is the money of the trip's end while it is being written; tries counts the failed writes and
	// retryAt is when the next one goes (zero while one is on its way)
	settlement *Settlement
	tries      int
	retryAt    time.Time
}

type offer struct {
	id      string
	expires time.Time
}

type stats struct {
	trips int
	stars float64
}

// Service is the ride-hailing module.
type Service struct {
	h      world.Host
	cfg    Config
	places map[string]content.Place
	bank   Bank
	log    *slog.Logger

	duty  map[uint32]bool
	of    map[uint32]*ride // a player's trip, in either role
	rides map[string]*ride
	stats map[string]stats // drivers' trips and stars, by character
	// loading are the drivers whose stats are being read
	loading map[string]bool
	// pairs counts today's trips between a driver and a passenger (the pair limit); in memory, so a restart
	// forgets it
	pairs    map[[2]string]int
	pairsDay string
	// booked is when each character last booked (the request gap); pruned once a gap is over
	booked map[string]time.Time
}

// New makes the module; hand it to world.Use.
func New(h world.Host, cfg Config, places []content.Place, bank Bank, log *slog.Logger) *Service {
	s := &Service{h: h, cfg: cfg, places: map[string]content.Place{}, bank: bank, log: log,
		duty: map[uint32]bool{}, of: map[uint32]*ride{}, rides: map[string]*ride{}, stats: map[string]stats{}, loading: map[string]bool{}, pairs: map[[2]string]int{},
		booked: map[string]time.Time{}}
	for _, p := range places {
		s.places[p.ID] = p
	}
	return s
}

func (s *Service) Handle(m world.Member, msg protocol.ClientMsg) bool {
	switch msg.T {
	case protocol.TypeDuty:
		s.onDuty(m, msg.On)
	case protocol.TypeRideRequest:
		s.onRequest(m, msg.Dest)
	case protocol.TypeRideCancel:
		s.onCancel(m)
	case protocol.TypeRideAccept:
		s.onAccept(m, msg.Offer)
	case protocol.TypeRideBoard:
		s.onBoard(m)
	case protocol.TypeRideAlight:
		s.onAlight(m)
	case protocol.TypeRideComplete:
		s.onComplete(m)
	case protocol.TypeRideRate:
		s.onRate(m, msg.Stars, msg.Tip)
	default:
		return false
	}
	return true
}

func (s *Service) fail(id uint32, code string) {
	s.h.Send(id, encode(errorMsg{T: "ride_error", Code: code}))
}

// ---- drivers

func (s *Service) onDuty(m world.Member, on bool) {
	if !on {
		s.offDuty(m.ID(), true)
		return
	}
	switch {
	case m.Character() == "":
		s.fail(m.ID(), ErrNoAccount)
		return
	case !m.Caps().Has(DriveCapability):
		s.fail(m.ID(), ErrNotAllowed)
		return
	case !onOwnBike(m):
		s.fail(m.ID(), ErrNoBike)
		return
	}
	s.duty[m.ID()] = true
	s.h.Send(m.ID(), encode(dutyMsg{T: "duty", On: true}))
	if _, known := s.stats[m.Character()]; !known {
		s.loadStats(m.Character())
	}
	// requests already waiting reach a driver coming on duty
	for _, r := range s.rides {
		if r.state == StateRequested {
			s.offer(r, m)
		}
	}
}

// offDuty takes a driver off duty, and their offers with them; tell is false for one who left.
func (s *Service) offDuty(id uint32, tell bool) {
	if !s.duty[id] {
		return
	}
	delete(s.duty, id)
	if tell {
		s.h.Send(id, encode(dutyMsg{T: "duty", On: false}))
	}
	for _, r := range s.rides {
		if o, ok := r.offers[id]; ok {
			delete(r.offers, id)
			if tell {
				s.h.Send(id, encode(offerGoneMsg{T: "ride_offer_gone", Offer: o.id, Why: "off_duty"}))
			}
		}
	}
}

func onOwnBike(m world.Member) bool {
	v, seat, own := m.Riding()
	return v != 0 && seat == 0 && own
}

// loadStats reads a driver's trips and stars (the ones known so far stay shown meanwhile).
func (s *Service) loadStats(driver string) {
	if s.loading[driver] {
		return
	}
	s.loading[driver] = true
	var st stats
	s.h.Do(driver, func(ctx context.Context) error {
		var err error
		st.trips, st.stars, err = s.bank.DriverStats(ctx, driver)
		return err
	}, func(err error) {
		delete(s.loading, driver)
		if err != nil {
			s.log.Warn("driver stats", "character", driver, "err", err)
			return
		}
		s.stats[driver] = st
	})
}

// ---- booking

func (s *Service) onRequest(m world.Member, dest string) {
	if r := s.of[m.ID()]; r != nil && r.state == StateCompleted && r.passenger == m.ID() && !r.rating {
		s.forget(r) // booking again skips rating the last trip
	}
	place, known := s.places[dest]
	vehicle, _, _ := m.Riding()
	switch {
	case m.Character() == "":
		s.fail(m.ID(), ErrNoAccount)
		return
	case s.of[m.ID()] != nil:
		s.fail(m.ID(), ErrBusy)
		return
	case vehicle != 0:
		s.fail(m.ID(), ErrRiding)
		return
	case !known:
		s.fail(m.ID(), ErrUnknownPlace)
		return
	case distanceTo(place, m.Pose().X, m.Pose().Z) <= place.R:
		s.fail(m.ID(), ErrHere)
		return
	case s.h.Now().Sub(s.booked[m.Character()]) < s.cfg.RequestGap:
		s.fail(m.ID(), ErrTooSoon)
		return
	}
	s.booked[m.Character()] = s.h.Now()
	p := m.Pose()
	r := &ride{id: ids.New(), passenger: m.ID(), pChar: m.Character(), pName: m.Name(), dest: dest, pickupX: p.X, pickupZ: p.Z,
		fare: s.cfg.Fare.Of(distanceTo(place, p.X, p.Z)), state: stateHolding, offers: map[uint32]offer{}, declined: map[uint32]bool{}}
	s.rides[r.id] = r
	s.of[m.ID()] = r
	h := Hold{Ride: r.id, Passenger: r.pChar, Dest: dest, PickupX: p.X, PickupZ: p.Z, Fare: r.fare, At: s.h.Now().UnixMilli()}
	var balance int64
	s.h.Do(r.pChar, func(ctx context.Context) error {
		var err error
		balance, err = s.bank.Hold(ctx, h)
		return err
	}, func(err error) { s.held(r, balance, err) })
}

// held follows the hold of a request: the trip looks for a driver, or ends there.
func (s *Service) held(r *ride, balance int64, err error) {
	switch {
	case errors.Is(err, ErrBalance):
		s.forget(r)
		s.fail(r.passenger, ErrBalanceLow)
		return
	case err != nil:
		// the hold may have been written all the same (a commit whose answer was lost): give it back, which
		// moves nothing if it was not
		s.log.Error("ride hold", "ride", r.id, "err", err)
		s.fail(r.passenger, ErrBusy)
		s.refund(r, StateCancelled, "hold_failed")
		return
	}
	s.h.Send(r.passenger, encode(walletMsg{T: "wallet", Balance: balance}))
	if r.cancelled {
		s.refund(r, StateCancelled, "cancelled_by_passenger")
		return
	}
	r.state = StateRequested
	r.deadline = s.h.Now().Add(s.cfg.Request)
	s.sendRide(r)
	s.offerAll(r)
}

func (s *Service) offerAll(r *ride) {
	for id := range s.duty {
		if d, ok := s.h.Member(id); ok {
			s.offer(r, d)
		}
	}
}

// offer sends a request to a driver who may take it: on duty, on their own bike, free, near enough, not the
// passenger, not one who dropped this trip, under the pair limit.
func (s *Service) offer(r *ride, d world.Member) {
	if _, sent := r.offers[d.ID()]; sent || !s.duty[d.ID()] || !onOwnBike(d) || s.of[d.ID()] != nil || d.ID() == r.passenger ||
		r.declined[d.ID()] || s.pairCount(d.Character(), r.pChar) >= s.cfg.PairLimit {
		return
	}
	p := d.Pose()
	if math.Hypot(p.X-r.pickupX, p.Z-r.pickupZ) > s.cfg.OfferRadius {
		return
	}
	o := offer{id: ids.New(), expires: s.h.Now().Add(s.cfg.Offer)}
	r.offers[d.ID()] = o
	s.h.Send(d.ID(), encode(offerMsg{T: "ride_offer", Offer: o.id, Dest: r.dest, Fare: r.fare, Pass: Person{ID: r.passenger, Name: r.pName},
		Pickup: [2]float64{r.pickupX, r.pickupZ}, Expires: o.expires.UnixMilli()}))
}

func (s *Service) onAccept(d world.Member, offerID string) {
	var r *ride
	for _, x := range s.rides {
		if o, ok := x.offers[d.ID()]; ok && o.id == offerID {
			r = x
			break
		}
	}
	switch {
	case r == nil || r.state != StateRequested:
		s.fail(d.ID(), ErrGone)
		return
	case !s.duty[d.ID()] || !onOwnBike(d):
		s.fail(d.ID(), ErrNoBike)
		return
	case s.of[d.ID()] != nil:
		s.fail(d.ID(), ErrBusy)
		return
	}
	// the first accept the loop reads wins: everyone else's offer is gone, and so are this driver's other offers
	delete(r.offers, d.ID())
	s.dropOffers(r, "taken")
	for _, x := range s.rides {
		if o, ok := x.offers[d.ID()]; ok {
			delete(x.offers, d.ID())
			s.h.Send(d.ID(), encode(offerGoneMsg{T: "ride_offer_gone", Offer: o.id, Why: "busy"}))
		}
	}
	r.driver, r.dChar, r.dName = d.ID(), d.Character(), d.Name()
	r.state, r.accepted = StateAccepted, s.h.Now()
	r.deadline = r.accepted.Add(s.cfg.Pickup)
	s.of[d.ID()] = r
	s.h.Pin(r.passenger, r.driver, true)
	s.sendRide(r)
}

func (s *Service) dropOffers(r *ride, why string) {
	for id, o := range r.offers {
		s.h.Send(id, encode(offerGoneMsg{T: "ride_offer_gone", Offer: o.id, Why: why}))
	}
	clear(r.offers)
}

func (s *Service) onCancel(m world.Member) {
	r := s.of[m.ID()]
	if r == nil {
		return
	}
	switch {
	case r.passenger == m.ID() && r.state == stateHolding:
		r.cancelled = true
	case r.passenger == m.ID() && (r.state == StateRequested || r.state == StateAccepted):
		s.refund(r, StateCancelled, "cancelled_by_passenger")
	case r.driver == m.ID() && r.state == StateAccepted:
		s.driverDropped(r, true)
	}
}

// driverDropped puts a trip whose driver cancelled (or left) before pickup back to finding a driver, for the time
// a request has, and offers it to the others.
func (s *Service) driverDropped(r *ride, tell bool) {
	if tell {
		m := s.rideMsg(r, RoleRider, "cancelled_by_driver")
		m.State = StateCancelled
		s.h.Send(r.driver, encode(m))
	}
	s.h.Pin(r.passenger, r.driver, false)
	delete(s.of, r.driver)
	r.declined[r.driver] = true
	r.driver, r.dChar, r.dName, r.accepted = 0, "", "", time.Time{}
	r.state, r.deadline = StateRequested, s.h.Now().Add(s.cfg.Request)
	s.sendRide(r)
	s.offerAll(r)
}

func (s *Service) pairCount(driver, passenger string) int {
	if day := Today(s.h.Now()); day != s.pairsDay {
		s.pairsDay = day
		clear(s.pairs)
	}
	return s.pairs[[2]string{driver, passenger}]
}

func (s *Service) forget(r *ride) {
	delete(s.rides, r.id)
	if s.of[r.passenger] == r {
		delete(s.of, r.passenger)
	}
	if r.driver != 0 && s.of[r.driver] == r {
		delete(s.of, r.driver)
	}
}

// ---- messages

func (s *Service) sendRide(r *ride) {
	s.h.Send(r.passenger, encode(s.rideMsg(r, RolePassenger, "")))
	if r.driver != 0 {
		s.h.Send(r.driver, encode(s.rideMsg(r, RoleRider, "")))
	}
}

func (s *Service) rideMsg(r *ride, role, end string) rideMsg {
	m := rideMsg{T: "ride", ID: r.id, State: r.state, Role: role, Dest: r.dest, Fare: r.fare, Pass: Person{ID: r.passenger, Name: r.pName},
		Pickup: [2]float64{r.pickupX, r.pickupZ}, Deadline: milli(r.deadline), End: end}
	if r.driver != 0 {
		st := s.stats[r.dChar]
		m.Drv = &Person{ID: r.driver, Name: r.dName, Stars: math.Round(st.stars*10) / 10, Trips: st.trips}
	}
	return m
}
