// Package world holds the shared world: who is in it, what they wear, where they stand, the vehicles and the chat.
// All of that state is owned by the Run goroutine; peers talk to it through one ordered channel, so there are
// no locks and a peer's join, messages and leave are always handled in the order they happened.
//
// The world never does I/O. Connections (package ws) read the socket, look the account up and hand the world
// events; the world answers by queueing frames on the peer, which the connection writes out.
//
// The files: move.go checks every reported move against the nav grid, view.go decides who sees whom and sends
// the snapshots, vehicle.go parks and mounts vehicles.
package world

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"saigon-noir/server/internal/nav"
	"saigon-noir/server/internal/protocol"
)

// Config tunes the world; DefaultConfig is what the server runs with.
type Config struct {
	MaxPlayers int
	// SnapInterval is how often positions are sent.
	SnapInterval time.Duration
	// ChatHistory is how many recent chat lines a new player receives.
	ChatHistory int
	// SendBuffer is the per-peer outgoing queue; a peer that falls this far behind is dropped.
	SendBuffer int
	// NearbyRadius is how far (m, on the ground) a nearby chat line carries.
	NearbyRadius float64
	Move         Movement
	View         View
}

func DefaultConfig() Config {
	return Config{
		MaxPlayers:   150,
		SnapInterval: time.Second / 12,
		ChatHistory:  30,
		SendBuffer:   64,
		NearbyRadius: 20,
		Move:         DefaultMovement(),
		View:         DefaultView(),
	}
}

// Arrival is a player entering the world: the profile from their hello, already cleaned and validated, its
// archetype as the server keeps it ("" for none), the phone apps that archetype's capabilities give, and where
// they stand.
type Arrival struct {
	// Character is the stored character's id, "" for a guest without an account. A character is in the world
	// once: arriving again replaces the earlier connection, which keeps playing from where that one stood.
	Character string
	Name      string
	Look      protocol.Look
	Arch      string
	Apps      []string
	// Home is the start that respawns go back to (x, z and the heading; Y is taken from the floor); nil for the
	// nav grid's spawn, facing the market.
	Home *protocol.Pose
	// Start is where the player stood at the end of their last visit, nil for none (they start at Home).
	Start *protocol.Pose
	// Vehicle is parked next to the player on arrival and leaves with them; nil for none.
	Vehicle *VehicleSpec
}

// VehicleSpec is a kind of vehicle (server/content/vehicles.json).
type VehicleSpec struct {
	Kind string
	// Speed is its top speed (m/s); moves while riding may reach it times Movement.Tolerance.
	Speed float64
}

// Peer is one participant's link to the world. The world queues outgoing frames on it and closes the queue when
// the participant is removed; the connection writes out what Outbox delivers and closes the socket after.
type Peer struct {
	send chan []byte
	// closed is owned by the world goroutine: set when send has been closed
	closed bool
	// gone is closed with send: from then on the peer's last pose is final
	gone chan struct{}

	// the last accepted pose, for the connection to save; the one value shared with another goroutine
	mu     sync.Mutex
	pose   protocol.Pose
	placed bool
}

// Outbox delivers the frames queued for this peer. It is closed once the world has dropped the peer.
func (p *Peer) Outbox() <-chan []byte { return p.send }

// Gone is closed once the world has dropped the peer (as Outbox is), even if nobody drains the outbox.
func (p *Peer) Gone() <-chan struct{} { return p.gone }

// LastPose is where the world last placed this peer's player, safe to call from any goroutine; ok is false
// until the player has joined.
func (p *Peer) LastPose() (pose protocol.Pose, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pose, p.placed
}

func (p *Peer) setPose(pose protocol.Pose) {
	p.mu.Lock()
	p.pose, p.placed = pose, true
	p.mu.Unlock()
}

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
	id        uint32
	character string
	name      string
	arch      string
	look      protocol.Look
	// pose is the last accepted position, poseAt when it arrived; flags are the state flags shown to others
	pose   protocol.Pose
	poseAt time.Time
	flags  int
	home   protocol.Pose
	gait   gait
	// owns is the vehicle that came with this player (nil for none), riding the one they sit on
	owns   *vehicle
	riding *vehicle
	view   view
}

type World struct {
	cfg    Config
	grid   *nav.Grid
	log    *slog.Logger
	events chan event
	done   chan struct{}
	count  atomic.Int32
	// now is the clock; tests replace it
	now func() time.Time

	// owned by Run
	members  map[*Peer]*member
	vehicles map[uint32]*vehicle
	nextID   uint32
	history  []protocol.ChatLine
	snaps    snapScratch
}

func New(cfg Config, grid *nav.Grid, log *slog.Logger) *World {
	return &World{
		cfg:      cfg,
		grid:     grid,
		log:      log,
		events:   make(chan event, 256),
		done:     make(chan struct{}),
		now:      time.Now,
		members:  make(map[*Peer]*member),
		vehicles: make(map[uint32]*vehicle),
	}
}

// NewPeer makes the link for a new connection; it joins the world with Join.
func (w *World) NewPeer() *Peer {
	return &Peer{send: make(chan []byte, w.cfg.SendBuffer), gone: make(chan struct{})}
}

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
			w.sendSnapshots()
		}
	}
}

