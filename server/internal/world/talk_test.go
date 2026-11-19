package world

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// castHarness is a world with the game's NPCs talking their real dialogue; the signals sent are kept in *signals.
func castHarness(t *testing.T, tune func(*Config)) (*harness, *[]events.Event) {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	var signals []events.Event
	h := newHarnessWith(t, func(cfg *Config) {
		cfg.NPCs, cfg.Dialogue = NPCsFrom(c), dialogue.NewScript(c)
		cfg.OnEvent = func(ev events.Event) {
			if ev.Kind == events.Signal {
				signals = append(signals, ev)
			}
		}
		if tune != nil {
			tune(cfg)
		}
	})
	return h, &signals
}

func (h *harness) npc(key string) *npc {
	h.t.Helper()
	for _, n := range h.w.npcs {
		if n.spec.Key == key {
			return n
		}
	}
	h.t.Fatalf("no npc %q", key)
	return nil
}

// near is the spot `dist` metres from the NPC at `angle` (rad) round it.
func (h *harness) near(n *npc, dist, angle float64) (x, z float64) {
	return n.pose.X + math.Sin(angle)*dist, n.pose.Z + math.Cos(angle)*dist
}

// joinNear puts a new player `dist` metres from the NPC.
func (h *harness) joinNear(name, arch string, n *npc, dist, angle float64) (*Peer, *member) {
	h.t.Helper()
	x, z := h.near(n, dist, angle)
	p, m := h.join(Arrival{Name: name, Arch: arch, Character: "char-" + name, Start: &protocol.Pose{X: x, Z: z}})
	h.drain(p)
	return p, m
}

func (h *harness) interact(p *Peer, n *npc, rid uint32) {
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeInteract, N: n.id, Rid: rid})
}

// choose answers the line the conversation is at now (the step the player was shown).
func (h *harness) choose(p *Peer, sid uint32, c int, rid uint32) {
	var step uint32
	if s := h.w.sessions[sid]; s != nil {
		step = s.step
	}
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeChoose, Sid: sid, C: c, Step: step, Rid: rid})
}

