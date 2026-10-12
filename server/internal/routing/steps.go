package routing

import (
	"math"
	"strings"
)

// Step is one instruction of turn-by-turn directions. Type and Modifier are stable codes for clients that render
// their own (or translated) instructions; Instruction is ready-made English.
type Step struct {
	// Type is depart, turn, continue, ramp, roundabout, exit_roundabout or arrive.
	Type string `json:"type"`
	// Modifier is straight, slight_left, left, sharp_left, slight_right, right, sharp_right or uturn.
	Modifier string `json:"modifier,omitempty"`
	// Name is the road or path travelled after the manoeuvre, empty when unnamed.
	Name        string     `json:"name,omitempty"`
	DistanceM   float64    `json:"distance_m"`
	DurationS   float64    `json:"duration_s"`
	Location    [2]float64 `json:"location"` // lon, lat of the manoeuvre
	Bearing     int        `json:"bearing"`  // heading after the manoeuvre, degrees clockwise from north
	Instruction string     `json:"instruction"`
}

// stepMinM is how far along a leg bearings are measured, smoothing out tiny kinks at junctions.
const stepMinM = 12

// A stretch is a run of legs travelled without a manoeuvre: the same road, or unnamed ways without a real turn.
type stretch struct {
	first, last int
	name        string // road name, "" when unnamed
	label       string // name, or a description of an unnamed path ("the foot over-bridge")
	kind        string // "roundabout", "ramp" or ""
}

// steps turns a route's legs into turn-by-turn directions.
func (g *Graph) steps(res *Result) []Step {
	legs := res.legs
	arrive := Step{Type: "arrive", Location: [2]float64{round6(res.To.Lon), round6(res.To.Lat)}, Instruction: "Arrive at your destination"}
	if len(legs) == 0 {
		return []Step{arrive}
	}
	var stretches []stretch
	for i, l := range legs {
		name := g.name(l.seg)
		label := name
		if label == "" {
			label = g.describe(l.seg)
		}
		kind := ""
		switch flags := g.segFlags[l.seg]; {
		case flags&flagRoundabout != 0:
			kind = "roundabout"
		case flags&flagLink != 0:
			kind = "ramp"
		}
		if n := len(stretches); n > 0 {
			cur := &stretches[n-1]
			turn := turnAngle(exitBearing(res.Path, legs[i-1]), entryBearing(res.Path, l))
			same := kind == cur.kind && label == cur.label && (name != "" || math.Abs(turn) < 50)
			if kind == "roundabout" && cur.kind == "roundabout" {
				same = true // keep going round, whatever the segments are called
			}
			if same {
				cur.last = i
				continue
			}
		}
		stretches = append(stretches, stretch{first: i, last: i, name: name, label: label, kind: kind})
	}

	steps := make([]Step, 0, len(stretches)+1)
	for k, st := range stretches {
		first := legs[st.first]
		bearing := entryBearing(res.Path, first)
		step := Step{
			Name:     st.name,
			Location: [2]float64{round6(res.Path[first.pathStart][0]), round6(res.Path[first.pathStart][1])},
			Bearing:  int(math.Round(math.Mod(bearing+360, 360))),
		}
		if math.IsNaN(bearing) {
			step.Bearing = 0
		}
		for i := st.first; i <= st.last; i++ {
			step.DistanceM += legs[i].distanceM
			step.DurationS += legs[i].durationS
		}
		step.DistanceM, step.DurationS = math.Round(step.DistanceM), math.Round(step.DurationS)
		if k == 0 {
			step.Type = "depart"
			step.Instruction = "Head " + cardinal(bearing) + along(st.label)
			steps = append(steps, step)
			continue
		}
		prev := stretches[k-1]
		angle := turnAngle(exitBearing(res.Path, legs[prev.last]), bearing)
		step.Modifier = modifier(angle)
		switch {
		case st.kind == "roundabout":
			step.Type = "roundabout"
			step.Instruction = "Enter the roundabout"
		case prev.kind == "roundabout":
			step.Type = "exit_roundabout"
			step.Instruction = "Exit the roundabout" + onto(st.label)
		case st.kind == "ramp" && prev.kind != "ramp":
			step.Type = "ramp"
			side := "right"
			if angle < 0 {
				side = "left"
			}
			step.Instruction = "Take the ramp on the " + side + onto(st.label)
		case step.Modifier == "straight":
			step.Type = "continue"
			step.Instruction = continueText(st.label)
		default:
			step.Type = "turn"
			step.Instruction = turnText(step.Modifier, st.label)
		}
		steps = append(steps, step)
	}
	return append(steps, arrive)
}

