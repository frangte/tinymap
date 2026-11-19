package world

import (
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/protocol"
)

// The tests drive the world by hand: no Run goroutine, a clock that only moves when told to, and peers whose
// queues the test reads.

type harness struct {
	t     *testing.T
	w     *World
	clock time.Time
	// island is the map's frame and spawn (content map.json)
	island content.Map
}

func newHarness(t *testing.T) *harness { return newHarnessWith(t, nil) }

// newHarnessWith lets a test change the configuration first (place NPCs, say).
func newHarnessWith(t *testing.T, tune func(*Config)) *harness {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.SendBuffer = 4096
	cfg.Spawn[0], cfg.Spawn[1] = c.Map.ThreeSpawn()
	if tune != nil {
		tune(&cfg)
	}
	h := &harness{t: t, island: c.Map, clock: time.Unix(1_700_000_000, 0)}
	h.w = New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.w.now = func() time.Time { return h.clock }
	return h
}

func (h *harness) tick(d time.Duration) { h.clock = h.clock.Add(d) }

func (h *harness) join(a Arrival) (*Peer, *member) {
	h.t.Helper()
	p := h.w.NewPeer()
	h.w.onJoin(p, a)
	m := h.w.members[p]
	if m == nil {
		h.t.Fatal("not joined")
	}
	return p, m
}

func (h *harness) state(p *Peer, m *member, x, z float64, s int) {
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: x, Y: m.pose.Y, Z: z, H: 0, S: s, K: m.gait.k})
}

type frame struct {
	T     string          `json:"t"`
	Code  string          `json:"code"`
	K     uint32          `json:"k"`
	ID    uint32          `json:"id"`
	Rider uint32          `json:"rider"`
	Owner uint32          `json:"owner"`
	Pose  []float64       `json:"pose"`
	P     json.RawMessage `json:"p"`
	Out   []uint32        `json:"out"`
	V     json.RawMessage `json:"vehicles"`
	NPCs  []protocol.NPC  `json:"npcs"`
	// conversations, results and chat
	Sid     uint32          `json:"sid"`
	N       uint32          `json:"n"`
	Mode    string          `json:"mode"`
	Step    uint32          `json:"step"`
	Lead    bool            `json:"lead"`
	Size    int             `json:"size"`
	Say     protocol.Text   `json:"say"`
	Choices []protocol.Text `json:"choices"`
	Reason  string          `json:"reason"`
	Rid     uint32          `json:"rid"`
	OK      bool            `json:"ok"`
	Name    string          `json:"name"`
	Text    string          `json:"text"`
	EN      string          `json:"en"`
	NPC     bool            `json:"npc"`
	// archetype changes and profiles
	Arch string        `json:"arch"`
	Apps []string      `json:"apps"`
	Look protocol.Look `json:"look"`
}

// drain returns every frame queued for p, and how many bytes they were.
func (h *harness) drain(p *Peer) ([]frame, int) {
	h.t.Helper()
	var out []frame
	n := 0
	for {
		select {
		case b, ok := <-p.send:
			if !ok {
				return out, n
			}
			n += len(b)
			var f frame
			if err := json.Unmarshal(b, &f); err != nil {
				h.t.Fatalf("bad frame %s: %v", b, err)
			}
			out = append(out, f)
		default:
			return out, n
		}
	}
}

func count(fs []frame, t string) int {
	n := 0
	for _, f := range fs {
		if f.T == t {
			n++
		}
	}
	return n
}

// lane is a straight run of `length` metres from (x, z) to walk up and down: east. The server knows nothing of
// walls, so any will do.
func (h *harness) lane(x, z, _ float64) (x0, z0, dx, dz float64) { return x, z, 1, 0 }

// walk moves m up and down a lane of `length` metres at `speed` m/s, reporting ten times a second, and returns
// how many corrections came.
func (h *harness) walk(p *Peer, m *member, x0, z0, dx, dz, length, speed, seconds float64) int {
	h.t.Helper()
	h.drain(p)
	steps := int(seconds * 10)
	for i := 1; i <= steps; i++ {
		h.tick(100 * time.Millisecond)
		d := math.Mod(speed*0.1*float64(i), 2*length)
		if d > length {
			d = 2*length - d
		}
		h.state(p, m, x0+dx*d, z0+dz*d, 0)
	}
	fs, _ := h.drain(p)
	return count(fs, protocol.TypeCorrect)
}

