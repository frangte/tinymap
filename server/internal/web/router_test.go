package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"saigon-noir/server/internal/hub"
)

const waitFor = 2 * time.Second

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestOriginAllowed(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		origin  string
		allowed []string
		want    bool
	}{
		{"no origin (not a browser)", "game.vn", "", nil, true},
		{"same host", "game.vn", "https://game.vn", nil, true},
		{"same host, dev proxy port", "192.168.1.5:5173", "http://192.168.1.5:5173", nil, true},
		{"other site", "game.vn", "https://evil.example", nil, false},
		{"garbage origin", "game.vn", "::", nil, false},
		{"listed origin", "api.game.vn", "https://game.vn", []string{"https://game.vn/"}, true},
		{"unlisted origin", "api.game.vn", "https://other.vn", []string{"https://game.vn"}, false},
		{"same host still allowed with a list", "api.game.vn", "https://api.game.vn", []string{"https://game.vn"}, true},
		{"listed host, wrong scheme", "api.game.vn", "http://game.vn", []string{"https://game.vn"}, false},
		{"several listed", "api.game.vn", "http://localhost:5173", []string{"https://game.vn", "http://localhost:5173"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Host = c.host
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			if got := originAllowed(req, c.allowed); got != c.want {
				t.Fatalf("originAllowed = %v, want %v", got, c.want)
			}
		})
	}
}

// ---- end to end over a real socket

type msg struct {
	T       string          `json:"t"`
	ID      uint32          `json:"id"`
	Name    string          `json:"name"`
	Code    string          `json:"code"`
	Text    string          `json:"text"`
	Max     int             `json:"max"`
	Players []player        `json:"players"`
	Chat    []chatLine      `json:"chat"`
	P       json.RawMessage `json:"p"`
	Look    *look           `json:"look"`
}
type look struct{ Body, Outfit, Hat string }
type player struct {
	ID   uint32      `json:"id"`
	Name string      `json:"name"`
	Look look        `json:"look"`
	Pose *[4]float64 `json:"pose"`
}
type chatLine struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
}

func startServer(t *testing.T, cfg hub.Config) *httptest.Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := hub.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go h.Run(ctx)
	r, err := NewRouter(h, Options{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(r)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})
	return srv
}

func testConfig() hub.Config {
	cfg := hub.DefaultConfig()
	cfg.MaxPlayers = 2
	cfg.SnapInterval = 20 * time.Millisecond
	cfg.HelloTimeout = 300 * time.Millisecond
	cfg.ChatHistory = 2
	return cfg
}

func dial(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	resp.Body.Close()
	t.Cleanup(func() { conn.Close() })
	return conn
}

func send(t *testing.T, c *websocket.Conn, v string) {
	t.Helper()
	if err := c.WriteMessage(websocket.TextMessage, []byte(v)); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// next reads until a message of type `want` arrives, skipping others (snaps, joins).
func next(t *testing.T, c *websocket.Conn, want string) msg {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(waitFor))
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %q: %v", want, err)
		}
		var m msg
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("bad frame %s: %v", data, err)
		}
		if m.T == want {
			return m
		}
	}
}

// closed asserts the server ends the connection (after any frames it still had queued).
func closed(t *testing.T, c *websocket.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(waitFor))
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
				t.Fatal("connection still open")
			}
			return
		}
	}
}

const (
	helloLan  = `{"t":"hello","name":"Lan","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`
	helloMinh = `{"t":"hello","name":"Minh","look":{"body":"man","outfit":"thun","hat":"cap"}}`
)

