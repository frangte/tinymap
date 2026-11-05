// Package web is the HTTP surface: the WebSocket endpoint, a health check and (optionally) the built game.
package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"saigon-noir/server/internal/hub"
)

// Options configures the router.
type Options struct {
	// StaticDir, if set, is served for every path that is not an API route (the Vite build in dist/).
	StaticDir string
	// AllowedOrigins lists page origins (scheme://host[:port]) that may open the socket.
	// Empty means the same host as the server, which covers both the Vite dev proxy and serving dist/.
	AllowedOrigins []string
	// TrustedProxies are the reverse proxies whose X-Forwarded-For gin may believe; none by default.
	TrustedProxies []string
}

// NewRouter wires the routes. The hub must already be running.
func NewRouter(h *hub.Hub, opt Options) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/healthz"}}), gin.Recovery(), securityHeaders)
	if err := r.SetTrustedProxies(opt.TrustedProxies); err != nil {
		return nil, fmt.Errorf("trusted proxies: %w", err)
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin:     func(req *http.Request) bool { return originAllowed(req, opt.AllowedOrigins) },
	}
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			// Upgrade has already answered with an HTTP error (bad origin, not a WebSocket request)
			return
		}
		h.Serve(conn)
	})
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "players": h.Players()})
	})

	if opt.StaticDir != "" {
		files := http.FileServer(http.Dir(opt.StaticDir))
		r.NoRoute(func(c *gin.Context) {
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				c.AbortWithStatus(http.StatusMethodNotAllowed)
				return
			}
			files.ServeHTTP(c.Writer, c.Request)
		})
	}
	return r, nil
}

func securityHeaders(c *gin.Context) {
	hd := c.Writer.Header()
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	hd.Set("X-Frame-Options", "SAMEORIGIN")
	c.Next()
}

// originAllowed blocks other websites from opening a socket in a visitor's browser. Requests without an Origin
// header do not come from a browser page, so they cannot be used that way and are let through.
func originAllowed(req *http.Request, allowed []string) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if len(allowed) == 0 {
		return strings.EqualFold(u.Host, req.Host)
	}
	self := strings.ToLower(u.Scheme + "://" + u.Host)
	for _, a := range allowed {
		if strings.ToLower(strings.TrimRight(a, "/")) == self {
			return true
		}
	}
	return false
}
