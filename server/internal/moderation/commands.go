package moderation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

const (
	muteDefault  = 30 * time.Minute
	muteMax      = 7 * 24 * time.Hour
	banDefault   = 24 * time.Hour
	banMax       = 30 * 24 * time.Hour
	reportsShown = 10
	// quoteMax keeps a report's quoted line short in the chat log (characters)
	quoteMax = 60
)

const help = "Lệnh điều hành: /mute <tên hoặc #số> [30m|2h|1d], /unmute <tên>, /ban <tên> [1d], /unban <tên>, /kick <tên>, /reports, /done <số báo cáo>"

// command runs a moderator's chat command; anyone else is told it is not for them.
func (s *Service) command(m world.Member, text string) {
	if !m.Caps().Has(content.CapModerate) {
		s.tell(m.ID(), protocol.Text{VI: "Lệnh này chỉ dành cho người điều hành.", EN: "This command is for moderators only."})
		return
	}
	f := strings.Fields(text)
	args := f[1:]
	switch strings.ToLower(f[0]) {
	case "/mute":
		s.mute(m, args)
	case "/unmute":
		s.unmute(m, args)
	case "/ban":
		s.ban(m, args)
	case "/unban":
		s.unban(m, args)
	case "/kick":
		s.kick(m, args)
	case "/reports":
		s.reports(m)
	case "/done":
		s.done(m, args)
	default:
		s.note(m.ID(), help)
	}
}

// split takes a trailing duration off the arguments: the rest is a name (names may have spaces).
func split(args []string, fallback, most time.Duration) (string, time.Duration) {
	if len(args) > 1 {
		if d, ok := parseDuration(args[len(args)-1]); ok {
			return strings.Join(args[:len(args)-1], " "), min(d, most)
		}
	}
	return strings.Join(args, " "), fallback
}

// parseDuration reads "30m", "2h", "1d". A bare number is not one: names end in numbers ("Khách 12").
func parseDuration(s string) (time.Duration, bool) {
	var unit time.Duration
	switch {
	case strings.HasSuffix(s, "m"):
		unit = time.Minute
	case strings.HasSuffix(s, "h"):
		unit = time.Hour
	case strings.HasSuffix(s, "d"):
		unit = 24 * time.Hour
	default:
		return 0, false
	}
	s = s[:len(s)-1]
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 100_000 {
		return 0, false
	}
	return time.Duration(n) * unit, true
}

// find is the player a moderator means: "#12" by id, or by name (any case) when one player online has it.
func (s *Service) find(mod world.Member, ref string) (world.Member, bool) {
	if ref == "" {
		s.note(mod.ID(), help)
		return nil, false
	}
	if strings.HasPrefix(ref, "#") {
		if id, err := strconv.ParseUint(ref[1:], 10, 32); err == nil {
			if o, ok := s.h.Member(uint32(id)); ok {
				return o, true
			}
		}
		s.note(mod.ID(), "Không có ai số "+ref+" đang chơi.")
		return nil, false
	}
	var found []world.Member
	s.h.Each(func(o world.Member) {
		if strings.EqualFold(o.Name(), ref) {
			found = append(found, o)
		}
	})
	switch len(found) {
	case 0:
		s.note(mod.ID(), "Không thấy ai tên "+ref+" đang chơi.")
		return nil, false
	case 1:
		return found[0], true
	}
	ids := make([]string, len(found))
	for i, o := range found {
		ids[i] = fmt.Sprintf("#%d", o.ID())
	}
	s.note(mod.ID(), fmt.Sprintf("Có %d người tên %s: dùng số, %s.", len(found), ref, strings.Join(ids, ", ")))
	return nil, false
}

// target is find, for an action that is stored: the player needs an account, and is not a moderator.
func (s *Service) target(mod world.Member, ref string) (world.Member, bool) {
	o, ok := s.find(mod, ref)
	switch {
	case !ok:
		return nil, false
	case o.Character() == "":
		s.note(mod.ID(), o.Name()+" chơi không có tài khoản: dùng /kick.")
		return nil, false
	case o.Caps().Has(content.CapModerate):
		s.note(mod.ID(), "Không áp dụng cho người điều hành.")
		return nil, false
	}
	return o, true
}

// store runs a write off the loop and tells the moderator if it failed.
func (s *Service) store(mod world.Member, what string, job func(ctx context.Context) error, ok func()) {
	id := mod.ID()
	s.h.Do(mod.Character(), job, func(err error) {
		if err != nil {
			s.log.Error("moderation", "action", what, "err", err)
			s.note(id, "Chưa lưu được ("+what+"), thử lại sau.")
			return
		}
		if ok != nil {
			ok()
		}
	})
}

