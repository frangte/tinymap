package ridehail

import (
	"context"
	"errors"
	"math"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/world"
)

// A settlement that could not be written is tried again after retryFirst, then twice as long each time, up to
// retryMax: the ledger's key makes a second write of one that did go through a no-op.
const (
	retryFirst = 2 * time.Second
	retryMax   = time.Minute
)

// ---- the trip: boarding, the ride, its end

func (s *Service) onBoard(m world.Member) {
	r := s.of[m.ID()]
	if r == nil || r.passenger != m.ID() || r.state != StateAccepted {
		s.fail(m.ID(), ErrGone)
		return
	}
	d, ok := s.h.Member(r.driver)
	if !ok || !onOwnBike(d) {
		s.fail(m.ID(), ErrNoBike)
		return
	}
	pp, dp := m.Pose(), d.Pose()
	switch {
	case math.Hypot(pp.X-dp.X, pp.Z-dp.Z) > s.cfg.BoardReach:
		s.fail(m.ID(), ErrTooFar)
		return
	case d.Speed() > s.cfg.StoppedSpeed:
		s.fail(m.ID(), ErrNotStopped)
		return
	case s.arrivedAt(r, dp.X, dp.Z):
		// walked to the destination while waiting: there is no trip left to make (cancelling refunds it)
		s.fail(m.ID(), ErrHere)
		return
	case !s.h.Seat(r.passenger, r.driver):
		s.fail(m.ID(), ErrRiding)
		return
	}
	r.state, r.boarded = StateOnTrip, s.h.Now()
	r.boardX, r.boardZ = dp.X, dp.Z
	r.deadline = r.boarded.Add(s.cfg.Trip)
	s.sendRide(r)
}

func (s *Service) onComplete(d world.Member) {
	r := s.of[d.ID()]
	if r == nil || r.driver != d.ID() || r.state != StateOnTrip {
		s.fail(d.ID(), ErrGone)
		return
	}
	p := d.Pose()
	switch {
	case !s.arrivedAt(r, p.X, p.Z):
		s.fail(d.ID(), ErrNotThere)
	case d.Speed() > s.cfg.StoppedSpeed:
		s.fail(d.ID(), ErrNotStopped)
	default:
		s.finish(r, StateCompleted)
	}
}

// onAlight: the passenger gets off. The fare is charged whole, as in real life; at the destination it is simply
// the trip completed.
func (s *Service) onAlight(m world.Member) {
	r := s.of[m.ID()]
	if r == nil || r.passenger != m.ID() || r.state != StateOnTrip {
		s.fail(m.ID(), ErrGone)
		return
	}
	p := m.Pose()
	if s.arrivedAt(r, p.X, p.Z) {
		s.finish(r, StateCompleted)
	} else {
		s.finish(r, "alighted")
	}
}

// arrivedAt: the three.js point (x, z) is within the trip's destination's reach.
func (s *Service) arrivedAt(r *ride, x, z float64) bool {
	place := s.places[r.dest]
	return distanceTo(place, x, z) <= place.R
}

// finish ends a trip the passenger was on with the whole fare: the driver gets their part, the passenger may rate.
func (s *Service) finish(r *ride, end string) {
	s.settle(r, StateCompleted, end, s.cfg.Paid(r.fare), 0)
}

// refund ends a trip before pickup (or one the driver broke off): the hold goes back.
func (s *Service) refund(r *ride, state, end string) {
	s.settle(r, state, end, 0, r.fare)
}

// settle ends a trip with paid going to the driver and refund back to the passenger (the rest of the fare is the
// cut). Everything about it in the world ends at once (the seat, the pin, the offers); the money follows, and
// both sides hear how it ended once it is written.
func (s *Service) settle(r *ride, state, end string, paid, refund int64) {
	if r.state == stateSettling {
		return
	}
	if r.state == StateOnTrip {
		s.h.Unseat(r.passenger)
	}
	r.state = stateSettling
	r.deadline = time.Time{}
	s.dropOffers(r, "cancelled")
	if d, ok := s.h.Member(r.driver); ok {
		p := d.Pose()
		r.endX, r.endZ = p.X, p.Z
		s.h.Pin(r.passenger, r.driver, false)
	}
	if r.dChar == "" {
		paid = 0
	}
	r.settlement = &Settlement{Ride: r.id, Passenger: r.pChar, Driver: r.dChar, Refund: refund, Paid: paid, State: state, EndReason: end,
		AcceptedAt: milli(r.accepted), BoardedAt: milli(r.boarded), FinishedAt: s.h.Now().UnixMilli()}
	s.pay(r)
}

