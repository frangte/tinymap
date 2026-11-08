package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"saigon-noir/server/internal/ids"
	"saigon-noir/server/internal/ridehail"
)

// ---- wallets and rides (ridehail.Bank)

// ArriveWallet gives a character its wallet on its first visit (start xu) and, once a day, tops a low balance back
// up to topupTo; it returns the balance.
func (db *DB) ArriveWallet(ctx context.Context, character string, start, topupTo int64, day string, at int64) (int64, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: wallet: %w", err)
	}
	defer tx.Rollback() // no-op after Commit
	res, err := tx.ExecContext(ctx, `insert into wallets (character_id, balance, topped_up_on, updated_at) values (?, ?, ?, ?)
		on conflict do nothing`, character, start, day, at)
	if err != nil {
		return 0, fmt.Errorf("store: wallet: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		if err := ledger(ctx, tx, character, start, "start", "", at); err != nil {
			return 0, err
		}
		return start, commit(tx, "wallet")
	}
	var balance int64
	var topped sql.NullString
	if err := tx.QueryRowContext(ctx, `select balance, topped_up_on from wallets where character_id = ?`, character).Scan(&balance, &topped); err != nil {
		return 0, fmt.Errorf("store: wallet: %w", err)
	}
	if topped.String != day {
		delta := max(0, topupTo-balance)
		if _, err := tx.ExecContext(ctx, `update wallets set balance = balance + ?, topped_up_on = ?, updated_at = ? where character_id = ?`,
			delta, day, at, character); err != nil {
			return 0, fmt.Errorf("store: wallet top-up: %w", err)
		}
		if delta > 0 {
			if err := ledger(ctx, tx, character, delta, "topup", "", at); err != nil {
				return 0, err
			}
			balance += delta
		}
	}
	return balance, commit(tx, "wallet")
}

// Hold takes the fare from the passenger and records the ride, or fails with ridehail.ErrBalance. Holding the
// same ride twice takes it once.
func (db *DB) Hold(ctx context.Context, h ridehail.Hold) (int64, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: hold: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `insert into ledger (id, character_id, delta, reason, ride_id, created_at) values (?, ?, ?, 'ride_hold', ?, ?)
		on conflict do nothing`, ids.New(), h.Passenger, -h.Fare, h.Ride, h.At)
	if err != nil {
		return 0, fmt.Errorf("store: hold: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		res, err := tx.ExecContext(ctx, `update wallets set balance = balance - ?, updated_at = ? where character_id = ? and balance >= ?`,
			h.Fare, h.At, h.Passenger, h.Fare)
		if err != nil {
			return 0, fmt.Errorf("store: hold: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, ridehail.ErrBalance
		}
		if _, err := tx.ExecContext(ctx, `insert into rides (id, passenger_id, dest_place, pickup_x, pickup_z, fare, state, requested_at)
			values (?, ?, ?, ?, ?, ?, 'requested', ?)`, h.Ride, h.Passenger, h.Dest, h.PickupX, h.PickupZ, h.Fare, h.At); err != nil {
			return 0, fmt.Errorf("store: hold: %w", err)
		}
	}
	balance, err := walletBalance(ctx, tx, h.Passenger)
	if err != nil {
		return 0, err
	}
	return balance, commit(tx, "hold")
}

// Settle ends a ride in one transaction: the refund and the driver's pay (each once, by the ledger's key) and
// the ride's final row. Only a ride still open moves money, and never more than its fare: settling one again (a
// retry whose first try was written after all), or one that was never held, only reads the balances.
func (db *DB) Settle(ctx context.Context, s ridehail.Settlement) (int64, int64, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("store: settle: %w", err)
	}
	defer tx.Rollback()
	var fare int64
	err = tx.QueryRowContext(ctx, `select fare from rides where id = ? and passenger_id = ? and state = 'requested'`, s.Ride, s.Passenger).Scan(&fare)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// settled before, or never held: nothing moves
	case err != nil:
		return 0, 0, fmt.Errorf("store: settle: %w", err)
	case s.Refund < 0 || s.Paid < 0 || s.Refund+s.Paid > fare:
		return 0, 0, fmt.Errorf("store: settle %s: a refund of %d and pay of %d exceed the fare of %d", s.Ride, s.Refund, s.Paid, fare)
	default:
		if err := db.settleOpen(ctx, tx, s); err != nil {
			return 0, 0, err
		}
	}
	p, err := walletBalance(ctx, tx, s.Passenger)
	if err != nil {
		return 0, 0, err
	}
	var d int64
	if s.Driver != "" {
		if d, err = walletBalance(ctx, tx, s.Driver); err != nil {
			return 0, 0, err
		}
	}
	return p, d, commit(tx, "settle")
}

func (db *DB) settleOpen(ctx context.Context, tx *sql.Tx, s ridehail.Settlement) error {
	if err := credit(ctx, tx, s.Passenger, s.Refund, "ride_refund", s.Ride, s.FinishedAt); err != nil {
		return err
	}
	if s.Driver != "" {
		if err := credit(ctx, tx, s.Driver, s.Paid, "ride_earn", s.Ride, s.FinishedAt); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `update rides set driver_id = nullif(?, ''), state = ?, end_reason = ?, paid = ?,
		accepted_at = nullif(?, 0), boarded_at = nullif(?, 0), finished_at = ? where id = ?`,
		s.Driver, s.State, s.EndReason, s.Paid, s.AcceptedAt, s.BoardedAt, s.FinishedAt, s.Ride); err != nil {
		return fmt.Errorf("store: settle: %w", err)
	}
	return nil
}

