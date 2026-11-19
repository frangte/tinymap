package content

import (
	_ "embed"
	"errors"
	"fmt"
	"math"

	"saigon-noir/server/internal/protocol"
)

//go:embed passengers.json
var passengersJSON []byte

// Rating is how an NPC passenger rates a trip: it starts at five stars and loses them for what the server can
// know (it checks speeds, not walls, so nothing about crashes).
type Rating struct {
	// WaitOK and WaitBad: waiting for the pickup (from accepting to boarding) longer than WaitOK costs a star, longer
	// than WaitBad two (s)
	WaitOK  float64 `json:"wait_ok"`
	WaitBad float64 `json:"wait_bad"`
	// DetourOK and DetourBad: the way ridden over the straight line from the pickup to the destination, past
	// DetourOK a star, past DetourBad two
	DetourOK  float64 `json:"detour_ok"`
	DetourBad float64 `json:"detour_bad"`
	// PaceOK and PaceBad: the straight line over the time on the bike (m/s), below PaceOK a star, below PaceBad two.
	// The worse of the detour and the pace counts, since a detour is also a slow trip.
	PaceOK  float64 `json:"pace_ok"`
	PaceBad float64 `json:"pace_bad"`
	// ScarySpeed (m/s): the share of the trip spent faster than this above ScaryShare costs a star
	ScarySpeed float64 `json:"scary_speed"`
	ScaryShare float64 `json:"scary_share"`
}

// PassengerRules says when an NPC asks for a ride and what it does.
type PassengerRules struct {
	// IdleSeconds: a driver on duty gets an NPC request once this long has passed with no request (real or NPC)
	// near them; CooldownSeconds is the same wait after an NPC request ended or went unanswered
	IdleSeconds     int `json:"idle_seconds"`
	CooldownSeconds int `json:"cooldown_seconds"`
	// RealRadius: no NPC request while a real request is open within this of the driver (m)
	RealRadius float64 `json:"real_radius"`
	// PickupMin and PickupMax: how far from the driver the NPC waits; TripMin and TripMax: how far it goes (m)
	PickupMin float64 `json:"pickup_min"`
	PickupMax float64 `json:"pickup_max"`
	TripMin   float64 `json:"trip_min"`
	TripMax   float64 `json:"trip_max"`
	// BoardReach: the NPC gets on when the stopped bike is this close (m)
	BoardReach float64 `json:"board_reach"`
	// PatienceSeconds: an accepted request the driver has not reached by then is given up by the NPC
	PatienceSeconds int `json:"patience_seconds"`
	// DailyTrips: paid NPC trips a driver may do a day (Saigon time), so they cannot be farmed
	DailyTrips int `json:"daily_trips"`
	// LingerSeconds: an NPC that got off, or gave up, stays this long before it leaves the world
	LingerSeconds int    `json:"linger_seconds"`
	Rating        Rating `json:"rating"`
}

// Passenger is someone who asks for a ride: a figure the game draws (like npcs.json) and what they say, picked by
// the server and said out loud (nearby chat from them), so each is at most protocol.ChatMax characters.
type Passenger struct {
	ID   string        `json:"id"`
	Name string        `json:"name"`
	Role Text          `json:"role"`
	Look protocol.Look `json:"look"`
	// Greet on boarding, Thanks at the end of a good trip, Grumble after a bad one, Left when they give up waiting
	Greet   []Text `json:"greet"`
	Thanks  []Text `json:"thanks"`
	Grumble []Text `json:"grumble"`
	Left    []Text `json:"left"`
}

// PassengerSpot is where a passenger waits: a pavement at a landmark, in Blender XY metres, facing Face.
type PassengerSpot struct {
	ID    string    `json:"id"`
	Place string    `json:"place"`
	At    []float64 `json:"at"`
	Face  []float64 `json:"face"`
}

// ThreePose is where the spot is in three.js terms, facing Face.
func (s PassengerSpot) ThreePose() (x, z, heading float64) { return threePose(s.At, s.Face) }

// Passengers is passengers.json.
type Passengers struct {
	About string          `json:"about"`
	Rules PassengerRules  `json:"rules"`
	Cast  []Passenger     `json:"cast"`
	Spots []PassengerSpot `json:"spots"`
}

// LoadPassengers reads and checks passengers.json against the places.
func LoadPassengers() (Passengers, error) {
	places, err := Places()
	if err != nil {
		return Passengers{}, err
	}
	ids := make(map[string]bool, len(places))
	for _, p := range places {
		ids[p.ID] = true
	}
	return parsePassengers(passengersJSON, ids)
}

