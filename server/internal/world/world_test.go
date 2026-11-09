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
	"saigon-noir/server/internal/nav"
	"saigon-noir/server/internal/protocol"
)

// The tests drive the world by hand: no Run goroutine, a clock that only moves when told to, and peers whose
// queues the test reads.

type harness struct {
	t     *testing.T
	w     *World
	grid  *nav.Grid
	clock time.Time
}

func newHarness(t *testing.T) *harness { return newHarnessWith(t, nil) }

// newHarnessWith lets a test change the configuration first (place NPCs, say).
func newHarnessWith(t *testing.T, tune func(*Config)) *harness {
	t.Helper()
	grid, err := content.Nav()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.SendBuffer = 4096
	if tune != nil {
		tune(&cfg)
	}
	h := &harness{t: t, grid: grid, clock: time.Unix(1_700_000_000, 0)}
	h.w = New(cfg, grid, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
	y, _ := h.grid.Floor(x, z)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: x, Y: y, Z: z, H: 0, S: s, K: m.gait.k})
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

// lane finds a straight run of at least `length` metres from near (x, z) that a body can walk all along.
func (h *harness) lane(x, z, length float64) (x0, z0, dx, dz float64) {
	h.t.Helper()
	for r := 0.0; r < 30; r += 0.5 {
		for a := 0.0; a < 2*math.Pi; a += math.Pi / 16 {
			sx, sz := x+math.Cos(a)*r, z+math.Sin(a)*r
			for b := 0.0; b < 2*math.Pi; b += math.Pi / 8 {
				ex, ez := sx+math.Sin(b)*length, sz+math.Cos(b)*length
				if h.fitsAlong(sx, sz, ex, ez) {
					return sx, sz, math.Sin(b), math.Cos(b)
				}
			}
		}
	}
	h.t.Fatalf("no straight %v m lane near (%v, %v)", length, x, z)
	return
}

func (h *harness) fitsAlong(ax, az, bx, bz float64) bool {
	n := int(math.Hypot(bx-ax, bz-az) / 0.1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		x, z := ax+(bx-ax)*t, az+(bz-az)*t
		if !h.grid.Fits(x, z, 0.3) {
			return false
		}
		if f, _ := h.grid.Floor(x, z); i > 0 {
			if pf, _ := h.grid.Floor(ax+(bx-ax)*(t-1/float64(n)), az+(bz-az)*(t-1/float64(n))); math.Abs(f-pf) > 0.4 {
				return false
			}
		}
	}
	return true
}

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
	y, _ := h.grid.Floor(x, z)
	h.w.place(p, m, protocol.Pose{X: x, Y: y, Z: z})
}

func TestHonestWalkingAndRunningPass(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	x0, z0, dx, dz := h.lane(m.pose.X, m.pose.Z, 12)
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
	x0, z0, dx, dz := h.lane(m.pose.X, m.pose.Z, 12)
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

func TestTeleportsWallsAndFlyingAreCorrected(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	start := m.pose
	cases := []struct {
		name string
		to   func() (x, y, z float64)
	}{
		{"teleport 40 m", func() (float64, float64, float64) {
			x, z, ok := h.grid.NearestFit(start.X+40, start.Z, 0.3, 10)
			if !ok {
				t.Fatal("nowhere to teleport to")
			}
			y, _ := h.grid.Floor(x, z)
			return x, y, z
		}},
		{"into a building", func() (float64, float64, float64) {
			x := start.X
			for h.grid.Walkable(x, start.Z) {
				x += 0.1
			}
			return x + 0.5, start.Y, start.Z
		}},
		{"flying", func() (float64, float64, float64) { return start.X, start.Y + 4, start.Z }},
		{"under the floor", func() (float64, float64, float64) { return start.X, start.Y - 2, start.Z }},
	}
	for _, c := range cases {
		h.at(p, m, start.X, start.Z)
		h.tick(2 * time.Second)
		h.drain(p)
		x, y, z := c.to()
		h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: x, Y: y, Z: z, K: m.gait.k})
		if fs, _ := h.drain(p); count(fs, protocol.TypeCorrect) != 1 || m.pose.X != start.X || m.pose.Z != start.Z {
			t.Errorf("%s: frames %+v, pose %+v", c.name, fs, m.pose)
		}
	}
}

