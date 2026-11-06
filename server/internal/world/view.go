package world

import (
	"cmp"
	"math"
	"slices"

	"saigon-noir/server/internal/protocol"
)

// View is who a player sees (docs/design/04-backend.md, the interest grid). Everyone still hears about everyone
// (join, leave, profile and vehicle messages are rare), but positions only go to the players nearby, and only
// for those that moved: what each player receives stays bounded however many are on the island.
type View struct {
	// Radius: others closer than this come into view (m, on the ground).
	Radius float64
	// Leave: others in view go out of it further than this (m), a little past Radius so that someone walking
	// along the edge does not blink in and out.
	Leave float64
	// Max is how many others a player sees at most, the nearest first: the game draws about this many avatars.
	Max int
}

func DefaultView() View { return View{Radius: 60, Leave: 68, Max: 40} }

// maxAgeMs caps the age in a snapshot row: someone who stood still for an hour is simply standing there.
const maxAgeMs = 60_000

// view is what one member's client has been shown, and how the entity itself is shown to others.
type view struct {
	// seen are the ids in view at the last snapshot; next is scratch for the one being built (nil for an NPC,
	// which sees nobody)
	seen, next map[uint32]struct{}
	// moved: the entity moved (or changed flags) since the last snapshot, so viewers need a new row
	moved bool
	// row is the member's snapshot row for snapshot number rowOf, built once and shared by every viewer
	row   []byte
	rowOf uint64
}

func newView() view {
	return view{seen: map[uint32]struct{}{}, next: map[uint32]struct{}{}}
}

type cellKey struct{ i, j int32 }

type candidate struct {
	e  *entity
	d2 float64
}

// snapScratch is reused from one snapshot to the next, so a snapshot allocates only what it sends.
type snapScratch struct {
	n     uint64
	cells map[cellKey][]*entity
	cands []candidate
	rows  [][]byte
	out   []uint32
}

// cellOf buckets a pose into cells of the Leave distance: a 3×3 block of them holds everyone within Leave.
func (w *World) cellOf(p protocol.Pose) cellKey {
	c := w.cfg.View.Leave
	return cellKey{int32(math.Floor(p.X / c)), int32(math.Floor(p.Z / c))}
}

// sendSnapshots gives every member the rows of the others in its view (players and NPCs) that moved or just
// came into view, and the ids of those that left it. A member with nothing new gets nothing.
func (w *World) sendSnapshots() {
	s := &w.snaps
	s.n++
	if s.cells == nil {
		s.cells = map[cellKey][]*entity{}
	}
	for k, es := range s.cells {
		s.cells[k] = es[:0]
	}
	for _, m := range w.members {
		k := w.cellOf(m.pose)
		s.cells[k] = append(s.cells[k], &m.entity)
	}
	for _, n := range w.npcs {
		k := w.cellOf(n.pose)
		s.cells[k] = append(s.cells[k], &n.entity)
	}
	now := w.now()
	at := now.UnixMilli()
	// a peer too slow to take its snapshot is removed after the loop: removing it now would leave it in the
	// cells, and later viewers would get a row for someone who already left
	var slow []*Peer
	for p, m := range w.members {
		w.lookAround(m)
		s.rows, s.out = s.rows[:0], s.out[:0]
		for _, c := range s.cands {
			o := c.e
			m.view.next[o.id] = struct{}{}
			if _, was := m.view.seen[o.id]; was && !o.view.moved {
				continue
			}
			if o.view.rowOf != s.n {
				age := min(now.Sub(o.poseAt).Milliseconds(), maxAgeMs)
				o.view.row = protocol.AppendSnapRow(o.view.row[:0], o.id, o.pose, o.flags, age)
				o.view.rowOf = s.n
			}
			s.rows = append(s.rows, o.view.row)
		}
		for id := range m.view.seen {
			if _, still := m.view.next[id]; !still {
				s.out = append(s.out, id)
			}
		}
		m.view.seen, m.view.next = m.view.next, m.view.seen
		clear(m.view.next)
		if len(s.rows) > 0 || len(s.out) > 0 {
			slices.Sort(s.out)
			if !w.offer(p, protocol.EncodeSnap(at, s.rows, s.out)) {
				slow = append(slow, p)
			}
		}
	}
	for _, m := range w.members {
		m.view.moved = false
	}
	for _, n := range w.npcs {
		n.view.moved = false
	}
	for _, p := range slow {
		if m, ok := w.members[p]; ok {
			w.log.Warn("dropping slow client", "id", m.id)
			w.remove(p)
		}
	}
}

// lookAround fills the scratch candidates with the others in m's view (players and NPCs): those within Radius,
// those already in view within Leave, the nearest Max of them.
func (w *World) lookAround(m *member) {
	s := &w.snaps
	s.cands = s.cands[:0]
	v := w.cfg.View
	enter, leave := v.Radius*v.Radius, v.Leave*v.Leave
	k := w.cellOf(m.pose)
	for dj := int32(-1); dj <= 1; dj++ {
		for di := int32(-1); di <= 1; di++ {
			for _, o := range s.cells[cellKey{k.i + di, k.j + dj}] {
				if o == &m.entity {
					continue
				}
				dx, dz := o.pose.X-m.pose.X, o.pose.Z-m.pose.Z
				d2 := dx*dx + dz*dz
				limit := enter
				if _, was := m.view.seen[o.id]; was {
					limit = leave
				}
				if d2 <= limit {
					s.cands = append(s.cands, candidate{o, d2})
				}
			}
		}
	}
	if len(s.cands) > v.Max {
		slices.SortFunc(s.cands, func(a, b candidate) int { return cmp.Compare(a.d2, b.d2) })
		s.cands = s.cands[:v.Max]
	}
	// pinned members are always in view, however far and however many others there are
	for o := range m.pins {
		if !slices.ContainsFunc(s.cands, func(c candidate) bool { return c.e == &o.entity }) {
			s.cands = append(s.cands, candidate{e: &o.entity})
		}
	}
}
