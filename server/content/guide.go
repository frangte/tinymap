package content

import (
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"time"
)

//go:embed guide.json
var guideJSON []byte

// GuideFacts are the quick facts at the top of a place's page; each one is optional, but a page has at least one.
type GuideFacts struct {
	Built  *Text `json:"built,omitempty"`
	Hours  *Text `json:"hours,omitempty"`
	Ticket *Text `json:"ticket,omitempty"`
}

// GuideEntry is the Cẩm nang's page about one place (docs/design/10-story.md, section 10): the info card in the
// street and the page in the Sổ tay. The game reads guide.json itself (src/game/guide.ts); the server only checks
// it, so a page about a place that does not exist, or without its English, never ships.
type GuideEntry struct {
	// Place is a place id in places.json
	Place string `json:"place"`
	// Checked is the day (YYYY-MM-DD) the facts that change (hours, prices, works) were last checked
	Checked      string     `json:"checked"`
	Summary      Text       `json:"summary"`
	Facts        GuideFacts `json:"facts"`
	History      []Text     `json:"history"`
	See          []Text     `json:"see"`
	Tips         []Text     `json:"tips"`
	GettingThere []Text     `json:"getting_there"`
	Food         []Text     `json:"food"`
	// Maps is what to search for on Google Maps to find the real place
	Maps string `json:"maps"`
	// Sources are the pages the facts come from (https)
	Sources []string `json:"sources"`
}

func parseGuide(f files, c *Catalog) error {
	var g struct {
		About   string       `json:"about"`
		Entries []GuideEntry `json:"entries"`
	}
	if err := decodeStrict(f.guide, &g); err != nil {
		return fmt.Errorf("guide.json: %w", err)
	}
	places := make(map[string]bool, len(c.Places))
	for _, p := range c.Places {
		places[p.ID] = true
	}
	seen := map[string]bool{}
	for i, e := range g.Entries {
		if err := e.check(places, seen); err != nil {
			return fmt.Errorf("guide.json: entry %d (%q): %w", i, e.Place, err)
		}
		seen[e.Place] = true
	}
	c.Guide = g.Entries
	return nil
}

func (e GuideEntry) check(places, seen map[string]bool) error {
	switch {
	case !places[e.Place]:
		return errors.New("unknown place")
	case seen[e.Place]:
		return errors.New("a second page for the same place")
	case !validDay(e.Checked):
		return fmt.Errorf("checked %q is not a YYYY-MM-DD day", e.Checked)
	case !bilingual(e.Summary):
		return errors.New("summary needs Vietnamese and English")
	case e.Facts.Built == nil && e.Facts.Hours == nil && e.Facts.Ticket == nil:
		return errors.New("no quick facts")
	case len(e.History) == 0:
		return errors.New("no history")
	case e.Maps == "":
		return errors.New("no Google Maps search")
	case len(e.Sources) == 0:
		return errors.New("no sources")
	}
	for _, fact := range []*Text{e.Facts.Built, e.Facts.Hours, e.Facts.Ticket} {
		if fact != nil && !bilingual(*fact) {
			return errors.New("a quick fact needs Vietnamese and English")
		}
	}
	for name, list := range map[string][]Text{"history": e.History, "see": e.See, "tips": e.Tips, "getting_there": e.GettingThere, "food": e.Food} {
		for _, t := range list {
			if !bilingual(t) {
				return fmt.Errorf("%s: %q needs its English", name, t.VI)
			}
		}
	}
	for _, s := range e.Sources {
		if u, err := url.Parse(s); err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("source %q is not an https link", s)
		}
	}
	return nil
}

func validDay(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}
