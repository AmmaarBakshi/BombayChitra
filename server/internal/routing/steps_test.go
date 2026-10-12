package routing

import (
	"math"
	"strings"
	"testing"
)

func instructions(steps []Step) string {
	var out []string
	for _, s := range steps {
		out = append(out, s.Instruction)
	}
	return strings.Join(out, " | ")
}

func TestStepsAtATurn(t *testing.T) {
	// East along Main Road, then left (north) up Hill Road.
	g := buildGraphV2(t, [][2]float64{{72.80, 19.00}, {72.81, 19.00}, {72.81, 19.01}}, []seg2{
		{from: 0, to: 1, dir: 3, speed: 30, access: all, name: "Main Road"},
		{from: 1, to: 2, dir: 3, speed: 30, access: all, name: "Hill Road"},
	})
	r := route(t, g, [2]float64{72.801, 19.0}, [2]float64{72.81, 19.009}, Options{Mode: Car})
	want := "Head east on Main Road | Turn left onto Hill Road | Arrive at your destination"
	if got := instructions(r.Steps); got != want {
		t.Fatalf("steps:\n got %s\nwant %s", got, want)
	}
	turn := r.Steps[1]
	if turn.Type != "turn" || turn.Modifier != "left" || turn.Name != "Hill Road" || turn.Bearing != 0 {
		t.Errorf("turn step %+v", turn)
	}
	if math.Abs(turn.Location[0]-72.81) > 1e-6 || math.Abs(turn.Location[1]-19.0) > 1e-6 {
		t.Errorf("turn at %v, want the junction", turn.Location)
	}
	var total float64
	for _, s := range r.Steps {
		total += s.DistanceM
	}
	if math.Abs(total-r.DistanceM) > 2 {
		t.Errorf("steps add up to %.0f m, route is %.0f m", total, r.DistanceM)
	}
	if last := r.Steps[len(r.Steps)-1]; last.Type != "arrive" || last.DistanceM != 0 {
		t.Errorf("last step %+v", last)
	}
}

func TestStepsMergeOneRoadAndNameChanges(t *testing.T) {
	// S V Road is split into two segments at a junction; it then becomes Linking Road straight ahead.
	g := buildGraphV2(t, [][2]float64{{72.80, 19.00}, {72.80, 19.01}, {72.80, 19.02}, {72.80, 19.03}, {72.79, 19.01}}, []seg2{
		{from: 0, to: 1, dir: 3, speed: 30, access: all, name: "S V Road"},
		{from: 1, to: 2, dir: 3, speed: 30, access: all, name: "S V Road"},
		{from: 2, to: 3, dir: 3, speed: 30, access: all, name: "Linking Road"},
		{from: 1, to: 4, dir: 3, speed: 30, access: all, name: "Side Street"},
	})
	r := route(t, g, [2]float64{72.80, 19.001}, [2]float64{72.80, 19.029}, Options{Mode: Car})
	want := "Head north on S V Road | Continue onto Linking Road | Arrive at your destination"
	if got := instructions(r.Steps); got != want {
		t.Fatalf("steps:\n got %s\nwant %s", got, want)
	}
	if r.Steps[0].DistanceM < 2000 {
		t.Errorf("first step %.0f m, want both S V Road segments", r.Steps[0].DistanceM)
	}
}

func TestWalkingStepsDescribePaths(t *testing.T) {
	// Steps up to a foot over-bridge across the tracks, then down a footpath.
	g := buildGraphV2(t, [][2]float64{{72.840, 19.050}, {72.8402, 19.050}, {72.8402, 19.0508}, {72.8410, 19.0508}}, []seg2{
		{from: 0, to: 1, dir: 3, access: modeFoot, flags: flagSteps, class: 10},
		{from: 1, to: 2, dir: 3, access: modeFoot, flags: flagBridge, class: 10},
		{from: 2, to: 3, dir: 3, access: modeFoot, class: 10},
	})
	r := route(t, g, [2]float64{72.840, 19.050}, [2]float64{72.8410, 19.0508}, Options{Mode: Foot})
	want := "Head east along the steps | Turn left and cross the foot over-bridge | Turn right onto the footpath | Arrive at your destination"
	if got := instructions(r.Steps); got != want {
		t.Fatalf("steps:\n got %s\nwant %s", got, want)
	}
}

func TestStepsForRampsAndRoundabouts(t *testing.T) {
	g := buildGraphV2(t, [][2]float64{{72.80, 19.00}, {72.81, 19.00}, {72.815, 19.003}, {72.82, 19.00}, {72.83, 19.00}}, []seg2{
		{from: 0, to: 1, dir: 3, speed: 45, access: all, name: "Western Express Highway"},
		{from: 1, to: 2, dir: 1, speed: 30, access: all, flags: flagLink},
		{from: 2, to: 3, dir: 1, speed: 20, access: all, flags: flagRoundabout},
		{from: 3, to: 4, dir: 3, speed: 30, access: all, name: "Airport Road"},
	})
	r := route(t, g, [2]float64{72.801, 19.0}, [2]float64{72.829, 19.0}, Options{Mode: Car})
	types := []string{}
	for _, s := range r.Steps {
		types = append(types, s.Type)
	}
	if got := strings.Join(types, ","); got != "depart,ramp,roundabout,exit_roundabout,arrive" {
		t.Fatalf("step types %s (%s)", got, instructions(r.Steps))
	}
	if r.Steps[1].Instruction != "Take the ramp on the left" || r.Steps[3].Instruction != "Exit the roundabout onto Airport Road" {
		t.Errorf("instructions: %s", instructions(r.Steps))
	}
}

func TestModifierAndCardinal(t *testing.T) {
	cases := map[float64]string{0: "straight", 19: "straight", -30: "slight_left", 30: "slight_right", 90: "right",
		-90: "left", 150: "sharp_right", -150: "sharp_left", 175: "uturn", -179: "uturn", math.NaN(): "straight"}
	for angle, want := range cases {
		if got := modifier(angle); got != want {
			t.Errorf("modifier(%v) = %s, want %s", angle, got, want)
		}
	}
	if got := turnAngle(350, 10); got != 20 {
		t.Errorf("turnAngle(350, 10) = %v", got)
	}
	if got := turnAngle(10, 350); got != -20 {
		t.Errorf("turnAngle(10, 350) = %v", got)
	}
	for b, want := range map[float64]string{0: "north", 44: "north-east", 90: "east", 181: "south", 270: "west", 330: "north-west", 338: "north", 359: "north"} {
		if got := cardinal(b); got != want {
			t.Errorf("cardinal(%v) = %s, want %s", b, got, want)
		}
	}
}
