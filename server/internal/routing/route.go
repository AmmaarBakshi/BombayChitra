package routing

import (
	"container/heap"
	"errors"
	"math"
	"sync"
)

var (
	// ErrNoRoad means no usable road or path is within reach of a route end.
	ErrNoRoad = errors.New("no usable road near this point")
	// ErrNoRoute means the two points are not connected for the travel mode.
	ErrNoRoute = errors.New("no route between these points")
)

// MaxSnapM is how far a route end may be from the nearest usable road or path.
const MaxSnapM = 1500

// Optimize selects what the route minimises.
type Optimize string

// Route optimisation modes.
const (
	Fastest  Optimize = "time"
	Shortest Optimize = "distance"
)

// Options configures a route search. The zero value is the fastest route by car.
type Options struct {
	Mode       Mode
	Optimize   Optimize
	AvoidTolls bool
}

// Snap is a point on a road segment.
type Snap struct {
	seg      int32
	frac     float64 // position along the segment's shape, 0..1
	index    int32   // shape point that starts the sub-segment containing the snap
	Lon, Lat float64
	DistM    float64 // from the requested point
	Road     string
}

// RoadUse is a named road along the route.
type RoadUse struct {
	Name      string  `json:"name"`
	DistanceM float64 `json:"distance_m"`
}

// Result is a computed route.
type Result struct {
	Mode      Mode         `json:"mode"`
	DistanceM float64      `json:"distance_m"`
	DurationS float64      `json:"duration_s"`
	TollM     float64      `json:"toll_m"` // distance on toll roads
	Roads     []RoadUse    `json:"roads"`
	Path      [][2]float64 `json:"-"`
	From, To  Snap         `json:"-"`
	Settled   int          `json:"-"`
	legs      []leg
}

// leg is one segment (or part of one) along the route, in travel order.
type leg struct {
	seg       int32
	from, to  float64 // positions along the segment's shape, 0..1
	pathStart int     // index in Path where the leg starts
	pathEnd   int     // index in Path where it ends
	distanceM float64
	durationS float64
}

// Nearest returns the closest point on a road or path a car may use, within MaxSnapM.
func (g *Graph) Nearest(lon, lat float64) (Snap, error) {
	p, err := g.profile(Car, false)
	if err != nil {
		return Snap{}, err
	}
	return g.nearest(lon, lat, p)
}

// nearest returns the closest point on a segment the profile may use.
func (g *Graph) nearest(lon, lat float64, p *profile) (Snap, error) {
	kx := 111_320 * math.Cos(lat*math.Pi/180)
	const ky = 110_574.0
	best := Snap{seg: -1, DistM: math.Inf(1)}
	cx := int64(math.Floor(lon / gridDeg))
	cy := int64(math.Floor(lat / gridDeg))
	seen := map[int32]bool{}
	for ring := int64(0); ring <= 3; ring++ {
		for dx := -ring; dx <= ring; dx++ {
			for dy := -ring; dy <= ring; dy++ {
				if max(abs(dx), abs(dy)) != ring {
					continue
				}
				for _, s := range g.grid[(cx+dx)<<32|((cy+dy)&0xffffffff)] {
					if seen[s] {
						continue
					}
					seen[s] = true
					if p.usable(s) {
						g.snapToSegment(s, lon, lat, kx, ky, &best)
					}
				}
			}
		}
		// A road found within this ring is nearer than anything a further ring can hold.
		if best.seg >= 0 && best.DistM <= float64(ring)*gridDeg*kx {
			break
		}
	}
	if best.seg < 0 || best.DistM > MaxSnapM {
		return Snap{}, ErrNoRoad
	}
	best.Road = g.name(best.seg)
	return best, nil
}

