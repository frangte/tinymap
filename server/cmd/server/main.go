// Command server runs the Sài Gòn Q1 multiplayer server: a WebSocket at /ws, the account API under /api,
// /healthz, and optionally the built game.
//
// Environment:
//
//	PORT             listen port (default 8787)
//	STATIC_DIR       serve this directory (the Vite build, e.g. ../dist) for every other path
//	ALLOWED_ORIGINS  comma-separated page origins allowed to connect (default: same host as the server)
//	TRUSTED_PROXIES  comma-separated reverse proxy IPs/CIDRs whose X-Forwarded-For is trusted (default: none)
//	MAX_PLAYERS      player cap (default 150)
//	DB_PATH          the SQLite database file (default data/game.db, from the working directory)
//	BACKUP_DIR       where database backups go (default: a backup folder next to the database)
//	LOG_LEVEL        debug | info | warn | error (default info)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/chatfilter"
	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/directions"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/moderation"
	"saigon-noir/server/internal/quest"
	"saigon-noir/server/internal/ridehail"
	"saigon-noir/server/internal/store"
	"saigon-noir/server/internal/web"
	"saigon-noir/server/internal/world"
	"saigon-noir/server/internal/ws"
)

const (
	shutdownGrace = 5 * time.Second
	// backupsKept daily database backups stay on disk
	backupsKept = 7
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mod" {
		if err := runMod(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(envOr("LOG_LEVEL", "info"))); err != nil {
		return fmt.Errorf("LOG_LEVEL: %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	port, err := strconv.Atoi(envOr("PORT", "8787"))
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("PORT must be a port number, got %q", os.Getenv("PORT"))
	}
	cfg := world.DefaultConfig()
	if v := os.Getenv("MAX_PLAYERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return fmt.Errorf("MAX_PLAYERS must be a positive number, got %q", v)
		}
		cfg.MaxPlayers = n
	}
	opt := web.Options{
		StaticDir:      os.Getenv("STATIC_DIR"),
		AllowedOrigins: splitList(os.Getenv("ALLOWED_ORIGINS")),
		TrustedProxies: splitList(os.Getenv("TRUSTED_PROXIES")),
		Log:            log,
	}
	if v := os.Getenv("GUEST_BURST"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return fmt.Errorf("GUEST_BURST must be a positive number, got %q", v)
		}
		opt.GuestBurst = n
		log.Warn("guest accounts per address raised: for a load test only", "burst", n)
	}
	if opt.StaticDir != "" {
		if st, err := os.Stat(opt.StaticDir); err != nil || !st.IsDir() {
			return fmt.Errorf("STATIC_DIR %q is not a directory (run `npm run build` first?)", opt.StaticDir)
		}
	}

	// the game data (archetypes, apps, places, speeds, the nav map) is embedded; tests keep it valid, this catches
	// a broken build
	catalog, err := content.Load()
	if err != nil {
		return fmt.Errorf("content: %w", err)
	}
	grid, err := content.Nav()
	if err != nil {
		return fmt.Errorf("content: %w", err)
	}
	cfg.Move.RunSpeed, cfg.Move.Tolerance = catalog.Movement.RunSpeed, catalog.Movement.Tolerance
	cfg.NPCs, cfg.Dialogue = world.NPCsFrom(catalog), dialogue.NewScript(catalog)
	cfg.Places, cfg.Emotes = catalog.Places, world.EmoteNames(catalog)
	words, err := content.LoadChatWords()
	if err != nil {
		return fmt.Errorf("content: %w", err)
	}
	filter := chatfilter.New(words)
	cfg.Chat.Filter = filter.Clean
	engine, err := quest.NewEngine(catalog)
	if err != nil {
		return fmt.Errorf("content: %w", err)
	}
	// the world publishes what players do; the quest service (made once the world exists) takes it from there
	var quests *quest.Service
	cfg.OnEvent = func(ev events.Event) { quests.Submit(ev) }

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// a database that cannot open stops the server here, so a bad deploy fails its health check
	dbPath := envOr("DB_PATH", "data/game.db")
	db, err := store.Open(ctx, store.Options{Path: dbPath, BackupDir: os.Getenv("BACKUP_DIR")}, log)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()
	// trips live in the world's memory: any a stopped server left open give their fares back now
	if n, err := db.ReturnOpenHolds(ctx, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("database: %w", err)
	} else if n > 0 {
		log.Warn("gave back the fares of rides left open", "rides", n)
	}
	go db.BackupDaily(ctx, backupsKept)
	accounts := identity.New(db)

	w := world.New(cfg, grid, log)
	// Crab Bike trips run inside the world loop; their money moves in the database (docs/design/08-ride-hailing.md)
	w.Use(func(h world.Host) world.Module {
		return ridehail.New(h, ridehail.ConfigFrom(catalog.Ridehail), catalog.Places, db, log)
	})
	// asking the way, the co-op quest: a Traveler asks, the Citizens online are called (D18)
	w.Use(func(h world.Host) world.Module { return directions.New(h, catalog, log) })
	// moderation sees every chat line first: reports, mutes, bans, the moderators' commands (D17)
	w.Use(func(h world.Host) world.Module { return moderation.New(h, db, log) })
	quests = quest.NewService(engine, db, w, quest.DefaultConfig(), log)
	sockets := ws.New(w, accounts, catalog, ws.DefaultConfig(), log)
	sockets.Names = filter.Clean
	sockets.Quests = quests
	sockets.Wallets = ridehail.Purse{Wallets: db, Cfg: catalog.Ridehail, Now: time.Now}
	quests.OnCapability = sockets.Grant
	go quests.Run(ctx)
	go w.Run(ctx)

	router, err := web.NewRouter(w, sockets, accounts, opt)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: router, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", srv.Addr, "static", opt.StaticDir, "max_players", cfg.MaxPlayers, "db", dbPath)

	select {
	case err := <-errc:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown: %w", err)
	}
	// the world closed every connection when ctx ended; let them save where their characters stood
	if err := sockets.Drain(shutdownCtx); err != nil {
		log.Warn("some positions were not saved", "err", err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
