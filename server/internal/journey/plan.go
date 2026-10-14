package journey

import (
	"container/heap"
	"errors"
	"math"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// Errors returned by Plan.
var (
	ErrNoStation = errors.New("no station within walking distance")
	ErrNoJourney = errors.New("no journey connects the points")
)

// Options tune a journey search.
type Options struct {
	MaxWalkM float64         // straight-line limit on the walks to the first station and from the last
	Modes    map[string]bool // modes to ride; nil for all
}

// Leg is part of a journey: a walk, or a ride on one route.
type Leg struct {
	Ride bool
	// From and To are stations; −1 is the journey's origin or destination.
	From, To int
	// Route, FromStop and ToStop locate a ride: the route and the indexes of its stops where it starts and ends.
	Route            int
	FromStop, ToStop int
	DistanceM        float64
	DurationS        float64 // walking or riding time
	WaitS            float64 // rides: waiting for the train, and changing platforms after another ride
}

// Journey is a planned trip.
type Journey struct {
	Legs      []Leg
	DurationS float64
}

// Plan finds the quickest journey between two points. A destination within walking distance is reached on foot when
// that is quicker than any train.
func (n *Network) Plan(from, to geo.Point, opt Options) (*Journey, error) {
	direct := geo.Haversine(from, to)
	walkOnly := &Journey{Legs: []Leg{{From: -1, To: -1, DistanceM: direct * detour, DurationS: walkTime(direct)}},
		DurationS: walkTime(direct)}
	allowed := func(r *route) bool { return opt.Modes == nil || opt.Modes[r.Line.Mode] }

	// The walk to each station within reach of the destination.
	ends := map[int]float64{}
	for i, st := range n.Stations {
		if d := geo.Haversine(st.Point(), to); d <= opt.MaxWalkM {
			ends[i] = d
		}
	}

	origin := int32(len(n.adj))
	cost := make([]float64, origin+1)
	prev := make([]int32, origin+1)
	via := make([]edgeKind, origin+1)
	for i := range cost {
		cost[i], prev[i] = math.Inf(1), -1
	}
	cost[origin] = 0
	var q queue
	starts := false
	for ri := range n.routes {
		r := &n.routes[ri]
		if !allowed(r) {
			continue
		}
		for i := 0; i+1 < len(r.station); i++ {
			d := geo.Haversine(n.Stations[r.station[i]].Point(), from)
			if d > opt.MaxWalkM {
				continue
			}
			starts = true
			p := r.first + int32(i)
			if c := walkTime(d) + r.wait; c < cost[p] {
				cost[p], prev[p], via[p] = c, origin, boardEdge
				heap.Push(&q, item{p, c})
			}
		}
	}
	if !starts || len(ends) == 0 {
		if direct <= opt.MaxWalkM {
			return walkOnly, nil
		}
		return nil, ErrNoStation
	}

	stations := int32(len(n.Stations))
	best, bestCost := int32(-1), math.Inf(1)
	for q.Len() > 0 {
		it := heap.Pop(&q).(item)
		if it.cost > cost[it.node] {
			continue
		}
		if it.cost >= bestCost {
			break
		}
		if d, ok := ends[int(it.node)]; ok && it.node < stations {
			if total := it.cost + walkTime(d); total < bestCost {
				best, bestCost = it.node, total
			}
		}
		for _, e := range n.adj[it.node] {
			if e.kind == boardEdge && !allowed(&n.routes[n.routeOf[e.to-stations]]) {
				continue
			}
			if e.kind == alightEdge && via[it.node] == boardEdge {
				continue // getting off where you got on
			}
			if c := it.cost + float64(e.cost); c < cost[e.to] {
				cost[e.to], prev[e.to], via[e.to] = c, it.node, e.kind
				heap.Push(&q, item{e.to, c})
			}
		}
	}
	if best < 0 {
		if direct <= opt.MaxWalkM {
			return walkOnly, nil
		}
		return nil, ErrNoJourney
	}
	if direct <= opt.MaxWalkM && walkOnly.DurationS <= bestCost {
		return walkOnly, nil
	}

	var path []int32
	for v := best; v != origin; v = prev[v] {
		path = append(path, v)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return n.legs(path, via, from, to, ends[int(best)]), nil
}

// legs turns a node path (from the first boarding to the last station) into walks and rides.
func (n *Network) legs(path []int32, via []edgeKind, from, to geo.Point, lastWalkM float64) *Journey {
	stations := int32(len(n.Stations))
	j := &Journey{}
	add := func(l Leg) {
		j.Legs = append(j.Legs, l)
		j.DurationS += l.DurationS + l.WaitS
	}
	var ride *Leg
	for k, v := range path {
		switch via[v] {
		case boardEdge:
			r := &n.routes[n.routeOf[v-stations]]
			stop := int(v - r.first)
			if k == 0 {
				d := geo.Haversine(from, n.Stations[r.station[stop]].Point())
				add(Leg{From: -1, To: r.station[stop], DistanceM: d * detour, DurationS: walkTime(d)})
			}
			wait := r.wait
			if k > 0 {
				wait += platformS
			}
			ride = &Leg{Ride: true, Route: int(n.routeOf[v-stations]), From: r.station[stop], FromStop: stop, ToStop: stop,
				WaitS: wait}
		case rideEdge:
			r := &n.routes[ride.Route]
			ride.ToStop = int(v - r.first)
			ride.DurationS += r.hop[ride.ToStop-1]
		case alightEdge:
			r := &n.routes[ride.Route]
			ride.To = r.station[ride.ToStop]
			ride.DistanceM = r.at[ride.ToStop] - r.at[ride.FromStop]
			add(*ride)
			ride = nil
		case changeEdge:
			a, b := n.Stations[path[k-1]], n.Stations[v]
			d := geo.Haversine(a.Point(), b.Point())
			add(Leg{From: int(path[k-1]), To: int(v), DistanceM: d * detour, DurationS: walkTime(d) + exitS})
		}
	}
	last := int(path[len(path)-1])
	add(Leg{From: last, To: -1, DistanceM: lastWalkM * detour, DurationS: walkTime(lastWalkM)})
	return j
}

type item struct {
	node int32
	cost float64
}

type queue []item

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)        { *q = append(*q, x.(item)) }
func (q *queue) Pop() any {
	old := *q
	it := old[len(old)-1]
	*q = old[:len(old)-1]
	return it
}
