package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// BuildingQuery filters buildings.
type BuildingQuery struct {
	BBox       *geo.BBox
	Area       string
	MinLevels  int
	MinHeightM float64
	NamedOnly  bool
	Text       string
	Sort       string // "height" (default), "levels", "footprint", "name"
	Limit      int
	Offset     int
}

const buildingColumns = `b.id, b.name, b.kind, b.levels, b.height_m, b.min_height_m, b.area_m2, b.lon, b.lat,
	b.address, b.locality`

func scanBuilding(sc interface{ Scan(...any) error }, extra ...any) (Building, error) {
	var b Building
	var name, kind, address, locality sql.NullString
	var levels sql.NullInt64
	var height, minHeight sql.NullFloat64
	dst := append([]any{&b.ID, &name, &kind, &levels, &height, &minHeight, &b.AreaM2, &b.Lon, &b.Lat, &address,
		&locality}, extra...)
	err := sc.Scan(dst...)
	b.Name, b.Kind, b.Address, b.Locality = str(name), str(kind), str(address), str(locality)
	b.Levels, b.HeightM, b.MinHeightM = ptrInt(levels), ptrFloat(height), ptrFloat(minHeight)
	b.OSMURL = osmURL(b.ID)
	return b, err
}

// Buildings lists buildings; by default the tallest first.
func (s *Store) Buildings(ctx context.Context, q BuildingQuery) ([]Building, error) {
	from := "buildings b"
	var where []string
	var args []any
	if q.BBox != nil {
		from += " JOIN buildings_rtree t ON t.id = b.id"
		where = append(where, "t.min_lon >= ? AND t.max_lon <= ? AND t.min_lat >= ? AND t.max_lat <= ?")
		args = append(args, q.BBox.MinLon, q.BBox.MaxLon, q.BBox.MinLat, q.BBox.MaxLat)
	}
	if q.Area != "" {
		shape := s.areaShapes[q.Area]
		a := s.areaByID[q.Area]
		if shape == nil || a == nil {
			return []Building{}, nil
		}
		// Buildings are not in area_members (there can be hundreds of thousands); filter by the area bbox in SQL and
		// by the exact polygon below.
		where = append(where, "b.lon BETWEEN ? AND ? AND b.lat BETWEEN ? AND ?")
		args = append(args, a.BBox[0], a.BBox[2], a.BBox[1], a.BBox[3])
	}
	if q.MinLevels > 0 {
		where = append(where, "b.levels >= ?")
		args = append(args, q.MinLevels)
	}
	if q.MinHeightM > 0 {
		where = append(where, "b.height_m >= ?")
		args = append(args, q.MinHeightM)
	}
	if q.NamedOnly {
		where = append(where, "b.name IS NOT NULL")
	}
	if q.Text != "" {
		where = append(where, "b.name LIKE ? ESCAPE '\\'")
		args = append(args, likePattern(q.Text))
	}
	sqlText := "SELECT " + buildingColumns + " FROM " + from
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	switch q.Sort {
	case "levels":
		sqlText += " ORDER BY b.levels IS NULL, b.levels DESC, b.height_m DESC"
	case "footprint":
		sqlText += " ORDER BY b.area_m2 DESC"
	case "name":
		sqlText += " ORDER BY b.name IS NULL, b.name COLLATE NOCASE"
	default:
		sqlText += " ORDER BY b.height_m IS NULL, b.height_m DESC, b.levels DESC"
	}
	filterArea := q.Area != ""
	if filterArea {
		sqlText += " LIMIT -1"
	} else {
		sqlText += " LIMIT ? OFFSET ?"
		args = append(args, q.Limit, q.Offset)
	}
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Building{}
	skipped := 0
	for rows.Next() {
		b, err := scanBuilding(rows)
		if err != nil {
			return nil, err
		}
		if filterArea {
			if !s.areaShapes[q.Area].Contains(geo.Point{Lon: b.Lon, Lat: b.Lat}) {
				continue
			}
			if skipped < q.Offset {
				skipped++
				continue
			}
		}
		out = append(out, b)
		if filterArea && len(out) >= q.Limit {
			break
		}
	}
	return out, rows.Err()
}

// Building returns one building with tags.
func (s *Store) Building(ctx context.Context, id int64) (*Building, error) {
	var tags string
	b, err := scanBuilding(s.db.QueryRowContext(ctx,
		"SELECT "+buildingColumns+", b.tags FROM buildings b WHERE b.id = ?", id), &tags)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Tags = json.RawMessage(tags)
	return &b, nil
}
