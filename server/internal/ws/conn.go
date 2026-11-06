// Package ws serves the game socket. Each connection has a reader and a writer goroutine: the reader checks the
// hello, resolves the account (here, so the world loop never waits on the database), rate-limits what the client
// sends and hands the rest to the world; the writer drains the peer's outbox and pings.
package ws

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = pongWait * 9 / 10
	// identityWait bounds the account lookup in a hello, and each profile save
	identityWait = 3 * time.Second
)

// Per-connection limits: states arrive at 10 Hz, so 20 Hz leaves room for jitter; chat about one line a second.
var (
	stateRate    = rate.Limit(20)
	stateBurst   = 20
	chatRate     = rate.Every(700 * time.Millisecond)
	chatBurst    = 3
	profileRate  = rate.Every(250 * time.Millisecond)
	profileBurst = 4
)

type Config struct {
	// HelloTimeout is how long a new connection may take to send its hello.
	HelloTimeout time.Duration
}

func DefaultConfig() Config { return Config{HelloTimeout: 10 * time.Second} }

// Identity says who a connection is; identity.Service in production.
type Identity interface {
	// Arrive finds the character behind a hello's token, stores the profile it came with and returns the
	// character's id and archetype (kept once chosen). An empty token is a guest without an account (no id, the
	// hello's archetype); a token that no account has is identity.ErrUnknownToken.
	Arrive(ctx context.Context, token string, p identity.Profile) (characterID, archetype string, err error)
	// SaveProfile stores a changed name or look.
	SaveProfile(ctx context.Context, characterID, name string, look protocol.Look) error
}

type Server struct {
	world   *world.World
	ids     Identity
	catalog *content.Catalog
	cfg     Config
	log     *slog.Logger
}

func New(w *world.World, ids Identity, catalog *content.Catalog, cfg Config, log *slog.Logger) *Server {
	return &Server{world: w, ids: ids, catalog: catalog, cfg: cfg, log: log}
}

// Serve runs a connection until it closes. Call it from the HTTP handler after the upgrade.
func (s *Server) Serve(conn *websocket.Conn) {
	c := &client{s: s, conn: conn, peer: s.world.NewPeer()}
	go c.writeLoop()
	c.readLoop()
}

type client struct {
	s    *Server
	conn *websocket.Conn
	peer *world.Peer
	// character is the account's character, "" for a guest without an account (or when the lookup failed)
	character string
	// caps are what the character's archetype grants: which phone apps it has, which looks it may wear
	caps capability.Set
}

// readLoop never closes the socket itself while the world runs: every way out ends in a Leave, the world then
// closes the outbox, and writeLoop closes the socket after flushing what is queued (such as an error code).
func (c *client) readLoop() {
	w := c.s.world
	c.conn.SetReadLimit(protocol.MaxMessageBytes)
	leave := func(code string) {
		if !w.Leave(c.peer, code) {
			c.conn.Close() // the world stopped: nobody will close the outbox
		}
	}

	// the first frame must be a valid hello, and soon
	_ = c.conn.SetReadDeadline(time.Now().Add(c.s.cfg.HelloTimeout))
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		leave("")
		return
	}
	hello, err := protocol.Decode(data)
	if err != nil || hello.T != protocol.TypeHello {
		leave(protocol.ErrBadHello)
		return
	}
	arrival, code := c.arrive(hello)
	if code != "" {
		leave(code)
		return
	}
	if !w.Join(c.peer, arrival) {
		c.conn.Close()
		return
	}

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	states := rate.NewLimiter(stateRate, stateBurst)
	chats := rate.NewLimiter(chatRate, chatBurst)
	profiles := rate.NewLimiter(profileRate, profileBurst)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) && !errors.Is(err, websocket.ErrReadLimit) {
				c.s.log.Debug("read", "err", err)
			}
			leave("")
			return
		}
		msg, err := protocol.Decode(data)
		if err != nil {
			continue // a malformed or out-of-range message is dropped, the connection stays
		}
		ok := true
		switch msg.T {
		case protocol.TypeState:
			if !states.Allow() {
				continue
			}
			ok = w.Handle(c.peer, msg)
		case protocol.TypeProfile:
			// a look this character may not wear only comes from a tampered client: the dressing panel hides it
			if !profiles.Allow() || !c.s.catalog.LookAllowed(*msg.Look, c.caps) {
				continue
			}
			ok = w.Handle(c.peer, msg)
			c.saveProfile(msg)
		case protocol.TypeChat:
			if chats.Allow() {
				ok = w.Handle(c.peer, msg)
			} else {
				ok = w.RateLimited(c.peer)
			}
		case protocol.TypeHello:
			continue
		}
		if !ok {
			c.conn.Close()
			return
		}
	}
}

// arrive resolves the hello's account and archetype into what the world needs to know; it returns an error code
// when the connection must be turned away.
func (c *client) arrive(hello protocol.ClientMsg) (world.Arrival, string) {
	cat := c.s.catalog
	arch := hello.Arch
	if _, ok := cat.Archetype(arch); !ok {
		arch = "" // an unknown id plays without an archetype rather than not at all
	}
	look := cat.AllowedLook(*hello.Look, cat.Grants(arch))
	ctx, cancel := context.WithTimeout(context.Background(), identityWait)
	defer cancel()
	id, kept, err := c.s.ids.Arrive(ctx, hello.Token, identity.Profile{Name: hello.Name, Look: look, Archetype: arch})
	switch {
	case errors.Is(err, identity.ErrUnknownToken):
		return world.Arrival{}, protocol.ErrAuth
	case err != nil:
		// the database is down or slow: better to play as a guest than not at all
		c.s.log.Error("account lookup failed, playing as a guest", "err", err)
		kept = arch
	}
	c.character = id
	c.caps = cat.Grants(kept)
	if kept != arch {
		// the character already had another archetype: what it may wear follows that one
		if allowed := cat.AllowedLook(look, c.caps); allowed != look {
			look = allowed
			c.saveProfile(protocol.ClientMsg{Name: hello.Name, Look: &look})
		}
	}
	return world.Arrival{Name: hello.Name, Look: look, Arch: kept, Apps: cat.AppsFor(c.caps)}, ""
}

// saveProfile stores a name or look change; it only ever logs, the game goes on either way.
func (c *client) saveProfile(m protocol.ClientMsg) {
	if c.character == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), identityWait)
	defer cancel()
	if err := c.s.ids.SaveProfile(ctx, c.character, m.Name, *m.Look); err != nil {
		c.s.log.Warn("saving profile", "character", c.character, "err", err)
	}
}

func (c *client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	out := c.peer.Outbox()
	for {
		select {
		case msg, ok := <-out:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
