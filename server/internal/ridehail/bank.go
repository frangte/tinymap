package ridehail

import (
	"context"
	"errors"
	"time"

	"saigon-noir/server/content"
)

// The money side of trips (D11). The trip itself runs in the world loop; every step that moves xu runs as a job
// off it (world.Host.Do) against a Bank (store.DB in production), one transaction each, and comes back to the loop
// with the result. The ledger's unique key (ride, reason, character) makes a retried step a no-op.

// ErrBalance: the passenger cannot pay the fare.
var ErrBalance = errors.New("ridehail: balance too low")

// Hold takes the fare from the passenger when they book, and records the ride.
type Hold struct {
	Ride, Passenger, Dest string
	PickupX, PickupZ      float64
	Fare                  int64
	At                    int64
}

// Settlement ends a ride: Refund goes back to the passenger, Paid to the driver (the platform keeps the rest of
// the fare), and the ride row gets its final state.
type Settlement struct {
	Ride, Passenger, Driver string
	Refund, Paid            int64
	State, EndReason        string
	AcceptedAt, BoardedAt   int64
	FinishedAt              int64
}

// Rating is the passenger's stars for a completed ride, and a tip moved from them to the driver.
type Rating struct {
	Ride, Passenger, Driver string
	Stars                   int
	Tip                     int64
	At                      int64
}

// Bank keeps wallets, the ledger and rides.
type Bank interface {
	Hold(ctx context.Context, h Hold) (balance int64, err error)
	Settle(ctx context.Context, s Settlement) (passenger, driver int64, err error)
	Rate(ctx context.Context, r Rating) (passenger, driver int64, err error)
	// DriverStats is a driver's completed trips and average stars (0 before any rating).
	DriverStats(ctx context.Context, driver string) (trips int, stars float64, err error)
}

// saigon is the game's day for the daily top-up (UTC+7, no daylight saving).
var saigon = time.FixedZone("ICT", 7*60*60)

// Today is the day (in Saigon) of t, as the wallet's daily top-up counts it.
func Today(t time.Time) string { return t.In(saigon).Format("2006-01-02") }

// Wallets opens a character's wallet when it arrives: store.DB in production.
type Wallets interface {
	ArriveWallet(ctx context.Context, character string, start, topupTo int64, day string, at int64) (int64, error)
}

// Purse is the wallet a character arrives with: started on its first visit, topped up on the first visit of a
// Saigon day (D11).
type Purse struct {
	Wallets Wallets
	Cfg     content.Ridehail
	Now     func() time.Time
}

// Arrive is the character's balance as it arrives.
func (p Purse) Arrive(ctx context.Context, character string) (int64, error) {
	now := p.Now()
	return p.Wallets.ArriveWallet(ctx, character, p.Cfg.StartBalance, p.Cfg.TopupTo, Today(now), now.UnixMilli())
}
