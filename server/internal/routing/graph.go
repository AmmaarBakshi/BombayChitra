// Package routing computes routes by car, two-wheeler or on foot over the road and path graph stored in the
// database ("distance by road").
//
// The pipeline stores the graph in the routing table as gzip-compressed little-endian arrays (see
// docs/database.md): vertices where ways meet, segments with their shapes and the travel modes allowed on them, and
// directed edges in compressed sparse row form. Routes are found with A* and a straight-line heuristic, which is
// exact (never overestimates) because no way is shorter than the straight line or faster than the mode's top speed.
package routing

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
)

// ErrUnavailable means the database has no routing graph (built by an older pipeline).
var ErrUnavailable = errors.New("routing graph not available in this database")

const (
	gridDeg  = 0.005 // snap index cell size, ~550 m
	e7       = 1e7
	earthR   = 6_371_008.8
	topSpeed = 65 / 3.6 // m/s, the fastest road speed in the graph; keeps the time heuristic admissible
)

// Graph is an immutable road graph; safe for concurrent use.
type Graph struct {
	nodes     []int32 // lon, lat × 1e7 per vertex
	offsets   []int32 // CSR: edges of vertex v are offsets[v]..offsets[v+1]
	edgeTo    []int32
	edgeSeg   []int32 // segment index, or ^index when the edge runs against the segment's shape
	edgeTime  []float32
	segLen    []float32
	segTime   []float32
	segFrom   []int32
	segTo     []int32
	segShape  []int32 // CSR offsets into shape, in points
	shape     []int32 // absolute lon, lat × 1e7
	segClass  []uint8
	segDir    []uint8 // vehicles — bit 1: may drive along the shape, bit 2: against it
	segAccess []uint8 // modeCar | modeTwoWheeler | modeFoot
	segFlags  []uint8 // flagToll, flagSteps…
	segSpeed  []float32
	segName   []int32
	names     []string
	grid      map[int64][]int32
	version   int

	profiles sync.Map // profileKey → *profile
}

// Stats describes the graph size.
type Stats struct {
	Version  int    `json:"version"`
	Vertices int    `json:"vertices"`
	Segments int    `json:"segments"`
	Edges    int    `json:"edges"`
	Modes    []Mode `json:"modes"`
}

// Stats returns the graph size.
func (g *Graph) Stats() Stats {
	return Stats{Version: g.version, Vertices: len(g.offsets) - 1, Segments: len(g.segLen), Edges: len(g.edgeTo),
		Modes: g.Modes()}
}

