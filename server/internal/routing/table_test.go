package routing

import (
	"errors"
	"math"
	"testing"
)

// Every cell of a table equals the route between the same two points, fastest and shortest.
func TestTableMatchesRoutes(t *testing.T) {
	g := testGraph(t)
	points := [][2]float64{{72.8005, 19.0}, {72.8195, 19.0}, {72.81, 19.0095}, {72.805, 19.0}}
	for _, opt := range []Optimize{Fastest, Shortest} {
		table, err := g.Table(points, points, Options{Optimize: opt})
		if err != nil {
			t.Fatal(err)
		}
		for i, a := range points {
			for j, b := range points {
				r, err := g.Route(a[0], a[1], b[0], b[1], Options{Optimize: opt})
				dur, dist := table.Durations[i][j], table.Distances[i][j]
				if errors.Is(err, ErrNoRoute) {
					if dur != nil {
						t.Errorf("%s %d→%d: table %.0f s, but there is no route", opt, i, j, *dur)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if dur == nil || dist == nil {
					t.Errorf("%s %d→%d: no table entry, route %.0f s", opt, i, j, r.DurationS)
					continue
				}
				if math.Abs(*dur-r.DurationS) > 1e-3 || math.Abs(*dist-r.DistanceM) > 1e-3 {
					t.Errorf("%s %d→%d: table %.1f s / %.1f m, route %.1f s / %.1f m", opt, i, j, *dur, *dist, r.DurationS, r.DistanceM)
				}
			}
		}
	}
}

func TestTableOneWaysAndPointsOffTheMap(t *testing.T) {
	g := testGraph(t)
	// From near A, east to near C on Main Road; back from C must go over the hill.
	table, err := g.Table([][2]float64{{72.8005, 19.0}, {73.5, 19.5}}, [][2]float64{{72.8195, 19.0}, {72.8005, 19.0}}, Options{Optimize: Shortest})
	if err != nil {
		t.Fatal(err)
	}
	if table.Sources[1] != nil || table.Durations[1][0] != nil {
		t.Error("a source far from any road should have no snap and no times")
	}
	ac := *table.Distances[0][0]
	back, err := g.Table([][2]float64{{72.8195, 19.0}}, [][2]float64{{72.8005, 19.0}}, Options{Optimize: Shortest})
	if err != nil {
		t.Fatal(err)
	}
	if ca := *back.Distances[0][0]; ca < ac+500 {
		t.Errorf("C→A %.0f m should be longer than A→C %.0f m: the one-way forces a detour", ca, ac)
	}
	if d := *table.Durations[0][1]; d != 0 {
		t.Errorf("a point to itself takes %.1f s", d)
	}
}
