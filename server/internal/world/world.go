// Package world holds the shared world: who is in it, what they wear, where they stand, and the chat.
// All of that state is owned by the Run goroutine; peers talk to it through one ordered channel, so there are
// no locks and a peer's join, messages and leave are always handled in the order they happened.
//
// The world never does I/O. Connections (package ws) read the socket, look the account up and hand the world
// events; the world answers by queueing frames on the peer, which the connection writes out.
package world

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"saigon-noir/server/internal/protocol"
)

// Config tunes the world; DefaultConfig is what the server runs with.
type Config struct {
	MaxPlayers int
	// SnapInterval is how often positions are broadcast.
	SnapInterval time.Duration
	// ChatHistory is how many recent chat lines a new player receives.
	ChatHistory int
	// SendBuffer is the per-peer outgoing queue; a peer that falls this far behind is dropped.
	SendBuffer int
}

func DefaultConfig() Config {
	return Config{
		MaxPlayers:   40,
		SnapInterval: time.Second / 12,
		ChatHistory:  30,
		SendBuffer:   64,
	}
}

// Arrival is a player entering the world: the profile from their hello, already cleaned and validated.
type Arrival struct {
	Name string
	Look protocol.Look
}

// Peer is one participant's link to the world. The world queues outgoing frames on it and closes the queue when
// the participant is removed; the connection writes out what Outbox delivers and closes the socket after.
type Peer struct {
	send chan []byte
	// closed is owned by the world goroutine: set when send has been closed
	closed bool
}

// Outbox delivers the frames queued for this peer. It is closed once the world has dropped the peer.
func (p *Peer) Outbox() <-chan []byte { return p.send }

type eventKind int

const (
	evJoin eventKind = iota
	evMsg
	evRateLimited
	evLeave
)

type event struct {
	kind    eventKind
	p       *Peer
	arrival Arrival
	msg     protocol.ClientMsg
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

type World struct {
	cfg    Config
	log    *slog.Logger
	events chan event
	done   chan struct{}
	count  atomic.Int32

	// owned by Run
	members map[*Peer]*member
	nextID  uint32
	history []protocol.ChatLine
	moved   bool
}

func New(cfg Config, log *slog.Logger) *World {
	return &World{
		cfg:     cfg,
		log:     log,
		events:  make(chan event, 256),
		done:    make(chan struct{}),
		members: make(map[*Peer]*member),
	}
}

// NewPeer makes the link for a new connection; it joins the world with Join.
func (w *World) NewPeer() *Peer { return &Peer{send: make(chan []byte, w.cfg.SendBuffer)} }

// Players is the number of joined players, safe to call from any goroutine.
func (w *World) Players() int { return int(w.count.Load()) }

// Join, Handle, RateLimited and Leave hand an event to Run, in the order they are called. They return false
// once the world has stopped: nobody will close the peer's queue then, so the caller closes its connection.

// Join adds the peer to the world (or refuses it with "full").
func (w *World) Join(p *Peer, a Arrival) bool { return w.submit(event{kind: evJoin, p: p, arrival: a}) }

// Handle applies a decoded message from a joined peer.
func (w *World) Handle(p *Peer, m protocol.ClientMsg) bool {
	return w.submit(event{kind: evMsg, p: p, msg: m})
}

// RateLimited tells the peer its last chat line was dropped for being too fast.
func (w *World) RateLimited(p *Peer) bool { return w.submit(event{kind: evRateLimited, p: p}) }

// Leave removes the peer, sending the error code first when there is one.
func (w *World) Leave(p *Peer, code string) bool {
	return w.submit(event{kind: evLeave, p: p, code: code})
}

// Run owns the world until ctx is cancelled, then closes every peer.
func (w *World) Run(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(w.cfg.SnapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			for p := range w.members {
				w.closePeer(p)
			}
			return
		case e := <-w.events:
			w.handle(e)
		case <-ticker.C:
			w.broadcastSnap()
		}
	}
}

func (w *World) submit(e event) bool {
	select {
	case w.events <- e:
		return true
	case <-w.done:
		return false
	}
}

func (w *World) handle(e event) {
	switch e.kind {
	case evJoin:
		w.onJoin(e.p, e.arrival)
	case evMsg:
		w.onMsg(e.p, e.msg)
	case evRateLimited:
		w.sendTo(e.p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrRate}))
	case evLeave:
		if e.code != "" {
			w.sendTo(e.p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: e.code}))
		}
		w.remove(e.p)
	}
}

