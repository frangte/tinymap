// Package store keeps the durable player state in one SQLite file (docs/design/04-backend.md, "Storage: SQLite").
//
// Everything goes through a single connection: SQLite has one writer anyway, and at this game's size a query
// takes microseconds. Schema changes are the numbered files in migrations/, applied in order by Open and tracked
// with PRAGMA user_version; a copy of the database is taken before any of them runs. Migrations only ever add
// (tables, nullable columns), so rolling back to an older server binary still works against a newer database.
package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // the pure-Go driver: release builds use CGO_ENABLED=0

	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const (
	pragmas = "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
	// backupStamp is the time in backup file names; it sorts by age
	backupStamp = "20060102-150405"
	dailyPrefix = "game"
)

// Options say where the database and its backups live.
type Options struct {
	// Path is the database file; its folder is created when missing.
	Path string
	// BackupDir receives the daily backups and the copy taken before each migration.
	// Empty means a "backup" folder next to the database.
	BackupDir string
}

type DB struct {
	sql       *sql.DB
	path      string
	backupDir string
	log       *slog.Logger
}

// Open opens (or creates) the database and brings its schema up to date.
func Open(ctx context.Context, opt Options, log *slog.Logger) (*DB, error) {
	return open(ctx, opt, log, migrationFiles)
}

func open(ctx context.Context, opt Options, log *slog.Logger, migrations fs.FS) (*DB, error) {
	if opt.Path == "" || strings.ContainsRune(opt.Path, '?') {
		return nil, fmt.Errorf("store: unusable database path %q", opt.Path)
	}
	if opt.BackupDir == "" {
		opt.BackupDir = filepath.Join(filepath.Dir(opt.Path), "backup")
	}
	steps, err := loadMigrations(migrations)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(opt.Path), 0o750); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	conn, err := sql.Open("sqlite", opt.Path+"?"+pragmas)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", opt.Path, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	db := &DB{sql: conn, path: opt.Path, backupDir: opt.BackupDir, log: log}
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("store: open %s: %w", opt.Path, err)
	}
	if err := db.migrate(ctx, steps); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error { return db.sql.Close() }

// ---- migrations

type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads migrations/NNNN_name.sql; the numbers must run 1, 2, 3… without gaps.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	names, err := fs.Glob(fsys, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("store: migrations: %w", err)
	}
	steps := make([]migration, 0, len(names))
	for _, name := range names {
		base := path.Base(name)
		num, _, ok := strings.Cut(base, "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil || v < 1 {
			return nil, fmt.Errorf("store: migration %s: name it NNNN_what.sql", base)
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("store: migration %s: %w", base, err)
		}
		steps = append(steps, migration{version: v, name: base, sql: string(body)})
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].version < steps[j].version })
	for i, m := range steps {
		if m.version != i+1 {
			return nil, fmt.Errorf("store: migration %s: expected number %d (no gaps, no repeats)", m.name, i+1)
		}
	}
	return steps, nil
}

func (db *DB) migrate(ctx context.Context, steps []migration) error {
	var current int
	if err := db.sql.QueryRowContext(ctx, "pragma user_version").Scan(&current); err != nil {
		return fmt.Errorf("store: schema version: %w", err)
	}
	latest := len(steps)
	switch {
	case current > latest:
		// a rollback to an older binary: the extra tables and columns are simply unused
		db.log.Warn("database schema is newer than this server", "database", current, "server", latest)
		return nil
	case current == latest:
		return nil
	case current > 0:
		file, err := db.backup(ctx, fmt.Sprintf("pre-v%d", latest), time.Now())
		if err != nil {
			return fmt.Errorf("store: backup before migrating: %w", err)
		}
		db.log.Info("database backed up before migrating", "file", file)
	}
	for _, m := range steps[current:] {
		if err := db.apply(ctx, m); err != nil {
			return err
		}
		db.log.Info("database migrated", "version", m.version, "file", m.name)
	}
	return nil
}

// apply runs one migration and records its number in the same transaction, so a failure leaves nothing behind.
func (db *DB) apply(ctx context.Context, m migration) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	defer tx.Rollback() // no-op after Commit
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	// PRAGMA takes no bound parameters; the version is an int we parsed
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("pragma user_version = %d", m.version)); err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	return nil
}

