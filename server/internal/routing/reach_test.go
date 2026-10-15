package routing

import (
	"math"
	"testing"
)

type visited struct{ lon, lat, seconds float64 }

func reach(t *testing.T, g *Graph, lon, lat, limit float64) []visited {
	t.Helper()
	var out []visited
	_, settled, err := g.Reach(lon, lat, Car, limit, func(lon, lat, s float64) { out = append(out, visited{lon, lat, s}) })
	if err != nil {
		t.Fatal(err)
	}
	if settled == 0 {
		t.Fatal("nothing settled")
	}
	return out
}

// near reports the shortest time at which a point within 20 m of (lon, lat) was reached, or +Inf.
func near(points []visited, lon, lat float64) float64 {
	best := math.Inf(1)
	for _, p := range points {
		if haversine(p.lon, p.lat, lon, lat) < 20 {
			best = math.Min(best, p.seconds)
		}
	}
	return best
}

func TestReachStopsAtTheTimeLimit(t *testing.T) {
	g := testGraph(t)
	ab := haversine(72.80, 19.00, 72.81, 19.00) / (30 / 3.6) // seconds from A to B at 30 km/h
	const limit = 200.0
	points := reach(t, g, 72.8005, 19.0, limit) // just east of A
	for _, p := range points {
		if p.seconds > limit+1e-9 {
			t.Fatalf("point reached after %.0f s, over the %.0f s limit", p.seconds, limit)
		}
	}
	if s := near(points, 72.81, 19.0); math.Abs(s-0.95*ab) > 1 {
		t.Errorf("B reached after %.0f s, want %.0f", s, 0.95*ab)
	}
	if s := near(points, 72.81, 19.01); math.IsInf(s, 1) {
		t.Error("D, three minutes up Hill Road, was not reached")
	}
	if s := near(points, 72.82, 19.0); !math.IsInf(s, 1) {
		t.Errorf("C reached after %.0f s, beyond the limit", s)
	}
	// Time runs out on B→C: the last point is part of the way along it.
	want := 72.81 + 0.01*(limit-0.95*ab)/ab
	if s := near(points, want, 19.0); math.Abs(s-limit) > 1e-6 {
		t.Errorf("no point where the limit falls on B→C (lon %.4f): %v", want, s)
	}
}

func TestReachFollowsOneWays(t *testing.T) {
	g := testGraph(t)
	// Just west of C on the one-way B→C: cars cannot go back towards B, only on to C and over the hill.
	points := reach(t, g, 72.8195, 19.0, 200)
	if s := near(points, 72.81, 19.0); !math.IsInf(s, 1) {
		t.Errorf("B reached after %.0f s against the one-way", s)
	}
	if s := near(points, 72.815, 19.0); !math.IsInf(s, 1) {
		t.Errorf("half-way back to B reached after %.0f s against the one-way", s)
	}
	if s := near(points, 72.81, 19.01); math.IsInf(s, 1) {
		t.Error("D was not reached from C")
	}
}
