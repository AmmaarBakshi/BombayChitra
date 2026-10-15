package geo

import (
	"math"
	"testing"
)

func cells(cs ...[2]int32) map[Cell]bool {
	m := map[Cell]bool{}
	for _, c := range cs {
		m[Cell{c[0], c[1]}] = true
	}
	return m
}

// block returns the cells of a w × h rectangle from (x, y), less the listed holes.
func block(x, y, w, h int32, holes ...[2]int32) map[Cell]bool {
	m := map[Cell]bool{}
	for i := x; i < x+w; i++ {
		for j := y; j < y+h; j++ {
			m[Cell{i, j}] = true
		}
	}
	for _, c := range holes {
		delete(m, Cell{c[0], c[1]})
	}
	return m
}

// signedArea of closed lon/lat coordinates in grid units (the test grid has 1° cells at the origin).
func signedArea(ring [][2]float64) float64 {
	var a float64
	for i := 0; i+1 < len(ring); i++ {
		a += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	return a / 2
}

var unit = Grid{DX: 1, DY: 1}

func TestOutlineSquaresAndRectangles(t *testing.T) {
	polys := unit.Outline(cells([2]int32{0, 0}))
	if len(polys) != 1 || len(polys[0]) != 1 || len(polys[0][0]) != 5 {
		t.Fatalf("one cell: %v", polys)
	}
	if ring := polys[0][0]; ring[0] != ring[4] || signedArea(ring) != 1 {
		t.Errorf("one cell should be a closed counter-clockwise unit square: %v", ring)
	}
	polys = unit.Outline(block(0, 0, 3, 2))
	if len(polys) != 1 || len(polys[0][0]) != 5 || signedArea(polys[0][0]) != 6 {
		t.Errorf("3×2 block: straight runs should merge into four corners: %v", polys)
	}
}

func TestOutlineHolesAndIslands(t *testing.T) {
	polys := unit.Outline(block(0, 0, 3, 3, [2]int32{1, 1}))
	if len(polys) != 1 || len(polys[0]) != 2 {
		t.Fatalf("ring of cells: want an outer ring and a hole, got %v", polys)
	}
	if signedArea(polys[0][0]) != 9 || signedArea(polys[0][1]) != -1 {
		t.Errorf("outer %.0f, hole %.0f; want 9 and -1", signedArea(polys[0][0]), signedArea(polys[0][1]))
	}

	// A 5×5 frame with a cell in the middle of its 3×3 hole: the frame with its hole, and the island.
	frame := block(0, 0, 5, 5)
	for x := int32(1); x <= 3; x++ {
		for y := int32(1); y <= 3; y++ {
			delete(frame, Cell{x, y})
		}
	}
	frame[Cell{2, 2}] = true
	polys = unit.Outline(frame)
	if len(polys) != 2 {
		t.Fatalf("frame and island: %d polygons", len(polys))
	}
	var areas []float64
	for _, p := range polys {
		total := 0.0
		for _, r := range p {
			total += signedArea(r)
		}
		areas = append(areas, total)
	}
	if !(areas[0] == 16 && areas[1] == 1 || areas[0] == 1 && areas[1] == 16) {
		t.Errorf("areas %v, want the frame (25 − 9 = 16) and the island (1)", areas)
	}
}

func TestOutlineSeparatesDiagonalNeighbours(t *testing.T) {
	polys := unit.Outline(cells([2]int32{0, 0}, [2]int32{1, 1}, [2]int32{5, 5}))
	if len(polys) != 3 {
		t.Errorf("cells touching at a corner, and a lone cell: %d polygons, want 3", len(polys))
	}
}

func TestGridCells(t *testing.T) {
	g := NewGrid(Point{Lon: 72.8, Lat: 19.0}, 100)
	if c := g.CellOf(Point{Lon: 72.8, Lat: 19.0}); c != (Cell{0, 0}) {
		t.Errorf("anchor in cell %v", c)
	}
	// 250 m east and 150 m south.
	p := Point{Lon: 72.8 + 2.5*g.DX, Lat: 19.0 - 1.5*g.DY}
	if c := g.CellOf(p); c != (Cell{2, -2}) {
		t.Errorf("cell %v, want {2 -2}", c)
	}
	if d := Haversine(Point{Lon: 72.8, Lat: 19.0}, Point{Lon: 72.8 + g.DX, Lat: 19.0}); math.Abs(d-100) > 0.5 {
		t.Errorf("cell width %.1f m, want 100", d)
	}
}

// Whatever the pattern, the rings enclose exactly the cells: outer areas less holes add up to the cell count.
func TestOutlineAreaMatchesCells(t *testing.T) {
	seed := uint32(7)
	random := func() uint32 { seed ^= seed << 13; seed ^= seed >> 17; seed ^= seed << 5; return seed }
	for round := 0; round < 20; round++ {
		set := map[Cell]bool{}
		for x := int32(0); x < 30; x++ {
			for y := int32(0); y < 30; y++ {
				if random()%2 == 0 {
					set[Cell{x, y}] = true
				}
			}
		}
		total := 0.0
		for _, p := range unit.Outline(set) {
			if signedArea(p[0]) <= 0 {
				t.Fatalf("round %d: outer ring not counter-clockwise", round)
			}
			for _, r := range p {
				total += signedArea(r)
			}
		}
		if total != float64(len(set)) {
			t.Fatalf("round %d: rings enclose %.0f cells, want %d", round, total, len(set))
		}
	}
}
