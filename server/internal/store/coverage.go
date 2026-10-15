package store

import "context"

// CoverageCell counts what is mapped in one cell of a grid over the region: places in OpenStreetMap, places known
// only to Overture Maps (businesses and amenities OpenStreetMap does not have yet), and named buildings.
type CoverageCell struct {
	X, Y           int32
	OSMPlaces      int
	OverturePlaces int
	Buildings      int
}

// Coverage counts places by source and named buildings in each cell of a grid of dx × dy degrees whose cell (0, 0)
// starts at (lon0, lat0). Only cells with something in them are returned.
func (s *Store) Coverage(ctx context.Context, lon0, lat0, dx, dy float64) ([]CoverageCell, error) {
	type key struct{ x, y int32 }
	cells := map[key]*CoverageCell{}
	get := func(x, y int32) *CoverageCell {
		k := key{x, y}
		if c, ok := cells[k]; ok {
			return c
		}
		c := &CoverageCell{X: x, Y: y}
		cells[k] = c
		return c
	}
	// Coordinates west or south of the origin are outside the region; CAST truncates, which is floor from there on.
	rows, err := s.db.QueryContext(ctx, `SELECT CAST((lon - ?1) / ?3 AS INTEGER), CAST((lat - ?2) / ?4 AS INTEGER),
		SUM(osm_type != 'overture'), SUM(osm_type = 'overture')
		FROM places WHERE lon >= ?1 AND lat >= ?2 GROUP BY 1, 2`, lon0, lat0, dx, dy)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x, y int32
		var osm, overture int
		if err := rows.Scan(&x, &y, &osm, &overture); err != nil {
			rows.Close()
			return nil, err
		}
		c := get(x, y)
		c.OSMPlaces, c.OverturePlaces = osm, overture
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `SELECT CAST((lon - ?1) / ?3 AS INTEGER), CAST((lat - ?2) / ?4 AS INTEGER), COUNT(*)
		FROM buildings WHERE lon >= ?1 AND lat >= ?2 GROUP BY 1, 2`, lon0, lat0, dx, dy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x, y int32
		var n int
		if err := rows.Scan(&x, &y, &n); err != nil {
			return nil, err
		}
		get(x, y).Buildings = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]CoverageCell, 0, len(cells))
	for _, c := range cells {
		out = append(out, *c)
	}
	return out, nil
}