// only returns the one frame of type t in fs, failing unless there is exactly one.
func only(t *testing.T, fs []frame, typ string) frame {
	t.Helper()
	var found []frame
	for _, f := range fs {
		if f.T == typ {
			found = append(found, f)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one %s, got %d in %+v", typ, len(found), fs)
	}
	return found[0]
}

func resultOf(t *testing.T, fs []frame) (ok bool, code string) {
	t.Helper()
	r := only(t, fs, protocol.TypeResult)
	return r.OK, r.Code
}

// The roadmap's phase 3 check: three players talk to Cô Ba at once, each in a conversation of their own, and
// walking away ends yours only.
func TestThreePlayersTalkToCoBaAtOnce(t *testing.T) {
	h, _ := castHarness(t, nil)
	coBa := h.npc("co_ba")
	type talker struct {
		p   *Peer
		m   *member
		sid uint32
		say string
	}
	var ts []*talker
	for i, who := range []struct{ name, arch string }{{"Lan", "traveler"}, {"Minh", "citizen"}, {"Tuấn", "driver"}} {
		p, m := h.joinNear(who.name, who.arch, coBa, 1.5, float64(i)*2)
		h.interact(p, coBa, uint32(10+i))
		fs, _ := h.drain(p)
		in := only(t, fs, protocol.TypeInteraction)
		if ok, code := resultOf(t, fs); !ok {
			t.Fatalf("%s could not talk: %s", who.name, code)
		}
		if in.N != coBa.id || in.Mode != protocol.ModePrivate || !in.Lead || in.Size != 1 || len(in.Choices) == 0 {
			t.Fatalf("%s: interaction %+v", who.name, in)
		}
		ts = append(ts, &talker{p, m, in.Sid, in.Say.VI})
	}
	if ts[0].sid == ts[1].sid || ts[1].sid == ts[2].sid || ts[0].sid == ts[2].sid {
		t.Fatalf("conversations share an id: %d %d %d", ts[0].sid, ts[1].sid, ts[2].sid)
	}
	if ts[0].say == ts[1].say || ts[1].say == ts[2].say {
		t.Fatalf("every archetype should hear its own greeting: %q / %q / %q", ts[0].say, ts[1].say, ts[2].say)
	}
	if !strings.Contains(ts[1].say, "Minh") {
		t.Fatalf("the citizen's greeting %q should name them", ts[1].say)
	}

	// each conversation moves on its own
	h.choose(ts[0].p, ts[0].sid, 0, 20) // the lesson
	h.choose(ts[1].p, ts[1].sid, 1, 21) // the market
	a, _ := h.drain(ts[0].p)
	b, _ := h.drain(ts[1].p)
	c, _ := h.drain(ts[2].p)
	la, lb := only(t, a, protocol.TypeInteraction), only(t, b, protocol.TypeInteraction)
	if la.Say.VI == lb.Say.VI {
		t.Fatalf("two answers, one line: %q", la.Say.VI)
	}
	if len(c) != 0 {
		t.Fatalf("Tuấn's conversation should not move with the others: %+v", c)
	}

	// Lan walks away: her conversation ends, the others' do not
	x, z := h.near(coBa, coBa.spec.Reach+h.w.cfg.Talk.Slack+1, 0)
	h.at(ts[0].p, ts[0].m, x, z)
	h.tick(100 * time.Millisecond)
	h.w.tick()
	end := only(t, mustDrain(h, ts[0].p), protocol.TypeInteractionEnd)
	if end.Sid != ts[0].sid || end.Reason != protocol.EndFar {
		t.Fatalf("end %+v", end)
	}
	if ts[0].m.talk != nil || len(h.w.sessions) != 2 || coBa.talks != 2 {
		t.Fatalf("after walking away: talk %v, %d sessions, %d talks", ts[0].m.talk, len(h.w.sessions), coBa.talks)
	}
	h.choose(ts[2].p, ts[2].sid, 2, 22)
	if ok, code := resultOf(t, mustDrain(h, ts[2].p)); !ok {
		t.Fatalf("Tuấn's answer after Lan left: %s", code)
	}
	if fs := mustDrain(h, ts[1].p); len(fs) != 0 {
		t.Fatalf("Minh heard someone else's conversation: %+v", fs)
	}
}

func mustDrain(h *harness, p *Peer) []frame {
	fs, _ := h.drain(p)
	return fs
}

func TestStartingAConversationIsChecked(t *testing.T) {
	h, _ := castHarness(t, nil)
	coBa := h.npc("co_ba")
	p, m := h.joinNear("Lan", "traveler", coBa, coBa.spec.Reach+0.6, 0)
	cases := []struct {
		name   string
		target uint32
		code   string
	}{
		{"too far away", coBa.id, protocol.ResFar},
		{"not an npc", m.id, protocol.ResTarget},
		{"nobody", 9999, protocol.ResTarget},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeInteract, N: c.target, Rid: uint32(i + 1)})
			fs := mustDrain(h, p)
			if ok, code := resultOf(t, fs); ok || code != c.code || count(fs, protocol.TypeInteraction) != 0 {
				t.Fatalf("got ok=%v %q (%+v), want %q", ok, code, fs, c.code)
			}
		})
	}
	if len(h.w.sessions) != 0 {
		t.Fatalf("%d conversations started", len(h.w.sessions))
	}

	// close enough, but answers that do not fit the conversation
	x, z := h.near(coBa, 1.5, 0)
	h.at(p, m, x, z)
	h.interact(p, coBa, 7)
	sid := only(t, mustDrain(h, p), protocol.TypeInteraction).Sid
	for _, c := range []struct {
		name string
		sid  uint32
		c    int
		code string
	}{
		{"another conversation", sid + 1, 0, protocol.ResStale},
		{"an answer not on offer", sid, 9, protocol.ResChoice},
	} {
		h.choose(p, c.sid, c.c, 8)
		if ok, code := resultOf(t, mustDrain(h, p)); ok || code != c.code {
			t.Fatalf("%s: ok=%v %q, want %q", c.name, ok, code, c.code)
		}
	}
	// without a request id nothing is answered
	h.choose(p, sid+1, 0, 0)
	if fs := mustDrain(h, p); len(fs) != 0 {
		t.Fatalf("an answer without a request id: %+v", fs)
	}
}