func (s *Service) mute(mod world.Member, args []string) {
	name, d := split(args, muteDefault, muteMax)
	o, ok := s.target(mod, name)
	if !ok {
		return
	}
	until := s.h.Now().Add(d)
	s.muted[o.Character()] = until
	who, char, by := o.Name(), o.Character(), mod.Character()
	s.tell(o.ID(), protocol.Text{VI: "Người điều hành đã tắt chat của bạn đến " + clock(until) + ".", EN: "A moderator has muted your chat until " + clock(until) + "."})
	s.log.Info("muted", "moderator", by, "character", char, "until", until)
	s.store(mod, "mute", func(ctx context.Context) error {
		return s.db.SetMuted(ctx, char, until.UnixMilli(), by, s.h.Now().UnixMilli())
	},
		func() { s.note(mod.ID(), "Đã tắt chat "+who+" đến "+clock(until)+".") })
}

func (s *Service) unmute(mod world.Member, args []string) {
	o, ok := s.target(mod, strings.Join(args, " "))
	if !ok {
		return
	}
	s.muted[o.Character()] = time.Time{}
	who, char, by := o.Name(), o.Character(), mod.Character()
	s.tell(o.ID(), protocol.Text{VI: "Bạn đã được chat lại.", EN: "You can chat again."})
	s.store(mod, "unmute", func(ctx context.Context) error { return s.db.SetMuted(ctx, char, 0, by, s.h.Now().UnixMilli()) },
		func() { s.note(mod.ID(), "Đã cho "+who+" chat lại.") })
}

func (s *Service) ban(mod world.Member, args []string) {
	name, d := split(args, banDefault, banMax)
	o, ok := s.target(mod, name)
	if !ok {
		return
	}
	until := s.h.Now().Add(d)
	who, char, by := o.Name(), o.Character(), mod.Character()
	s.log.Info("banned", "moderator", by, "character", char, "until", until)
	s.h.Kick(o.ID(), protocol.ErrBanned)
	s.store(mod, "ban", func(ctx context.Context) error {
		return s.db.SetBanned(ctx, char, until.UnixMilli(), by, s.h.Now().UnixMilli())
	},
		func() { s.note(mod.ID(), "Đã khóa "+who+" đến "+clock(until)+".") })
}

func (s *Service) unban(mod world.Member, args []string) {
	name := strings.Join(args, " ")
	if name == "" {
		s.note(mod.ID(), help)
		return
	}
	var n int
	id, by := mod.ID(), mod.Character()
	s.store(mod, "unban", func(ctx context.Context) error {
		var err error
		n, err = s.db.UnbanNamed(ctx, name, by, s.h.Now().UnixMilli())
		return err
	}, func() {
		switch n {
		case 0:
			s.note(id, "Không có ai tên "+name+".")
		case 1:
			s.note(id, "Đã mở khóa "+name+".")
		default:
			s.note(id, fmt.Sprintf("Có %d người tên %s: chưa mở khóa ai (dùng sg-server trên máy chủ).", n, name))
		}
	})
}

func (s *Service) kick(mod world.Member, args []string) {
	o, ok := s.find(mod, strings.Join(args, " "))
	if !ok {
		return
	}
	if o.Caps().Has(content.CapModerate) {
		s.note(mod.ID(), "Không áp dụng cho người điều hành.")
		return
	}
	s.log.Info("kicked", "moderator", mod.Character(), "character", o.Character())
	who := o.Name()
	s.h.Kick(o.ID(), protocol.ErrKicked)
	s.note(mod.ID(), "Đã mời "+who+" ra khỏi game.")
}

func (s *Service) reports(mod world.Member) {
	var rows []ReportRow
	id := mod.ID()
	s.store(mod, "reports", func(ctx context.Context) error {
		var err error
		rows, err = s.db.OpenReports(ctx, reportsShown)
		return err
	}, func() {
		if len(rows) == 0 {
			s.note(id, "Không có báo cáo nào đang chờ.")
			return
		}
		for _, r := range rows {
			s.note(id, fmt.Sprintf("#%d %s: %s báo cáo %s%s", r.ID, clock(time.UnixMilli(r.At)), r.ReporterName, r.TargetName, quote(r)))
		}
		s.note(id, "Xong một báo cáo: /done <số>.")
	})
}

// quote is the target's last line in a report, shortened: «…».
func quote(r ReportRow) string {
	for i := len(r.Context) - 1; i >= 0; i-- {
		if r.Context[i].Name == r.TargetName {
			t := []rune(r.Context[i].Text)
			if len(t) > quoteMax {
				t = append(t[:quoteMax], '…')
			}
			return " «" + string(t) + "»"
		}
	}
	return ""
}

func (s *Service) done(mod world.Member, args []string) {
	if len(args) != 1 {
		s.note(mod.ID(), help)
		return
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(args[0], "#"), 10, 64)
	if err != nil {
		s.note(mod.ID(), help)
		return
	}
	var closed bool
	id, by := mod.ID(), mod.Character()
	s.store(mod, "close report", func(ctx context.Context) error {
		var err error
		closed, err = s.db.CloseReport(ctx, n, by, s.h.Now().UnixMilli())
		return err
	}, func() {
		if closed {
			s.note(id, fmt.Sprintf("Đã xong báo cáo #%d.", n))
		} else {
			s.note(id, fmt.Sprintf("Không có báo cáo #%d đang chờ.", n))
		}
	})
}
