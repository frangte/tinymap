// Package shop is buying items from an NPC's shop (docs/design/11-items.md): a player standing next to the NPC
// sends `buy`, the price comes out of the wallet and the item and its capabilities are kept, in one database
// transaction off the world loop. On success the player hears its new balance, every item it owns and the phone
// apps it has now, and the quest engine hears an `item.bought` event. A world module: the checks run on the world
// goroutine, the money moves in a job (world.Host.Do), and the answer comes back to the loop.
package shop

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// spot is where a shop's NPC stands and how close a player must be to buy, the same reach that starts a
// conversation with it.
type spot struct{ x, z, reach float64 }

// Service is the shop module.
type Service struct {
	h       world.Host
	catalog *content.Catalog
	bank    Bank
	log     *slog.Logger
	// OnCapability, when set, hears every capability a purchase gave (the connection updates what the character may
	// do); it is called from the job that wrote the purchase, before the player is answered. Set it before the
	// world runs.
	OnCapability func(character, capability string)
	spots        map[string]spot // by shop id
	// deals are the bargains in force, by player then shop: the percent off the next purchase there. They end with
	// a purchase or when the player leaves; a client never names a price.
	deals map[uint32]map[string]int
}

// New makes the module; hand it to world.Use.
func New(h world.Host, c *content.Catalog, bank Bank, log *slog.Logger) *Service {
	s := &Service{h: h, catalog: c, bank: bank, log: log, spots: map[string]spot{}, deals: map[uint32]map[string]int{}}
	for _, sh := range c.Shops {
		if n, ok := c.NPCByID(sh.NPC); ok {
			x, z, _ := n.ThreePose()
			s.spots[sh.ID] = spot{x: x, z: z, reach: n.Reach}
		}
	}
	return s
}

func (s *Service) Handle(m world.Member, msg protocol.ClientMsg) bool {
	switch msg.T {
	case protocol.TypeBuy:
		s.onBuy(m, msg)
	case protocol.TypePay:
		s.onPay(m, msg)
	default:
		return false
	}
	return true
}

func (s *Service) Tick(time.Time) {}

func (s *Service) Left(m world.Member) { delete(s.deals, m.ID()) }

// Signalled starts a bargain: the signal a shop's haggle names, said to the shop's NPC, takes a percent off the
// player's next purchase there (docs/design/13-driver.md).
func (s *Service) Signalled(m world.Member, npc, signal string) {
	for _, sh := range s.catalog.Shops {
		h := sh.Haggle
		if h == nil || sh.NPC != npc || h.Signal != signal {
			continue
		}
		if s.deals[m.ID()] == nil {
			s.deals[m.ID()] = map[string]int{}
		}
		s.deals[m.ID()][sh.ID] = h.Percent
		s.h.Send(m.ID(), protocol.Encode(protocol.Deal{T: protocol.TypeDeal, Shop: sh.ID, Percent: h.Percent}))
	}
}

// priceFor is what the player pays for the item now: its price less the bargain in force in its shop.
func (s *Service) priceFor(m world.Member, item content.Item) int64 {
	pct := int64(s.deals[m.ID()][item.Shop])
	return item.Price - item.Price*pct/100
}

func (s *Service) result(id, rid uint32, code string) {
	if rid == 0 {
		return
	}
	s.h.Send(id, protocol.Encode(protocol.Result{T: protocol.TypeResult, Rid: rid, OK: code == "", Code: code}))
}

func (s *Service) onBuy(m world.Member, msg protocol.ClientMsg) {
	item, known := s.catalog.Item(msg.Item)
	switch {
	case m.Character() == "":
		// a guest has no wallet to pay from
		s.result(m.ID(), msg.Rid, protocol.ResBalance)
		return
	case !known:
		s.result(m.ID(), msg.Rid, protocol.ResTarget)
		return
	case !s.near(m, item.Shop):
		s.result(m.ID(), msg.Rid, protocol.ResFar)
		return
	case !m.Caps().HasAll(item.Requires):
		s.result(m.ID(), msg.Rid, protocol.ResLocked)
		return
	}
	id, rid, character := m.ID(), msg.Rid, m.Character()
	p := Purchase{Character: character, Item: item.ID, Price: s.priceFor(m, item), Grants: item.Grants, At: s.h.Now().UnixMilli()}
	var balance int64
	var owned []string
	s.h.Do(character, func(ctx context.Context) error {
		var err error
		if balance, err = s.bank.Buy(ctx, p); err != nil {
			return err
		}
		// what the connection may do follows at once: these reach the world before the answer below does
		if s.OnCapability != nil {
			for _, k := range item.Grants {
				s.OnCapability(character, k)
			}
		}
		if owned, err = s.bank.Owned(ctx, character); err != nil {
			// the purchase stands; the player gets the item it just bought and the rest on its next visit
			s.log.Warn("owned items after a purchase", "character", character, "err", err)
			owned = []string{item.ID}
		}
		return nil
	}, func(err error) { s.bought(id, rid, item, p, balance, owned, err) })
}

// near: the player stands within reach of the shop's NPC.
func (s *Service) near(m world.Member, shop string) bool {
	sp, ok := s.spots[shop]
	if !ok {
		return false
	}
	pose := m.Pose()
	dx, dz := pose.X-sp.x, pose.Z-sp.z
	return dx*dx+dz*dz <= sp.reach*sp.reach
}

// bought answers a purchase, back on the world goroutine.
func (s *Service) bought(id, rid uint32, item content.Item, p Purchase, balance int64, owned []string, err error) {
	switch {
	case errors.Is(err, ErrOwned):
		s.result(id, rid, protocol.ResOwned)
		return
	case errors.Is(err, ErrBalance):
		s.result(id, rid, protocol.ResBalance)
		return
	case err != nil:
		s.log.Error("purchase failed", "character", p.Character, "item", p.Item, "err", err)
		s.result(id, rid, protocol.ResBusy)
		return
	}
	// the quest hears of it whether or not the player is still here to be told
	s.h.Publish(events.Event{Kind: events.ItemBought, Character: p.Character, Item: item.ID, ItemKind: item.Kind})
	m, ok := s.h.Member(id)
	if !ok || m.Character() != p.Character {
		return
	}
	if _, bargained := s.deals[id][item.Shop]; bargained {
		delete(s.deals[id], item.Shop)
		s.h.Send(id, protocol.Encode(protocol.Deal{T: protocol.TypeDeal, Shop: item.Shop}))
	}
	if v, ok := s.catalog.Vehicle(item.Vehicle); ok && item.Kind == content.KindBike {
		// the bike it bought stands beside it now, and on every arrival after
		s.h.Park(id, world.VehicleSpec{Kind: v.ID, Speed: v.Speed, Requires: v.Requires})
	}
	s.h.Send(id, protocol.Encode(protocol.Wallet{T: protocol.TypeWallet, Balance: balance}))
	s.h.Send(id, protocol.Encode(protocol.Items{T: protocol.TypeItems, Items: owned, Apps: s.catalog.AppsFor(s.capsWith(m, owned))}))
	s.result(id, rid, "")
}

// capsWith is what the player may do: its capabilities and those of every item it owns (the connection has
// normally added the new ones already; this keeps the answer right if it has not yet).
func (s *Service) capsWith(m world.Member, owned []string) capability.Set {
	caps := capability.Of()
	for k := range m.Caps() {
		caps[k] = struct{}{}
	}
	for _, id := range owned {
		if it, ok := s.catalog.Item(id); ok {
			for _, k := range it.Grants {
				caps[k] = struct{}{}
			}
		}
	}
	return caps
}
