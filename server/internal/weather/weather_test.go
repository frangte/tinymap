package weather

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// an answer of Open-Meteo for District 1 (2026-11-19, 22:15 in Saigon), with the code and rain swapped in
func answer(code int, cloud, precip float64) string {
	b, _ := json.Marshal(map[string]any{
		"utc_offset_seconds": 25200,
		"timezone":           "Asia/Ho_Chi_Minh",
		"current": map[string]any{
			"time": "2026-11-19T22:15", "interval": 900, "temperature_2m": 28.04,
			"weather_code": code, "cloud_cover": cloud, "precipitation": precip,
		},
	})
	return string(b)
}

func TestFromWMO(t *testing.T) {
	cases := []struct {
		code int
		kind Kind
		rain float64
	}{
		{0, Clear, 0}, {2, Clear, 0}, {3, Cloudy, 0}, {45, Cloudy, 0},
		{51, Drizzle, 0.2}, {55, Drizzle, 0.35},
		{61, Rain, 0.5}, {80, Rain, 0.5}, {63, Rain, 0.75}, {65, Rain, 1}, {82, Rain, 1},
		{95, Storm, 1}, {99, Storm, 1}, {1234, Clear, 0},
	}
	for _, c := range cases {
		if k, r := FromWMO(c.code); k != c.kind || r != c.rain {
			t.Errorf("FromWMO(%d) = %s %v, want %s %v", c.code, k, r, c.kind, c.rain)
		}
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		kind  Kind
		rain  float64
		cloud float64
	}{
		{"clear night", answer(0, 1, 0), Clear, 0, 0.01},
		{"rain under few clouds still looks overcast", answer(63, 20, 1.2), Rain, 0.75, 0.7},
		{"wet but the code says dry: drizzle", answer(2, 60, 0.3), Drizzle, 0.2, 0.7},
		{"storm", answer(95, 100, 8), Storm, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, err := parse([]byte(c.body))
			if err != nil {
				t.Fatal(err)
			}
			if w.Kind != c.kind || w.Rain != c.rain || w.Cloud != c.cloud {
				t.Errorf("got %s rain %v cloud %v, want %s %v %v", w.Kind, w.Rain, w.Cloud, c.kind, c.rain, c.cloud)
			}
			if w.Temp != 28 {
				t.Errorf("temp %v, want 28 (one decimal)", w.Temp)
			}
			if want := time.Date(2026, 11, 19, 15, 15, 0, 0, time.UTC); !w.At.Equal(want) {
				t.Errorf("at %v, want %v (Saigon time back to UTC)", w.At, want)
			}
		})
	}
	if _, err := parse([]byte(`{"current":{"time":"2026-11-19T22:15"}}`)); err == nil {
		t.Error("a report without weather_code should be an error")
	}
	if _, err := parse([]byte(`not json`)); err == nil {
		t.Error("garbage should be an error")
	}
}

func TestSource(t *testing.T) {
	var body atomic.Value
	var status atomic.Int32
	body.Store(answer(61, 90, 0.8))
	status.Store(http.StatusOK)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(status.Load()))
		_, _ = io.WriteString(w, body.Load().(string))
	}))
	defer api.Close()

	clock := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)
	s := NewSource(api.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.now = func() time.Time { return clock }
	ctx := context.Background()

	if got := s.Now(); got.Kind != Clear {
		t.Fatalf("before any report: %s, want clear", got.Kind)
	}
	if err := s.Fetch(ctx); err != nil {
		t.Fatal(err)
	}
	if got := s.Now(); got.Kind != Rain || got.Rain != 0.5 {
		t.Fatalf("after a report: %+v, want rain 0.5", got)
	}
	// the API fails: the last report stays
	status.Store(http.StatusBadGateway)
	if err := s.Fetch(ctx); err == nil {
		t.Fatal("HTTP 502 should be an error")
	}
	body.Store(`{}`)
	status.Store(http.StatusOK)
	if err := s.Fetch(ctx); err == nil {
		t.Fatal("an empty report should be an error")
	}
	if got := s.Now(); got.Kind != Rain {
		t.Fatalf("after failures: %s, want the last report (rain)", got.Kind)
	}
	// too long without a good report: clear again
	clock = clock.Add(Stale + time.Minute)
	if got := s.Now(); got.Kind != Clear {
		t.Fatalf("stale report: %s, want clear", got.Kind)
	}
}

