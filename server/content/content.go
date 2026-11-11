// Package content is the game data the server and the client share, embedded in the server binary.
// places.json is also bundled into the game by Vite (src/game/places.ts), so both sides read one list;
// the tests here are what keeps that file valid.
package content

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
)

//go:embed places.json
var placesJSON []byte

type PlaceKind string

const (
	Landmark PlaceKind = "landmark"
	Street   PlaceKind = "street"
	Area     PlaceKind = "area"
)

// Place is a named spot on the island, in Blender XY metres (three.js x = X, z = -Y).
type Place struct {
	// ID is stable lower_snake_case; quests refer to places by it
	ID string `json:"id"`
	// Name and Note are in Vietnamese and English; the game shows the one in the player's language
	Name Text      `json:"name"`
	Note Text      `json:"note"`
	Kind PlaceKind `json:"kind"`
	// Pts is one point, or a polyline for streets and the coast
	Pts [][]float64 `json:"pts"`
	// R is the reach in metres from the point or the line
	R float64 `json:"r"`
}

// coordLimit bounds Blender X and Y; the island spans about -85…130 m, the server's world bounds are ±400.
const coordLimit = 400

var idPattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// Places parses and checks the embedded places.json.
func Places() ([]Place, error) { return parsePlaces(placesJSON) }

func parsePlaces(data []byte) ([]Place, error) {
	var file struct {
		About  string  `json:"about"`
		Places []Place `json:"places"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("places.json: %w", err)
	}
	if len(file.Places) == 0 {
		return nil, errors.New("places.json: no places")
	}
	seen := make(map[string]bool, len(file.Places))
	for i, p := range file.Places {
		if err := checkPlace(p, seen); err != nil {
			return nil, fmt.Errorf("places.json: place %d (%q): %w", i, p.ID, err)
		}
		seen[p.ID] = true
	}
	return file.Places, nil
}

func checkPlace(p Place, seen map[string]bool) error {
	switch {
	case !idPattern.MatchString(p.ID):
		return errors.New("id must be lower_snake_case ascii")
	case seen[p.ID]:
		return errors.New("duplicate id")
	case p.Name.VI == "" || p.Name.EN == "":
		return errors.New("no name in Vietnamese and English")
	case (p.Note.VI == "") != (p.Note.EN == ""):
		return errors.New("note needs both Vietnamese and English")
	case p.Kind != Landmark && p.Kind != Street && p.Kind != Area:
		return fmt.Errorf("unknown kind %q", p.Kind)
	case len(p.Pts) == 0:
		return errors.New("no points")
	case !(p.R > 0):
		return errors.New("reach must be positive")
	}
	for _, pt := range p.Pts {
		if len(pt) != 2 || math.Abs(pt[0]) > coordLimit || math.Abs(pt[1]) > coordLimit {
			return fmt.Errorf("point %v must be [x, y] within ±%d m", pt, coordLimit)
		}
	}
	return nil
}
