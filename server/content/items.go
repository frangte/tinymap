package content

import (
	_ "embed"
	"errors"
	"fmt"
)

// Items and the shops that sell them (items.json, shared with the game, which shows the shops and what a player
// has; docs/design/11-items.md). The server keeps what each character owns by item id, so ids are stable.

//go:embed items.json
var itemsJSON []byte

// Item kinds. Only phones exist so far.
const KindPhone = "phone"

var itemKinds = map[string]bool{KindPhone: true}

// Shop is a place that sells items, run by an NPC: a player must stand within that NPC's reach to buy.
type Shop struct {
	ID   string `json:"id"`
	NPC  string `json:"npc"`
	Name Text   `json:"name"`
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
	Name   Text     `json:"name"`
	About  Text     `json:"about"`
}

// ItemPriceMax keeps a price within what a wallet can hold in a few days of play.
const ItemPriceMax = 100_000_000

func parseItems(f files, c *Catalog) error {
	var i struct {
		About string `json:"about"`
		Shops []Shop `json:"shops"`
		Items []Item `json:"items"`
	}
	if err := decodeStrict(f.items, &i); err != nil {
		return fmt.Errorf("items.json: %w", err)
	}
	c.Shops, c.Items = i.Shops, i.Items
	c.shopByID, c.itemByID = map[string]Shop{}, map[string]Item{}
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
	return nil
}

// Item finds an item by id.
func (c *Catalog) Item(id string) (Item, bool) {
	it, ok := c.itemByID[id]
	return it, ok
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
