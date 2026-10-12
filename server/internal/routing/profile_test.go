package routing

import (
	"errors"
	"math"
	"testing"
)

const all = modeCar | modeTwoWheeler | modeFoot

type seg2 struct {
	from, to int
	dir      uint8   // vehicles: 1 forward, 2 backward, 3 both
	speed    float64 // car km/h; 0 on footpaths
	access   uint8
	flags    uint8
	class    uint8
	name     string
}

// buildGraphV2 assembles a version 2 graph the way the pipeline does: an edge for every direction a vehicle may
// drive or anyone may walk, with the car time (or +Inf) on it.
func buildGraphV2(t *testing.T, vertices [][2]float64, segs []seg2) *Graph {
	t.Helper()
	g := &Graph{offsets: make([]int32, len(vertices)+1)}
	for _, v := range vertices {
		g.nodes = append(g.nodes, int32(math.Round(v[0]*e7)), int32(math.Round(v[1]*e7)))
	}
	g.segShape = []int32{0}
	type edge struct {
		from, to, seg int32
		time          float32
	}
	var edges []edge
	names := map[string]int32{}
	inf := float32(math.Inf(1))
	for i, s := range segs {
		a, b := vertices[s.from], vertices[s.to]
		length := haversine(a[0], a[1], b[0], b[1])
		g.segLen = append(g.segLen, float32(length))
		x0, y0 := int32(math.Round(a[0]*e7)), int32(math.Round(a[1]*e7))
		x1, y1 := int32(math.Round(b[0]*e7)), int32(math.Round(b[1]*e7))
		g.shape = append(g.shape, x0, y0, x1-x0, y1-y0)
		g.segShape = append(g.segShape, int32(len(g.shape)/2))
		class := s.class
		if class == 0 {
			class = 6
		}
		g.segClass = append(g.segClass, class)
		g.segDir = append(g.segDir, s.dir)
		g.segAccess = append(g.segAccess, s.access)
		g.segFlags = append(g.segFlags, s.flags)
		g.segSpeed = append(g.segSpeed, float32(s.speed))
		idx := int32(-1)
		if s.name != "" {
			if _, ok := names[s.name]; !ok {
				names[s.name] = int32(len(g.names))
				g.names = append(g.names, s.name)
			}
			idx = names[s.name]
		}
		g.segName = append(g.segName, idx)
		carTime := inf
		if s.access&modeCar != 0 && s.speed > 0 {
			carTime = float32(length / (s.speed / 3.6))
		}
		vehicles := s.access&(modeCar|modeTwoWheeler) != 0
		foot := s.access&modeFoot != 0
		if foot || vehicles && s.dir&1 != 0 {
			tm := inf
			if s.dir&1 != 0 {
				tm = carTime
			}
			edges = append(edges, edge{int32(s.from), int32(s.to), int32(i), tm})
		}
		if foot || vehicles && s.dir&2 != 0 {
			tm := inf
			if s.dir&2 != 0 {
				tm = carTime
			}
			edges = append(edges, edge{int32(s.to), int32(s.from), ^int32(i), tm})
		}
	}
	for v := range vertices {
		g.offsets[v] = int32(len(g.edgeTo))
		for _, e := range edges {
			if int(e.from) == v {
				g.edgeTo = append(g.edgeTo, e.to)
				g.edgeSeg = append(g.edgeSeg, e.seg)
				g.edgeTime = append(g.edgeTime, e.time)
			}
		}
	}
	g.offsets[len(vertices)] = int32(len(g.edgeTo))
	if err := g.init(); err != nil {
		t.Fatal(err)
	}
	return g
}

func route(t *testing.T, g *Graph, from, to [2]float64, opts Options) *Result {
	t.Helper()
	r, err := g.Route(from[0], from[1], to[0], to[1], opts)
	if err != nil {
		t.Fatalf("%+v: %v", opts, err)
	}
	return r
}

func names(r *Result) []string {
	var out []string
	for _, road := range r.Roads {
		out = append(out, road.Name)
	}
	return out
}

func TestPedestriansIgnoreOneWays(t *testing.T) {
	// A → B is a one-way street; the way back for vehicles is a long loop via D.
	g := buildGraphV2(t, [][2]float64{{72.80, 19.00}, {72.81, 19.00}, {72.805, 19.02}}, []seg2{
		{from: 0, to: 1, dir: 1, speed: 18, access: all, name: "One Way Lane"},
		{from: 1, to: 2, dir: 3, speed: 30, access: all, name: "Loop Road"},
		{from: 2, to: 0, dir: 3, speed: 30, access: all, name: "Loop Road"},
	})
	b, a := [2]float64{72.8098, 19.0}, [2]float64{72.8002, 19.0}
	car := route(t, g, b, a, Options{Mode: Car})
	walk := route(t, g, b, a, Options{Mode: Foot})
	if car.DistanceM < 3000 {
		t.Errorf("car went against the one-way street: %.0f m via %v", car.DistanceM, names(car))
	}
	if walk.DistanceM > 1100 || names(walk)[0] != "One Way Lane" {
		t.Errorf("walk took %.0f m via %v, want the lane", walk.DistanceM, names(walk))
	}
	if want := walk.DistanceM / walkSpeed; math.Abs(walk.DurationS-want) > 1 {
		t.Errorf("walking time %.0f s, want %.0f s at 4.8 km/h", walk.DurationS, want)
	}
	if walk.Mode != Foot || car.Mode != Car {
		t.Errorf("modes %q, %q", walk.Mode, car.Mode)
	}
}