// pay writes a trip's settlement. A failed write is tried again from Tick; until it is written the trip stays
// both sides' (neither can book or take another), so no money is left behind.
func (s *Service) pay(r *ride) {
	st := *r.settlement
	r.retryAt = time.Time{}
	var pBal, dBal int64
	s.h.Do(r.pChar, func(ctx context.Context) error {
		var err error
		pBal, dBal, err = s.bank.Settle(ctx, st)
		return err
	}, func(err error) {
		if err != nil {
			r.tries++
			wait := min(retryFirst<<min(r.tries-1, 6), retryMax)
			r.retryAt = s.h.Now().Add(wait)
			s.log.Error("ride settlement failed, trying again", "ride", r.id, "tries", r.tries, "in", wait, "err", err)
			return
		}
		r.settlement = nil
		s.h.Send(r.passenger, encode(walletMsg{T: "wallet", Balance: pBal}))
		if r.driver != 0 {
			s.h.Send(r.driver, encode(walletMsg{T: "wallet", Balance: dBal}))
		}
		s.ended(r, st.State, st.EndReason, st.Paid)
	})
}

// ended tells both sides how a trip ended, once its money moved. A completed trip stays the passenger's for the
// rating window, and counts for both sides' quests when it reached the destination.
func (s *Service) ended(r *ride, state, end string, paid int64) {
	r.state = state
	if r.driver != 0 {
		m := s.rideMsg(r, RoleRider, end)
		m.Paid = paid
		s.h.Send(r.driver, encode(m))
		if s.of[r.driver] == r {
			delete(s.of, r.driver)
		}
	}
	m := s.rideMsg(r, RolePassenger, end)
	m.Paid = paid
	if state != StateCompleted {
		s.h.Send(r.passenger, encode(m))
		s.forget(r)
		return
	}
	r.deadline = s.h.Now().Add(s.cfg.Rating)
	m.Deadline = r.deadline.UnixMilli()
	s.h.Send(r.passenger, encode(m))
	s.pairCount(r.dChar, r.pChar) // rolls the day over first
	s.pairs[[2]string{r.dChar, r.pChar}]++
	if end != StateCompleted {
		return
	}
	metres := int(math.Hypot(r.endX-r.boardX, r.endZ-r.boardZ))
	s.h.Publish(events.Event{Kind: events.TripCompleted, Character: r.dChar, Role: RoleRider, Partners: []string{r.pChar}, Metres: metres})
	s.h.Publish(events.Event{Kind: events.TripCompleted, Character: r.pChar, Role: RolePassenger, Partners: []string{r.dChar}, Metres: metres})
}

func (s *Service) onRate(m world.Member, stars int, tip int64) {
	r := s.of[m.ID()]
	switch {
	case r == nil || r.passenger != m.ID() || r.state != StateCompleted || r.rating:
		s.fail(m.ID(), ErrGone)
		return
	case !validTip(s.cfg.Tips, tip):
		s.fail(m.ID(), ErrTip)
		return
	}
	r.rating = true
	rt := Rating{Ride: r.id, Passenger: r.pChar, Driver: r.dChar, Stars: stars, Tip: tip, At: s.h.Now().UnixMilli()}
	var pBal, dBal int64
	s.h.Do(rt.Passenger, func(ctx context.Context) error {
		var err error
		pBal, dBal, err = s.bank.Rate(ctx, rt)
		return err
	}, func(err error) {
		if err != nil {
			// a smaller tip, or the same one again, may still go through while the window lasts
			r.rating = false
			if errors.Is(err, ErrBalance) {
				s.fail(r.passenger, ErrBalanceLow)
			} else {
				s.log.Error("ride rating", "ride", rt.Ride, "err", err)
				s.fail(r.passenger, ErrBusy)
			}
			return
		}
		s.h.Send(r.passenger, encode(walletMsg{T: "wallet", Balance: pBal}))
		s.h.Send(r.driver, encode(walletMsg{T: "wallet", Balance: dBal}))
		s.h.Send(r.passenger, encode(ratedMsg{T: "ride_rated", ID: r.id}))
		s.loadStats(rt.Driver)
		s.forget(r)
	})
}

func validTip(tips []int64, tip int64) bool {
	for _, t := range tips {
		if t == tip {
			return true
		}
	}
	return false
}

// ---- time and leaving

