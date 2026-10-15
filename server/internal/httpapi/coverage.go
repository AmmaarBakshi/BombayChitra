package httpapi

import (
	"cmp"
	"encoding/json"
	"math"
	"net/http"
	"slices"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

// coverageLevel classifies a cell by the share of its known places that OpenStreetMap has: where Overture Maps
// knows many places OpenStreetMap lacks, mappers are most needed. Cells with too few places to judge are "few".
func coverageLevel(osm, overture int) (share *float64, level string) {
	total := osm + overture
	if total == 0 {
		return nil, "few"
	}
	s := math.Round(float64(osm)/float64(total)*100) / 100
	switch {
	case total < 5:
		level = "few"
	case s < 0.4:
		level = "thin"
	case s < 0.7:
		level = "partial"
	default:
		level = "good"
	}
	return &s, level
}

// handleCoverage returns a grid over the region (cell metres square, default 1000) with, per cell, the places in
// OpenStreetMap and those only Overture Maps knows, OpenStreetMap's share of them and a level from thin to good — a
// map of where OpenStreetMap needs mappers. Cells with nothing in them are left out. The grid is computed once per
// cell size.
func (s *Server) handleCoverage(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	cellM := p.intRange("cell", 1000, 500, 5000)
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	if cached, ok := s.coverage.Load(cellM); ok {
		s.writeGeoJSON(w, r, cached)
		return
	}
	var bounds [4]float64
	if err := json.Unmarshal([]byte(s.store.Meta()["bounds"]), &bounds); err != nil {
		s.handleErr(w, r, err)
		return
	}
	grid := geo.NewGrid(geo.Point{Lon: bounds[0], Lat: (bounds[1] + bounds[3]) / 2}, float64(cellM))
	grid.Lat0 = bounds[1]
	cells, err := s.store.Coverage(r.Context(), grid.Lon0, grid.Lat0, grid.DX, grid.DY)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	// South to north, west to east, so the output is stable.
	slices.SortFunc(cells, func(a, b store.CoverageCell) int { return cmp.Or(cmp.Compare(a.Y, b.Y), cmp.Compare(a.X, b.X)) })
	fc := featureCollection{Type: "FeatureCollection", Features: make([]feature, 0, len(cells))}
	for _, c := range cells {
		x0, y0 := grid.Lon0+float64(c.X)*grid.DX, grid.Lat0+float64(c.Y)*grid.DY
		x1, y1 := x0+grid.DX, y0+grid.DY
		ring := [][2]float64{{round6(x0), round6(y0)}, {round6(x1), round6(y0)}, {round6(x1), round6(y1)}, {round6(x0), round6(y1)}, {round6(x0), round6(y0)}}
		geometry, err := json.Marshal(map[string]any{"type": "Polygon", "coordinates": [][][2]float64{ring}})
		if err != nil {
			s.handleErr(w, r, err)
			return
		}
		share, level := coverageLevel(c.OSMPlaces, c.OverturePlaces)
		fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: geometry, Properties: map[string]any{
			"osm_places": c.OSMPlaces, "overture_places": c.OverturePlaces, "buildings": c.Buildings,
			"osm_share": share, "level": level,
		}})
	}
	s.coverage.Store(cellM, fc)
	s.writeGeoJSON(w, r, fc)
}
