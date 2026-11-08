package moderation_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/moderation"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

type member struct {
	id    uint32
	char  string
	name  string
	caps  capability.Set
	muted time.Time
}

func (m *member) ID() uint32                  { return m.id }
func (m *member) Character() string           { return m.char }
func (m *member) Name() string                { return m.name }
func (m *member) Arch() string                { return "" }
func (m *member) Caps() capability.Set        { return m.caps }
func (m *member) Pose() protocol.Pose         { return protocol.Pose{} }
func (m *member) Speed() float64              { return 0 }
func (m *member) Riding() (uint32, int, bool) { return 0, 0, false }
func (m *member) MutedUntil() time.Time       { return m.muted }

type host struct {
	now     time.Time
	members map[uint32]*member
	inbox   map[uint32][]map[string]any
	kicked  map[uint32]string
}

func (h *host) Member(id uint32) (world.Member, bool) {
	m, ok := h.members[id]
	if !ok {
		return nil, false
	}
	return m, true
}

func (h *host) Each(fn func(world.Member)) {
	for _, m := range h.members {
		fn(m)
	}
}

func (h *host) Seat(uint32, uint32) bool { return false }
func (h *host) Unseat(uint32)            {}
func (h *host) Pin(uint32, uint32, bool) {}
func (h *host) Publish(events.Event)     {}
func (h *host) Now() time.Time           { return h.now }

func (h *host) Send(id uint32, frame []byte) {
	var m map[string]any
	if err := json.Unmarshal(frame, &m); err != nil {
		panic(err)
	}
	h.inbox[id] = append(h.inbox[id], m)
}

func (h *host) Do(_ string, job func(context.Context) error, done func(error)) {
	done(job(context.Background()))
}

func (h *host) Kick(id uint32, code string) {
	h.kicked[id] = code
	delete(h.members, id)
}

// notes are the mod_notice texts a player got, and clears the inbox.
func (h *host) notes(id uint32) []string {
	var out []string
	for _, m := range h.inbox[id] {
		if m["t"] == "mod_notice" {
			out = append(out, m["text"].(string))
		}
	}
	h.inbox[id] = nil
	return out
}

func (h *host) got(id uint32, t string) map[string]any {
	for _, m := range h.inbox[id] {
		if m["t"] == t {
			return m
		}
	}
	return nil
}

type store struct {
	muted, banned map[string]int64
	reports       []moderation.Report
	closed        map[int64]bool
}

func (s *store) SetMuted(_ context.Context, c string, until int64, _ string, _ int64) error {
	s.muted[c] = until
	return nil
}

func (s *store) SetBanned(_ context.Context, c string, until int64, _ string, _ int64) error {
	s.banned[c] = until
	return nil
}

func (s *store) UnbanNamed(_ context.Context, name, _ string, _ int64) (int, error) {
	if name == "Kẻ phá" {
		s.banned["c9"] = 0
		return 1, nil
	}
	return 0, nil
}

func (s *store) SaveReport(_ context.Context, r moderation.Report) (int64, error) {
	s.reports = append(s.reports, r)
	return int64(len(s.reports)), nil
}

func (s *store) OpenReports(context.Context, int) ([]moderation.ReportRow, error) {
	var out []moderation.ReportRow
	for i, r := range s.reports {
		if !s.closed[int64(i+1)] {
			names := map[string]string{"c2": "Lan", "c3": "Minh"}
			out = append(out, moderation.ReportRow{ID: int64(i + 1), ReporterName: names[r.Reporter], TargetName: names[r.Target], Context: r.Context, At: r.At})
		}
	}
	return out, nil
}

func (s *store) CloseReport(_ context.Context, id int64, _ string, _ int64) (bool, error) {
	if id < 1 || int(id) > len(s.reports) || s.closed[id] {
		return false, nil
	}
	s.closed[id] = true
	return true, nil
}

