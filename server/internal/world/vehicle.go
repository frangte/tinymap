package world

import (
	"cmp"
	"math"
	"slices"

	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// vehicle is a vehicle in the world (docs/design/02-architecture.md, vehicle and seat entities). Each one belongs
// to the player it arrived with and leaves with them; only its owner rides it, in seat 0. Seat 1 takes a passenger
// a module seats there (a Crab Bike trip), carried along with the vehicle.
type vehicle struct {
	id    uint32
	kind  string
	speed float64
	owner *member
	// rider sits in seat 0, nil while parked; pose follows the rider and is where the vehicle stays once they
	// get off
	rider *member
	// passenger sits in seat 1 behind the rider, nil for none
	passenger *member
	pose      protocol.Pose
}

// parkOffset is how far to the side of its player a vehicle is parked (m).
const parkOffset = 1.4

func (v *vehicle) wire() protocol.Vehicle {
	out := protocol.Vehicle{ID: v.id, Kind: v.kind, Owner: v.owner.id, Pose: protocol.PoseArray(v.pose)}
	if v.rider != nil {
		out.Rider = v.rider.id
	}
	if v.passenger != nil {
		out.Passenger = v.passenger.id
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
	if v.passenger != nil {
		w.unseat(v.passenger)
	}
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
	if v == nil || v.owner != m || v.rider != nil || m.riding != nil || m.seated != nil ||
		math.Hypot(v.pose.X-m.pose.X, v.pose.Z-m.pose.Z) > w.cfg.Move.MountReach {
		w.sendTo(p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrMount}))
		return
	}
	v.rider, m.riding = m, v
	m.flags = protocol.FlagRide
	m.view.moved = true
	w.broadcast(v.encode(), nil)
	w.publish(events.Event{Kind: events.Mounted, Character: m.character, Vehicle: v.kind})
}

// dismount leaves the member's vehicle parked where it is.
func (w *World) dismount(m *member) {
	v := m.riding
	if v == nil {
		return
	}
	// nobody stays on a parked bike: the passenger gets off first
	if v.passenger != nil {
		w.unseat(v.passenger)
	}
	v.pose, v.rider, m.riding = m.pose, nil, nil
	m.flags &^= protocol.FlagRide
	m.view.moved = true
	w.broadcast(v.encode(), nil)
}

// seat puts a passenger in seat 1 of a ridden vehicle: from now on its pose follows the vehicle and its own
// reports are ignored.
func (w *World) seat(p *member, v *vehicle) bool {
	if v.rider == nil || v.rider == p || v.passenger != nil || p.riding != nil || p.seated != nil {
		return false
	}
	v.passenger, p.seated = p, v
	w.carry(v)
	w.broadcast(v.encode(), nil)
	return true
}

// unseat puts a passenger down beside the vehicle, on its left, and sends its client there with a correction.
func (w *World) unseat(p *member) { w.putDown(p, true) }

// putDown puts a passenger down beside the vehicle; correct sends its client there (false when the caller is
// about to place it somewhere itself, as a respawn does).
func (w *World) putDown(p *member, correct bool) {
	v := p.seated
	if v == nil {
		return
	}
	v.passenger, p.seated = nil, nil
	at := v.pose
	if v.rider != nil {
		at = v.rider.pose
	}
	// three.js heading h faces (sin h, cos h); the left hand is (cos h, -sin h)
	spot := w.settle(protocol.Pose{X: at.X + math.Cos(at.H)*parkOffset, Z: at.Z - math.Sin(at.H)*parkOffset, H: at.H}, bodyRadius)
	p.flags = 0
	w.place(p.peer, p, spot)
	if correct {
		p.gait.k++
		w.sendTo(p.peer, protocol.Encode(protocol.Correct{T: protocol.TypeCorrect, K: p.gait.k, Pose: protocol.PoseArray(spot)}))
	}
	w.broadcast(v.encode(), nil)
}

// carry moves a vehicle's passenger with it (after each accepted move of the rider).
func (w *World) carry(v *vehicle) {
	p := v.passenger
	if p == nil || v.rider == nil {
		return
	}
	p.pose, p.poseAt = v.rider.pose, w.now()
	p.flags = protocol.FlagSeat1
	p.view.moved = true
	p.peer.setPose(p.pose)
}