// at puts a member at a spot as if it had walked there (tests only).
func (h *harness) at(p *Peer, m *member, x, z float64) {
	h.w.place(p, m, protocol.Pose{X: x, Z: z})
}

func TestHonestWalkingAndRunningPass(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	x0, z0, dx, dz := m.pose.X, m.pose.Z, 1.0, 0.0
	h.at(p, m, x0, z0)
	h.tick(time.Second)
	if n := h.walk(p, m, x0, z0, dx, dz, 12, 3.1, 3); n != 0 {
		t.Fatalf("walking: %d corrections", n)
	}
	h.at(p, m, x0, z0)
	h.tick(time.Second)
	if n := h.walk(p, m, x0, z0, dx, dz, 12, 6.8, 1.7); n != 0 {
		t.Fatalf("running: %d corrections", n)
	}
	if m.gait.refused != 0 {
		t.Fatalf("refused %d", m.gait.refused)
	}
}

func TestSpeedingIsCorrected(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	x0, z0, dx, dz := m.pose.X, m.pose.Z, 1.0, 0.0
	h.at(p, m, x0, z0)
	h.drain(p)
	// twice the running speed for 3 s: the saved-up second lets the first ones through, then the server says no
	for i := 1; i <= 30; i++ {
		h.tick(100 * time.Millisecond)
		d := 13.6 * 0.1 * float64(i)
		h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: x0 + dx*d, Y: m.pose.Y, Z: z0 + dz*d, K: 0})
	}
	fs, _ := h.drain(p)
	if count(fs, protocol.TypeCorrect) != 1 {
		t.Fatalf("frames %+v: want exactly one correction (the rest are stale, k=0)", fs)
	}
	var c frame
	for _, f := range fs {
		if f.T == protocol.TypeCorrect {
			c = f
		}
	}
	if c.K != 1 || math.Hypot(c.Pose[0]-m.pose.X, c.Pose[2]-m.pose.Z) > 0.01 {
		t.Fatalf("correction %+v, server pose %+v", c, m.pose)
	}
	mv := DefaultMovement()
	if moved, most := math.Hypot(m.pose.X-x0, m.pose.Z-z0), mv.RunSpeed*mv.Tolerance*4+mv.Slack; moved > most {
		t.Fatalf("got %v m in 3 s, at most %v allowed", moved, most)
	}
	// once the client has the correction (k=1), it moves on from the corrected pose
	before := m.pose
	h.tick(100 * time.Millisecond)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: before.X + dx*0.3, Y: before.Y, Z: before.Z + dz*0.3, K: 1})
	if m.pose == before {
		t.Fatal("a state echoing the correction should be accepted")
	}
}

func TestTeleportsAreCorrected(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	start := m.pose
	h.tick(2 * time.Second)
	h.drain(p)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: start.X + 40, Y: start.Y, Z: start.Z, K: m.gait.k})
	if fs, _ := h.drain(p); count(fs, protocol.TypeCorrect) != 1 || m.pose.X != start.X || m.pose.Z != start.Z {
		t.Fatalf("teleport 40 m: frames %+v, pose %+v", fs, m.pose)
	}
}

// The server knows nothing of the map's shapes: where a move at a walkable speed ends (up a wall, in the air, on a
// roof) is the game's to keep right (docs/design/04-backend.md, "What the server enforces").
func TestTheGameDecidesWhereItsPlayerStands(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	start := m.pose
	for i, to := range []protocol.Pose{
		{X: start.X + 1, Y: start.Y + 4, Z: start.Z},      // up on a ledge, or in a jump
		{X: start.X + 2, Y: start.Y - 2, Z: start.Z},      // down a step
		{X: start.X + 2, Y: start.Y + 12, Z: start.Z + 1}, // on the hill
	} {
		h.tick(time.Second)
		h.drain(p)
		h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: to.X, Y: to.Y, Z: to.Z, K: m.gait.k})
		if fs, _ := h.drain(p); count(fs, protocol.TypeCorrect) != 0 || m.pose.Y != to.Y {
			t.Errorf("move %d to %+v: frames %+v, pose %+v", i, to, fs, m.pose)
		}
	}
}

