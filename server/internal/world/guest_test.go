package world

import (
	"encoding/json"
	"math"
	"testing"

	"saigon-noir/server/internal/protocol"
)

// rawFrames is every frame queued for p, undecoded.
func rawFrames(p *Peer) [][]byte {
	var out [][]byte
	for {
		select {
		case b, ok := <-p.send:
			if !ok {
				return out
			}
			out = append(out, b)
		default:
			return out
		}
	}
}

func guestHost(t *testing.T, rec *recorder) GuestHost {
	t.Helper()
	g, ok := rec.h.(GuestHost)
	if !ok {
		t.Fatal("the world's host handles guests")
	}
	return g
}

// rowsOf are the snapshot rows (id, flags) in frames.
func rowsOf(t *testing.T, frames [][]byte) map[uint32]int {
	t.Helper()
	got := map[uint32]int{}
	for _, b := range frames {
		var f struct {
			T string      `json:"t"`
			P [][]float64 `json:"p"`
		}
		if err := json.Unmarshal(b, &f); err != nil {
			t.Fatal(err)
		}
		if f.T != protocol.TypeSnap {
			continue
		}
		for _, r := range f.P {
			got[uint32(r[0])] = int(r[5])
		}
	}
	return got
}

func TestAGuestComesSitsBehindTheRiderAndLeaves(t *testing.T) {
	h, rec := moduleHarness(t)
	g := guestHost(t, rec)
	pd, d, pp, p := h.driverAndPassenger()
	nearby := d.pose

	id := g.AddGuest("chi_thao", "Chị Thảo", nearby.X+2, nearby.Z, 0)
	var join struct {
		T   string `json:"t"`
		ID  uint32 `json:"id"`
		NPC string `json:"npc"`
	}
	var found bool
	for _, b := range rawFrames(pd) {
		if json.Unmarshal(b, &join) == nil && join.T == protocol.TypeNPCJoin {
			found = join.ID == id && join.NPC == "chi_thao"
		}
	}
	if !found {
		t.Fatal("everyone hears of the new NPC")
	}
	// it is in the snapshots like any NPC, and a welcome lists it
	h.w.sendSnapshots()
	if _, ok := rowsOf(t, rawFrames(pd))[id]; !ok {
		t.Fatal("the driver sees the guest")
	}
	if len(h.w.npcList()) != len(h.w.npcs) || h.w.byKey["chi_thao"] != nil || h.w.npcByID[id] != nil {
		t.Fatal("a guest is listed in a welcome, but is nobody to talk to")
	}

	// it sits in seat 1 and follows the bike; the vehicle names it as its passenger
	if !g.SeatGuest(id, d.id) || !g.GuestSeated(id) {
		t.Fatal("seating a guest behind a ridden bike failed")
	}
	if g.SeatGuest(id, d.id) {
		t.Fatal("seated twice")
	}
	if rec.h.Seat(p.id, d.id) {
		t.Fatal("a player cannot take seat 1 while a guest has it")
	}
	var named bool
	for _, b := range rawFrames(pd) {
		var veh protocol.Vehicle
		if json.Unmarshal(b, &veh) == nil && veh.T == protocol.TypeVehicle && veh.Passenger == id {
			named = true
		}
	}
	if !named {
		t.Fatal("the vehicle names the guest as its passenger")
	}
	x0, z0, dx, dz := h.lane(d.pose.X, d.pose.Z, 20)
	h.at(pd, d, x0, z0)
	h.tick(1e9)
	h.walk(pd, d, x0, z0, dx, dz, 20, 8, 2)
	n := h.w.guest(id)
	if n.pose != d.pose || n.flags != protocol.FlagSeat1 {
		t.Fatalf("the guest at %+v flags %d, the driver at %+v", n.pose, n.flags, d.pose)
	}
	h.w.sendSnapshots()
	if flags, ok := rowsOf(t, rawFrames(pp))[id]; ok && flags&protocol.FlagSeat1 == 0 {
		t.Fatalf("others see it seated: flags %d", flags)
	}
	// getting off: it stands beside the bike with no flags
	g.UnseatGuest(id)
	if g.GuestSeated(id) || n.flags != 0 || math.Hypot(n.pose.X-d.pose.X, n.pose.Z-d.pose.Z) > 3 {
		t.Fatalf("after getting off: %+v (driver %+v)", n.pose, d.pose)
	}
	if !rec.h.Seat(p.id, d.id) {
		t.Fatal("the seat is free again")
	}
	rec.h.Unseat(p.id)

	// and gone: everyone hears it
	g.RemoveGuest(id)
	var left bool
	for _, b := range rawFrames(pd) {
		var f struct {
			T  string `json:"t"`
			ID uint32 `json:"id"`
		}
		if json.Unmarshal(b, &f) == nil && f.T == protocol.TypeNPCLeave && f.ID == id {
			left = true
		}
	}
	if !left || h.w.guest(id) != nil {
		t.Fatal("the guest left the world")
	}
}

func TestADismountOrALeavingRiderPutsAGuestDown(t *testing.T) {
	h, rec := moduleHarness(t)
	g := guestHost(t, rec)
	pd, d, _, _ := h.driverAndPassenger()
	id := g.AddGuest("chi_thao", "Chị Thảo", d.pose.X+2, d.pose.Z, 0)
	g.SeatGuest(id, d.id)
	h.w.onMsg(pd, protocol.ClientMsg{T: protocol.TypeDismount})
	if g.GuestSeated(id) || d.owns.guest != nil {
		t.Fatal("a guest stayed on a parked bike")
	}
	if g.SeatGuest(id, d.id) {
		t.Fatal("seated on a parked bike")
	}
	// the rider leaves the world with a guest on the bike: the guest is put down, and stays
	h.w.onMsg(pd, protocol.ClientMsg{T: protocol.TypeMount, V: d.owns.id})
	g.SeatGuest(id, d.id)
	h.w.remove(pd)
	if g.GuestSeated(id) || h.w.guest(id) == nil {
		t.Fatal("the guest was put down and left standing")
	}
}