// Load reads the graph from the database's routing table.
func Load(ctx context.Context, db *sql.DB) (*Graph, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, data FROM routing`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, ErrUnavailable
		}
		return nil, err
	}
	defer rows.Close()
	blobs := map[string][]byte{}
	for rows.Next() {
		var name string
		var data []byte
		if err := rows.Scan(&name, &data); err != nil {
			return nil, err
		}
		raw, err := gunzip(data)
		if err != nil {
			return nil, fmt.Errorf("routing %s: %w", name, err)
		}
		blobs[name] = raw
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(blobs) == 0 {
		return nil, ErrUnavailable
	}
	g := &Graph{
		nodes:     ints(blobs["nodes"]),
		offsets:   ints(blobs["offsets"]),
		edgeTo:    ints(blobs["edge_to"]),
		edgeSeg:   ints(blobs["edge_seg"]),
		edgeTime:  floats(blobs["edge_time"]),
		segLen:    floats(blobs["seg_len"]),
		segShape:  ints(blobs["seg_shape"]),
		shape:     ints(blobs["shape"]),
		segClass:  blobs["seg_class"],
		segDir:    blobs["seg_dir"],
		segAccess: blobs["seg_access"],
		segFlags:  blobs["seg_flags"],
		segSpeed:  floats(blobs["seg_speed"]),
		segName:   ints(blobs["seg_name"]),
		version:   1,
	}
	if g.segAccess != nil {
		g.version = 2
	}
	if n := blobs["names"]; len(n) > 0 {
		g.names = strings.Split(string(n), "\n")
	}
	if err := g.init(); err != nil {
		return nil, err
	}
	return g, nil
}

// init validates the arrays, fills in what version 1 graphs lack (cars only) and derives segment endpoints, car
// times, absolute shapes and the snap index.
func (g *Graph) init() error {
	vertices, segments, edges := len(g.offsets)-1, len(g.segLen), len(g.edgeTo)
	if vertices < 1 || len(g.nodes) != 2*vertices || len(g.edgeSeg) != edges || len(g.edgeTime) != edges ||
		len(g.segShape) != segments+1 || len(g.segClass) != segments || len(g.segDir) != segments ||
		len(g.segName) != segments || int(g.offsets[vertices]) != edges || int(g.segShape[segments])*2 != len(g.shape) {
		return errors.New("routing graph arrays are inconsistent")
	}
	if g.version >= 2 && (len(g.segAccess) != segments || len(g.segFlags) != segments || len(g.segSpeed) != segments) {
		return errors.New("routing graph access arrays are inconsistent")
	}
	g.segFrom = make([]int32, segments)
	g.segTo = make([]int32, segments)
	g.segTime = make([]float32, segments)
	for i := range g.segTime {
		g.segTime[i] = float32(math.Inf(1))
	}
	for v := 0; v < vertices; v++ {
		for e := g.offsets[v]; e < g.offsets[v+1]; e++ {
			s := g.edgeSeg[e]
			if s >= 0 {
				g.segFrom[s], g.segTo[s] = int32(v), g.edgeTo[e]
			} else {
				s = ^s
				g.segFrom[s], g.segTo[s] = g.edgeTo[e], int32(v)
			}
			// The reverse edge of a one-way street exists for pedestrians only and has no car time.
			if t := g.edgeTime[e]; !math.IsInf(float64(t), 1) {
				g.segTime[s] = t
			}
		}
	}
	if g.version < 2 {
		g.segAccess = make([]uint8, segments)
		g.segFlags = make([]uint8, segments)
		g.segSpeed = make([]float32, segments)
		for s := range segments {
			g.segAccess[s] = modeCar
			if g.segTime[s] > 0 {
				g.segSpeed[s] = g.segLen[s] / g.segTime[s] * 3.6
			}
		}
	}
	// Shapes are stored as a first point plus deltas.
	for s := 0; s < segments; s++ {
		for p := g.segShape[s] + 1; p < g.segShape[s+1]; p++ {
			g.shape[2*p] += g.shape[2*p-2]
			g.shape[2*p+1] += g.shape[2*p-1]
		}
	}
	g.grid = make(map[int64][]int32, segments/2)
	for s := 0; s < segments; s++ {
		minX, minY, maxX, maxY := int32(math.MaxInt32), int32(math.MaxInt32), int32(math.MinInt32), int32(math.MinInt32)
		for p := g.segShape[s]; p < g.segShape[s+1]; p++ {
			x, y := g.shape[2*p], g.shape[2*p+1]
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
		for cx := cell(minX); cx <= cell(maxX); cx++ {
			for cy := cell(minY); cy <= cell(maxY); cy++ {
				key := cx<<32 | (cy & 0xffffffff)
				g.grid[key] = append(g.grid[key], int32(s))
			}
		}
	}
	return nil
}

func cell(e7coord int32) int64 { return int64(math.Floor(float64(e7coord) / e7 / gridDeg)) }

func (g *Graph) vertex(v int32) (lon, lat float64) {
	return float64(g.nodes[2*v]) / e7, float64(g.nodes[2*v+1]) / e7
}

func (g *Graph) point(p int32) (lon, lat float64) {
	return float64(g.shape[2*p]) / e7, float64(g.shape[2*p+1]) / e7
}

func (g *Graph) name(s int32) string {
	if i := g.segName[s]; i >= 0 && int(i) < len(g.names) {
		return g.names[i]
	}
	return ""
}

func gunzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func ints(b []byte) []int32 {
	out := make([]int32, len(b)/4)
	for i := range out {
		out[i] = int32(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out
}

func floats(b []byte) []float32 {
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out
}

// haversine returns the great-circle distance in metres.
func haversine(lon1, lat1, lon2, lat2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthR * math.Asin(math.Min(1, math.Sqrt(a)))
}