const (
	mod   = 1
	lan   = 2
	minh  = 3
	guest = 4
	minh2 = 5
)

func setup() (*host, *store, *moderation.Service) {
	h := &host{now: time.Date(2026, 11, 7, 5, 0, 0, 0, time.UTC), members: map[uint32]*member{}, inbox: map[uint32][]map[string]any{},
		kicked: map[uint32]string{}}
	h.members[mod] = &member{id: mod, char: "cm", name: "Điều hành", caps: capability.Of(content.CapModerate)}
	h.members[lan] = &member{id: lan, char: "c2", name: "Lan", caps: capability.Of()}
	h.members[minh] = &member{id: minh, char: "c3", name: "Minh", caps: capability.Of()}
	h.members[guest] = &member{id: guest, name: "Khách 4", caps: capability.Of()}
	st := &store{muted: map[string]int64{}, banned: map[string]int64{}, closed: map[int64]bool{}}
	return h, st, moderation.New(h, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func chat(s *moderation.Service, h *host, id uint32, text string) bool {
	return s.Chat(h.members[id], protocol.ClientMsg{T: protocol.TypeChat, Ch: protocol.ChannelGlobal, Text: text})
}

func TestOnlyModeratorsUseCommandsAndNoCommandIsBroadcast(t *testing.T) {
	h, st, s := setup()
	if !chat(s, h, lan, "/mute Minh") {
		t.Fatal("a command line went out to everyone")
	}
	if n := h.notes(lan); len(n) != 1 || !strings.Contains(n[0], "chỉ dành cho người điều hành") {
		t.Fatalf("Lan heard %v", n)
	}
	if len(st.muted) != 0 {
		t.Fatal("a player muted someone")
	}
	if chat(s, h, lan, "chào cả nhà") {
		t.Fatal("an ordinary line was taken")
	}
}

func TestAMuteSilencesUntilItEndsAndUnmuteLiftsIt(t *testing.T) {
	h, st, s := setup()
	chat(s, h, mod, "/mute Minh 10m")
	if st.muted["c3"] != h.now.Add(10*time.Minute).UnixMilli() {
		t.Fatalf("stored mute %v", st.muted)
	}
	if n := h.notes(minh); len(n) != 1 || !strings.Contains(n[0], "tắt chat") {
		t.Fatalf("Minh heard %v", n)
	}
	if !chat(s, h, minh, "alo alo") || h.got(minh, "error")["code"] != protocol.ErrMuted {
		t.Fatalf("a muted line went out: %v", h.inbox[minh])
	}
	h.notes(minh)
	h.now = h.now.Add(11 * time.Minute)
	if chat(s, h, minh, "hết rồi") {
		t.Fatal("still muted after the mute ended")
	}
	// one who arrived muted stays muted until a moderator lifts it
	h.members[lan].muted = h.now.Add(time.Hour)
	if !chat(s, h, lan, "hello") {
		t.Fatal("arrived muted, yet the line went out")
	}
	chat(s, h, mod, "/unmute Lan")
	if chat(s, h, lan, "hello") || st.muted["c2"] != 0 {
		t.Fatalf("after /unmute: stored %v", st.muted)
	}
}

func TestTheRightPlayerIsMeant(t *testing.T) {
	h, st, s := setup()
	h.members[minh2] = &member{id: minh2, char: "c5", name: "minh", caps: capability.Of()}
	chat(s, h, mod, "/mute Minh 5m")
	if n := h.notes(mod); len(n) != 1 || !strings.Contains(n[0], "#3") || !strings.Contains(n[0], "#5") {
		t.Fatalf("two Minh: %v", n)
	}
	chat(s, h, mod, "/mute #5 5m")
	if _, ok := st.muted["c5"]; !ok || len(st.muted) != 1 {
		t.Fatalf("muted by number: %v", st.muted)
	}
	h.notes(mod)
	// a guest is kicked, not muted; a moderator is neither
	chat(s, h, mod, "/mute Khách 4")
	if n := h.notes(mod); len(n) != 1 || !strings.Contains(n[0], "/kick") {
		t.Fatalf("muting a guest: %v", n)
	}
	h.members[6] = &member{id: 6, char: "c6", name: "Mod hai", caps: capability.Of(content.CapModerate)}
	chat(s, h, mod, "/ban Mod hai")
	if _, banned := st.banned["c6"]; banned || h.kicked[6] != "" {
		t.Fatal("a moderator was banned")
	}
}

func TestABanKicksAndIsStored(t *testing.T) {
	h, st, s := setup()
	chat(s, h, mod, "/ban Lan 2d")
	if h.kicked[lan] != protocol.ErrBanned {
		t.Fatalf("kicked %v", h.kicked)
	}
	if st.banned["c2"] != h.now.Add(48*time.Hour).UnixMilli() {
		t.Fatalf("stored ban %v", st.banned)
	}
	chat(s, h, mod, "/unban Kẻ phá")
	if n := h.notes(mod); !strings.Contains(strings.Join(n, "|"), "Đã mở khóa Kẻ phá") {
		t.Fatalf("unban: %v", n)
	}
	chat(s, h, mod, "/kick Khách 4")
	if h.kicked[guest] != protocol.ErrKicked {
		t.Fatalf("kick: %v", h.kicked)
	}
}

func TestAReportCarriesTheLinesOfTheTwo(t *testing.T) {
	h, st, s := setup()
	for i, line := range []struct {
		id   uint32
		text string
	}{{minh, "xấu tính"}, {lan, "sao vậy"}, {mod, "không liên quan"}, {minh, "đi chỗ khác"}} {
		h.now = h.now.Add(time.Duration(i) * time.Second)
		chat(s, h, line.id, line.text)
	}
	s.Handle(h.members[lan], protocol.ClientMsg{T: protocol.TypeReport, N: minh})
	if len(st.reports) != 1 {
		t.Fatalf("reports %v", st.reports)
	}
	r := st.reports[0]
	var texts []string
	for _, l := range r.Context {
		texts = append(texts, l.Text)
	}
	if r.Reporter != "c2" || r.Target != "c3" || strings.Join(texts, "|") != "xấu tính|sao vậy|đi chỗ khác" {
		t.Fatalf("report %+v", r)
	}
	if h.got(lan, "reported") == nil {
		t.Fatal("no thanks for the report")
	}
	if n := h.notes(mod); len(n) != 1 || !strings.Contains(n[0], "/reports") {
		t.Fatalf("the moderator heard %v", n)
	}
	// the same report again straight away is not stored twice
	s.Handle(h.members[lan], protocol.ClientMsg{T: protocol.TypeReport, N: minh})
	if len(st.reports) != 1 {
		t.Fatal("a second report within minutes was stored")
	}
	// the moderator reads it and closes it
	chat(s, h, mod, "/reports")
	if n := h.notes(mod); len(n) != 2 || !strings.HasPrefix(n[0], "#1") || !strings.Contains(n[0], "«đi chỗ khác»") {
		t.Fatalf("/reports: %v", n)
	}
	chat(s, h, mod, "/done 1")
	chat(s, h, mod, "/reports")
	if n := h.notes(mod); !strings.Contains(strings.Join(n, "|"), "Không có báo cáo nào") {
		t.Fatalf("after /done: %v", n)
	}
}

func TestGuestsAndSelfCannotReport(t *testing.T) {
	h, st, s := setup()
	s.Handle(h.members[guest], protocol.ClientMsg{T: protocol.TypeReport, N: minh})
	s.Handle(h.members[lan], protocol.ClientMsg{T: protocol.TypeReport, N: lan})
	s.Handle(h.members[lan], protocol.ClientMsg{T: protocol.TypeReport, N: guest})
	if len(st.reports) != 0 {
		t.Fatalf("stored %v", st.reports)
	}
}