func TestPlayersSeeEachOther(t *testing.T) {
	srv := startServer(t, testConfig())
	a := dial(t, srv)
	send(t, a, helloLan)
	wa := next(t, a, "welcome")
	if wa.ID == 0 || wa.Name != "Lan" || len(wa.Players) != 0 || wa.Max != 2 {
		t.Fatalf("welcome A = %+v", wa)
	}
	send(t, a, `{"t":"state","x":1.234,"y":0.18,"z":-5,"h":3.1}`)
	next(t, a, "snap") // the hub has A's pose before B arrives

	b := dial(t, srv)
	send(t, b, helloMinh)
	wb := next(t, b, "welcome")
	if len(wb.Players) != 1 || wb.Players[0].Name != "Lan" || wb.Players[0].Look.Outfit != "aodai" {
		t.Fatalf("welcome B players = %+v", wb.Players)
	}
	if p := wb.Players[0].Pose; p == nil || p[0] != 1.23 {
		t.Fatalf("A's pose in B's welcome = %v", p)
	}
	if j := next(t, a, "join"); !strings.Contains(string(j.P), `"Minh"`) {
		t.Fatalf("join seen by A = %s", j.P)
	}

	// positions: B moves, A gets it in a snapshot
	send(t, b, `{"t":"state","x":4,"y":0.2,"z":2,"h":0}`)
	for found := false; !found; {
		s := next(t, a, "snap")
		var rows [][]float64
		if err := json.Unmarshal(s.P, &rows); err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if len(r) != 6 {
				t.Fatalf("row %v: want [id,x,y,z,h,age]", r)
			}
			if r[5] < 0 || r[5] > 1000 {
				t.Fatalf("age %v ms out of range", r[5])
			}
			if uint32(r[0]) == wb.ID && r[1] == 4 && r[3] == 2 {
				found = true
			}
		}
	}

	// outfit change reaches the other player
	send(t, b, `{"t":"profile","name":"Minh","look":{"body":"man","outfit":"xeom","hat":"none"}}`)
	if p := next(t, a, "profile"); p.ID != wb.ID || p.Look == nil || p.Look.Outfit != "xeom" {
		t.Fatalf("profile seen by A = %+v", p)
	}

	// chat is cleaned and echoed to everyone, sender included
	send(t, a, `{"t":"chat","text":"  xin   chào ​"}`)
	for _, c := range []*websocket.Conn{a, b} {
		if m := next(t, c, "chat"); m.Text != "xin chào" || m.Name != "Lan" || m.ID != wa.ID {
			t.Fatalf("chat = %+v", m)
		}
	}

	// leaving tells the others
	b.Close()
	if l := next(t, a, "leave"); l.ID != wb.ID {
		t.Fatalf("leave = %+v", l)
	}
}

func TestFullServerRejects(t *testing.T) {
	srv := startServer(t, testConfig())
	for i := 0; i < 2; i++ {
		c := dial(t, srv)
		send(t, c, helloLan)
		next(t, c, "welcome")
	}
	extra := dial(t, srv)
	send(t, extra, helloMinh)
	if e := next(t, extra, "error"); e.Code != "full" {
		t.Fatalf("error = %+v", e)
	}
	closed(t, extra)
}

func TestBadHelloIsRejected(t *testing.T) {
	srv := startServer(t, testConfig())
	c := dial(t, srv)
	send(t, c, `{"t":"hello","name":"x","look":{"body":"robot","outfit":"thun","hat":"none"}}`)
	if e := next(t, c, "error"); e.Code != "bad_hello" {
		t.Fatalf("error = %+v", e)
	}
	closed(t, c)
}

func TestSilentConnectionTimesOut(t *testing.T) {
	srv := startServer(t, testConfig())
	c := dial(t, srv)
	closed(t, c) // HelloTimeout is 300 ms
}

func TestChatRateLimitAndHistory(t *testing.T) {
	srv := startServer(t, testConfig())
	a := dial(t, srv)
	send(t, a, helloLan)
	next(t, a, "welcome")
	for i := 0; i < 5; i++ {
		send(t, a, `{"t":"chat","text":"spam"}`)
	}
	if e := next(t, a, "error"); e.Code != "rate" {
		t.Fatalf("error = %+v", e)
	}
	// a late joiner gets the last ChatHistory (2) lines
	b := dial(t, srv)
	send(t, b, helloMinh)
	if w := next(t, b, "welcome"); len(w.Chat) != 2 || w.Chat[0].Text != "spam" {
		t.Fatalf("history = %+v", w.Chat)
	}
}

func TestBadMessagesAreDroppedNotFatal(t *testing.T) {
	srv := startServer(t, testConfig())
	a := dial(t, srv)
	send(t, a, helloLan)
	next(t, a, "welcome")
	send(t, a, `not json`)
	send(t, a, `{"t":"state","x":1e9,"y":0,"z":0,"h":0}`)
	send(t, a, `{"t":"teleport"}`)
	send(t, a, `{"t":"chat","text":"vẫn ở đây"}`)
	if m := next(t, a, "chat"); m.Text != "vẫn ở đây" {
		t.Fatalf("chat = %+v", m)
	}
}

func TestHealthzAndForeignOrigin(t *testing.T) {
	srv := startServer(t, testConfig())
	a := dial(t, srv)
	send(t, a, helloLan)
	next(t, a, "welcome")

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		OK      bool `json:"ok"`
		Players int  `json:"players"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if err != nil || !body.OK || body.Players != 1 {
		t.Fatalf("healthz = %+v, %v", body, err)
	}

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	_, r2, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {"https://evil.example"}})
	if err == nil {
		t.Fatal("a foreign origin must not connect")
	}
	if r2 == nil || r2.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v", r2)
	}
	r2.Body.Close()
}
