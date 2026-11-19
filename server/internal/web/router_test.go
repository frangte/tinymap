package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
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
	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/quest"
	"saigon-noir/server/internal/ridehail"
	"saigon-noir/server/internal/shop"
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
	// welcome: where the server placed us; correct: where to go back to; vehicle: where it is parked
	Pose     []float64 `json:"pose"`
	K        uint32    `json:"k"`
	Vehicles []vehicle `json:"vehicles"`
	Owner    uint32    `json:"owner"`
	Rider    uint32    `json:"rider"`
	// conversations
	NPCs    []protocol.NPC  `json:"npcs"`
	Sid     uint32          `json:"sid"`
	Mode    string          `json:"mode"`
	Step    uint32          `json:"step"`
	Lead    bool            `json:"lead"`
	Say     protocol.Text   `json:"say"`
	Choices []protocol.Text `json:"choices"`
	Reason  string          `json:"reason"`
	Rid     uint32          `json:"rid"`
	OK      bool            `json:"ok"`
	// wallet, items (welcome and items messages)
	Wallet  *int64   `json:"wallet"`
	Balance int64    `json:"balance"`
	Items   []string `json:"items"`
	// progress
	Quests []protocol.QuestState `json:"quests"`
	Stamps []string              `json:"stamps"`
	News   []protocol.News       `json:"news"`
}
type look struct{ Body, Outfit, Hat string }
type player struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	Look look   `json:"look"`
}
type vehicle struct {
	ID    uint32    `json:"id"`
	Kind  string    `json:"kind"`
	Owner uint32    `json:"owner"`
	Rider uint32    `json:"rider"`
	Pose  []float64 `json:"pose"`
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
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	// quests wired as the server does it (cmd/server/main.go)
	engine, err := quest.NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	var quests *quest.Service
	cfg.Places, cfg.Emotes = catalog.Places, world.EmoteNames(catalog)
	cfg.OnEvent = func(ev events.Event) { quests.Submit(ev) }
	// the map's spawn, as the server sets it (cmd/server/main.go)
	cfg.Spawn[0], cfg.Spawn[1] = catalog.Map.ThreeSpawn()
	w := world.New(cfg, log)
	w.Use(func(h world.Host) world.Module {
		return ridehail.New(h, ridehail.ConfigFrom(catalog.Ridehail), catalog.Places, db, log)
	})
	var shops *shop.Service
	w.Use(func(h world.Host) world.Module {
		shops = shop.New(h, catalog, db, log)
		return shops
	})
	quests = quest.NewService(engine, db, w, quest.DefaultConfig(), log)
	go quests.Run(ctx)
	go w.Run(ctx)
	sockets := ws.New(w, accounts, catalog, ws.Config{HelloTimeout: 300 * time.Millisecond}, log)
	sockets.Quests = quests
	sockets.Wallets = ridehail.Purse{Wallets: db, Cfg: catalog.Ridehail, Now: time.Now}
	sockets.Items = db
	quests.OnCapability = sockets.Grant
	shops.OnCapability = sockets.Grant
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

// stateNear is a state message a short walk from where the welcome placed the player: dist metres east. The
// server takes the game's word for walls and floors, so any direction will do.
func stateNear(t *testing.T, welcome msg, dist float64) (state string, x, z float64) {
	t.Helper()
	x, z = welcome.Pose[0]+dist, welcome.Pose[2]
	return fmt.Sprintf(`{"t":"state","x":%.2f,"y":%.2f,"z":%.2f,"h":0}`, x, welcome.Pose[1], z), math.Round(x*100) / 100, math.Round(z*100) / 100
}

// snapRows reads snapshots until one has a row for id, and returns that row.
func snapRow(t *testing.T, c *websocket.Conn, id uint32) []float64 {
	t.Helper()
	for {
		var rows [][]float64
		if err := json.Unmarshal(next(t, c, "snap").P, &rows); err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if len(r) != 7 {
				t.Fatalf("row %v: want [id,x,y,z,h,s,age]", r)
			}
			if r[6] < 0 || r[6] > 60_000 {
				t.Fatalf("age %v ms out of range", r[6])
			}
			if uint32(r[0]) == id {
				return r
			}
		}
	}
}

