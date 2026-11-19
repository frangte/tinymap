package content

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"
)

//go:embed tips.json
var tipsJSON []byte

// Trigger kinds of a tip: which situation in play shows it (the game's src/game/tips.ts wires each one).
const (
	TipPlaceKind     = "place_kind"
	TipPlace         = "place"
	TipNPC           = "npc"
	TipCrabBooked    = "crab_booked"
	TipFarePaid      = "fare_paid"
	TipPhoneInStreet = "phone_in_street"
)

// TipTrigger is the situation that shows a tip; each kind takes the one parameter it names and no other.
type TipTrigger struct {
	Kind      string `json:"kind"`
	PlaceKind string `json:"place_kind,omitempty"`
	Place     string `json:"place,omitempty"`
	NPC       string `json:"npc,omitempty"`
}

// Tip is one travel tip: a line shown the first time its situation happens, then kept in the Sổ mẹo.
type Tip struct {
	ID      string     `json:"id"`
	Title   Text       `json:"title"`
	Text    Text       `json:"text"`
	Trigger TipTrigger `json:"trigger"`
}

// Tips is tips.json. The game reads the file itself; the server only checks it.
type Tips struct {
	About string `json:"about"`
	Tips  []Tip  `json:"tips"`
}

var tipID = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

var placeKinds = map[string]bool{"landmark": true, "street": true, "area": true, "district": true}

// LoadTips reads and checks tips.json against the places and NPCs of the catalog.
func LoadTips() (Tips, error) {
	c, err := Load()
	if err != nil {
		return Tips{}, err
	}
	places := make(map[string]bool, len(c.Places))
	for _, p := range c.Places {
		places[p.ID] = true
	}
	npcs := make(map[string]bool, len(c.NPCs))
	for _, n := range c.NPCs {
		npcs[n.ID] = true
	}
	return parseTips(tipsJSON, places, npcs)
}

func parseTips(data []byte, places, npcs map[string]bool) (Tips, error) {
	var t Tips
	if err := decodeStrict(data, &t); err != nil {
		return t, fmt.Errorf("tips.json: %w", err)
	}
	if len(t.Tips) == 0 {
		return t, errors.New("tips.json: no tips")
	}
	seen := map[string]bool{}
	for i, tip := range t.Tips {
		if err := tip.check(places, npcs, seen); err != nil {
			return t, fmt.Errorf("tips.json: tip %d (%q): %w", i, tip.ID, err)
		}
		seen[tip.ID] = true
	}
	return t, nil
}

func (t Tip) check(places, npcs, seen map[string]bool) error {
	switch {
	case !tipID.MatchString(t.ID):
		return errors.New("id must be lower_snake_case")
	case seen[t.ID]:
		return errors.New("a second tip with the same id")
	case !bilingual(t.Title):
		return errors.New("title needs Vietnamese and English")
	case !bilingual(t.Text):
		return errors.New("text needs Vietnamese and English")
	}
	return t.Trigger.check(places, npcs)
}

func (g TipTrigger) check(places, npcs map[string]bool) error {
	var want string
	switch g.Kind {
	case TipPlaceKind:
		if !placeKinds[g.PlaceKind] {
			return fmt.Errorf("place_kind %q is not landmark, street, area or district", g.PlaceKind)
		}
		want = "place_kind"
	case TipPlace:
		if !places[g.Place] {
			return fmt.Errorf("unknown place %q", g.Place)
		}
		want = "place"
	case TipNPC:
		if !npcs[g.NPC] {
			return fmt.Errorf("unknown npc %q", g.NPC)
		}
		want = "npc"
	case TipCrabBooked, TipFarePaid, TipPhoneInStreet:
	default:
		return fmt.Errorf("unknown trigger kind %q", g.Kind)
	}
	given := map[string]bool{"place_kind": g.PlaceKind != "", "place": g.Place != "", "npc": g.NPC != ""}
	for name, set := range given {
		if set && name != want {
			return fmt.Errorf("trigger %s takes no %s", g.Kind, name)
		}
	}
	return nil
}
