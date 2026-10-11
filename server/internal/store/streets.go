package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// StreetQuery filters streets.
type StreetQuery struct {
	Text   string
	BBox   *geo.BBox
	Area   string
	Kinds  []string
	Limit  int
	Offset int
}

const streetColumns = `s.id, s.name, s.ref, s.kind, s.length_m, s.way_count, s.locality,
	s.min_lon, s.min_lat, s.max_lon, s.max_lat, s.lon, s.lat`

func scanStreet(sc interface{ Scan(...any) error }) (Street, error) {
	var st Street
	var name, ref, locality sql.NullString
	var b [4]float64
	err := sc.Scan(&st.ID, &name, &ref, &st.Kind, &st.LengthM, &st.WayCount, &locality,
		&b[0], &b[1], &b[2], &b[3], &st.Center[0], &st.Center[1])
	st.Name, st.Ref, st.Locality = str(name), str(ref), str(locality)
	st.BBox = b[:]
	return st, err
}

// Streets lists streets, longest first unless a text query ranks them.
func (s *Store) Streets(ctx context.Context, q StreetQuery) ([]Street, error) {
	from := "streets s"
	var where []string
	var args []any
	order := "s.length_m DESC"
	if q.Text != "" {
		match := MatchExpression(q.Text)
		if match == "" {
			return []Street{}, nil
		}
		from += ` JOIN (SELECT ref, bm25(search, 10.0, 4.0, 1.0) AS score FROM search
		                WHERE search MATCH ? AND kind = 'street') f ON f.ref = s.id`
		args = append(args, match)
		order = "f.score"
	}
	if q.BBox != nil {
		where = append(where, "s.max_lon >= ? AND s.min_lon <= ? AND s.max_lat >= ? AND s.min_lat <= ?")
		args = append(args, q.BBox.MinLon, q.BBox.MaxLon, q.BBox.MinLat, q.BBox.MaxLat)
	}
	if q.Area != "" {
		where = append(where, "s.id IN (SELECT member_id FROM area_members WHERE area_id = ? AND kind = 'street')")
		args = append(args, q.Area)
	}
	if len(q.Kinds) > 0 {
		where = append(where, "s.kind IN ("+placeholders(len(q.Kinds))+")")
		args = append(args, anySlice(q.Kinds)...)
	}
	sqlText := "SELECT " + streetColumns + " FROM " + from
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	sqlText += " ORDER BY " + order + " LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Street{}
	for rows.Next() {
		st, err := scanStreet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// Street returns a street with all of its ways and a merged MultiLineString geometry.
func (s *Store) Street(ctx context.Context, id int64) (*StreetDetail, error) {
	st, err := scanStreet(s.db.QueryRowContext(ctx, "SELECT "+streetColumns+" FROM streets s WHERE s.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d := &StreetDetail{Street: st, Ways: []Road{}}
	rows, err := s.db.QueryContext(ctx, `SELECT `+roadColumns+` FROM roads r WHERE r.street_id = ? ORDER BY r.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		road, geometry, err := scanRoad(rows)
		if err != nil {
			return nil, err
		}
		var g geo.Geometry
		if json.Unmarshal([]byte(geometry), &g) == nil && g.Type == "LineString" {
			lines = append(lines, string(g.Coordinates))
		}
		d.Ways = append(d.Ways, road)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	d.Geometry = json.RawMessage(`{"type":"MultiLineString","coordinates":[` + strings.Join(lines, ",") + `]}`)
	if d.Areas, err = s.memberAreas(ctx, "street", id); err != nil {
		return nil, err
	}
	return d, nil
}

const roadColumns = `r.id, r.street_id, r.name, r.ref, r.kind, r.highway, r.oneway, r.lanes, r.maxspeed, r.surface,
	r.bridge, r.tunnel, r.length_m, r.geometry`

func scanRoad(sc interface{ Scan(...any) error }) (Road, string, error) {
	var r Road
	var name, ref, maxspeed, surface sql.NullString
	var oneway, lanes sql.NullInt64
	var geometry string
	err := sc.Scan(&r.ID, &r.StreetID, &name, &ref, &r.Kind, &r.Highway, &oneway, &lanes, &maxspeed, &surface,
		&r.Bridge, &r.Tunnel, &r.LengthM, &geometry)
	r.Name, r.Ref, r.MaxSpeed, r.Surface = str(name), str(ref), str(maxspeed), str(surface)
	r.Oneway, r.Lanes = ptrInt(oneway), ptrInt(lanes)
	r.OSMURL = osmURL(r.ID)
	return r, geometry, err
}

// Road returns one road way with geometry.
func (s *Store) Road(ctx context.Context, id int64) (*Road, error) {
	r, geometry, err := scanRoad(s.db.QueryRowContext(ctx, "SELECT "+roadColumns+" FROM roads r WHERE r.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.Geometry = json.RawMessage(geometry)
	return &r, nil
}
