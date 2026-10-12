package routing

import (
	"errors"
	"math"
	"testing"
)

type testSeg struct {
	from, to int
	dir      uint8   // 1 forward, 2 backward, 3 both
	speed    float64 // km/h
	name     string
}

// buildGraph assembles a graph the way the pipeline stores it (straight two-point segments).
func buildGraph(t *testing.T, vertices [][2]float64, segs []testSeg) *Graph {
	t.Helper()
	g := &Graph{offsets: make([]int32, len(vertices)+1)}
	names := map[string]int32{}
	for _, v := range vertices {
		g.nodes = append(g.nodes, int32(math.Round(v[0]*e7)), int32(math.Round(v[1]*e7)))
	}
	g.segShape = []int32{0}
	type edge struct{ from, to, seg int32 }
	var edges []edge
	var times []float32
	for i, s := range segs {
		a, b := vertices[s.from], vertices[s.to]
		length := haversine(a[0], a[1], b[0], b[1])
		g.segLen = append(g.segLen, float32(length))
		x0, y0 := int32(math.Round(a[0]*e7)), int32(math.Round(a[1]*e7))
		x1, y1 := int32(math.Round(b[0]*e7)), int32(math.Round(b[1]*e7))
		g.shape = append(g.shape, x0, y0, x1-x0, y1-y0)
		g.segShape = append(g.segShape, int32(len(g.shape)/2))
		g.segClass = append(g.segClass, 5)
		g.segDir = append(g.segDir, s.dir)
		if _, ok := names[s.name]; !ok {
			names[s.name] = int32(len(g.names))
			g.names = append(g.names, s.name)
		}
		g.segName = append(g.segName, names[s.name])
		secs := float32(length / (s.speed / 3.6))
		if s.dir&1 != 0 {
			edges = append(edges, edge{int32(s.from), int32(s.to), int32(i)})
			times = append(times, secs)
		}
		if s.dir&2 != 0 {
			edges = append(edges, edge{int32(s.to), int32(s.from), ^int32(i)})
			times = append(times, secs)
		}
	}
	// CSR ordering by source vertex.
	for v := range vertices {
		g.offsets[v] = int32(len(g.edgeTo))
		for i, e := range edges {
			if int(e.from) == v {
				g.edgeTo = append(g.edgeTo, e.to)
				g.edgeSeg = append(g.edgeSeg, e.seg)
				g.edgeTime = append(g.edgeTime, times[i])
			}
		}
	}
	g.offsets[len(vertices)] = int32(len(g.edgeTo))
	if err := g.init(); err != nil {
		t.Fatal(err)
	}
	return g
}

// D (72.81, 19.01)
// |  \
// A — B → C      A–B two-way, B→C one-way, A–D and D–C two-way
func testGraph(t *testing.T) *Graph {
	return buildGraph(t, [][2]float64{{72.80, 19.00}, {72.81, 19.00}, {72.82, 19.00}, {72.81, 19.01}}, []testSeg{
		{0, 1, 3, 30, "Main Road"},
		{1, 2, 1, 30, "Main Road"},
		{0, 3, 3, 30, "Hill Road"},
		{3, 2, 3, 30, "Hill Road"},
	})
}

func TestRouteFollowsOneWays(t *testing.T) {
	g := testGraph(t)
	ab := haversine(72.80, 19.00, 72.81, 19.00)
	ad := haversine(72.80, 19.00, 72.81, 19.01)

	r, err := g.Route(72.8005, 19.0, 72.8195, 19.0, Options{Optimize: Shortest})
	if err != nil {
		t.Fatal(err)
	}
	if want := 1.9 * ab; math.Abs(r.DistanceM-want) > 5 { // 0.0005° short of A and of C
		t.Errorf("A→C distance %.0f m, want ≈ %.0f m along Main Road", r.DistanceM, want)
	}
	if len(r.Roads) != 1 || r.Roads[0].Name != "Main Road" {
		t.Errorf("A→C roads %+v, want Main Road only", r.Roads)
	}

	// C→A cannot use the one-way B→C backwards, so it goes over the hill.
	r, err = g.Route(72.8195, 19.0, 72.8005, 19.0, Options{Optimize: Shortest})
	if err != nil {
		t.Fatal(err)
	}
	if r.DistanceM < 2*ad-200 {
		t.Errorf("C→A distance %.0f m, want ≥ %.0f m via Hill Road", r.DistanceM, 2*ad-200)
	}
	if r.Roads[0].Name != "Main Road" && r.Roads[0].Name != "Hill Road" {
		t.Errorf("unexpected roads %+v", r.Roads)
	}
	if first, last := r.Path[0], r.Path[len(r.Path)-1]; math.Abs(first[0]-72.8195) > 1e-6 || math.Abs(last[0]-72.8005) > 1e-6 {
		t.Errorf("path runs %v → %v, want from C to A", first, last)
	}
}

func TestRouteAlongOneSegment(t *testing.T) {
	g := testGraph(t)
	r, err := g.Route(72.802, 19.0001, 72.808, 19.0001, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := haversine(72.802, 19.0, 72.808, 19.0)
	if math.Abs(r.DistanceM-want) > 2 {
		t.Errorf("distance %.1f m, want %.1f m", r.DistanceM, want)
	}
	if math.Abs(r.DurationS-want/(30/3.6)) > 1 {
		t.Errorf("duration %.1f s, want %.1f s", r.DurationS, want/(30/3.6))
	}
	if len(r.Path) != 2 {
		t.Errorf("path %v, want the two snapped points", r.Path)
	}
}

func TestFastestPrefersQuickerRoad(t *testing.T) {
	// Two parallel roads A→B: a short slow lane and a longer fast highway via C.
	g := buildGraph(t, [][2]float64{{72.80, 19.00}, {72.83, 19.00}, {72.815, 19.004}}, []testSeg{
		{0, 1, 3, 10, "Slow Lane"},
		{0, 2, 3, 65, "Highway"},
		{2, 1, 3, 65, "Highway"},
	})
	fast, err := g.Route(72.80, 19.0, 72.83, 19.0, Options{})
	if err != nil {
		t.Fatal(err)
	}
	short, err := g.Route(72.80, 19.0, 72.83, 19.0, Options{Optimize: Shortest})
	if err != nil {
		t.Fatal(err)
	}
	if fast.Roads[0].Name != "Highway" || short.Roads[0].Name != "Slow Lane" {
		t.Errorf("fastest via %q, shortest via %q", fast.Roads[0].Name, short.Roads[0].Name)
	}
	if fast.DurationS >= short.DurationS || fast.DistanceM <= short.DistanceM {
		t.Errorf("fastest %.0fs/%.0fm vs shortest %.0fs/%.0fm", fast.DurationS, fast.DistanceM, short.DurationS, short.DistanceM)
	}
}

func TestNoRoadNearby(t *testing.T) {
	g := testGraph(t)
	if _, err := g.Route(73.5, 19.5, 72.80, 19.0, Options{}); !errors.Is(err, ErrNoRoad) {
		t.Errorf("err = %v, want ErrNoRoad", err)
	}
}
