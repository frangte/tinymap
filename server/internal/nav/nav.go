// Package nav is the walkable heightfield baked from the Blender diorama: the same grid the game walks on
// (src/game/nav.ts), 0.35 m cells over the island with a floor height and a walkable flag each. The server
// checks the moves clients report against it; nothing here raycasts the city.
//
// Coordinates are three.js metres (x, z), as on the wire: Blender (X, Y) is three.js (x, -z). Cell (i, j) covers
// x in [x0 + i·cell, …) and Blender Y in [y0 + j·cell, …); row 0 is the south edge and is the PNG's first row.
package nav

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

// Meta is <base>_nav.json, written by scripts/blender/bake_nav.py.
type Meta struct {
	// X0 and Y0 are the Blender X and Y of the grid's west and south edges
	X0   float64 `json:"x0"`
	Y0   float64 `json:"y0"`
	Cell float64 `json:"cell"`
	NX   int     `json:"nx"`
	NY   int     `json:"ny"`
	// Spawn is the default start, Blender XY (the square in front of the market's south gate)
	Spawn [2]float64 `json:"spawn"`
}

// Grid is the loaded nav map. It is never changed after Load, so any goroutine may read it.
type Grid struct {
	meta  Meta
	floor []float32
	walk  []bool
}

// maxCells bounds the grid the server accepts (the island is 640×651).
const maxCells = 4096 * 4096

// Load reads the nav PNG (R·256+G = floor height in cm + 1000, B > 127 = walkable) and its metadata.
func Load(pngData, metaJSON []byte) (*Grid, error) {
	var m Meta
	dec := json.NewDecoder(bytes.NewReader(metaJSON))
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("nav: metadata: %w", err)
	}
	if !(m.Cell > 0) || m.NX <= 0 || m.NY <= 0 || m.NX*m.NY > maxCells {
		return nil, fmt.Errorf("nav: metadata: bad grid %dx%d, cell %v", m.NX, m.NY, m.Cell)
	}
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("nav: map: %w", err)
	}
	b := img.Bounds()
	if b.Dx() != m.NX || b.Dy() != m.NY {
		return nil, fmt.Errorf("nav: map is %dx%d, metadata says %dx%d", b.Dx(), b.Dy(), m.NX, m.NY)
	}
	g := &Grid{meta: m, floor: make([]float32, m.NX*m.NY), walk: make([]bool, m.NX*m.NY)}
	for j := 0; j < m.NY; j++ {
		for i := 0; i < m.NX; i++ {
			c := pixel(img, b.Min.X+i, b.Min.Y+j)
			k := j*m.NX + i
			g.floor[k] = float32(int(c.R)*256+int(c.G))/100 - 10
			g.walk[k] = c.B > 127
		}
	}
	if !g.anyWalkable() {
		return nil, errors.New("nav: no walkable cell")
	}
	return g, nil
}

// pixel reads raw channel values: the map holds numbers, not colours, so nothing may be premultiplied.
func pixel(img image.Image, x, y int) color.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n.NRGBAAt(x, y)
	}
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func (g *Grid) anyWalkable() bool {
	for _, w := range g.walk {
		if w {
			return true
		}
	}
	return false
}

// Meta is the grid's metadata.
func (g *Grid) Meta() Meta { return g.meta }

// Spawn is the default start in three.js (x, z).
func (g *Grid) Spawn() (x, z float64) { return g.meta.Spawn[0], -g.meta.Spawn[1] }

// cell is the cell under three.js (x, z); ok is false outside the grid.
func (g *Grid) cell(x, z float64) (i, j int, ok bool) {
	i = int(math.Floor((x - g.meta.X0) / g.meta.Cell))
	j = int(math.Floor((-z - g.meta.Y0) / g.meta.Cell))
	return i, j, i >= 0 && j >= 0 && i < g.meta.NX && j < g.meta.NY
}

// Walkable reports whether the cell under (x, z) can be walked on (false outside the grid).
func (g *Grid) Walkable(x, z float64) bool {
	i, j, ok := g.cell(x, z)
	return ok && g.walk[j*g.meta.NX+i]
}

// Floor is the floor height of the cell under (x, z); ok is false outside the grid.
func (g *Grid) Floor(x, z float64) (y float64, ok bool) {
	i, j, ok := g.cell(x, z)
	if !ok {
		return 0, false
	}
	return float64(g.floor[j*g.meta.NX+i]), true
}

