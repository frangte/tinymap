package world

import (
	"cmp"
	"math"
	"slices"

	"saigon-noir/server/internal/protocol"
)

// vehicle is a vehicle in the world (docs/design/02-architecture.md, vehicle and seat entities). For now each
// one belongs to the player it arrived with and leaves with them, and only its owner rides it, in seat 0.
// Seat 1, a passenger, comes with ride-hailing.
type vehicle struct {
	id    uint32
	kind  string
	speed float64
	owner *member
	// rider sits in seat 0, nil while parked; pose follows the rider and is where the vehicle stays once they
	// get off
	rider *member
	pose  protocol.Pose
}

// parkOffset is how far to the side of its player a vehicle is parked (m).
const parkOffset = 1.4

func (v *vehicle) wire() protocol.Vehicle {
	out := protocol.Vehicle{ID: v.id, Kind: v.kind, Owner: v.owner.id, Pose: protocol.PoseArray(v.pose)}
	if v.rider != nil {
		out.Rider = v.rider.id
	}
	return out
}

func (v *vehicle) encode() []byte {
	out := v.wire()
	out.T = protocol.TypeVehicle
	return protocol.Encode(out)
}

// addVehicle parks a new vehicle beside its owner.
func (w *World) addVehicle(owner *member, spec VehicleSpec) *vehicle {
	w.nextID++
	v := &vehicle{id: w.nextID, kind: spec.Kind, speed: spec.Speed, owner: owner}
	w.parkBeside(v, owner.pose)
	w.vehicles[v.id] = v
	owner.owns = v
	return v
}

// parkBeside puts the vehicle at the right hand of someone standing at p, facing the same way.
func (w *World) parkBeside(v *vehicle, p protocol.Pose) {
	// three.js heading h faces (sin h, cos h) on (x, z); the right hand is (-cos h, sin h)
	spot := protocol.Pose{X: p.X - math.Cos(p.H)*parkOffset, Z: p.Z + math.Sin(p.H)*parkOffset, H: p.H}
	v.pose = w.settle(spot, vehicleRadius)
}

func (w *World) removeVehicle(v *vehicle) {
	if v.rider != nil {
		v.rider.riding = nil
		v.rider.flags &^= protocol.FlagRide
	}
	if v.owner.owns == v {
		v.owner.owns = nil
	}
	delete(w.vehicles, v.id)
	w.broadcast(protocol.Encode(protocol.VehicleGone{T: protocol.TypeVehicleGone, ID: v.id}), nil)
}

// vehicleList is every vehicle, by id, for a welcome.
func (w *World) vehicleList() []protocol.Vehicle {
	out := make([]protocol.Vehicle, 0, len(w.vehicles))
	for _, v := range w.vehicles {
		out = append(out, v.wire())
	}
	slices.SortFunc(out, func(a, b protocol.Vehicle) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// onMount seats the member on its own vehicle, if it stands next to it. From then on its moves are checked
// against the vehicle's speed. Everyone hears of it; a refusal only goes to the member.
func (w *World) onMount(p *Peer, m *member, id uint32) {
	v := w.vehicles[id]
	if v == nil || v.owner != m || v.rider != nil || m.riding != nil ||
		math.Hypot(v.pose.X-m.pose.X, v.pose.Z-m.pose.Z) > w.cfg.Move.MountReach {
		w.sendTo(p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrMount}))
		return
	}
	v.rider, m.riding = m, v
	m.flags = protocol.FlagRide
	m.view.moved = true
	w.broadcast(v.encode(), nil)
}

// dismount leaves the member's vehicle parked where it is.
func (w *World) dismount(m *member) {
	v := m.riding
	if v == nil {
		return
	}
	v.pose, v.rider, m.riding = m.pose, nil, nil
	m.flags &^= protocol.FlagRide
	m.view.moved = true
	w.broadcast(v.encode(), nil)
}