// ---- the module, over a fake world

type member struct{ id uint32 }

func (m *member) ID() uint32                  { return m.id }
func (m *member) Character() string           { return "" }
func (m *member) Name() string                { return "" }
func (m *member) Arch() string                { return "" }
func (m *member) Caps() capability.Set        { return nil }
func (m *member) Pose() protocol.Pose         { return protocol.Pose{} }
func (m *member) Speed() float64              { return 0 }
func (m *member) Riding() (uint32, int, bool) { return 0, 0, false }
func (m *member) MutedUntil() time.Time       { return time.Time{} }

type players struct {
	members []*member
	got     map[uint32][]string
}

func (p *players) Each(fn func(world.Member)) {
	for _, m := range p.members {
		fn(m)
	}
}

func (p *players) Send(id uint32, frame []byte) {
	var msg protocol.Weather
	if err := json.Unmarshal(frame, &msg); err != nil || msg.T != protocol.TypeWeather {
		panic("not a weather frame: " + string(frame))
	}
	p.got[id] = append(p.got[id], msg.Kind)
}

type fixed struct{ w Now }

func (f *fixed) Now() Now { return f.w }

func TestModule(t *testing.T) {
	p := &players{members: []*member{{id: 1}, {id: 2}}, got: map[uint32][]string{}}
	src := &fixed{w: Now{Kind: Rain, Rain: 0.5, Cloud: 0.9, Temp: 27.5, At: time.Now()}}
	m := New(p, src)
	at := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)
	tick := func() {
		at = at.Add(checkEvery)
		m.Tick(at)
	}

	tick()
	if len(p.got[1]) != 1 || len(p.got[2]) != 1 || p.got[1][0] != "rain" {
		t.Fatalf("first tick: %v, want rain once each", p.got)
	}
	tick()
	if len(p.got[1]) != 1 {
		t.Fatalf("same weather: sent again (%v)", p.got[1])
	}
	// someone arrives: only they get it
	p.members = append(p.members, &member{id: 3})
	tick()
	if len(p.got[3]) != 1 || len(p.got[1]) != 1 {
		t.Fatalf("arrival: %v", p.got)
	}
	// the weather changes: everyone gets it
	src.w = Now{Kind: Storm, Rain: 1, Cloud: 1, At: time.Now()}
	tick()
	for _, id := range []uint32{1, 2, 3} {
		if got := p.got[id]; len(got) == 0 || got[len(got)-1] != "storm" {
			t.Fatalf("change: player %d got %v, want storm last", id, got)
		}
	}
	// a player leaves and the id comes back: the new owner gets it
	m.Left(p.members[0])
	tick()
	if n := len(p.got[1]); n != 3 {
		t.Fatalf("id 1 after leaving and coming back: %d frames, want 3", n)
	}
	// checks at most once a second
	src.w = ClearSky
	m.Tick(at.Add(checkEvery / 2))
	if got := p.got[2]; got[len(got)-1] != "storm" {
		t.Fatalf("checked again within a second: %v", got)
	}
}

func TestFrameTemp(t *testing.T) {
	var msg map[string]any
	if err := json.Unmarshal(Frame(ClearSky), &msg); err != nil {
		t.Fatal(err)
	}
	if _, ok := msg["temp"]; ok {
		t.Errorf("no report, no temperature: %v", msg)
	}
	if err := json.Unmarshal(Frame(Now{Kind: Clear, Temp: 0, At: time.Now()}), &msg); err != nil {
		t.Fatal(err)
	}
	if v, ok := msg["temp"]; !ok || v != 0.0 {
		t.Errorf("a report at 0 °C keeps its temperature: %v", msg)
	}
}
