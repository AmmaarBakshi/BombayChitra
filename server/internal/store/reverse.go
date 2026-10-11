package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// Reverse describes a coordinate: the nearest named road (snapped), the nearest locality, the configured areas that
// contain it and the closest named places.
func (s *Store) Reverse(ctx context.Context, p geo.Point) (*Reverse, error) {
	out := &Reverse{Point: [2]float64{p.Lon, p.Lat}, Areas: s.AreasAt(p), Nearby: []Place{}}

	road, err := s.nearestRoad(ctx, p)
	if err != nil {
		return nil, err
	}
	out.Road = road

	if out.Locality, err = s.nearestLocality(ctx, p); err != nil {
		return nil, err
	}
	nearby, err := s.Places(ctx, PlaceQuery{Near: &p, RadiusM: 150, NamedOnly: true, Limit: 5})
	if err != nil {
		return nil, err
	}
	out.Nearby = nearby

	var parts []string
	if len(nearby) > 0 && *nearby[0].DistanceM < 25 {
		parts = append(parts, nearby[0].Name)
	}
	if road != nil && road.DistanceM < 150 {
		if road.Name != "" {
			parts = append(parts, road.Name)
		} else {
			parts = append(parts, road.Ref)
		}
	}
	if out.Locality != nil {
		parts = append(parts, out.Locality.Name)
	}
	if n := len(out.Areas); n > 0 {
		parts = append(parts, out.Areas[n-1].Name)
	}
	out.Label = strings.Join(dedupeStrings(parts), ", ")
	return out, nil
}

var roadPenalty = map[string]float64{
	"motorway": 1, "trunk": 1, "primary": 1, "secondary": 1, "tertiary": 1, "minor": 1.1,
	"service": 1.8, "pedestrian": 1.8, "track": 2, "path": 3,
}

func (s *Store) nearestRoad(ctx context.Context, p geo.Point) (*RoadMatch, error) {
	for _, radius := range []float64{150, 600} {
		b := geo.Around(p, radius)
		rows, err := s.db.QueryContext(ctx, `
			SELECT r.id, r.street_id, r.name, r.ref, r.kind, r.geometry
			FROM roads_rtree t JOIN roads r ON r.id = t.id
			WHERE t.max_lon >= ? AND t.min_lon <= ? AND t.max_lat >= ? AND t.min_lat <= ?`,
			b.MinLon, b.MaxLon, b.MinLat, b.MaxLat)
		if err != nil {
			return nil, err
		}
		var best *RoadMatch
		var bestCost float64
		for rows.Next() {
			var m RoadMatch
			var name, ref sql.NullString
			var geometry string
			if err := rows.Scan(&m.ID, &m.StreetID, &name, &ref, &m.Kind, &geometry); err != nil {
				rows.Close()
				return nil, err
			}
			var g geo.Geometry
			if json.Unmarshal([]byte(geometry), &g) != nil {
				continue
			}
			lines, err := g.Lines()
			if err != nil {
				continue
			}
			for _, line := range lines {
				snap, ok := geo.NearestOnLine(p, line)
				if !ok {
					continue
				}
				// Prefer real roads: footpaths and service roads only win when clearly closer.
				cost := snap.DistanceM * roadPenalty[m.Kind]
				if best == nil || cost < bestCost {
					m.Name, m.Ref = str(name), str(ref)
					m.DistanceM = round(snap.DistanceM, 1)
					m.Snapped = [2]float64{round(snap.Point.Lon, 7), round(snap.Point.Lat, 7)}
					cp := m
					best, bestCost = &cp, cost
				}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if best != nil && best.DistanceM <= radius {
			return best, nil
		}
	}
	return nil, nil
}

func (s *Store) nearestLocality(ctx context.Context, p geo.Point) (*Locality, error) {
	b := geo.Around(p, 5000)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, kind, lon, lat, population FROM localities
		WHERE kind IN ('suburb', 'quarter', 'neighbourhood', 'village', 'town', 'hamlet')
		  AND lon BETWEEN ? AND ? AND lat BETWEEN ? AND ?`, b.MinLon, b.MaxLon, b.MinLat, b.MaxLat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []Locality
	for rows.Next() {
		var l Locality
		var pop sql.NullInt64
		if err := rows.Scan(&l.ID, &l.Name, &l.Kind, &l.Lon, &l.Lat, &pop); err != nil {
			return nil, err
		}
		l.Population = ptrInt(pop)
		d := round(geo.Haversine(p, geo.Point{Lon: l.Lon, Lat: l.Lat}), 0)
		l.DistanceM = &d
		candidates = append(candidates, l)
	}
	if len(candidates) == 0 {
		return nil, rows.Err()
	}
	// Prefer broader localities (suburbs) unless a neighbourhood is much closer.
	weight := map[string]float64{"suburb": 1, "town": 1, "quarter": 1.15, "neighbourhood": 1.3, "village": 1.2,
		"hamlet": 1.5}
	sort.Slice(candidates, func(i, j int) bool {
		return *candidates[i].DistanceM*weight[candidates[i].Kind] < *candidates[j].DistanceM*weight[candidates[j].Kind]
	})
	best := candidates[0]
	if math.IsNaN(*best.DistanceM) {
		return nil, nil
	}
	return &best, rows.Err()
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
