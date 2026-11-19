package shop_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// ---- fees, bargains and prerequisites (docs/design/13-driver.md)

// goTo stands the player a metre from an NPC.
func (f *fixture) goTo(npc string) {
	f.t.Helper()
	n, ok := f.c.NPCByID(npc)
	if !ok {
		f.t.Fatalf("no npc %q", npc)
	}
	x, z, _ := n.ThreePose()
	f.h.members[lan].pose = protocol.Pose{X: x + 1, Z: z}
}

func (f *fixture) can(caps ...string) {
	for _, k := range caps {
		f.h.members[lan].caps[k] = struct{}{}
	}
}

func (f *fixture) pay(fee string, rid uint32) {
	f.t.Helper()
	f.h.inbox = map[uint32][][]byte{}
	f.h.events = nil
	if !f.s.Handle(f.h.members[lan], protocol.ClientMsg{T: protocol.TypePay, Fee: fee, Rid: rid}) {
		f.t.Fatal("pay not handled")
	}
	f.h.settle()
}

func (f *fixture) balance() int64 {
	f.t.Helper()
	// the wallet exists and was topped up today: opening it again only reads the balance
	b, err := f.db.ArriveWallet(context.Background(), "charLan", 0, 0, "2026-11-19", 9)
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

func TestPayingAFeeTakesTheMoneyGivesTheTicketAndTellsTheQuests(t *testing.T) {
	f := newFixture(t, 500_000)
	f.goTo("thay_muoi")
	f.pay("thi_a1", 4)
	if got := f.frames(); !slices.Equal(got, []string{protocol.TypeWallet, protocol.TypeResult}) {
		t.Fatalf("frames %v: want wallet then result", got)
	}
	if r := f.result(); !r.OK || r.Rid != 4 {
		t.Fatalf("result %+v", r)
	}
	if b := f.balance(); b != 300_000 {
		t.Fatalf("balance %d, want 300000", b)
	}
	if !slices.Equal(f.granted, []string{content.CapExamTicket}) {
		t.Fatalf("granted %v: want the ticket", f.granted)
	}
	if len(f.h.events) != 1 || f.h.events[0].Kind != events.FeePaid || f.h.events[0].Fee != "thi_a1" {
		t.Fatalf("events %+v", f.h.events)
	}
	// the ticket is held: paying again is refused and nothing is charged
	f.pay("thi_a1", 5)
	f.refused(protocol.ResOwned)
	if b := f.balance(); b != 300_000 {
		t.Fatalf("a second payment charged: %d", b)
	}
}

func TestAFeeIsRefusedWhenItsPrerequisitesOrTheMoneyOrTheDistanceAreWrong(t *testing.T) {
	cases := []struct {
		name    string
		balance int64
		caps    []string
		fee     string
		at      string
		code    string
	}{
		{"not enough money", 100_000, nil, "thi_a1", "thay_muoi", protocol.ResBalance},
		{"too far from the examiner", 500_000, nil, "thi_a1", "anh_bay", protocol.ResFar},
		{"a retake before any attempt", 500_000, nil, "thi_lai_a1", "thay_muoi", protocol.ResLocked},
		{"the first fee after an attempt", 500_000, []string{content.CapExamRetake}, "thi_a1", "thay_muoi", protocol.ResOwned},
		{"a fee for a licence already held", 500_000, []string{content.CapRide}, "thi_a1", "thay_muoi", protocol.ResOwned},
		{"a retake with a ticket in hand", 500_000, []string{content.CapExamRetake, content.CapExamTicket}, "thi_lai_a1", "thay_muoi", protocol.ResOwned},
		{"a fee that does not exist", 500_000, nil, "thi_b2", "thay_muoi", protocol.ResTarget},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, c.balance)
			f.goTo(c.at)
			f.can(c.caps...)
			f.pay(c.fee, 1)
			f.refused(c.code)
			if b := f.balance(); b != c.balance {
				t.Fatalf("a refused fee charged: %d", b)
			}
			if len(f.granted) != 0 {
				t.Fatalf("a refused fee granted %v", f.granted)
			}
		})
	}
}

func TestARetakeCostsLessAndNeedsAnAttempt(t *testing.T) {
	f := newFixture(t, 500_000)
	f.goTo("thay_muoi")
	f.can(content.CapExamRetake)
	f.pay("thi_lai_a1", 1)
	if r := f.result(); !r.OK {
		t.Fatalf("result %+v", r)
	}
	if b := f.balance(); b != 450_000 {
		t.Fatalf("balance %d, want 450000", b)
	}
}

