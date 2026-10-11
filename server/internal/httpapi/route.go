package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/routing"
)

type routeEnd struct {
	Lat     float64      `json:"lat"`
	Lon     float64      `json:"lon"`
	Snapped routeSnapped `json:"snapped"`
}

type routeSnapped struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Road    string  `json:"road,omitempty"`
	OffsetM float64 `json:"offset_m"`
}

type routeRoad struct {
	Name      string  `json:"name,omitempty"`
	DistanceM float64 `json:"distance_m"`
}

type routeResponse struct {
	Optimize  string      `json:"optimize"`
	DistanceM float64     `json:"distance_m"`
	DurationS float64     `json:"duration_s"`
	StraightM float64     `json:"straight_m"`
	From      routeEnd    `json:"from"`
	To        routeEnd    `json:"to"`
	Roads     []routeRoad `json:"roads"`
	BBox      []float64   `json:"bbox"`
	Geometry  lineString  `json:"geometry"`
}

type lineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// pair reads a "lat,lon" parameter.
func (p *params) pair(name string) *geo.Point {
	v := p.str(name)
	if v == "" {
		p.fail("%s is required as lat,lon (e.g. 19.0544,72.8402)", name)
		return nil
	}
	lat, lon, ok := strings.Cut(v, ",")
	if !ok {
		p.fail("%s must be lat,lon", name)
		return nil
	}
	pt, err := geo.ParsePoint(lat, lon)
	if err != nil {
		p.fail("%s: %v", name, err)
		return nil
	}
	return &pt
}

func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	from, to := p.pair("from"), p.pair("to")
	opt := p.oneOf("optimize", "time", "time", "distance")
	format := p.oneOf("format", "json", "json", "geojson")
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	res, err := s.routes.Route(r.Context(), from.Lon, from.Lat, to.Lon, to.Lat, routing.Optimize(opt))
	switch {
	case errors.Is(err, routing.ErrNoRoad):
		s.fail(w, r, http.StatusUnprocessableEntity, "no_road", "no drivable road within 1.5 km of one of the points")
		return
	case errors.Is(err, routing.ErrNoRoute):
		s.fail(w, r, http.StatusUnprocessableEntity, "no_route", "the points are not connected by drivable roads")
		return
	case errors.Is(err, routing.ErrUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "routing_unavailable", "this dataset has no routing graph; rebuild it with the current pipeline")
		return
	case err != nil:
		s.handleErr(w, r, err)
		return
	}

	out := routeResponse{
		Optimize:  opt,
		DistanceM: math.Round(res.DistanceM),
		DurationS: math.Round(res.DurationS),
		StraightM: math.Round(geo.Haversine(*from, *to)),
		From:      end(*from, res.From),
		To:        end(*to, res.To),
		Roads:     []routeRoad{},
		Geometry:  lineString{Type: "LineString", Coordinates: make([][2]float64, len(res.Path))},
	}
	bbox := []float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for i, c := range res.Path {
		out.Geometry.Coordinates[i] = [2]float64{round6(c[0]), round6(c[1])}
		bbox[0], bbox[1] = math.Min(bbox[0], c[0]), math.Min(bbox[1], c[1])
		bbox[2], bbox[3] = math.Max(bbox[2], c[0]), math.Max(bbox[3], c[1])
	}
	out.BBox = []float64{round6(bbox[0]), round6(bbox[1]), round6(bbox[2]), round6(bbox[3])}
	for _, road := range res.Roads {
		if n := len(out.Roads); n > 0 && out.Roads[n-1].Name == road.Name {
			out.Roads[n-1].DistanceM += math.Round(road.DistanceM)
			continue
		}
		out.Roads = append(out.Roads, routeRoad{Name: road.Name, DistanceM: math.Round(road.DistanceM)})
	}

	if format == "geojson" {
		line, err := json.Marshal(out.Geometry)
		if err != nil {
			s.handleErr(w, r, err)
			return
		}
		s.writeGeoJSON(w, r, map[string]any{
			"type": "FeatureCollection",
			"features": []feature{
				{Type: "Feature", Geometry: line, Properties: map[string]any{"role": "route", "optimize": opt,
					"distance_m": out.DistanceM, "duration_s": out.DurationS, "straight_m": out.StraightM, "roads": out.Roads}},
				{Type: "Feature", Geometry: pointGeometry(out.From.Snapped.Lon, out.From.Snapped.Lat), Properties: map[string]any{"role": "from", "road": out.From.Snapped.Road}},
				{Type: "Feature", Geometry: pointGeometry(out.To.Snapped.Lon, out.To.Snapped.Lat), Properties: map[string]any{"role": "to", "road": out.To.Snapped.Road}},
			},
		})
		return
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

func end(requested geo.Point, snap routing.Snap) routeEnd {
	return routeEnd{
		Lat: requested.Lat,
		Lon: requested.Lon,
		Snapped: routeSnapped{Lat: round6(snap.Lat), Lon: round6(snap.Lon), Road: snap.Road,
			OffsetM: math.Round(snap.DistM)},
	}
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
