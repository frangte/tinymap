package web

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/shop"
	"saigon-noir/server/internal/world"
)

// ---- items and the phone shop, over real sockets (docs/design/11-items.md)

// giveItem gives the character behind token an item through the store, as a purchase would: its wallet, the item
// row and the capabilities. The character's next arrival has them.
func giveItem(t *testing.T, srv *testServer, token, item string) {
	t.Helper()
	cat, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	it, ok := cat.Item(item)
	if !ok {
		t.Fatalf("no item %q", item)
	}
	c, err := storedCharacter(srv, token)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := srv.db.ArriveWallet(ctx, c.ID, 10_000_000, 10_000_000, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.db.Buy(ctx, shop.Purchase{Character: c.ID, Item: it.ID, Price: it.Price, Grants: it.Grants, At: 2}); err != nil {
		t.Fatal(err)
	}
}

// standBy saves the character's last position a metre from the NPC, so its next welcome places it there.
func standBy(t *testing.T, srv *testServer, token, npc string) {
	t.Helper()
	cat, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	n, ok := cat.NPCByID(npc)
	if !ok {
		t.Fatalf("no npc %q", npc)
	}
	x, z, _ := n.ThreePose()
	c, err := storedCharacter(srv, token)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.db.SavePose(context.Background(), c.ID, protocol.Pose{X: x - 1, Z: z}, 3); err != nil {
		t.Fatal(err)
	}
}

// nextBalance reads until a wallet message and returns its balance. It looks at nothing else, so it can read past
// messages the test's msg type does not hold (a ride's id is a string).
func nextBalance(t *testing.T, c *websocket.Conn) int64 {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(waitFor))
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for a wallet: %v", err)
		}
		var m struct {
			T       string `json:"t"`
			Balance int64  `json:"balance"`
		}
		if err := json.Unmarshal(data, &m); err == nil && m.T == "wallet" {
			return m.Balance
		}
	}
}

func shopConfig(t *testing.T) world.Config {
	t.Helper()
	cat, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.NPCs, cfg.Dialogue = world.NPCsFrom(cat), dialogue.NewScript(cat)
	return cfg
}

func TestACharacterWithoutAPhoneHasNoAppsAndNoItems(t *testing.T) {
	srv := startServer(t, shopConfig(t))
	_, _, token := postGuest(t, srv, "")
	a := dial(t, srv)
	send(t, a, helloAs(token, "citizen", "baba", "nonla"))
	w := next(t, a, "welcome")
	if len(w.Apps) != 0 || w.Items == nil || len(w.Items) != 0 {
		t.Fatalf("apps %v, items %#v: want no apps and an empty list of items", w.Apps, w.Items)
	}
	if w.Wallet == nil || *w.Wallet != 500_000 {
		t.Fatalf("wallet %v", w.Wallet)
	}
	// a guest without an account has no items at all
	b := dial(t, srv)
	send(t, b, helloAs("", "citizen", "baba", "nonla"))
	if g := next(t, b, "welcome"); g.Items != nil {
		t.Fatalf("a guest's items = %v", g.Items)
	}
}

func TestAPhoneInTheStoreGivesTheAppsOnArrival(t *testing.T) {
	cases := []struct {
		name, arch, outfit, hat, item string
		apps                          []string
		missing                       []string
	}{
		{"a brick", "citizen", "baba", "nonla", "nokla_1280", []string{"notices"}, []string{"crab", "chat"}},
		{"a smartphone", "citizen", "baba", "nonla", "ifone_x", []string{"notices", "crab", "chat", "community"}, nil},
		{"a driver with a smartphone", "driver", "crab", "helmet", "ifone_x", []string{"notices", "crab", "chat", "crab_driver"}, nil},
		{"a driver with only a brick", "driver", "crab", "helmet", "nokla_1280", []string{"notices"}, []string{"crab_driver"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := startServer(t, shopConfig(t))
			_, _, token := postGuest(t, srv, "")
			// the character exists once it has arrived
			first := dial(t, srv)
			send(t, first, helloAs(token, c.arch, c.outfit, c.hat))
			next(t, first, "welcome")
			first.Close()
			giveItem(t, srv, token, c.item)

			a := dial(t, srv)
			send(t, a, helloAs(token, c.arch, c.outfit, c.hat))
			w := next(t, a, "welcome")
			if !slices.Equal(w.Items, []string{c.item}) {
				t.Fatalf("items %v", w.Items)
			}
			for _, app := range c.apps {
				if !slices.Contains(w.Apps, app) {
					t.Errorf("apps %v lack %s", w.Apps, app)
				}
			}
			for _, app := range c.missing {
				if slices.Contains(w.Apps, app) {
					t.Errorf("apps %v have %s", w.Apps, app)
				}
			}
		})
	}
}

