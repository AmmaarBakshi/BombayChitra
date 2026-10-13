package geo

import (
	"math"
	"testing"
)

func TestConvexHull(t *testing.T) {
	// A square with points inside and on its edges.
	pts := []Point{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0.5, 0.5}, {0.5, 0}, {0.2, 0.7}, {1, 1}}
	hull := ConvexHull(pts)
	want := []Point{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}
	if len(hull) != len(want) {
		t.Fatalf("hull %v, want %v", hull, want)
	}
	for i := range want {
		if hull[i] != want[i] {
			t.Fatalf("hull %v, want %v", hull, want)
		}
	}
}

func TestConvexHullDegenerate(t *testing.T) {
	if h := ConvexHull(nil); len(h) != 0 {
		t.Errorf("empty input: %v", h)
	}
	if h := ConvexHull([]Point{{72.8, 19}, {72.8, 19}}); len(h) != 2 {
		t.Errorf("one distinct point: %v", h)
	}
	if h := ConvexHull([]Point{{72.8, 19}, {72.9, 19.1}}); len(h) != 3 || h[0] != h[2] {
		t.Errorf("two points: %v", h)
	}
}

func TestRingArea(t *testing.T) {
	// 0.01° × 0.01° at 19° N ≈ 1.0526 km × 1.1057 km.
	ring := []Point{{72.80, 19.00}, {72.81, 19.00}, {72.81, 19.01}, {72.80, 19.01}, {72.80, 19.00}}
	want := 0.01 * 111.32 * math.Cos(19.005*math.Pi/180) * 0.01 * 110.574
	if got := RingAreaKm2(ring); math.Abs(got-want) > 1e-6 {
		t.Errorf("area %.6f km², want %.6f", got, want)
	}
	if RingAreaKm2(ring[:3]) != 0 {
		t.Error("an open or degenerate ring has no area")
	}
}
