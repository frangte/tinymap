package shop

import (
	"context"
	"errors"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// Fees (docs/design/13-driver.md): money paid to an NPC each time for something that is not kept. The dialogue
// shows a line with a fee and the game offers a button; the button is the `pay` message, so a dialogue never moves
// money by itself. The server checks the player stands next to the fee's NPC, holds the capabilities the fee
// requires and none it is refused for, and that the wallet covers it; paying grants the fee's capabilities (a
// ticket) and sends the quest event `fee.paid`.

// feeSpot is where a fee's NPC stands.
func (s *Service) feeSpot(f content.Fee) (spot, bool) {
	n, ok := s.catalog.NPCByID(f.NPC)
	if !ok {
		return spot{}, false
	}
	x, z, _ := n.ThreePose()
	return spot{x: x, z: z, reach: n.Reach}, true
}

func (s *Service) onPay(m world.Member, msg protocol.ClientMsg) {
	fee, known := s.catalog.Fee(msg.Fee)
	switch {
	case m.Character() == "":
		s.result(m.ID(), msg.Rid, protocol.ResBalance)
		return
	case !known:
		s.result(m.ID(), msg.Rid, protocol.ResTarget)
		return
	}
	if sp, ok := s.feeSpot(fee); !ok || !within(m.Pose(), sp) {
		s.result(m.ID(), msg.Rid, protocol.ResFar)
		return
	}
	caps := m.Caps()
	switch {
	case anyOf(caps, fee.UnlessCaps):
		s.result(m.ID(), msg.Rid, protocol.ResOwned)
		return
	case !caps.HasAll(fee.RequiresCaps):
		s.result(m.ID(), msg.Rid, protocol.ResLocked)
		return
	}
	id, rid, character := m.ID(), msg.Rid, m.Character()
	p := Payment{Character: character, Fee: fee.ID, Amount: fee.Amount, Grants: fee.Grants, At: s.h.Now().UnixMilli()}
	var balance int64
	s.h.Do(character, func(ctx context.Context) error {
		var err error
		if balance, err = s.bank.Pay(ctx, p); err != nil {
			return err
		}
		if s.OnCapability != nil {
			for _, k := range fee.Grants {
				s.OnCapability(character, k)
			}
		}
		return nil
	}, func(err error) { s.paid(id, rid, fee, p, balance, err) })
}

// paid answers a payment, back on the world goroutine.
func (s *Service) paid(id, rid uint32, fee content.Fee, p Payment, balance int64, err error) {
	switch {
	case errors.Is(err, ErrOwned):
		s.result(id, rid, protocol.ResOwned)
		return
	case errors.Is(err, ErrBalance):
		s.result(id, rid, protocol.ResBalance)
		return
	case err != nil:
		s.log.Error("fee failed", "character", p.Character, "fee", p.Fee, "err", err)
		s.result(id, rid, protocol.ResBusy)
		return
	}
	s.h.Publish(events.Event{Kind: events.FeePaid, Character: p.Character, Fee: fee.ID})
	m, ok := s.h.Member(id)
	if !ok || m.Character() != p.Character {
		return
	}
	s.h.Send(id, protocol.Encode(protocol.Wallet{T: protocol.TypeWallet, Balance: balance}))
	s.result(id, rid, "")
}

func within(p protocol.Pose, sp spot) bool {
	dx, dz := p.X-sp.x, p.Z-sp.z
	return dx*dx+dz*dz <= sp.reach*sp.reach
}

func anyOf(caps interface{ Has(string) bool }, ids []string) bool {
	for _, id := range ids {
		if caps.Has(id) {
			return true
		}
	}
	return false
}
