package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

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
	Mode      routing.Mode   `json:"mode"`
	Optimize  string         `json:"optimize"`
	Avoid     []string       `json:"avoid"`
	DistanceM float64        `json:"distance_m"`
	DurationS float64        `json:"duration_s"`
	StraightM float64        `json:"straight_m"`
	TollM     float64        `json:"toll_m"`
	From      routeEnd       `json:"from"`
	To        routeEnd       `json:"to"`
	Roads     []routeRoad    `json:"roads"`
	Steps     []routing.Step `json:"steps,omitempty"`
	// Hotspots are the waterlogging hotspots the route passes; Monsoon says whether they are in season now.
	Hotspots []routeHotspot `json:"hotspots"`
	Monsoon  bool           `json:"monsoon"`
	BBox     []float64      `json:"bbox"`
	Geometry lineString     `json:"geometry"`
}

// modeAliases accept everyday names for the travel modes.
var modeAliases = map[string]routing.Mode{
	"car": routing.Car, "drive": routing.Car, "driving": routing.Car,
	"two_wheeler": routing.TwoWheeler, "bike": routing.TwoWheeler, "motorcycle": routing.TwoWheeler, "scooter": routing.TwoWheeler,
	"foot": routing.Foot, "walk": routing.Foot, "walking": routing.Foot,
}

// travel describes a mode in error messages.
var travel = map[routing.Mode]string{routing.Car: "drivable road", routing.TwoWheeler: "road open to two-wheelers", routing.Foot: "road or path you can walk on"}

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
	mode, ok := modeAliases[strings.ToLower(p.str("mode"))]
	if p.str("mode") == "" {
		mode, ok = routing.Car, true
	}
	if !ok {
		p.fail("mode must be car, two_wheeler or foot")
	}
	avoid := []string{}
	for _, a := range p.list("avoid") {
		if a != "toll" {
			p.fail("avoid must be toll")
		}
		avoid = append(avoid, a)
	}
	withSteps := p.str("steps") == "" || p.boolean("steps")
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	start := time.Now()
	res, err := s.routes.Route(r.Context(), from.Lon, from.Lat, to.Lon, to.Lat,
		routing.Options{Mode: mode, Optimize: routing.Optimize(opt), AvoidTolls: len(avoid) > 0})
	s.metrics.routeDuration.Observe(time.Since(start).Seconds(), string(mode))
	s.metrics.routeSearches.Inc(string(mode), outcome(err))
	if res != nil {
		s.metrics.routeSettled.Observe(float64(res.Settled), string(mode))
	}
	switch {
	case errors.Is(err, routing.ErrNoRoad):
		s.fail(w, r, http.StatusUnprocessableEntity, "no_road", "no "+travel[mode]+" within 1.5 km of one of the points")
		return
	case errors.Is(err, routing.ErrNoRoute):
		msg := "the points are not connected by any " + travel[mode]
		if len(avoid) > 0 {
			msg += " without tolls"
		}
		s.fail(w, r, http.StatusUnprocessableEntity, "no_route", msg)
		return
	case errors.Is(err, routing.ErrModeUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "mode_unavailable", "this dataset only has the car network; rebuild it with the current pipeline for "+string(mode)+" routes")
		return
	case errors.Is(err, routing.ErrUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "routing_unavailable", "this dataset has no routing graph; rebuild it with the current pipeline")
		return
	case err != nil:
		s.handleErr(w, r, err)
		return
	}

	out := routeResponse{
		Mode:      mode,
		Optimize:  opt,
		Avoid:     avoid,
		DistanceM: math.Round(res.DistanceM),
		DurationS: math.Round(res.DurationS),
		StraightM: math.Round(geo.Haversine(*from, *to)),
		TollM:     math.Round(res.TollM),
		From:      end(*from, res.From),
		To:        end(*to, res.To),
		Roads:     []routeRoad{},
		Geometry:  lineString{Type: "LineString", Coordinates: make([][2]float64, len(res.Path))},
	}
	if withSteps {
		out.Steps = res.Steps
	}
	out.Hotspots = s.hotspotsAlong(res.Path)
	out.Monsoon = s.season().Active
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
				{Type: "Feature", Geometry: line, Properties: map[string]any{"role": "route", "mode": mode, "optimize": opt,
					"avoid": avoid, "distance_m": out.DistanceM, "duration_s": out.DurationS, "straight_m": out.StraightM,
					"toll_m": out.TollM, "roads": out.Roads, "steps": out.Steps, "hotspots": out.Hotspots, "monsoon": out.Monsoon}},
				{Type: "Feature", Geometry: pointGeometry(out.From.Snapped.Lon, out.From.Snapped.Lat), Properties: map[string]any{"role": "from", "road": out.From.Snapped.Road}},
				{Type: "Feature", Geometry: pointGeometry(out.To.Snapped.Lon, out.To.Snapped.Lat), Properties: map[string]any{"role": "to", "road": out.To.Snapped.Road}},
			},
		})
		return
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

// outcome classifies a routing error for metrics.
func outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, routing.ErrNoRoad):
		return "no_road"
	case errors.Is(err, routing.ErrNoRoute):
		return "no_route"
	case errors.Is(err, routing.ErrUnavailable):
		return "unavailable"
	}
	return "error"
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
