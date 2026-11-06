// Package ws serves the game socket. Each connection has a reader and a writer goroutine: the reader checks the
// hello, resolves the account (here, so the world loop never waits on the database), rate-limits what the client
// sends and hands the rest to the world; the writer drains the peer's outbox and pings.
package ws

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/quest"
	"saigon-noir/server/internal/world"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = pongWait * 9 / 10
	// identityWait bounds the account lookup in a hello, and each profile or pose save
	identityWait = 3 * time.Second
	// poseSaveEvery: where a character stands is saved this often while it plays, and when it leaves
	poseSaveEvery = 30 * time.Second
)

// Per-connection limits: states arrive at 10 Hz, so 20 Hz leaves room for jitter; chat about one line a second.
var (
	stateRate    = rate.Limit(20)
	stateBurst   = 20
	chatRate     = rate.Every(700 * time.Millisecond)
	chatBurst    = 3
	profileRate  = rate.Every(250 * time.Millisecond)
	profileBurst = 4
	// mount, dismount and respawn: a few presses a second is already more than anyone does
	actionRate  = rate.Every(250 * time.Millisecond)
	actionBurst = 4
	// interact, choose, interact_leave: reading a line and picking an answer takes a moment
	talkRate  = rate.Every(150 * time.Millisecond)
	talkBurst = 6
	// Crab Bike trips: a few presses a second (booking itself is spaced out by the ride service)
	rideRate  = rate.Every(250 * time.Millisecond)
	rideBurst = 4
)

type Config struct {
	// HelloTimeout is how long a new connection may take to send its hello.
	HelloTimeout time.Duration
}

func DefaultConfig() Config { return Config{HelloTimeout: 10 * time.Second} }

// Identity says who a connection is; identity.Service in production.
type Identity interface {
	// Arrive finds the character behind a hello's token, stores the profile it came with and returns the
	// character's id, archetype (kept once chosen) and last pose. An empty token is a guest without an account
	// (no id, the hello's archetype); a token that no account has is identity.ErrUnknownToken.
	Arrive(ctx context.Context, token string, p identity.Profile) (identity.Arrived, error)
	// SaveProfile stores a changed name or look.
	SaveProfile(ctx context.Context, characterID, name string, look protocol.Look) error
	// SavePose stores where the character stands, for its next visit.
	SavePose(ctx context.Context, characterID string, p protocol.Pose) error
}

// Wallets opens a character's wallet as it arrives (its balance goes in the welcome): ridehail.Purse in
// production.
type Wallets interface {
	Arrive(ctx context.Context, character string) (int64, error)
}

// Quests runs a character's quests while it plays: quest.Service in production.
type Quests interface {
	Join(character string, session uint64, who quest.Player)
	Leave(character string, session uint64)
	ChooseTitle(character, title string)
}

type Server struct {
	world   *world.World
	ids     Identity
	catalog *content.Catalog
	cfg     Config
	log     *slog.Logger
	// Quests, when set, hears which characters are playing; nil plays without quests
	Quests Quests
	// Wallets, when set, gives arriving characters their balance; nil plays without wallets
	Wallets Wallets
	// saving counts the connections that still have a pose to save; closing (under mu) stops new ones once
	// Drain has started
	mu      sync.Mutex
	closing bool
	saving  sync.WaitGroup
	// playing are the connections by character, for capabilities earned while playing (under mu)
	playing map[string]*client
	// sessions numbers the visits
	sessions atomic.Uint64
}

// track counts one more connection with a pose to save, unless the server is draining.
func (s *Server) track() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.saving.Add(1)
	return true
}

func New(w *world.World, ids Identity, catalog *content.Catalog, cfg Config, log *slog.Logger) *Server {
	return &Server{world: w, ids: ids, catalog: catalog, cfg: cfg, log: log, playing: map[string]*client{}}
}

// Grant gives a playing character a capability it just earned: what it may wear and play follows at once, and
// so does what NPCs offer it. Safe from any goroutine; a character not playing gets it on its next visit.
func (s *Server) Grant(character, capability string) {
	s.mu.Lock()
	c := s.playing[character]
	s.mu.Unlock()
	if c == nil {
		return
	}
	caps := maps.Clone(*c.caps.Load())
	caps[capability] = struct{}{}
	c.caps.Store(&caps)
	s.world.SetCaps(c.peer, caps)
}

func (s *Server) play(c *client) {
	s.mu.Lock()
	s.playing[c.character] = c
	s.mu.Unlock()
}

func (s *Server) stop(c *client) {
	s.mu.Lock()
	if s.playing[c.character] == c {
		delete(s.playing, c.character)
	}
	s.mu.Unlock()
}

// Drain waits until every connection that has left has saved where its character stood, or ctx ends. Call it
// after the world has stopped and before closing the database.
func (s *Server) Drain(ctx context.Context) error {
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		s.saving.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	// caps are what the character may do (its archetype's grants and the ones it earned): which phone apps it has,
	// which looks it may wear; replaced whole when it earns one (Server.Grant)
	caps atomic.Pointer[capability.Set]
}

