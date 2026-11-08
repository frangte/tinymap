package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"saigon-noir/server/internal/store"
)

const modUsage = `usage: sg-server mod list | add <name or character id> | remove <name or character id>
  Moderators (any character of a moderator account) mute, ban, kick and read reports with chat commands in the
  game: /mute, /unmute, /ban, /unban, /kick, /reports, /done. Uses DB_PATH like the server.`

// runMod lists, adds or removes moderators in the database (docs/design/README.md, D17).
func runMod(args []string) error {
	if len(args) == 0 || (args[0] != "list" && len(args) < 2) {
		return errors.New(modUsage)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(ctx, store.Options{Path: envOr("DB_PATH", "data/game.db"), BackupDir: os.Getenv("BACKUP_DIR")},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()
	switch args[0] {
	case "list":
		mods, err := db.Moderators(ctx)
		if err != nil {
			return err
		}
		if len(mods) == 0 {
			fmt.Println("no moderators")
		}
		for _, m := range mods {
			fmt.Printf("%s  %s\n", m.ID, m.Name)
		}
		return nil
	case "add", "remove":
		who, err := oneCharacter(ctx, db, args[1])
		if err != nil {
			return err
		}
		if err := db.SetModerator(ctx, who.AccountID, args[0] == "add"); err != nil {
			return err
		}
		fmt.Printf("%s %s (%s): moderator %v, from their next visit\n", args[0], who.Name, who.ID, args[0] == "add")
		return nil
	}
	return errors.New(modUsage)
}

// oneCharacter finds the character meant by a name or an id; several with the name is an error listing them.
func oneCharacter(ctx context.Context, db *store.DB, ref string) (store.Named, error) {
	found, err := db.CharacterByID(ctx, ref)
	if err == nil && len(found) == 0 {
		found, err = db.CharactersNamed(ctx, ref)
	}
	if err != nil {
		return store.Named{}, err
	}
	switch len(found) {
	case 0:
		return store.Named{}, fmt.Errorf("no character named %q", ref)
	case 1:
		return found[0], nil
	}
	msg := fmt.Sprintf("%d characters are named %q; give the id of one:", len(found), ref)
	for _, f := range found {
		msg += fmt.Sprintf("\n  %s  %s  made %s", f.ID, f.Name, time.UnixMilli(f.CreatedAt).Format(time.DateTime))
	}
	return store.Named{}, errors.New(msg)
}