// ---- backups

// Backup writes a consistent copy of the database to the backup folder as game-<UTC time>.db and deletes all
// but the newest `keep` of those. It is safe while the game is running.
func (db *DB) Backup(ctx context.Context, keep int, now time.Time) (string, error) {
	// a connection of its own, read-only: the game's one connection keeps serving hellos and saves while a big
	// database is copied (WAL lets a reader copy while the writer writes)
	ro, err := sql.Open("sqlite", "file:"+db.path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return "", fmt.Errorf("store: backup: %w", err)
	}
	defer ro.Close()
	ro.SetMaxOpenConns(1)
	file, err := db.backupWith(ctx, ro, dailyPrefix, now)
	if err != nil {
		return "", err
	}
	return file, prune(db.backupDir, dailyPrefix, keep)
}

// backup copies the database through its own connection (before a migration, when nothing else runs yet).
func (db *DB) backup(ctx context.Context, prefix string, now time.Time) (string, error) {
	return db.backupWith(ctx, db.sql, prefix, now)
}

func (db *DB) backupWith(ctx context.Context, conn *sql.DB, prefix string, now time.Time) (string, error) {
	if err := os.MkdirAll(db.backupDir, 0o750); err != nil {
		return "", fmt.Errorf("store: backup: %w", err)
	}
	file := filepath.Join(db.backupDir, prefix+"-"+now.UTC().Format(backupStamp)+".db")
	tmp := file + ".tmp"
	_ = os.Remove(tmp) // a leftover from a crash; VACUUM INTO refuses an existing file
	if _, err := conn.ExecContext(ctx, "vacuum into ?", tmp); err != nil {
		return "", fmt.Errorf("store: backup: %w", err)
	}
	if err := os.Rename(tmp, file); err != nil {
		return "", fmt.Errorf("store: backup: %w", err)
	}
	return file, nil
}

// backups lists the <prefix>-<time>.db files in dir, oldest first.
func backups(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: backups: %w", err)
	}
	var names []string
	for _, e := range entries {
		if n := e.Name(); strings.HasPrefix(n, prefix+"-") && strings.HasSuffix(n, ".db") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

func prune(dir, prefix string, keep int) error {
	names, err := backups(dir, prefix)
	if err != nil {
		return err
	}
	for _, n := range names[:max(0, len(names)-keep)] {
		if err := os.Remove(filepath.Join(dir, n)); err != nil {
			return fmt.Errorf("store: prune backups: %w", err)
		}
	}
	return nil
}

// LastBackup is when the newest daily backup was taken; the zero time when there is none.
func (db *DB) LastBackup() time.Time {
	names, err := backups(db.backupDir, dailyPrefix)
	if err != nil || len(names) == 0 {
		return time.Time{}
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(names[len(names)-1], dailyPrefix+"-"), ".db")
	t, err := time.Parse(backupStamp, stamp)
	if err != nil {
		return time.Time{}
	}
	return t
}

// BackupDaily takes one backup a day until ctx ends, keeping `keep` of them. Deploys restart the server often,
// so it does not count from the start: it goes by the newest backup on disk, waiting at least a minute after
// start, and an hour after a failure.
func (db *DB) BackupDaily(ctx context.Context, keep int) {
	const (
		day        = 24 * time.Hour
		startDelay = time.Minute
		retryDelay = time.Hour
	)
	wait := max(startDelay, time.Until(db.LastBackup().Add(day)))
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		file, err := db.Backup(ctx, keep, time.Now())
		if err != nil {
			db.log.Error("database backup failed", "err", err)
			wait = retryDelay
			continue
		}
		db.log.Info("database backed up", "file", file)
		wait = day
	}
}

// ---- identity.Repo

func (db *DB) CreateAccount(ctx context.Context, accountID string, tokenHash []byte, at int64) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: create account: %w", err)
	}
	defer tx.Rollback() // no-op after Commit
	if _, err := tx.ExecContext(ctx, `insert into accounts (id, created_at) values (?, ?)`, accountID, at); err != nil {
		return fmt.Errorf("store: create account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `insert into auth_tokens (token_hash, account_id, created_at) values (?, ?, ?)`,
		tokenHash, accountID, at); err != nil {
		return fmt.Errorf("store: create account: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: create account: %w", err)
	}
	return nil
}

