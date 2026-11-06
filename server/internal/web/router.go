// Package web is the HTTP surface: the WebSocket endpoint, the account API, a health check and (optionally) the
// built game.
package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"

	"saigon-noir/server/internal/world"
	"saigon-noir/server/internal/ws"
)

// Guest accounts per client address: a burst for a classroom behind one NAT, then one every 10 s. A refused
// player still plays (as a guest without an account) and gets an account on a later visit.
const (
	guestBurst = 20
	guestEvery = 10 * time.Second
)

// Options configures the router.
type Options struct {
	// StaticDir, if set, is served for every path that is not an API route (the Vite build in dist/).
	StaticDir string
	// AllowedOrigins lists extra page origins (scheme://host[:port]) that may open the socket and call the API,
	// for a page hosted on another domain. A page on the server's own host is always allowed (the Vite dev
	// proxy, nginx serving dist/).
	AllowedOrigins []string
	// TrustedProxies are the reverse proxies whose X-Forwarded-For gin may believe; none by default.
	TrustedProxies []string
}

// Guests issues anonymous accounts; identity.Service in production.
type Guests interface {
	NewGuest(ctx context.Context) (token string, err error)
}

// NewRouter wires the routes. The world must already be running.
func NewRouter(w *world.World, sockets *ws.Server, guests Guests, opt Options) (*gin.Engine, error) {
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
		sockets.Serve(conn)
	})
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "players": w.Players()})
	})

	api := r.Group("/api", apiOrigin(opt.AllowedOrigins), noStore)
	limit := newIPLimiter(rate.Every(guestEvery), guestBurst)
	api.POST("/auth/guest", func(c *gin.Context) {
		if !limit.allow(c.ClientIP(), time.Now()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate"})
			return
		}
		token, err := guests.NewGuest(c.Request.Context())
		if err != nil {
			_ = c.Error(err) // printed by the request log
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
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

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Next()
}

// apiOrigin lets the same pages that may open the socket call the API: the server's own host always, other
// origins only when listed. Those get Access-Control-Allow-Origin so their scripts can read the answer; the
// game only sends simple requests (a POST without a body), so there is no preflight to answer.
func apiOrigin(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !originAllowed(c.Request, allowed) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if o := c.GetHeader("Origin"); o != "" {
			c.Header("Access-Control-Allow-Origin", o)
			c.Header("Vary", "Origin")
		}
		c.Next()
	}
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
	if strings.EqualFold(u.Host, req.Host) {
		return true
	}
	self := strings.ToLower(u.Scheme + "://" + u.Host)
	for _, a := range allowed {
		if strings.ToLower(strings.TrimRight(a, "/")) == self {
			return true
		}
	}
	return false
}

// ipLimiter is a token bucket per client address. Addresses idle for a while are forgotten, so the map only
// holds recent visitors.
type ipLimiter struct {
	mu    sync.Mutex
	every rate.Limit
	burst int
	seen  map[string]*ipEntry
	swept time.Time
}

type ipEntry struct {
	lim  *rate.Limiter
	last time.Time
}

const (
	ipForget = 10 * time.Minute
	ipSweep  = time.Minute
)

func newIPLimiter(every rate.Limit, burst int) *ipLimiter {
	return &ipLimiter{every: every, burst: burst, seen: make(map[string]*ipEntry)}
}

func (l *ipLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.swept) > ipSweep {
		for k, e := range l.seen {
			if now.Sub(e.last) > ipForget {
				delete(l.seen, k)
			}
		}
		l.swept = now
	}
	e := l.seen[ip]
	if e == nil {
		e = &ipEntry{lim: rate.NewLimiter(l.every, l.burst)}
		l.seen[ip] = e
	}
	e.last = now
	return e.lim.AllowN(now, 1)
}
