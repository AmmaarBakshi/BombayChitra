package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// OpenStreetMap has no postal-code boundaries for the region, so PIN code areas are derived from the places whose
// address carries the PIN: their median position, an outline around them (the convex hull, after dropping outliers
// such as a head office's PIN on a branch) and statistics. Areas are approximate by nature: they cover where the
// addressed places are, not the postal department's delivery boundary.

const (
	// minPinPlaces is how many addressed places a PIN needs to get an area.
	minPinPlaces = 8
	// Places further than this many times the median distance from the PIN's median position are outliers, within
	// [pinMinRadiusM, pinMaxRadiusM].
	pinSpread     = 3.0
	pinMinRadiusM = 1500.0
	pinMaxRadiusM = 6000.0
)

var pinPattern = regexp.MustCompile(`^[1-9][0-9]{5}$`)

// normalisePin returns the six-digit PIN in a postcode value ("400 050", "Mumbai 400050"), or "".
func normalisePin(value string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
	if !pinPattern.MatchString(digits) {
		return ""
	}
	return digits
}

// IsPinCode reports whether id looks like an Indian PIN code (six digits, not starting with 0).
func IsPinCode(id string) bool { return pinPattern.MatchString(id) }

type pinArea struct {
	area  Area
	ring  []geo.Point
	stats map[string]float64
}

type pinIndex struct {
	once  sync.Once
	err   error
	list  []Area
	byPin map[string]*pinArea
}

// loadPins builds every PIN code area once, on first use (~0.2 s).
func (s *Store) loadPins(ctx context.Context) (*pinIndex, error) {
	idx := &s.pins
	idx.once.Do(func() { idx.err = idx.build(context.WithoutCancel(ctx), s) })
	return idx, idx.err
}

type pinPlace struct {
	lon, lat           float64
	locality, category string
}

