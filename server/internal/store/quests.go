package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"saigon-noir/server/internal/quest"
)

// ---- quest.Store

// LoadQuests is everything kept of a character's quests.
func (db *DB) LoadQuests(ctx context.Context, character string) (quest.Saved, error) {
	var s quest.Saved
	rows, err := db.sql.QueryContext(ctx, `select quest_id, cycle, def_version, state, objectives, started_at, coalesce(completed_at, 0)
		from quest_progress where character_id = ?`, character)
	if err != nil {
		return s, fmt.Errorf("store: load quests: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r quest.Record
		var objectives string
		if err := rows.Scan(&r.ID, &r.Cycle, &r.Version, &r.State, &objectives, &r.StartedAt, &r.CompletedAt); err != nil {
			return s, fmt.Errorf("store: load quests: %w", err)
		}
		if err := json.Unmarshal([]byte(objectives), &r.Objectives); err != nil {
			return s, fmt.Errorf("store: quest %s of %s: %w", r.ID, character, err)
		}
		s.Records = append(s.Records, r)
	}
	if err := rows.Err(); err != nil {
		return s, fmt.Errorf("store: load quests: %w", err)
	}
	earned, err := db.sql.QueryContext(ctx, `select kind, item from earned where character_id = ? order by earned_at, item`, character)
	if err != nil {
		return s, fmt.Errorf("store: load earned: %w", err)
	}
	defer earned.Close()
	for earned.Next() {
		var kind, item string
		if err := earned.Scan(&kind, &item); err != nil {
			return s, fmt.Errorf("store: load earned: %w", err)
		}
		if kind == "title" {
			s.Titles = append(s.Titles, item)
		} else {
			s.Stamps = append(s.Stamps, item)
		}
	}
	if err := earned.Err(); err != nil {
		return s, fmt.Errorf("store: load earned: %w", err)
	}
	var title, streakDay sql.NullString
	if err := db.sql.QueryRowContext(ctx, `select title_id, streak_days, streak_day from characters where id = ?`, character).
		Scan(&title, &s.Streak.Days, &streakDay); err != nil {
		return s, fmt.Errorf("store: load title: %w", err)
	}
	s.Title, s.Streak.Last = title.String, streakDay.String
	return s, nil
}

// SaveQuests stores one event's changes in one transaction. A grant whose source was given before is skipped
// with everything it gives, so a retry or a replay never rewards twice.
func (db *DB) SaveQuests(ctx context.Context, character string, c quest.Commit) (quest.Stored, error) {
	var out quest.Stored
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("store: save quests: %w", err)
	}
	defer tx.Rollback() // no-op after Commit
	for _, r := range c.Records {
		objectives, err := json.Marshal(r.Objectives)
		if err != nil {
			return out, fmt.Errorf("store: save quests: %w", err)
		}
		var completed any
		if r.CompletedAt != 0 {
			completed = r.CompletedAt
		}
		if _, err := tx.ExecContext(ctx, `insert into quest_progress (character_id, quest_id, cycle, def_version, state, objectives, started_at, completed_at)
			values (?, ?, ?, ?, ?, ?, ?, ?)
			on conflict (character_id, quest_id, cycle) do update set def_version = excluded.def_version, state = excluded.state,
				objectives = excluded.objectives, completed_at = excluded.completed_at`,
			character, r.ID, r.Cycle, r.Version, r.State, string(objectives), r.StartedAt, completed); err != nil {
			return out, fmt.Errorf("store: save quest %s: %w", r.ID, err)
		}
	}
	for _, g := range c.Grants {
		given, paid, err := grant(ctx, tx, character, g, c.At)
		if err != nil {
			return out, err
		}
		if given {
			out.Granted = append(out.Granted, g.Source)
		}
		out.Paid = out.Paid || paid
	}
	if out.Paid {
		if out.Balance, err = walletBalance(ctx, tx, character); err != nil {
			return out, err
		}
	}
	if c.Streak != nil {
		var day any
		if c.Streak.Last != "" {
			day = c.Streak.Last
		}
		if _, err := tx.ExecContext(ctx, `update characters set streak_days = ?, streak_day = ?, updated_at = ? where id = ?`,
			c.Streak.Days, day, c.At, character); err != nil {
			return out, fmt.Errorf("store: save streak: %w", err)
		}
	}
	if c.Title != nil {
		var title any
		if *c.Title != "" {
			title = *c.Title
		}
		if _, err := tx.ExecContext(ctx, `update characters set title_id = ?, updated_at = ? where id = ?`, title, c.At, character); err != nil {
			return out, fmt.Errorf("store: save title: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return quest.Stored{}, fmt.Errorf("store: save quests: %w", err)
	}
	return out, nil
}

// grant gives a reward once: given is false (and nothing given) when its source was given before; paid when it
// put xu in the wallet.
func grant(ctx context.Context, tx *sql.Tx, character string, g quest.Grant, at int64) (given, paid bool, err error) {
	res, err := tx.ExecContext(ctx, `insert into reward_grants (character_id, source, granted_at) values (?, ?, ?)
		on conflict do nothing`, character, g.Source, at)
	if err != nil {
		return false, false, fmt.Errorf("store: grant %s: %w", g.Source, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return false, false, err
	}
	for kind, items := range map[string][]string{"title": g.Titles, "stamp": g.Stamps} {
		for _, item := range items {
			if _, err := tx.ExecContext(ctx, `insert into earned (character_id, kind, item, source, earned_at) values (?, ?, ?, ?, ?)
				on conflict do nothing`, character, kind, item, g.Source, at); err != nil {
				return false, false, fmt.Errorf("store: grant %s %s: %w", kind, item, err)
			}
		}
	}
	if g.Capability != "" {
		if _, err := tx.ExecContext(ctx, `insert into capability_grants (character_id, capability, source, granted_at) values (?, ?, ?, ?)
			on conflict do nothing`, character, g.Capability, g.Source, at); err != nil {
			return false, false, fmt.Errorf("store: grant %s: %w", g.Capability, err)
		}
	}
	if g.Xu > 0 {
		if paid, err = payReward(ctx, tx, character, g, at); err != nil {
			return false, false, err
		}
	}
	return true, paid, nil
}

// payReward puts a reward's xu in the wallet, with its line in the ledger. A character without a wallet yet (it
// opens on arrival, ridehail.Purse, and that failed) is not paid: the grant is kept and the visit goes on.
func payReward(ctx context.Context, tx *sql.Tx, character string, g quest.Grant, at int64) (bool, error) {
	res, err := tx.ExecContext(ctx, `update wallets set balance = balance + ?, updated_at = ? where character_id = ?`, g.Xu, at, character)
	if err != nil {
		return false, fmt.Errorf("store: pay %s: %w", g.Source, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return false, err
	}
	return true, ledger(ctx, tx, character, g.Xu, "quest_reward", "", at)
}

// EarnedCapabilities are the capabilities a character earned while playing (quest rewards), beyond its
// archetype's.
func (db *DB) EarnedCapabilities(ctx context.Context, character string) ([]string, error) {
	rows, err := db.sql.QueryContext(ctx, `select distinct capability from capability_grants where character_id = ? order by capability`, character)
	if err != nil {
		return nil, fmt.Errorf("store: earned capabilities: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("store: earned capabilities: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