func TestOneConversationAtATime(t *testing.T) {
	h, _ := castHarness(t, func(cfg *Config) {
		// a second NPC standing by Cô Ba
		x, z, hd := cfg.NPCs[0].X, cfg.NPCs[0].Z, cfg.NPCs[0].H
		cfg.NPCs = append(cfg.NPCs, NPCSpec{Key: "co_ba_twin", Name: "Cô Út", X: x + 1.5, Z: z, H: hd, Mode: protocol.ModePrivate, Dialogue: "co_ba", Reach: 3})
	})
	coBa, twin := h.npc("co_ba"), h.npc("co_ba_twin")
	p, m := h.joinNear("Lan", "traveler", coBa, 1, 0)
	h.interact(p, coBa, 1)
	first := only(t, mustDrain(h, p), protocol.TypeInteraction)

	// talking to it again shows the same line again
	h.interact(p, coBa, 2)
	again := only(t, mustDrain(h, p), protocol.TypeInteraction)
	if again.Sid != first.Sid || again.Say != first.Say {
		t.Fatalf("again %+v, first %+v", again, first)
	}

	h.interact(p, twin, 3)
	fs := mustDrain(h, p)
	if end := only(t, fs, protocol.TypeInteractionEnd); end.Sid != first.Sid || end.Reason != protocol.EndOther {
		t.Fatalf("end %+v", end)
	}
	if in := only(t, fs, protocol.TypeInteraction); in.N != twin.id || m.talk == nil || m.talk.npc != twin {
		t.Fatalf("interaction %+v", in)
	}
	if coBa.talks != 0 || twin.talks != 1 || len(h.w.sessions) != 1 {
		t.Fatalf("talks %d / %d, %d sessions", coBa.talks, twin.talks, len(h.w.sessions))
	}

	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeInteractLeave, Sid: m.talk.id})
	if end := only(t, mustDrain(h, p), protocol.TypeInteractionEnd); end.Reason != protocol.EndLeft {
		t.Fatalf("leave %+v", end)
	}
	if m.talk != nil || len(h.w.sessions) != 0 || twin.talks != 0 {
		t.Fatal("closing should end the conversation")
	}
}

func TestAGroupSharesOneConversation(t *testing.T) {
	h, signals := castHarness(t, func(cfg *Config) { cfg.Talk.MaxGroup = 2 })
	guide := h.npc("chi_hanh")
	pa, a := h.joinNear("Tuấn", "traveler", guide, 2, 0)
	pb, b := h.joinNear("Hoa", "citizen", guide, 2, 2)
	pc, _ := h.joinNear("Lan", "traveler", guide, 2, 4)

	h.interact(pa, guide, 1)
	if in := only(t, mustDrain(h, pa), protocol.TypeInteraction); in.Mode != protocol.ModeGroup || !in.Lead || in.Size != 1 {
		t.Fatalf("leader %+v", in)
	}
	h.interact(pb, guide, 2)
	inB := only(t, mustDrain(h, pb), protocol.TypeInteraction)
	inA := only(t, mustDrain(h, pa), protocol.TypeInteraction)
	if inB.Lead || inB.Size != 2 || inA.Size != 2 || !inA.Lead || inA.Sid != inB.Sid {
		t.Fatalf("after Hoa joined: Tuấn %+v, Hoa %+v", inA, inB)
	}
	if inA.Say == inB.Say {
		t.Fatalf("each hears their own variant of the line: %q", inA.Say.VI)
	}
	if len(inA.Choices) != len(inB.Choices) || inA.Choices[0] != inB.Choices[0] {
		t.Fatalf("everyone sees the leader's answers: %v / %v", inA.Choices, inB.Choices)
	}
	h.interact(pc, guide, 3)
	if ok, code := resultOf(t, mustDrain(h, pc)); ok || code != protocol.ResFull {
		t.Fatalf("a third player in a group of two: ok=%v %q", ok, code)
	}

	// only the leader picks; the line moves for both
	h.choose(pb, inB.Sid, 0, 4)
	if ok, code := resultOf(t, mustDrain(h, pb)); ok || code != protocol.ResLead {
		t.Fatalf("Hoa picked: ok=%v %q", ok, code)
	}
	h.choose(pa, inA.Sid, 2, 5) // where next
	na, nb := only(t, mustDrain(h, pa), protocol.TypeInteraction), only(t, mustDrain(h, pb), protocol.TypeInteraction)
	if na.Say.VI != nb.Say.VI || na.Say.VI == inA.Say.VI {
		t.Fatalf("the line should move for both: %q / %q", na.Say.VI, nb.Say.VI)
	}

	// the leader leaves: Hoa leads now
	h.w.onMsg(pa, protocol.ClientMsg{T: protocol.TypeInteractLeave, Sid: inA.Sid})
	if end := only(t, mustDrain(h, pa), protocol.TypeInteractionEnd); end.Reason != protocol.EndLeft {
		t.Fatalf("Tuấn's end %+v", end)
	}
	if in := only(t, mustDrain(h, pb), protocol.TypeInteraction); !in.Lead || in.Size != 1 || a.talk != nil {
		t.Fatalf("Hoa after Tuấn left %+v", in)
	}
	// Hoa finishes the tour: the signal goes to everyone in it (only Hoa now)
	h.choose(pb, inB.Sid, 0, 6)
	mustDrain(h, pb)
	if len(*signals) != 1 || (*signals)[0].Signal != "tour_lam_son" || (*signals)[0].Character != "char-Hoa" || (*signals)[0].NPC != "chi_hanh" {
		t.Fatalf("signals %+v", *signals)
	}
	// the last line has no answers: only goodbye; Hoa leaving the world ends the conversation
	h.w.remove(pb)
	if b.talk != nil || len(h.w.sessions) != 0 || guide.talks != 0 || guide.group != nil {
		t.Fatalf("after the last player left: %d sessions, %d talks", len(h.w.sessions), guide.talks)
	}
}

