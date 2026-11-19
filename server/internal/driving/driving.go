// Package driving is the A1 licence test (docs/design/13-driver.md): what the test's NPC signals do, and the
// practical on the school's lent bike. A world module. The theory is a dialogue (a quiz in dialogue/thay_muoi.json
// whose tallies live in the conversation); this module only turns its signals into tickets:
//
//	fee paid                -> exam.a1.ticket                       (the shop module)
//	thi_bat_dau  (theory)   -> ticket spent, exam.a1.retake given   (any attempt makes the next fee the retake one)
//	ly_thuyet_dau (passed)  -> exam.a1.theory and a ticket for the practical
//	thuc_hanh_bat_dau       -> ticket spent, the school's bike lent, the clock waits for the player to get on
//
// The route is then ridden through checkpoints within the time limit without getting off; the last one passes the
// test (the event test.passed, which the quest turns into the licence) and the capabilities of the attempt go.
// Failing keeps the theory pass and the retake mark: the next practical costs the retake fee.
package driving

import (
	"context"
	"log/slog"
	"math"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// Bank keeps the capabilities the test hands out and takes back (store.DB in production).
type Bank interface {
	GrantCapability(ctx context.Context, character, capability, source string, at int64) error
	RevokeCapability(ctx context.Context, character, capability string) error
}

// How a practical ends badly (protocol.DriveTest.Reason).
const (
	reasonTime = "time"
	reasonOff  = "off"
	reasonIdle = "idle"
)

const grantSource = "test:a1"

// run is one player's practical.
type run struct {
	character string
	// lentAt is when the bike was lent; started is when the player got on and the clock began (zero before)
	lentAt, started time.Time
	next            int
}

// Service is the driving module.
type Service struct {
	h    world.Host
	cat  *content.Catalog
	bank Bank
	log  *slog.Logger
	// OnGrant and OnRevoke tell the connection a capability changed (ws.Server.Grant and Revoke); called from the
	// job that wrote it. Set them before the world runs.
	OnGrant  func(character, capability string)
	OnRevoke func(character, capability string)
	runs     map[uint32]*run
}

// New makes the module; hand it to world.Use.
func New(h world.Host, c *content.Catalog, bank Bank, log *slog.Logger) *Service {
	return &Service{h: h, cat: c, bank: bank, log: log, runs: map[uint32]*run{}}
}

func (s *Service) Handle(world.Member, protocol.ClientMsg) bool { return false }

// Left drops a run: the lent bike goes with the player.
func (s *Service) Left(m world.Member) { delete(s.runs, m.ID()) }

// ArchBusy: a change of archetype waits for a test in progress.
func (s *Service) ArchBusy(m world.Member) bool { return s.runs[m.ID()] != nil }

// ArchChanged: nothing to drop.
func (s *Service) ArchChanged(world.Member) {}

// Signalled turns the test NPC's signals into tickets and starts the practical.
func (s *Service) Signalled(m world.Member, npc, signal string) {
	if npc != s.cat.Driving.NPC || m.Character() == "" {
		return
	}
	caps := m.Caps()
	switch signal {
	case content.SignalTheoryStart:
		if caps.Has(content.CapExamTicket) && !caps.Has(content.CapExamTheory) {
			s.change(m, []string{content.CapExamRetake}, []string{content.CapExamTicket})
		}
	case content.SignalTheoryPass:
		// only an attempt that was started (it gave the retake mark) can pass
		if caps.Has(content.CapExamRetake) && !caps.Has(content.CapExamTheory) {
			s.change(m, []string{content.CapExamTheory, content.CapExamTicket}, nil)
		}
	case content.SignalPracticalGo:
		s.startPractical(m)
	}
}

func (s *Service) startPractical(m world.Member) {
	caps := m.Caps()
	if !caps.Has(content.CapExamTicket) || !caps.Has(content.CapExamTheory) || s.runs[m.ID()] != nil {
		return
	}
	v, ok := s.cat.Vehicle(s.cat.Driving.Vehicle)
	if !ok || !s.h.Lend(m.ID(), world.VehicleSpec{Kind: v.ID, Speed: v.Speed, Requires: v.Requires}) {
		return
	}
	s.runs[m.ID()] = &run{character: m.Character(), lentAt: s.h.Now()}
	s.change(m, nil, []string{content.CapExamTicket})
	s.send(m.ID(), protocol.DriveTest{State: "lent"})
}

// change gives and takes capabilities in one job off the loop.
func (s *Service) change(m world.Member, give, take []string) {
	character, at := m.Character(), s.h.Now().UnixMilli()
	s.h.Do(character, func(ctx context.Context) error {
		for _, k := range give {
			if err := s.bank.GrantCapability(ctx, character, k, grantSource, at); err != nil {
				return err
			}
			if s.OnGrant != nil {
				s.OnGrant(character, k)
			}
		}
		for _, k := range take {
			if err := s.bank.RevokeCapability(ctx, character, k); err != nil {
				return err
			}
			if s.OnRevoke != nil {
				s.OnRevoke(character, k)
			}
		}
		return nil
	}, func(err error) {
		if err != nil {
			s.log.Error("licence test capabilities", "character", character, "give", give, "take", take, "err", err)
		}
	})
}

// Tick follows every practical: the player gets on, rides the checkpoints in order, within the time.
func (s *Service) Tick(now time.Time) {
	for id, r := range s.runs {
		m, ok := s.h.Member(id)
		if !ok {
			delete(s.runs, id)
			continue
		}
		s.follow(id, m, r, now)
	}
}

func (s *Service) follow(id uint32, m world.Member, r *run, now time.Time) {
	d := s.cat.Driving
	// the only vehicle it can be on while lent is the lent one (its own, the world put the other away)
	v, _, own := m.Riding()
	riding := v != 0 && own
	if r.started.IsZero() {
		switch {
		case riding:
			r.started = now
			s.sendRun(id, r, now)
		case now.Sub(r.lentAt).Seconds() > d.MountWithin:
			s.end(id, m, r, "fail", reasonIdle)
		}
		return
	}
	switch {
	case !riding:
		s.end(id, m, r, "fail", reasonOff)
	case now.Sub(r.started).Seconds() > d.TimeLimit:
		s.end(id, m, r, "fail", reasonTime)
	default:
		cp := d.Checkpoints[r.next]
		x, z := cp.ThreeXZ()
		p := m.Pose()
		if math.Hypot(p.X-x, p.Z-z) <= cp.R {
			r.next++
			if r.next == len(d.Checkpoints) {
				s.end(id, m, r, "pass", "")
				return
			}
			s.sendRun(id, r, now)
		}
	}
}

// end finishes a practical: the bike goes back, the player is told, and a pass is published.
func (s *Service) end(id uint32, m world.Member, r *run, state, reason string) {
	delete(s.runs, id)
	s.h.Return(id)
	s.send(id, protocol.DriveTest{State: state, Reason: reason})
	if state != "pass" {
		return
	}
	s.h.Publish(events.Event{Kind: events.TestPassed, Character: r.character, Test: content.TestA1})
	// the attempt is over: nothing of it is kept but the licence the quest gives
	s.change(m, nil, []string{content.CapExamTheory, content.CapExamRetake})
}

func (s *Service) sendRun(id uint32, r *run, now time.Time) {
	d := s.cat.Driving
	x, z := d.Checkpoints[r.next].ThreeXZ()
	left := d.TimeLimit - now.Sub(r.started).Seconds()
	s.send(id, protocol.DriveTest{State: "run", Index: r.next, Total: len(d.Checkpoints), Left: math.Max(0, math.Round(left*10)/10),
		Next: []float64{x, z}, Radius: d.Checkpoints[r.next].R})
}

func (s *Service) send(id uint32, m protocol.DriveTest) {
	m.T = protocol.TypeDriveTest
	s.h.Send(id, protocol.Encode(m))
}
