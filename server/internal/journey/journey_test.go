package journey

import (
	"errors"
	"math"
	"testing"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// A north–south railway (A1…A4, about 1.1 km apart) and an east–west metro (B1…B3) starting 315 m east of A3.
var (
	railway = Line{Key: "Rail:A", Label: "A Line", Ref: "A", Mode: "train", Colour: "#ff0000"}
	metro   = Line{Key: "Metro:B", Label: "Line B", Ref: "B", Mode: "subway", Colour: "#0000ff"}
	aStops  = []Stop{{1, "A1", 72.85, 19.00}, {2, "A2", 72.85, 19.01}, {3, "A3 Junction", 72.85, 19.02}, {4, "A4", 72.85, 19.03}}
	bStops  = []Stop{{11, "B1", 72.853, 19.0205}, {12, "B2", 72.865, 19.0205}, {13, "B3", 72.875, 19.0205}}
)

func testNetwork() *Network {
	backwards := []Stop{{21, "A4", 72.85, 19.0301}, {22, "A3", 72.8501, 19.02}, {23, "A2", 72.85, 19.01}, {24, "A1", 72.85, 19.0}}
	return Build([]Route{
		// The track is drawn north to south, against the stops of the first route.
		{ID: 100, Line: railway, Headsign: "A4", Stops: aStops, HeadwayMin: 4,
			Track: [][][2]float64{{{72.85, 19.035}, {72.85, 18.995}}}},
		{ID: 101, Line: railway, Headsign: "A1", Stops: backwards, HeadwayMin: 4},
		{ID: 200, Line: metro, Headsign: "B3", Stops: bStops, HeadwayMin: 8},
		{ID: 300, Line: Line{Key: "Bus:1", Mode: "bus"}, Stops: aStops},
	})
}

func TestBuildMergesStationsAndSkipsUnknownModes(t *testing.T) {
	n := testNetwork()
	if len(n.Stations) != 7 {
		t.Fatalf("%d stations, want 7 (both directions of A share stations)", len(n.Stations))
	}
	if len(n.routes) != 3 {
		t.Fatalf("%d routes, want 3 (the bus is not a rail mode)", len(n.routes))
	}
	if got := n.Stations[2].Lines; len(got) != 1 || got[0] != "Rail:A" {
		t.Errorf("A3 lines = %v", got)
	}
	r := n.routes[0]
	if r.track == nil || r.track[0][1] > r.track[1][1] {
		t.Fatalf("track not turned to run with the stops: %v", r.track)
	}
	if d := r.at[3] - r.at[0]; math.Abs(d-3335) > 20 {
		t.Errorf("A1→A4 along the track = %.0f m, want about 3335", d)
	}
	if n.routes[1].track != nil {
		t.Error("a route without a track should join its stops directly")
	}
}

func TestPlanRideChangeAndWalk(t *testing.T) {
	n := testNetwork()
	from, to := geo.Point{Lon: 72.85, Lat: 18.998}, geo.Point{Lon: 72.875, Lat: 19.022}
	j, err := n.Plan(from, to, Options{MaxWalkM: 1500})
	if err != nil {
		t.Fatal(err)
	}
	kinds := ""
	for _, l := range j.Legs {
		if l.Ride {
			kinds += "R"
		} else {
			kinds += "W"
		}
	}
	if kinds != "WRWRW" {
		t.Fatalf("legs %s, want walk, ride, change, ride, walk: %+v", kinds, j.Legs)
	}
	first, change, second := j.Legs[1], j.Legs[2], j.Legs[3]
	if n.routes[first.Route].ID != 100 || first.FromStop != 0 || first.ToStop != 2 {
		t.Errorf("first ride %+v, want route 100 from A1 to A3", first)
	}
	if first.WaitS != 120 {
		t.Errorf("wait %.0f s, want half of the 4-minute gap", first.WaitS)
	}
	if n.Stations[change.From].Name != "A3 Junction" || n.Stations[change.To].Name != "B1" {
		t.Errorf("change from %s to %s", n.Stations[change.From].Name, n.Stations[change.To].Name)
	}
	if second.WaitS != 240+platformS {
		t.Errorf("wait after a change %.0f s, want half the gap plus changing platforms", second.WaitS)
	}
	total := 0.0
	for _, l := range j.Legs {
		total += l.DurationS + l.WaitS
	}
	if math.Abs(total-j.DurationS) > 1e-6 {
		t.Errorf("legs add up to %.0f s, journey says %.0f", total, j.DurationS)
	}
	j.SetWalk(2, 400, 300)
	if j.Legs[2].DistanceM != 400 || j.Legs[2].DurationS != 300+exitS {
		t.Errorf("measured change walk %+v, want 400 m in 300 s plus leaving and entering the stations", j.Legs[2])
	}
	if math.Abs(j.DurationS-(total-change.DurationS+300+exitS)) > 1e-6 {
		t.Errorf("journey %.0f s after measuring the change walk", j.DurationS)
	}
	if got := n.Via(first); len(got) != 1 || n.Stations[got[0]].Name != "A2" {
		t.Errorf("via %v, want A2", got)
	}
	if geom := n.RideGeometry(first); len(geom) < 2 || math.Abs(geom[0][1]-19.0) > 1e-6 || math.Abs(geom[len(geom)-1][1]-19.02) > 1e-6 {
		t.Errorf("ride geometry %v should run from A1 to A3 along the track", geom)
	}
}

func TestPlanWalksWhenThatIsQuicker(t *testing.T) {
	n := testNetwork()
	j, err := n.Plan(geo.Point{Lon: 72.85, Lat: 19.0}, geo.Point{Lon: 72.851, Lat: 19.003}, Options{MaxWalkM: 1500})
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Legs) != 1 || j.Legs[0].Ride || j.Legs[0].From != -1 || j.Legs[0].To != -1 {
		t.Fatalf("legs %+v, want a single walk", j.Legs)
	}
}

