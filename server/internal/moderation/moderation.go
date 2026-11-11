// Package moderation keeps chat safe (docs/design/README.md, D17; docs/design/04-backend.md, Moderation): players
// report someone with the lines around it; moderators (characters with content.CapModerate) mute, ban, kick and
// read the reports with chat commands; a muted player's lines go nowhere. A world module with a chat hook: it
// sees every line first. The database is reached through jobs off the world loop.
package moderation

import (
	"context"
	"log/slog"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

const (
	// keep is how many recent lines are kept for reports
	keep = 300
	// contextLines is how many lines of the two a report carries
	contextLines = 20
	// reportGap: one report per reporter and target this often
	reportGap = 10 * time.Minute
)

// Line is a chat line kept for reports.
type Line struct {
	Name string `json:"name"`
	Ch   string `json:"ch"`
	Text string `json:"text"`
	At   int64  `json:"at"`
}

// Report is a player's report of another, with their last lines, oldest first.
type Report struct {
	Reporter string
	Target   string
	Context  []Line
	At       int64
}

// ReportRow is a report as a moderator reads it.
type ReportRow struct {
	ID           int64
	ReporterName string
	TargetName   string
	Context      []Line
	At           int64
}

// Store keeps what moderation decides (store.DB in production). Characters are ids; times unix milliseconds.
type Store interface {
	// SetMuted and SetBanned put the character's account under a mute or a ban until then (0 lifts it)
	SetMuted(ctx context.Context, character string, until int64, by string, at int64) error
	SetBanned(ctx context.Context, character string, until int64, by string, at int64) error
	// UnbanNamed lifts the ban of the one character with that name; n is how many have it (only 1 is lifted)
	UnbanNamed(ctx context.Context, name, by string, at int64) (n int, err error)
	SaveReport(ctx context.Context, r Report) (int64, error)
	OpenReports(ctx context.Context, limit int) ([]ReportRow, error)
	// CloseReport marks a report handled; false when there is no open one with that id
	CloseReport(ctx context.Context, id int64, by string, at int64) (bool, error)
}

// Service runs moderation in the world.
type Service struct {
	h   world.Host
	db  Store
	log *slog.Logger
	// lines are the recent lines, oldest first (at most keep)
	lines []kept
	// muted are mutes given or lifted while the server runs, by character (zero: lifted); they win over the
	// mute a player arrived with
	muted map[string]time.Time
	// reported is when each (reporter, target) pair last reported
	reported map[[2]string]time.Time
}

type kept struct {
	character string
	line      Line
}

// New makes the module; pass it to world.Use.
func New(h world.Host, db Store, log *slog.Logger) *Service {
	return &Service{h: h, db: db, log: log, muted: map[string]time.Time{}, reported: map[[2]string]time.Time{}}
}

// mutedUntil is when the player's mute ends (zero or past: not muted).
func (s *Service) mutedUntil(m world.Member) time.Time {
	if until, ok := s.muted[m.Character()]; ok {
		return until
	}
	return m.MutedUntil()
}

// Chat sees every line first: commands are taken here, a muted player's lines go nowhere, and every other line is
// kept for reports.
func (s *Service) Chat(m world.Member, msg protocol.ClientMsg) bool {
	if len(msg.Text) > 0 && msg.Text[0] == '/' {
		s.command(m, msg.Text)
		return true
	}
	if until := s.mutedUntil(m); s.h.Now().Before(until) {
		s.h.Send(m.ID(), encode(protocol.Error{T: protocol.TypeError, Code: protocol.ErrMuted}))
		s.tell(m.ID(), protocol.Text{VI: "Bạn đang bị tắt chat đến " + clock(until) + ".", EN: "Your chat is muted until " + clock(until) + "."})
		return true
	}
	if m.Character() != "" {
		s.lines = append(s.lines, kept{character: m.Character(), line: Line{Name: m.Name(), Ch: msg.Ch, Text: msg.Text, At: s.h.Now().UnixMilli()}})
		if over := len(s.lines) - keep; over > 0 {
			s.lines = append(s.lines[:0], s.lines[over:]...)
		}
	}
	return false
}

// Handle takes reports.
func (s *Service) Handle(m world.Member, msg protocol.ClientMsg) bool {
	if msg.T != protocol.TypeReport {
		return false
	}
	s.onReport(m, msg.N)
	return true
}

func (s *Service) onReport(m world.Member, targetID uint32) {
	target, ok := s.h.Member(targetID)
	switch {
	case m.Character() == "":
		s.tell(m.ID(), protocol.Text{VI: "Báo cáo cần một tài khoản.", EN: "Reporting needs an account."})
		return
	case !ok || target.Character() == "" || target.ID() == m.ID():
		s.tell(m.ID(), protocol.Text{VI: "Không báo cáo được người này.", EN: "You can't report this player."})
		return
	}
	pair := [2]string{m.Character(), target.Character()}
	now := s.h.Now()
	if now.Sub(s.reported[pair]) < reportGap {
		s.tell(m.ID(), protocol.Text{VI: "Bạn vừa báo cáo người này rồi. Cảm ơn bạn!", EN: "You just reported this player. Thank you!"})
		return
	}
	s.reported[pair] = now
	r := Report{Reporter: m.Character(), Target: target.Character(), Context: s.contextOf(pair), At: now.UnixMilli()}
	reporter, name := m.ID(), target.Name()
	s.h.Do(m.Character(), func(ctx context.Context) error {
		_, err := s.db.SaveReport(ctx, r)
		return err
	}, func(err error) {
		if err != nil {
			s.log.Error("saving a chat report", "err", err)
			delete(s.reported, pair)
			s.tell(reporter, protocol.Text{VI: "Chưa gửi được báo cáo, thử lại sau nhé.", EN: "The report didn't go through. Please try again later."})
			return
		}
		s.h.Send(reporter, encode(reportedMsg{T: typeReported, N: targetID}))
		s.toModerators("Có báo cáo mới về " + name + ". Gõ /reports để xem.")
	})
}

// contextOf is the last lines of the two players, oldest first.
func (s *Service) contextOf(pair [2]string) []Line {
	var out []Line
	for i := len(s.lines) - 1; i >= 0 && len(out) < contextLines; i-- {
		if k := s.lines[i]; k.character == pair[0] || k.character == pair[1] {
			out = append(out, k.line)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Tick forgets old report times.
func (s *Service) Tick(now time.Time) {
	for pair, at := range s.reported {
		if now.Sub(at) > reportGap {
			delete(s.reported, pair)
		}
	}
}

// Left needs nothing: lines stay for reports, mutes are by character.
func (s *Service) Left(world.Member) {}

// note answers a moderator with a line of text (the game shows it in the chat log). Moderators are the team:
// their replies are in Vietnamese only.
func (s *Service) note(id uint32, text string) {
	s.tell(id, protocol.Text{VI: text})
}

// tell sends a player a line for the chat log, in Vietnamese and English; the game shows the player's language.
func (s *Service) tell(id uint32, t protocol.Text) {
	s.h.Send(id, encode(noticeMsg{T: typeNotice, Text: t.VI, EN: t.EN}))
}

func (s *Service) toModerators(text string) {
	s.h.Each(func(m world.Member) {
		if m.Caps().Has(content.CapModerate) {
			s.note(m.ID(), text)
		}
	})
}

var saigon = time.FixedZone("ICT", 7*60*60)

// clock is a time as players read it: "14:30 07/11", Saigon time.
func clock(t time.Time) string { return t.In(saigon).Format("15:04 02/01") }
