// Package hub holds the shared world: who is connected, what they wear, where they stand, and the chat.
// All of that state is owned by the Run goroutine; connections talk to it through one ordered channel, so
// there are no locks and a client's join, messages and leave are always handled in the order they happened.
package hub

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"saigon-noir/server/internal/protocol"
)

// Config tunes the hub; DefaultConfig is what the server runs with.
type Config struct {
	MaxPlayers int
	// SnapInterval is how often positions are broadcast.
	SnapInterval time.Duration
	// HelloTimeout is how long a new connection may take to send its hello.
	HelloTimeout time.Duration
	// ChatHistory is how many recent chat lines a new player receives.
	ChatHistory int
	// SendBuffer is the per-client outgoing queue; a client that falls this far behind is dropped.
	SendBuffer int
}

func DefaultConfig() Config {
	return Config{
		MaxPlayers:   40,
		SnapInterval: time.Second / 12,
		HelloTimeout: 10 * time.Second,
		ChatHistory:  30,
		SendBuffer:   64,
	}
}

type eventKind int

const (
	evJoin eventKind = iota
	evMsg
	evRateLimited
	evLeave
)

type event struct {
	kind eventKind
	c    *Client
	msg  protocol.ClientMsg
	// code, for evLeave: an error to send before closing ("" for a plain disconnect)
	code string
}

type member struct {
	id     uint32
	name   string
	look   protocol.Look
	pose   protocol.Pose
	poseAt time.Time
	placed bool
}

type Hub struct {
	cfg    Config
	log    *slog.Logger
	events chan event
	done   chan struct{}
	count  atomic.Int32

	// owned by Run
	members map[*Client]*member
	nextID  uint32
	history []protocol.ChatLine
	moved   bool
}

func New(cfg Config, log *slog.Logger) *Hub {
	return &Hub{
		cfg:     cfg,
		log:     log,
		events:  make(chan event, 256),
		done:    make(chan struct{}),
		members: make(map[*Client]*member),
	}
}

// Players is the number of joined players, safe to call from any goroutine.
func (h *Hub) Players() int { return int(h.count.Load()) }

// Run owns the world until ctx is cancelled, then closes every connection.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	ticker := time.NewTicker(h.cfg.SnapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			for c := range h.members {
				h.closeClient(c)
			}
			return
		case e := <-h.events:
			h.handle(e)
		case <-ticker.C:
			h.broadcastSnap()
		}
	}
}

// submit hands an event to Run; false once the hub has stopped.
func (h *Hub) submit(e event) bool {
	select {
	case h.events <- e:
		return true
	case <-h.done:
		return false
	}
}

func (h *Hub) handle(e event) {
	switch e.kind {
	case evJoin:
		h.onJoin(e.c, e.msg)
	case evMsg:
		h.onMsg(e.c, e.msg)
	case evRateLimited:
		h.sendTo(e.c, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrRate}))
	case evLeave:
		if e.code != "" {
			h.sendTo(e.c, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: e.code}))
		}
		h.remove(e.c)
	}
}

func (h *Hub) onJoin(c *Client, hello protocol.ClientMsg) {
	if c.closed {
		return
	}
	if len(h.members) >= h.cfg.MaxPlayers {
		h.sendTo(c, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrFull}))
		h.closeClient(c)
		return
	}
	h.nextID++
	m := &member{id: h.nextID, name: hello.Name, look: *hello.Look}
	if m.name == "" {
		m.name = fmt.Sprintf("Khách %d", m.id)
	}
	others := make([]protocol.Player, 0, len(h.members))
	for _, o := range h.members {
		others = append(others, o.player())
	}
	h.members[c] = m
	h.count.Add(1)
	h.sendTo(c, protocol.Encode(protocol.Welcome{
		T: protocol.TypeWelcome, ID: m.id, Name: m.name, MaxPlayers: h.cfg.MaxPlayers,
		Players: others, Chat: append([]protocol.ChatLine{}, h.history...),
	}))
	h.broadcast(protocol.Encode(protocol.Join{T: protocol.TypeJoin, P: m.player()}), c)
	h.log.Info("join", "id", m.id, "name", m.name, "players", len(h.members))
}

func (h *Hub) onMsg(c *Client, msg protocol.ClientMsg) {
	m, ok := h.members[c]
	if !ok {
		return
	}
	switch msg.T {
	case protocol.TypeState:
		m.pose = protocol.Pose{X: msg.X, Y: msg.Y, Z: msg.Z, H: msg.H}
		m.poseAt = time.Now()
		m.placed = true
		h.moved = true
	case protocol.TypeProfile:
		if msg.Name != "" {
			m.name = msg.Name
		}
		m.look = *msg.Look
		h.broadcast(protocol.Encode(protocol.Profile{T: protocol.TypeProfile, ID: m.id, Name: m.name, Look: m.look}), c)
	case protocol.TypeChat:
		line := protocol.ChatLine{ID: m.id, Name: m.name, Text: msg.Text, At: time.Now().UnixMilli()}
		h.history = append(h.history, line)
		if over := len(h.history) - h.cfg.ChatHistory; over > 0 {
			h.history = append(h.history[:0], h.history[over:]...)
		}
		line.T = protocol.TypeChat
		// the sender gets its own line back too, so every client shows the same, server-cleaned text
		h.broadcast(protocol.Encode(line), nil)
	}
}

// broadcastSnap sends every placed player's pose, but only when someone moved since the last one.
func (h *Hub) broadcastSnap() {
	if !h.moved {
		return
	}
	h.moved = false
	now := time.Now()
	snap := protocol.Snap{T: protocol.TypeSnap, At: now.UnixMilli(), P: make([][6]float64, 0, len(h.members))}
	for _, m := range h.members {
		if !m.placed {
			continue
		}
		p := protocol.PoseArray(m.pose)
		age := float64(now.Sub(m.poseAt).Milliseconds())
		snap.P = append(snap.P, [6]float64{float64(m.id), p[0], p[1], p[2], p[3], age})
	}
	h.broadcast(protocol.Encode(snap), nil)
}

func (m *member) player() protocol.Player {
	p := protocol.Player{ID: m.id, Name: m.name, Look: m.look}
	if m.placed {
		pose := protocol.PoseArray(m.pose)
		p.Pose = &pose
	}
	return p
}

// broadcast queues b for every member except `except`; members whose queue is full are dropped.
func (h *Hub) broadcast(b []byte, except *Client) {
	var slow []*Client
	for c := range h.members {
		if c == except || c.closed {
			continue
		}
		select {
		case c.send <- b:
		default:
			slow = append(slow, c)
		}
	}
	for _, c := range slow {
		// an earlier removal's own broadcast may already have dropped this one
		if m, ok := h.members[c]; ok {
			h.log.Warn("dropping slow client", "id", m.id)
			h.remove(c)
		}
	}
}

func (h *Hub) sendTo(c *Client, b []byte) {
	if c.closed {
		return
	}
	select {
	case c.send <- b:
	default:
		h.remove(c)
	}
}

// remove forgets a client, closes its queue (its writer then closes the socket) and tells everyone else.
func (h *Hub) remove(c *Client) {
	m, ok := h.members[c]
	h.closeClient(c)
	if !ok {
		return
	}
	delete(h.members, c)
	h.count.Add(-1)
	h.broadcast(protocol.Encode(protocol.Leave{T: protocol.TypeLeave, ID: m.id}), nil)
	h.log.Info("leave", "id", m.id, "name", m.name, "players", len(h.members))
}

func (h *Hub) closeClient(c *Client) {
	if !c.closed {
		c.closed = true
		close(c.send)
	}
}
