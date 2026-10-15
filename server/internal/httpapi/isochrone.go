package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/routing"
)

// isochroneCellM is the grid an isochrone is drawn on, by mode: fine enough on foot to follow the streets, coarser
// for the larger areas reached by road.
var isochroneCellM = map[routing.Mode]float64{routing.Foot: 75, routing.TwoWheeler: 200, routing.Car: 200}

// handleIsochrone returns the areas reachable from a point within one or more times, as GeoJSON polygons — one
// feature per time, largest first — and the snapped start.
func (s *Server) handleIsochrone(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	from := p.pair("from")
	mode, ok := modeAliases[strings.ToLower(p.str("mode"))]
	if p.str("mode") == "" {
		mode, ok = routing.Car, true
	}
	if !ok {
		p.fail("mode must be car, two_wheeler or foot")
	}
	minutes := []int{10, 20, 30}
	if raw := p.list("minutes"); len(raw) > 0 {
		minutes = minutes[:0]
		for _, v := range raw {
			m, err := strconv.Atoi(v)
			if err != nil || m < 1 || m > 60 {
				p.fail("minutes must be whole minutes from 1 to 60")
				break
			}
			if !slices.Contains(minutes, m) {
				minutes = append(minutes, m)
			}
		}
		if len(minutes) > 4 {
			p.fail("at most four times in minutes")
		}
	}
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	slices.Sort(minutes)
	limit := float64(minutes[len(minutes)-1] * 60)

	// The quickest time to any point of each grid cell, then each cell grown by its neighbours so the area covers
	// the land beside the roads.
	cellM := isochroneCellM[mode]
	grid := geo.NewGrid(*from, cellM)
	reached := map[geo.Cell]float64{}
	start := time.Now()
	snap, _, err := s.routes.Reach(r.Context(), from.Lon, from.Lat, mode, limit, func(lon, lat, seconds float64) {
		c := grid.CellOf(geo.Point{Lon: lon, Lat: lat})
		if t, ok := reached[c]; !ok || seconds < t {
			reached[c] = seconds
		}
	})
	s.metrics.routeDuration.Observe(time.Since(start).Seconds(), "isochrone")
	switch {
	case errors.Is(err, routing.ErrNoRoad):
		s.fail(w, r, http.StatusUnprocessableEntity, "no_road", "no "+travel[mode]+" within 1.5 km of the point")
		return
	case errors.Is(err, routing.ErrModeUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "mode_unavailable", "this dataset only has the car network; rebuild it with the current pipeline for "+string(mode)+" isochrones")
		return
	case errors.Is(err, routing.ErrUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "routing_unavailable", "this dataset has no routing graph; rebuild it with the current pipeline")
		return
	case err != nil:
		s.handleErr(w, r, err)
		return
	}
	grown := map[geo.Cell]float64{}
	for c, t := range reached {
		for dx := int32(-1); dx <= 1; dx++ {
			for dy := int32(-1); dy <= 1; dy++ {
				n := geo.Cell{X: c.X + dx, Y: c.Y + dy}
				if g, ok := grown[n]; !ok || t < g {
					grown[n] = t
				}
			}
		}
	}

	fc := featureCollection{Type: "FeatureCollection", Features: []feature{}}
	for i := len(minutes) - 1; i >= 0; i-- {
		within := map[geo.Cell]bool{}
		for c, t := range grown {
			if t <= float64(minutes[i]*60) {
				within[c] = true
			}
		}
		polygons := grid.Outline(within)
		for _, poly := range polygons {
			for _, ring := range poly {
				for k := range ring {
					ring[k] = [2]float64{round6(ring[k][0]), round6(ring[k][1])}
				}
			}
		}
		geometry, err := json.Marshal(map[string]any{"type": "MultiPolygon", "coordinates": polygons})
		if err != nil {
			s.handleErr(w, r, err)
			return
		}
		fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: geometry, Properties: map[string]any{
			"role": "isochrone", "minutes": minutes[i], "mode": mode,
			"area_km2": math.Round(float64(len(within))*cellM*cellM/1e4) / 100,
		}})
	}
	fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: pointGeometry(round6(snap.Lon), round6(snap.Lat)),
		Properties: map[string]any{"role": "from", "road": snap.Road, "offset_m": math.Round(snap.DistM)}})
	s.writeGeoJSON(w, r, fc)
}
