// Package store provides read-only, query-oriented access to bombaychitra.db.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
	_ "modernc.org/sqlite" // registers the "sqlite" driver (pure Go, no cgo)
)

// SupportedSchema is the database schema version this server understands.
const SupportedSchema = 2

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps the database plus a few small tables cached in memory at startup.
type Store struct {
	db         *sql.DB
	meta       map[string]string
	areas      []Area
	areaByID   map[string]*Area
	areaShapes map[string]*geo.PolygonIndex
	groups     []CategoryGroup
	categories map[string]Category
	pins       pinIndex // PIN code areas, derived from addresses on first use
}

// Open opens the database read-only. The file is treated as immutable: rebuild it and restart the server to publish
// new data.
func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("immutable", "1")
	q.Add("_pragma", "query_only(1)")
	q.Add("_pragma", "mmap_size(268435456)")
	q.Add("_pragma", "cache_size(-32768)")
	q.Add("_pragma", "temp_store(memory)")
	escaper := strings.NewReplacer("%", "%25", " ", "%20", "?", "%3f", "#", "%23")
	dsn := "file:" + escaper.Replace(filepath.ToSlash(abs)) + "?" + q.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(max(4, runtime.NumCPU()*2))
	db.SetMaxIdleConns(max(4, runtime.NumCPU()*2))
	s := &Store{db: db}
	if err := s.load(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return s, nil
}

// Ping runs a trivial query to check the database is readable.
func (s *Store) Ping(ctx context.Context) error {
	var one int
	return s.db.QueryRowContext(ctx, `SELECT 1 FROM meta LIMIT 1`).Scan(&one)
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying handle for exports.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) load(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM meta`)
	if err != nil {
		return err
	}
	s.meta = map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return err
		}
		s.meta[k] = v
	}
	rows.Close()
	if v, _ := strconv.Atoi(s.meta["schema_version"]); v != SupportedSchema {
		return fmt.Errorf("database schema %q, server supports %d", s.meta["schema_version"], SupportedSchema)
	}
	if err := s.loadCategories(ctx); err != nil {
		return err
	}
	return s.loadAreas(ctx)
}

// Meta returns the raw key/value metadata table.
func (s *Store) Meta() map[string]string { return s.meta }

// Version identifies the dataset build; used for cache validators.
func (s *Store) Version() string { return s.meta["built_at"] }

// MetaJSON decodes a JSON-valued metadata entry (e.g. bounds, center).
func (s *Store) MetaJSON(key string, dst any) error {
	v, ok := s.meta[key]
	if !ok {
		return ErrNotFound
	}
	return json.Unmarshal([]byte(v), dst)
}

func (s *Store) loadCategories(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.label, g.colour, c.id, c.label, c.icon, c.rank, c.minzoom, c.count
		FROM categories c JOIN category_groups g ON g.id = c.group_id
		ORDER BY g.sort, c.sort`)
	if err != nil {
		return err
	}
	defer rows.Close()
	s.categories = map[string]Category{}
	byGroup := map[string]int{}
	for rows.Next() {
		var g CategoryGroup
		var c Category
		if err := rows.Scan(&g.ID, &g.Label, &g.Colour, &c.ID, &c.Label, &c.Icon, &c.Rank, &c.MinZoom, &c.Count); err != nil {
			return err
		}
		c.Group = g.ID
		idx, ok := byGroup[g.ID]
		if !ok {
			idx = len(s.groups)
			byGroup[g.ID] = idx
			s.groups = append(s.groups, g)
		}
		s.groups[idx].Categories = append(s.groups[idx].Categories, c)
		s.groups[idx].Count += c.Count
		s.categories[c.ID] = c
	}
	return rows.Err()
}

// Groups returns category groups in display order.
func (s *Store) Groups() []CategoryGroup { return s.groups }

// Category looks up one category.
func (s *Store) Category(id string) (Category, bool) {
	c, ok := s.categories[id]
	return c, ok
}

// GroupExists reports whether a category group id is known.
func (s *Store) GroupExists(id string) bool {
	for _, g := range s.groups {
		if g.ID == id {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- helpers

func str(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func ptrFloat(nf sql.NullFloat64) *float64 {
	if nf.Valid {
		v := nf.Float64
		return &v
	}
	return nil
}

func ptrInt(ni sql.NullInt64) *int64 {
	if ni.Valid {
		v := ni.Int64
		return &v
	}
	return nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func anySlice[T any](items []T) []any {
	out := make([]any, len(items))
	for i, v := range items {
		out[i] = v
	}
	return out
}

func round(v float64, decimals int) float64 {
	p := 1.0
	for range decimals {
		p *= 10
	}
	if v < 0 {
		return -float64(int64(-v*p+0.5)) / p
	}
	return float64(int64(v*p+0.5)) / p
}

// overtureType is the id type digit of records added from Overture Maps (OpenStreetMap uses 1–3).
const overtureType = 4

// osmURL returns the openstreetmap.org page of an element id encoded as osm_id*10+type.
func osmURL(id int64) string {
	kind := map[int64]string{1: "node", 2: "way", 3: "relation"}[id%10]
	if kind == "" {
		return ""
	}
	return "https://www.openstreetmap.org/" + kind + "/" + strconv.FormatInt(id/10, 10)
}
