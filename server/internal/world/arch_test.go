package world

import (
	"slices"
	"testing"
	"time"

	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

// changeArch hands the world an archetype change the way ChangeArch does, and returns its answer.
func (h *harness) changeArch(p *Peer, c ArchChange) (code string, ok bool) {
	h.t.Helper()
	reply := make(chan string, 1)
	h.w.onArch(p, c, reply)
	code, ok = <-reply
	return code, ok
}

var (
	toCitizen = ArchChange{Arch: "citizen", Apps: []string{"profile", "community"}, Caps: capability.Of("citizen.community_app"),
		Look: protocol.Look{Body: "man", Outfit: "baba", Hat: "nonla"}, Home: &protocol.Pose{X: -30, Z: 19}}
	toDriver = ArchChange{Arch: "driver", Apps: []string{"profile", "crab_driver"}, Caps: capability.Of("driver.app", "uniform.crab"),
		Look: protocol.Look{Body: "man", Outfit: "crab", Hat: "helmet"}, Home: &protocol.Pose{X: 36, Z: 37},
		Vehicle: &VehicleSpec{Kind: "xe_so_110", Speed: 11}}
)

// archHook is a module that may hold a player and hears who changed.
type archHook struct {
	recorder
	busy    map[uint32]bool
	changed []string
}

func (a *archHook) ArchBusy(m Member) bool { return a.busy[m.ID()] }
func (a *archHook) ArchChanged(m Member)   { a.changed = append(a.changed, m.Arch()) }

func frameOf(fs []frame, t string) (frame, bool) {
	i := slices.IndexFunc(fs, func(f frame) bool { return f.T == t })
	if i < 0 {
		return frame{}, false
	}
	return fs[i], true
}

func TestChangingArchetypeSwapsTheBikeAndTellsEveryone(t *testing.T) {
	h := newHarness(t)
	home := protocol.Pose{X: 36, Z: 37}
	p, m := h.join(Arrival{Name: "Tài", Character: "char-tai", Arch: "driver", Caps: toDriver.Caps, Look: toDriver.Look, Home: &home,
		Vehicle: toDriver.Vehicle})
	near := m.pose
	near.X += 2
	po, _ := h.join(Arrival{Name: "Lan", Start: &near})
	bike := m.owns.id
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: bike})
	h.drain(p)
	h.drain(po)

	if code, ok := h.changeArch(p, toCitizen); !ok || code != "" {
		t.Fatalf("change = %q, %v", code, ok)
	}
	if m.arch != "citizen" || m.look != toCitizen.Look || !m.caps.Has("citizen.community_app") || m.caps.Has("driver.app") {
		t.Fatalf("member after the change: arch %q look %+v caps %v", m.arch, m.look, m.caps)
	}
	if m.riding != nil || m.owns != nil || h.w.vehicles[bike] != nil || m.flags&protocol.FlagRide != 0 {
		t.Fatal("the driver's bike must go with the driver's archetype")
	}
	if m.home.X > -25 || m.home.X < -35 {
		t.Fatalf("home = %+v, want the Citizen's start", m.home)
	}
	mine, _ := h.drain(p)
	a, ok := frameOf(mine, protocol.TypeArch)
	if !ok || a.Arch != "citizen" || !slices.Equal(a.Apps, toCitizen.Apps) || a.Look != toCitizen.Look {
		t.Fatalf("the player is told %+v (%v)", a, ok)
	}
	if count(mine, protocol.TypeProfile) != 0 {
		t.Fatal("the player gets an arch, not a profile of their own")
	}
	theirs, _ := h.drain(po)
	pr, ok := frameOf(theirs, protocol.TypeProfile)
	if !ok || pr.ID != m.id || pr.Arch != "citizen" || pr.Look != toCitizen.Look {
		t.Fatalf("the others see %+v (%v)", pr, ok)
	}
	if count(theirs, protocol.TypeVehicleGone) != 1 {
		t.Fatalf("the others see the bike go: %+v", theirs)
	}

	// back to driving a minute later: a new bike beside them, for everyone to see
	h.tick(DefaultArchGap)
	if code, ok := h.changeArch(p, toDriver); !ok || code != "" {
		t.Fatalf("change back = %q, %v", code, ok)
	}
	if m.owns == nil || m.owns.id == bike || m.owns.kind != "xe_so_110" {
		t.Fatalf("a driver again needs a new bike: %+v", m.owns)
	}
	theirs, _ = h.drain(po)
	if v, ok := frameOf(theirs, protocol.TypeVehicle); !ok || v.ID != m.owns.id || v.Owner != m.id {
		t.Fatalf("the others see the new bike: %+v (%v)", v, ok)
	}
}

