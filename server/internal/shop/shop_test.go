package shop_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/shop"
	"saigon-noir/server/internal/store"
	"saigon-noir/server/internal/world"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// member is a player as the module sees it.
type member struct {
	id   uint32
	char string
	caps capability.Set
	pose protocol.Pose
}

func (m *member) ID() uint32                  { return m.id }
func (m *member) Character() string           { return m.char }
func (m *member) Name() string                { return "Lan" }
func (m *member) Arch() string                { return "" }
func (m *member) Caps() capability.Set        { return m.caps }
func (m *member) Pose() protocol.Pose         { return m.pose }
func (m *member) Speed() float64              { return 0 }
func (m *member) Riding() (uint32, int, bool) { return 0, 0, false }
func (m *member) MutedUntil() time.Time       { return time.Time{} }

type job struct {
	run  func(context.Context) error
	done func(error)
}

// host is the world as the module sees it: jobs wait in a queue until settle runs them, as the loop would.
type host struct {
	members map[uint32]*member
	inbox   map[uint32][][]byte
	jobs    []job
	events  []events.Event
}

func (h *host) Member(id uint32) (world.Member, bool) { m, ok := h.members[id]; return m, ok }
func (h *host) Each(fn func(world.Member)) {
	for _, m := range h.members {
		fn(m)
	}
}
func (h *host) Seat(passenger, rider uint32) bool { return false }
func (h *host) Unseat(passenger uint32)           {}
func (h *host) Pin(a, b uint32, on bool)          {}
func (h *host) Send(id uint32, frame []byte)      { h.inbox[id] = append(h.inbox[id], frame) }
func (h *host) Do(_ string, run func(context.Context) error, done func(error)) {
	h.jobs = append(h.jobs, job{run, done})
}
func (h *host) Publish(ev events.Event) { h.events = append(h.events, ev) }
func (h *host) Now() time.Time          { return time.UnixMilli(1_000) }
func (h *host) Kick(uint32, string)     {}

func (h *host) settle() {
	for len(h.jobs) > 0 {
		j := h.jobs[0]
		h.jobs = h.jobs[1:]
		j.done(j.run(context.Background()))
	}
}

type fixture struct {
	t       *testing.T
	h       *host
	db      *store.DB
	s       *shop.Service
	c       *content.Catalog
	granted []string
	npc     content.NPC
}

const lan = 1

