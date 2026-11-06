package world

import (
	"context"
	"hash/fnv"
	"math"
	"time"

	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// Modules run inside the world loop (ride-hailing, server/internal/ridehail): the world hands them the messages
// it does not handle itself, ticks them, and tells them who left. They reach the world through a Host. Nothing here
// knows what a module does; the core only gains generic pieces (seats, pins, jobs) that any module may use.

// Module is something that runs inside the world loop. Every method is called on the world goroutine.
type Module interface {
	// Handle takes a message of a type the world does not know; true when it was the module's.
	Handle(m Member, msg protocol.ClientMsg) bool
	// Tick runs once a snapshot (timeouts, arrival checks).
	Tick(now time.Time)
	// Left: a player left the world (or was replaced); m is still answerable during the call.
	Left(m Member)
}

// Member is what a module may know about a player.
type Member interface {
	ID() uint32
	Character() string
	Name() string
	Arch() string
	Caps() capability.Set
	Pose() protocol.Pose
	// Speed is how fast it moves now, from its last accepted moves (m/s; 0 once it stopped reporting)
	Speed() float64
	// Riding is the vehicle it is on and its seat (0 rider, 1 passenger), vehicle 0 for none; own says whether
	// the vehicle is its own
	Riding() (vehicle uint32, seat int, own bool)
}

// Host is what a module may ask of the world, on the world goroutine.
type Host interface {
	Member(id uint32) (Member, bool)
	// Seat puts a passenger in seat 1 of the vehicle the rider is on; false when that is not possible
	Seat(passenger, rider uint32) bool
	// Unseat puts a passenger down beside the vehicle (its client gets a correction to the spot)
	Unseat(passenger uint32)
	// Pin keeps a and b in each other's view, however far apart
	Pin(a, b uint32, on bool)
	Send(id uint32, frame []byte)
	// Do runs job off the world loop, in order with the character's other jobs, then done with its error back on
	// the world goroutine (not at all if the world stopped)
	Do(character string, job func(ctx context.Context) error, done func(error))
	Publish(ev events.Event)
	Now() time.Time
}

// Use adds a module; call it before Run. make gets the world as the module's host.
func (w *World) Use(make func(Host) Module) {
	w.modules = append(w.modules, make(host{w}))
}

// ---- member as a Member

func (m *member) ID() uint32           { return m.id }
func (m *member) Character() string    { return m.character }
func (m *member) Name() string         { return m.name }
func (m *member) Arch() string         { return m.arch }
func (m *member) Caps() capability.Set { return m.caps }
func (m *member) Pose() protocol.Pose  { return m.pose }

func (m *member) Speed() float64 {
	// a player who stops moving stops reporting: no report for a while means standing still
	if m.clock == nil || m.clock().Sub(m.poseAt) > stillAfter {
		return 0
	}
	return m.speed
}

func (m *member) Riding() (uint32, int, bool) {
	switch {
	case m.riding != nil:
		return m.riding.id, 0, m.riding.owner == m
	case m.seated != nil:
		return m.seated.id, 1, m.seated.owner == m
	}
	return 0, 0, false
}

// stillAfter: no accepted move for this long and a player is standing still.
const stillAfter = 400 * time.Millisecond

// trackSpeed follows an accepted move: speed over the last report, smoothed a little.
func (m *member) trackSpeed(from protocol.Pose, dt time.Duration) {
	if dt <= 0 {
		return
	}
	v := math.Hypot(m.pose.X-from.X, m.pose.Z-from.Z) / dt.Seconds()
	m.speed += (v - m.speed) * 0.5
}

// ---- the world as a Host

type host struct{ w *World }

func (h host) Member(id uint32) (Member, bool) {
	if m := h.w.byID(id); m != nil {
		return m, true
	}
	return nil, false
}

func (h host) Seat(passenger, rider uint32) bool {
	p, r := h.w.byID(passenger), h.w.byID(rider)
	if p == nil || r == nil || r.riding == nil {
		return false
	}
	return h.w.seat(p, r.riding)
}

func (h host) Unseat(passenger uint32) {
	if p := h.w.byID(passenger); p != nil {
		h.w.unseat(p)
	}
}

func (h host) Pin(a, b uint32, on bool) {
	ma, mb := h.w.byID(a), h.w.byID(b)
	if ma == nil || mb == nil || ma == mb {
		return
	}
	pin(ma, mb, on)
	pin(mb, ma, on)
}

func (h host) Send(id uint32, frame []byte) {
	if m := h.w.byID(id); m != nil {
		h.w.sendTo(m.peer, frame)
	}
}

func (h host) Do(character string, job func(ctx context.Context) error, done func(error)) {
	h.w.jobs.do(h.w, character, job, done)
}

func (h host) Publish(ev events.Event) { h.w.publish(ev) }
func (h host) Now() time.Time          { return h.w.now() }

func pin(m, o *member, on bool) {
	if on {
		if m.pins == nil {
			m.pins = map[*member]bool{}
		}
		m.pins[o] = true
	} else {
		delete(m.pins, o)
	}
}

// unpinAll drops every pin of a member leaving the world.
func unpinAll(m *member) {
	for o := range m.pins {
		delete(o.pins, m)
	}
	m.pins = nil
}

// byID finds a member by its id (a few hundred at most: a scan is fine at module message rates).
func (w *World) byID(id uint32) *member {
	for _, m := range w.members {
		if m.id == id {
			return m
		}
	}
	return nil
}

// ---- jobs: work off the loop, answered back in it

// jobQueue is a few workers running modules' jobs (money, here), each character always on the same one.
type jobQueue struct {
	workers []chan func(context.Context)
}

const (
	jobWorkers = 4
	jobBacklog = 256
	// jobTimeout bounds one job, so a stalled database call cannot hold up the jobs queued behind it for long
	jobTimeout = 10 * time.Second
)

func (q *jobQueue) start(ctx context.Context) {
	for range jobWorkers {
		ch := make(chan func(context.Context), jobBacklog)
		q.workers = append(q.workers, ch)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case f := <-ch:
					f(ctx)
				}
			}
		}()
	}
}

// do queues a job; its done comes back through the world's events. A full queue (or a world not running) answers
// at once with errBusy, on the world goroutine, which is where do is called from.
func (q *jobQueue) do(w *World, character string, job func(context.Context) error, done func(error)) {
	if len(q.workers) == 0 {
		done(errBusy)
		return
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(character))
	f := func(ctx context.Context) {
		jctx, cancel := context.WithTimeout(ctx, jobTimeout)
		err := job(jctx)
		cancel()
		w.submit(event{kind: evDone, done: func() { done(err) }})
	}
	select {
	case q.workers[int(h.Sum32()%uint32(len(q.workers)))] <- f:
	default:
		done(errBusy)
	}
}