func testConfig() world.Config {
	cfg := world.DefaultConfig()
	cfg.MaxPlayers = 2
	cfg.SnapInterval = 20 * time.Millisecond
	cfg.ChatHistory = 2
	cfg.Chat.GlobalAfter = 0 // the players here are made a moment before they chat
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
	if wa.ID == 0 || wa.Name != "Lan" || len(wa.Players) != 0 || wa.Max != 2 || len(wa.Pose) != 4 {
		t.Fatalf("welcome A = %+v", wa)
	}
	// both start at the market gate (no archetype): they see each other from the start
	b := dial(t, srv)
	send(t, b, helloMinh)
	wb := next(t, b, "welcome")
	if len(wb.Players) != 1 || wb.Players[0].Name != "Lan" || wb.Players[0].Look.Outfit != "aodai" {
		t.Fatalf("welcome B players = %+v", wb.Players)
	}
	if r := snapRow(t, b, wa.ID); r[1] != wa.Pose[0] || r[3] != wa.Pose[2] {
		t.Fatalf("A's row seen by B = %v, A's welcome pose %v", r, wa.Pose)
	}
	if j := next(t, a, "join"); !strings.Contains(string(j.P), `"Minh"`) {
		t.Fatalf("join seen by A = %s", j.P)
	}

	// positions: B walks a little, A gets it in a snapshot
	state, x, z := stateNear(t, wb, 2)
	send(t, b, state)
	for r := snapRow(t, a, wb.ID); r[1] != x || r[3] != z; r = snapRow(t, a, wb.ID) {
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
		send(t, a, fmt.Sprintf(`{"t":"chat","text":"spam %d"}`, i))
	}
	if e := next(t, a, "error"); e.Code != "rate" {
		t.Fatalf("error = %+v", e)
	}
	// a late joiner gets the last ChatHistory (2) lines
	b := dial(t, srv)
	send(t, b, helloMinh)
	if w := next(t, b, "welcome"); len(w.Chat) != 2 || w.Chat[0].Text != "spam 1" {
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
// asDriver makes the character behind token a Crab driver from before driving became a job earned in play: it
// arrives once as a Citizen, then the store gives it the driver archetype and uniform (a hello cannot pick a job).
func asDriver(t *testing.T, srv *testServer, token string) {
	t.Helper()
	a := dial(t, srv)
	send(t, a, helloAs(token, "citizen", "thun", "none"))
	next(t, a, "welcome")
	a.Close()
	c, err := storedCharacter(srv, token)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.db.ChangeArchetype(context.Background(), c.ID, "driver", protocol.Look{Body: "woman", Outfit: "crab", Hat: "helmet"}, 1); err != nil {
		t.Fatal(err)
	}
}

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
	// no phone yet, so no apps: a phone is an item (docs/design/11-items.md)
	if w.Arch != "traveler" || len(w.Apps) != 0 {
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

	// an older client without archetypes: no badge, and no apps without a phone
	c := dial(t, srv)
	send(t, c, helloMinh)
	if w := next(t, c, "welcome"); w.Arch != "" || len(w.Apps) != 0 {
		t.Fatalf("no archetype: %q apps %v", w.Arch, w.Apps)
	}
	if got, _ := storedCharacter(srv, token); got.Look.Outfit == "crab" {
		t.Fatal("the dropped profile message must not reach the database")
	}
}

// Nobody picks the Crab driver: it is a job earned in play (docs/design/11-items.md). A new character or a guest
// asking for it in a hello starts without an archetype, out of the uniform and with no bike; a change to it is
// ignored.
func TestNobodyPicksTheDriverJob(t *testing.T) {
	srv := startServer(t, testConfig())
	_, _, token := postGuest(t, srv, "")
	a := dial(t, srv)
	send(t, a, helloAs(token, "driver", "crab", "helmet"))
	if w := next(t, a, "welcome"); w.Arch != "" || len(w.Vehicles) != 0 {
		t.Fatalf("a new character asking for the driver: arch %q vehicles %+v", w.Arch, w.Vehicles)
	}
	eventually(t, "the uniform to be taken off", func() bool {
		c, err := storedCharacter(srv, token)
		return err == nil && c.Archetype == "" && c.Look.Outfit != "crab" && c.Look.Hat != "helmet"
	})
	a.Close()

	g := dial(t, srv)
	send(t, g, helloAs("", "driver", "crab", "helmet"))
	if w := next(t, g, "welcome"); w.Arch != "" || len(w.Vehicles) != 0 {
		t.Fatalf("a guest asking for the driver: arch %q vehicles %+v", w.Arch, w.Vehicles)
	}
	g.Close()

	// a Citizen asking to change to the driver gets nothing; its next change (to a Traveler) is the one made
	_, _, other := postGuest(t, srv, "")
	c := dial(t, srv)
	send(t, c, helloAs(other, "citizen", "thun", "none"))
	next(t, c, "welcome")
	send(t, c, `{"t":"arch","arch":"driver"}`)
	send(t, c, `{"t":"arch","arch":"traveler"}`)
	if ch := next(t, c, "arch"); ch.Arch != "traveler" {
		t.Fatalf("the change after asking for the driver: %q", ch.Arch)
	}
}

func TestDriverWearsTheUniform(t *testing.T) {
	srv := startServer(t, testConfig())
	_, _, token := postGuest(t, srv, "")
	asDriver(t, srv, token)
	a := dial(t, srv)
	send(t, a, helloAs(token, "driver", "crab", "helmet"))
	// the Crab Driver app needs a smartphone too (bought, docs/design/11-items.md)
	if w := next(t, a, "welcome"); w.Arch != "driver" || len(w.Apps) != 0 {
		t.Fatalf("driver welcome: %q %v", w.Arch, w.Apps)
	}
	if c, err := storedCharacter(srv, token); err != nil || c.Look.Outfit != "crab" || c.Archetype != "driver" {
		t.Fatalf("stored driver = %+v, %v", c, err)
	}
}

func TestTheArchetypeChangesInGame(t *testing.T) {
	srv := startServer(t, testConfig())
	_, _, token := postGuest(t, srv, "")
	asDriver(t, srv, token)
	a := dial(t, srv)
	send(t, a, helloAs(token, "driver", "crab", "helmet"))
	w := next(t, a, "welcome")
	if len(w.Vehicles) != 1 || w.Vehicles[0].Owner != w.ID {
		t.Fatalf("a driver arrives with a bike: %+v", w.Vehicles)
	}
	bike := w.Vehicles[0].ID
	next(t, a, "progress")
	b := dial(t, srv)
	send(t, b, helloAs("", "traveler", "thun", "cap"))
	next(t, b, "welcome")

	send(t, a, `{"t":"arch","arch":"citizen"}`)
	ch := next(t, a, "arch")
	if ch.Arch != "citizen" || len(ch.Apps) != 0 ||
		ch.Look == nil || *ch.Look != (look{Body: "woman", Outfit: "somi", Hat: "none"}) {
		t.Fatalf("the change: arch %q apps %v look %+v", ch.Arch, ch.Apps, ch.Look)
	}
	if p := next(t, a, "progress"); !hasQuest(p, "citizen.ca_phe_sang", false) || !hasQuest(p, "driver.nhan_xe", false) {
		t.Fatalf("the Citizen's quests open, the driver's stay: %+v", p.Quests)
	}
	if g := next(t, b, "vehicle_gone"); g.ID != bike {
		t.Fatalf("the others see the bike go: %+v", g)
	}
	if pr := next(t, b, "profile"); pr.ID != w.ID || pr.Arch != "citizen" || pr.Look == nil || pr.Look.Outfit != "somi" {
		t.Fatalf("the others see %+v", pr)
	}
	eventually(t, "the change to be stored", func() bool {
		c, err := storedCharacter(srv, token)
		return err == nil && c.Archetype == "citizen" && c.Look.Outfit == "somi"
	})
	// another change at once waits
	send(t, a, `{"t":"arch","arch":"traveler"}`)
	if e := next(t, a, "error"); e.Code != protocol.ErrArchWait {
		t.Fatalf("a second change at once = %+v", e)
	}
	a.Close()

	// the next visit is a Citizen, whatever the hello asks for
	c := dial(t, srv)
	send(t, c, helloAs(token, "driver", "baba", "nonla"))
	if w := next(t, c, "welcome"); w.Arch != "citizen" || len(w.Vehicles) != 0 {
		t.Fatalf("after the change: arch %q vehicles %+v", w.Arch, w.Vehicles)
	}
}

func TestNearbyChatStaysNearby(t *testing.T) {
	cfg := testConfig()
	cfg.MaxPlayers = 3
	srv := startServer(t, cfg)
	// two travellers start at the roundabout, a citizen at home on Lê Lai, 38 m away; with accounts, since guests
	// do not post in global chat
	conns := make([]*websocket.Conn, 3)
	for i, arch := range []string{"traveler", "traveler", "citizen"} {
		_, _, token := postGuest(t, srv, "")
		conns[i] = dial(t, srv)
		send(t, conns[i], helloAs(token, arch, "thun", "none"))
		next(t, conns[i], "welcome")
	}
	near, mid, far := conns[0], conns[1], conns[2]

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

// ---- movement, vehicles and the last position

func TestMovesAreChecked(t *testing.T) {
	srv := startServer(t, testConfig())
	a := dial(t, srv)
	send(t, a, helloLan)
	w := next(t, a, "welcome")
	// 40 m in one report
	send(t, a, fmt.Sprintf(`{"t":"state","x":%v,"y":%v,"z":%v,"h":0}`, w.Pose[0]+40, w.Pose[1], w.Pose[2]))
	c := next(t, a, "correct")
	if c.K != 1 || c.Pose[0] != w.Pose[0] || c.Pose[2] != w.Pose[2] {
		t.Fatalf("correct = %+v, welcome pose %v", c, w.Pose)
	}
	// still k=0: sent before the correction arrived, ignored without another one
	state, _, _ := stateNear(t, w, 1)
	send(t, a, state)
	// echoing k=1 it is accepted: a second player sees it there
	state, x, z := stateNear(t, w, 1.5)
	send(t, a, strings.Replace(state, `"h":0`, `"h":0,"k":1`, 1))
	b := dial(t, srv)
	send(t, b, helloMinh)
	next(t, b, "welcome")
	for r := snapRow(t, b, w.ID); r[1] != x || r[3] != z; r = snapRow(t, b, w.ID) {
	}
}

func TestTheNextVisitStartsWhereTheLastEnded(t *testing.T) {
	srv := startServer(t, testConfig())
	_, _, token := postGuest(t, srv, "")
	a := dial(t, srv)
	send(t, a, helloAs(token, "traveler", "thun", "cap"))
	w := next(t, a, "welcome")
	state, x, z := stateNear(t, w, 2)
	send(t, a, state)
	a.Close() // the world reads the state before it hears of the close
	eventually(t, "the pose to be saved", func() bool {
		c, err := storedCharacter(srv, token)
		return err == nil && c.Pose != nil && math.Abs(c.Pose.X-x) < 0.01 && math.Abs(c.Pose.Z-z) < 0.01
	})
	b := dial(t, srv)
	send(t, b, helloAs(token, "traveler", "thun", "cap"))
	if w := next(t, b, "welcome"); math.Abs(w.Pose[0]-x) > 0.01 || math.Abs(w.Pose[2]-z) > 0.01 {
		t.Fatalf("second visit starts at %v, want (%v, %v)", w.Pose, x, z)
	}
}

func TestTheDriverGetsOnTheirBike(t *testing.T) {
	srv := startServer(t, testConfig())
	_, _, token := postGuest(t, srv, "")
	asDriver(t, srv, token)
	a := dial(t, srv)
	send(t, a, helloAs(token, "driver", "crab", "helmet"))
	w := next(t, a, "welcome")
	if len(w.Vehicles) != 1 || w.Vehicles[0].Owner != w.ID || w.Vehicles[0].Kind != "xe_so_110" || w.Vehicles[0].Rider != 0 {
		t.Fatalf("welcome vehicles = %+v", w.Vehicles)
	}
	bike := w.Vehicles[0].ID
	send(t, a, fmt.Sprintf(`{"t":"mount","v":%d}`, bike+100))
	if e := next(t, a, "error"); e.Code != "mount" {
		t.Fatalf("mounting nothing = %+v", e)
	}
	send(t, a, fmt.Sprintf(`{"t":"mount","v":%d}`, bike))
	if v := next(t, a, "vehicle"); v.ID != bike || v.Rider != w.ID {
		t.Fatalf("vehicle after mounting = %+v", v)
	}
	// a traveller arriving sees the bike, ridden
	b := dial(t, srv)
	send(t, b, helloAs("", "traveler", "thun", "cap"))
	if wb := next(t, b, "welcome"); len(wb.Vehicles) != 1 || wb.Vehicles[0].Rider != w.ID {
		t.Fatalf("vehicles seen by B = %+v", wb.Vehicles)
	}
	send(t, a, `{"t":"dismount"}`)
	if v := next(t, b, "vehicle"); v.Rider != 0 {
		t.Fatalf("vehicle after dismounting = %+v", v)
	}
	a.Close()
	if g := next(t, b, "vehicle_gone"); g.ID != bike {
		t.Fatalf("vehicle_gone = %+v", g)
	}
}

// ---- NPCs

// The phase 3 check over real sockets: walk up to Cô Ba, talk, answer, walk away and the conversation ends.
func TestTalkingToCoBa(t *testing.T) {
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.NPCs, cfg.Dialogue = world.NPCsFrom(catalog), dialogue.NewScript(catalog)
	srv := startServer(t, cfg)
	a := dial(t, srv)
	// no archetype yet: the player starts at the map's spawn, in front of the market (the archetypes start elsewhere)
	send(t, a, helloAs("", "", "thun", "cap"))
	w := next(t, a, "welcome")
	var coBa protocol.NPC
	for _, n := range w.NPCs {
		if n.Key == "co_ba" {
			coBa = n
		}
	}
	if coBa.ID == 0 || len(w.NPCs) != len(catalog.NPCs) {
		t.Fatalf("welcome npcs = %+v", w.NPCs)
	}
	// from too far away she does not hear us
	send(t, a, fmt.Sprintf(`{"t":"interact","n":%d,"rid":1}`, coBa.ID))
	if r := next(t, a, "result"); r.Rid != 1 || r.OK || r.Code != protocol.ResFar {
		t.Fatalf("talking from %v m: %+v", math.Hypot(w.Pose[0]-coBa.Pose[0], w.Pose[2]-coBa.Pose[2]), r)
	}
	walkTo := func(x, z float64) {
		t.Helper()
		// the server lets a player cover a second's worth of walking at once, after standing still that long
		time.Sleep(time.Second)
		send(t, a, fmt.Sprintf(`{"t":"state","x":%.2f,"y":0,"z":%.2f,"h":0}`, x, z))
	}
	// the player starts on the market square, ~10 m from her: walk up in steps along a clear line
	from := [2]float64{w.Pose[0], w.Pose[2]}
	for _, d := range []float64{6, 3, 1.5} {
		x, z := spotToward(coBa.Pose, from, d)
		walkTo(x, z)
		from = [2]float64{x, z}
	}
	send(t, a, fmt.Sprintf(`{"t":"interact","n":%d,"rid":2}`, coBa.ID))
	in := next(t, a, "interaction")
	if in.Mode != protocol.ModePrivate || !in.Lead || len(in.Choices) == 0 || in.Say.VI == "" {
		t.Fatalf("interaction = %+v", in)
	}
	if r := next(t, a, "result"); r.Rid != 2 || !r.OK {
		t.Fatalf("result = %+v", r)
	}
	send(t, a, fmt.Sprintf(`{"t":"choose","sid":%d,"c":0,"step":%d,"rid":3}`, in.Sid, in.Step))
	if l := next(t, a, "interaction"); l.Sid != in.Sid || l.Say.VI == in.Say.VI {
		t.Fatalf("after the answer = %+v", l)
	}
	// walk away: 3 m reach + 2 m slack
	x, z := spotToward(coBa.Pose, from, 6)
	walkTo(x, z)
	if e := next(t, a, "interaction_end"); e.Sid != in.Sid || e.Reason != protocol.EndFar {
		t.Fatalf("end = %+v", e)
	}
}

// spotToward is the spot `dist` metres from the npc on the side of `from`.
func spotToward(npc [4]float64, from [2]float64, dist float64) (x, z float64) {
	a := math.Atan2(from[0]-npc[0], from[1]-npc[2])
	return npc[0] + math.Sin(a)*dist, npc[2] + math.Cos(a)*dist
}

// ---- quests

// A quest over real sockets: a Citizen with an account arrives with the morning coffee running, walks up to Chú Tư
// and orders a cà phê sữa đá, and the server moves both the coffee (its order step) and the neighbourhood round
// (one familiar face talked to).
func TestAQuestMovesOverTheSocket(t *testing.T) {
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.NPCs, cfg.Dialogue = world.NPCsFrom(catalog), dialogue.NewScript(catalog)
	srv := startServer(t, cfg)
	_, _, token := postGuest(t, srv, "")
	a := dial(t, srv)
	send(t, a, helloAs(token, "citizen", "baba", "nonla"))
	w := next(t, a, "welcome")
	p := next(t, a, "progress")
	if !hasQuest(p, "citizen.ca_phe_sang", false) || questN(p, "citizen.ca_phe_sang", "order") != 0 {
		t.Fatalf("arriving: %+v", p.Quests)
	}
	var chuTu protocol.NPC
	for _, n := range w.NPCs {
		if n.Key == "chu_tu" {
			chuTu = n
		}
	}
	from := [2]float64{w.Pose[0], w.Pose[2]}
	for _, d := range []float64{6, 3, 1.5} {
		time.Sleep(time.Second)
		x, z := spotToward(chuTu.Pose, from, d)
		send(t, a, fmt.Sprintf(`{"t":"state","x":%.2f,"y":0,"z":%.2f,"h":0}`, x, z))
		from = [2]float64{x, z}
	}
	send(t, a, fmt.Sprintf(`{"t":"interact","n":%d,"rid":1}`, chuTu.ID))
	in := next(t, a, "interaction")
	// his first answer orders the coffee (which sends order_ca_phe_sua_da)
	send(t, a, fmt.Sprintf(`{"t":"choose","sid":%d,"c":0,"step":%d,"rid":2}`, in.Sid, in.Step))
	for p = next(t, a, "progress"); questN(p, "citizen.ca_phe_sang", "order") != 1; p = next(t, a, "progress") {
	}
	if hasQuest(p, "citizen.ca_phe_sang", true) || questN(p, "citizen.chuyen_khu_pho", "talk") != 1 {
		t.Fatalf("after ordering: quests %+v", p.Quests)
	}
}

// questN is how far a running quest's objective has got (-1 when the quest is not running).
func questN(p msg, id, objective string) int {
	for _, q := range p.Quests {
		if q.ID == id && !q.Done {
			return q.N[objective]
		}
	}
	return -1
}

func hasQuest(p msg, id string, done bool) bool {
	for _, q := range p.Quests {
		if q.ID == id {
			return q.Done == done
		}
	}
	return false
}