func TestFootpathsAndFootOverBridges(t *testing.T) {
	// A foot over-bridge crosses the tracks between A and C; vehicles must go round via B.
	g := buildGraphV2(t, [][2]float64{{72.84, 19.05}, {72.86, 19.06}, {72.84, 19.055}}, []seg2{
		{from: 0, to: 2, dir: 3, access: modeFoot, flags: flagBridge, class: 10},
		{from: 0, to: 1, dir: 3, speed: 30, access: all, name: "Station Road"},
		{from: 1, to: 2, dir: 3, speed: 30, access: all, name: "Station Road"},
	})
	a, c := [2]float64{72.8401, 19.0501}, [2]float64{72.8401, 19.0549}
	walk := route(t, g, a, c, Options{Mode: Foot})
	if walk.DistanceM > 600 {
		t.Errorf("walk %.0f m via %v, want the bridge", walk.DistanceM, names(walk))
	}
	car := route(t, g, a, c, Options{Mode: Car})
	if car.DistanceM < 3000 || car.Roads[0].Name != "Station Road" {
		t.Errorf("car %.0f m via %v, want Station Road", car.DistanceM, names(car))
	}
	// A point on the bridge snaps to the road for a car.
	if car.From.Road != "Station Road" {
		t.Errorf("car snapped to %q", car.From.Road)
	}
}

func TestTwoWheelersStayOffMotorways(t *testing.T) {
	g := buildGraphV2(t, [][2]float64{{72.80, 19.00}, {72.90, 19.00}, {72.85, 19.03}}, []seg2{
		{from: 0, to: 1, dir: 3, speed: 65, access: modeCar, class: 1, name: "Sea Link"},
		{from: 0, to: 2, dir: 3, speed: 35, access: all, class: 3, name: "S V Road"},
		{from: 2, to: 1, dir: 3, speed: 35, access: all, class: 3, name: "S V Road"},
	})
	from, to := [2]float64{72.8, 19.0}, [2]float64{72.9, 19.0}
	if car := route(t, g, from, to, Options{Mode: Car}); car.Roads[0].Name != "Sea Link" {
		t.Errorf("car via %v, want the sea link", names(car))
	}
	bike := route(t, g, from, to, Options{Mode: TwoWheeler})
	if bike.Roads[0].Name != "S V Road" {
		t.Errorf("two-wheeler via %v, want S V Road", names(bike))
	}
	if want := bike.DistanceM / (35 / 3.6 * twoWheelerFactor); math.Abs(bike.DurationS-want) > 1 {
		t.Errorf("two-wheeler time %.0f s, want %.0f s", bike.DurationS, want)
	}
}

func TestAvoidTolls(t *testing.T) {
	g := buildGraphV2(t, [][2]float64{{72.90, 18.95}, {73.00, 18.95}, {72.95, 18.99}}, []seg2{
		{from: 0, to: 1, dir: 3, speed: 65, access: modeCar, flags: flagToll | flagBridge, class: 1, name: "Atal Setu"},
		{from: 0, to: 2, dir: 3, speed: 30, access: all, class: 4, name: "Palm Beach Road"},
		{from: 2, to: 1, dir: 3, speed: 30, access: all, class: 4, name: "Palm Beach Road"},
	})
	from, to := [2]float64{72.90, 18.95}, [2]float64{73.00, 18.95}
	toll := route(t, g, from, to, Options{Mode: Car})
	if toll.TollM < 10_000 {
		t.Errorf("toll distance %.0f m, want the whole bridge", toll.TollM)
	}
	free := route(t, g, from, to, Options{Mode: Car, AvoidTolls: true})
	if free.TollM != 0 || free.Roads[0].Name != "Palm Beach Road" {
		t.Errorf("avoiding tolls: %.0f m of toll via %v", free.TollM, names(free))
	}
}

func TestStepsAreSlow(t *testing.T) {
	g := buildGraphV2(t, [][2]float64{{72.84, 19.05}, {72.84, 19.051}}, []seg2{
		{from: 0, to: 1, dir: 3, access: modeFoot, flags: flagSteps, class: 10},
	})
	r := route(t, g, [2]float64{72.84, 19.05}, [2]float64{72.84, 19.051}, Options{Mode: Foot})
	if want := r.DistanceM / stepsSpeed; math.Abs(r.DurationS-want) > 1 {
		t.Errorf("steps took %.0f s, want %.0f s", r.DurationS, want)
	}
}

func TestVersion1GraphsAreCarsOnly(t *testing.T) {
	g := testGraph(t)
	if got := g.Modes(); len(got) != 1 || got[0] != Car {
		t.Errorf("modes %v, want car only", got)
	}
	if _, err := g.Route(72.8005, 19.0, 72.8195, 19.0, Options{Mode: Foot}); !errors.Is(err, ErrModeUnavailable) {
		t.Errorf("walking on a version 1 graph: %v", err)
	}
	if g.Stats().Version != 1 {
		t.Errorf("version %d", g.Stats().Version)
	}
}
