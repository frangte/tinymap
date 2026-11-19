package world

import (
	"math"
	"slices"

	"saigon-noir/server/internal/protocol"
)

// Guests are NPCs a module brings into the world and takes out again, such as the NPC passenger of a Crab Bike
// trip (docs/design/08-ride-hailing.md, "NPC passengers"). A guest is a figure like any NPC (it goes into the
// snapshots, and a welcome lists it), but it has no conversation and does nothing on its own; the module says
// where it stands, when it takes seat 1 behind a rider, and when it leaves. Seat 1 holds one passenger, a player
// or a guest.

// GuestHost is the part of the host that handles guests. The world's host has it; a module asks for it with a type
// assertion, so a host without guests (a fake in a test, say) still works as a Host.
type GuestHost interface {
	// AddGuest brings an NPC of the cast (key, its id in the game's copy of the cast) in at (x, z) facing h, and
	// tells everyone; it returns the NPC's id (the id a vehicle's passenger names)
	AddGuest(key, name string, x, z, h float64) uint32
	// RemoveGuest takes it out again; one still seated is put down first
	RemoveGuest(id uint32)
	// SeatGuest puts it in seat 1 of the vehicle the rider is on; false when that is not possible
	SeatGuest(guest, rider uint32) bool
	// UnseatGuest puts it down beside the vehicle, where it stands from then on
	UnseatGuest(guest uint32)
	// GuestSeated says whether it is still in a seat
	GuestSeated(guest uint32) bool
	// GuestSays makes it say a line out loud: nearby chat from it, and a bubble over it
	GuestSays(guest uint32, line protocol.Text)
}

var _ GuestHost = host{}

func (w *World) guest(id uint32) *npc {
	i := slices.IndexFunc(w.npcs, func(n *npc) bool { return n.guest && n.id == id })
	if i < 0 {
		return nil
	}
	return w.npcs[i]
}

func (w *World) addGuest(key, name string, x, z, h float64) uint32 {
	w.nextID++
	n := &npc{entity: entity{id: w.nextID, pose: protocol.Pose{X: x, Z: z, H: h}, poseAt: w.now()}, face: h, guest: true,
		spec: NPCSpec{Key: key, Name: name, X: x, Z: z, H: h, Mode: protocol.ModePrivate}}
	n.view.moved = true
	w.npcs = append(w.npcs, n)
	w.broadcast(protocol.Encode(protocol.NPCJoin{T: protocol.TypeNPCJoin, NPC: protocol.NPC{ID: n.id, Key: key, Pose: protocol.PoseArray(n.pose)}}), nil)
	return n.id
}

func (w *World) removeGuest(n *npc) {
	w.putDownGuest(n)
	w.npcs = slices.DeleteFunc(w.npcs, func(o *npc) bool { return o == n })
	w.broadcast(protocol.Encode(protocol.NPCLeave{T: protocol.TypeNPCLeave, ID: n.id}), nil)
}

// seatGuest puts a guest in seat 1 of a ridden vehicle: from then on it follows the vehicle.
func (w *World) seatGuest(n *npc, v *vehicle) bool {
	if v.rider == nil || v.passenger != nil || v.guest != nil || n.seat != nil {
		return false
	}
	v.guest, n.seat = n, v
	w.carry(v)
	w.broadcast(v.encode(), nil)
	return true
}

// putDownGuest puts a seated guest down beside the vehicle, on its left like a player, and leaves it standing
// there. A guest not seated is left alone.
func (w *World) putDownGuest(n *npc) {
	v := n.seat
	if v == nil {
		return
	}
	v.guest, n.seat = nil, nil
	at := v.pose
	if v.rider != nil {
		at = v.rider.pose
	}
	n.pose = protocol.Pose{X: at.X + math.Cos(at.H)*parkOffset, Y: at.Y, Z: at.Z - math.Sin(at.H)*parkOffset, H: at.H}
	n.flags, n.poseAt = 0, w.now()
	n.view.moved = true
	w.broadcast(v.encode(), nil)
}

func (h host) AddGuest(key, name string, x, z, hd float64) uint32 {
	return h.w.addGuest(key, name, x, z, hd)
}

func (h host) RemoveGuest(id uint32) {
	if n := h.w.guest(id); n != nil {
		h.w.removeGuest(n)
	}
}

func (h host) SeatGuest(guest, rider uint32) bool {
	n, r := h.w.guest(guest), h.w.byID(rider)
	if n == nil || r == nil || r.riding == nil {
		return false
	}
	return h.w.seatGuest(n, r.riding)
}

func (h host) UnseatGuest(guest uint32) {
	if n := h.w.guest(guest); n != nil {
		h.w.putDownGuest(n)
	}
}

func (h host) GuestSeated(guest uint32) bool {
	n := h.w.guest(guest)
	return n != nil && n.seat != nil
}

func (h host) GuestSays(guest uint32, line protocol.Text) {
	if n := h.w.guest(guest); n != nil {
		h.w.npcSays(n, line)
	}
}
