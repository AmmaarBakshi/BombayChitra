package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// BridgeQuery filters bridge ways or named bridges.
type BridgeQuery struct {
	BBox       *geo.BBox
	Kinds      []string
	MinLengthM float64
	Area       string
	Text       string
	Sort       string // "length" (default) or "name"
	Limit      int
	Offset     int
}

const bridgeColumns = `b.id, b.group_id, b.name, b.kind, b.class, b.structure, b.layer, b.ref, b.lanes,
	b.length_m, b.area_m2, b.lon, b.lat`

func scanBridge(sc interface{ Scan(...any) error }, extra ...any) (Bridge, error) {
	var b Bridge
	var group, layer, lanes sql.NullInt64
	var name, class, structure, ref sql.NullString
	dst := append([]any{&b.ID, &group, &name, &b.Kind, &class, &structure, &layer, &ref, &lanes,
		&b.LengthM, &b.AreaM2, &b.Lon, &b.Lat}, extra...)
	err := sc.Scan(dst...)
	b.GroupID, b.Layer, b.Lanes = ptrInt(group), ptrInt(layer), ptrInt(lanes)
	b.Name, b.Class, b.Structure, b.Ref = str(name), str(class), str(structure), str(ref)
	b.OSMURL = osmURL(b.ID)
	return b, err
}

// Bridges lists individual bridge ways.
func (s *Store) Bridges(ctx context.Context, q BridgeQuery) ([]Bridge, error) {
	from := "bridges b"
	var where []string
	var args []any
	if q.BBox != nil {
		from += " JOIN bridges_rtree t ON t.id = b.id"
		where = append(where, "t.max_lon >= ? AND t.min_lon <= ? AND t.max_lat >= ? AND t.min_lat <= ?")
		args = append(args, q.BBox.MinLon, q.BBox.MaxLon, q.BBox.MinLat, q.BBox.MaxLat)
	}
	if len(q.Kinds) > 0 {
		where = append(where, "b.kind IN ("+placeholders(len(q.Kinds))+")")
		args = append(args, anySlice(q.Kinds)...)
	}
	if q.MinLengthM > 0 {
		where = append(where, "b.length_m >= ?")
		args = append(args, q.MinLengthM)
	}
	if q.Area != "" {
		where = append(where, "b.id IN (SELECT member_id FROM area_members WHERE area_id = ? AND kind = 'bridge')")
		args = append(args, q.Area)
	}
	if q.Text != "" {
		where = append(where, "b.name LIKE ? ESCAPE '\\'")
		args = append(args, likePattern(q.Text))
	}
	sqlText := "SELECT " + bridgeColumns + " FROM " + from
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	if q.Sort == "name" {
		sqlText += " ORDER BY b.name IS NULL, b.name COLLATE NOCASE"
	} else {
		sqlText += " ORDER BY b.length_m DESC"
	}
	sqlText += " LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Bridge{}
	for rows.Next() {
		b, err := scanBridge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Bridge returns one bridge way with geometry and tags.
func (s *Store) Bridge(ctx context.Context, id int64) (*Bridge, error) {
	var geometry, tags string
	b, err := scanBridge(s.db.QueryRowContext(ctx,
		"SELECT "+bridgeColumns+", b.geometry, b.tags FROM bridges b WHERE b.id = ?", id), &geometry, &tags)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Geometry, b.Tags = json.RawMessage(geometry), json.RawMessage(tags)
	return &b, nil
}

const namedBridgeColumns = `n.id, n.name, n.kind, n.length_m, n.total_length_m, n.way_count, n.locality,
	n.min_lon, n.min_lat, n.max_lon, n.max_lat, n.lon, n.lat`

func scanNamedBridge(sc interface{ Scan(...any) error }) (NamedBridge, error) {
	var n NamedBridge
	var locality sql.NullString
	var b [4]float64
	err := sc.Scan(&n.ID, &n.Name, &n.Kind, &n.LengthM, &n.TotalLengthM, &n.WayCount, &locality,
		&b[0], &b[1], &b[2], &b[3], &n.Center[0], &n.Center[1])
	n.Locality, n.BBox = str(locality), b[:]
	return n, err
}

// NamedBridges lists named bridges (sea links, flyovers, viaducts) aggregated from their ways.
func (s *Store) NamedBridges(ctx context.Context, q BridgeQuery) ([]NamedBridge, error) {
	var where []string
	var args []any
	if q.BBox != nil {
		where = append(where, "n.max_lon >= ? AND n.min_lon <= ? AND n.max_lat >= ? AND n.min_lat <= ?")
		args = append(args, q.BBox.MinLon, q.BBox.MaxLon, q.BBox.MinLat, q.BBox.MaxLat)
	}
	if len(q.Kinds) > 0 {
		where = append(where, "n.kind IN ("+placeholders(len(q.Kinds))+")")
		args = append(args, anySlice(q.Kinds)...)
	}
	if q.MinLengthM > 0 {
		where = append(where, "n.length_m >= ?")
		args = append(args, q.MinLengthM)
	}
	if q.Area != "" {
		where = append(where, `n.id IN (SELECT b.group_id FROM bridges b JOIN area_members am
			ON am.member_id = b.id AND am.kind = 'bridge' WHERE am.area_id = ?)`)
		args = append(args, q.Area)
	}
	if q.Text != "" {
		where = append(where, "n.name LIKE ? ESCAPE '\\'")
		args = append(args, likePattern(q.Text))
	}
	sqlText := "SELECT " + namedBridgeColumns + " FROM named_bridges n"
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	if q.Sort == "name" {
		sqlText += " ORDER BY n.name COLLATE NOCASE"
	} else {
		sqlText += " ORDER BY n.length_m DESC"
	}
	sqlText += " LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NamedBridge{}
	for rows.Next() {
		n, err := scanNamedBridge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// NamedBridge returns a named bridge with its ways and combined geometry.
func (s *Store) NamedBridge(ctx context.Context, id int64) (*NamedBridge, error) {
	n, err := scanNamedBridge(s.db.QueryRowContext(ctx, "SELECT "+namedBridgeColumns+" FROM named_bridges n WHERE n.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+bridgeColumns+", b.geometry FROM bridges b WHERE b.group_id = ? ORDER BY b.id", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var geometry string
		b, err := scanBridge(rows, &geometry)
		if err != nil {
			return nil, err
		}
		var g geo.Geometry
		if json.Unmarshal([]byte(geometry), &g) == nil && g.Type == "LineString" {
			lines = append(lines, string(g.Coordinates))
		}
		n.Ways = append(n.Ways, b)
	}
	n.Geometry = json.RawMessage(`{"type":"MultiLineString","coordinates":[` + strings.Join(lines, ",") + `]}`)
	return &n, rows.Err()
}

func likePattern(text string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.TrimSpace(text)) + "%"
}