func TestRespawnGoesHomeOnly(t *testing.T) {
	h := newHarness(t)
	home := protocol.Pose{X: 8, Z: 19, H: 1}
	p, m := h.join(Arrival{Name: "Lan", Home: &home})
	far, fz := home.X+30, home.Z
	h.at(p, m, far, fz)
	h.drain(p)
	// the game found the floor there: its height is taken as given
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeRespawn, X: m.home.X + 1, Y: 0.18, Z: m.home.Z, H: 2, K: m.gait.k})
	if fs, _ := h.drain(p); count(fs, protocol.TypeCorrect) != 0 || math.Abs(m.pose.X-(m.home.X+1)) > 1e-9 || m.pose.Y != 0.18 {
		t.Fatalf("respawn home: frames %+v, pose %+v", fs, m.pose)
	}
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeRespawn, X: far, Z: fz, K: m.gait.k})
	if fs, _ := h.drain(p); count(fs, protocol.TypeCorrect) != 1 {
		t.Fatalf("respawn somewhere else: frames %+v", fs)
	}
}

func TestTheLastPoseIsWhereAPlayerStarts(t *testing.T) {
	h := newHarness(t)
	// on the hill, 12 m up: the height comes back as saved (the game moves the body to where it fits, if need be)
	_, m := h.join(Arrival{Name: "Lan", Start: &protocol.Pose{X: 20, Y: 12, Z: 30, H: 1}})
	if m.pose.X != 20 || m.pose.Z != 30 || m.pose.Y != 12 {
		t.Fatalf("start = %+v", m.pose)
	}
	// a saved spot off the map: home instead
	_, m2 := h.join(Arrival{Name: "Minh", Start: &protocol.Pose{X: 1000, Z: 0}})
	sx, sz := h.island.ThreeSpawn()
	if m2.pose.X == 1000 || math.Hypot(m2.pose.X-sx, m2.pose.Z-sz) > 2*arrivalSpread {
		t.Fatalf("start = %+v, want near the spawn (%v, %v)", m2.pose, sx, sz)
	}
}

func TestArrivingAgainReplacesTheEarlierConnection(t *testing.T) {
	h := newHarness(t)
	old, m := h.join(Arrival{Character: "01ABC", Name: "Lan"})
	other, _ := h.join(Arrival{Name: "Minh"})
	x, z := m.pose.X+5, m.pose.Z
	h.at(old, m, x, z)
	h.drain(other)
	p, again := h.join(Arrival{Character: "01ABC", Name: "Lan", Start: &protocol.Pose{X: 0, Z: 0}})
	if fs, _ := h.drain(old); len(fs) == 0 || fs[len(fs)-1].T != protocol.TypeError || fs[len(fs)-1].Code != protocol.ErrReplaced {
		t.Fatalf("the earlier connection got %+v", fs)
	}
	if !old.closed || len(h.w.members) != 2 || h.w.members[p] != again {
		t.Fatal("the earlier connection should be gone")
	}
	if again.pose.X != x || again.pose.Z != z {
		t.Fatalf("starts at %+v, want where the earlier one stood (%v, %v)", again.pose, x, z)
	}
	if fs, _ := h.drain(other); count(fs, protocol.TypeLeave) != 1 || count(fs, protocol.TypeJoin) != 1 {
		t.Fatalf("others see %+v", fs)
	}
}