// describe names an unnamed path for directions ("Cross the foot over-bridge"); unnamed roads stay unnamed.
func (g *Graph) describe(s int32) string {
	flags := g.segFlags[s]
	switch g.segClass[s] {
	case 10:
		switch {
		case flags&flagSteps != 0:
			return "the steps"
		case flags&flagBridge != 0:
			return "the foot over-bridge"
		case flags&flagTunnel != 0:
			return "the subway"
		}
		return "the footpath"
	case 8:
		return "the pedestrian street"
	case 9:
		return "the track"
	}
	return ""
}

func along(label string) string {
	switch {
	case label == "":
		return ""
	case strings.HasPrefix(label, "the "):
		return " along " + label
	}
	return " on " + label
}

func onto(label string) string {
	if label == "" {
		return ""
	}
	return " onto " + label
}

func continueText(label string) string {
	switch label {
	case "":
		return "Continue straight"
	case "the steps":
		return "Take the steps"
	case "the foot over-bridge":
		return "Cross the foot over-bridge"
	case "the subway":
		return "Go through the subway"
	}
	return "Continue onto " + label
}

func turnText(mod, label string) string {
	if mod == "uturn" {
		return "Make a U-turn" + onto(label)
	}
	text := "Turn " + strings.ReplaceAll(mod, "_", " ")
	switch label {
	case "the steps":
		return text + " and take the steps"
	case "the foot over-bridge":
		return text + " and cross the foot over-bridge"
	}
	return text + onto(label)
}

// modifier classifies a turn angle (degrees, positive to the right).
func modifier(angle float64) string {
	if math.IsNaN(angle) {
		return "straight"
	}
	a := math.Abs(angle)
	side := "right"
	if angle < 0 {
		side = "left"
	}
	switch {
	case a < 20:
		return "straight"
	case a < 45:
		return "slight_" + side
	case a < 135:
		return side
	case a < 170:
		return "sharp_" + side
	}
	return "uturn"
}

// turnAngle is the change of heading from in to out, in (-180, 180]; NaN when either is unknown.
func turnAngle(in, out float64) float64 {
	d := math.Mod(out-in+540, 360) - 180
	if d == -180 {
		d = 180
	}
	return d
}

func cardinal(bearing float64) string {
	if math.IsNaN(bearing) {
		return "off"
	}
	names := [...]string{"north", "north-east", "east", "south-east", "south", "south-west", "west", "north-west"}
	return names[int(math.Round(math.Mod(bearing+360, 360)/45))%8]
}

// entryBearing is the heading at the start of a leg, measured over its first stepMinM metres.
func entryBearing(path [][2]float64, l leg) float64 {
	start := path[l.pathStart]
	for j := l.pathStart + 1; j <= l.pathEnd; j++ {
		if haversine(start[0], start[1], path[j][0], path[j][1]) >= stepMinM || j == l.pathEnd {
			return bearing(start, path[j])
		}
	}
	return math.NaN()
}

// exitBearing is the heading at the end of a leg, measured over its last stepMinM metres.
func exitBearing(path [][2]float64, l leg) float64 {
	end := path[l.pathEnd]
	for j := l.pathEnd - 1; j >= l.pathStart; j-- {
		if haversine(path[j][0], path[j][1], end[0], end[1]) >= stepMinM || j == l.pathStart {
			return bearing(path[j], end)
		}
	}
	return math.NaN()
}

// bearing is the initial great-circle heading from a to b in degrees clockwise from north.
func bearing(a, b [2]float64) float64 {
	if a == b {
		return math.NaN()
	}
	lat1, lat2 := a[1]*math.Pi/180, b[1]*math.Pi/180
	dLon := (b[0] - a[0]) * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