func TestAnArchetypeChangeWaitsAMinute(t *testing.T) {
	h := newHarness(t)
	p, _ := h.join(Arrival{Name: "Tài", Character: "char-tai", Arch: "traveler"})
	if code, _ := h.changeArch(p, toCitizen); code != "" {
		t.Fatalf("first change = %q", code)
	}
	h.drain(p)
	h.tick(DefaultArchGap - time.Second)
	if code, _ := h.changeArch(p, toDriver); code != protocol.ErrArchWait {
		t.Fatalf("a change within the minute = %q, want %q", code, protocol.ErrArchWait)
	}
	if fs, _ := h.drain(p); count(fs, protocol.TypeError) != 1 || fs[0].Code != protocol.ErrArchWait {
		t.Fatalf("the player is told to wait: %+v", fs)
	}
	// coming back on another connection does not skip the wait
	p2, m2 := h.join(Arrival{Name: "Tài", Character: "char-tai", Arch: "citizen"})
	if code, _ := h.changeArch(p2, toDriver); code != protocol.ErrArchWait {
		t.Fatalf("after a reconnect = %q, want %q", code, protocol.ErrArchWait)
	}
	// a guest's wait is its own
	pg, _ := h.join(Arrival{Name: "Khách", Arch: "traveler"})
	if code, _ := h.changeArch(pg, toCitizen); code != "" {
		t.Fatalf("a guest's first change = %q", code)
	}
	h.tick(time.Second)
	if code, _ := h.changeArch(p2, toDriver); code != "" || m2.arch != "driver" {
		t.Fatalf("a minute after the change = %q, arch %q", code, m2.arch)
	}
	if len(h.w.archAt) != 1 {
		t.Fatalf("the stamps older than a minute are dropped: %v", h.w.archAt)
	}
}

func TestABusyPlayerKeepsTheirArchetype(t *testing.T) {
	h := newHarness(t)
	hook := &archHook{busy: map[uint32]bool{}}
	h.w.Use(func(Host) Module { return hook })
	pd, d, pp, p := h.driverAndPassenger()
	if !h.w.seat(p, d.riding) {
		t.Fatal("not seated")
	}
	if code, _ := h.changeArch(pp, toDriver); code != protocol.ErrArchBusy {
		t.Fatalf("a passenger's change = %q, want %q", code, protocol.ErrArchBusy)
	}
	if code, _ := h.changeArch(pd, toCitizen); code != protocol.ErrArchBusy {
		t.Fatalf("a driver carrying someone = %q, want %q", code, protocol.ErrArchBusy)
	}
	h.w.unseat(p)
	hook.busy[d.id] = true
	if code, _ := h.changeArch(pd, toCitizen); code != protocol.ErrArchBusy || d.owns == nil {
		t.Fatalf("a module holding the driver = %q, want %q and the bike kept", code, protocol.ErrArchBusy)
	}
	p.talk = &session{}
	if code, _ := h.changeArch(pp, toCitizen); code != protocol.ErrArchBusy {
		t.Fatalf("in a conversation = %q, want %q", code, protocol.ErrArchBusy)
	}
	p.talk = nil
	if len(hook.changed) != 0 {
		t.Fatalf("modules heard of changes that were refused: %v", hook.changed)
	}
	hook.busy[d.id] = false
	if code, _ := h.changeArch(pd, toCitizen); code != "" || !slices.Equal(hook.changed, []string{"citizen"}) {
		t.Fatalf("a free driver = %q; modules heard %v", code, hook.changed)
	}
	// a player who left gets no answer
	h.w.remove(pp)
	if _, ok := h.changeArch(pp, toCitizen); ok {
		t.Fatal("a peer no longer in the world must get no answer")
	}
}