func TestAnAnswerSendsItsSignal(t *testing.T) {
	h, signals := castHarness(t, nil)
	coBa := h.npc("co_ba")
	p, _ := h.joinNear("Lan", "traveler", coBa, 1.5, 0)
	h.interact(p, coBa, 1)
	sid := only(t, mustDrain(h, p), protocol.TypeInteraction).Sid
	// greet → the lesson → greeting → thanks → asking the price: each answer sends its signal
	for i := range 4 {
		h.choose(p, sid, 0, uint32(2+i))
		if ok, code := resultOf(t, mustDrain(h, p)); !ok {
			t.Fatalf("answer %d: %s", i, code)
		}
	}
	want := []string{"phrase_xin_chao", "phrase_cam_on", "greetings_learned"}
	if len(*signals) != len(want) {
		t.Fatalf("signals %+v", *signals)
	}
	for i, s := range *signals {
		if s.Character != "char-Lan" || s.NPC != "co_ba" || s.Signal != want[i] {
			t.Fatalf("signal %d %+v, want %s", i, s, want[i])
		}
	}
}

func TestPublicLinesAreHeardNearby(t *testing.T) {
	h, _ := castHarness(t, nil)
	diNam := h.npc("di_nam")
	p, _ := h.joinNear("Lan", "traveler", diNam, 1.5, 0)
	near, _ := h.joinNear("Minh", "citizen", diNam, h.w.cfg.Talk.VoiceRadius-2, 1)
	far, _ := h.joinNear("Tuấn", "driver", diNam, h.w.cfg.Talk.VoiceRadius+6, 2)
	h.interact(p, diNam, 1)
	in := only(t, mustDrain(h, p), protocol.TypeInteraction)
	heard := only(t, mustDrain(h, near), protocol.TypeChat)
	if !heard.NPC || heard.ID != diNam.id || heard.Name != "Dì Năm" || heard.Text != in.Say.VI || heard.EN == "" || heard.EN != in.Say.EN {
		t.Fatalf("heard %+v, said %q", heard, in.Say.VI)
	}
	if fs := mustDrain(h, far); count(fs, protocol.TypeChat) != 0 {
		t.Fatalf("heard from too far: %+v", fs)
	}
}

func TestTheNPCTurnsToTheSpeakerAndBack(t *testing.T) {
	h, _ := castHarness(t, nil)
	coBa := h.npc("co_ba")
	rest := coBa.spec.H
	// stand behind her, where she has to turn round to see us
	p, m := h.joinNear("Lan", "traveler", coBa, 1.5, rest+math.Pi)
	h.interact(p, coBa, 1)
	mustDrain(h, p)
	if protocol.EmoteOf(coBa.flags) == 0 {
		t.Fatal("she should wave as she greets")
	}
	toward := math.Atan2(m.pose.X-coBa.pose.X, m.pose.Z-coBa.pose.Z)
	for range 24 { // two seconds
		h.tick(time.Second / 12)
		h.w.tick()
		h.w.sendSnapshots()
	}
	if d := math.Abs(wrapAngle(coBa.pose.H - toward)); d > 0.01 {
		t.Fatalf("heading %.2f, toward the player %.2f", coBa.pose.H, toward)
	}
	if protocol.EmoteOf(coBa.flags) == 0 {
		t.Fatal("two seconds into a 2.4 s wave she is still waving")
	}
	sawHer := false
	for _, f := range mustDrain(h, p) {
		if f.T == protocol.TypeSnap && strings.Contains(string(f.P), "["+strconv.Itoa(int(coBa.id))+",") {
			sawHer = true
		}
	}
	if !sawHer {
		t.Fatal("snapshots should carry her turning")
	}
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeInteractLeave, Sid: m.talk.id})
	for range 12 * 9 { // past LookFor and the turn back
		h.tick(time.Second / 12)
		h.w.tick()
	}
	if d := math.Abs(wrapAngle(coBa.pose.H - rest)); d > 0.01 || protocol.EmoteOf(coBa.flags) != 0 {
		t.Fatalf("heading %.2f, rest %.2f, emote %d", coBa.pose.H, rest, protocol.EmoteOf(coBa.flags))
	}
}

