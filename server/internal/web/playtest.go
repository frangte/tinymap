package web

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"saigon-noir/server/internal/playtest"
)

// Playtester makes a test character in a chosen story state (playtest.Maker); Options.Playtest is nil unless the
// server runs with SG_DEV_TOOLS, and then the route below does not exist.
type Playtester interface {
	Make(ctx context.Context, req playtest.Request) (playtest.Result, error)
}

// maxPlaytestBody bounds the request body (a few hundred quests with outcomes fit well within it).
const maxPlaytestBody = 64 << 10

// forwardHeaders say a request came through a proxy: its own address is then the proxy's, not the caller's.
var forwardHeaders = []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Real-Ip", "Forwarded"}

// playtestRoute registers POST /api/dev/playtest on the API group.
func playtestRoute(api *gin.RouterGroup, pt Playtester) {
	api.POST("/dev/playtest", localOnly, func(c *gin.Context) {
		var req playtest.Request
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPlaytestBody)
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "bad json: " + err.Error()})
			return
		}
		res, err := pt.Make(c.Request.Context(), req)
		switch {
		case errors.Is(err, playtest.ErrInvalid):
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case err != nil:
			_ = c.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "playtest failed: " + err.Error()})
		default:
			c.JSON(http.StatusOK, res)
		}
	})
}

// localOnly refuses a caller that is not on this machine: the socket's peer must be a loopback address (never
// X-Forwarded-For, which a caller can write) and the request must not have come through a proxy, since a
// reverse proxy on the same host would otherwise make every visitor look local.
func localOnly(c *gin.Context) {
	if !isLoopback(c.Request.RemoteAddr) || hasForwardHeader(c.Request) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "local only"})
		return
	}
	c.Next()
}

func isLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hasForwardHeader(r *http.Request) bool {
	for _, h := range forwardHeaders {
		if r.Header.Get(h) != "" {
			return true
		}
	}
	return false
}
