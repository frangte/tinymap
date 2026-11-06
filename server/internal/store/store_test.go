package store

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func mustOpen(t *testing.T, opt Options, migrations fs.FS) *DB {
	t.Helper()
	db, err := open(context.Background(), opt, quiet, migrations)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func tempOptions(t *testing.T) Options {
	return Options{Path: filepath.Join(t.TempDir(), "data", "game.db")}
}

func intPragma(t *testing.T, db *DB, name string) int {
	t.Helper()
	var v int
	if err := db.sql.QueryRow("pragma " + name).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

var (
	v1 = fstest.MapFS{"migrations/0001_a.sql": {Data: []byte("create table a (x integer) strict;")}}
	v2 = fstest.MapFS{
		"migrations/0001_a.sql": v1["migrations/0001_a.sql"],
		"migrations/0002_b.sql": {Data: []byte("create table b (y integer) strict; insert into b values (7);")},
	}
)

func TestOpenSetsUpTheDatabase(t *testing.T) {
	opt := tempOptions(t)
	db := mustOpen(t, opt, migrationFiles)
	steps, err := loadMigrations(migrationFiles)
	if err != nil {
		t.Fatal(err)
	}
	if v := intPragma(t, db, "user_version"); v != len(steps) || v == 0 {
		t.Fatalf("user_version = %d, want %d", v, len(steps))
	}
	if fk := intPragma(t, db, "foreign_keys"); fk != 1 {
		t.Fatalf("foreign_keys = %d", fk)
	}
	var mode string
	if err := db.sql.QueryRow("pragma journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v", mode, err)
	}
	if _, err := os.Stat(opt.Path); err != nil {
		t.Fatalf("database file: %v", err)
	}
}

func TestMigrationsUpgradeWithABackupFirst(t *testing.T) {
	opt := tempOptions(t)
	db := mustOpen(t, opt, v1)
	if _, err := db.sql.Exec("insert into a values (1)"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// the next release adds migration 2
	db = mustOpen(t, opt, v2)
	if v := intPragma(t, db, "user_version"); v != 2 {
		t.Fatalf("user_version = %d", v)
	}
	var x, y int
	if err := db.sql.QueryRow("select (select x from a), (select y from b)").Scan(&x, &y); err != nil || x != 1 || y != 7 {
		t.Fatalf("data after migrating: x=%d y=%d err=%v", x, y, err)
	}
	pre, err := backups(filepath.Join(filepath.Dir(opt.Path), "backup"), "pre-v2")
	if err != nil || len(pre) != 1 {
		t.Fatalf("backup before migrating: %v, %v", pre, err)
	}
}

func TestFailedMigrationLeavesNothing(t *testing.T) {
	opt := tempOptions(t)
	mustOpen(t, opt, v1).Close()
	bad := fstest.MapFS{
		"migrations/0001_a.sql": v1["migrations/0001_a.sql"],
		"migrations/0002_c.sql": {Data: []byte("create table c (z integer); this is not sql;")},
	}
	if db, err := open(context.Background(), opt, quiet, bad); err == nil {
		db.Close()
		t.Fatal("a broken migration must fail Open")
	}
	db := mustOpen(t, opt, v1)
	if v := intPragma(t, db, "user_version"); v != 1 {
		t.Fatalf("user_version = %d after a failed migration", v)
	}
	if _, err := db.sql.Exec("select * from c"); err == nil {
		t.Fatal("table c should have been rolled back")
	}
}

func TestNewerDatabaseStillOpens(t *testing.T) {
	// a rollback to the previous binary meets the schema of the next one
	opt := tempOptions(t)
	mustOpen(t, opt, v2).Close()
	db := mustOpen(t, opt, v1)
	if v := intPragma(t, db, "user_version"); v != 2 {
		t.Fatalf("user_version = %d", v)
	}
}

func TestMigrationNumbering(t *testing.T) {
	sqlFile := &fstest.MapFile{Data: []byte("select 1;")}
	cases := []struct {
		name  string
		files fstest.MapFS
	}{
		{"gap", fstest.MapFS{"migrations/0001_a.sql": sqlFile, "migrations/0003_c.sql": sqlFile}},
		{"repeat", fstest.MapFS{"migrations/0001_a.sql": sqlFile, "migrations/0001_b.sql": sqlFile}},
		{"no number", fstest.MapFS{"migrations/first.sql": sqlFile}},
		{"zero", fstest.MapFS{"migrations/0000_a.sql": sqlFile}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := loadMigrations(c.files); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestBackupKeepsTheNewest(t *testing.T) {
	opt := tempOptions(t)
	db := mustOpen(t, opt, migrationFiles)
	if !db.LastBackup().IsZero() {
		t.Fatal("no backup yet")
	}
	t0 := time.Date(2026, 11, 6, 4, 0, 0, 0, time.UTC)
	var last string
	for i := range 3 {
		f, err := db.Backup(context.Background(), 2, t0.Add(time.Duration(i)*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		last = f
	}
	names, err := backups(filepath.Join(filepath.Dir(opt.Path), "backup"), dailyPrefix)
	if err != nil || len(names) != 2 || filepath.Base(last) != names[1] {
		t.Fatalf("backups = %v (%v), newest written %s", names, err, last)
	}
	if got, want := db.LastBackup(), t0.Add(48*time.Hour); !got.Equal(want) {
		t.Fatalf("LastBackup = %v, want %v", got, want)
	}
	// a backup is a whole database
	copyDB, err := sql.Open("sqlite", last)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var n int
	if err := copyDB.QueryRow("select count(*) from accounts").Scan(&n); err != nil {
		t.Fatalf("backup is not readable: %v", err)
	}
}

func TestAccountsAndCharacters(t *testing.T) {
	ctx := context.Background()
	db := mustOpen(t, tempOptions(t), migrationFiles)
	hash := []byte("0123456789abcdef0123456789abcdef")
	if err := db.CreateAccount(ctx, "A1", hash, 1000); err != nil {
		t.Fatal(err)
	}
	if id, err := db.AccountByToken(ctx, hash, 2000); err != nil || id != "A1" {
		t.Fatalf("AccountByToken = %q, %v", id, err)
	}
	var seen int64
	if err := db.sql.QueryRow("select last_seen from auth_tokens").Scan(&seen); err != nil || seen != 2000 {
		t.Fatalf("last_seen = %d, %v", seen, err)
	}
	if _, err := db.AccountByToken(ctx, []byte("other"), 0); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("unknown token: %v", err)
	}
	if _, err := db.CharacterOf(ctx, "A1"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("no character yet: %v", err)
	}

	lan := protocol.Look{Body: "woman", Outfit: "aodai", Hat: "nonla"}
	if err := db.CreateCharacter(ctx, identity.Character{ID: "C1", AccountID: "A1", Name: "Lan", Look: lan, CreatedAt: 1000}); err != nil {
		t.Fatal(err)
	}
	if c, err := db.CharacterOf(ctx, "A1"); err != nil || c.ID != "C1" || c.Name != "Lan" || c.Look != lan {
		t.Fatalf("CharacterOf = %+v, %v", c, err)
	}

	xeom := protocol.Look{Body: "woman", Outfit: "xeom", Hat: "none"}
	if err := db.UpdateProfile(ctx, "C1", "", xeom, 3000); err != nil {
		t.Fatal(err)
	}
	if c, _ := db.CharacterOf(ctx, "A1"); c.Name != "Lan" || c.Look != xeom {
		t.Fatalf("an empty name must keep the old one: %+v", c)
	}
	if err := db.UpdateProfile(ctx, "C1", "Lan Anh", xeom, 3000); err != nil {
		t.Fatal(err)
	}
	if c, _ := db.CharacterOf(ctx, "A1"); c.Name != "Lan Anh" {
		t.Fatalf("name = %q", c.Name)
	}
	if err := db.UpdateProfile(ctx, "nobody", "x", xeom, 0); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("unknown character: %v", err)
	}
	err := db.CreateCharacter(ctx, identity.Character{ID: "C2", AccountID: "nobody", Look: lan})
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("a character needs an account: %v", err)
	}
}

func TestABackupDoesNotWaitForTheGamesConnection(t *testing.T) {
	db := mustOpen(t, tempOptions(t), migrationFiles)
	character(t, db)
	// the game's one connection is busy (a save in progress): the daily backup must not queue behind it
	tx, err := db.sql.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`update characters set updated_at = updated_at + 1`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := db.Backup(ctx, 7, time.Now()); err != nil {
		t.Fatalf("a backup while the connection is busy: %v", err)
	}
}
