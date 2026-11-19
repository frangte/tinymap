package content

import (
	_ "embed"
	"errors"
	"fmt"
)

// The A1 licence test (driving.json, docs/design/13-driver.md): the NPC who runs it, the school's bike lent for the
// practical and the route of checkpoints to ride within the time. Shared with the game, which draws the checkpoints
// from what the server sends and never from this file.

//go:embed driving.json
var drivingJSON []byte

// The capabilities of the test (archetypes.json): paying the fee gives a ticket, a theory attempt spends it, and
// passing the theory gives a ticket for the practical and the theory pass itself.
const (
	CapExamTicket = "exam.a1.ticket"
	CapExamRetake = "exam.a1.retake"
	CapExamTheory = "exam.a1.theory"
	// CapRide is the licence itself
	CapRide = "vehicle.motorbike.ride"
	// TestA1 is the test's id in the event TestPassed and the objective pass_test
	TestA1 = "a1"
)

// The signals the test's NPC sends (dialogue/thay_muoi.json) that the driving module acts on.
const (
	SignalTheoryStart = "thi_bat_dau"
	SignalTheoryPass  = "ly_thuyet_dau"
	SignalPracticalGo = "thuc_hanh_bat_dau"
)

// Checkpoint is a place to ride through: Blender XY metres and a radius.
type Checkpoint struct {
	At []float64 `json:"at"`
	R  float64   `json:"r"`
}

// ThreeXZ is the checkpoint's centre in three.js terms.
func (c Checkpoint) ThreeXZ() (x, z float64) { return c.At[0], -c.At[1] }

// Driving is the licence test's setup.
type Driving struct {
	About string `json:"about"`
	// NPC runs the test (npcs.json); Vehicle is the school's bike (vehicles.json)
	NPC     string `json:"npc"`
	Vehicle string `json:"vehicle"`
	// TimeLimit is how long the route may take once the player is on the bike (s); MountWithin how long the
	// lent bike waits to be got on (s)
	TimeLimit   float64      `json:"time_limit"`
	MountWithin float64      `json:"mount_within"`
	Checkpoints []Checkpoint `json:"checkpoints"`
}

func parseDriving(f files, c *Catalog) error {
	if err := decodeStrict(f.driving, &c.Driving); err != nil {
		return fmt.Errorf("driving.json: %w", err)
	}
	return nil
}

func (c *Catalog) checkDriving(caps map[string]bool) error {
	d := c.Driving
	if d.NPC == "" && len(d.Checkpoints) == 0 {
		return nil // a catalog without a licence test (small test casts)
	}
	for _, k := range []string{CapExamTicket, CapExamRetake, CapExamTheory, CapRide} {
		if !caps[k] {
			return fmt.Errorf("archetypes.json: the licence test needs the capability %q", k)
		}
	}
	switch {
	case func() bool { _, ok := c.NPCByID(d.NPC); return !ok }():
		return fmt.Errorf("driving.json: npc %q is not in npcs.json", d.NPC)
	case c.vehicleByID[d.Vehicle].ID == "":
		return fmt.Errorf("driving.json: vehicle %q is not in vehicles.json", d.Vehicle)
	case c.vehicleByID[d.Vehicle].Requires != CapExamTheory:
		return fmt.Errorf("driving.json: the school's bike must require %q", CapExamTheory)
	case !(d.TimeLimit >= 30 && d.TimeLimit <= 600) || !(d.MountWithin >= 10 && d.MountWithin <= 300):
		return errors.New("driving.json: time_limit is 30 to 600 s and mount_within 10 to 300 s")
	case len(d.Checkpoints) < 2 || len(d.Checkpoints) > 16:
		return errors.New("driving.json: needs 2 to 16 checkpoints")
	}
	for i, cp := range d.Checkpoints {
		if !point(cp.At) || !(cp.R >= 2 && cp.R <= 15) || !c.Map.Inside(cp.At[0], cp.At[1]) {
			return fmt.Errorf("driving.json: checkpoint %d needs a point on the island and a radius of 2 to 15 m", i)
		}
	}
	return nil
}
