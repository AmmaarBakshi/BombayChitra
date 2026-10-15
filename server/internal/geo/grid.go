package geo

import "math"

// Cell is a grid cell by column and row.
type Cell struct{ X, Y int32 }

// Grid is a regular lon/lat grid: cells of DX × DY degrees, cell (0, 0) starting at Lon0, Lat0.
type Grid struct{ Lon0, Lat0, DX, DY float64 }

// NewGrid returns a grid of cells about sizeM metres square, anchored at a point.
func NewGrid(at Point, sizeM float64) Grid {
	return Grid{Lon0: at.Lon, Lat0: at.Lat, DX: sizeM / (111320 * math.Cos(rad(at.Lat))), DY: sizeM / 110574}
}

// CellOf returns the cell containing p.
func (g Grid) CellOf(p Point) Cell {
	return Cell{int32(math.Floor((p.Lon - g.Lon0) / g.DX)), int32(math.Floor((p.Lat - g.Lat0) / g.DY))}
}

type corner struct{ x, y int32 }

// Outline traces the boundary of a set of cells as GeoJSON polygon coordinates ([polygon][ring][point], rings
// closed): each polygon's outer ring runs counter-clockwise and is followed by its holes, running clockwise. Cells
// that touch only at a corner make separate polygons.
func (g Grid) Outline(cells map[Cell]bool) [][][][2]float64 {
	// Boundary edges between cell corners, directed so the cells lie on their left.
	next := map[corner][]corner{}
	add := func(a, b corner) { next[a] = append(next[a], b) }
	for c := range cells {
		x, y := c.X, c.Y
		if !cells[Cell{x, y - 1}] {
			add(corner{x, y}, corner{x + 1, y})
		}
		if !cells[Cell{x + 1, y}] {
			add(corner{x + 1, y}, corner{x + 1, y + 1})
		}
		if !cells[Cell{x, y + 1}] {
			add(corner{x + 1, y + 1}, corner{x, y + 1})
		}
		if !cells[Cell{x - 1, y}] {
			add(corner{x, y + 1}, corner{x, y})
		}
	}

	var outers, holes [][]corner
	trace := func(start corner) {
		ring := []corner{start}
		cur, dir := start, corner{}
		for {
			to := take(next, cur, dir)
			dir = corner{to.x - cur.x, to.y - cur.y}
			cur = to
			if cur == start {
				break
			}
			ring = append(ring, cur)
		}
		ring = straighten(ring)
		if area2(ring) > 0 {
			outers = append(outers, ring)
		} else {
			holes = append(holes, ring)
		}
	}
	// Rings start at a corner only one edge leaves, so a diagonal touch is never a ring's start and end.
	starts := make([]corner, 0, len(next))
	for c := range next {
		starts = append(starts, c)
	}
	for _, c := range starts {
		if len(next[c]) == 1 {
			trace(c)
		}
	}
	for _, c := range starts {
		for len(next[c]) > 0 {
			trace(c)
		}
	}

	// Each hole belongs to the smallest outer ring around the cell beside it.
	owned := make([][][]corner, len(outers))
	for _, h := range holes {
		a, b := h[0], h[1]
		dx, dy := sign(b.x-a.x), sign(b.y-a.y)
		qx := float64(a.x) + float64(dx)*0.5 - float64(dy)*0.5 // centre of the cell on the edge's left
		qy := float64(a.y) + float64(dy)*0.5 + float64(dx)*0.5
		best, bestArea := -1, math.Inf(1)
		for i, o := range outers {
			if a := float64(area2(o)); a < bestArea && insideCorners(qx, qy, o) {
				best, bestArea = i, a
			}
		}
		if best >= 0 {
			owned[best] = append(owned[best], h)
		}
	}

	out := make([][][][2]float64, 0, len(outers))
	for i, o := range outers {
		polygon := [][][2]float64{g.coords(o)}
		for _, h := range owned[i] {
			polygon = append(polygon, g.coords(h))
		}
		out = append(out, polygon)
	}
	return out
}

// take removes and returns the edge leaving c, preferring a left turn from the incoming direction where two leave
// (a corner where cells touch diagonally), which keeps such cells in separate rings.
func take(next map[corner][]corner, c, dir corner) corner {
	edges := next[c]
	pick := 0
	if len(edges) > 1 {
		left := corner{-dir.y, dir.x}
		for i, e := range edges {
			if (corner{e.x - c.x, e.y - c.y}) == left {
				pick = i
			}
		}
	}
	to := edges[pick]
	edges = append(edges[:pick], edges[pick+1:]...)
	if len(edges) == 0 {
		delete(next, c)
	} else {
		next[c] = edges
	}
	return to
}

// straighten drops corners in the middle of straight runs.
func straighten(ring []corner) []corner {
	n := len(ring)
	out := make([]corner, 0, n)
	for i, c := range ring {
		p, q := ring[(i+n-1)%n], ring[(i+1)%n]
		if (c.x-p.x)*(q.y-c.y) != (c.y-p.y)*(q.x-c.x) {
			out = append(out, c)
		}
	}
	return out
}

// area2 is twice the signed area of a ring: positive when it runs counter-clockwise.
func area2(ring []corner) int64 {
	var a int64
	for i, c := range ring {
		d := ring[(i+1)%len(ring)]
		a += int64(c.x)*int64(d.y) - int64(d.x)*int64(c.y)
	}
	return a
}

func insideCorners(x, y float64, ring []corner) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		xi, yi, xj, yj := float64(ring[i].x), float64(ring[i].y), float64(ring[j].x), float64(ring[j].y)
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

func sign(v int32) int32 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// coords converts a ring of corners to closed lon/lat coordinates.
func (g Grid) coords(ring []corner) [][2]float64 {
	out := make([][2]float64, 0, len(ring)+1)
	for _, c := range ring {
		out = append(out, [2]float64{g.Lon0 + float64(c.x)*g.DX, g.Lat0 + float64(c.y)*g.DY})
	}
	return append(out, out[0])
}