// A citizen walks into the shop's reach, buys an Ifone X over the socket: it hears its new balance, its items and
// its apps, and the phone quest completes. Booking a Crab, refused before, is not refused for the phone now.
func TestBuyingAPhoneOverTheSocket(t *testing.T) {
	srv := startServer(t, shopConfig(t))
	_, _, token := postGuest(t, srv, "")
	first := dial(t, srv)
	send(t, first, helloAs(token, "citizen", "baba", "nonla"))
	next(t, first, "welcome")
	first.Close()
	standBy(t, srv, token, "anh_bay")

	a := dial(t, srv)
	send(t, a, helloAs(token, "citizen", "baba", "nonla"))
	next(t, a, "welcome")
	p := next(t, a, "progress")
	if !hasQuest(p, "story.dien_thoai_dau_tien", false) {
		t.Fatalf("the phone quest is offered to a character without a phone: %+v", p.Quests)
	}
	// no smartphone, no Crab, whatever the game shows
	send(t, a, `{"t":"ride_request","dest":"cho_ben_thanh"}`)
	if e := next(t, a, "ride_error"); e.Code != "not_allowed" {
		t.Fatalf("booking without a phone: %+v", e)
	}

	send(t, a, `{"t":"buy","rid":9,"item":"ifone_15_pro"}`)
	if r := next(t, a, "result"); r.Rid != 9 || r.OK || r.Code != protocol.ResBalance {
		t.Fatalf("a phone it cannot pay: %+v", r)
	}
	send(t, a, `{"t":"buy","rid":10,"item":"ifone_x"}`)
	if w := next(t, a, "wallet"); w.Balance != 100_000 {
		t.Fatalf("balance after buying: %+v", w)
	}
	it := next(t, a, "items")
	if !slices.Equal(it.Items, []string{"ifone_x"}) || !slices.Contains(it.Apps, "crab") || !slices.Contains(it.Apps, "notices") {
		t.Fatalf("items %+v", it)
	}
	if r := next(t, a, "result"); r.Rid != 10 || !r.OK {
		t.Fatalf("result %+v", r)
	}
	// the quest hears of it
	for p = next(t, a, "progress"); !hasQuest(p, "story.dien_thoai_dau_tien", true); p = next(t, a, "progress") {
	}

	// again: it is owned, and nothing is charged
	send(t, a, `{"t":"buy","rid":11,"item":"ifone_x"}`)
	if r := next(t, a, "result"); r.Rid != 11 || r.OK || r.Code != protocol.ResOwned {
		t.Fatalf("buying it again: %+v", r)
	}
	// what the new capabilities allow follows at once, without reconnecting
	send(t, a, `{"t":"ride_request","dest":"cho_ben_thanh"}`)
	// the booking is taken, not refused for want of a phone: its fare comes out of the wallet
	if b := nextBalance(t, a); b >= 100_000 {
		t.Fatalf("after booking a Crab: %d, want less than 100000 (the fare is held)", b)
	}
	send(t, a, `{"t":"ride_cancel"}`)
	if b := nextBalance(t, a); b != 100_000 {
		t.Fatalf("after cancelling it: %d", b)
	}
	a.Close()

	// the next visit has it all from the start, and the phone quest is not offered again
	b := dial(t, srv)
	send(t, b, helloAs(token, "citizen", "baba", "nonla"))
	w := next(t, b, "welcome")
	if !slices.Equal(w.Items, []string{"ifone_x"}) || !slices.Contains(w.Apps, "crab") || w.Wallet == nil || *w.Wallet != 100_000 {
		t.Fatalf("next welcome: items %v apps %v wallet %v", w.Items, w.Apps, w.Wallet)
	}
}

func TestBuyingFromTooFarOrUnknownIsRefused(t *testing.T) {
	srv := startServer(t, shopConfig(t))
	_, _, token := postGuest(t, srv, "")
	a := dial(t, srv)
	send(t, a, helloAs(token, "citizen", "baba", "nonla"))
	next(t, a, "welcome")
	// the citizen starts a long way from Anh Bảy's pavement
	for _, c := range []struct{ item, code string }{{"ifone_x", protocol.ResFar}, {"nokia_3310", protocol.ResTarget}} {
		send(t, a, fmt.Sprintf(`{"t":"buy","rid":5,"item":%q}`, c.item))
		if r := next(t, a, "result"); r.Rid != 5 || r.OK || r.Code != c.code {
			t.Fatalf("%s: %+v, want %s", c.item, r, c.code)
		}
	}
	// a guest has no wallet to pay from
	g := dial(t, srv)
	send(t, g, helloAs("", "citizen", "baba", "nonla"))
	next(t, g, "welcome")
	send(t, g, `{"t":"buy","rid":6,"item":"nokla_1280"}`)
	if r := next(t, g, "result"); r.OK || r.Code != protocol.ResBalance {
		t.Fatalf("a guest buying: %+v", r)
	}
}
