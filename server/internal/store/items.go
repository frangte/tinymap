package store

import (
	"context"
	"fmt"

	"saigon-noir/server/internal/ids"
	"saigon-noir/server/internal/shop"
)

// ---- items (shop.Bank)

// Buy records an item a character bought: the item row first (a retry finds it and does nothing, shop.ErrOwned),
// then the balance down by the price only if it covers it (else shop.ErrBalance and nothing is kept), the
// ledger line and the capability grants (source "item:<id>"), in one transaction. It returns the new balance.
func (db *DB) Buy(ctx context.Context, p shop.Purchase) (int64, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: buy: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `insert into owned_items (character_id, item_id, price, bought_at) values (?, ?, ?, ?)
		on conflict do nothing`, p.Character, p.Item, p.Price, p.At)
	if err != nil {
		return 0, fmt.Errorf("store: buy %s: %w", p.Item, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, shop.ErrOwned
	}
	res, err = tx.ExecContext(ctx, `update wallets set balance = balance - ?, updated_at = ? where character_id = ? and balance >= ?`,
		p.Price, p.At, p.Character, p.Price)
	if err != nil {
		return 0, fmt.Errorf("store: buy %s: %w", p.Item, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, shop.ErrBalance
	}
	if _, err := tx.ExecContext(ctx, `insert into ledger (id, character_id, delta, reason, created_at) values (?, ?, ?, 'buy', ?)`,
		ids.New(), p.Character, -p.Price, p.At); err != nil {
		return 0, fmt.Errorf("store: buy %s: ledger: %w", p.Item, err)
	}
	for _, k := range p.Grants {
		if _, err := tx.ExecContext(ctx, `insert into capability_grants (character_id, capability, source, granted_at) values (?, ?, ?, ?)
			on conflict do nothing`, p.Character, k, "item:"+p.Item, p.At); err != nil {
			return 0, fmt.Errorf("store: buy %s: grant %s: %w", p.Item, k, err)
		}
	}
	balance, err := walletBalance(ctx, tx, p.Character)
	if err != nil {
		return 0, err
	}
	return balance, commit(tx, "buy")
}

// Owned lists the ids of a character's items, in the order it got them.
func (db *DB) Owned(ctx context.Context, character string) ([]string, error) {
	rows, err := db.sql.QueryContext(ctx, `select item_id from owned_items where character_id = ? order by bought_at, rowid`, character)
	if err != nil {
		return nil, fmt.Errorf("store: owned items: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: owned items: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
