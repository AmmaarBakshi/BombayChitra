package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
)

const routeColumns = `t.id, t.osm_type, t.line_key, t.label, t.name, t.mode, t.network, t.ref, t.colour, t.operator, t.from_name,
	t.to_name, t.state, t.length_m, t.stop_count, t.min_lon, t.min_lat, t.max_lon, t.max_lat`

func scanRoute(sc interface{ Scan(...any) error }, extra ...any) (TransitRoute, error) {
	var r TransitRoute
	var name, network, ref, colour, operator, from, to, state sql.NullString
	var b [4]float64
	dst := append([]any{&r.ID, &r.OSMType, &r.LineKey, &r.Label, &name, &r.Mode, &network, &ref, &colour, &operator, &from, &to,
		&state, &r.LengthM, &r.StopCount, &b[0], &b[1], &b[2], &b[3]}, extra...)
	err := sc.Scan(dst...)
	r.Name, r.Network, r.Ref, r.Colour = str(name), str(network), str(ref), str(colour)
	r.Operator, r.From, r.To, r.State = str(operator), str(from), str(to), str(state)
	r.BBox = b[:]
	r.OSMURL = "https://www.openstreetmap.org/" + r.OSMType + "/" + itoa(r.ID)
	return r, err
}

// TransitLines lists lines (grouping each line's route relations), optionally filtered by mode and network.
func (s *Store) TransitLines(ctx context.Context, modes []string, network string) ([]TransitLine, error) {
	sqlText := "SELECT " + routeColumns + " FROM transit_lines t"
	var where []string
	var args []any
	if len(modes) > 0 {
		where = append(where, "t.mode IN ("+placeholders(len(modes))+")")
		args = append(args, anySlice(modes)...)
	}
	if network != "" {
		where = append(where, "t.network = ?")
		args = append(args, network)
	}
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	sqlText += ` ORDER BY CASE t.mode WHEN 'subway' THEN 0 WHEN 'light_rail' THEN 0 WHEN 'monorail' THEN 1
		WHEN 'train' THEN 2 ELSE 3 END, t.network, CAST(t.ref AS INTEGER), t.ref, t.line_key, t.id`
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []TransitLine
	index := map[string]int{}
	for rows.Next() {
		r, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		i, ok := index[r.LineKey]
		if !ok {
			i = len(lines)
			index[r.LineKey] = i
			lines = append(lines, TransitLine{Key: r.LineKey, Label: r.Label, Mode: r.Mode, Network: r.Network,
				Ref: r.Ref, Colour: r.Colour, BBox: append([]float64(nil), r.BBox...)})
		}
		l := &lines[i]
		l.LengthM = math.Max(l.LengthM, r.LengthM)
		l.BBox = []float64{math.Min(l.BBox[0], r.BBox[0]), math.Min(l.BBox[1], r.BBox[1]),
			math.Max(l.BBox[2], r.BBox[2]), math.Max(l.BBox[3], r.BBox[3])}
		l.Routes = append(l.Routes, r)
	}
	if lines == nil {
		lines = []TransitLine{}
	}
	return lines, rows.Err()
}

// TransitRoute returns one route (an OSM route relation, or a line assembled from track ways) with its ordered stops
// and geometry.
func (s *Store) TransitRoute(ctx context.Context, id int64) (*TransitRoute, error) {
	var stops, geometry string
	r, err := scanRoute(s.db.QueryRowContext(ctx,
		"SELECT "+routeColumns+", t.stops, t.geometry FROM transit_lines t WHERE t.id = ?", id), &stops, &geometry)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.Stops, r.Geometry = json.RawMessage(stops), json.RawMessage(geometry)
	return &r, nil
}

// StationCategories are the place categories treated as rail stations.
var StationCategories = []string{"metro_station", "monorail_station", "railway_station"}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
