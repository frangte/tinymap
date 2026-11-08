package content

import (
	_ "embed"
	"errors"
	"fmt"
	"math"
	"slices"
)

//go:embed ridehail.json
var ridehailJSON []byte

// Fare is how a trip is priced: Base plus PerMetre for each metre of straight line, rounded to Round.
type Fare struct {
	Base     int64 `json:"base"`
	PerMetre int64 `json:"per_metre"`
	Round    int64 `json:"round"`
}

// Of is the fare for a trip of d metres.
func (f Fare) Of(d float64) int64 {
	raw := float64(f.Base) + float64(f.PerMetre)*math.Max(0, d)
	return int64(math.Round(raw/float64(f.Round))) * f.Round
}

// Ridehail is booking a Crab Bike: money (in xu), timeouts (s) and distances (m). ridehail.json says what each is.
type Ridehail struct {
	About         string  `json:"about"`
	StartBalance  int64   `json:"start_balance"`
	TopupTo       int64   `json:"topup_to"`
	Fare          Fare    `json:"fare"`
	CutPercent    int64   `json:"cut_percent"`
	Tips          []int64 `json:"tips"`
	PairLimit     int     `json:"pair_limit"`
	OfferRadius   float64 `json:"offer_radius"`
	OfferSeconds  int     `json:"offer_seconds"`
	RequestSecs   int     `json:"request_seconds"`
	PickupSeconds int     `json:"pickup_seconds"`
	TripSeconds   int     `json:"trip_seconds"`
	RatingSeconds int     `json:"rating_seconds"`
	BoardReach    float64 `json:"board_reach"`
	StoppedSpeed  float64 `json:"stopped_speed"`
}

// Paid is what the driver gets of a fare (the platform keeps its cut).
func (r Ridehail) Paid(fare int64) int64 { return fare - fare*r.CutPercent/100 }

func parseRidehail(f files, c *Catalog) error {
	if err := decodeStrict(f.ridehail, &c.Ridehail); err != nil {
		return fmt.Errorf("ridehail.json: %w", err)
	}
	return nil
}

func (r Ridehail) check() error {
	switch {
	case r.StartBalance <= 0 || r.TopupTo < 0 || r.TopupTo > r.StartBalance*10:
		return errors.New("needs a start balance, and a top-up within reason")
	case r.Fare.Base <= 0 || r.Fare.PerMetre < 0 || r.Fare.Round <= 0:
		return errors.New("fare needs a base, a per-metre price and a rounding step")
	case r.CutPercent < 0 || r.CutPercent >= 100:
		return errors.New("cut_percent must be in [0, 100)")
	case len(r.Tips) == 0 || r.Tips[0] != 0 || !slices.IsSorted(r.Tips):
		return errors.New("tips must start at 0 and go up")
	case r.PairLimit < 1 || r.OfferRadius <= 0 || r.BoardReach <= 0 || r.StoppedSpeed <= 0:
		return errors.New("needs a pair limit, an offer radius, a board reach and a stopped speed")
	case r.OfferSeconds < 5 || r.RequestSecs < r.OfferSeconds || r.PickupSeconds < 30 || r.TripSeconds < 30 || r.RatingSeconds < 10:
		return errors.New("timeouts too short: offers ≥ 5 s, requests ≥ offers, pickups ≥ 30 s, ratings ≥ 10 s")
	}
	if r.Fare.Base > r.StartBalance {
		return errors.New("the start balance must pay for at least one trip")
	}
	return nil
}
