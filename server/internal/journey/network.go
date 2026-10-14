// Package journey plans trips on the suburban railway, metro and monorail: a walk to a station, rides with changes
// of line, and a walk on to the destination.
//
// The network is built from the transit lines in the database. Stops of one mode with the same name are one station;
// stations close to each other are joined by short walks for changing lines. A ride takes the distance along the
// line at the mode's running speed plus an allowance for every stop, and each boarding waits half the line's usual
// gap between trains. Journeys are therefore estimates from typical frequencies, not timetables.
package journey

import (
	"math"
	"strings"
	"unicode"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

const (
	mergeM    = 300.0 // stops of one mode with the same name closer than this are one station
	transferM = 450.0 // the furthest walk between two stations to change lines
	walkSpeed = 1.25  // m/s, about 4.5 km/h
	detour    = 1.3   // metres walked per metre of straight line
	platformS = 90.0  // changing platforms within a station
	exitS     = 60.0  // leaving one station and entering another when changing lines
	snapM     = 250.0 // the furthest a stop may lie from its line's track for rides to follow the track
)

// speed is how a mode runs: cruising speed between stations (m/s) and the time each stop adds for braking, dwelling
// and accelerating (s).
type speed struct{ cruise, stopS float64 }

var speeds = map[string]speed{
	"train":      {55 / 3.6, 75},
	"subway":     {50 / 3.6, 35},
	"light_rail": {40 / 3.6, 35},
	"monorail":   {30 / 3.6, 45},
	"tram":       {18 / 3.6, 30},
}

// Modes are the transit modes the planner rides.
var Modes = []string{"train", "subway", "light_rail", "monorail", "tram"}

// DefaultHeadways are the gaps between trains (minutes) for datasets that do not record them.
var DefaultHeadways = map[string]float64{"train": 10, "subway": 8, "light_rail": 8, "monorail": 18, "tram": 10}

// Line is a transit line as the API presents it.
type Line struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Ref     string `json:"ref,omitempty"`
	Mode    string `json:"mode"`
	Network string `json:"network,omitempty"`
	Colour  string `json:"colour,omitempty"`
}

// Stop is a stop of a route as stored with its line.
type Stop struct {
	ID       int64
	Name     string
	Lon, Lat float64
}

// Route is one service pattern of a line in one direction: its stops in order and, when known, its track.
type Route struct {
	ID         int64
	Line       Line
	Headsign   string         // where the trains are going
	Stops      []Stop         // in running order
	Track      [][][2]float64 // parts of the track; each may run either way
	HeadwayMin float64        // usual gap between trains; 0 for the mode's default
}

// Station is where one or more routes of the same mode stop.
type Station struct {
	Name     string
	Lon, Lat float64
	Mode     string
	Place    int64    // the station's place id, 0 when unknown
	Lines    []string // keys of the lines that call here
}

// Point returns the station's location.
func (s Station) Point() geo.Point { return geo.Point{Lon: s.Lon, Lat: s.Lat} }

type route struct {
	Route
	line    int
	express bool
	station []int        // station of each stop
	at      []float64    // metres along track (or along the chain of stops) of each stop
	track   [][2]float64 // the track part the stops lie on, oriented with them; nil when the stops are joined directly
	trackAt []float64    // metres along track of each of its points
	hop     []float64    // seconds from each stop to the next
	wait    float64      // seconds waited at a boarding
	first   int32        // node of the first stop
}

type edgeKind uint8

const (
	rideEdge   edgeKind = iota // to the next stop of a route
	alightEdge                 // from a route's stop to its station
	boardEdge                  // from a station onto a route
	changeEdge                 // walking between stations
)

type edge struct {
	to   int32
	kind edgeKind
	cost float32
}

// Network is the station graph; safe for concurrent use once built.
type Network struct {
	Lines    []Line
	Stations []Station
	routes   []route
	adj      [][]edge // nodes: stations, then one per stop of every route
	routeOf  []int32  // route of each stop node (index by node − len(Stations))
}

