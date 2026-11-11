package content

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"

	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

//go:embed archetypes.json
var archetypesJSON []byte

//go:embed apps.json
var appsJSON []byte

// A restricted part that a character may not wear becomes one of these (both are open to everyone).
const (
	fallbackOutfit = "thun"
	fallbackHat    = "none"
)

// Text is a string in each language the game speaks; Vietnamese is required. It is the protocol's Text, so
// content can go out on the wire as it is.
type Text = protocol.Text

// bilingual: the text has both its Vietnamese and its English (the game shows the player's language).
func bilingual(t Text) bool { return t.VI != "" && t.EN != "" }

type Capability struct {
	ID    string `json:"id"`
	About string `json:"about"`
}

// Badge is how an archetype shows next to a name: a short label on a colour.
type Badge struct {
	// Short is the label, in Vietnamese and English
	Short Text   `json:"short"`
	Color string `json:"color"`
	Text  string `json:"text"`
}

// Spawn is where an archetype starts, in Blender XY metres, facing Face.
type Spawn struct {
	Place  string    `json:"place"`
	At     []float64 `json:"at"`
	Face   []float64 `json:"face"`
	Button Text      `json:"button"`
}

// ThreePose is the spawn in three.js terms: x and z (Blender (X, Y) is three.js (x, -y)) and the heading that
// faces Face (0 faces +z), as src/game/play.ts spawns it.
func (s Spawn) ThreePose() (x, z, heading float64) { return threePose(s.At, s.Face) }

// threePose turns a Blender XY point and the point it faces into three.js x, z and a heading (0 faces +z).
func threePose(at, face []float64) (x, z, heading float64) {
	x, z = at[0], -at[1]
	return x, z, math.Atan2(face[0]-at[0], -(face[1] - at[1]))
}

type StarterLook struct {
	Outfit string `json:"outfit"`
	Hat    string `json:"hat"`
}

type Archetype struct {
	ID string `json:"id"`
	// Tags say which quests it is offered (quests.json tags_any)
	Tags  []string    `json:"tags"`
	Name  Text        `json:"name"`
	Pitch Text        `json:"pitch"`
	Badge Badge       `json:"badge"`
	Spawn Spawn       `json:"spawn"`
	Look  StarterLook `json:"look"`
	// Vehicle is the starter vehicle (vehicles.json), "" for none
	Vehicle      string   `json:"vehicle,omitempty"`
	Capabilities []string `json:"capabilities"`
}

// RestrictedLook is an outfit or a hat that only characters with a capability may wear.
type RestrictedLook struct {
	Outfit   string `json:"outfit,omitempty"`
	Hat      string `json:"hat,omitempty"`
	Requires string `json:"requires"`
}

// App is a phone app; Requires lists the capabilities a character needs to have it.
type App struct {
	ID       string   `json:"id"`
	Name     Text     `json:"name"`
	Icon     string   `json:"icon"`
	Requires []string `json:"requires,omitempty"`
}

// Catalog is all the game data, every reference between the files checked.
type Catalog struct {
	Places       []Place
	Capabilities []Capability
	Archetypes   []Archetype
	Restricted   []RestrictedLook
	Apps         []App
	Movement     Movement
	Vehicles     []Vehicle
	Emotes       []Emote
	NPCs         []NPC
	Dialogues    map[string]Dialogue
	Titles       []Title
	Stamps       []Stamp
	Quests       []Quest
	Achievements []Achievement
	Ridehail     Ridehail
	Directions   Directions
	byID         map[string]Archetype
	vehicleByID  map[string]Vehicle
	questByID    map[string]Quest
}

