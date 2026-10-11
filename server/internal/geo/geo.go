// Package geo holds the small amount of geometry the API needs: distances, bounding boxes, point-to-line snapping
// and point-in-polygon tests on GeoJSON coordinates.
package geo

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const earthRadiusM = 6371008.8

// Point is a lon/lat pair.
type Point struct{ Lon, Lat float64 }

// BBox is [minLon, minLat, maxLon, maxLat].
type BBox struct{ MinLon, MinLat, MaxLon, MaxLat float64 }

// Haversine returns the great-circle distance in metres.
func Haversine(a, b Point) float64 {
	dLat := rad(b.Lat - a.Lat)
	dLon := rad(b.Lon - a.Lon)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(rad(a.Lat))*math.Cos(rad(b.Lat))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(h)))
}

func rad(d float64) float64 { return d * math.Pi / 180 }

// Around returns a bounding box that contains every point within radius metres of p.
func Around(p Point, radiusM float64) BBox {
	dLat := radiusM / 111320.0
	dLon := radiusM / (111320.0 * math.Max(0.01, math.Cos(rad(p.Lat))))
	return BBox{p.Lon - dLon, p.Lat - dLat, p.Lon + dLon, p.Lat + dLat}
}

// ParseBBox parses "minLon,minLat,maxLon,maxLat".
func ParseBBox(s string) (BBox, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return BBox{}, errors.New("bbox must be minLon,minLat,maxLon,maxLat")
	}
	var v [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return BBox{}, fmt.Errorf("bbox value %q is not a number", p)
		}
		v[i] = f
	}
	b := BBox{v[0], v[1], v[2], v[3]}
	if b.MinLon >= b.MaxLon || b.MinLat >= b.MaxLat || b.MinLat < -90 || b.MaxLat > 90 {
		return BBox{}, errors.New("bbox is empty or out of range")
	}
	return b, nil
}

// ParsePoint parses latitude and longitude strings.
func ParsePoint(lat, lon string) (Point, error) {
	la, err1 := strconv.ParseFloat(strings.TrimSpace(lat), 64)
	lo, err2 := strconv.ParseFloat(strings.TrimSpace(lon), 64)
	if err1 != nil || err2 != nil || la < -90 || la > 90 || lo < -180 || lo > 180 {
		return Point{}, errors.New("lat/lon must be valid coordinates")
	}
	return Point{Lon: lo, Lat: la}, nil
}

// Contains reports whether the box contains p.
func (b BBox) Contains(p Point) bool {
	return p.Lon >= b.MinLon && p.Lon <= b.MaxLon && p.Lat >= b.MinLat && p.Lat <= b.MaxLat
}

// Slice returns the box as a JSON-friendly array.
func (b BBox) Slice() []float64 { return []float64{b.MinLon, b.MinLat, b.MaxLon, b.MaxLat} }

// Snap is the closest point on a line to a probe.
type Snap struct {
	Point     Point
	DistanceM float64
}

// NearestOnLine returns the closest point on a polyline. It projects locally (equirectangular around the probe), which
// is accurate to centimetres at city scale.
func NearestOnLine(p Point, line [][2]float64) (Snap, bool) {
	if len(line) == 0 {
		return Snap{}, false
	}
	kx := 111320.0 * math.Cos(rad(p.Lat))
	const ky = 110574.0
	best := Snap{DistanceM: math.Inf(1)}
	for i := 0; i < len(line); i++ {
		ax, ay := (line[i][0]-p.Lon)*kx, (line[i][1]-p.Lat)*ky
		bx, by := ax, ay
		if i+1 < len(line) {
			bx, by = (line[i+1][0]-p.Lon)*kx, (line[i+1][1]-p.Lat)*ky
		}
		dx, dy := bx-ax, by-ay
		t := 0.0
		if l2 := dx*dx + dy*dy; l2 > 0 {
			t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l2))
		}
		cx, cy := ax+t*dx, ay+t*dy
		if d := math.Hypot(cx, cy); d < best.DistanceM {
			best = Snap{Point{p.Lon + cx/kx, p.Lat + cy/ky}, d}
		}
	}
	return best, true
}

// Geometry is a parsed GeoJSON geometry (only the parts the server needs).
type Geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// Lines extracts LineString / MultiLineString coordinates.
func (g Geometry) Lines() ([][][2]float64, error) {
	switch g.Type {
	case "LineString":
		var l [][2]float64
		err := json.Unmarshal(g.Coordinates, &l)
		return [][][2]float64{l}, err
	case "MultiLineString":
		var ml [][][2]float64
		err := json.Unmarshal(g.Coordinates, &ml)
		return ml, err
	case "Polygon":
		var poly [][][2]float64
		err := json.Unmarshal(g.Coordinates, &poly)
		return poly, err
	}
	return nil, fmt.Errorf("unsupported geometry %s", g.Type)
}

// Polygons extracts Polygon / MultiPolygon rings.
func (g Geometry) Polygons() ([][][][2]float64, error) {
	switch g.Type {
	case "Polygon":
		var p [][][2]float64
		err := json.Unmarshal(g.Coordinates, &p)
		return [][][][2]float64{p}, err
	case "MultiPolygon":
		var mp [][][][2]float64
		err := json.Unmarshal(g.Coordinates, &mp)
		return mp, err
	}
	return nil, fmt.Errorf("unsupported geometry %s", g.Type)
}

// PolygonIndex answers point-in-polygon queries for a (multi)polygon.
type PolygonIndex struct {
	polygons [][][][2]float64
	bbox     BBox
}

// NewPolygonIndex prepares a GeoJSON polygon or multipolygon.
func NewPolygonIndex(raw string) (*PolygonIndex, error) {
	var g Geometry
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return nil, err
	}
	polys, err := g.Polygons()
	if err != nil {
		return nil, err
	}
	b := BBox{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, poly := range polys {
		if len(poly) == 0 {
			continue
		}
		for _, c := range poly[0] {
			b.MinLon, b.MaxLon = math.Min(b.MinLon, c[0]), math.Max(b.MaxLon, c[0])
			b.MinLat, b.MaxLat = math.Min(b.MinLat, c[1]), math.Max(b.MaxLat, c[1])
		}
	}
	return &PolygonIndex{polygons: polys, bbox: b}, nil
}

// Contains reports whether p lies inside the polygon (holes excluded).
func (pi *PolygonIndex) Contains(p Point) bool {
	if !pi.bbox.Contains(p) {
		return false
	}
	for _, poly := range pi.polygons {
		if len(poly) == 0 || !inRing(p, poly[0]) {
			continue
		}
		inHole := false
		for _, hole := range poly[1:] {
			if inRing(p, hole) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}
	return false
}

func inRing(p Point, ring [][2]float64) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]
		if (yi > p.Lat) != (yj > p.Lat) && p.Lon < (xj-xi)*(p.Lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}
