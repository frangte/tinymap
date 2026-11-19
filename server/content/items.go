package content

import (
	_ "embed"
	"errors"
	"fmt"
)

// Items and the shops that sell them (items.json, shared with the game, which shows the shops and what a player
// has; docs/design/11-items.md). The server keeps what each character owns by item id, so ids are stable. Fees are
// the other thing money buys here: paid each time, never owned (docs/design/13-driver.md).

//go:embed items.json
var itemsJSON []byte

// Item kinds.
const (
	KindPhone   = "phone"
	KindBike    = "bike"
	KindUniform = "uniform"
)

var itemKinds = map[string]bool{KindPhone: true, KindBike: true, KindUniform: true}

// HaggleMax is the biggest discount a bargain may give (percent).
const HaggleMax = 50

// Haggle lets a shop's customer bargain: the dialogue signal Signal (said to the shop's NPC) takes Percent off the
// next purchase in that shop, for the rest of the visit (docs/design/13-driver.md).
type Haggle struct {
	Signal  string `json:"signal"`
	Percent int    `json:"percent"`
}

// Shop is a place that sells items, run by an NPC: a player must stand within that NPC's reach to buy.
type Shop struct {
	ID     string  `json:"id"`
	NPC    string  `json:"npc"`
	Name   Text    `json:"name"`
	Haggle *Haggle `json:"haggle,omitempty"`
}

// Item is something a character can buy once.
type Item struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Shop string `json:"shop"`
	// Price is in whole xu (shown as đồng)
	Price int64 `json:"price"`
	// Grants are the capabilities (archetypes.json) the owner has from then on
	Grants []string `json:"grants"`
	// Requires are capabilities the buyer must have already (the uniform wants a licence, a bike and a smartphone)
	Requires []string `json:"requires,omitempty"`
	// Vehicle is the vehicle (vehicles.json) a bike parks beside its owner; only for kind bike
	Vehicle string `json:"vehicle,omitempty"`
	Name    Text   `json:"name"`
	About   Text   `json:"about"`
}

// Fee is money paid to an NPC each time, for something that is not kept (the licence test). Paying grants the
// capabilities in Grants (a ticket the NPC's dialogue and the test module spend later); RequiresCaps must be held
// and UnlessCaps not held, which the server checks.
type Fee struct {
	ID           string   `json:"id"`
	NPC          string   `json:"npc"`
	Amount       int64    `json:"amount"`
	Grants       []string `json:"grants"`
	RequiresCaps []string `json:"requires_caps,omitempty"`
	UnlessCaps   []string `json:"unless_caps,omitempty"`
	Name         Text     `json:"name"`
	About        Text     `json:"about"`
}

// ItemPriceMax keeps a price within what a wallet can hold in a few days of play.
const ItemPriceMax = 100_000_000

func parseItems(f files, c *Catalog) error {
	var i struct {
		About string `json:"about"`
		Shops []Shop `json:"shops"`
		Fees  []Fee  `json:"fees"`
		Items []Item `json:"items"`
	}
	if err := decodeStrict(f.items, &i); err != nil {
		return fmt.Errorf("items.json: %w", err)
	}
	c.Shops, c.Fees, c.Items = i.Shops, i.Fees, i.Items
	c.shopByID, c.itemByID, c.feeByID = map[string]Shop{}, map[string]Item{}, map[string]Fee{}
	return nil
}

// checkItems runs after the capabilities and the cast are checked.
func (c *Catalog) checkItems(caps map[string]bool) error {
	for _, s := range c.Shops {
		if err := c.checkShop(s); err != nil {
			return fmt.Errorf("items.json: shop %q: %w", s.ID, err)
		}
		c.shopByID[s.ID] = s
	}
	for _, it := range c.Items {
		if err := c.checkItem(it, caps); err != nil {
			return fmt.Errorf("items.json: item %q: %w", it.ID, err)
		}
		c.itemByID[it.ID] = it
	}
	for _, f := range c.Fees {
		if err := c.checkFee(f, caps); err != nil {
			return fmt.Errorf("items.json: fee %q: %w", f.ID, err)
		}
		c.feeByID[f.ID] = f
	}
	return nil
}

