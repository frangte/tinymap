package world

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"saigon-noir/server/internal/protocol"
)

// recorder is a module that remembers what the world told it.
type recorder struct {
	h       Host
	handled []string
	ticks   int
	left    []uint32
}

func (r *recorder) Handle(m Member, msg protocol.ClientMsg) bool {
	if msg.T != "test" {
		return false
	}
	r.handled = append(r.handled, m.Name())
	return true
}
func (r *recorder) Tick(time.Time) { r.ticks++ }
func (r *recorder) Left(m Member)  { r.left = append(r.left, m.ID()) }

func moduleHarness(t *testing.T) (*harness, *recorder) {
	h := newHarness(t)
	rec := &recorder{}
	h.w.Use(func(host Host) Module {
		rec.h = host
		return rec
	})
	return h, rec
}

// a driver on their bike and a passenger next to it
func (h *harness) driverAndPassenger() (*Peer, *member, *Peer, *member) {
	h.t.Helper()
	home := protocol.Pose{X: 36, Z: 37}
	pd, d := h.join(Arrival{Name: "Tài", Character: "char-tai", Home: &home, Vehicle: &VehicleSpec{Kind: "xe_so_110", Speed: 11}})
	h.w.onMsg(pd, protocol.ClientMsg{T: protocol.TypeMount, V: d.owns.id})
	near := d.pose
	near.X += 1.5
	pp, p := h.join(Arrival{Name: "Lan", Character: "char-lan", Start: &near})
	h.drain(pd)
	h.drain(pp)
	return pd, d, pp, p
}

func TestModulesHearTheirMessagesTicksAndLeaves(t *testing.T) {
	h, rec := moduleHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	h.w.onMsg(p, protocol.ClientMsg{T: "test"})
	h.w.tick()
	h.w.remove(p)
	if len(rec.handled) != 1 || rec.handled[0] != "Lan" || rec.ticks != 1 || len(rec.left) != 1 || rec.left[0] != m.id {
		t.Fatalf("module heard %+v", rec)
	}
}

func TestAPassengerIsCarried(t *testing.T) {
	h, rec := moduleHarness(t)
	pd, d, pp, p := h.driverAndPassenger()
	if !rec.h.Seat(p.id, d.id) {
		t.Fatal("seating next to a ridden bike failed")
	}
	if v, seat, _ := p.Riding(); v != d.riding.id || seat != 1 || p.flags != protocol.FlagSeat1 {
		t.Fatalf("seated: vehicle %d seat %d flags %d", v, seat, p.flags)
	}
	// nobody else fits, and the passenger cannot sit twice
	if rec.h.Seat(p.id, d.id) {
		t.Fatal("seat 1 taken twice")
	}
	// the passenger's own reports are ignored: a tampered client cannot move itself
	before := p.pose
	h.tick(time.Second)
	h.w.onMsg(pp, protocol.ClientMsg{T: protocol.TypeState, X: before.X + 3, Y: before.Y, Z: before.Z, K: p.gait.k})
	if p.pose != before {
		t.Fatal("a passenger moved itself")
	}
	// the driver rides: the passenger goes along
	x0, z0, dx, dz := h.lane(d.pose.X, d.pose.Z, 20)
	h.at(pd, d, x0, z0)
	h.tick(time.Second)
	h.walk(pd, d, x0, z0, dx, dz, 20, 8, 2)
	if p.pose != d.pose {
		t.Fatalf("passenger at %+v, driver at %+v", p.pose, d.pose)
	}
	// getting off: beside the bike, with a correction to go there
	h.drain(pp)
	rec.h.Unseat(p.id)
	if p.seated != nil || d.riding.passenger != nil || math.Hypot(p.pose.X-d.pose.X, p.pose.Z-d.pose.Z) > 3 {
		t.Fatalf("after getting off: %+v (driver %+v)", p.pose, d.pose)
	}
	if fs, _ := h.drain(pp); count(fs, protocol.TypeCorrect) != 1 {
		t.Fatalf("frames %+v", fs)
	}
}