func (idx *pinIndex) build(ctx context.Context, s *Store) error {
	rows, err := s.db.QueryContext(ctx, `SELECT postcode, lon, lat, locality, category FROM places WHERE postcode IS NOT NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()
	byPin := map[string][]pinPlace{}
	for rows.Next() {
		var postcode string
		var p pinPlace
		var locality *string
		if err := rows.Scan(&postcode, &p.lon, &p.lat, &locality, &p.category); err != nil {
			return err
		}
		if locality != nil {
			p.locality = *locality
		}
		if pin := normalisePin(postcode); pin != "" {
			byPin[pin] = append(byPin[pin], p)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	idx.byPin = map[string]*pinArea{}
	for pin, places := range byPin {
		if a := buildPinArea(pin, places); a != nil {
			idx.byPin[pin] = a
			idx.list = append(idx.list, a.area)
		}
	}
	sort.Slice(idx.list, func(i, j int) bool { return idx.list[i].ID < idx.list[j].ID })
	return nil
}

func buildPinArea(pin string, places []pinPlace) *pinArea {
	if len(places) < minPinPlaces {
		return nil
	}
	centre := geo.Point{Lon: median(places, func(p pinPlace) float64 { return p.lon }), Lat: median(places, func(p pinPlace) float64 { return p.lat })}
	dist := make([]float64, len(places))
	for i, p := range places {
		dist[i] = geo.Haversine(centre, geo.Point{Lon: p.lon, Lat: p.lat})
	}
	sorted := append([]float64(nil), dist...)
	sort.Float64s(sorted)
	limit := math.Min(pinMaxRadiusM, math.Max(pinMinRadiusM, pinSpread*sorted[len(sorted)/2]))

	var kept []pinPlace
	var points []geo.Point
	bbox := geo.BBox{MinLon: math.Inf(1), MinLat: math.Inf(1), MaxLon: math.Inf(-1), MaxLat: math.Inf(-1)}
	stats := map[string]float64{}
	localities := map[string]int{}
	for i, p := range places {
		if dist[i] > limit {
			continue
		}
		kept = append(kept, p)
		pt := geo.Point{Lon: p.lon, Lat: p.lat}
		points = append(points, pt)
		bbox.MinLon, bbox.MinLat = math.Min(bbox.MinLon, p.lon), math.Min(bbox.MinLat, p.lat)
		bbox.MaxLon, bbox.MaxLat = math.Max(bbox.MaxLon, p.lon), math.Max(bbox.MaxLat, p.lat)
		stats["places:"+p.category]++
		if p.locality != "" {
			localities[p.locality]++
		}
	}
	if len(kept) < minPinPlaces {
		return nil
	}
	stats["places"] = float64(len(kept))
	ring := geo.ConvexHull(points)
	if len(ring) < 4 {
		return nil // every place at one spot: no outline to draw
	}
	area := Area{
		ID:          pin,
		Name:        "PIN " + pin,
		ShortName:   pin,
		Kind:        "pincode",
		Description: describeLocalities(localities),
		AreaKm2:     round(geo.RingAreaKm2(ring), 2),
		BBox:        []float64{round(bbox.MinLon, 6), round(bbox.MinLat, 6), round(bbox.MaxLon, 6), round(bbox.MaxLat, 6)},
		Center:      [2]float64{round(centre.Lon, 6), round(centre.Lat, 6)},
		Relations:   json.RawMessage("[]"),
	}
	return &pinArea{area: area, ring: ring, stats: stats}
}

// describeLocalities names the three localities most of a PIN's places are in: "Bandra West, Pali Hill and Khar".
func describeLocalities(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) > 3 {
		names = names[:3]
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func median(places []pinPlace, value func(pinPlace) float64) float64 {
	v := make([]float64, len(places))
	for i, p := range places {
		v[i] = value(p)
	}
	sort.Float64s(v)
	return v[len(v)/2]
}

// PinCodes lists the PIN code areas, ordered by PIN.
func (s *Store) PinCodes(ctx context.Context) ([]Area, error) {
	idx, err := s.loadPins(ctx)
	if err != nil {
		return nil, err
	}
	return idx.list, nil
}

// PinCodeExists reports whether a PIN has an area.
func (s *Store) PinCodeExists(ctx context.Context, pin string) bool {
	if !IsPinCode(pin) {
		return false
	}
	idx, err := s.loadPins(ctx)
	if err != nil {
		return false
	}
	_, ok := idx.byPin[pin]
	return ok
}

// PinCode returns one PIN code area with its outline and statistics.
func (s *Store) PinCode(ctx context.Context, pin string) (*Area, error) {
	idx, err := s.loadPins(ctx)
	if err != nil {
		return nil, err
	}
	p, ok := idx.byPin[pin]
	if !ok {
		return nil, ErrNotFound
	}
	a := p.area
	a.Stats = p.stats
	a.Geometry = ringGeoJSON(p.ring)
	return &a, nil
}

// PinCodeAt returns the PIN of the smallest PIN code area whose outline contains p ("" outside all of them).
// Outlines of neighbouring PINs overlap a little, being convex; the smallest is the most specific.
func (s *Store) PinCodeAt(ctx context.Context, p geo.Point) (string, error) {
	idx, err := s.loadPins(ctx)
	if err != nil {
		return "", err
	}
	best, bestArea := "", math.Inf(1)
	for pin, a := range idx.byPin {
		b := a.area.BBox
		if p.Lon < b[0] || p.Lon > b[2] || p.Lat < b[1] || p.Lat > b[3] || a.area.AreaKm2 >= bestArea {
			continue
		}
		if inRing(a.ring, p) {
			best, bestArea = pin, a.area.AreaKm2
		}
	}
	return best, nil
}

// inRing is a ray-casting point-in-polygon test for a closed ring.
func inRing(ring []geo.Point, p geo.Point) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		a, b := ring[i], ring[j]
		if (a.Lat > p.Lat) != (b.Lat > p.Lat) && p.Lon < (b.Lon-a.Lon)*(p.Lat-a.Lat)/(b.Lat-a.Lat)+a.Lon {
			in = !in
		}
	}
	return in
}

// pinSearchResult is the search hit for a query that is a PIN with an area.
func (s *Store) pinSearchResult(ctx context.Context, query string) *SearchResult {
	if !IsPinCode(query) || !s.PinCodeExists(ctx, query) {
		return nil
	}
	a := s.pins.byPin[query].area
	detail := "PIN code"
	if a.Description != "" {
		detail += " · " + a.Description
	}
	return &SearchResult{Kind: "area", ID: a.ID, Name: a.Name, Detail: detail, Category: "pincode",
		Lon: a.Center[0], Lat: a.Center[1], BBox: a.BBox}
}

func ringGeoJSON(ring []geo.Point) json.RawMessage {
	var b strings.Builder
	b.WriteString(`{"type":"Polygon","coordinates":[[`)
	for i, p := range ring {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "[%g,%g]", round(p.Lon, 6), round(p.Lat, 6))
	}
	b.WriteString(`]]}`)
	return json.RawMessage(b.String())
}