func TestNPCsCallToPassersBy(t *testing.T) {
	h, _ := castHarness(t, func(cfg *Config) { cfg.Talk.BarkJitter = 0 })
	chuSau := h.npc("chu_sau")
	p, _ := h.joinNear("Lan", "traveler", chuSau, 5, 0)
	for range 3 {
		h.tick(time.Second / 12)
		h.w.tick()
	}
	call := only(t, mustDrain(h, p), protocol.TypeChat)
	if !call.NPC || call.ID != chuSau.id || call.Text != chuSau.spec.Barks[0].VI || call.EN == "" || call.EN != chuSau.spec.Barks[0].EN {
		t.Fatalf("call %+v", call)
	}
	// the next one waits its turn
	h.tick(time.Second)
	h.w.tick()
	if fs := mustDrain(h, p); len(fs) != 0 {
		t.Fatalf("called again at once: %+v", fs)
	}
}

func TestNPCsAreInTheWelcome(t *testing.T) {
	h, _ := castHarness(t, nil)
	p := h.w.NewPeer()
	h.w.onJoin(p, Arrival{Name: "Lan"})
	w := only(t, mustDrain(h, p), protocol.TypeWelcome)
	if len(w.NPCs) != len(h.w.npcs) || len(w.NPCs) == 0 {
		t.Fatalf("welcome npcs %+v", w.NPCs)
	}
	for i, n := range h.w.npcs {
		if got := w.NPCs[i]; got.ID != n.id || got.Key != n.spec.Key || math.Abs(got.Pose[0]-n.pose.X) > 0.01 {
			t.Fatalf("npc %d: %+v, want %s at %+v", i, got, n.spec.Key, n.pose)
		}
	}
}

func TestAnAnswerToAnOldLineIsRefused(t *testing.T) {
	h, _ := castHarness(t, nil)
	coBa := h.npc("co_ba")
	p, _ := h.joinNear("Lan", "traveler", coBa, 1.5, 0)
	h.interact(p, coBa, 1)
	first := only(t, mustDrain(h, p), protocol.TypeInteraction)
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeChoose, Sid: first.Sid, C: 0, Step: first.Step, Rid: 2})
	second := only(t, mustDrain(h, p), protocol.TypeInteraction)
	if second.Step != first.Step+1 {
		t.Fatalf("steps %d then %d", first.Step, second.Step)
	}
	// a second click on the first line's buttons, sent before the second line arrived
	h.w.onMsg(p, protocol.ClientMsg{T: protocol.TypeChoose, Sid: first.Sid, C: 0, Step: first.Step, Rid: 3})
	if ok, code := resultOf(t, mustDrain(h, p)); ok || code != protocol.ResStale {
		t.Fatalf("an answer to the old line: ok=%v %q", ok, code)
	}
}

func TestPublicLinesAreNotHeardFasterThanPublicEvery(t *testing.T) {
	h, _ := castHarness(t, nil)
	diNam := h.npc("di_nam")
	p, _ := h.joinNear("Lan", "traveler", diNam, 1.5, 0)
	near, _ := h.joinNear("Minh", "citizen", diNam, 5, 1)
	h.interact(p, diNam, 1)
	sid := only(t, mustDrain(h, p), protocol.TypeInteraction).Sid
	h.choose(p, sid, 1, 2) // what is in it: answered at once
	mustDrain(h, p)
	if n := count(mustDrain(h, near), protocol.TypeChat); n != 1 {
		t.Fatalf("a bystander heard %d lines in one breath, want 1", n)
	}
	h.tick(h.w.cfg.Talk.PublicEvery)
	h.choose(p, sid, 0, 3)
	mustDrain(h, p)
	if n := count(mustDrain(h, near), protocol.TypeChat); n != 1 {
		t.Fatalf("after the pause: %d lines, want 1", n)
	}
}

