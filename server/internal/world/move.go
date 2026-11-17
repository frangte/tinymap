package world

import (
	"math"
	"math/rand/v2"
	"time"

	"saigon-noir/server/internal/protocol"
)

// Movement is what a reported move is checked against (docs/design/04-backend.md, "What the server enforces").
// The game moves its own player through the map's own shapes (src/game/collider.ts) and reports where it got to,
// about ten times a second. The server knows nothing of walls and floors: it takes the game's word for where its
// player stands, and checks only that nobody moves faster than they can (and stays on the map: protocol.WorldHalf).
// It keeps the last position it accepted and answers a move it refuses with a correction back to it.
type Movement struct {
	// RunSpeed is the top speed on foot (m/s, server/content/movement.json); a vehicle brings its own.
	RunSpeed float64
	// Tolerance multiplies the top speed, for timing jitter between the game's frames and its reports.
	Tolerance float64
	// Window is how long unused movement can be saved up: after standing still, a player may cover
	// speed × Tolerance × Window at once (reports bunched up by the network), plus Slack.
	Window time.Duration
	Slack  float64
	// HomeRadius is how close to home a respawn must land (m): the game scatters arrivals and finds a free spot.
	HomeRadius float64
	// MountReach is how close to a vehicle a player must be to get on (m).
	MountReach float64
}

func DefaultMovement() Movement {
	return Movement{
		RunSpeed:   6.8,
		Tolerance:  1.25,
		Window:     time.Second,
		Slack:      0.5,
		HomeRadius: 4,
		MountReach: 2.5,
	}
}

const (
	// arrivalSpread scatters arrivals at home up to this far on each axis (m), so players who arrive together do
	// not stand inside each other (the game's own respawn scatters the same way); the game then finds the nearest
	// spot where the body fits
	arrivalSpread = 1.5
	// refusalLogEvery: the first refused move of a player is logged, then every this many
	refusalLogEvery = 50
)

// gait is a member's movement check state.
type gait struct {
	// credit is how far the player may still move right now (m), topped up with time at the top speed
	credit float64
	at     time.Time
	// k counts the corrections sent; reports that do not echo it were sent before the latest one arrived
	k       uint32
	refused int
}

// place puts a member at a pose the server chose (arrival, respawn) with a full movement allowance.
func (w *World) place(p *Peer, m *member, pose protocol.Pose) {
	now := w.now()
	m.pose, m.poseAt = pose, now
	m.gait.at = now
	m.gait.credit = w.allowance(m)
	m.view.moved = true
	p.setPose(pose)
}

// topSpeed is the fastest a member may move now (m/s), tolerance included.
func (w *World) topSpeed(m *member) float64 {
	speed := w.cfg.Move.RunSpeed
	if m.riding != nil {
		speed = m.riding.speed
	}
	return speed * w.cfg.Move.Tolerance
}

// allowance is the most movement a member can save up (m).
func (w *World) allowance(m *member) float64 {
	return w.topSpeed(m)*w.cfg.Move.Window.Seconds() + w.cfg.Move.Slack
}

func (w *World) onState(p *Peer, m *member, msg protocol.ClientMsg) {
	if m.seated != nil {
		return // a passenger is carried: where it is comes from the vehicle, whatever its client says
	}
	if msg.K != m.gait.k {
		return // sent before the client had our latest correction: it is on its way back already
	}
	to := protocol.Pose{X: msg.X, Y: msg.Y, Z: msg.Z, H: msg.H}
	if why := w.checkMove(m, to); why != "" {
		w.refuse(p, m, why)
		return
	}
	flags := msg.S &^ (protocol.FlagRide | protocol.FlagSeat1)
	if m.riding != nil {
		flags = protocol.FlagRide // no jumping or emotes on a bike
		m.riding.pose = to
	}
	if to != m.pose || flags != m.flags {
		m.view.moved = true
	}
	from, oldFlags, fromAt := m.pose, m.flags, m.poseAt
	m.pose, m.poseAt, m.flags = to, w.now(), flags
	p.setPose(to)
	m.trackSpeed(from, m.poseAt.Sub(fromAt))
	if m.riding != nil {
		w.carry(m.riding)
	}
	w.observeMove(m, from, oldFlags)
}

// checkMove says why a move from the member's accepted pose to `to` is refused, or "" if it is fine: faster than
// the member can go (a teleport is the same thing). Where it ends up (a wall, the air) is the game's to keep right.
func (w *World) checkMove(m *member, to protocol.Pose) string {
	now := w.now()
	g := &m.gait
	g.credit = math.Min(w.allowance(m), g.credit+w.topSpeed(m)*now.Sub(g.at).Seconds())
	g.at = now
	d := math.Hypot(to.X-m.pose.X, to.Z-m.pose.Z)
	if d > g.credit {
		return "speed"
	}
	g.credit -= d
	return ""
}

// refuse sends the member back to its last accepted pose.
func (w *World) refuse(p *Peer, m *member, why string) {
	g := &m.gait
	g.k++
	g.refused++
	if g.refused == 1 || g.refused%refusalLogEvery == 0 {
		w.log.Warn("move refused", "id", m.id, "name", m.name, "why", why, "refused", g.refused)
	}
	w.sendTo(p, protocol.Encode(protocol.Correct{T: protocol.TypeCorrect, K: g.k, Pose: protocol.PoseArray(m.pose)}))
}

// onRespawn is the one teleport there is: back to the member's home, near enough that the game's scatter and its
// search for a spot where the body fits explain the distance. The member's own vehicle comes along.
func (w *World) onRespawn(p *Peer, m *member, msg protocol.ClientMsg) {
	// the game gets off the bike before it respawns, whether or not the respawn lands: so does the server
	w.dismount(m)
	// a passenger respawning gets off without a correction: the respawn places them (or its refusal does)
	w.putDown(m, false)
	if msg.K != m.gait.k {
		return
	}
	to := protocol.Pose{X: msg.X, Y: msg.Y, Z: msg.Z, H: msg.H}
	if math.Hypot(to.X-m.home.X, to.Z-m.home.Z) > w.cfg.Move.HomeRadius {
		w.refuse(p, m, "respawn")
		return
	}
	m.flags = 0
	w.place(p, m, to)
	if v := m.owns; v != nil {
		w.parkBeside(v, to)
		w.broadcast(v.encode(), nil)
	}
}

// homeOf is an arrival's home: its own, or the map's spawn facing the market.
func (w *World) homeOf(a Arrival) protocol.Pose { return w.homeAt(a.Home) }

// homeAt is a home given (an archetype's start), or without one the map's spawn (content map.json) facing the
// market (west, three.js -X; heading 0 is +Z). Its height is the game's to find (on the ground there).
func (w *World) homeAt(home *protocol.Pose) protocol.Pose {
	if home != nil {
		return *home
	}
	return protocol.Pose{X: w.cfg.Spawn[0], Z: w.cfg.Spawn[1], H: -math.Pi / 2}
}

// startPose is where an arriving player stands: where they left off, else a little way from home. The game moves
// them to the nearest spot their body fits, should the map have changed under it since.
func (w *World) startPose(a Arrival, home protocol.Pose) protocol.Pose {
	if s := a.Start; s != nil && math.Abs(s.X) <= protocol.WorldHalf && math.Abs(s.Z) <= protocol.WorldHalf {
		return *s
	}
	spread := func() float64 { return (rand.Float64()*2 - 1) * arrivalSpread }
	return protocol.Pose{X: home.X + spread(), Y: home.Y, Z: home.Z + spread(), H: home.H}
}