// Build assembles the network from routes. Routes of unknown modes or with fewer than two stations are skipped.
func Build(input []Route) *Network {
	n := &Network{}
	lineIndex := map[string]int{}
	byName := map[string][]int{}
	byID := map[int64]int{}
	for _, in := range input {
		sp, ok := speeds[in.Line.Mode]
		if !ok {
			continue
		}
		r := route{Route: in}
		r.Stops = nil
		for _, s := range in.Stops {
			st := n.station(s, in.Line, byName, byID)
			if k := len(r.station); k > 0 && r.station[k-1] == st {
				continue
			}
			r.Stops = append(r.Stops, s)
			r.station = append(r.station, st)
		}
		if len(r.station) < 2 {
			continue
		}
		li, ok := lineIndex[in.Line.Key]
		if !ok {
			li = len(n.Lines)
			lineIndex[in.Line.Key] = li
			n.Lines = append(n.Lines, in.Line)
		}
		r.line = li
		r.orient()
		for i := 0; i+1 < len(r.at); i++ {
			straight := geo.Haversine(stopPoint(r.Stops[i]), stopPoint(r.Stops[i+1]))
			r.hop = append(r.hop, math.Max(r.at[i+1]-r.at[i], straight)/sp.cruise+sp.stopS)
		}
		for _, st := range r.station {
			n.Stations[st].addLine(in.Line.Key)
		}
		n.routes = append(n.routes, r)
	}
	n.setWaits()
	n.link()
	return n
}

// station finds or creates the station for a stop: the same stop id, or the same name and mode close by.
func (n *Network) station(s Stop, line Line, byName map[string][]int, byID map[int64]int) int {
	if i, ok := byID[s.ID]; ok && s.ID != 0 && n.Stations[i].Mode == line.Mode {
		return i
	}
	key := line.Mode + "|" + fold(s.Name)
	if fold(s.Name) != "" {
		for _, i := range byName[key] {
			if geo.Haversine(n.Stations[i].Point(), stopPoint(s)) <= mergeM {
				byID[s.ID] = i
				return i
			}
		}
	}
	i := len(n.Stations)
	n.Stations = append(n.Stations, Station{Name: s.Name, Lon: s.Lon, Lat: s.Lat, Mode: line.Mode})
	byName[key] = append(byName[key], i)
	byID[s.ID] = i
	return i
}

func (s *Station) addLine(key string) {
	for _, k := range s.Lines {
		if k == key {
			return
		}
	}
	s.Lines = append(s.Lines, key)
}

// dropWords are left out when comparing station names ("Kalyan Junction" is "Kalyan").
var dropWords = map[string]bool{"station": true, "railway": true, "rly": true, "metro": true, "monorail": true,
	"junction": true, "jn": true, "suburban": true}

// fold reduces a station name to the words that identify it.
func fold(name string) string {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if !dropWords[w] {
			words = append(words, w)
		}
	}
	return strings.Join(words, " ")
}

// orient places the stops along the track part they all lie on, turning it to run with them. Without such a part
// (a stop far from the track, or a track that doubles back) the stops are joined directly.
func (r *route) orient() {
	best, bestOff := -1, math.Inf(1)
	var bestAt []float64
	reverse := false
	for pi, part := range r.Track {
		if len(part) < 2 {
			continue
		}
		cum := cumulative(part)
		at := make([]float64, len(r.Stops))
		off, ok := 0.0, true
		for i, s := range r.Stops {
			a, d := project(part, cum, stopPoint(s))
			if d > snapM {
				ok = false
				break
			}
			at[i], off = a, off+d
		}
		if !ok {
			continue
		}
		fwd, back := monotonic(at, 1), monotonic(at, -1)
		if (fwd || back) && off < bestOff {
			best, bestOff, bestAt, reverse = pi, off, at, !fwd
		}
	}
	if best < 0 {
		r.at = make([]float64, len(r.Stops))
		for i := 1; i < len(r.Stops); i++ {
			r.at[i] = r.at[i-1] + geo.Haversine(stopPoint(r.Stops[i-1]), stopPoint(r.Stops[i]))
		}
		return
	}
	track := r.Track[best]
	if reverse {
		length := cumulative(track)[len(track)-1]
		track = reversed(track)
		for i := range bestAt {
			bestAt[i] = length - bestAt[i]
		}
	}
	r.track, r.trackAt, r.at = track, cumulative(track), bestAt
	for i := 1; i < len(r.at); i++ {
		r.at[i] = math.Max(r.at[i], r.at[i-1]) // small projection jitter
	}
}

