package journey

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// placeM is how far a station's place (its card in the app) may be from the station's stops.
const placeM = 350.0

// stationCategory is the place category of each mode's stations.
var stationCategory = map[string]string{"train": "railway_station", "subway": "metro_station",
	"light_rail": "metro_station", "monorail": "monorail_station"}

// Service loads the network from the database on first use.
type Service struct {
	db   *sql.DB
	once sync.Once
	net  *Network
	err  error
	done atomic.Bool
}

// NewService creates a lazily loading journey planner over the dataset database.
func NewService(db *sql.DB) *Service { return &Service{db: db} }

// Network returns the network, building it on the first call.
func (s *Service) Network(ctx context.Context) (*Network, error) {
	s.once.Do(func() {
		s.net, s.err = Load(context.WithoutCancel(ctx), s.db)
		s.done.Store(true)
	})
	return s.net, s.err
}

// Peek returns the network without building it: ok is false until the first journey request has built it (or failed).
func (s *Service) Peek() (n *Network, ok bool, err error) {
	if !s.done.Load() {
		return nil, false, nil
	}
	return s.net, true, s.err
}

// Load builds the network from the lines in service in transit_lines, with the gaps between trains recorded in
// meta.transit_headways (defaults for older datasets), and links each station to its place.
func Load(ctx context.Context, db *sql.DB) (*Network, error) {
	var headways struct {
		Modes map[string]float64 `json:"modes"`
		Lines map[string]float64 `json:"lines"`
	}
	var raw string
	switch err := db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'transit_headways'`).Scan(&raw); {
	case err == nil:
		if err := json.Unmarshal([]byte(raw), &headways); err != nil {
			return nil, fmt.Errorf("meta.transit_headways: %w", err)
		}
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `SELECT id, line_key, label, ref, mode, network, colour, to_name, stops, geometry
		FROM transit_lines WHERE state IS NULL AND mode IN (`+strings.TrimSuffix(strings.Repeat("?,", len(Modes)), ",")+`)
		ORDER BY id`, anySlice(Modes)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routes []Route
	for rows.Next() {
		var r Route
		var ref, network, colour, to sql.NullString
		var stops, geometry string
		if err := rows.Scan(&r.ID, &r.Line.Key, &r.Line.Label, &ref, &r.Line.Mode, &network, &colour, &to, &stops,
			&geometry); err != nil {
			return nil, err
		}
		r.Line.Ref, r.Line.Network, r.Line.Colour, r.Headsign = ref.String, network.String, colour.String, to.String
		var parsed []struct {
			ID   int64   `json:"id"`
			Name string  `json:"name"`
			Lon  float64 `json:"lon"`
			Lat  float64 `json:"lat"`
		}
		if err := json.Unmarshal([]byte(stops), &parsed); err != nil {
			return nil, fmt.Errorf("transit line %d stops: %w", r.ID, err)
		}
		for _, s := range parsed {
			r.Stops = append(r.Stops, Stop{ID: s.ID, Name: s.Name, Lon: s.Lon, Lat: s.Lat})
		}
		var g geo.Geometry
		if err := json.Unmarshal([]byte(geometry), &g); err == nil {
			r.Track, _ = g.Lines()
		}
		r.HeadwayMin = headways.Lines[r.Line.Key]
		if r.HeadwayMin == 0 {
			r.HeadwayMin = headways.Modes[r.Line.Mode]
		}
		routes = append(routes, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	n := Build(routes)
	return n, linkPlaces(ctx, db, n)
}

// linkPlaces gives each station the nearest place of its mode's station category.
func linkPlaces(ctx context.Context, db *sql.DB, n *Network) error {
	rows, err := db.QueryContext(ctx, `SELECT id, category, lon, lat FROM places
		WHERE category IN ('railway_station', 'metro_station', 'monorail_station')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	best := make([]float64, len(n.Stations))
	for i := range best {
		best[i] = placeM
	}
	for rows.Next() {
		var id int64
		var category string
		var p geo.Point
		if err := rows.Scan(&id, &category, &p.Lon, &p.Lat); err != nil {
			return err
		}
		for i := range n.Stations {
			st := &n.Stations[i]
			if stationCategory[st.Mode] != category {
				continue
			}
			if d := geo.Haversine(st.Point(), p); d < best[i] {
				best[i], st.Place = d, id
			}
		}
	}
	return rows.Err()
}

func anySlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