func TestADismountPutsThePassengerDown(t *testing.T) {
	h, rec := moduleHarness(t)
	pd, d, _, p := h.driverAndPassenger()
	rec.h.Seat(p.id, d.id)
	h.w.onMsg(pd, protocol.ClientMsg{T: protocol.TypeDismount})
	if p.seated != nil || d.owns.passenger != nil {
		t.Fatal("a passenger stayed on a parked bike")
	}
	// and the driver leaving the world takes the bike: its passenger is put down first
	rec.h.Seat(p.id, d.id) // not ridden any more: refused
	if p.seated != nil {
		t.Fatal("seated on a parked bike")
	}
}

func TestAPassengerRespawningLandsAtHome(t *testing.T) {
	h, rec := moduleHarness(t)
	_, d, pp, p := h.driverAndPassenger()
	rec.h.Seat(p.id, d.id)
	h.drain(pp)
	home := p.home
	h.w.onMsg(pp, protocol.ClientMsg{T: protocol.TypeRespawn, X: home.X, Z: home.Z, H: 1, K: p.gait.k})
	if p.seated != nil || d.owns.passenger != nil {
		t.Fatal("still seated after a respawn")
	}
	if math.Hypot(p.pose.X-home.X, p.pose.Z-home.Z) > 0.01 {
		t.Fatalf("the respawn did not land: at (%.1f, %.1f), home (%.1f, %.1f)", p.pose.X, p.pose.Z, home.X, home.Z)
	}
	if fs, _ := h.drain(pp); count(fs, protocol.TypeCorrect) != 0 {
		t.Fatalf("a respawn that landed was corrected: %+v", fs)
	}
}

func TestPinnedPlayersSeeEachOtherFromAnywhere(t *testing.T) {
	h, rec := moduleHarness(t)
	pa, a := h.join(Arrival{Name: "Lan"})
	// a walkable spot far past the view's reach
	var far protocol.Pose
	for _, try := range [][2]float64{{90, 0}, {-60, -40}, {40, -100}, {60, 70}} {
		if x, z, ok := h.grid.NearestFit(a.pose.X+try[0], a.pose.Z+try[1], bodyRadius, 15); ok && math.Hypot(x-a.pose.X, z-a.pose.Z) > 90 {
			far = protocol.Pose{X: x, Z: z}
			break
		}
	}
	pb, b := h.join(Arrival{Name: "Tài", Start: &far})
	if d := math.Hypot(b.pose.X-a.pose.X, b.pose.Z-a.pose.Z); d < 90 {
		t.Fatalf("the two stand only %.0f m apart", d)
	}
	h.drain(pa)
	h.drain(pb)
	rec.h.Pin(a.id, b.id, true)
	h.w.sendSnapshots()
	fs, _ := h.drain(pa)
	if !strings.Contains(string(fs[len(fs)-1].P), "["+strconv.Itoa(int(b.id))+",") {
		t.Fatalf("a pinned player 150 m away is not in view: %+v", fs)
	}
	rec.h.Pin(a.id, b.id, false)
	h.w.sendSnapshots()
	if fs, _ := h.drain(pa); len(fs) != 1 || len(fs[0].Out) != 1 {
		t.Fatalf("unpinned, it should leave the view: %+v", fs)
	}
}

func TestAJobAnswersInTheLoop(t *testing.T) {
	h, rec := moduleHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.w.Run(ctx)
	got := make(chan error, 1)
	// Do is called from the loop in real use; here the world runs, so ask it through an event
	h.w.submit(event{kind: evDone, done: func() {
		rec.h.Do("char-lan", func(context.Context) error { return errors.New("disk full") }, func(err error) { got <- err })
	}})
	select {
	case err := <-got:
		if err == nil || err.Error() != "disk full" {
			t.Fatalf("job answered %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the job never answered")
	}
}

func TestSpeedFallsToZeroWhenReportsStop(t *testing.T) {
	h := newHarness(t)
	p, m := h.join(Arrival{Name: "Lan"})
	x0, z0, dx, dz := h.lane(m.pose.X, m.pose.Z, 12)
	h.at(p, m, x0, z0)
	h.tick(time.Second)
	h.walk(p, m, x0, z0, dx, dz, 12, 3, 1)
	if s := m.Speed(); s < 2 || s > 4 {
		t.Fatalf("walking at 3 m/s reads %.1f", s)
	}
	h.tick(time.Second)
	if s := m.Speed(); s != 0 {
		t.Fatalf("standing still reads %.1f", s)
	}
}