// setWaits marks express patterns (a line's route calling at far fewer stations over about the same distance) and
// sets the wait at boarding: half the gap between trains, which is twice as long for express patterns.
func (n *Network) setWaits() {
	for i := range n.routes {
		r := &n.routes[i]
		length := r.at[len(r.at)-1] - r.at[0]
		for j := range n.routes {
			o := &n.routes[j]
			oLength := o.at[len(o.at)-1] - o.at[0]
			if o.line == r.line && float64(len(o.station)) >= 1.4*float64(len(r.station)) &&
				math.Abs(oLength-length) <= 0.15*oLength {
				r.express = true
				break
			}
		}
		gap := r.HeadwayMin
		if gap <= 0 {
			gap = DefaultHeadways[r.Line.Mode]
		}
		if r.express {
			gap *= 2
		}
		r.wait = gap * 60 / 2
	}
}

// link numbers the nodes and builds the edges: rides between consecutive stops, alighting and boarding at stations,
// and walks between stations close enough to change lines.
func (n *Network) link() {
	next := int32(len(n.Stations))
	for i := range n.routes {
		n.routes[i].first = next
		next += int32(len(n.routes[i].station))
		for range n.routes[i].station {
			n.routeOf = append(n.routeOf, int32(i))
		}
	}
	n.adj = make([][]edge, next)
	for _, r := range n.routes {
		for i, st := range r.station {
			p := r.first + int32(i)
			if i+1 < len(r.station) {
				n.adj[p] = append(n.adj[p], edge{p + 1, rideEdge, float32(r.hop[i])})
				n.adj[st] = append(n.adj[st], edge{p, boardEdge, float32(r.wait + platformS)})
			}
			if i > 0 {
				n.adj[p] = append(n.adj[p], edge{int32(st), alightEdge, 0})
			}
		}
	}
	for a := range n.Stations {
		for b := a + 1; b < len(n.Stations); b++ {
			d := geo.Haversine(n.Stations[a].Point(), n.Stations[b].Point())
			if d > transferM {
				continue
			}
			cost := float32(walkTime(d) + exitS)
			n.adj[a] = append(n.adj[a], edge{int32(b), changeEdge, cost})
			n.adj[b] = append(n.adj[b], edge{int32(a), changeEdge, cost})
		}
	}
}

// walkTime estimates the time to walk between two points the given straight-line distance apart.
func walkTime(straightM float64) float64 { return straightM * detour / walkSpeed }

func stopPoint(s Stop) geo.Point { return geo.Point{Lon: s.Lon, Lat: s.Lat} }

func cumulative(line [][2]float64) []float64 {
	cum := make([]float64, len(line))
	for i := 1; i < len(line); i++ {
		cum[i] = cum[i-1] + geo.Haversine(geo.Point{Lon: line[i-1][0], Lat: line[i-1][1]},
			geo.Point{Lon: line[i][0], Lat: line[i][1]})
	}
	return cum
}

// project returns how far along line (with cumulative lengths cum) the point nearest p lies, and its distance from p.
// It projects locally (equirectangular around p), which is accurate at city scale.
func project(line [][2]float64, cum []float64, p geo.Point) (along, dist float64) {
	kx := 111320.0 * math.Cos(p.Lat*math.Pi/180)
	const ky = 110574.0
	dist = math.Inf(1)
	for i := 0; i+1 < len(line); i++ {
		ax, ay := (line[i][0]-p.Lon)*kx, (line[i][1]-p.Lat)*ky
		bx, by := (line[i+1][0]-p.Lon)*kx, (line[i+1][1]-p.Lat)*ky
		dx, dy := bx-ax, by-ay
		t := 0.0
		if l2 := dx*dx + dy*dy; l2 > 0 {
			t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l2))
		}
		if d := math.Hypot(ax+t*dx, ay+t*dy); d < dist {
			dist, along = d, cum[i]+t*(cum[i+1]-cum[i])
		}
	}
	return along, dist
}

// monotonic reports whether values run in one direction (sign 1 up, −1 down), allowing 30 m of jitter.
func monotonic(values []float64, sign float64) bool {
	for i := 1; i < len(values); i++ {
		if sign*(values[i]-values[i-1]) < -30 {
			return false
		}
	}
	return true
}

func reversed(line [][2]float64) [][2]float64 {
	out := make([][2]float64, len(line))
	for i, c := range line {
		out[len(line)-1-i] = c
	}
	return out
}
