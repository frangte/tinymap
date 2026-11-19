package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"saigon-noir/server/internal/ridehail"
)

// ---- trips with an NPC passenger (ridehail.Bank)

// SettleNPC ends a completed trip with an NPC passenger in one transaction: the driver's pay comes from nobody's
// wallet (ledger reason 'ride_npc', once per trip by the ledger's key) and the trip is stored with the stars the
// NPC gave. A driver already over the day's cap gets 0 for it. Settling a trip again (a retry whose first try was
// written after all) moves nothing and returns what the first try paid.
func (db *DB) SettleNPC(ctx context.Context, t ridehail.NPCTrip) (paid, balance int64, err error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("store: npc trip: %w", err)
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `select paid from npc_rides where id = ?`, t.Ride).Scan(&paid)
	switch {
	case err == nil:
		// written before: nothing moves
	case errors.Is(err, sql.ErrNoRows):
		if paid, err = db.writeNPCTrip(ctx, tx, t); err != nil {
			return 0, 0, err
		}
	default:
		return 0, 0, fmt.Errorf("store: npc trip: %w", err)
	}
	if balance, err = walletBalance(ctx, tx, t.Driver); err != nil {
		return 0, 0, err
	}
	return paid, balance, commit(tx, "npc trip")
}

func (db *DB) writeNPCTrip(ctx context.Context, tx *sql.Tx, t ridehail.NPCTrip) (int64, error) {
	var done int
	if err := tx.QueryRowContext(ctx, `select count(*) from npc_rides where driver_id = ? and day = ? and paid > 0`, t.Driver, t.Day).Scan(&done); err != nil {
		return 0, fmt.Errorf("store: npc trip: %w", err)
	}
	paid := t.Paid
	if done >= t.DailyTrips {
		paid = 0
	}
	if _, err := tx.ExecContext(ctx, `insert into npc_rides (id, driver_id, passenger, dest_place, pickup_x, pickup_z, fare, paid, stars, day,
		accepted_at, boarded_at, finished_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, nullif(?, 0), nullif(?, 0), ?)`,
		t.Ride, t.Driver, t.Passenger, t.Dest, t.PickupX, t.PickupZ, t.Fare, paid, t.Stars, t.Day, t.AcceptedAt, t.BoardedAt, t.FinishedAt); err != nil {
		return 0, fmt.Errorf("store: npc trip: %w", err)
	}
	if err := credit(ctx, tx, t.Driver, paid, "ride_npc", t.Ride, t.FinishedAt); err != nil {
		return 0, err
	}
	return paid, nil
}

// NPCTripsToday is how many paid NPC trips a driver has done on a Saigon day.
func (db *DB) NPCTripsToday(ctx context.Context, driver, day string) (int, error) {
	var n int
	if err := db.sql.QueryRowContext(ctx, `select count(*) from npc_rides where driver_id = ? and day = ? and paid > 0`, driver, day).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: npc trips today: %w", err)
	}
	return n, nil
}
