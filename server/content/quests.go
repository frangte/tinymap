package content

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Quests and achievements (quests.json, shared with the game, which shows their text). This file checks their
// shape and every reference between files; the quest engine (server/internal/quest) checks each objective's
// params when it compiles them, and its tests run that over this file.

//go:embed quests.json
var questsJSON []byte

// Title is a name players can carry under their own, earned as a reward.
type Title struct {
	ID   string `json:"id"`
	Name Text   `json:"name"`
}

// Stamp is a collectible in the Traveler's stamp book (Sổ tem), earned as a reward.
type Stamp struct {
	ID   string `json:"id"`
	Name Text   `json:"name"`
}

// Objective is one thing a quest asks for. Type names an objective type of the quest engine, which reads Params.
type Objective struct {
	// ID is stable within its quest: progress is stored by it (achievements have one objective and no id)
	ID     string          `json:"id,omitempty"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
	// Count is how many times it must happen (0 means once): distinct places, NPCs or players where that makes
	// sense, seconds for stay_place, metres for ride_distance
	Count int  `json:"count,omitempty"`
	Text  Text `json:"text,omitempty"`
	// At is where the quest tracker points: a place or an NPC id ("" for nowhere in particular)
	At string `json:"at,omitempty"`
}

// Times is Count, at least 1.
func (o Objective) Times() int { return max(1, o.Count) }

// Rewards are what finishing a quest or an achievement gives.
type Rewards struct {
	Titles     []string `json:"titles,omitempty"`
	Stamps     []string `json:"stamps,omitempty"`
	Capability string   `json:"capability,omitempty"`
}

// Flows: in what order a quest's objectives can be done.
const (
	FlowSequence = "sequence"
	FlowAll      = "all"
	FlowAny      = "any"
)

type Quest struct {
	ID string `json:"id"`
	// Version goes up when objectives change meaning; progress records which version it was made under
	Version   int    `json:"version"`
	Questline string `json:"questline"`
	Title     Text   `json:"title"`
	About     Text   `json:"about"`
	// TagsAny: offered to characters whose archetype has one of these tags (archetypes.json); empty: everyone
	TagsAny []string `json:"tags_any,omitempty"`
	// Requires are quests to have completed first
	Requires   []string    `json:"requires,omitempty"`
	Flow       string      `json:"flow"`
	Objectives []Objective `json:"objectives"`
	Rewards    Rewards     `json:"rewards"`
}

// Achievement is a passive quest: one objective, always running for those it is open to.
type Achievement struct {
	ID        string    `json:"id"`
	Title     Text      `json:"title"`
	About     Text      `json:"about"`
	Hidden    bool      `json:"hidden,omitempty"`
	TagsAny   []string  `json:"tags_any,omitempty"`
	Objective Objective `json:"objective"`
	Rewards   Rewards   `json:"rewards"`
}

const (
	// TitleMax keeps a title readable under a name tag (characters)
	TitleMax      = 32
	objectivesMax = 8
	textMax       = 200
)

var (
	questPattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*(\.[a-z0-9]+(_[a-z0-9]+)*)*$`)
	typePattern  = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)
	flows        = map[string]bool{FlowSequence: true, FlowAll: true, FlowAny: true}
)

func parseQuests(f files, c *Catalog) error {
	var q struct {
		About        string        `json:"about"`
		Titles       []Title       `json:"titles"`
		Stamps       []Stamp       `json:"stamps"`
		Quests       []Quest       `json:"quests"`
		Achievements []Achievement `json:"achievements"`
	}
	if err := decodeStrict(f.quests, &q); err != nil {
		return fmt.Errorf("quests.json: %w", err)
	}
	c.Titles, c.Stamps, c.Quests, c.Achievements = q.Titles, q.Stamps, q.Quests, q.Achievements
	// quest ids are known before the dialogue is checked: an answer can require a quest
	c.questByID = map[string]Quest{}
	for _, qu := range c.Quests {
		if _, dup := c.questByID[qu.ID]; dup {
			return fmt.Errorf("quests.json: quest %q: duplicate id", qu.ID)
		}
		c.questByID[qu.ID] = qu
	}
	return nil
}

