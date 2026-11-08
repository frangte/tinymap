package content

import (
	_ "embed"
	"errors"
	"fmt"
)

//go:embed directions.json
var directionsJSON []byte

//go:embed chat_filter.json
var chatFilterJSON []byte

// ChatWords are the words the chat filter masks (chat_filter.json says how each list is matched).
type ChatWords struct {
	About    string   `json:"about"`
	Accented []string `json:"accented"`
	Plain    []string `json:"plain"`
}

// LoadChatWords reads the chat filter's lists (the server's chatfilter compiles them).
func LoadChatWords() (ChatWords, error) {
	var w ChatWords
	if err := decodeStrict(chatFilterJSON, &w); err != nil {
		return w, fmt.Errorf("chat_filter.json: %w", err)
	}
	for _, s := range w.Plain {
		for _, r := range s {
			if r >= 0x80 && r != 'đ' {
				return w, fmt.Errorf("chat_filter.json: plain %q has a mark: it goes in accented", s)
			}
		}
	}
	return w, nil
}

// Directions is asking the way (directions.json says what each is): seconds and metres.
type Directions struct {
	About         string  `json:"about"`
	WaitSeconds   int     `json:"wait_seconds"`
	Reach         float64 `json:"reach"`
	AskGapSeconds int     `json:"ask_gap_seconds"`
}

// Capabilities of asking the way (archetypes.json).
const (
	CapAskWay  = "directions.ask"
	CapGiveWay = "directions.give"
)

// CapModerate is a moderator's: chat commands to mute, ban, kick and read reports. No archetype gives it: a
// moderator account's characters get it on arrival (`sg-server mod add`).
const CapModerate = "moderation.act"

func parseDirections(f files, c *Catalog) error {
	if err := decodeStrict(f.directions, &c.Directions); err != nil {
		return fmt.Errorf("directions.json: %w", err)
	}
	return nil
}

func (d Directions) check(caps map[string]bool) error {
	switch {
	case d.WaitSeconds < 30 || d.WaitSeconds > 600:
		return errors.New("wait_seconds must be 30 to 600")
	case d.Reach < 1 || d.Reach > 10:
		return errors.New("reach must be 1 to 10 metres")
	case d.AskGapSeconds < 0 || d.AskGapSeconds > d.WaitSeconds:
		return errors.New("ask_gap_seconds must be 0 to wait_seconds")
	case !caps[CapAskWay] || !caps[CapGiveWay]:
		return fmt.Errorf("archetypes.json must declare %s and %s", CapAskWay, CapGiveWay)
	}
	return nil
}
