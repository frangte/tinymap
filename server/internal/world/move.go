package world

import (
	"math"
	"math/rand/v2"
	"time"

	"saigon-noir/server/internal/protocol"
)

// Movement is what a reported move is checked against (docs/design/04-backend.md, "What the server enforces").
// The game moves its own player and reports where it got to, about ten times a second; the server keeps the last
// position it accepted and answers a move it refuses with a correction back to it.
type Movement struct {
	// RunSpeed is the top speed on foot (m/s, server/content/movement.json); a vehicle brings its own.
	RunSpeed float64
	// Tolerance multiplies the top speed, for timing jitter between the game's frames and its reports.
	Tolerance float64
	// Window is how long unused movement can be saved up: after standing still, a player may cover
	// speed × Tolerance × Window at once (reports bunched up by the network), plus Slack.
	Window time.Duration
	Slack  float64
	// MaxGap is how much blocked ground the straight line between two reports may cross (m): the path between
	// them is not straight, and going round a corner the line clips it.
	MaxGap float64
	// Below and Above bound the reported height around the floor nearby (m): the game eases up kerbs, and jumps.
	Below, Above float64
	// MaxClimb is the biggest floor change between neighbouring cells a move may cross (m): a kerb in a stride,
	// a little more in a jump, never a ledge.
	MaxClimb float64
	// Aloft: higher than this above the floor nearby (m) counts as in the air, which may last MaxAir at most.
	// A jump lasts about 0.6 s of game time, but a slow device runs the game slower than the clock (a frame
	// counts 50 ms at most): at 8 fps the same jump takes 1.5 s. Someone hovering longer is put back down.
	Aloft  float64
	MaxAir time.Duration
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
		MaxGap:     0.2,
		Below:      0.6,
		Above:      1.5,
		MaxClimb:   1.2,
		Aloft:      0.3,
		MaxAir:     2500 * time.Millisecond,
		HomeRadius: 4,
		MountReach: 2.5,
	}
}

// bodyRadius is a player on the nav grid (src/game/player.ts), vehicleRadius a parked motorbike (m).
const (
	bodyRadius    = 0.26
	vehicleRadius = 0.45
	// settleSearch is how far settle looks for a free spot (m)
	settleSearch = 8
	// arrivalSpread scatters arrivals at home up to this far on each axis (m), so players who arrive together do
	// not stand inside each other (the game's own respawn scatters the same way)
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
	// airSince is when the player last left the ground (zero while on it)
	airSince time.Time
}

// place puts a member at a pose the server chose (arrival, respawn) with a full movement allowance.
func (w *World) place(p *Peer, m *member, pose protocol.Pose) {
	now := w.now()
	m.pose, m.poseAt = pose, now
	m.gait.at = now
	m.gait.credit = w.allowance(m)
	m.gait.airSince = time.Time{}
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
	if msg.K != m.gait.k {
		return // sent before the client had our latest correction: it is on its way back already
	}
	to := protocol.Pose{X: msg.X, Y: msg.Y, Z: msg.Z, H: msg.H}
	if why := w.checkMove(m, to); why != "" {
		if why == "hover" {
			// back on the ground where it last stood, for everyone
			m.pose.Y, _ = w.grid.Floor(m.pose.X, m.pose.Z)
			m.gait.airSince = time.Time{}
			m.view.moved = true
			p.setPose(m.pose)
		}
		w.refuse(p, m, why)
		return
	}
	flags := msg.S &^ protocol.FlagRide
	if m.riding != nil {
		flags = protocol.FlagRide // no jumping or emotes on a bike
		m.riding.pose = to
	}
	if to != m.pose || flags != m.flags {
		m.view.moved = true
	}
	from, oldFlags := m.pose, m.flags
	m.pose, m.poseAt, m.flags = to, w.now(), flags
	p.setPose(to)
	w.observeMove(m, from, oldFlags)
}

// checkMove says why a move from the member's accepted pose to `to` is refused, or "" if it is fine.
func (w *World) checkMove(m *member, to protocol.Pose) string {
	mv := w.cfg.Move
	now := w.now()
	g := &m.gait
	g.credit = math.Min(w.allowance(m), g.credit+w.topSpeed(m)*now.Sub(g.at).Seconds())
	g.at = now
	d := math.Hypot(to.X-m.pose.X, to.Z-m.pose.Z)
	switch {
	case d > g.credit:
		return "speed"
	case !w.grid.Standable(to.X, to.Z):
		return "blocked"
	case !w.grid.Clear(m.pose.X, m.pose.Z, to.X, to.Z, mv.MaxGap):
		return "wall"
	}
	lo, hi, _ := w.grid.FloorRange(to.X, to.Z)
	if to.Y < lo-mv.Below || to.Y > hi+mv.Above {
		return "height"
	}
	if w.grid.Climb(m.pose.X, m.pose.Z, to.X, to.Z) > mv.MaxClimb {
		return "climb"
	}
	aloft := to.Y > hi+mv.Aloft
	if aloft && !g.airSince.IsZero() && now.Sub(g.airSince) > mv.MaxAir {
		return "hover"
	}
	switch {
	case !aloft:
		g.airSince = time.Time{}
	case g.airSince.IsZero():
		g.airSince = now
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
// search for a free spot explain the distance. The member's own vehicle comes along.
func (w *World) onRespawn(p *Peer, m *member, msg protocol.ClientMsg) {
	// the game gets off the bike before it respawns, whether or not the respawn lands: so does the server
	w.dismount(m)
	if msg.K != m.gait.k {
		return
	}
	to := protocol.Pose{X: msg.X, Z: msg.Z, H: msg.H}
	if math.Hypot(to.X-m.home.X, to.Z-m.home.Z) > w.cfg.Move.HomeRadius || !w.grid.Standable(to.X, to.Z) {
		w.refuse(p, m, "respawn")
		return
	}
	to.Y, _ = w.grid.Floor(to.X, to.Z)
	m.flags = 0
	w.place(p, m, to)
	if v := m.owns; v != nil {
		w.parkBeside(v, to)
		w.broadcast(v.encode(), nil)
	}
}

// homeOf is an arrival's home: its own, or the nav grid's spawn facing the market (three.js -z, Blender +Y).
func (w *World) homeOf(a Arrival) protocol.Pose {
	if a.Home != nil {
		return *a.Home
	}
	x, z := w.grid.Spawn()
	return protocol.Pose{X: x, Z: z, H: math.Pi}
}

// startPose is where an arriving player stands: where they left off, unless that spot is no longer walkable
// (the map changed), else a little way from home.
func (w *World) startPose(a Arrival, home protocol.Pose) protocol.Pose {
	if s := a.Start; s != nil && w.grid.Fits(s.X, s.Z, bodyRadius) {
		y, _ := w.grid.Floor(s.X, s.Z)
		return protocol.Pose{X: s.X, Y: y, Z: s.Z, H: s.H}
	}
	spread := func() float64 { return (rand.Float64()*2 - 1) * arrivalSpread }
	return w.settle(protocol.Pose{X: home.X + spread(), Z: home.Z + spread(), H: home.H}, bodyRadius)
}

// settle moves a pose to the nearest spot where a body of radius r fits, at floor height (the nav grid's spawn
// when there is none nearby).
func (w *World) settle(p protocol.Pose, r float64) protocol.Pose {
	x, z, ok := w.grid.NearestFit(p.X, p.Z, r, settleSearch)
	if !ok {
		x, z = w.grid.Spawn()
	}
	y, _ := w.grid.Floor(x, z)
	return protocol.Pose{X: x, Y: y, Z: z, H: p.H}
}