func TestAGuestHasNoWalletToPayFrom(t *testing.T) {
	f := newFixture(t, 500_000)
	f.goTo("thay_muoi")
	f.h.members[lan].char = ""
	f.pay("thi_a1", 1)
	f.refused(protocol.ResBalance)
}

func (f *fixture) deal() (protocol.Deal, bool) {
	for _, b := range f.h.inbox[lan] {
		var d protocol.Deal
		if err := json.Unmarshal(b, &d); err == nil && d.T == protocol.TypeDeal {
			return d, true
		}
	}
	return protocol.Deal{}, false
}

func TestBargainingTakesTenPercentOffAndOnlyOnTheServer(t *testing.T) {
	f := newFixture(t, 2_000_000)
	f.goTo("chu_tam")
	// a signal from another NPC, or another signal, starts no bargain
	f.h.inbox = map[uint32][][]byte{}
	f.s.Signalled(f.h.members[lan], "co_ba", "phrase_mac_qua")
	f.s.Signalled(f.h.members[lan], "chu_tam", "phrase_xin_chao")
	if _, ok := f.deal(); ok {
		t.Fatal("a bargain started without the phrase said to Chú Tám")
	}
	f.s.Signalled(f.h.members[lan], "chu_tam", "phrase_mac_qua")
	if d, ok := f.deal(); !ok || d.Shop != "xe_chu_tam" || d.Percent != 10 {
		t.Fatalf("deal %+v, %v", d, ok)
	}
	f.buy("xe_so_110", 3)
	if r := f.result(); !r.OK {
		t.Fatalf("result %+v", r)
	}
	if b := f.balance(); b != 1_100_000 {
		t.Fatalf("balance %d: the bike cost 900000 with the bargain, 1000000 without", b)
	}
	if d, ok := f.deal(); !ok || d.Percent != 0 {
		t.Fatalf("the bargain should end with the purchase: %+v, %v", d, ok)
	}
	// the bike stands beside its owner, ready for the licence
	if len(f.h.parked) != 1 || f.h.parked[0].Kind != "xe_so_110" || f.h.parked[0].Requires != content.CapRide {
		t.Fatalf("parked %+v", f.h.parked)
	}
	if !slices.Contains(f.granted, "vehicle.motorbike.own") {
		t.Fatalf("granted %v", f.granted)
	}
}

func TestWithoutBargainingTheBikeCostsTheTag(t *testing.T) {
	f := newFixture(t, 2_000_000)
	f.goTo("chu_tam")
	f.buy("xe_so_110", 3)
	if b := f.balance(); b != 1_000_000 {
		t.Fatalf("balance %d, want 1000000", b)
	}
}

func TestABargainEndsWhenThePlayerLeaves(t *testing.T) {
	f := newFixture(t, 2_000_000)
	f.goTo("chu_tam")
	f.s.Signalled(f.h.members[lan], "chu_tam", "phrase_mac_qua")
	f.s.Left(f.h.members[lan])
	f.buy("xe_so_110", 3)
	if b := f.balance(); b != 1_000_000 {
		t.Fatalf("balance %d: a bargain must not outlive the visit", b)
	}
}

func TestTheUniformWaitsForTheSmartphoneTheLicenceAndTheBike(t *testing.T) {
	need := []string{"phone.smart", content.CapRide, "vehicle.motorbike.own"}
	for i := range need {
		t.Run(fmt.Sprintf("without %s", need[i]), func(t *testing.T) {
			f := newFixture(t, 2_000_000)
			f.goTo("chi_vy")
			f.can(slices.Delete(slices.Clone(need), i, i+1)...)
			f.buy("dong_phuc_crab", 1)
			f.refused(protocol.ResLocked)
			if len(f.owned()) != 0 || f.balance() != 2_000_000 {
				t.Fatalf("a locked purchase kept something: %v", f.owned())
			}
		})
	}
	f := newFixture(t, 2_000_000)
	f.goTo("chi_vy")
	f.can(need...)
	f.buy("dong_phuc_crab", 1)
	if r := f.result(); !r.OK {
		t.Fatalf("result %+v", r)
	}
	want := capability.Of("uniform.crab", "driver.app", "ridehail.drive")
	for k := range want {
		if !slices.Contains(f.granted, k) {
			t.Errorf("the uniform did not grant %s (granted %v)", k, f.granted)
		}
	}
	if f.balance() != 1_700_000 {
		t.Fatalf("balance %d", f.balance())
	}
}
