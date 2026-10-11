package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// SearchKinds are the entity kinds stored in the search index.
var SearchKinds = []string{"area", "locality", "line", "place", "street", "bridge"}

const maxSearchTokens = 8

// MatchExpression converts free text into a safe FTS5 query: every token must match and the last one is a prefix,
// so "andheri sta" finds "Andheri Station". Returns "" when the text has no searchable characters.
func MatchExpression(text string) string {
	tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.Is(unicode.Mn, r) && !unicode.Is(unicode.Mc, r)
	})
	if len(tokens) > maxSearchTokens {
		tokens = tokens[:maxSearchTokens]
	}
	parts := make([]string, 0, len(tokens))
	for i, t := range tokens {
		q := `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
		if i == len(tokens)-1 {
			q += "*"
		}
		parts = append(parts, q)
	}
	return strings.Join(parts, " ")
}

func normalise(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// Search queries the unified index (areas, localities, transit lines, places, streets, bridges) and ranks results by
// text relevance, importance, name match quality and — when near is given — proximity.
func (s *Store) Search(ctx context.Context, text string, near *geo.Point, kinds []string, limit int) ([]SearchResult, error) {
	match := MatchExpression(text)
	if match == "" {
		return []SearchResult{}, nil
	}
	sqlText := `SELECT kind, ref, name, detail, category, lon, lat, weight, bm25(search, 10.0, 4.0, 1.0)
		FROM search WHERE search MATCH ?`
	args := []any{match}
	if len(kinds) > 0 {
		sqlText += " AND kind IN (" + placeholders(len(kinds)) + ")"
		args = append(args, anySlice(kinds)...)
	}
	sqlText += " ORDER BY bm25(search, 10.0, 4.0, 1.0) LIMIT 400"
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	query := normalise(text)
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var ref any
		var detail, category sql.NullString
		var weight, bm25 float64
		if err := rows.Scan(&r.Kind, &ref, &r.Name, &detail, &category, &r.Lon, &r.Lat, &weight, &bm25); err != nil {
			return nil, err
		}
		r.Detail, r.Category = str(detail), str(category)
		r.ID = normaliseRef(ref)
		score := -bm25 * (1 + 0.15*-weight)
		name := normalise(r.Name)
		switch {
		case name == query:
			score *= 2.2
		case strings.HasPrefix(name, query):
			score *= 1.6
		case strings.Contains(name, query):
			score *= 1.15
		}
		if near != nil {
			d := round(geo.Haversine(*near, geo.Point{Lon: r.Lon, Lat: r.Lat}), 0)
			r.DistanceM = &d
			score *= 1 + 1.2*math.Exp(-d/4000)
		}
		r.score = score
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })
	results = dedupe(results, limit)
	if err := s.attachBBoxes(ctx, results); err != nil {
		return nil, err
	}
	return results, nil
}

// dedupe drops repeated hits for the same thing: e.g. the two bus stops named "Lilavati Hospital" on either side of
// the road, or a flyover that is both a street and a bridge. Results arrive best-first, so the kept hit is the best.
func dedupe(results []SearchResult, limit int) []SearchResult {
	out := make([]SearchResult, 0, min(limit, len(results)))
	for _, r := range results {
		dup := false
		for _, o := range out {
			if normalise(o.Name) != normalise(r.Name) {
				continue
			}
			// Same name and kind nearby is the same thing (e.g. a station, its bus stop and its building all called
			// "Andheri"); the higher-ranked hit is kept.
			sameThing := o.Kind == r.Kind
			crossKind := (o.Kind == "street" && r.Kind == "bridge") || (o.Kind == "bridge" && r.Kind == "street")
			if (sameThing || crossKind) &&
				geo.Haversine(geo.Point{Lon: o.Lon, Lat: o.Lat}, geo.Point{Lon: r.Lon, Lat: r.Lat}) < 400 {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, r)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func normaliseRef(ref any) any {
	switch v := ref.(type) {
	case []byte:
		return string(v)
	case float64:
		return int64(v)
	}
	return ref
}

// attachBBoxes adds bounding boxes to results that represent extended features, so clients can fit the map to them.
func (s *Store) attachBBoxes(ctx context.Context, results []SearchResult) error {
	tables := map[string]string{"street": "streets", "bridge": "named_bridges", "line": "transit_lines"}
	for i := range results {
		r := &results[i]
		if r.Kind == "area" {
			if a, ok := s.areaByID[fmt.Sprint(r.ID)]; ok {
				r.BBox = a.BBox
			}
			continue
		}
		table, ok := tables[r.Kind]
		if !ok {
			continue
		}
		var b [4]float64
		err := s.db.QueryRowContext(ctx,
			"SELECT min_lon, min_lat, max_lon, max_lat FROM "+table+" WHERE id = ?", r.ID).
			Scan(&b[0], &b[1], &b[2], &b[3])
		if err == nil {
			r.BBox = b[:]
		} else if err != sql.ErrNoRows {
			return err
		}
	}
	return nil
}
