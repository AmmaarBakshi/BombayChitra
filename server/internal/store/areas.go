package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

func (s *Store) loadAreas(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, short_name, kind, description, relations, area_km2,
		       min_lon, min_lat, max_lon, max_lat, lon, lat, geometry
		FROM areas ORDER BY sort`)
	if err != nil {
		return err
	}
	defer rows.Close()
	s.areaByID = map[string]*Area{}
	s.areaShapes = map[string]*geo.PolygonIndex{}
	for rows.Next() {
		var a Area
		var short, kind, desc, geometry sql.NullString
		var relations string
		var bbox [4]float64
		if err := rows.Scan(&a.ID, &a.Name, &short, &kind, &desc, &relations, &a.AreaKm2,
			&bbox[0], &bbox[1], &bbox[2], &bbox[3], &a.Center[0], &a.Center[1], &geometry); err != nil {
			return err
		}
		a.ShortName, a.Kind, a.Description = str(short), str(kind), str(desc)
		a.Relations = json.RawMessage(relations)
		a.BBox = bbox[:]
		if geometry.Valid {
			if idx, err := geo.NewPolygonIndex(geometry.String); err == nil {
				s.areaShapes[a.ID] = idx
			}
		}
		s.areas = append(s.areas, a)
	}
	for i := range s.areas {
		s.areaByID[s.areas[i].ID] = &s.areas[i]
	}
	return rows.Err()
}

// Areas lists configured areas (without geometry) in configured order.
func (s *Store) Areas() []Area { return s.areas }

// AreaExists reports whether an area id is known.
func (s *Store) AreaExists(id string) bool { _, ok := s.areaByID[id]; return ok }

// Area returns one area with its simplified geometry and statistics.
func (s *Store) Area(ctx context.Context, id string) (*Area, error) {
	base, ok := s.areaByID[id]
	if !ok {
		return nil, ErrNotFound
	}
	a := *base
	var geometry sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT geometry FROM areas WHERE id = ?`, id).Scan(&geometry); err != nil {
		return nil, err
	}
	if geometry.Valid {
		a.Geometry = json.RawMessage(geometry.String)
	}
	stats, err := s.Stats(ctx, id)
	if err != nil {
		return nil, err
	}
	a.Stats = stats
	return &a, nil
}

// Stats returns statistics for an area id, or for the whole dataset when id is "" or "*".
func (s *Store) Stats(ctx context.Context, id string) (map[string]float64, error) {
	if id == "" {
		id = "*"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM area_stats WHERE area_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var k string
		var v float64
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// AreasAt returns the configured areas containing a point, in configured order.
func (s *Store) AreasAt(p geo.Point) []AreaRef {
	out := []AreaRef{}
	for _, a := range s.areas {
		if shape := s.areaShapes[a.ID]; shape != nil && shape.Contains(p) {
			out = append(out, AreaRef{ID: a.ID, Name: a.Name, Kind: a.Kind})
		}
	}
	return out
}

func (s *Store) memberAreas(ctx context.Context, kind string, id int64) ([]AreaRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT area_id FROM area_members WHERE kind = ? AND member_id = ?`, kind, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	in := map[string]bool{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		in[a] = true
	}
	out := []AreaRef{}
	for _, a := range s.areas {
		if in[a.ID] {
			out = append(out, AreaRef{ID: a.ID, Name: a.Name, Kind: a.Kind})
		}
	}
	return out, rows.Err()
}