// FloorRange is the lowest and highest floor of the walkable cells in the 3×3 block around (x, z). The game
// blends the floor over neighbouring cells and eases toward it, so its height can sit anywhere in that range.
func (g *Grid) FloorRange(x, z float64) (lo, hi float64, ok bool) {
	ci, cj, _ := g.cell(x, z)
	lo, hi = math.Inf(1), math.Inf(-1)
	for j := cj - 1; j <= cj+1; j++ {
		for i := ci - 1; i <= ci+1; i++ {
			if i < 0 || j < 0 || i >= g.meta.NX || j >= g.meta.NY || !g.walk[j*g.meta.NX+i] {
				continue
			}
			f := float64(g.floor[j*g.meta.NX+i])
			lo, hi = math.Min(lo, f), math.Max(hi, f)
		}
	}
	return lo, hi, lo <= hi
}

// roundSlack covers the wire's centimetre rounding: a point the game accepted can land this far over a cell edge.
const roundSlack = 0.02

// Standable reports whether (x, z), give or take the wire's rounding, is on a walkable cell.
func (g *Grid) Standable(x, z float64) bool {
	if g.Walkable(x, z) {
		return true
	}
	for _, d := range [4][2]float64{{roundSlack, 0}, {-roundSlack, 0}, {0, roundSlack}, {0, -roundSlack}} {
		if g.Walkable(x+d[0], z+d[1]) {
			return true
		}
	}
	return false
}

// Fits reports whether a body of radius r fits at (x, z): its centre and eight points on its edge are walkable
// (the same footprint test as NavGrid.canStand in the game, without the step check).
func (g *Grid) Fits(x, z, r float64) bool {
	if !g.Walkable(x, z) {
		return false
	}
	for k := range 8 {
		a := float64(k) * math.Pi / 4
		if !g.Walkable(x+math.Cos(a)*r, z+math.Sin(a)*r) {
			return false
		}
	}
	return true
}

// NearestFit is the closest point to (x, z) where a body of radius r fits, searching rings outward up to
// maxDist metres; ok is false when there is none.
func (g *Grid) NearestFit(x, z, r, maxDist float64) (fx, fz float64, ok bool) {
	c := g.meta.Cell
	for d := 0.0; d <= maxDist; d += c {
		n := max(1, int(math.Round(2*math.Pi*d/c)))
		for k := range n {
			a := float64(k) / float64(n) * 2 * math.Pi
			px, pz := x+math.Cos(a)*d, z+math.Sin(a)*d
			if g.Fits(px, pz, r) {
				return px, pz, true
			}
		}
	}
	return 0, 0, false
}

// segmentStep is how finely Clear samples a line (m), well under a cell.
const segmentStep = 0.1

// Clear reports whether the straight line from a to b stays on walkable cells, except for blocked stretches of
// at most maxGap metres. Two reported positions are 0.1 s apart and the path between them is not straight: going
// round a corner, the line between them can clip it.
func (g *Grid) Clear(ax, az, bx, bz, maxGap float64) bool {
	length := math.Hypot(bx-ax, bz-az)
	n := int(math.Ceil(length / segmentStep))
	gap := 0.0
	for s := 0; s <= n; s++ {
		t := 1.0
		if n > 0 {
			t = float64(s) / float64(n)
		}
		if g.Standable(ax+(bx-ax)*t, az+(bz-az)*t) {
			gap = 0
			continue
		}
		if n > 0 {
			gap += length / float64(n)
		}
		if gap > maxGap {
			return false
		}
	}
	return true
}

// Climb is the largest floor change between two neighbouring walkable samples on the straight line from a to b
// (m). The game climbs a kerb in one stride and a little more in a jump, never a wall or a ledge.
func (g *Grid) Climb(ax, az, bx, bz float64) float64 {
	length := math.Hypot(bx-ax, bz-az)
	n := int(math.Ceil(length / segmentStep))
	worst := 0.0
	prev, have := 0.0, false
	for s := 0; s <= n; s++ {
		t := 1.0
		if n > 0 {
			t = float64(s) / float64(n)
		}
		x, z := ax+(bx-ax)*t, az+(bz-az)*t
		if !g.Walkable(x, z) {
			continue // blocked stretches are Clear's business
		}
		f, _ := g.Floor(x, z)
		if have {
			worst = math.Max(worst, math.Abs(f-prev))
		}
		prev, have = f, true
	}
	return worst
}
