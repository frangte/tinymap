package content

import (
	_ "embed"
	"errors"
	"fmt"

	"saigon-noir/server/internal/capability"
)

//go:embed movement.json
var movementJSON []byte

//go:embed vehicles.json
var vehiclesJSON []byte

//go:embed emotes.json
var emotesJSON []byte

// Movement is how fast everyone moves on foot (m/s, m/s²), and how much faster the server lets a reported move be.
type Movement struct {
	About     string  `json:"about"`
	WalkSpeed float64 `json:"walk_speed"`
	RunSpeed  float64 `json:"run_speed"`
	JumpSpeed float64 `json:"jump_speed"`
	Gravity   float64 `json:"gravity"`
	Tolerance float64 `json:"tolerance"`
}

// Vehicle is a kind of vehicle. Seat 0 is the rider, who needs Requires.
type Vehicle struct {
	ID       string  `json:"id"`
	Name     Text    `json:"name"`
	Speed    float64 `json:"speed"`
	Seats    int     `json:"seats"`
	Requires string  `json:"requires"`
}

// Emote is a gesture everyone sees. Code is its number in the state flags (1–15).
type Emote struct {
	ID       string  `json:"id"`
	Code     int     `json:"code"`
	Name     Text    `json:"name"`
	Seconds  float64 `json:"seconds"`
	Requires string  `json:"requires,omitempty"`
}

// MaxEmoteCode is the highest code that fits the four bits of the state flags.
const MaxEmoteCode = 15

// sane bounds for speeds (m/s): a typo of 110 for 11 should fail the tests, not reach players
const maxSpeed = 30

// maxEmoteSeconds bounds an emote's length: a gesture, not a state
const maxEmoteSeconds = 30

func parseMotion(f files, c *Catalog) error {
	if err := decodeStrict(f.movement, &c.Movement); err != nil {
		return fmt.Errorf("movement.json: %w", err)
	}
	var v struct {
		About    string    `json:"about"`
		Vehicles []Vehicle `json:"vehicles"`
	}
	if err := decodeStrict(f.vehicles, &v); err != nil {
		return fmt.Errorf("vehicles.json: %w", err)
	}
	var e struct {
		About  string  `json:"about"`
		Emotes []Emote `json:"emotes"`
	}
	if err := decodeStrict(f.emotes, &e); err != nil {
		return fmt.Errorf("emotes.json: %w", err)
	}
	c.Vehicles, c.Emotes = v.Vehicles, e.Emotes
	return nil
}

func (c *Catalog) checkMotion(caps map[string]bool) error {
	m := c.Movement
	if !(m.WalkSpeed > 0 && m.WalkSpeed <= m.RunSpeed && m.RunSpeed <= maxSpeed) || !(m.JumpSpeed > 0) || !(m.Gravity > 0) ||
		!(m.Tolerance >= 1 && m.Tolerance <= 2) {
		return errors.New("movement.json: needs 0 < walk ≤ run ≤ 30 m/s, a jump speed and gravity, and a tolerance in [1, 2]")
	}
	c.vehicleByID = map[string]Vehicle{}
	for _, v := range c.Vehicles {
		switch {
		case !idPattern.MatchString(v.ID) || c.vehicleByID[v.ID].ID != "":
			return fmt.Errorf("vehicles.json: vehicle %q: needs a unique lower_snake_case id", v.ID)
		case !bilingual(v.Name):
			return fmt.Errorf("vehicles.json: vehicle %q: needs a name in Vietnamese and English", v.ID)
		case !(v.Speed > 0 && v.Speed <= maxSpeed) || v.Seats < 1:
			return fmt.Errorf("vehicles.json: vehicle %q: needs a speed in (0, 30] m/s and at least one seat", v.ID)
		case !caps[v.Requires]:
			return fmt.Errorf("vehicles.json: vehicle %q requires unknown capability %q", v.ID, v.Requires)
		}
		c.vehicleByID[v.ID] = v
	}
	ids, codes := map[string]bool{}, map[int]bool{}
	for _, e := range c.Emotes {
		switch {
		case !idPattern.MatchString(e.ID) || ids[e.ID]:
			return fmt.Errorf("emotes.json: emote %q: needs a unique lower_snake_case id", e.ID)
		case e.Code < 1 || e.Code > MaxEmoteCode || codes[e.Code]:
			return fmt.Errorf("emotes.json: emote %q: code must be unique and in 1…%d", e.ID, MaxEmoteCode)
		case !bilingual(e.Name) || !(e.Seconds >= 0 && e.Seconds <= maxEmoteSeconds):
			return fmt.Errorf("emotes.json: emote %q: needs a name in Vietnamese and English and seconds in [0, %d] (0 loops)", e.ID, maxEmoteSeconds)
		case e.Requires != "" && !caps[e.Requires]:
			return fmt.Errorf("emotes.json: emote %q requires unknown capability %q", e.ID, e.Requires)
		}
		ids[e.ID], codes[e.Code] = true, true
	}
	return nil
}

// Vehicle finds a kind of vehicle by id.
func (c *Catalog) Vehicle(id string) (Vehicle, bool) {
	v, ok := c.vehicleByID[id]
	return v, ok
}

// StarterVehicle is the vehicle an archetype parks next to its players, if they may ride it.
func (c *Catalog) StarterVehicle(archetype string, caps capability.Set) (Vehicle, bool) {
	a, ok := c.byID[archetype]
	if !ok || a.Vehicle == "" {
		return Vehicle{}, false
	}
	v, ok := c.vehicleByID[a.Vehicle]
	return v, ok && caps.Has(v.Requires)
}

// EmoteAllowed reports whether a character with caps may play the emote with this code (0, none, always may).
func (c *Catalog) EmoteAllowed(code int, caps capability.Set) bool {
	if code == 0 {
		return true
	}
	for _, e := range c.Emotes {
		if e.Code == code {
			return e.Requires == "" || caps.Has(e.Requires)
		}
	}
	return false
}