func TestRespawnGoesHomeOnly(t *testing.T) {
	h := newHarness(t)
	home := protocol.Pose{X: 8, Z: 19, H: 1}
	p, m := h.join(Arrival{Name: "Lan", Home: &home})
	far, fz, _ := h.grid.NearestFit(home.X+30, home.Z, 0.3, 10)
	h.at(p, m, far, fz)
	h.drain(p)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeRespawn, X: m.home.X + 1, Z: m.home.Z, H: 2, K: m.gait.k})
	if fs, _ := h.drain(p); count(fs, protocol.TypeCorrect) != 0 || math.Abs(m.pose.X-(m.home.X+1)) > 1e-9 {
		t.Fatalf("respawn home: frames %+v, pose %+v", fs, m.pose)
	}
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeRespawn, X: far, Z: fz, K: m.gait.k})
	if fs, _ := h.drain(p); count(fs, protocol.TypeCorrect) != 1 {
		t.Fatalf("respawn somewhere else: frames %+v", fs)
	}
}

func TestTheLastPoseIsWhereAPlayerStarts(t *testing.T) {
	h := newHarness(t)
	x, z, _ := h.grid.NearestFit(20, 30, 0.3, 10)
	_, m := h.join(Arrival{Name: "Lan", Start: &protocol.Pose{X: x, Y: 99, Z: z, H: 1}})
	if m.pose.X != x || m.pose.Z != z || m.pose.Y == 99 {
		t.Fatalf("start = %+v", m.pose)
	}
	// a saved spot that is now inside a wall: home instead
	_, m2 := h.join(Arrival{Name: "Minh", Start: &protocol.Pose{X: 1000, Z: 0}})
	if !h.grid.Fits(m2.pose.X, m2.pose.Z, bodyRadius) || m2.pose.X == 1000 {
		t.Fatalf("start = %+v", m2.pose)
	}
}

func TestArrivingAgainReplacesTheEarlierConnection(t *testing.T) {
	h := newHarness(t)
	old, m := h.join(Arrival{Character: "01ABC", Name: "Lan"})
	other, _ := h.join(Arrival{Name: "Minh"})
	x, z, _ := h.grid.NearestFit(m.pose.X+5, m.pose.Z, 0.3, 5)
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

func TestHoveringIsCorrected(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	x0, z0, dx, dz := h.lane(m.pose.X, m.pose.Z, 12)
	h.at(p, m, x0, z0)
	h.drain(p)
	// 1.2 m up, walking along: allowed for a slow jump's length, then put back on the ground
	for i := 1; i <= 40 && m.gait.refused == 0; i++ {
		h.tick(100 * time.Millisecond)
		x, z := x0+dx*0.25*float64(i), z0+dz*0.25*float64(i)
		f, _ := h.grid.Floor(x, z)
		h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: x, Y: f + 1.2, Z: z, K: m.gait.k})
	}
	floor, _ := h.grid.Floor(m.pose.X, m.pose.Z)
	if m.gait.refused != 1 || m.pose.Y != floor {
		t.Fatalf("hovering: refused %d, pose %+v (floor %v)", m.gait.refused, m.pose, floor)
	}
	// a jump on a device at 8 fps (the game runs at 0.4 of the clock: 1.5 s up and down) is fine
	h.at(p, m, x0, z0)
	for i := 1; i <= 15; i++ {
		h.tick(100 * time.Millisecond)
		x, z := x0+dx*0.12*float64(i), z0+dz*0.12*float64(i)
		f, _ := h.grid.Floor(x, z)
		up := 0.7 * math.Sin(math.Pi*float64(i)/15)
		h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: x, Y: f + up, Z: z, K: m.gait.k})
	}
	if m.gait.refused != 1 {
		t.Fatalf("a jump was refused: %d", m.gait.refused)
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
	x0, z0, dx, dz := h.lane(m.pose.X, m.pose.Z, 30)
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
	fx, fz, ok := h.grid.NearestFit(ax+90, az, 0.3, 20)
	if !ok || math.Hypot(fx-ax, fz-az) < 75 {
		t.Fatal("no spot far enough away")
	}
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
	meta := h.grid.Meta()
	for i := range bots {
		var x, z float64
		for {
			x = meta.X0 + rng.Float64()*float64(meta.NX)*meta.Cell
			z = -(meta.Y0 + rng.Float64()*float64(meta.NY)*meta.Cell)
			if within > 0 {
				sx, sz := h.grid.Spawn()
				a, r := rng.Float64()*2*math.Pi, rng.Float64()*within
				x, z = sx+math.Cos(a)*r, sz+math.Sin(a)*r
			}
			if h.grid.Fits(x, z, 0.3) {
				break
			}
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
				if h.grid.Fits(x, z, 0.3) && h.grid.Clear(b.m.pose.X, b.m.pose.Z, x, z, 0) {
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
