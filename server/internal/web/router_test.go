package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/store"
	"saigon-noir/server/internal/world"
	"saigon-noir/server/internal/ws"
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
	Arch    string          `json:"arch"`
	Apps    []string        `json:"apps"`
	Ch      string          `json:"ch"`
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

// testServer is the whole server over HTTP, with its own database in a temporary folder.
type testServer struct {
	*httptest.Server
	db *store.DB
}

func startServer(t *testing.T, cfg world.Config) *testServer { return startWith(t, cfg, Options{}) }

func startWith(t *testing.T, cfg world.Config, opt Options) *testServer {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.Open(ctx, store.Options{Path: filepath.Join(t.TempDir(), "game.db")}, log)
	if err != nil {
		t.Fatal(err)
	}
	accounts := identity.New(db)
	w := world.New(cfg, log)
	go w.Run(ctx)
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	sockets := ws.New(w, accounts, catalog, ws.Config{HelloTimeout: 300 * time.Millisecond}, log)
	r, err := NewRouter(w, sockets, accounts, opt)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(r)
	t.Cleanup(func() {
		cancel()
		srv.Close()
		db.Close()
	})
	return &testServer{Server: srv, db: db}
}

func testConfig() world.Config {
	cfg := world.DefaultConfig()
	cfg.MaxPlayers = 2
	cfg.SnapInterval = 20 * time.Millisecond
	cfg.ChatHistory = 2
	return cfg
}

func dial(t *testing.T, srv *testServer) *websocket.Conn {
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

// ---- accounts

// postGuest asks for a guest token, from a page at origin ("" for no Origin header, like curl).
func postGuest(t *testing.T, srv *testServer, origin string) (status int, header http.Header, token string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/guest", nil)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, resp.Header, body.Token
}

// eventually polls ok until it holds, or fails the test after waitFor.
func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitFor)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// storedCharacter is the character behind a token, read straight from the database.
func storedCharacter(srv *testServer, token string) (identity.Character, error) {
	sum := sha256.Sum256([]byte(token))
	account, err := srv.db.AccountByToken(context.Background(), sum[:], 0)
	if err != nil {
		return identity.Character{}, err
	}
	return srv.db.CharacterOf(context.Background(), account)
}

func TestGuestTokenKeepsTheCharacter(t *testing.T) {
	srv := startServer(t, testConfig())
	status, _, token := postGuest(t, srv, "")
	if status != http.StatusOK || len(token) != 43 {
		t.Fatalf("guest = %d %q", status, token)
	}
	hello := `{"t":"hello","token":"` + token + `","name":"Lan","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`

	a := dial(t, srv)
	send(t, a, hello)
	if w := next(t, a, "welcome"); w.Name != "Lan" {
		t.Fatalf("welcome = %+v", w)
	}
	send(t, a, `{"t":"profile","name":"Lan","look":{"body":"woman","outfit":"xeom","hat":"none"}}`)
	var first identity.Character
	eventually(t, "the outfit change to be stored", func() bool {
		c, err := storedCharacter(srv, token)
		first = c
		return err == nil && c.Look.Outfit == "xeom"
	})
	a.Close()

	// the next visit with the same token is the same character
	b := dial(t, srv)
	send(t, b, hello)
	next(t, b, "welcome")
	if again, err := storedCharacter(srv, token); err != nil || again.ID != first.ID {
		t.Fatalf("character after reconnecting = %+v, %v; want id %s", again, err, first.ID)
	}
}

func TestUnknownTokenIsTurnedAway(t *testing.T) {
	srv := startServer(t, testConfig())
	c := dial(t, srv)
	send(t, c, `{"t":"hello","token":"made-up-token","name":"Lan","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`)
	if e := next(t, c, "error"); e.Code != "auth" {
		t.Fatalf("error = %+v", e)
	}
	closed(t, c)
}

func TestGuestEndpointOriginsAndRate(t *testing.T) {
	srv := startWith(t, testConfig(), Options{AllowedOrigins: []string{"https://game.vn"}})

	// a listed page origin may call it, and its script may read the answer
	status, h, token := postGuest(t, srv, "https://game.vn")
	if status != http.StatusOK || token == "" || h.Get("Access-Control-Allow-Origin") != "https://game.vn" {
		t.Fatalf("listed origin: %d %q, ACAO %q", status, token, h.Get("Access-Control-Allow-Origin"))
	}
	if h.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", h.Get("Cache-Control"))
	}
	// another site may not
	if status, _, _ := postGuest(t, srv, "https://evil.example"); status != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", status)
	}
	// one address gets a burst, then has to wait
	for i := 1; i < guestBurst; i++ {
		if status, _, _ := postGuest(t, srv, ""); status != http.StatusOK {
			t.Fatalf("request %d of the burst: %d", i+1, status)
		}
	}
	if status, _, _ := postGuest(t, srv, ""); status != http.StatusTooManyRequests {
		t.Fatalf("after the burst: %d", status)
	}
}