func (s *Service) Tick(now time.Time) {
	for c, at := range s.booked {
		if now.Sub(at) >= s.cfg.RequestGap {
			delete(s.booked, c)
		}
	}
	for id := range s.duty {
		if d, ok := s.h.Member(id); !ok || !onOwnBike(d) {
			s.offDuty(id, ok) // got off the bike: off duty
		}
	}
	for _, r := range s.rides {
		for id, o := range r.offers {
			if now.After(o.expires) {
				delete(r.offers, id)
				s.h.Send(id, encode(offerGoneMsg{T: "ride_offer_gone", Offer: o.id, Why: "expired"}))
			}
		}
		if r.state == stateSettling && r.settlement != nil && !r.retryAt.IsZero() && !now.Before(r.retryAt) {
			s.pay(r)
			continue
		}
		if r.state == StateOnTrip {
			s.checkSeat(r)
		}
		if r.deadline.IsZero() || !now.After(r.deadline) {
			continue
		}
		switch r.state {
		case StateRequested:
			s.refund(r, StateExpired, "no_driver")
		case StateAccepted:
			s.refund(r, StateCancelled, "pickup_timeout")
		case StateOnTrip:
			// a ride across the island takes a minute: one still going after this is not a trip
			s.settle(r, StateEnded, "trip_timeout", 0, r.fare)
		case StateCompleted:
			if !r.rating {
				s.forget(r) // the rating window closed
			}
		}
	}
}

// checkSeat ends a trip whose passenger is no longer on the bike without either side ending it. Where the
// passenger was put down says how: at the destination it is the trip completed; elsewhere, the driver getting off
// costs the passenger nothing, and the passenger leaving the seat some other way (a respawn) is getting off
// early, at the whole fare.
func (s *Service) checkSeat(r *ride) {
	p, okP := s.h.Member(r.passenger)
	d, okD := s.h.Member(r.driver)
	if !okP || !okD {
		return // Left handles it
	}
	vehicle, seat, _ := p.Riding()
	dv, _, _ := d.Riding()
	if seat == 1 && vehicle != 0 && vehicle == dv {
		return
	}
	pp := p.Pose()
	switch {
	case s.arrivedAt(r, pp.X, pp.Z):
		s.finish(r, StateCompleted)
	case !onOwnBike(d):
		s.settle(r, StateEnded, "driver_got_off", 0, r.fare)
	default:
		s.settle(r, StateEnded, "passenger_got_off", s.cfg.Paid(r.fare), 0)
	}
}

func (s *Service) Left(m world.Member) {
	s.offDuty(m.ID(), false)
	r := s.of[m.ID()]
	if r == nil {
		return
	}
	if r.passenger == m.ID() {
		switch r.state {
		case stateHolding:
			r.cancelled = true
		case StateRequested, StateAccepted:
			s.refund(r, StateCancelled, "passenger_left")
		case StateOnTrip:
			// a dropped connection is not getting off on purpose, but nor is it a free ride: the way covered is charged
			charged := s.charged(r)
			s.settle(r, StateEnded, "passenger_left", s.cfg.Paid(charged), r.fare-charged)
		case StateCompleted:
			if !r.rating {
				s.forget(r)
			}
		}
		return
	}
	switch r.state {
	case StateAccepted:
		s.driverDropped(r, false)
	case StateOnTrip:
		// the driver is gone: the passenger is put down and owes nothing
		s.settle(r, StateEnded, "driver_left", 0, r.fare)
	}
}

// charged is the part of the fare for the way covered so far (rounded like a fare), at least the base fare.
func (s *Service) charged(r *ride) int64 {
	place := s.places[r.dest]
	total := distanceTo(place, r.boardX, r.boardZ)
	d, ok := s.h.Member(r.driver)
	if !ok || total <= 0 {
		return r.fare
	}
	p := d.Pose()
	part := math.Max(0, math.Min(1, 1-distanceTo(place, p.X, p.Z)/total))
	round := float64(max(s.cfg.Fare.Round, 1))
	charged := int64(math.Round(float64(r.fare)*part/round)) * int64(round)
	return min(r.fare, max(s.cfg.Fare.Base, charged))
}

// ---- helpers

func milli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// distanceTo is how far a three.js point (x, z) is from a place (its point, or its line, in Blender XY), in metres:
// the same measure as the world's places (world/observe.go).
func distanceTo(p content.Place, x, z float64) float64 {
	by := -z // three.js (x, z) is Blender (x, -y)
	if len(p.Pts) == 1 {
		return math.Hypot(x-p.Pts[0][0], by-p.Pts[0][1])
	}
	best := math.Inf(1)
	for i := 1; i < len(p.Pts); i++ {
		a, b := p.Pts[i-1], p.Pts[i]
		vx, vy := b[0]-a[0], b[1]-a[1]
		l2 := vx*vx + vy*vy
		t := 0.0
		if l2 > 0 {
			t = math.Max(0, math.Min(1, ((x-a[0])*vx+(by-a[1])*vy)/l2))
		}
		best = math.Min(best, math.Hypot(x-(a[0]+vx*t), by-(a[1]+vy*t)))
	}
	return best
}
