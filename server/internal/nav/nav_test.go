package nav_test

import (
	"math"
	"testing"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/nav"
)

func grid(t *testing.T) *nav.Grid {
	t.Helper()
	g, err := content.Nav()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestLoadsTheIslandMap(t *testing.T) {
	g := grid(t)
	m := g.Meta()
	if m.NX != 640 || m.NY != 651 || m.Cell != 0.35 {
		t.Fatalf("meta = %+v", m)
	}
	x, z := g.Spawn()
	if !g.Walkable(x, z) {
		t.Fatalf("the default spawn (%v, %v) is not walkable", x, z)
	}
	if f, ok := g.Floor(x, z); !ok || f < -1 || f > 3 {
		t.Fatalf("spawn floor = %v, %v", f, ok)
	}
	if g.Walkable(1000, 0) || g.Walkable(m.X0-1, 0) {
		t.Fatal("outside the grid must not be walkable")
	}
	if _, ok := g.Floor(1000, 0); ok {
		t.Fatal("no floor outside the grid")
	}
}

func TestRejectsBadInput(t *testing.T) {
	if _, err := nav.Load([]byte("not a png"), []byte(`{"x0":0,"y0":0,"cell":0.35,"nx":2,"ny":2,"spawn":[0,0]}`)); err == nil {
		t.Fatal("garbage PNG accepted")
	}
	if _, err := nav.Load(nil, []byte(`{"cell":0}`)); err == nil {
		t.Fatal("zero cell accepted")
	}
}

// Walking from the spawn in a straight line must eventually hit something that is not walkable (the island ends).
func TestClearStopsAtWalls(t *testing.T) {
	g := grid(t)
	x, z := g.Spawn()
	if !g.Clear(x, z, x+0.5, z, 0.2) {
		t.Fatal("half a metre across the spawn square should be clear")
	}
	// find the first blocked cell east of the spawn, then a point well past it that is walkable again
	bx := x
	for g.Walkable(bx, z) {
		bx += 0.05
	}
	if g.Clear(x, z, bx+30, z, 0.2) {
		t.Fatal("a line through a blocked stretch must not be clear")
	}
	if !g.Clear(x, z, bx-0.1, z, 0.2) {
		t.Fatal("up to the wall should be clear")
	}
}

func TestNearestFitAndFloorRange(t *testing.T) {
	g := grid(t)
	x, z := g.Spawn()
	fx, fz, ok := g.NearestFit(x, z, 0.26, 8)
	if !ok || !g.Fits(fx, fz, 0.26) || math.Hypot(fx-x, fz-z) > 8 {
		t.Fatalf("nearest fit = %v %v %v", fx, fz, ok)
	}
	lo, hi, ok := g.FloorRange(fx, fz)
	f, _ := g.Floor(fx, fz)
	if !ok || lo > f || hi < f {
		t.Fatalf("floor range [%v, %v] does not hold the cell floor %v", lo, hi, f)
	}
	if !g.Standable(fx, fz) {
		t.Fatal("a fitting point is standable")
	}
}

func TestClimbFindsTheStepsAlongALine(t *testing.T) {
	g := grid(t)
	x, z := g.Spawn()
	if c := g.Climb(x, z, x, z); c != 0 {
		t.Fatalf("no line, no climb: %v", c)
	}
	// somewhere on the island there are neighbouring walkable cells far apart in height (the bridge, the plaza
	// steps): find the biggest jump between two neighbours and check Climb sees it
	m := g.Meta()
	best, bx, bz := 0.0, 0.0, 0.0
	for j := 0; j < m.NY; j++ {
		for i := 0; i+1 < m.NX; i++ {
			ax := m.X0 + (float64(i)+0.5)*m.Cell
			az := -(m.Y0 + (float64(j)+0.5)*m.Cell)
			if !g.Walkable(ax, az) || !g.Walkable(ax+m.Cell, az) {
				continue
			}
			fa, _ := g.Floor(ax, az)
			fb, _ := g.Floor(ax+m.Cell, az)
			if d := math.Abs(fa - fb); d > best {
				best, bx, bz = d, ax, az
			}
		}
	}
	if best == 0 {
		t.Skip("a flat map")
	}
	if c := g.Climb(bx, bz, bx+m.Cell, bz); math.Abs(c-best) > 1e-6 {
		t.Fatalf("climb across the biggest step = %v, want %v", c, best)
	}
	t.Logf("biggest step between walkable neighbours: %.2f m", best)
}