// A respawn always gets the player off the bike, even one the server drops (sent before a correction arrived):
// the game has already got off, and the two must not disagree about who is riding.
func TestARespawnGetsOffTheBikeEvenWhenDropped(t *testing.T) {
	h := newHarness(t)
	home := protocol.Pose{X: 36, Z: 37}
	p, m := h.join(Arrival{Name: "Tài", Home: &home, Vehicle: &VehicleSpec{Kind: "xe_so_110", Speed: 11}})
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: m.owns.id})
	if m.riding == nil {
		t.Fatal("not riding")
	}
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeRespawn, X: m.home.X, Z: m.home.Z, K: m.gait.k + 1})
	if m.riding != nil || m.owns.rider != nil || m.flags&protocol.FlagRide != 0 {
		t.Fatal("still riding after a dropped respawn")
	}
}

func TestTheDriverRidesAtBikeSpeed(t *testing.T) {
	h := newHarness(t)
	home := protocol.Pose{X: 36, Z: 37}
	bike := &VehicleSpec{Kind: "xe_so_110", Speed: 11}
	p, m := h.join(Arrival{Name: "Tài", Home: &home, Vehicle: bike})
	fs, _ := h.drain(p)
	var vehicles []protocol.Vehicle
	if err := json.Unmarshal(fs[0].V, &vehicles); err != nil || len(vehicles) != 1 || vehicles[0].Owner != m.id || vehicles[0].Rider != 0 {
		t.Fatalf("welcome vehicles %s: %v", fs[0].V, err)
	}
	v := m.owns
	if d := math.Hypot(v.pose.X-m.pose.X, v.pose.Z-m.pose.Z); d > DefaultMovement().MountReach {
		t.Fatalf("parked %v m away", d)
	}

	// someone else may not take it
	op, _ := h.join(Arrival{Name: "Lan", Home: &home})
	h.drain(op)
	h.w.onMsg(op, protocol.ClientMsg{T: protocol.TypeMount, V: v.id})
	if fs, _ := h.drain(op); count(fs, protocol.TypeError) != 1 || v.rider != nil {
		t.Fatalf("someone else mounting: %+v", fs)
	}

	h.drain(p)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: v.id})
	if fs, _ := h.drain(p); count(fs, protocol.TypeVehicle) != 1 || v.rider != m || m.flags != protocol.FlagRide {
		t.Fatalf("mounting: %+v", fs)
	}
	x0, z0, dx, dz := m.pose.X, m.pose.Z, 1.0, 0.0
	h.at(p, m, x0, z0)
	h.tick(time.Second)
	if n := h.walk(p, m, x0, z0, dx, dz, 30, 11, 6); n != 0 {
		t.Fatalf("riding at 11 m/s: %d corrections", n)
	}
	if v.pose != m.pose {
		t.Fatal("the bike moves with its rider")
	}

	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeDismount})
	if v.rider != nil || m.riding != nil || m.flags&protocol.FlagRide != 0 {
		t.Fatal("still riding after dismount")
	}
	h.at(p, m, x0, z0)
	h.tick(time.Second)
	if n := h.walk(p, m, x0, z0, dx, dz, 30, 11, 6); n == 0 {
		t.Fatal("11 m/s on foot must be corrected")
	}

	h.w.remove(p)
	if len(h.w.vehicles) != 0 {
		t.Fatal("the bike leaves with its owner")
	}
	if fs, _ := h.drain(op); count(fs, protocol.TypeVehicleGone) != 1 {
		t.Fatalf("others hear the bike go: %+v", fs)
	}
}

