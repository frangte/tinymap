package store

import (
	"context"
	"database/sql"
	"fmt"

	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/quest"
)

// ---- the phone (server/internal/phone): numbers and text messages

// AddContacts keeps the numbers a character has; it returns the ones that were new (a number it had is skipped).
func (db *DB) AddContacts(ctx context.Context, character string, ids []string, at int64) ([]string, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store: add contacts: %w", err)
	}
	defer tx.Rollback()
	added, err := addContacts(ctx, tx, character, ids, at)
	if err != nil {
		return nil, err
	}
	return added, commit(tx, "add contacts")
}

func addContacts(ctx context.Context, tx *sql.Tx, character string, ids []string, at int64) ([]string, error) {
	var added []string
	for _, id := range ids {
		res, err := tx.ExecContext(ctx, `insert into contacts (character_id, contact_id, since) values (?, ?, ?) on conflict do nothing`,
			character, id, at)
		if err != nil {
			return nil, fmt.Errorf("store: add contact %s: %w", id, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added = append(added, id)
		}
	}
	return added, nil
}

// Contacts lists the numbers a character has, in the order it got them.
func (db *DB) Contacts(ctx context.Context, character string) ([]string, error) {
	rows, err := db.sql.QueryContext(ctx, `select contact_id from contacts where character_id = ? order by since, rowid`, character)
	if err != nil {
		return nil, fmt.Errorf("store: contacts: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: contacts: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// HasContact says whether the character has the number.
func (db *DB) HasContact(ctx context.Context, character, contact string) (bool, error) {
	var n int
	err := db.sql.QueryRowContext(ctx, `select count(*) from contacts where character_id = ? and contact_id = ?`, character, contact).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("store: has contact: %w", err)
	}
	return n > 0, nil
}

// Texts lists a character's text messages, oldest first.
func (db *DB) Texts(ctx context.Context, character string) ([]protocol.PhoneText, error) {
	rows, err := db.sql.QueryContext(ctx, `select id, sender, vi, en, sent_at, read_at is not null from texts
		where character_id = ? order by sent_at, rowid`, character)
	if err != nil {
		return nil, fmt.Errorf("store: texts: %w", err)
	}
	defer rows.Close()
	out := []protocol.PhoneText{}
	for rows.Next() {
		var t protocol.PhoneText
		if err := rows.Scan(&t.ID, &t.From, &t.Say.VI, &t.Say.EN, &t.At, &t.Read); err != nil {
			return nil, fmt.Errorf("store: texts: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ReadTexts marks every text the character has as read.
func (db *DB) ReadTexts(ctx context.Context, character string, at int64) error {
	if _, err := db.sql.ExecContext(ctx, `update texts set read_at = ? where character_id = ? and read_at is null`, at, character); err != nil {
		return fmt.Errorf("store: read texts: %w", err)
	}
	return nil
}

// saveTexts stores the messages of a quest commit and the numbers of their senders, inside its transaction. A
// message stored before (a retried commit) is skipped; the new ones and the new numbers come back.
func saveTexts(ctx context.Context, tx *sql.Tx, character string, msgs []quest.Message, at int64, out *quest.Stored) error {
	for _, m := range msgs {
		res, err := tx.ExecContext(ctx, `insert into texts (character_id, id, sender, vi, en, sent_at) values (?, ?, ?, ?, ?, ?)
			on conflict do nothing`, character, m.Key, m.From, m.Say.VI, m.Say.EN, at)
		if err != nil {
			return fmt.Errorf("store: save text %s: %w", m.Key, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		out.Texts = append(out.Texts, protocol.PhoneText{ID: m.Key, From: m.From, Say: m.Say, At: at})
		// whoever texts you has your number, and you have theirs
		added, err := addContacts(ctx, tx, character, []string{m.From}, at)
		if err != nil {
			return err
		}
		out.Contacts = append(out.Contacts, added...)
	}
	return nil
}
