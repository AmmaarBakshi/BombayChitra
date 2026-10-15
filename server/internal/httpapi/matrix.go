package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/routing"
)

// Matrix size limits: each source is one search over the road graph.
const (
	maxMatrixSources      = 10
	maxMatrixDestinations = 50
	maxBodyBytes          = 64 << 10
)

type latLonPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type matrixRequest struct {
	Sources      []latLonPoint `json:"sources"`
	Destinations []latLonPoint `json:"destinations"`
	Mode         string        `json:"mode"`
	Optimize     string        `json:"optimize"`
	Avoid        []string      `json:"avoid"`
}

type matrixEnd struct {
	Lat     float64       `json:"lat"`
	Lon     float64       `json:"lon"`
	Snapped *routeSnapped `json:"snapped"` // null when no usable road is within 1.5 km
}

type matrixResponse struct {
	Mode         routing.Mode `json:"mode"`
	Optimize     string       `json:"optimize"`
	Durations    [][]*float64 `json:"durations"` // seconds, [source][destination]; null when unreachable
	Distances    [][]*float64 `json:"distances"` // metres
	Sources      []matrixEnd  `json:"sources"`
	Destinations []matrixEnd  `json:"destinations"`
}

// handleMatrix answers POST /api/v1/matrix: the travel time and distance from every source to every destination.
// Without destinations, the sources are used for both (a square matrix).
func (s *Server) handleMatrix(w http.ResponseWriter, r *http.Request) {
	var req matrixRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		s.badRequest(w, r, "the body must be JSON like {\"sources\": [{\"lat\": 19.12, \"lon\": 72.85}], \"destinations\": […]}: "+err.Error())
		return
	}
	if len(req.Destinations) == 0 {
		req.Destinations = req.Sources
	}
	mode, ok := modeAliases[strings.ToLower(req.Mode)]
	if req.Mode == "" {
		mode, ok = routing.Car, true
	}
	var problems []string
	if !ok {
		problems = append(problems, "mode must be car, two_wheeler or foot")
	}
	if req.Optimize == "" {
		req.Optimize = "time"
	}
	if req.Optimize != "time" && req.Optimize != "distance" {
		problems = append(problems, "optimize must be time or distance")
	}
	for _, a := range req.Avoid {
		if a != "toll" {
			problems = append(problems, "avoid must be toll")
		}
	}
	if n := len(req.Sources); n == 0 || n > maxMatrixSources {
		problems = append(problems, fmt.Sprintf("give 1 to %d sources", maxMatrixSources))
	}
	if n := len(req.Destinations); n > maxMatrixDestinations {
		problems = append(problems, fmt.Sprintf("give at most %d destinations", maxMatrixDestinations))
	}
	points := func(list []latLonPoint, name string) [][2]float64 {
		out := make([][2]float64, len(list))
		for i, p := range list {
			if math.Abs(p.Lat) > 90 || math.Abs(p.Lon) > 180 || p.Lat == 0 && p.Lon == 0 {
				problems = append(problems, fmt.Sprintf("%s[%d] must have a valid lat and lon", name, i))
			}
			out[i] = [2]float64{p.Lon, p.Lat}
		}
		return out
	}
	sources, destinations := points(req.Sources, "sources"), points(req.Destinations, "destinations")
	if len(problems) > 0 {
		s.badRequest(w, r, strings.Join(problems, "; "))
		return
	}

	start := time.Now()
	table, err := s.routes.Table(r.Context(), sources, destinations, routing.Options{Mode: mode, Optimize: routing.Optimize(req.Optimize), AvoidTolls: len(req.Avoid) > 0})
	s.metrics.routeDuration.Observe(time.Since(start).Seconds(), "matrix")
	s.metrics.routeSearches.Inc("matrix", outcome(err))
	switch {
	case errors.Is(err, routing.ErrModeUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "mode_unavailable", "this dataset only has the car network; rebuild it with the current pipeline for "+string(mode)+" matrices")
		return
	case errors.Is(err, routing.ErrUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "routing_unavailable", "this dataset has no routing graph; rebuild it with the current pipeline")
		return
	case err != nil:
		s.handleErr(w, r, err)
		return
	}

	out := matrixResponse{Mode: mode, Optimize: req.Optimize, Durations: rounded(table.Durations), Distances: rounded(table.Distances)}
	ends := func(list []latLonPoint, snaps []*routing.Snap) []matrixEnd {
		res := make([]matrixEnd, len(list))
		for i, p := range list {
			res[i] = matrixEnd{Lat: p.Lat, Lon: p.Lon}
			if sn := snaps[i]; sn != nil {
				res[i].Snapped = &routeSnapped{Lat: round6(sn.Lat), Lon: round6(sn.Lon), Road: sn.Road, OffsetM: math.Round(sn.DistM)}
			}
		}
		return res
	}
	out.Sources, out.Destinations = ends(req.Sources, table.Sources), ends(req.Destinations, table.Destinations)
	s.writeJSON(w, r, http.StatusOK, out)
}

// rounded rounds every value of a matrix to whole seconds or metres, keeping nulls.
func rounded(m [][]*float64) [][]*float64 {
	for _, row := range m {
		for j, v := range row {
			if v != nil {
				r := math.Round(*v)
				row[j] = &r
			}
		}
	}
	return m
}
