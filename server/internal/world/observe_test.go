package world

import (
	"math"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// observingHarness is a world with the game's places and emotes, keeping every event in *evs.
func observingHarness(t *testing.T) (*harness, *[]events.Event) {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	var evs []events.Event
	h := newHarnessWith(t, func(cfg *Config) {
		cfg.Places, cfg.Emotes = c.Places, EmoteNames(c)
		cfg.OnEvent = func(ev events.Event) { evs = append(evs, ev) }
	})
	return h, &evs
}

func ofKind(evs []events.Event, k events.Kind) []events.Event {
	var out []events.Event
	for _, e := range evs {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

// ticks advances the clock in snapshot steps, running the world's tick each time.
func (h *harness) ticks(d time.Duration) {
	step := h.w.cfg.SnapInterval
	for t := time.Duration(0); t < d; t += step {
		h.tick(step)
		h.w.tick()
	}
}

// three.js x, z of a place's (first) point
func placePoint(t *testing.T, id string) (x, z float64) {
	t.Helper()
	c, _ := content.Load()
	p, ok := c.Place(id)
	if !ok {
		t.Fatalf("no place %q", id)
	}
	return p.Pts[0][0], -p.Pts[0][1]
}

func TestEnteringAPlaceTakesTwoSeconds(t *testing.T) {
	h, evs := observingHarness(t)
	x, z := placePoint(t, "nha_tho_duc_ba")
	x, z, _ = h.grid.NearestFit(x, z+18, bodyRadius, 6)
	p, _ := h.join(Arrival{Name: "Lan", Character: "char-lan", Start: &protocol.Pose{X: x, Z: z}})
	h.drain(p)
	h.ticks(time.Second)
	if n := len(ofKind(*evs, events.PlaceEntered)); n != 0 {
		t.Fatalf("entered after 1 s: %d events", n)
	}
	h.ticks(1500 * time.Millisecond)
	entered := ofKind(*evs, events.PlaceEntered)
	if len(entered) != 1 || entered[0].Character != "char-lan" || entered[0].PlaceKind != string(content.Landmark) || entered[0].Riding {
		t.Fatalf("entered %+v", entered)
	}
	place := entered[0].Place
	// staying: time in steps of 5 s, and no second entry
	h.ticks(11 * time.Second)
	if n := len(ofKind(*evs, events.PlaceEntered)); n != 1 {
		t.Fatalf("entered again while staying: %d", n)
	}
	times := ofKind(*evs, events.PlaceTime)
	if len(times) != 2 || times[0].Seconds != 5 || times[0].Place != place {
		t.Fatalf("time spent %+v", times)
	}
}

func TestARefusedMoveEntersNothing(t *testing.T) {
	h, evs := observingHarness(t)
	p, m := h.join(Arrival{Name: "Lan", Character: "char-lan"})
	h.drain(p)
	h.ticks(3 * time.Second)
	*evs = nil
	// a tampered client teleports to the Opera House and stays there
	x, z := placePoint(t, "nha_hat_thanh_pho")
	x, z, _ = h.grid.NearestFit(x-10, z, bodyRadius, 6)
	for range 30 {
		h.tick(100 * time.Millisecond)
		y, _ := h.grid.Floor(x, z)
		h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: x, Y: y, Z: z, K: m.gait.k})
		h.w.tick()
	}
	for _, e := range *evs {
		if e.Place == "nha_hat_thanh_pho" {
			t.Fatalf("the server published %+v for a move it refused", e)
		}
	}
	if m.gait.refused == 0 {
		t.Fatal("the teleport should have been refused")
	}
}

func TestGuestsWithoutAnAccountPublishNothing(t *testing.T) {
	h, evs := observingHarness(t)
	p, _ := h.join(Arrival{Name: "Khách"})
	h.drain(p)
	h.ticks(8 * time.Second)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: h.w.members[p].pose.X, Y: h.w.members[p].pose.Y, Z: h.w.members[p].pose.Z, S: 1 << protocol.EmoteShift})
	if len(*evs) != 0 {
		t.Fatalf("a guest published %+v", *evs)
	}
}

// wave sets a member's emote flag (as a state report from where it stands) and clears it again.
func (h *harness) wave(p *Peer, m *member) {
	h.tick(100 * time.Millisecond)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: m.pose.X, Y: m.pose.Y, Z: m.pose.Z, S: 1 << protocol.EmoteShift, K: m.gait.k})
	h.tick(100 * time.Millisecond)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: m.pose.X, Y: m.pose.Y, Z: m.pose.Z, K: m.gait.k})
}