func (g *Graph) snapToSegment(s int32, lon, lat, kx, ky float64, best *Snap) {
	first, last := g.segShape[s], g.segShape[s+1]-1
	var total float64
	lengths := make([]float64, 0, last-first)
	for p := first; p < last; p++ {
		x1, y1 := g.point(p)
		x2, y2 := g.point(p + 1)
		l := haversine(x1, y1, x2, y2)
		lengths = append(lengths, l)
		total += l
	}
	var along float64
	for p := first; p < last; p++ {
		x1, y1 := g.point(p)
		x2, y2 := g.point(p + 1)
		ax, ay := (x2-x1)*kx, (y2-y1)*ky
		bx, by := (lon-x1)*kx, (lat-y1)*ky
		t := 0.0
		if d := ax*ax + ay*ay; d > 0 {
			t = math.Max(0, math.Min(1, (ax*bx+ay*by)/d))
		}
		px, py := x1+(x2-x1)*t, y1+(y2-y1)*t
		dist := math.Hypot((lon-px)*kx, (lat-py)*ky)
		l := lengths[p-first]
		if dist < best.DistM {
			frac := 0.0
			if total > 0 {
				frac = (along + l*t) / total
			}
			*best = Snap{seg: s, frac: frac, index: p, Lon: px, Lat: py, DistM: dist}
		}
		along += l
	}
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---------------------------------------------------------------- search

type search struct {
	g     []float64
	prev  []int32 // edge into the vertex; -1 for a start vertex
	stamp []uint32
	gen   uint32
	queue queue
}

var searches = sync.Pool{}

func (g *Graph) acquire() *search {
	n := len(g.offsets) - 1
	if v := searches.Get(); v != nil {
		if s := v.(*search); len(s.g) == n {
			s.gen++
			s.queue = s.queue[:0]
			return s
		}
	}
	return &search{g: make([]float64, n), prev: make([]int32, n), stamp: make([]uint32, n), gen: 1}
}

func (s *search) cost(v int32) float64 {
	if s.stamp[v] != s.gen {
		return math.Inf(1)
	}
	return s.g[v]
}

func (s *search) set(v int32, cost float64, prev int32) {
	s.stamp[v], s.g[v], s.prev[v] = s.gen, cost, prev
}

// Route finds the best route between two lon/lat points.
func (g *Graph) Route(fromLon, fromLat, toLon, toLat float64, opts Options) (*Result, error) {
	if opts.Mode == "" {
		opts.Mode = Car
	}
	p, err := g.profile(opts.Mode, opts.AvoidTolls)
	if err != nil {
		return nil, err
	}
	from, err := g.nearest(fromLon, fromLat, p)
	if err != nil {
		return nil, err
	}
	to, err := g.nearest(toLon, toLat, p)
	if err != nil {
		return nil, err
	}
	weight := func(s int32) float64 { return float64(g.segLen[s]) }
	edgeWeight := func(e int32) float64 {
		if math.IsInf(float64(p.edgeTime[e]), 1) {
			return math.Inf(1)
		}
		return float64(g.segLen[segOf(g.edgeSeg[e])])
	}
	scale := 1.0
	if opts.Optimize != Shortest {
		weight = func(s int32) float64 { return float64(p.segTime[s]) }
		edgeWeight = func(e int32) float64 { return float64(p.edgeTime[e]) }
		scale = 1 / p.maxSpeed
	}

	// Costs to leave the start segment towards each end, and to reach the target from each end of its segment.
	fs, ts := from.seg, to.seg
	fwd := func(s int32) bool { return g.allowedDirection(opts.Mode, s, true) }
	back := func(s int32) bool { return g.allowedDirection(opts.Mode, s, false) }
	best := math.Inf(1)
	bestEnd := int32(-1)
	direct := false
	if fs == ts {
		if fwd(fs) && from.frac <= to.frac || back(fs) && from.frac >= to.frac {
			best, direct = math.Abs(to.frac-from.frac)*weight(fs), true
		}
	}

	sr := g.acquire()
	defer searches.Put(sr)
	tLon, tLat := to.Lon, to.Lat
	h := func(v int32) float64 {
		lon, lat := g.vertex(v)
		return haversine(lon, lat, tLon, tLat) * scale * 0.999
	}
	seed := func(v int32, c float64) {
		if c < sr.cost(v) {
			sr.set(v, c, -1)
			heap.Push(&sr.queue, item{v, c + h(v)})
		}
	}
	if fwd(fs) {
		seed(g.segTo[fs], (1-from.frac)*weight(fs))
	}
	if back(fs) {
		seed(g.segFrom[fs], from.frac*weight(fs))
	}
	targetLeg := func(v int32) float64 {
		leg := math.Inf(1)
		if v == g.segFrom[ts] && fwd(ts) {
			leg = to.frac * weight(ts)
		}
		if v == g.segTo[ts] && back(ts) {
			leg = math.Min(leg, (1-to.frac)*weight(ts))
		}
		return leg
	}

	settled := 0
	for sr.queue.Len() > 0 {
		it := heap.Pop(&sr.queue).(item)
		if it.f >= best {
			break
		}
		v := it.v
		gv := sr.cost(v)
		if it.f > gv+h(v)+1e-9 {
			continue // stale entry
		}
		settled++
		if leg := targetLeg(v); gv+leg < best {
			best, bestEnd, direct = gv+leg, v, false
		}
		for e := g.offsets[v]; e < g.offsets[v+1]; e++ {
			ew := edgeWeight(e)
			if math.IsInf(ew, 1) {
				continue
			}
			w := g.edgeTo[e]
			if c := gv + ew; c < sr.cost(w) {
				sr.set(w, c, e)
				heap.Push(&sr.queue, item{w, c + h(w)})
			}
		}
	}
	if math.IsInf(best, 1) {
		return nil, ErrNoRoute
	}

	res := &Result{Mode: opts.Mode, From: from, To: to, Settled: settled}
	acc := &builder{g: g, p: p, res: res}
	if direct {
		acc.partial(fs, from, to.frac)
		acc.add(to.Lon, to.Lat)
		acc.closeLeg()
	} else {
		var edges []int32
		v := bestEnd
		for sr.prev[v] >= 0 {
			e := sr.prev[v]
			edges = append(edges, e)
			v = g.edgeFrom(e)
		}
		// v is the start vertex: the route left the start segment towards it.
		if v == g.segTo[fs] {
			acc.partial(fs, from, 1)
		} else {
			acc.partial(fs, from, 0)
		}
		for i := len(edges) - 1; i >= 0; i-- {
			acc.edge(edges[i])
		}
		// The route reaches the target segment at its shape start (travelling along it) or end (against it).
		if bestEnd == g.segFrom[ts] {
			acc.partialFromEnd(ts, 0, to)
		} else {
			acc.partialFromEnd(ts, 1, to)
		}
	}
	return res, nil
}

func segOf(s int32) int32 {
	if s < 0 {
		return ^s
	}
	return s
}

// edgeFrom returns the vertex an edge leaves from.
func (g *Graph) edgeFrom(e int32) int32 {
	s := g.edgeSeg[e]
	if s >= 0 {
		return g.segFrom[s]
	}
	return g.segTo[^s]
}

// ---------------------------------------------------------------- path assembly

type builder struct {
	g   *Graph
	p   *profile
	res *Result
	cur *leg
}

func (b *builder) add(lon, lat float64) {
	p := b.res.Path
	if n := len(p); n > 0 && p[n-1][0] == lon && p[n-1][1] == lat {
		return
	}
	b.res.Path = append(p, [2]float64{lon, lat})
}

// openLeg starts recording a leg at the current end of the path.
func (b *builder) openLeg(s int32, from, to float64) {
	b.closeLeg()
	start := max(0, len(b.res.Path)-1)
	b.cur = &leg{seg: s, from: from, to: to, pathStart: start}
}

func (b *builder) closeLeg() {
	if b.cur == nil {
		return
	}
	b.cur.pathEnd = len(b.res.Path) - 1
	if b.cur.distanceM >= 0.01 {
		b.res.legs = append(b.res.legs, *b.cur)
	}
	b.cur = nil
}

func (b *builder) use(s int32, frac float64) {
	length := float64(b.g.segLen[s]) * frac
	if length < 0.01 {
		return // a route end snapped exactly onto a junction
	}
	duration := float64(b.p.segTime[s]) * frac
	b.res.DistanceM += length
	b.res.DurationS += duration
	if b.g.segFlags[s]&flagToll != 0 {
		b.res.TollM += length
	}
	if b.cur != nil {
		b.cur.distanceM += length
		b.cur.durationS += duration
	}
	name := b.g.name(s)
	if n := len(b.res.Roads); n > 0 && b.res.Roads[n-1].Name == name {
		b.res.Roads[n-1].DistanceM += length
	} else {
		b.res.Roads = append(b.res.Roads, RoadUse{Name: name, DistanceM: length})
	}
}

// partial travels along segment s from a snap point to position frac (0 = shape start, 1 = shape end).
func (b *builder) partial(s int32, from Snap, frac float64) {
	g := b.g
	b.add(from.Lon, from.Lat)
	b.openLeg(s, from.frac, frac)
	first, last := g.segShape[s], g.segShape[s+1]-1
	if frac >= from.frac {
		for p := from.index + 1; p <= last; p++ {
			if frac < 1 && b.beyond(s, p, frac) {
				break
			}
			b.add(g.point(p))
		}
	} else {
		for p := from.index; p >= first; p-- {
			if frac > 0 && b.before(s, p, frac) {
				break
			}
			b.add(g.point(p))
		}
	}
	b.use(s, math.Abs(frac-from.frac))
}

// partialFromEnd travels the last segment from one of its ends (0 = shape start, 1 = shape end) to the snap point.
func (b *builder) partialFromEnd(s int32, end float64, to Snap) {
	g := b.g
	b.openLeg(s, end, to.frac)
	first, last := g.segShape[s], g.segShape[s+1]-1
	if end == 0 {
		for p := first; p <= to.index; p++ {
			b.add(g.point(p))
		}
	} else {
		for p := last; p > to.index; p-- {
			b.add(g.point(p))
		}
	}
	b.add(to.Lon, to.Lat)
	b.use(s, math.Abs(end-to.frac))
	b.closeLeg()
}

// beyond reports whether shape point p lies past position frac along segment s; before is the reverse.
func (b *builder) beyond(s, p int32, frac float64) bool { return b.fracAt(s, p) > frac }
func (b *builder) before(s, p int32, frac float64) bool { return b.fracAt(s, p) < frac }

func (b *builder) fracAt(s, p int32) float64 {
	g := b.g
	first, last := g.segShape[s], g.segShape[s+1]-1
	var along, total float64
	for q := first; q < last; q++ {
		x1, y1 := g.point(q)
		x2, y2 := g.point(q + 1)
		l := haversine(x1, y1, x2, y2)
		if q < p {
			along += l
		}
		total += l
	}
	if total == 0 {
		return 0
	}
	return along / total
}

func (b *builder) edge(e int32) {
	g := b.g
	s := g.edgeSeg[e]
	if s >= 0 {
		b.openLeg(s, 0, 1)
		for p := g.segShape[s]; p < g.segShape[s+1]; p++ {
			b.add(g.point(p))
		}
	} else {
		s = ^s
		b.openLeg(s, 1, 0)
		for p := g.segShape[s+1] - 1; p >= g.segShape[s]; p-- {
			b.add(g.point(p))
		}
	}
	b.use(s, 1)
}

// ---------------------------------------------------------------- priority queue

type item struct {
	v int32
	f float64
}

type queue []item

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].f < q[j].f }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)        { *q = append(*q, x.(item)) }
func (q *queue) Pop() any {
	old := *q
	it := old[len(old)-1]
	*q = old[:len(old)-1]
	return it
}
