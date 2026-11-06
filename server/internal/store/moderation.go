package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"saigon-noir/server/internal/moderation"
)

// ---- moderation.Store, and the moderators (sg-server mod)

// SetMuted mutes the character's account until then (0 lifts it), and logs who did it.
func (db *DB) SetMuted(ctx context.Context, character string, until int64, by string, at int64) error {
	return db.standing(ctx, "muted_until", "mute", character, until, by, at)
}

// SetBanned bans the character's account until then (0 lifts it), and logs who did it.
func (db *DB) SetBanned(ctx context.Context, character string, until int64, by string, at int64) error {
	return db.standing(ctx, "banned_until", "ban", character, until, by, at)
}

// standing sets one of the account's moderation columns (column is one of two constants above, never input).
func (db *DB) standing(ctx context.Context, column, action, character string, until int64, by string, at int64) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: %s: %w", action, err)
	}
	defer tx.Rollback()
	var value any
	if until > 0 {
		value = until
	} else {
		action = "un" + action
	}
	res, err := tx.ExecContext(ctx, `update accounts set `+column+` = ? where id = (select account_id from characters where id = ?)`, value, character)
	if err != nil {
		return fmt.Errorf("store: %s: %w", action, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: %s: no character %s", action, character)
	}
	if err := logAction(ctx, tx, by, character, action, value, at); err != nil {
		return err
	}
	return commit(tx, action)
}

func logAction(ctx context.Context, tx *sql.Tx, by, target, action string, until any, at int64) error {
	if _, err := tx.ExecContext(ctx, `insert into moderation_log (moderator, target, action, until, at) values (?, ?, ?, ?, ?)`,
		by, target, action, until, at); err != nil {
		return fmt.Errorf("store: log %s: %w", action, err)
	}
	return nil
}

// UnbanNamed lifts the ban of the one character named so (any case); n is how many characters have that name.
func (db *DB) UnbanNamed(ctx context.Context, name, by string, at int64) (int, error) {
	found, err := db.CharactersNamed(ctx, name)
	if err != nil || len(found) != 1 {
		return len(found), err
	}
	return 1, db.SetBanned(ctx, found[0].ID, 0, by, at)
}

// SaveReport stores a chat report.
func (db *DB) SaveReport(ctx context.Context, r moderation.Report) (int64, error) {
	lines, err := json.Marshal(r.Context)
	if err != nil {
		return 0, fmt.Errorf("store: report: %w", err)
	}
	res, err := db.sql.ExecContext(ctx, `insert into chat_reports (reporter, target, context, created_at) values (?, ?, ?, ?)`,
		r.Reporter, r.Target, string(lines), r.At)
	if err != nil {
		return 0, fmt.Errorf("store: report: %w", err)
	}
	return res.LastInsertId()
}

// OpenReports are the reports not handled yet, oldest first.
func (db *DB) OpenReports(ctx context.Context, limit int) ([]moderation.ReportRow, error) {
	rows, err := db.sql.QueryContext(ctx, `select r.id, a.name, b.name, r.context, r.created_at from chat_reports r
		join characters a on a.id = r.reporter join characters b on b.id = r.target
		where r.handled_at is null order by r.created_at, r.id limit ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: open reports: %w", err)
	}
	defer rows.Close()
	var out []moderation.ReportRow
	for rows.Next() {
		var r moderation.ReportRow
		var lines string
		if err := rows.Scan(&r.ID, &r.ReporterName, &r.TargetName, &lines, &r.At); err != nil {
			return nil, fmt.Errorf("store: open reports: %w", err)
		}
		if err := json.Unmarshal([]byte(lines), &r.Context); err != nil {
			return nil, fmt.Errorf("store: report %d: %w", r.ID, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CloseReport marks an open report handled; false when there is none with that id.
func (db *DB) CloseReport(ctx context.Context, id int64, by string, at int64) (bool, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: close report: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `update chat_reports set handled_by = ?, handled_at = ? where id = ? and handled_at is null`, by, at, id)
	if err != nil {
		return false, fmt.Errorf("store: close report: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	if err := logAction(ctx, tx, by, fmt.Sprintf("report:%d", id), "close_report", nil, at); err != nil {
		return false, err
	}
	return true, commit(tx, "close report")
}

// Named is a character found by name, for the moderation commands on the server.
type Named struct {
	ID        string
	Name      string
	AccountID string
	Moderator bool
	CreatedAt int64
}

// CharactersNamed are the characters with that name (any case, Vietnamese letters too: SQLite's lower() only
// knows ASCII, so the names are compared here), oldest first. It reads every character: fine for the moderation
// commands at this scale.
func (db *DB) CharactersNamed(ctx context.Context, name string) ([]Named, error) {
	all, err := db.named(ctx, ``)
	if err != nil {
		return nil, err
	}
	var out []Named
	for _, n := range all {
		if strings.EqualFold(n.Name, name) {
			out = append(out, n)
		}
	}
	return out, nil
}

// CharacterByID is the character with that id, if any.
func (db *DB) CharacterByID(ctx context.Context, id string) ([]Named, error) {
	return db.named(ctx, `where c.id = ?`, id)
}

// Moderators are the characters of moderator accounts.
func (db *DB) Moderators(ctx context.Context) ([]Named, error) {
	return db.named(ctx, `where a.moderator = 1`)
}

func (db *DB) named(ctx context.Context, where string, args ...any) ([]Named, error) {
	rows, err := db.sql.QueryContext(ctx, `select c.id, c.name, c.account_id, a.moderator, c.created_at from characters c
		join accounts a on a.id = c.account_id `+where+` order by c.created_at, c.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: characters: %w", err)
	}
	defer rows.Close()
	var out []Named
	for rows.Next() {
		var n Named
		if err := rows.Scan(&n.ID, &n.Name, &n.AccountID, &n.Moderator, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: characters: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SetModerator makes an account a moderator, or not.
func (db *DB) SetModerator(ctx context.Context, accountID string, on bool) error {
	v := 0
	if on {
		v = 1
	}
	if _, err := db.sql.ExecContext(ctx, `update accounts set moderator = ? where id = ?`, v, accountID); err != nil {
		return fmt.Errorf("store: set moderator: %w", err)
	}
	return nil
}