func TestAGreetingMustBeReturned(t *testing.T) {
	h, evs := observingHarness(t)
	p, m := h.join(Arrival{Name: "Lan", Character: "char-lan"})
	near := m.pose
	near.X += 2
	far := m.pose
	far.X += 30
	pn, mn := h.join(Arrival{Name: "Minh", Character: "char-minh", Start: &near})
	pf, mf := h.join(Arrival{Name: "Tú", Character: "char-tu", Start: &far})
	for _, q := range []*Peer{p, pn, pf} {
		h.drain(q)
	}
	// Lan waves at Minh: an emote, not yet a greeting; holding it is not another emote
	h.tick(100 * time.Millisecond)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: m.pose.X, Y: m.pose.Y, Z: m.pose.Z, S: 1 << protocol.EmoteShift})
	h.tick(100 * time.Millisecond)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeState, X: m.pose.X, Y: m.pose.Y, Z: m.pose.Z, S: 1 << protocol.EmoteShift})
	if n := len(ofKind(*evs, events.Emote)); n != 1 || len(ofKind(*evs, events.Greeted)) != 0 {
		t.Fatalf("one wave: %d emotes, %+v", n, ofKind(*evs, events.Greeted))
	}
	// Tú, far away, waves too: nobody is greeted
	h.wave(pf, mf)
	if g := ofKind(*evs, events.Greeted); len(g) != 0 {
		t.Fatalf("greeted from 30 m: %+v", g)
	}
	// Minh waves back: both are greeted, each by the other
	h.wave(pn, mn)
	got := map[string][]string{}
	for _, g := range ofKind(*evs, events.Greeted) {
		got[g.Character] = g.Partners
	}
	if len(got) != 2 || len(got["char-lan"]) != 1 || got["char-lan"][0] != "char-minh" || got["char-minh"][0] != "char-lan" {
		t.Fatalf("greetings %+v", got)
	}
	// too late to wave back: no greeting
	*evs = nil
	h.tick(greetWindow + time.Second)
	h.wave(pf, mf)
	near2 := mf.pose
	near2.X += 1
	pl, ml := h.join(Arrival{Name: "Hoa", Character: "char-hoa", Start: &near2})
	h.drain(pl)
	h.tick(greetWindow + time.Second)
	h.wave(pl, ml)
	if g := ofKind(*evs, events.Greeted); len(g) != 0 {
		t.Fatalf("a wave returned %v later counted: %+v", greetWindow, g)
	}
}

func TestMountingAndRidingArePublished(t *testing.T) {
	h, evs := observingHarness(t)
	home := protocol.Pose{X: 36, Z: 37}
	p, m := h.join(Arrival{Name: "Tài", Character: "char-tai", Home: &home, Vehicle: &VehicleSpec{Kind: "xe_so_110", Speed: 11}})
	h.drain(p)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeMount, V: m.owns.id})
	if mounted := ofKind(*evs, events.Mounted); len(mounted) != 1 || mounted[0].Vehicle != "xe_so_110" {
		t.Fatalf("mounted %+v", mounted)
	}
	x0, z0, dx, dz := h.lane(m.pose.X, m.pose.Z, 30)
	h.at(p, m, x0, z0)
	h.tick(time.Second)
	h.walk(p, m, x0, z0, dx, dz, 30, 11, 6) // 66 m
	h.walk(p, m, x0, z0, dx, dz, 30, 11, 6) // 132 m
	rode := ofKind(*evs, events.Rode)
	if len(rode) != 1 || rode[0].Metres != 100 {
		t.Fatalf("rode %+v after %.0f m", rode, m.where.rode+100*float64(len(rode)))
	}
	if math.Abs(m.where.rode-32) > 8 {
		t.Fatalf("carried over %.1f m", m.where.rode)
	}
}

func TestTalkingToAnNPCIsPublished(t *testing.T) {
	h, _ := castHarness(t, nil)
	var evs []events.Event
	h.w.cfg.OnEvent = func(ev events.Event) { evs = append(evs, ev) }
	coBa := h.npc("co_ba")
	p, _ := h.joinNear("Lan", "traveler", coBa, 1.5, 0)
	h.interact(p, coBa, 1)
	h.interact(p, coBa, 2) // the line again: not another conversation
	if talked := ofKind(evs, events.NPCTalked); len(talked) != 1 || talked[0].NPC != "co_ba" || talked[0].Character != "char-Lan" {
		t.Fatalf("talked %+v", talked)
	}
}
