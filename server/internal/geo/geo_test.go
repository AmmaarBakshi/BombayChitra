package geo

import (
	"math"
	"testing"
)

func TestHaversine(t *testing.T) {
	// Chhatrapati Shivaji Maharaj Terminus → Churchgate is ~1 km in a straight line.
	d := Haversine(Point{72.8355, 18.9398}, Point{72.8272, 18.9353})
	if d < 950 || d > 1050 {
		t.Fatalf("CSMT–Churchgate = %.0f m", d)
	}
}

func TestParseBBox(t *testing.T) {
	if _, err := ParseBBox("72.8,19.0,72.9,19.1"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "1,2,3", "a,b,c,d", "72.9,19,72.8,19.1", "1,95,2,96"} {
		if _, err := ParseBBox(bad); err == nil {
			t.Errorf("ParseBBox(%q) should fail", bad)
		}
	}
}

func TestNearestOnLine(t *testing.T) {
	line := [][2]float64{{72.80, 19.00}, {72.82, 19.00}}
	snap, ok := NearestOnLine(Point{72.81, 19.001}, line)
	if !ok {
		t.Fatal("no snap")
	}
	if math.Abs(snap.Point.Lon-72.81) > 1e-9 || math.Abs(snap.Point.Lat-19.00) > 1e-9 {
		t.Errorf("snapped to %v", snap.Point)
	}
	if snap.DistanceM < 100 || snap.DistanceM > 120 {
		t.Errorf("distance %.1f m, want ~110 m", snap.DistanceM)
	}
}

func TestPolygonIndex(t *testing.T) {
	poly, err := NewPolygonIndex(`{"type":"Polygon","coordinates":[[[0,0],[10,0],[10,10],[0,10],[0,0]],[[4,4],[6,4],[6,6],[4,6],[4,4]]]}`)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[Point]bool{{1, 1}: true, {5, 5}: false, {11, 5}: false, {9.9, 9.9}: true}
	for p, want := range cases {
		if got := poly.Contains(p); got != want {
			t.Errorf("Contains(%v) = %v, want %v", p, got, want)
		}
	}
}
