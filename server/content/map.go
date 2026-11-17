package content

import (
	_ "embed"
	"errors"
	"fmt"
	"math"
)

// map.json is the island as a whole; the game reads the same file (src/game/mapInfo.ts).
//
//go:embed map.json
var mapJSON []byte

// Map is the island as a whole, in Blender XY metres (three.js x = X, z = -Y): where a player without an archetype
// starts, the sea, the frame of the island. Walls, floors and steps are not here: the game collides with the map
// itself (src/game/collider.ts), and the server takes its word for where its player stands; it checks speeds only.
type Map struct {
	About string `json:"about"`
	// Spawn is where a player without an archetype starts, on the square in front of Chợ Bến Thành
	Spawn []float64 `json:"spawn"`
	// SeaLevel is the height (Blender Z) the sea is drawn at
	SeaLevel float64 `json:"sea_level"`
	// IslandCenter is where the gulls and clouds circle
	IslandCenter []float64 `json:"island_center"`
	// Bounds frame the island and its beach: [x0, y0, x1, y1]
	Bounds []float64 `json:"bounds"`
}

// maxSeaLevel bounds the sea's height (m): a typo of 45 for -0.45 should fail the tests
const maxSeaLevel = 10

func parseMap(data []byte, c *Catalog) error {
	if err := decodeStrict(data, &c.Map); err != nil {
		return fmt.Errorf("map.json: %w", err)
	}
	m := c.Map
	b := m.Bounds
	switch {
	case len(b) != 4 || !(b[0] < b[2] && b[1] < b[3]) || math.Abs(b[0]) > coordLimit || math.Abs(b[1]) > coordLimit ||
		math.Abs(b[2]) > coordLimit || math.Abs(b[3]) > coordLimit:
		return fmt.Errorf("map.json: bounds must be [x0, y0, x1, y1] with x0 < x1 and y0 < y1, within ±%d m", coordLimit)
	case len(m.Spawn) != 2 || !m.Inside(m.Spawn[0], m.Spawn[1]):
		return errors.New("map.json: spawn must be [x, y] inside the bounds")
	case len(m.IslandCenter) != 2 || !m.Inside(m.IslandCenter[0], m.IslandCenter[1]):
		return errors.New("map.json: island_center must be [x, y] inside the bounds")
	case !(math.Abs(m.SeaLevel) <= maxSeaLevel):
		return fmt.Errorf("map.json: sea_level must be within ±%d m", maxSeaLevel)
	}
	return nil
}

// Inside reports whether Blender (x, y) lies within the island's bounds.
func (m Map) Inside(x, y float64) bool {
	b := m.Bounds
	return len(b) == 4 && x >= b[0] && x <= b[2] && y >= b[1] && y <= b[3]
}

// ThreeSpawn is the spawn in three.js terms (x, z).
func (m Map) ThreeSpawn() (x, z float64) { return m.Spawn[0], -m.Spawn[1] }
