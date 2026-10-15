package routing

import (
	"container/heap"
	"math"
)

// Reach explores everything reachable from a point within limitS seconds by mode, with the same costs and one-way
// rules as Route (Dijkstra without a target), for isochrones. visit is called with every point of the network reached
// — vertices and the shape points of the segments travelled, plus the point where time runs out on a segment only
// partly reached — and the travel time to it; a point may be visited more than once. It returns the snapped start and
// the number of vertices settled.
func (g *Graph) Reach(lon, lat float64, mode Mode, limitS float64, visit func(lon, lat, seconds float64)) (Snap, int, error) {
	if mode == "" {
		mode = Car
	}
	p, err := g.profile(mode, false)
	if err != nil {
		return Snap{}, 0, err
	}
	from, err := g.nearest(lon, lat, p)
	if err != nil {
		return Snap{}, 0, err
	}
	sr := g.acquire()
	defer searches.Put(sr)
	seed := func(v int32, c float64) {
		if c <= limitS && c < sr.cost(v) {
			sr.set(v, c, -1)
			heap.Push(&sr.queue, item{v, c})
		}
	}

	// Along the start segment in each direction allowed, from the snapped point.
	visit(from.Lon, from.Lat, 0)
	fs := from.seg
	t := float64(p.segTime[fs])
	fracs := g.shapeFracs(fs)
	first := g.segShape[fs]
	for _, forward := range []bool{true, false} {
		if !g.allowedDirection(mode, fs, forward) {
			continue
		}
		for i, f := range fracs {
			if forward && f > from.frac || !forward && f < from.frac {
				if c := math.Abs(f-from.frac) * t; c <= limitS {
					lon, lat := g.point(first + int32(i))
					visit(lon, lat, c)
				}
			}
		}
		if forward {
			seed(g.segTo[fs], (1-from.frac)*t)
		} else {
			seed(g.segFrom[fs], from.frac*t)
		}
	}

	settled := 0
	for sr.queue.Len() > 0 {
		it := heap.Pop(&sr.queue).(item)
		v := it.v
		gv := sr.cost(v)
		if it.f > gv {
			continue // stale entry
		}
		settled++
		lon, lat := g.vertex(v)
		visit(lon, lat, gv)
		for e := g.offsets[v]; e < g.offsets[v+1]; e++ {
			te := float64(p.edgeTime[e])
			if math.IsInf(te, 1) {
				continue
			}
			g.travel(e, gv, te, limitS, visit)
			w := g.edgeTo[e]
			if c := gv + te; c <= limitS && c < sr.cost(w) {
				sr.set(w, c, e)
				heap.Push(&sr.queue, item{w, c})
			}
		}
	}
	return from, settled, nil
}

// travel visits the shape points of edge e left at time start (taking te seconds), up to the time limit, and the
// point where the limit falls when it falls on the edge.
func (g *Graph) travel(e int32, start, te, limitS float64, visit func(lon, lat, seconds float64)) {
	s := g.edgeSeg[e]
	forward := s >= 0
	if !forward {
		s = ^s
	}
	fracs := g.shapeFracs(s)
	first := g.segShape[s]
	n := len(fracs)
	prevLon, prevLat, prevT := 0.0, 0.0, start
	for k := range n {
		i := k
		f := fracs[i]
		if !forward {
			i, f = n-1-k, 1-fracs[n-1-k]
		}
		lon, lat := g.point(first + int32(i))
		c := start + f*te
		if c > limitS {
			if k > 0 && c > prevT {
				r := (limitS - prevT) / (c - prevT)
				visit(prevLon+(lon-prevLon)*r, prevLat+(lat-prevLat)*r, limitS)
			}
			return
		}
		visit(lon, lat, c)
		prevLon, prevLat, prevT = lon, lat, c
	}
}

// shapeFracs returns how far along segment s each of its shape points lies, from 0 at the start to 1 at the end.
func (g *Graph) shapeFracs(s int32) []float64 {
	first, last := g.segShape[s], g.segShape[s+1]-1
	fracs := make([]float64, last-first+1)
	var total float64
	for p := first; p < last; p++ {
		x1, y1 := g.point(p)
		x2, y2 := g.point(p + 1)
		total += haversine(x1, y1, x2, y2)
		fracs[p-first+1] = total
	}
	for i := range fracs {
		if total > 0 {
			fracs[i] /= total
		}
	}
	return fracs
}