func (db *DB) AccountByToken(ctx context.Context, tokenHash []byte, at int64) (string, error) {
	var id string
	err := db.sql.QueryRowContext(ctx, `update auth_tokens set last_seen = ? where token_hash = ? returning account_id`,
		at, tokenHash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", identity.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: account by token: %w", err)
	}
	return id, nil
}

// CharacterOf is the account's first character (an account has one for now).
func (db *DB) CharacterOf(ctx context.Context, accountID string) (identity.Character, error) {
	c := identity.Character{AccountID: accountID}
	var look string
	var x, y, z, h sql.NullFloat64
	var title sql.NullString
	var muted, banned sql.NullInt64
	err := db.sql.QueryRowContext(ctx, `select c.id, c.name, c.appearance, c.archetype_id, c.created_at, c.last_x, c.last_y, c.last_z, c.last_h,
		c.title_id, a.moderator, a.muted_until, a.banned_until
		from characters c join accounts a on a.id = c.account_id where c.account_id = ? order by c.created_at, c.id limit 1`, accountID).
		Scan(&c.ID, &c.Name, &look, &c.Archetype, &c.CreatedAt, &x, &y, &z, &h, &title, &c.Standing.Moderator, &muted, &banned)
	if errors.Is(err, sql.ErrNoRows) {
		return c, identity.ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("store: character of %s: %w", accountID, err)
	}
	if err := json.Unmarshal([]byte(look), &c.Look); err != nil {
		return c, fmt.Errorf("store: character %s appearance: %w", c.ID, err)
	}
	if x.Valid && y.Valid && z.Valid && h.Valid {
		c.Pose = &protocol.Pose{X: x.Float64, Y: y.Float64, Z: z.Float64, H: h.Float64}
	}
	c.Title = title.String
	c.Standing.MutedUntil, c.Standing.BannedUntil = muted.Int64, banned.Int64
	if c.Caps, err = db.EarnedCapabilities(ctx, c.ID); err != nil {
		return c, err
	}
	return c, nil
}

func (db *DB) CreateCharacter(ctx context.Context, c identity.Character) error {
	look, err := json.Marshal(c.Look)
	if err != nil {
		return fmt.Errorf("store: create character: %w", err)
	}
	if _, err := db.sql.ExecContext(ctx, `insert into characters (id, account_id, name, appearance, archetype_id, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?, ?)`, c.ID, c.AccountID, c.Name, string(look), c.Archetype, c.CreatedAt, c.CreatedAt); err != nil {
		return fmt.Errorf("store: create character: %w", err)
	}
	return nil
}

func (db *DB) SetArchetype(ctx context.Context, characterID, archetype string, at int64) error {
	if _, err := db.sql.ExecContext(ctx, `update characters set archetype_id = ?, updated_at = ?
		where id = ? and archetype_id = ''`, archetype, at, characterID); err != nil {
		return fmt.Errorf("store: set archetype: %w", err)
	}
	return nil
}

// SavePose stores where the character stands, for its next visit.
func (db *DB) SavePose(ctx context.Context, characterID string, p protocol.Pose, at int64) error {
	res, err := db.sql.ExecContext(ctx, `update characters set last_x = ?, last_y = ?, last_z = ?, last_h = ?, updated_at = ?
		where id = ?`, p.X, p.Y, p.Z, p.H, at, characterID)
	if err != nil {
		return fmt.Errorf("store: save pose: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("store: save pose: %w", err)
	} else if n == 0 {
		return identity.ErrNotFound
	}
	return nil
}

// UpdateProfile stores a new look and, unless name is empty, a new name (an empty name never erases one, the
// same rule the world follows for profile messages).
func (db *DB) UpdateProfile(ctx context.Context, characterID, name string, look protocol.Look, at int64) error {
	b, err := json.Marshal(look)
	if err != nil {
		return fmt.Errorf("store: update profile: %w", err)
	}
	res, err := db.sql.ExecContext(ctx, `update characters set name = coalesce(nullif(?, ''), name), appearance = ?,
		updated_at = ? where id = ?`, name, string(b), at, characterID)
	if err != nil {
		return fmt.Errorf("store: update profile: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update profile: %w", err)
	}
	if n == 0 {
		return identity.ErrNotFound
	}
	return nil
}
