package routing

import (
	"container/heap"
	"math"
)

// Table holds the times and distances between every source and destination of a matrix; nil where a destination
// cannot be reached (or a point is not near a usable road).
type Table struct {
	Durations    [][]*float64 // seconds, [source][destination]
	Distances    [][]*float64 // metres
	Sources      []*Snap      // nil for a point with no usable road within MaxSnapM
	Destinations []*Snap
}

// Table computes the travel time and distance from each source to each destination (points as lon, lat), with the
// same costs as Route: one search per source that stops once every destination is settled. The best route is the
// fastest (or, with Shortest, the shortest); the other figure is that route's.
func (g *Graph) Table(sources, destinations [][2]float64, opts Options) (*Table, error) {
	if opts.Mode == "" {
		opts.Mode = Car
	}
	p, err := g.profile(opts.Mode, opts.AvoidTolls)
	if err != nil {
		return nil, err
	}
	snap := func(points [][2]float64) []*Snap {
		out := make([]*Snap, len(points))
		for i, pt := range points {
			if s, err := g.nearest(pt[0], pt[1], p); err == nil {
				out[i] = &s
			}
		}
		return out
	}
	t := &Table{Sources: snap(sources), Destinations: snap(destinations)}
	t.Durations = make([][]*float64, len(sources))
	t.Distances = make([][]*float64, len(sources))

	// Primary cost (what is minimised) and secondary (reported alongside) per segment and per edge.
	segTime := func(s int32) float64 { return float64(p.segTime[s]) }
	segLen := func(s int32) float64 { return float64(g.segLen[s]) }
	primary, secondary := segTime, segLen
	if opts.Optimize == Shortest {
		primary, secondary = segLen, segTime
	}
	fwd := func(s int32) bool { return g.allowedDirection(opts.Mode, s, true) }
	back := func(s int32) bool { return g.allowedDirection(opts.Mode, s, false) }

	// The vertices a search must settle: both ends of every destination's segment.
	targets := map[int32]bool{}
	for _, d := range t.Destinations {
		if d != nil {
			targets[g.segFrom[d.seg]] = true
			targets[g.segTo[d.seg]] = true
		}
	}

	for i, from := range t.Sources {
		t.Durations[i] = make([]*float64, len(destinations))
		t.Distances[i] = make([]*float64, len(destinations))
		if from == nil {
			continue
		}
		sr := g.acquire()
		other := map[int32]float64{} // secondary cost to each settled target
		aux := map[int32]float64{}   // secondary cost along the best path to each vertex queued
		seed := func(v int32, c, c2 float64) {
			if c < sr.cost(v) {
				sr.set(v, c, -1)
				aux[v] = c2
				heap.Push(&sr.queue, item{v, c})
			}
		}
		fs := from.seg
		if fwd(fs) {
			seed(g.segTo[fs], (1-from.frac)*primary(fs), (1-from.frac)*secondary(fs))
		}
		if back(fs) {
			seed(g.segFrom[fs], from.frac*primary(fs), from.frac*secondary(fs))
		}
		left := len(targets)
		for sr.queue.Len() > 0 && left > 0 {
			it := heap.Pop(&sr.queue).(item)
			v := it.v
			gv := sr.cost(v)
			if it.f > gv {
				continue
			}
			if targets[v] {
				if _, done := other[v]; !done {
					other[v] = aux[v]
					left--
				}
			}
			for e := g.offsets[v]; e < g.offsets[v+1]; e++ {
				if math.IsInf(float64(p.edgeTime[e]), 1) {
					continue
				}
				s := segOf(g.edgeSeg[e])
				w := g.edgeTo[e]
				if c := gv + primary(s); c < sr.cost(w) {
					sr.set(w, c, e)
					aux[w] = aux[v] + secondary(s)
					heap.Push(&sr.queue, item{w, c})
				}
			}
		}

		for j, to := range t.Destinations {
			if to == nil {
				continue
			}
			ts := to.seg
			best, best2 := math.Inf(1), 0.0
			try := func(c, c2 float64) {
				if c < best {
					best, best2 = c, c2
				}
			}
			if v := g.segFrom[ts]; fwd(ts) {
				if c2, ok := other[v]; ok {
					try(sr.cost(v)+to.frac*primary(ts), c2+to.frac*secondary(ts))
				}
			}
			if v := g.segTo[ts]; back(ts) {
				if c2, ok := other[v]; ok {
					try(sr.cost(v)+(1-to.frac)*primary(ts), c2+(1-to.frac)*secondary(ts))
				}
			}
			// Both on one segment, in a direction it may be travelled.
			if ts == fs && (fwd(fs) && from.frac <= to.frac || back(fs) && from.frac >= to.frac) {
				f := math.Abs(to.frac - from.frac)
				try(f*primary(fs), f*secondary(fs))
			}
			if math.IsInf(best, 1) {
				continue
			}
			duration, distance := best, best2
			if opts.Optimize == Shortest {
				duration, distance = best2, best
			}
			t.Durations[i][j], t.Distances[i][j] = &duration, &distance
		}
		searches.Put(sr)
	}
	return t, nil
}
