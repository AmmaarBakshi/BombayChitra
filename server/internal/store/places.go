package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// PlaceQuery filters places. All fields are optional and combine with AND.
type PlaceQuery struct {
	BBox       *geo.BBox
	Near       *geo.Point
	RadiusM    float64
	Categories []string
	Groups     []string
	Area       string
	Text       string
	NamedOnly  bool
	Limit      int
	Offset     int
}

// MaxNearCandidates bounds the work of radius queries.
const MaxNearCandidates = 20000

const placeColumns = `p.id, p.osm_type, p.osm_id, p.name, p.category, p.group_id, p.lon, p.lat, p.locality`

func scanPlace(sc interface{ Scan(...any) error }, p *Place, extra ...any) error {
	var name, locality sql.NullString
	dst := append([]any{&p.ID, &p.OSMType, &p.OSMID, &name, &p.Category, &p.Group, &p.Lon, &p.Lat, &locality}, extra...)
	if err := sc.Scan(dst...); err != nil {
		return err
	}
	p.Name, p.Locality = str(name), str(locality)
	return nil
}

// Places runs a filtered place query. Radius queries are ordered by distance; text queries by relevance; everything
// else by category importance then name.
func (s *Store) Places(ctx context.Context, q PlaceQuery) ([]Place, error) {
	var (
		from  = "places p"
		where []string
		args  []any
		order = "c.rank, p.name IS NULL, p.name COLLATE NOCASE, p.id"
	)
	bbox := q.BBox
	if q.Near != nil {
		around := geo.Around(*q.Near, q.RadiusM)
		bbox = &around
	}
	if bbox != nil {
		from += " JOIN places_rtree r ON r.id = p.id"
		where = append(where, "r.min_lon >= ? AND r.max_lon <= ? AND r.min_lat >= ? AND r.max_lat <= ?")
		args = append(args, bbox.MinLon, bbox.MaxLon, bbox.MinLat, bbox.MaxLat)
	}
	if q.Text != "" {
		match := MatchExpression(q.Text)
		if match == "" {
			return []Place{}, nil
		}
		from += ` JOIN (SELECT ref, bm25(search, 10.0, 4.0, 1.0) AS score FROM search
		                WHERE search MATCH ? AND kind = 'place') s ON s.ref = p.id`
		args = append([]any{match}, args...)
		order = "s.score, p.id"
	}
	from += " JOIN categories c ON c.id = p.category"
	if len(q.Categories) > 0 {
		where = append(where, "p.category IN ("+placeholders(len(q.Categories))+")")
		args = append(args, anySlice(q.Categories)...)
	}
	if len(q.Groups) > 0 {
		where = append(where, "p.group_id IN ("+placeholders(len(q.Groups))+")")
		args = append(args, anySlice(q.Groups)...)
	}
	if q.Area != "" {
		where = append(where, "p.id IN (SELECT member_id FROM area_members WHERE area_id = ? AND kind = 'place')")
		args = append(args, q.Area)
	}
	if q.NamedOnly {
		where = append(where, "p.name IS NOT NULL")
	}
	sqlText := "SELECT " + placeColumns + " FROM " + from
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	sqlText += " ORDER BY " + order

	if q.Near == nil {
		sqlText += " LIMIT ? OFFSET ?"
		args = append(args, q.Limit, q.Offset)
	} else {
		sqlText += " LIMIT ?"
		args = append(args, MaxNearCandidates)
	}

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Place{}
	for rows.Next() {
		var p Place
		if err := scanPlace(rows, &p); err != nil {
			return nil, err
		}
		if q.Near != nil {
			d := round(geo.Haversine(*q.Near, geo.Point{Lon: p.Lon, Lat: p.Lat}), 1)
			if d > q.RadiusM {
				continue
			}
			p.DistanceM = &d
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if q.Near != nil {
		sort.SliceStable(out, func(i, j int) bool { return *out[i].DistanceM < *out[j].DistanceM })
		start := min(q.Offset, len(out))
		end := min(start+q.Limit, len(out))
		out = out[start:end]
	}
	return out, nil
}

// Place returns the full record of one place.
func (s *Store) Place(ctx context.Context, id int64) (*PlaceDetail, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+placeColumns+`, p.name_local, p.address, p.postcode, p.phone, p.website, p.email,
		       p.opening_hours, p.operator, p.brand, p.cuisine, p.wheelchair, p.wikidata, p.wikipedia,
		       p.area_m2, p.extra, p.tags
		FROM places p WHERE p.id = ?`, id)
	var d PlaceDetail
	var nameLocal, address, postcode, phone, website, email, hours, operator, brand, cuisine, wheelchair, wikidata,
		wikipedia, extra sql.NullString
	var area sql.NullFloat64
	var tags string
	err := scanPlace(row, &d.Place, &nameLocal, &address, &postcode, &phone, &website, &email, &hours, &operator,
		&brand, &cuisine, &wheelchair, &wikidata, &wikipedia, &area, &extra, &tags)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.NameLocal, d.Address, d.Postcode, d.Phone = str(nameLocal), str(address), str(postcode), str(phone)
	d.Website, d.Email, d.OpeningHours, d.Operator = str(website), str(email), str(hours), str(operator)
	d.Brand, d.Cuisine, d.Wheelchair = str(brand), str(cuisine), str(wheelchair)
	d.Wikidata, d.Wikipedia = str(wikidata), str(wikipedia)
	d.AreaM2 = ptrFloat(area)
	d.Tags = json.RawMessage(tags)
	d.OSMURL = osmURL(d.ID)
	if c, ok := s.categories[d.Category]; ok {
		d.CategoryLabel = c.Label
	}
	if extra.Valid {
		var e struct {
			Lines json.RawMessage `json:"lines"`
		}
		if json.Unmarshal([]byte(extra.String), &e) == nil {
			d.Lines = e.Lines
		}
	}
	if d.Areas, err = s.memberAreas(ctx, "place", id); err != nil {
		return nil, err
	}
	return &d, nil
}

// EachPlace streams every place matching q (ignoring Limit/Offset); used by exports.
func (s *Store) EachPlace(ctx context.Context, q PlaceQuery, fn func(*PlaceDetail) error) error {
	q.Limit, q.Offset = -1, 0
	q.Near = nil
	places, err := s.Places(ctx, q)
	if err != nil {
		return err
	}
	for _, p := range places {
		d, err := s.Place(ctx, p.ID)
		if err != nil {
			return err
		}
		if err := fn(d); err != nil {
			return err
		}
	}
	return nil
}