// Rate stores the stars of a completed ride and moves the tip (once) from the passenger to the driver; a tip the
// passenger cannot pay fails with ridehail.ErrBalance and nothing is stored.
func (db *DB) Rate(ctx context.Context, r ridehail.Rating) (int64, int64, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("store: rate: %w", err)
	}
	defer tx.Rollback()
	// only a completed ride of these two, not rated yet, takes stars and a tip
	var open int
	if err := tx.QueryRowContext(ctx, `select count(*) from rides where id = ? and passenger_id = ? and driver_id = ? and state = 'completed' and stars is null`,
		r.Ride, r.Passenger, r.Driver).Scan(&open); err != nil {
		return 0, 0, fmt.Errorf("store: rate: %w", err)
	}
	if open == 1 && r.Tip > 0 {
		res, err := tx.ExecContext(ctx, `insert into ledger (id, character_id, delta, reason, ride_id, created_at) values (?, ?, ?, 'ride_tip_out', ?, ?)
			on conflict do nothing`, ids.New(), r.Passenger, -r.Tip, r.Ride, r.At)
		if err != nil {
			return 0, 0, fmt.Errorf("store: tip: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			res, err := tx.ExecContext(ctx, `update wallets set balance = balance - ?, updated_at = ? where character_id = ? and balance >= ?`,
				r.Tip, r.At, r.Passenger, r.Tip)
			if err != nil {
				return 0, 0, fmt.Errorf("store: tip: %w", err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return 0, 0, ridehail.ErrBalance
			}
			if err := credit(ctx, tx, r.Driver, r.Tip, "ride_tip_in", r.Ride, r.At); err != nil {
				return 0, 0, err
			}
		}
	}
	if open == 1 {
		if _, err := tx.ExecContext(ctx, `update rides set stars = ?, tip = ? where id = ?`, r.Stars, r.Tip, r.Ride); err != nil {
			return 0, 0, fmt.Errorf("store: rate: %w", err)
		}
	}
	p, err := walletBalance(ctx, tx, r.Passenger)
	if err != nil {
		return 0, 0, err
	}
	d, err := walletBalance(ctx, tx, r.Driver)
	if err != nil {
		return 0, 0, err
	}
	return p, d, commit(tx, "rate")
}

// DriverStats is a driver's completed trips and their average stars (0 before any rating).
func (db *DB) DriverStats(ctx context.Context, driver string) (int, float64, error) {
	var trips int
	var stars sql.NullFloat64
	if err := db.sql.QueryRowContext(ctx, `select count(*), avg(stars) from rides where driver_id = ? and state = 'completed'`,
		driver).Scan(&trips, &stars); err != nil {
		return 0, 0, fmt.Errorf("store: driver stats: %w", err)
	}
	return trips, stars.Float64, nil
}

// ReturnOpenHolds ends the rides a stopped server left open (trips live in the world's memory, so none survives a
// restart): each passenger gets the fare back, once, and the ride is ended. Call it at startup, before the world
// runs; it returns how many rides it ended.
func (db *DB) ReturnOpenHolds(ctx context.Context, at int64) (int, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: open holds: %w", err)
	}
	defer tx.Rollback()
	type hold struct {
		ride, passenger string
		fare            int64
	}
	var open []hold
	rows, err := tx.QueryContext(ctx, `select id, passenger_id, fare from rides where state = 'requested'`)
	if err != nil {
		return 0, fmt.Errorf("store: open holds: %w", err)
	}
	for rows.Next() {
		var h hold
		if err := rows.Scan(&h.ride, &h.passenger, &h.fare); err != nil {
			rows.Close()
			return 0, fmt.Errorf("store: open holds: %w", err)
		}
		open = append(open, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("store: open holds: %w", err)
	}
	for _, h := range open {
		if err := credit(ctx, tx, h.passenger, h.fare, "ride_refund", h.ride, at); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `update rides set state = 'ended', end_reason = 'server_stopped', paid = 0, finished_at = ? where id = ?`,
			at, h.ride); err != nil {
			return 0, fmt.Errorf("store: open holds: %w", err)
		}
	}
	return len(open), commit(tx, "open holds")
}

// credit adds amount (once per ride and reason) to a wallet, making the wallet if needed.
func credit(ctx context.Context, tx *sql.Tx, character string, amount int64, reason, ride string, at int64) error {
	if amount <= 0 {
		return nil
	}
	res, err := tx.ExecContext(ctx, `insert into ledger (id, character_id, delta, reason, ride_id, created_at) values (?, ?, ?, ?, ?, ?)
		on conflict do nothing`, ids.New(), character, amount, reason, ride, at)
	if err != nil {
		return fmt.Errorf("store: %s: %w", reason, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // given before: a retried step
	}
	if _, err := tx.ExecContext(ctx, `insert into wallets (character_id, balance, updated_at) values (?, ?, ?)
		on conflict (character_id) do update set balance = balance + excluded.balance, updated_at = excluded.updated_at`,
		character, amount, at); err != nil {
		return fmt.Errorf("store: %s: %w", reason, err)
	}
	return nil
}

func ledger(ctx context.Context, tx *sql.Tx, character string, delta int64, reason, ride string, at int64) error {
	var rideID any
	if ride != "" {
		rideID = ride
	}
	if _, err := tx.ExecContext(ctx, `insert into ledger (id, character_id, delta, reason, ride_id, created_at) values (?, ?, ?, ?, ?, ?)`,
		ids.New(), character, delta, reason, rideID, at); err != nil {
		return fmt.Errorf("store: ledger %s: %w", reason, err)
	}
	return nil
}

func walletBalance(ctx context.Context, tx *sql.Tx, character string) (int64, error) {
	var b int64
	err := tx.QueryRowContext(ctx, `select balance from wallets where character_id = ?`, character).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: balance: %w", err)
	}
	return b, nil
}

func commit(tx *sql.Tx, what string) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: %s: %w", what, err)
	}
	return nil
}