func (c *client) can() capability.Set { return *c.caps.Load() }

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
	// counted before joining, so a Drain that starts after the join waits for this connection's last save
	saves := c.character != "" && c.s.track()
	if !w.Join(c.peer, arrival) {
		if saves {
			c.s.saving.Done()
		}
		c.conn.Close()
		return
	}
	if saves {
		go c.keepPose()
	}
	if c.character != "" {
		c.s.play(c)
		defer c.s.stop(c)
		if q := c.s.Quests; q != nil {
			session := c.s.sessions.Add(1)
			q.Join(c.character, session, quest.Player{Tags: c.s.catalog.TagsOf(arrival.Arch)})
			defer q.Leave(c.character, session)
		}
	}

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	states := rate.NewLimiter(stateRate, stateBurst)
	chats := rate.NewLimiter(chatRate, chatBurst)
	profiles := rate.NewLimiter(profileRate, profileBurst)
	actions := rate.NewLimiter(actionRate, actionBurst)
	talks := rate.NewLimiter(talkRate, talkBurst)
	rides := rate.NewLimiter(rideRate, rideBurst)
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
			// an emote this character may not play only comes from a tampered client: the others never see it
			if !c.s.catalog.EmoteAllowed(protocol.EmoteOf(msg.S), c.can()) {
				msg.S &^= protocol.EmoteMask
			}
			ok = w.Handle(c.peer, msg)
		case protocol.TypeMount, protocol.TypeDismount, protocol.TypeRespawn:
			if !actions.Allow() {
				continue
			}
			ok = w.Handle(c.peer, msg)
		case protocol.TypeInteract, protocol.TypeChoose, protocol.TypeInteractLeave:
			// a dropped command gets no result: the game gives up waiting for it after a while
			if !talks.Allow() {
				continue
			}
			ok = w.Handle(c.peer, msg)
		case protocol.TypeDuty, protocol.TypeRideRequest, protocol.TypeRideCancel, protocol.TypeRideAccept, protocol.TypeRideBoard,
			protocol.TypeRideAlight, protocol.TypeRideComplete, protocol.TypeRideRate:
			if !rides.Allow() {
				continue
			}
			ok = w.Handle(c.peer, msg)
		case protocol.TypeProfile:
			// a look this character may not wear only comes from a tampered client: the dressing panel hides it
			if !profiles.Allow() || !c.s.catalog.LookAllowed(*msg.Look, c.can()) {
				continue
			}
			ok = w.Handle(c.peer, msg)
			c.saveProfile(msg)
		case protocol.TypeTitle:
			// whether it was earned is the quest service's to check
			if profiles.Allow() && c.character != "" && c.s.Quests != nil {
				c.s.Quests.ChooseTitle(c.character, msg.Title)
			}
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
	found, err := c.s.ids.Arrive(ctx, hello.Token, identity.Profile{Name: hello.Name, Look: look, Archetype: arch})
	switch {
	case errors.Is(err, identity.ErrUnknownToken):
		return world.Arrival{}, protocol.ErrAuth
	case err != nil:
		// the database is down or slow: better to play as a guest than not at all
		c.s.log.Error("account lookup failed, playing as a guest", "err", err)
		found = identity.Arrived{Archetype: arch}
	}
	kept := found.Archetype
	c.character = found.ID
	caps := cat.Grants(kept)
	for _, earned := range found.Caps {
		caps[earned] = struct{}{}
	}
	c.caps.Store(&caps)
	if kept != arch {
		// the character already had another archetype: what it may wear follows that one
		if allowed := cat.AllowedLook(look, caps); allowed != look {
			look = allowed
			c.saveProfile(protocol.ClientMsg{Name: hello.Name, Look: &look})
		}
	}
	a := world.Arrival{Character: found.ID, Name: hello.Name, Look: look, Arch: kept, Apps: cat.AppsFor(caps), Caps: caps,
		Title: found.Title, Start: found.Pose}
	if at, ok := cat.Archetype(kept); ok {
		x, z, h := at.Spawn.ThreePose()
		a.Home = &protocol.Pose{X: x, Z: z, H: h}
	}
	if v, ok := cat.StarterVehicle(kept, caps); ok {
		a.Vehicle = &world.VehicleSpec{Kind: v.ID, Speed: v.Speed}
	}
	if c.character != "" && c.s.Wallets != nil {
		// a wallet that cannot open plays without one this visit; booking then fails on its own
		if balance, err := c.s.Wallets.Arrive(ctx, c.character); err != nil {
			c.s.log.Error("wallet", "character", c.character, "err", err)
		} else {
			a.Wallet = &balance
		}
	}
	return a, ""
}

// keepPose saves where the character stands every poseSaveEvery while it plays, and once more when the world
// has dropped it (its pose cannot change after that).
// Saving is best effort: a failure is logged and the game goes on.
func (c *client) keepPose() {
	defer c.s.saving.Done()
	ticker := time.NewTicker(poseSaveEvery)
	defer ticker.Stop()
	var saved protocol.Pose
	save := func() {
		p, ok := c.peer.LastPose()
		if !ok || p == saved {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), identityWait)
		defer cancel()
		if err := c.s.ids.SavePose(ctx, c.character, p); err != nil {
			c.s.log.Warn("saving pose", "character", c.character, "err", err)
			return
		}
		saved = p
	}
	for {
		select {
		case <-ticker.C:
			save()
		case <-c.peer.Gone():
			save()
			return
		}
	}
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
