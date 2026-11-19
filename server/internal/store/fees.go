package store

import (
	"context"
	"fmt"

	"saigon-noir/server/internal/shop"
)

// ---- fees (shop.Bank) and the licence test's tickets (driving.Bank)

// Pay takes a fee from the wallet and records the capabilities it grants (source "fee:<id>"), in one transaction:
// the ledger line has the reason 'fee'. A fee whose capabilities the character holds already is not charged again
// (shop.ErrOwned: a retry, or two presses in flight), and a wallet that does not cover it keeps everything as it
// was (shop.ErrBalance). It returns the new balance.
func (db *DB) Pay(ctx context.Context, p shop.Payment) (int64, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: pay: %w", err)
	}
	defer tx.Rollback()
	if len(p.Grants) > 0 {
		var held int
		if err := tx.QueryRowContext(ctx, `select count(distinct capability) from capability_grants where character_id = ? and capability in (`+
			placeholders(len(p.Grants))+`)`, append([]any{p.Character}, anys(p.Grants)...)...).Scan(&held); err != nil {
			return 0, fmt.Errorf("store: pay %s: %w", p.Fee, err)
		}
		if held == len(p.Grants) {
			return 0, shop.ErrOwned
		}
	}
	res, err := tx.ExecContext(ctx, `update wallets set balance = balance - ?, updated_at = ? where character_id = ? and balance >= ?`,
		p.Amount, p.At, p.Character, p.Amount)
	if err != nil {
		return 0, fmt.Errorf("store: pay %s: %w", p.Fee, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, shop.ErrBalance
	}
	if err := ledger(ctx, tx, p.Character, -p.Amount, "fee", "", p.At); err != nil {
		return 0, err
	}
	for _, k := range p.Grants {
		if _, err := tx.ExecContext(ctx, `insert into capability_grants (character_id, capability, source, granted_at) values (?, ?, ?, ?)
			on conflict do nothing`, p.Character, k, "fee:"+p.Fee, p.At); err != nil {
			return 0, fmt.Errorf("store: pay %s: grant %s: %w", p.Fee, k, err)
		}
	}
	balance, err := walletBalance(ctx, tx, p.Character)
	if err != nil {
		return 0, err
	}
	return balance, commit(tx, "pay")
}

// GrantCapability gives a character a capability for good (until revoked), recorded with its source.
func (db *DB) GrantCapability(ctx context.Context, character, capability, source string, at int64) error {
	if _, err := db.sql.ExecContext(ctx, `insert into capability_grants (character_id, capability, source, granted_at) values (?, ?, ?, ?)
		on conflict do nothing`, character, capability, source, at); err != nil {
		return fmt.Errorf("store: grant %s: %w", capability, err)
	}
	return nil
}

// RevokeCapability takes a capability away, whatever gave it (a spent ticket).
func (db *DB) RevokeCapability(ctx context.Context, character, capability string) error {
	if _, err := db.sql.ExecContext(ctx, `delete from capability_grants where character_id = ? and capability = ?`, character, capability); err != nil {
		return fmt.Errorf("store: revoke %s: %w", capability, err)
	}
	return nil
}

func placeholders(n int) string {
	out := ""
	for i := range n {
		if i > 0 {
			out += ", "
		}
		out += "?"
	}
	return out
}

func anys(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
