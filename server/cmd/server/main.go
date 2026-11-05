// Command server runs the Sài Gòn Q1 multiplayer hub: a WebSocket at /ws, /healthz, and optionally the built game.
//
// Environment:
//
//	PORT             listen port (default 8787)
//	STATIC_DIR       serve this directory (the Vite build, e.g. ../dist) for every other path
//	ALLOWED_ORIGINS  comma-separated page origins allowed to connect (default: same host as the server)
//	TRUSTED_PROXIES  comma-separated reverse proxy IPs/CIDRs whose X-Forwarded-For is trusted (default: none)
//	MAX_PLAYERS      player cap (default 40)
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

	"saigon-noir/server/internal/hub"
	"saigon-noir/server/internal/web"
)

const shutdownGrace = 5 * time.Second

func main() {
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
	cfg := hub.DefaultConfig()
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
	}
	if opt.StaticDir != "" {
		if st, err := os.Stat(opt.StaticDir); err != nil || !st.IsDir() {
			return fmt.Errorf("STATIC_DIR %q is not a directory (run `npm run build` first?)", opt.StaticDir)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	h := hub.New(cfg, log)
	go h.Run(ctx)

	router, err := web.NewRouter(h, opt)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: router, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", srv.Addr, "static", opt.StaticDir, "max_players", cfg.MaxPlayers)

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