func TestPlanErrorsAndModes(t *testing.T) {
	n := testNetwork()
	if _, err := n.Plan(geo.Point{Lon: 73.5, Lat: 19.5}, geo.Point{Lon: 72.85, Lat: 19.0}, Options{MaxWalkM: 1500}); !errors.Is(err, ErrNoStation) {
		t.Errorf("far from any station: %v", err)
	}
	from, to := geo.Point{Lon: 72.85, Lat: 18.998}, geo.Point{Lon: 72.875, Lat: 19.022}
	if _, err := n.Plan(from, to, Options{MaxWalkM: 1500, Modes: map[string]bool{"subway": true}}); !errors.Is(err, ErrNoStation) {
		t.Errorf("metro only, no metro near the start: %v", err)
	}
	// Riding the railway the wrong way round is impossible: route 101 runs north to south only.
	j, err := n.Plan(geo.Point{Lon: 72.85, Lat: 19.031}, geo.Point{Lon: 72.85, Lat: 18.999}, Options{MaxWalkM: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Legs) != 3 || n.routes[j.Legs[1].Route].ID != 101 {
		t.Errorf("southbound journey %+v, want a ride on route 101", j.Legs)
	}
}

func TestExpressPatternsWaitLonger(t *testing.T) {
	n := Build([]Route{
		{ID: 1, Line: railway, Stops: aStops, HeadwayMin: 4},
		{ID: 2, Line: railway, Stops: []Stop{aStops[0], aStops[3]}, HeadwayMin: 4},
	})
	if n.routes[0].express || !n.routes[1].express {
		t.Fatalf("express = %v, %v; want the two-stop pattern to be express", n.routes[0].express, n.routes[1].express)
	}
	if n.routes[0].wait != 120 || n.routes[1].wait != 240 {
		t.Errorf("waits %.0f, %.0f s", n.routes[0].wait, n.routes[1].wait)
	}
	if n.routes[1].hop[0] >= n.routes[0].hop[0]+n.routes[0].hop[1]+n.routes[0].hop[2] {
		t.Error("the express should ride A1→A4 faster than the stopping train")
	}
}

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Kalyan Junction":         "kalyan",
		"Panvel (Suburban)":       "panvel",
		"Dahisar (East)":          "dahisar east",
		"Ghatkopar Metro Station": "ghatkopar",
		"Vasai Road":              "vasai road",
	}
	for in, want := range cases {
		if got := fold(in); got != want {
			t.Errorf("fold(%q) = %q, want %q", in, got, want)
		}
	}
}
