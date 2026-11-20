package store

import (
	"context"
	"fmt"
)

// ---- play-test characters (docs/design/14-story-tools.md): what the dev route gives besides quest progress

// GiveItem puts an item in a character's hands without charging for it: the same rows Buy writes (the item at
// price 0, the capability grants under source "item:<id>"), minus the wallet and the ledger. An item owned
// already is left as it was.
func (db *DB) GiveItem(ctx context.Context, character, item string, grants []string, at int64) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: give %s: %w", item, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `insert into owned_items (character_id, item_id, price, bought_at) values (?, ?, 0, ?)
		on conflict do nothing`, character, item, at); err != nil {
		return fmt.Errorf("store: give %s: %w", item, err)
	}
	for _, k := range grants {
		if _, err := tx.ExecContext(ctx, `insert into capability_grants (character_id, capability, source, granted_at) values (?, ?, ?, ?)
			on conflict do nothing`, character, k, "item:"+item, at); err != nil {
			return fmt.Errorf("store: give %s: grant %s: %w", item, k, err)
		}
	}
	return commit(tx, "give item")
}

// GiveXu adds xu to a character's wallet with a ledger line of the given reason.
func (db *DB) GiveXu(ctx context.Context, character string, xu int64, reason string, at int64) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: give xu: %w", err)
	}
	defer tx.Rollback()
	if err := credit(ctx, tx, character, xu, reason, "", at); err != nil {
		return err
	}
	return commit(tx, "give xu")
}
