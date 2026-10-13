package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// HeritageSite is a curated heritage site: a UNESCO World Heritage Site, a heritage precinct or a listed monument.
type HeritageSite struct {
	ID string `json:"id"`
	// Status is unesco, precinct or monument.
	Status      string  `json:"status"`
	Name        string  `json:"name"`
	Year        *int64  `json:"year,omitempty"` // of UNESCO inscription
	PlaceID     *int64  `json:"place_id,omitempty"`
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	Description string  `json:"description,omitempty"`
	Wikipedia   string  `json:"wikipedia,omitempty"`
}

// Hotspot is a spot that floods in heavy rain.
type Hotspot struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Name        string  `json:"name"`
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	RadiusM     float64 `json:"radius_m"`
	Description string  `json:"description,omitempty"`
}

// Season is the months (1–12, inclusive) when waterlogging hotspots flood.
type Season struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Contains reports whether a date's month is in the season.
func (s Season) Contains(t time.Time) bool {
	m := int(t.Month())
	if s.From <= s.To {
		return m >= s.From && m <= s.To
	}
	return m >= s.From || m <= s.To // a season across the new year
}

// loadCurated reads the small curated tables into memory; databases built before they existed have none.
func (s *Store) loadCurated(ctx context.Context) error {
	s.season = Season{From: 6, To: 9}
	if from, to, ok := strings.Cut(s.meta["waterlogging_season"], "-"); ok {
		f, err1 := strconv.Atoi(from)
		t, err2 := strconv.Atoi(to)
		if err1 == nil && err2 == nil {
			s.season = Season{From: f, To: t}
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, status, name, year, place_id, lon, lat, description, wikipedia
		FROM heritage_sites ORDER BY CASE status WHEN 'unesco' THEN 0 WHEN 'precinct' THEN 1 ELSE 2 END, name`)
	if err != nil {
		if !missingTable(err) {
			return err
		}
	} else {
		defer rows.Close()
		for rows.Next() {
			var h HeritageSite
			var year, place sql.NullInt64
			var desc, wiki sql.NullString
			if err := rows.Scan(&h.ID, &h.Status, &h.Name, &year, &place, &h.Lon, &h.Lat, &desc, &wiki); err != nil {
				return err
			}
			h.Year, h.PlaceID, h.Description, h.Wikipedia = ptrInt(year), ptrInt(place), str(desc), str(wiki)
			s.heritage = append(s.heritage, h)
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}
	hot, err := s.db.QueryContext(ctx, `SELECT id, kind, name, lon, lat, radius_m, description FROM hotspots ORDER BY name`)
	if err != nil {
		if missingTable(err) {
			return nil
		}
		return err
	}
	defer hot.Close()
	for hot.Next() {
		var h Hotspot
		var desc sql.NullString
		if err := hot.Scan(&h.ID, &h.Kind, &h.Name, &h.Lon, &h.Lat, &h.RadiusM, &desc); err != nil {
			return err
		}
		h.Description = str(desc)
		s.hotspots = append(s.hotspots, h)
	}
	return hot.Err()
}

func missingTable(err error) bool { return strings.Contains(err.Error(), "no such table") }

// HeritageSites lists the curated heritage sites: World Heritage Sites first, then precincts and monuments.
func (s *Store) HeritageSites() []HeritageSite { return s.heritage }

// HeritageFor returns the heritage site a place represents, if any.
func (s *Store) HeritageFor(placeID int64) *HeritageSite {
	for i := range s.heritage {
		if p := s.heritage[i].PlaceID; p != nil && *p == placeID {
			return &s.heritage[i]
		}
	}
	return nil
}

// Hotspots lists the waterlogging hotspots.
func (s *Store) Hotspots() []Hotspot { return s.hotspots }

// WaterloggingSeason is when the hotspots flood (the monsoon, June to September).
func (s *Store) WaterloggingSeason() Season { return s.season }