// checkQuests runs last: it needs the places, NPCs, capabilities and archetype tags.
func (c *Catalog) checkQuests(caps, places map[string]bool) error {
	refs := questRefs{c: c, caps: caps, places: places, npcs: map[string]bool{}, tags: map[string]bool{}}
	for _, n := range c.NPCs {
		refs.npcs[n.ID] = true
	}
	for _, a := range c.Archetypes {
		for _, t := range a.Tags {
			refs.tags[t] = true
		}
	}
	refs.titles, refs.stamps = map[string]bool{}, map[string]bool{}
	for _, t := range c.Titles {
		if !idPattern.MatchString(t.ID) || refs.titles[t.ID] {
			return fmt.Errorf("quests.json: title %q: needs a unique lower_snake_case id", t.ID)
		}
		if err := checkText(t.Name, TitleMax, false); err != nil {
			return fmt.Errorf("quests.json: title %q: %w", t.ID, err)
		}
		refs.titles[t.ID] = true
	}
	for _, s := range c.Stamps {
		if !idPattern.MatchString(s.ID) || refs.stamps[s.ID] {
			return fmt.Errorf("quests.json: stamp %q: needs a unique lower_snake_case id", s.ID)
		}
		if err := checkText(s.Name, TitleMax, false); err != nil {
			return fmt.Errorf("quests.json: stamp %q: %w", s.ID, err)
		}
		refs.stamps[s.ID] = true
	}
	for _, q := range c.Quests {
		if err := refs.quest(q); err != nil {
			return fmt.Errorf("quests.json: quest %q: %w", q.ID, err)
		}
	}
	if err := c.noRequireCycles(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, a := range c.Achievements {
		if err := refs.achievement(a, seen); err != nil {
			return fmt.Errorf("quests.json: achievement %q: %w", a.ID, err)
		}
		seen[a.ID] = true
	}
	return nil
}

type questRefs struct {
	c                                        *Catalog
	caps, places, npcs, tags, titles, stamps map[string]bool
}

func (r questRefs) quest(q Quest) error {
	switch {
	case !questPattern.MatchString(q.ID):
		return errors.New("id must be lower_snake_case words joined by dots")
	case q.Version < 1:
		return errors.New("version must be 1 or more")
	case !idPattern.MatchString(q.Questline):
		return errors.New("needs a lower_snake_case questline")
	case !flows[q.Flow]:
		return fmt.Errorf("flow must be %s, %s or %s", FlowSequence, FlowAll, FlowAny)
	case len(q.Objectives) == 0 || len(q.Objectives) > objectivesMax:
		return fmt.Errorf("needs 1 to %d objectives", objectivesMax)
	}
	if err := r.texts(q.Title, q.About); err != nil {
		return err
	}
	if err := r.tagList(q.TagsAny); err != nil {
		return err
	}
	for _, id := range q.Requires {
		if _, ok := r.c.questByID[id]; !ok || id == q.ID {
			return fmt.Errorf("requires %q, which is not another quest", id)
		}
	}
	ids := map[string]bool{}
	for _, o := range q.Objectives {
		if !idPattern.MatchString(o.ID) || ids[o.ID] {
			return fmt.Errorf("objective %q: needs an id unique in the quest", o.ID)
		}
		ids[o.ID] = true
		if err := r.objective(o, true); err != nil {
			return fmt.Errorf("objective %q: %w", o.ID, err)
		}
	}
	return r.rewards(q.Rewards)
}

func (r questRefs) achievement(a Achievement, seen map[string]bool) error {
	switch {
	case !idPattern.MatchString(a.ID) || seen[a.ID]:
		return errors.New("needs a unique lower_snake_case id")
	case a.Objective.ID != "":
		return errors.New("its objective has no id")
	}
	if err := r.texts(a.Title, a.About); err != nil {
		return err
	}
	if err := r.tagList(a.TagsAny); err != nil {
		return err
	}
	if err := r.objective(a.Objective, false); err != nil {
		return fmt.Errorf("objective: %w", err)
	}
	return r.rewards(a.Rewards)
}

func (r questRefs) texts(title, about Text) error {
	if err := checkText(title, TitleMax*2, false); err != nil {
		return fmt.Errorf("title: %w", err)
	}
	if err := checkText(about, textMax, false); err != nil {
		return fmt.Errorf("about: %w", err)
	}
	return nil
}

func (r questRefs) tagList(tags []string) error {
	for _, t := range tags {
		if !r.tags[t] {
			return fmt.Errorf("tag %q is no archetype's (archetypes.json)", t)
		}
	}
	return nil
}

func (r questRefs) objective(o Objective, needsText bool) error {
	switch {
	case !typePattern.MatchString(o.Type):
		return errors.New("needs a type")
	case len(o.Params) == 0 || o.Params[0] != '{':
		return errors.New("params must be an object ({} for none)")
	case o.Count < 0:
		return errors.New("count must not be negative")
	case o.At != "" && !r.places[o.At] && !r.npcs[o.At]:
		return fmt.Errorf("at %q is neither a place nor an NPC", o.At)
	}
	if needsText || o.Text.VI != "" {
		if err := checkText(o.Text, textMax, false); err != nil {
			return fmt.Errorf("text: %w", err)
		}
	}
	return nil
}

func (r questRefs) rewards(w Rewards) error {
	for _, t := range w.Titles {
		if !r.titles[t] {
			return fmt.Errorf("reward title %q is not in titles", t)
		}
	}
	for _, s := range w.Stamps {
		if !r.stamps[s] {
			return fmt.Errorf("reward stamp %q is not in stamps", s)
		}
	}
	if w.Capability != "" && !r.caps[w.Capability] {
		return fmt.Errorf("reward capability %q is unknown (archetypes.json)", w.Capability)
	}
	return nil
}

// noRequireCycles makes sure every quest can become available: requires never loop.
func (c *Catalog) noRequireCycles() error {
	const (
		unseen = iota
		visiting
		done
	)
	state := map[string]int{}
	var visit func(id string, path []string) error
	visit = func(id string, path []string) error {
		switch state[id] {
		case visiting:
			return fmt.Errorf("quests.json: quests require each other in a loop: %v", append(path, id))
		case done:
			return nil
		}
		state[id] = visiting
		for _, r := range c.questByID[id].Requires {
			if err := visit(r, append(path, id)); err != nil {
				return err
			}
		}
		state[id] = done
		return nil
	}
	for _, q := range c.Quests {
		if err := visit(q.ID, nil); err != nil {
			return err
		}
	}
	return nil
}

// Quest finds a quest by id.
func (c *Catalog) Quest(id string) (Quest, bool) {
	q, ok := c.questByID[id]
	return q, ok
}

// TagsOf is the tags of an archetype (none for "" or an unknown id).
func (c *Catalog) TagsOf(archetype string) []string { return c.byID[archetype].Tags }

// Place finds a place by id.
func (c *Catalog) Place(id string) (Place, bool) {
	for _, p := range c.Places {
		if p.ID == id {
			return p, true
		}
	}
	return Place{}, false
}

// NPCByID finds an NPC by id.
func (c *Catalog) NPCByID(id string) (NPC, bool) {
	for _, n := range c.NPCs {
		if n.ID == id {
			return n, true
		}
	}
	return NPC{}, false
}

// Emits reports whether some answer of the NPC's dialogue sends the signal.
func (c *Catalog) Emits(npc, signal string) bool {
	n, ok := c.NPCByID(npc)
	if !ok {
		return false
	}
	for _, node := range c.Dialogues[n.Dialogue].Nodes {
		for _, ch := range node.Choices {
			if ch.Emit == signal {
				return true
			}
		}
	}
	return false
}

// TitleByID finds a title by id.
func (c *Catalog) TitleByID(id string) (Title, bool) {
	for _, t := range c.Titles {
		if t.ID == id {
			return t, true
		}
	}
	return Title{}, false
}