func (w *World) onJoin(p *Peer, a Arrival) {
	if p.closed {
		return
	}
	if len(w.members) >= w.cfg.MaxPlayers {
		w.sendTo(p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrFull}))
		w.closePeer(p)
		return
	}
	w.nextID++
	m := &member{id: w.nextID, name: a.Name, look: a.Look}
	if m.name == "" {
		m.name = fmt.Sprintf("Khách %d", m.id)
	}
	others := make([]protocol.Player, 0, len(w.members))
	for _, o := range w.members {
		others = append(others, o.player())
	}
	w.members[p] = m
	w.count.Add(1)
	w.sendTo(p, protocol.Encode(protocol.Welcome{
		T: protocol.TypeWelcome, ID: m.id, Name: m.name, MaxPlayers: w.cfg.MaxPlayers,
		Players: others, Chat: append([]protocol.ChatLine{}, w.history...),
	}))
	w.broadcast(protocol.Encode(protocol.Join{T: protocol.TypeJoin, P: m.player()}), p)
	w.log.Info("join", "id", m.id, "name", m.name, "players", len(w.members))
}

func (w *World) onMsg(p *Peer, msg protocol.ClientMsg) {
	m, ok := w.members[p]
	if !ok {
		return
	}
	switch msg.T {
	case protocol.TypeState:
		m.pose = protocol.Pose{X: msg.X, Y: msg.Y, Z: msg.Z, H: msg.H}
		m.poseAt = time.Now()
		m.placed = true
		w.moved = true
	case protocol.TypeProfile:
		if msg.Name != "" {
			m.name = msg.Name
		}
		m.look = *msg.Look
		w.broadcast(protocol.Encode(protocol.Profile{T: protocol.TypeProfile, ID: m.id, Name: m.name, Look: m.look}), p)
	case protocol.TypeChat:
		line := protocol.ChatLine{ID: m.id, Name: m.name, Text: msg.Text, At: time.Now().UnixMilli()}
		w.history = append(w.history, line)
		if over := len(w.history) - w.cfg.ChatHistory; over > 0 {
			w.history = append(w.history[:0], w.history[over:]...)
		}
		line.T = protocol.TypeChat
		// the sender gets its own line back too, so every client shows the same, server-cleaned text
		w.broadcast(protocol.Encode(line), nil)
	}
}

// broadcastSnap sends every placed player's pose, but only when someone moved since the last one.
func (w *World) broadcastSnap() {
	if !w.moved {
		return
	}
	w.moved = false
	now := time.Now()
	snap := protocol.Snap{T: protocol.TypeSnap, At: now.UnixMilli(), P: make([][6]float64, 0, len(w.members))}
	for _, m := range w.members {
		if !m.placed {
			continue
		}
		p := protocol.PoseArray(m.pose)
		age := float64(now.Sub(m.poseAt).Milliseconds())
		snap.P = append(snap.P, [6]float64{float64(m.id), p[0], p[1], p[2], p[3], age})
	}
	w.broadcast(protocol.Encode(snap), nil)
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
func (w *World) broadcast(b []byte, except *Peer) {
	var slow []*Peer
	for p := range w.members {
		if p == except || p.closed {
			continue
		}
		select {
		case p.send <- b:
		default:
			slow = append(slow, p)
		}
	}
	for _, p := range slow {
		// an earlier removal's own broadcast may already have dropped this one
		if m, ok := w.members[p]; ok {
			w.log.Warn("dropping slow client", "id", m.id)
			w.remove(p)
		}
	}
}

func (w *World) sendTo(p *Peer, b []byte) {
	if p.closed {
		return
	}
	select {
	case p.send <- b:
	default:
		w.remove(p)
	}
}

// remove forgets a peer, closes its queue (its connection then closes the socket) and tells everyone else.
func (w *World) remove(p *Peer) {
	m, ok := w.members[p]
	w.closePeer(p)
	if !ok {
		return
	}
	delete(w.members, p)
	w.count.Add(-1)
	w.broadcast(protocol.Encode(protocol.Leave{T: protocol.TypeLeave, ID: m.id}), nil)
	w.log.Info("leave", "id", m.id, "name", m.name, "players", len(w.members))
}

func (w *World) closePeer(p *Peer) {
	if !p.closed {
		p.closed = true
		close(p.send)
	}
}