func TestPositionsOnlyGoToThoseNearby(t *testing.T) {
	h := newHarness(t)
	a, ma := h.join(Arrival{Name: "A"})
	b, mb := h.join(Arrival{Name: "B"})
	c, mc := h.join(Arrival{Name: "C"})
	ax, az := ma.pose.X, ma.pose.Z
	fx, fz := ax+90, az
	h.at(c, mc, fx, fz)
	h.w.sendSnapshots()
	rows := func(p *Peer) map[uint32]bool {
		got := map[uint32]bool{}
		fs, _ := h.drain(p)
		for _, f := range fs {
			if f.T != protocol.TypeSnap {
				continue
			}
			var rs [][]float64
			if err := json.Unmarshal(f.P, &rs); err != nil {
				t.Fatal(err)
			}
			for _, r := range rs {
				if len(r) != 7 {
					t.Fatalf("row %v: want [id,x,y,z,h,s,age]", r)
				}
				got[uint32(r[0])] = true
			}
			for _, id := range f.Out {
				got[id] = false
			}
		}
		return got
	}
	if got := rows(a); !got[mb.id] || got[mc.id] || got[ma.id] {
		t.Fatalf("A sees %v: want B only (not itself, not C 90 m away)", got)
	}
	// nobody moved: nothing to send
	h.drain(b)
	h.drain(c)
	h.w.sendSnapshots()
	if fs, _ := h.drain(b); len(fs) != 0 {
		t.Fatalf("an idle snapshot sent %+v", fs)
	}
	// C walks over: A sees it come into view; B goes far away: A sees it leave
	h.at(c, mc, ax+1, az)
	h.at(b, mb, fx, fz)
	h.w.sendSnapshots()
	if got := rows(a); !got[mc.id] || got[mb.id] {
		t.Fatalf("A sees %v: want C in, B out", got)
	}
}

// TestSnapshotBandwidth is the roadmap's phase 2 budget: 150 players walking about at 12 snapshots a second
// cost each of them under ~25 KB/s, spread over the island or all crowded into one square (where the view's
// cap on how many others a player sees is what holds it).
func TestSnapshotBandwidth(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	t.Run("spread out", func(t *testing.T) { bandwidth(t, 0) })
	t.Run("crowded", func(t *testing.T) { bandwidth(t, 25) })
}

// bandwidth runs 150 walking players for 10 s; within > 0 starts them all within that many metres of the spawn.
func bandwidth(t *testing.T, within float64) {
	h := newHarness(t)
	rng := rand.New(rand.NewPCG(1, 2))
	type bot struct {
		p       *Peer
		m       *member
		heading float64
	}
	bots := make([]*bot, 150)
	b := h.island.Bounds
	for i := range bots {
		x := b[0] + rng.Float64()*(b[2]-b[0])
		z := -(b[1] + rng.Float64()*(b[3]-b[1]))
		if within > 0 {
			sx, sz := h.island.ThreeSpawn()
			a, r := rng.Float64()*2*math.Pi, rng.Float64()*within
			x, z = sx+math.Cos(a)*r, sz+math.Sin(a)*r
		}
		b := &bot{heading: rng.Float64() * 2 * math.Pi}
		b.p, b.m = h.join(Arrival{Name: "bot", Start: &protocol.Pose{X: x, Z: z}})
		bots[i] = b
	}
	for _, b := range bots {
		h.drain(b.p)
	}
	const seconds = 10
	bytes := make([]int, len(bots))
	refused := 0
	// 120 ticks of 1/120 s: states at 10 Hz (each bot on its own phase), snapshots at 12 Hz
	for tick := 0; tick < seconds*120; tick++ {
		h.tick(time.Second / 120)
		for i, b := range bots {
			if (tick+i)%12 != 0 {
				continue
			}
			for try := 0; try < 8; try++ {
				x, z := b.m.pose.X+math.Sin(b.heading)*0.31, b.m.pose.Z+math.Cos(b.heading)*0.31
				if h.island.Inside(x, -z) {
					h.state(b.p, b.m, x, z, 0)
					break
				}
				b.heading = rng.Float64() * 2 * math.Pi
			}
		}
		if tick%10 == 0 {
			h.w.sendSnapshots()
		}
		for i, b := range bots {
			fs, n := h.drain(b.p)
			bytes[i] += n
			refused += count(fs, protocol.TypeCorrect)
		}
	}
	worst, total := 0, 0
	for _, n := range bytes {
		worst, total = max(worst, n), total+n
	}
	t.Logf("150 players: %.1f KB/s per player on average, %.1f KB/s at most; %d moves refused",
		float64(total)/float64(len(bots))/seconds/1000, float64(worst)/seconds/1000, refused)
	if worst/seconds > 25_000 {
		t.Fatalf("the busiest player gets %d B/s, over the 25 KB/s budget", worst/seconds)
	}
	if refused > 0 {
		t.Fatalf("%d honest moves were refused", refused)
	}
}
