package geo

import (
	"math"
	"sort"
)

// ConvexHull returns the convex hull of points as a closed ring (first point repeated last), counter-clockwise, using
// Andrew's monotone chain. Fewer than three distinct points give a degenerate ring of what there is.
func ConvexHull(points []Point) []Point {
	pts := make([]Point, len(points))
	copy(pts, points)
	sort.Slice(pts, func(i, j int) bool {
		if pts[i].Lon != pts[j].Lon {
			return pts[i].Lon < pts[j].Lon
		}
		return pts[i].Lat < pts[j].Lat
	})
	// Drop duplicates.
	uniq := pts[:0]
	for i, p := range pts {
		if i == 0 || p != pts[i-1] {
			uniq = append(uniq, p)
		}
	}
	pts = uniq
	if len(pts) < 3 {
		out := append([]Point(nil), pts...)
		if len(out) > 0 {
			out = append(out, out[0])
		}
		return out
	}
	cross := func(o, a, b Point) float64 {
		return (a.Lon-o.Lon)*(b.Lat-o.Lat) - (a.Lat-o.Lat)*(b.Lon-o.Lon)
	}
	hull := make([]Point, 0, 2*len(pts))
	for _, p := range pts {
		for len(hull) >= 2 && cross(hull[len(hull)-2], hull[len(hull)-1], p) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	lower := len(hull) + 1
	for i := len(pts) - 2; i >= 0; i-- {
		p := pts[i]
		for len(hull) >= lower && cross(hull[len(hull)-2], hull[len(hull)-1], p) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	return hull // the last point equals the first: a closed ring
}

// RingAreaKm2 is the area of a closed lon/lat ring in km², projected equirectangularly at its mean latitude (accurate
// to well under 1 % for areas the size of a city).
func RingAreaKm2(ring []Point) float64 {
	if len(ring) < 4 {
		return 0
	}
	var lat float64
	for _, p := range ring[:len(ring)-1] {
		lat += p.Lat
	}
	lat /= float64(len(ring) - 1)
	kx := 111.32 * math.Cos(rad(lat))
	const ky = 110.574
	var sum float64
	for i := 0; i+1 < len(ring); i++ {
		a, b := ring[i], ring[i+1]
		sum += a.Lon*kx*b.Lat*ky - b.Lon*kx*a.Lat*ky
	}
	return math.Abs(sum) / 2
}