func (w *World) submit(e event) bool {
	// once stopped, refuse at once: the send below could still win against done, with nobody left to read it
	select {
	case <-w.done:
		return false
	default:
	}
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
	w.replaceEarlier(&a)
	if len(w.members) >= w.cfg.MaxPlayers {
		w.sendTo(p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrFull}))
		w.closePeer(p)
		return
	}
	w.nextID++
	m := &member{id: w.nextID, character: a.Character, name: a.Name, arch: a.Arch, look: a.Look, view: newView()}
	if m.name == "" {
		m.name = fmt.Sprintf("Khách %d", m.id)
	}
	m.home = w.settle(w.homeOf(a), bodyRadius)
	w.place(p, m, w.startPose(a, m.home))
	others := make([]protocol.Player, 0, len(w.members))
	for _, o := range w.members {
		others = append(others, o.player())
	}
	w.members[p] = m
	w.count.Add(1)
	var bike *vehicle
	if a.Vehicle != nil {
		bike = w.addVehicle(m, *a.Vehicle)
	}
	apps := a.Apps
	if apps == nil {
		apps = []string{}
	}
	w.sendTo(p, protocol.Encode(protocol.Welcome{
		T: protocol.TypeWelcome, ID: m.id, Name: m.name, Arch: m.arch, Apps: apps, MaxPlayers: w.cfg.MaxPlayers,
		Pose: protocol.PoseArray(m.pose), Players: others, Vehicles: w.vehicleList(),
		Chat: append([]protocol.ChatLine{}, w.history...),
	}))
	w.broadcast(protocol.Encode(protocol.Join{T: protocol.TypeJoin, P: m.player()}), p)
	if bike != nil {
		w.broadcast(bike.encode(), p)
	}
	w.log.Info("join", "id", m.id, "name", m.name, "players", len(w.members))
}

// replaceEarlier drops the character's earlier connection, if it is still here (its network went quiet without
// closing, or another tab), and starts the new one where that one stood.
func (w *World) replaceEarlier(a *Arrival) {
	if a.Character == "" {
		return
	}
	for p, m := range w.members {
		if m.character != a.Character {
			continue
		}
		pose := m.pose
		a.Start = &pose
		w.sendTo(p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrReplaced}))
		w.remove(p)
		return
	}
}

func (w *World) onMsg(p *Peer, msg protocol.ClientMsg) {
	m, ok := w.members[p]
	if !ok {
		return
	}
	switch msg.T {
	case protocol.TypeState:
		w.onState(p, m, msg)
	case protocol.TypeRespawn:
		w.onRespawn(p, m, msg)
	case protocol.TypeMount:
		w.onMount(p, m, msg.V)
	case protocol.TypeDismount:
		w.dismount(m)
	case protocol.TypeProfile:
		if msg.Name != "" {
			m.name = msg.Name
		}
		m.look = *msg.Look
		w.broadcast(protocol.Encode(protocol.Profile{T: protocol.TypeProfile, ID: m.id, Name: m.name, Look: m.look}), p)
	case protocol.TypeChat:
		w.onChat(p, m, msg)
	}
}

func (w *World) onChat(p *Peer, m *member, msg protocol.ClientMsg) {
	line := protocol.ChatLine{ID: m.id, Name: m.name, Arch: m.arch, Ch: msg.Ch, Text: msg.Text, At: w.now().UnixMilli()}
	if msg.Ch == protocol.ChannelNearby {
		line.T = protocol.TypeChat
		w.sendNearby(p, m, protocol.Encode(line))
		return
	}
	w.history = append(w.history, line)
	if over := len(w.history) - w.cfg.ChatHistory; over > 0 {
		w.history = append(w.history[:0], w.history[over:]...)
	}
	line.T = protocol.TypeChat
	// the sender gets its own line back too, so every client shows the same, server-cleaned text
	w.broadcast(protocol.Encode(line), nil)
}

// sendNearby gives a nearby chat line to the sender and to every player within NearbyRadius on the ground.
// It is not kept in the history: a late joiner was not there to hear it.
func (w *World) sendNearby(from *Peer, sender *member, b []byte) {
	r2 := w.cfg.NearbyRadius * w.cfg.NearbyRadius
	for p, o := range w.members {
		if p == from {
			continue
		}
		dx, dz := o.pose.X-sender.pose.X, o.pose.Z-sender.pose.Z
		if dx*dx+dz*dz <= r2 {
			w.sendTo(p, b)
		}
	}
	w.sendTo(from, b)
}

func (m *member) player() protocol.Player {
	return protocol.Player{ID: m.id, Name: m.name, Arch: m.arch, Look: m.look}
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
	if !w.offer(p, b) {
		w.remove(p)
	}
}

// offer queues b for p; false when p's queue is full (p is not removed). A closed peer takes nothing, quietly.
func (w *World) offer(p *Peer, b []byte) bool {
	if p.closed {
		return true
	}
	select {
	case p.send <- b:
		return true
	default:
		return false
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
	if v := m.owns; v != nil {
		w.removeVehicle(v)
	}
	w.log.Info("leave", "id", m.id, "name", m.name, "players", len(w.members), "moves_refused", m.gait.refused)
}

func (w *World) closePeer(p *Peer) {
	if !p.closed {
		p.closed = true
		close(p.send)
		close(p.gone)
	}
}