func parsePassengers(data []byte, places map[string]bool) (Passengers, error) {
	var p Passengers
	if err := decodeStrict(data, &p); err != nil {
		return Passengers{}, fmt.Errorf("passengers.json: %w", err)
	}
	if err := p.Rules.check(); err != nil {
		return Passengers{}, fmt.Errorf("passengers.json: rules: %w", err)
	}
	if len(p.Cast) < 3 || len(p.Spots) < 4 {
		return Passengers{}, errors.New("passengers.json: needs at least 3 passengers and 4 spots")
	}
	seen := map[string]bool{}
	for _, c := range p.Cast {
		if err := c.check(seen); err != nil {
			return Passengers{}, fmt.Errorf("passengers.json: passenger %q: %w", c.ID, err)
		}
		seen[c.ID] = true
	}
	seen = map[string]bool{}
	for _, s := range p.Spots {
		if err := s.check(seen, places); err != nil {
			return Passengers{}, fmt.Errorf("passengers.json: spot %q: %w", s.ID, err)
		}
		seen[s.ID] = true
	}
	return p, nil
}

func (r PassengerRules) check() error {
	g := r.Rating
	switch {
	case r.IdleSeconds < 1 || r.CooldownSeconds < 1 || r.PatienceSeconds < 30 || r.DailyTrips < 1 || r.LingerSeconds < 1:
		return errors.New("needs an idle time, a cooldown, a patience of at least 30 s, a daily cap and a linger time")
	case !(r.RealRadius > 0) || !(r.BoardReach > 0):
		return errors.New("needs a real radius and a board reach")
	case !(r.PickupMin >= 0) || r.PickupMax <= r.PickupMin || !(r.TripMin > 0) || r.TripMax <= r.TripMin:
		return errors.New("pickup and trip distances need 0 <= min < max")
	case !(g.WaitOK > 0) || g.WaitBad <= g.WaitOK:
		return errors.New("rating: wait_ok must be above 0 and below wait_bad")
	case !(g.DetourOK >= 1) || g.DetourBad <= g.DetourOK:
		return errors.New("rating: detour_ok must be at least 1 and below detour_bad")
	case !(g.PaceBad > 0) || g.PaceOK <= g.PaceBad:
		return errors.New("rating: pace_bad must be above 0 and below pace_ok")
	case !(g.ScarySpeed > 0) || !(g.ScaryShare > 0 && g.ScaryShare <= 1):
		return errors.New("rating: needs a scary speed and a scary share in (0, 1]")
	}
	return nil
}

func (c Passenger) check(seen map[string]bool) error {
	switch {
	case !idPattern.MatchString(c.ID):
		return errors.New("id must be lower_snake_case ascii")
	case seen[c.ID]:
		return errors.New("duplicate id")
	case c.Name == "" || !bilingual(c.Role):
		return errors.New("needs a name and a role in Vietnamese and English")
	case !c.Look.Valid():
		return fmt.Errorf("unknown look %+v", c.Look)
	}
	for what, lines := range map[string][]Text{"greet": c.Greet, "thanks": c.Thanks, "grumble": c.Grumble, "left": c.Left} {
		if len(lines) == 0 {
			return fmt.Errorf("no %s lines", what)
		}
		for _, l := range lines {
			if !bilingual(l) || len([]rune(l.VI)) > protocol.ChatMax || len([]rune(l.EN)) > protocol.ChatMax {
				return fmt.Errorf("a %s line needs Vietnamese and English, each at most %d characters", what, protocol.ChatMax)
			}
		}
	}
	return nil
}

func (s PassengerSpot) check(seen, places map[string]bool) error {
	switch {
	case !idPattern.MatchString(s.ID):
		return errors.New("id must be lower_snake_case ascii")
	case seen[s.ID]:
		return errors.New("duplicate id")
	case !places[s.Place]:
		return fmt.Errorf("unknown place %q", s.Place)
	case len(s.At) != 2 || len(s.Face) != 2:
		return errors.New("at and face need [x, y]")
	}
	for _, v := range append(append([]float64{}, s.At...), s.Face...) {
		if math.Abs(v) > coordLimit {
			return fmt.Errorf("%v is outside ±%d m", v, coordLimit)
		}
	}
	if s.At[0] == s.Face[0] && s.At[1] == s.Face[1] {
		return errors.New("face must be a different point from at")
	}
	return nil
}
