package routing

import (
	"errors"
	"math"
)

// Mode is a way of travelling.
type Mode string

// Travel modes.
const (
	Car        Mode = "car"
	TwoWheeler Mode = "two_wheeler"
	Foot       Mode = "foot"
)

// Modes lists every travel mode in display order.
var Modes = []Mode{Car, TwoWheeler, Foot}

// ErrModeUnavailable means the graph was built without the data a mode needs (version 1 graphs are cars only).
var ErrModeUnavailable = errors.New("this travel mode is not available in this dataset")

// Access bits in seg_access and flag bits in seg_flags (see docs/database.md).
const (
	modeCar        uint8 = 1
	modeTwoWheeler uint8 = 2
	modeFoot       uint8 = 4

	flagToll       uint8 = 1
	flagSteps      uint8 = 2
	flagBridge     uint8 = 4
	flagTunnel     uint8 = 8
	flagRoundabout uint8 = 16
	flagLink       uint8 = 32
)

// Speeds that are not in the graph. Walking is a steady 4.8 km/h, half that on steps. Two-wheelers filter through
// Mumbai traffic faster than cars, so they get the car speed plus a fifth, up to 60 km/h.
const (
	walkSpeed        = 4.8 / 3.6 // m/s
	stepsSpeed       = walkSpeed / 2
	twoWheelerFactor = 1.2
	twoWheelerMax    = 60 / 3.6
)

func (m Mode) bit() uint8 {
	switch m {
	case Car:
		return modeCar
	case TwoWheeler:
		return modeTwoWheeler
	case Foot:
		return modeFoot
	}
	return 0
}

// Valid reports whether m is a known mode.
func (m Mode) Valid() bool { return m.bit() != 0 }

// Modes returns the travel modes this graph supports.
func (g *Graph) Modes() []Mode {
	if g.version < 2 {
		return []Mode{Car}
	}
	return Modes
}

func (g *Graph) supports(m Mode) bool {
	for _, x := range g.Modes() {
		if x == m {
			return true
		}
	}
	return false
}

// profile holds the travel times of one mode (and toll preference) for every segment and edge, computed once per
// graph: +Inf marks what the mode may not use.
type profile struct {
	mode      Mode
	avoidToll bool
	segTime   []float32 // seconds to traverse a segment
	edgeTime  []float32 // seconds along an edge, +Inf where the mode may not go that way
	maxSpeed  float64   // m/s over the fastest usable segment, for an admissible A* heuristic
}

type profileKey struct {
	mode      Mode
	avoidToll bool
}

// profile returns the cost profile for a mode, building it on first use.
func (g *Graph) profile(mode Mode, avoidToll bool) (*profile, error) {
	if !g.supports(mode) {
		return nil, ErrModeUnavailable
	}
	key := profileKey{mode, avoidToll}
	if p, ok := g.profiles.Load(key); ok {
		return p.(*profile), nil
	}
	p := g.buildProfile(mode, avoidToll)
	actual, _ := g.profiles.LoadOrStore(key, p)
	return actual.(*profile), nil
}

func (g *Graph) buildProfile(mode Mode, avoidToll bool) *profile {
	inf := float32(math.Inf(1))
	p := &profile{mode: mode, avoidToll: avoidToll, segTime: make([]float32, len(g.segLen)), edgeTime: make([]float32, len(g.edgeTo))}
	bit := mode.bit()
	for s := range g.segLen {
		speed := g.speed(mode, int32(s))
		if g.segAccess[s]&bit == 0 || speed <= 0 || avoidToll && g.segFlags[s]&flagToll != 0 {
			p.segTime[s] = inf
			continue
		}
		p.segTime[s] = g.segLen[s] / float32(speed)
		p.maxSpeed = math.Max(p.maxSpeed, speed)
	}
	for e := range g.edgeTo {
		s := g.edgeSeg[e]
		forward := s >= 0
		if !forward {
			s = ^s
		}
		if g.allowedDirection(mode, s, forward) {
			p.edgeTime[e] = p.segTime[s]
		} else {
			p.edgeTime[e] = inf
		}
	}
	if p.maxSpeed == 0 {
		p.maxSpeed = walkSpeed
	}
	return p
}

// speed is the mode's speed on a segment in m/s (0 when unknown).
func (g *Graph) speed(mode Mode, s int32) float64 {
	switch mode {
	case Foot:
		if g.segFlags[s]&flagSteps != 0 {
			return stepsSpeed
		}
		return walkSpeed
	case TwoWheeler:
		return math.Min(float64(g.segSpeed[s])/3.6*twoWheelerFactor, math.Max(twoWheelerMax, float64(g.segSpeed[s])/3.6))
	}
	return float64(g.segSpeed[s]) / 3.6
}

// allowedDirection reports whether a mode may travel a segment along (forward) or against its shape. Pedestrians
// ignore one-way rules.
func (g *Graph) allowedDirection(mode Mode, s int32, forward bool) bool {
	if g.segAccess[s]&mode.bit() == 0 {
		return false
	}
	if mode == Foot {
		return true
	}
	if forward {
		return g.segDir[s]&1 != 0
	}
	return g.segDir[s]&2 != 0
}

func (p *profile) usable(s int32) bool { return !math.IsInf(float64(p.segTime[s]), 1) }