// ---- archetypes and chat channels

func helloAs(token, arch, outfit, hat string) string {
	return `{"t":"hello","token":"` + token + `","arch":"` + arch + `","name":"Lan","look":{"body":"woman","outfit":"` + outfit + `","hat":"` + hat + `"}}`
}

func TestArchetypeGivesAppsAndIsKept(t *testing.T) {
	srv := startServer(t, testConfig())
	_, _, token := postGuest(t, srv, "")

	a := dial(t, srv)
	send(t, a, helloAs(token, "traveler", "thun", "cap"))
	w := next(t, a, "welcome")
	if w.Arch != "traveler" || !slices.Contains(w.Apps, "guide") || slices.Contains(w.Apps, "crab_driver") {
		t.Fatalf("traveler welcome: arch %q apps %v", w.Arch, w.Apps)
	}
	a.Close()

	// the same character asks to be a driver, in the driver's uniform: it stays a traveler, out of the uniform
	b := dial(t, srv)
	send(t, b, helloAs(token, "driver", "crab", "helmet"))
	if w := next(t, b, "welcome"); w.Arch != "traveler" || slices.Contains(w.Apps, "crab_driver") {
		t.Fatalf("archetype after asking for another: %q apps %v", w.Arch, w.Apps)
	}
	eventually(t, "the uniform to be taken off", func() bool {
		c, err := storedCharacter(srv, token)
		return err == nil && c.Look.Outfit == "thun" && c.Look.Hat == "none"
	})
	// a profile message putting it back on is dropped
	send(t, b, `{"t":"profile","name":"Lan","look":{"body":"woman","outfit":"crab","hat":"helmet"}}`)
	b.Close()

	// an older client without archetypes: the core apps, no badge
	c := dial(t, srv)
	send(t, c, helloMinh)
	if w := next(t, c, "welcome"); w.Arch != "" || len(w.Apps) != 4 {
		t.Fatalf("no archetype: %q apps %v", w.Arch, w.Apps)
	}
	if got, _ := storedCharacter(srv, token); got.Look.Outfit == "crab" {
		t.Fatal("the dropped profile message must not reach the database")
	}
}

func TestDriverWearsTheUniform(t *testing.T) {
	srv := startServer(t, testConfig())
	_, _, token := postGuest(t, srv, "")
	a := dial(t, srv)
	send(t, a, helloAs(token, "driver", "crab", "helmet"))
	if w := next(t, a, "welcome"); w.Arch != "driver" || !slices.Contains(w.Apps, "crab_driver") {
		t.Fatalf("driver welcome: %q %v", w.Arch, w.Apps)
	}
	if c, err := storedCharacter(srv, token); err != nil || c.Look.Outfit != "crab" || c.Archetype != "driver" {
		t.Fatalf("stored driver = %+v, %v", c, err)
	}
}

func TestNearbyChatStaysNearby(t *testing.T) {
	cfg := testConfig()
	cfg.MaxPlayers = 3
	srv := startServer(t, cfg)
	conns := make([]*websocket.Conn, 3)
	for i, x := range []string{"0", "10", "100"} {
		conns[i] = dial(t, srv)
		send(t, conns[i], helloLan)
		next(t, conns[i], "welcome")
		send(t, conns[i], `{"t":"state","x":`+x+`,"y":0.2,"z":0,"h":0}`)
	}
	near, mid, far := conns[0], conns[1], conns[2]
	// wait until the world has placed all three
	for placed := 0; placed < 3; {
		var rows [][]float64
		if err := json.Unmarshal(next(t, far, "snap").P, &rows); err != nil {
			t.Fatal(err)
		}
		placed = len(rows)
	}

	send(t, near, `{"t":"chat","ch":"nearby","text":"gần đây"}`)
	for _, c := range []*websocket.Conn{near, mid} {
		if m := next(t, c, "chat"); m.Text != "gần đây" || m.Ch != "nearby" {
			t.Fatalf("nearby line = %+v", m)
		}
	}
	send(t, near, `{"t":"chat","text":"cả phố"}`)
	if m := next(t, far, "chat"); m.Text != "cả phố" || m.Ch != "global" {
		t.Fatalf("100 m away, the first line heard should be the global one: %+v", m)
	}
}