func newFixture(t *testing.T, balance int64) *fixture {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db, err := store.Open(ctx, store.Options{Path: filepath.Join(t.TempDir(), "game.db")}, quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.CreateAccount(ctx, "acc", []byte("hash"), 1); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateCharacter(ctx, identity.Character{ID: "charLan", AccountID: "acc", Name: "Lan", CreatedAt: 1,
		Look: protocol.Look{Body: "woman", Outfit: "thun", Hat: "none"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ArriveWallet(ctx, "charLan", balance, balance, "2026-11-19", 1); err != nil {
		t.Fatal(err)
	}
	// a new character owns nothing: the backfill only covers those made before the migration
	if _, err := db.Owned(ctx, "charLan"); err != nil {
		t.Fatal(err)
	}
	npc, _ := c.NPCByID("anh_bay")
	x, z, _ := npc.ThreePose()
	h := &host{members: map[uint32]*member{lan: {id: lan, char: "charLan", caps: capability.Of(), pose: protocol.Pose{X: x + 1, Z: z}}},
		inbox: map[uint32][][]byte{}}
	f := &fixture{t: t, h: h, db: db, c: c, npc: npc}
	f.s = shop.New(h, c, db, quiet)
	f.s.OnCapability = func(character, k string) {
		f.granted = append(f.granted, k)
		if m := h.members[lan]; m != nil {
			m.caps[k] = struct{}{} // what the connection does through the world
		}
	}
	return f
}

func (f *fixture) buy(item string, rid uint32) {
	f.t.Helper()
	f.h.inbox = map[uint32][][]byte{}
	if !f.s.Handle(f.h.members[lan], protocol.ClientMsg{T: protocol.TypeBuy, Item: item, Rid: rid}) {
		f.t.Fatal("buy not handled")
	}
	f.h.settle()
}

// frames are the types of what the player was sent, in order.
func (f *fixture) frames() []string {
	var out []string
	for _, b := range f.h.inbox[lan] {
		var m struct {
			T string `json:"t"`
		}
		_ = json.Unmarshal(b, &m)
		out = append(out, m.T)
	}
	return out
}

func (f *fixture) result() protocol.Result {
	f.t.Helper()
	for _, b := range f.h.inbox[lan] {
		var r protocol.Result
		if err := json.Unmarshal(b, &r); err == nil && r.T == protocol.TypeResult {
			return r
		}
	}
	f.t.Fatalf("no result in %d frames", len(f.h.inbox[lan]))
	return protocol.Result{}
}

func (f *fixture) owned() []string {
	f.t.Helper()
	o, err := f.db.Owned(context.Background(), "charLan")
	if err != nil {
		f.t.Fatal(err)
	}
	return o
}

func (f *fixture) refused(code string) {
	f.t.Helper()
	if r := f.result(); r.OK || r.Code != code {
		f.t.Fatalf("result %+v, want %q", r, code)
	}
	if got := f.frames(); !slices.Equal(got, []string{protocol.TypeResult}) {
		f.t.Fatalf("a refusal sends only the result, got %v", got)
	}
	if len(f.h.events) != 0 {
		f.t.Fatalf("a refusal published %v", f.h.events)
	}
}

func TestBuyingAPhoneAnswersInOrder(t *testing.T) {
	f := newFixture(t, 500_000)
	f.buy("ifone_x", 7)
	if got := f.frames(); !slices.Equal(got, []string{protocol.TypeWallet, protocol.TypeItems, protocol.TypeResult}) {
		t.Fatalf("frames %v: want wallet, items, then the result", got)
	}
	if r := f.result(); !r.OK || r.Rid != 7 {
		t.Fatalf("result %+v", r)
	}
	var w protocol.Wallet
	var it protocol.Items
	_ = json.Unmarshal(f.h.inbox[lan][0], &w)
	_ = json.Unmarshal(f.h.inbox[lan][1], &it)
	if w.Balance != 100_000 {
		t.Fatalf("balance %d", w.Balance)
	}
	if !slices.Equal(it.Items, []string{"ifone_x"}) {
		t.Fatalf("items %v", it.Items)
	}
	// the apps of a smartphone: Crab is among them
	if !slices.Contains(it.Apps, "crab") || !slices.Contains(it.Apps, "notices") {
		t.Fatalf("apps %v", it.Apps)
	}
	if !slices.Equal(f.owned(), []string{"ifone_x"}) {
		t.Fatalf("stored %v", f.owned())
	}
	if !slices.Equal(f.granted, []string{"phone.basic", "phone.smart"}) {
		t.Fatalf("granted %v", f.granted)
	}
	want := events.Event{Kind: events.ItemBought, Character: "charLan", Item: "ifone_x", ItemKind: "phone"}
	if len(f.h.events) != 1 || f.h.events[0].Kind != want.Kind || f.h.events[0].Item != want.Item ||
		f.h.events[0].ItemKind != want.ItemKind || f.h.events[0].Character != want.Character {
		t.Fatalf("events %+v, want %+v", f.h.events, want)
	}
}

func TestABasicPhoneOpensOnlyNotifications(t *testing.T) {
	f := newFixture(t, 500_000)
	f.buy("nokla_1280", 1)
	var it protocol.Items
	_ = json.Unmarshal(f.h.inbox[lan][1], &it)
	if !slices.Equal(it.Apps, []string{"notices"}) {
		t.Fatalf("apps on a brick: %v", it.Apps)
	}
}

func TestTheItemsListHoldsEverythingOwned(t *testing.T) {
	f := newFixture(t, 1_000_000)
	f.buy("nokla_1280", 1)
	f.buy("ifone_x", 2)
	var it protocol.Items
	_ = json.Unmarshal(f.h.inbox[lan][1], &it)
	if !slices.Equal(it.Items, []string{"nokla_1280", "ifone_x"}) {
		t.Fatalf("items %v", it.Items)
	}
}

func TestRefusals(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*fixture)
		item  string
		code  string
	}{
		{"an item that does not exist", func(*fixture) {}, "nokia_3310", protocol.ResTarget},
		{"too far from Anh Bảy", func(f *fixture) {
			f.h.members[lan].pose.X += f.npc.Reach + 0.5
		}, "ifone_x", protocol.ResFar},
		{"more than the wallet holds", func(*fixture) {}, "ifone_15_pro", protocol.ResBalance},
		{"a guest without an account", func(f *fixture) { f.h.members[lan].char = "" }, "ifone_x", protocol.ResBalance},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, 500_000)
			c.setup(f)
			f.buy(c.item, 3)
			f.refused(c.code)
			if len(f.owned()) != 0 || len(f.granted) != 0 {
				t.Fatalf("a refused purchase kept %v and granted %v", f.owned(), f.granted)
			}
		})
	}
}

func TestStandingAtTheEdgeOfReachCanBuy(t *testing.T) {
	f := newFixture(t, 500_000)
	f.h.members[lan].pose.X += f.npc.Reach - 1 // one metre was already added: exactly at the reach
	f.buy("nokla_1280", 1)
	if r := f.result(); !r.OK {
		t.Fatalf("at the edge of reach: %+v", r)
	}
}

func TestBuyingAgainIsRefusedAndCostsNothing(t *testing.T) {
	f := newFixture(t, 500_000)
	f.buy("ifone_x", 1)
	f.h.events = nil
	for range 2 {
		f.buy("ifone_x", 2)
		f.refused(protocol.ResOwned)
	}
	if b, _ := f.db.ArriveWallet(context.Background(), "charLan", 500_000, 500_000, "2026-11-19", 2); b != 100_000 {
		t.Fatalf("balance %d: charged again", b)
	}
	if len(f.granted) != 2 {
		t.Fatalf("granted %v: a retry granted again", f.granted)
	}
}

func TestAPlayerWhoLeftIsNotAnswered(t *testing.T) {
	f := newFixture(t, 500_000)
	f.h.inbox = map[uint32][][]byte{}
	f.s.Handle(f.h.members[lan], protocol.ClientMsg{T: protocol.TypeBuy, Item: "nokla_1280", Rid: 1})
	delete(f.h.members, lan) // gone while the purchase was being written
	f.h.settle()
	if len(f.h.inbox[lan]) != 0 {
		t.Fatalf("sent %d frames to a player who left", len(f.h.inbox[lan]))
	}
	// the purchase stands, and the quest still hears of it
	if len(f.owned()) != 1 || len(f.h.events) != 1 {
		t.Fatalf("owned %v, events %v", f.owned(), f.h.events)
	}
}

func TestOtherMessagesAreNotTheShops(t *testing.T) {
	f := newFixture(t, 1)
	if f.s.Handle(f.h.members[lan], protocol.ClientMsg{T: protocol.TypeChat}) {
		t.Fatal("the shop took a chat line")
	}
}
