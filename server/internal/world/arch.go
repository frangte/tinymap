package world

import (
	"time"

	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

// Changing archetype in game (docs/design/README.md, D1). The connection checks the archetype id and works out
// what the new one gives (an ArchChange); the world refuses the change while the player is busy or changed a
// moment ago, and otherwise swaps everything in one step of the loop, so no move, mount or trip slips in between:
// off the bike, the old archetype's vehicle gone, the new one's parked beside them, the new clothes, home and
// capabilities, and everyone told.

// DefaultArchGap is how long after one archetype change the next one waits (protocol.ErrArchWait).
const DefaultArchGap = time.Minute

// ArchChange is what a player gets with another archetype.
type ArchChange struct {
	Arch string
	// Apps are the phone apps the new capabilities give, in phone order
	Apps []string
	// Caps are what the player may do now: the new archetype's grants and the ones earned before
	Caps capability.Set
	// Look is what they wear now: the new archetype's starter clothes on their body
	Look protocol.Look
	// Home is the new archetype's start (x, z and heading), nil for the nav grid's spawn
	Home *protocol.Pose
	// Vehicle is the new archetype's starter vehicle, parked beside the player; nil for none
	Vehicle *VehicleSpec
}

// ArchHook is a module with a say in archetype changes; modules without one do not mind them.
type ArchHook interface {
	// ArchBusy: the module has the player in something a change would break (a trip, an open ask), so the
	// change is refused
	ArchBusy(m Member) bool
	// ArchChanged: the player has another archetype and capabilities now; the module drops what they no longer
	// may do (a driver's duty, the calls to answer an ask)
	ArchChanged(m Member)
}

// ChangeArch changes a joined peer's archetype and waits for the world's answer: code is "" when the change was
// made, else the error the player was sent (protocol.ErrArchBusy, protocol.ErrArchWait). ok is false when the
// world has stopped or the peer is no longer in it; nothing changed then.
func (w *World) ChangeArch(p *Peer, c ArchChange) (code string, ok bool) {
	reply := make(chan string, 1)
	if !w.submit(event{kind: evArch, p: p, change: &c, reply: reply}) {
		return "", false
	}
	select {
	case code, ok = <-reply:
		return code, ok
	case <-w.done:
		return "", false
	}
}

func (w *World) onArch(p *Peer, c ArchChange, reply chan<- string) {
	m, ok := w.members[p]
	if !ok {
		close(reply)
		return
	}
	if code := w.archRefused(m); code != "" {
		w.sendTo(p, protocol.Encode(protocol.Error{T: protocol.TypeError, Code: code}))
		reply <- code
		return
	}
	w.changeArch(p, m, c)
	reply <- ""
}

// archRefused is why the member may not change archetype now, "" when it may.
func (w *World) archRefused(m *member) string {
	if gap := w.cfg.ArchGap; gap > 0 && w.now().Sub(w.lastArch(m)) < gap {
		return protocol.ErrArchWait
	}
	// sitting behind someone, carrying someone, or in a conversation whose lines follow the archetype
	if m.seated != nil || (m.riding != nil && m.riding.passenger != nil) || m.talk != nil {
		return protocol.ErrArchBusy
	}
	for _, mod := range w.modules {
		if h, ok := mod.(ArchHook); ok && h.ArchBusy(m) {
			return protocol.ErrArchBusy
		}
	}
	return ""
}

// lastArch is when the member's character last changed archetype (zero for never). A character's is kept by the
// world, so coming back on another connection does not skip the wait; a guest's goes with its connection.
func (w *World) lastArch(m *member) time.Time {
	if m.character == "" {
		return m.archAt
	}
	return w.archAt[m.character]
}

func (w *World) stampArch(m *member, now time.Time) {
	if m.character == "" {
		m.archAt = now
		return
	}
	// the stamps only matter for a gap: older ones go, so the map stays as small as the last minute's changes
	for c, at := range w.archAt {
		if now.Sub(at) >= w.cfg.ArchGap {
			delete(w.archAt, c)
		}
	}
	w.archAt[m.character] = now
}

func (w *World) changeArch(p *Peer, m *member, c ArchChange) {
	w.stampArch(m, w.now())
	w.dismount(m)
	if v := m.owns; v != nil {
		w.removeVehicle(v)
	}
	m.arch, m.caps, m.look = c.Arch, c.Caps, c.Look
	m.home = w.settle(w.homeAt(c.Home), bodyRadius)
	apps := c.Apps
	if apps == nil {
		apps = []string{}
	}
	w.sendTo(p, protocol.Encode(protocol.Arch{T: protocol.TypeArch, Arch: m.arch, Apps: apps, Look: m.look}))
	w.broadcast(m.profile(), p)
	if c.Vehicle != nil {
		w.broadcast(w.addVehicle(m, *c.Vehicle).encode(), nil)
	}
	for _, mod := range w.modules {
		if h, ok := mod.(ArchHook); ok {
			h.ArchChanged(m)
		}
	}
	w.log.Info("arch", "id", m.id, "arch", m.arch)
}