// Anh Bảy's shop node carries the shop id in its interaction line, and no other line does.
func TestTheShopNodeSendsItsShop(t *testing.T) {
	h, _ := castHarness(t, nil)
	anh := h.npc("anh_bay")
	p, _ := h.joinNear("Lan", "citizen", anh, 1.5, 0)
	h.interact(p, anh, 1)
	greet := only(t, mustDrain(h, p), protocol.TypeInteraction)
	if greet.Shop != "" {
		t.Fatalf("the greeting opens shop %q", greet.Shop)
	}
	h.choose(p, greet.Sid, 0, 2) // "Cho em coi máy"
	if line := only(t, mustDrain(h, p), protocol.TypeInteraction); line.Shop != "de_yeu" {
		t.Fatalf("the shop line has shop %q", line.Shop)
	}
}

// A welcome carries the items of a character (an empty list for one with none), and none for a guest.
func TestTheWelcomeCarriesItems(t *testing.T) {
	h := newHarness(t)
	for _, c := range []struct {
		name  string
		items []string
		want  string
	}{
		{"Lan", []string{"nokla_1280", "ifone_x"}, `["nokla_1280","ifone_x"]`},
		{"Minh", []string{}, `[]`},
		{"Khách", nil, ``},
	} {
		p, _ := h.join(Arrival{Name: c.name, Items: c.items})
		if got := string(only(t, mustDrain(h, p), protocol.TypeWelcome).Items); got != c.want {
			t.Errorf("%s: welcome items %q, want %q", c.name, got, c.want)
		}
	}
}

// Every NPC greets a newcomer with its default line (no variant is written for one), and the line reads right for
// someone who just arrived: no placeholder left over, and nothing meant only for a neighbour or a native.
func TestEveryNPCGreetsANewcomerWithItsDefaultLine(t *testing.T) {
	h, _ := castHarness(t, nil)
	for i, key := range []string{"co_ba", "chu_tu", "di_nam", "chi_hanh", "chu_sau"} {
		n := h.npc(key)
		p, _ := h.joinNear("Tay"+strconv.Itoa(i), "newcomer", n, 1.5, 0)
		h.interact(p, n, 1)
		fs, _ := h.drain(p)
		in := only(t, fs, protocol.TypeInteraction)
		if in.Say.VI == "" || in.Say.EN == "" || strings.ContainsAny(in.Say.VI+in.Say.EN, "{}") {
			t.Errorf("%s greets a newcomer with %+v", key, in.Say)
		}
		// the line a native hears ("Ủa {name}...", "Như cũ hả con?") must not reach a newcomer
		pc, _ := h.joinNear("Minh"+strconv.Itoa(i), "citizen", n, 1.5, 1)
		h.interact(pc, n, 2)
		natFrames, _ := h.drain(pc)
		if nat := only(t, natFrames, protocol.TypeInteraction); nat.Say.VI == in.Say.VI {
			t.Errorf("%s: the native's own greeting %q is the newcomer's too", key, nat.Say.VI)
		}
	}
}

// {anh_chi} follows the body of the player: Chị Hạnh greets a Crab driver as "anh" or "chị".
func TestAnhChiInAGreetingFollowsTheBody(t *testing.T) {
	h, _ := castHarness(t, nil)
	hanh := h.npc("chi_hanh")
	for _, tc := range []struct{ body, want, not string }{{"man", "Chào anh tài xế", "chị"}, {"woman", "Chào chị tài xế", "anh tài"}} {
		x, z := h.near(hanh, 1.5, 0)
		p, _ := h.join(Arrival{Name: "Tài", Arch: "driver", Character: "char-" + tc.body, Start: &protocol.Pose{X: x, Z: z},
			Look: protocol.Look{Body: tc.body, Outfit: "crab", Hat: "helmet"}})
		h.drain(p)
		h.interact(p, hanh, 3)
		fs, _ := h.drain(p)
		in := only(t, fs, protocol.TypeInteraction)
		if !strings.Contains(in.Say.VI, tc.want) || strings.Contains(in.Say.VI, tc.not) || strings.Contains(in.Say.EN, "{") {
			t.Errorf("%s: %+v", tc.body, in.Say)
		}
	}
}