func (c *Catalog) checkFee(f Fee, caps map[string]bool) error {
	switch {
	case !idPattern.MatchString(f.ID):
		return errors.New("id must be lower_snake_case ascii")
	case c.feeByID[f.ID].ID != "":
		return errors.New("duplicate id")
	case f.Amount <= 0 || f.Amount > ItemPriceMax:
		return fmt.Errorf("amount must be 1 to %d xu", ItemPriceMax)
	case !bilingual(f.Name) || !bilingual(f.About):
		return errors.New("needs a name and an about in Vietnamese and English")
	}
	if _, ok := c.NPCByID(f.NPC); !ok {
		return fmt.Errorf("npc %q is not in npcs.json", f.NPC)
	}
	for _, list := range [][]string{f.Grants, f.RequiresCaps, f.UnlessCaps} {
		for _, k := range list {
			if !caps[k] {
				return fmt.Errorf("names unknown capability %q", k)
			}
		}
	}
	return nil
}

func (c *Catalog) checkShop(s Shop) error {
	switch {
	case !idPattern.MatchString(s.ID):
		return errors.New("id must be lower_snake_case ascii")
	case c.shopByID[s.ID].ID != "":
		return errors.New("duplicate id")
	case !bilingual(s.Name):
		return errors.New("needs a name in Vietnamese and English")
	}
	if _, ok := c.NPCByID(s.NPC); !ok {
		return fmt.Errorf("npc %q is not in npcs.json", s.NPC)
	}
	if h := s.Haggle; h != nil && (!signalPattern.MatchString(h.Signal) || h.Percent < 1 || h.Percent > HaggleMax) {
		return fmt.Errorf("haggle needs a signal and a percent in 1 to %d", HaggleMax)
	}
	return nil
}

func (c *Catalog) checkItem(it Item, caps map[string]bool) error {
	switch {
	case !idPattern.MatchString(it.ID):
		return errors.New("id must be lower_snake_case ascii")
	case c.itemByID[it.ID].ID != "":
		return errors.New("duplicate id")
	case !itemKinds[it.Kind]:
		return fmt.Errorf("unknown kind %q", it.Kind)
	case c.shopByID[it.Shop].ID == "":
		return fmt.Errorf("shop %q is not in items.json", it.Shop)
	case it.Price <= 0 || it.Price > ItemPriceMax:
		return fmt.Errorf("price must be 1 to %d xu", ItemPriceMax)
	case !bilingual(it.Name) || !bilingual(it.About):
		return errors.New("needs a name and an about in Vietnamese and English")
	case (it.Kind == KindBike) != (it.Vehicle != ""):
		return errors.New("a bike names its vehicle, and nothing else does")
	case it.Vehicle != "" && c.vehicleByID[it.Vehicle].ID == "":
		return fmt.Errorf("vehicle %q is not in vehicles.json", it.Vehicle)
	}
	seen := map[string]bool{}
	for _, k := range it.Grants {
		if !caps[k] {
			return fmt.Errorf("grants unknown capability %q", k)
		}
		if seen[k] {
			return fmt.Errorf("grants %q twice", k)
		}
		seen[k] = true
	}
	for _, k := range it.Requires {
		if !caps[k] {
			return fmt.Errorf("requires unknown capability %q", k)
		}
	}
	return nil
}

// Item finds an item by id.
func (c *Catalog) Item(id string) (Item, bool) {
	it, ok := c.itemByID[id]
	return it, ok
}

// Fee finds a fee by id.
func (c *Catalog) Fee(id string) (Fee, bool) {
	f, ok := c.feeByID[id]
	return f, ok
}

// ItemKindKnown says whether kind is a kind of item.
func ItemKindKnown(kind string) bool { return itemKinds[kind] }

// ItemsOfKind is how many items there are of the kind.
func (c *Catalog) ItemsOfKind(kind string) int {
	n := 0
	for _, it := range c.Items {
		if it.Kind == kind {
			n++
		}
	}
	return n
}

// Shop finds a shop by id.
func (c *Catalog) Shop(id string) (Shop, bool) {
	s, ok := c.shopByID[id]
	return s, ok
}

// ShopItems are the items a shop sells, in file order.
func (c *Catalog) ShopItems(shop string) []Item {
	var out []Item
	for _, it := range c.Items {
		if it.Shop == shop {
			out = append(out, it)
		}
	}
	return out
}

// OwnedBike is the bike in hand of a character that owns these items: the last bike in file order, so a better
// bike added later replaces an older one. False for none.
func (c *Catalog) OwnedBike(owned []string) (Item, Vehicle, bool) {
	var best Item
	rank := -1
	for i, it := range c.Items {
		if it.Kind != KindBike {
			continue
		}
		for _, id := range owned {
			if id == it.ID {
				best, rank = it, i
			}
		}
	}
	if rank < 0 {
		return Item{}, Vehicle{}, false
	}
	return best, c.vehicleByID[best.Vehicle], true
}