var (
	capPattern   = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)+$`)
	colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

// files are the data files a catalog is read from; embedded() is what ships.
type files struct {
	places, archetypes, apps, movement, vehicles, emotes, npcs, quests, ridehail, directions []byte
	// dialogues are the files of dialogue/, by name
	dialogues map[string][]byte
}

func embedded() files {
	return files{places: placesJSON, archetypes: archetypesJSON, apps: appsJSON, movement: movementJSON, vehicles: vehiclesJSON,
		emotes: emotesJSON, npcs: npcsJSON, quests: questsJSON, ridehail: ridehailJSON, directions: directionsJSON,
		dialogues: dialogueFiles()}
}

// Load parses and checks the embedded data files.
func Load() (*Catalog, error) { return load(embedded()) }

func load(f files) (*Catalog, error) {
	c := &Catalog{byID: map[string]Archetype{}}
	var err error
	if c.Places, err = parsePlaces(f.places); err != nil {
		return nil, err
	}
	var a struct {
		About        string           `json:"about"`
		Capabilities []Capability     `json:"capabilities"`
		Archetypes   []Archetype      `json:"archetypes"`
		Restricted   []RestrictedLook `json:"restricted_looks"`
	}
	if err := decodeStrict(f.archetypes, &a); err != nil {
		return nil, fmt.Errorf("archetypes.json: %w", err)
	}
	var p struct {
		About string `json:"about"`
		Apps  []App  `json:"apps"`
	}
	if err := decodeStrict(f.apps, &p); err != nil {
		return nil, fmt.Errorf("apps.json: %w", err)
	}
	if err := parseMotion(f, c); err != nil {
		return nil, err
	}
	if err := parseQuests(f, c); err != nil {
		return nil, err
	}
	if err := parseRidehail(f, c); err != nil {
		return nil, err
	}
	if err := parseDirections(f, c); err != nil {
		return nil, err
	}
	if err := parseCast(f, c); err != nil {
		return nil, err
	}
	c.Capabilities, c.Archetypes, c.Restricted, c.Apps = a.Capabilities, a.Archetypes, a.Restricted, p.Apps
	if err := c.check(); err != nil {
		return nil, err
	}
	return c, nil
}

func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (c *Catalog) check() error {
	caps := map[string]bool{}
	for _, k := range c.Capabilities {
		if !capPattern.MatchString(k.ID) || caps[k.ID] || k.About == "" {
			return fmt.Errorf("archetypes.json: capability %q: needs a unique dotted id (area.what) and an about", k.ID)
		}
		caps[k.ID] = true
	}
	places := map[string]bool{}
	for _, pl := range c.Places {
		places[pl.ID] = true
	}
	if err := c.checkMotion(caps); err != nil {
		return err
	}
	for _, r := range c.Restricted {
		if err := checkRestricted(r, caps); err != nil {
			return fmt.Errorf("archetypes.json: restricted look %+v: %w", r, err)
		}
	}
	if len(c.Archetypes) == 0 {
		return errors.New("archetypes.json: no archetypes")
	}
	for _, a := range c.Archetypes {
		if err := c.checkArchetype(a, caps, places); err != nil {
			return fmt.Errorf("archetypes.json: archetype %q: %w", a.ID, err)
		}
		c.byID[a.ID] = a
	}
	seen := map[string]bool{}
	for _, app := range c.Apps {
		if !idPattern.MatchString(app.ID) || seen[app.ID] || app.Name.VI == "" || app.Name.EN == "" || app.Icon == "" {
			return fmt.Errorf("apps.json: app %q: needs a unique lower_snake_case id, a Vietnamese name and an icon", app.ID)
		}
		for _, r := range app.Requires {
			if !caps[r] {
				return fmt.Errorf("apps.json: app %q requires unknown capability %q", app.ID, r)
			}
		}
		seen[app.ID] = true
	}
	for _, fallback := range []protocol.Look{{Outfit: fallbackOutfit}, {Hat: fallbackHat}} {
		if c.restrictionOf(fallback) != "" {
			return errors.New("archetypes.json: the fallback outfit and hat must not be restricted")
		}
	}
	if err := c.checkCast(caps, places); err != nil {
		return err
	}
	if err := c.Ridehail.check(); err != nil {
		return fmt.Errorf("ridehail.json: %w", err)
	}
	if err := c.Directions.check(caps); err != nil {
		return fmt.Errorf("directions.json: %w", err)
	}
	return c.checkQuests(caps, places)
}

func checkRestricted(r RestrictedLook, caps map[string]bool) error {
	switch {
	case (r.Outfit == "") == (r.Hat == ""):
		return errors.New("name exactly one of outfit or hat")
	case r.Outfit != "" && !protocol.OutfitKnown(r.Outfit):
		return errors.New("unknown outfit")
	case r.Hat != "" && !protocol.HatKnown(r.Hat):
		return errors.New("unknown hat")
	case !caps[r.Requires]:
		return fmt.Errorf("unknown capability %q", r.Requires)
	}
	return nil
}

func (c *Catalog) checkArchetype(a Archetype, caps, places map[string]bool) error {
	switch {
	case !idPattern.MatchString(a.ID):
		return errors.New("id must be lower_snake_case ascii")
	case c.byID[a.ID].ID != "":
		return errors.New("duplicate id")
	case len(a.Tags) == 0:
		return errors.New("needs at least one tag (quests are offered by tag)")
	case !bilingual(a.Name) || !bilingual(a.Pitch) || !bilingual(a.Spawn.Button):
		return errors.New("needs a name, pitch and spawn button in Vietnamese and English")
	case !bilingual(a.Badge.Short) || !colorPattern.MatchString(a.Badge.Color) || !colorPattern.MatchString(a.Badge.Text):
		return errors.New("badge needs a short label and #rrggbb colours")
	case !places[a.Spawn.Place]:
		return fmt.Errorf("spawn place %q is not in places.json", a.Spawn.Place)
	case !point(a.Spawn.At) || !point(a.Spawn.Face):
		return fmt.Errorf("spawn at and face must be [x, y] within ±%d m", coordLimit)
	case !protocol.OutfitKnown(a.Look.Outfit) || !protocol.HatKnown(a.Look.Hat):
		return errors.New("unknown starter outfit or hat")
	case a.Vehicle != "" && c.vehicleByID[a.Vehicle].ID == "":
		return fmt.Errorf("starter vehicle %q is not in vehicles.json", a.Vehicle)
	}
	for _, k := range a.Capabilities {
		if !caps[k] {
			return fmt.Errorf("unknown capability %q", k)
		}
	}
	for _, t := range a.Tags {
		if !idPattern.MatchString(t) {
			return fmt.Errorf("tag %q must be lower_snake_case ascii", t)
		}
	}
	granted := capability.Of(a.Capabilities...)
	start := protocol.Look{Body: "man", Outfit: a.Look.Outfit, Hat: a.Look.Hat}
	if !c.LookAllowed(start, granted) {
		return errors.New("its starter look needs a capability it does not grant")
	}
	if v := c.vehicleByID[a.Vehicle]; a.Vehicle != "" && !granted.Has(v.Requires) {
		return fmt.Errorf("its starter vehicle needs %q, which it does not grant", v.Requires)
	}
	return nil
}

func point(p []float64) bool {
	return len(p) == 2 && math.Abs(p[0]) <= coordLimit && math.Abs(p[1]) <= coordLimit
}

// Archetype finds an archetype by id; false for "" and unknown ids.
func (c *Catalog) Archetype(id string) (Archetype, bool) {
	a, ok := c.byID[id]
	return a, ok
}

// Grants are the capabilities an archetype gives (none for "" or an unknown id).
func (c *Catalog) Grants(archetype string) capability.Set {
	a, ok := c.byID[archetype]
	if !ok {
		return capability.Of()
	}
	return capability.Of(a.Capabilities...)
}

// AppsFor lists the ids of the phone apps a character with caps has, in phone order.
func (c *Catalog) AppsFor(caps capability.Set) []string {
	ids := make([]string, 0, len(c.Apps))
	for _, a := range c.Apps {
		if caps.HasAll(a.Requires) {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

// restrictionOf is the capability the look's outfit or hat needs ("" when it needs none).
func (c *Catalog) restrictionOf(look protocol.Look) string {
	for _, r := range c.Restricted {
		if (r.Outfit != "" && r.Outfit == look.Outfit) || (r.Hat != "" && r.Hat == look.Hat) {
			return r.Requires
		}
	}
	return ""
}

// LookAllowed reports whether a character with caps may wear the look.
func (c *Catalog) LookAllowed(look protocol.Look, caps capability.Set) bool {
	for _, r := range c.Restricted {
		if ((r.Outfit != "" && r.Outfit == look.Outfit) || (r.Hat != "" && r.Hat == look.Hat)) && !caps.Has(r.Requires) {
			return false
		}
	}
	return true
}

// AllowedLook is the look with every part the character may not wear swapped for an open one.
func (c *Catalog) AllowedLook(look protocol.Look, caps capability.Set) protocol.Look {
	for _, r := range c.Restricted {
		if caps.Has(r.Requires) {
			continue
		}
		if r.Outfit != "" && r.Outfit == look.Outfit {
			look.Outfit = fallbackOutfit
		}
		if r.Hat != "" && r.Hat == look.Hat {
			look.Hat = fallbackHat
		}
	}
	return look
}
