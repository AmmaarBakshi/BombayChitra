package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/journey"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/routing"
)

// journeyNote says what journey times are based on.
const journeyNote = "Estimated from typical gaps between trains and running times, not from a timetable."

// journeyModes maps the modes a client can ask for to the planner's modes.
var journeyModes = map[string][]string{
	"train":    {"train"},
	"metro":    {"subway", "light_rail"},
	"monorail": {"monorail"},
}

// journeyStop is where a leg starts or ends: a station, or the journey's origin or destination.
type journeyStop struct {
	Name  string  `json:"name,omitempty"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Place int64   `json:"place,omitempty"`
}

type journeyLeg struct {
	Type      string        `json:"type"` // walk or ride
	From      journeyStop   `json:"from"`
	To        journeyStop   `json:"to"`
	DistanceM float64       `json:"distance_m"`
	DurationS float64       `json:"duration_s"`
	WaitS     float64       `json:"wait_s,omitempty"`
	Line      *journey.Line `json:"line,omitempty"`
	Route     int64         `json:"route,omitempty"`
	Headsign  string        `json:"headsign,omitempty"`
	Express   bool          `json:"express,omitempty"`
	Stops     int           `json:"stops,omitempty"`
	Via       []journeyStop `json:"via,omitempty"`
	Geometry  lineString    `json:"geometry"`
}

type journeyResponse struct {
	From      journeyStop  `json:"from"`
	To        journeyStop  `json:"to"`
	DurationS float64      `json:"duration_s"`
	DistanceM float64      `json:"distance_m"`
	WalkM     float64      `json:"walk_m"`
	Rides     int          `json:"rides"`
	Changes   int          `json:"changes"`
	StraightM float64      `json:"straight_m"`
	Legs      []journeyLeg `json:"legs"`
	BBox      []float64    `json:"bbox"`
	Note      string       `json:"note"`
}

func (s *Server) handleJourney(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	from, to := p.pair("from"), p.pair("to")
	maxWalk := p.float("max_walk", 1500, 200, 3000)
	format := p.oneOf("format", "json", "json", "geojson")
	var modes map[string]bool
	for _, m := range p.list("modes") {
		planner, ok := journeyModes[strings.ToLower(m)]
		if !ok {
			p.fail("modes must be train, metro or monorail")
			break
		}
		if modes == nil {
			modes = map[string]bool{}
		}
		for _, pm := range planner {
			modes[pm] = true
		}
	}
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	// The search time includes loading the network on first use and measuring the walks.
	start := time.Now()
	defer func() { s.metrics.routeDuration.Observe(time.Since(start).Seconds(), "transit") }()
	n, err := s.journeys.Network(r.Context())
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	j, err := n.Plan(*from, *to, journey.Options{MaxWalkM: maxWalk, Modes: modes})
	s.metrics.routeSearches.Inc("transit", journeyOutcome(err))
	switch {
	case errors.Is(err, journey.ErrNoStation):
		s.fail(w, r, http.StatusUnprocessableEntity, "no_station", "no station within "+formatMetres(maxWalk)+" of one of the points")
		return
	case errors.Is(err, journey.ErrNoJourney):
		s.fail(w, r, http.StatusUnprocessableEntity, "no_journey", "no train, metro or monorail journey connects the points")
		return
	case err != nil:
		s.handleErr(w, r, err)
		return
	}
	walks := s.measureWalks(r.Context(), n, j, *from, *to)
	out := describeJourney(n, j, *from, *to, walks)
	if format == "geojson" {
		fc, err := journeyFeatures(out)
		if err != nil {
			s.handleErr(w, r, err)
			return
		}
		s.writeGeoJSON(w, r, fc)
		return
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

// journeyFeatures presents a journey as GeoJSON: a line per leg (rides carry their line and colour), the stations
// where rides start and end, and the two ends of the journey with its totals on the "from" point.
func journeyFeatures(out journeyResponse) (featureCollection, error) {
	fc := featureCollection{Type: "FeatureCollection", Features: []feature{}}
	for i, l := range out.Legs {
		geometry, err := json.Marshal(l.Geometry)
		if err != nil {
			return fc, err
		}
		props := map[string]any{"role": "leg", "leg": i, "type": l.Type, "from": l.From.Name, "to": l.To.Name,
			"distance_m": l.DistanceM, "duration_s": l.DurationS}
		if l.Type == "ride" {
			props["line"], props["line_key"], props["mode"], props["colour"] = l.Line.Label, l.Line.Key, l.Line.Mode, l.Line.Colour
			props["route"], props["headsign"], props["express"] = l.Route, l.Headsign, l.Express
			props["stops"], props["wait_s"] = l.Stops, l.WaitS
		}
		fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: geometry, Properties: props})
		if l.Type == "ride" {
			for _, end := range []struct {
				role string
				stop journeyStop
			}{{"board", l.From}, {"alight", l.To}} {
				fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: pointGeometry(end.stop.Lon, end.stop.Lat),
					Properties: map[string]any{"role": end.role, "leg": i, "name": end.stop.Name, "place": end.stop.Place,
						"line": l.Line.Label, "colour": l.Line.Colour}})
			}
		}
	}
	fc.Features = append(fc.Features,
		feature{Type: "Feature", Geometry: pointGeometry(out.From.Lon, out.From.Lat), Properties: map[string]any{
			"role": "from", "duration_s": out.DurationS, "distance_m": out.DistanceM, "walk_m": out.WalkM,
			"rides": out.Rides, "changes": out.Changes, "note": out.Note}},
		feature{Type: "Feature", Geometry: pointGeometry(out.To.Lon, out.To.Lat), Properties: map[string]any{"role": "to"}},
	)
	return fc, nil
}

// measureWalks routes each walk along streets and footpaths with the foot router, replacing the planner's
// straight-line estimate, and returns the paths by leg. Walks the router cannot find (or a dataset without a routing
// graph) keep the estimate and a straight line.
func (s *Server) measureWalks(ctx context.Context, n *journey.Network, j *journey.Journey, from, to geo.Point) map[int][][2]float64 {
	paths := map[int][][2]float64{}
	if s.routes == nil {
		return paths
	}
	point := func(station int, end geo.Point) geo.Point {
		if station < 0 {
			return end
		}
		return n.Stations[station].Point()
	}
	for i, l := range j.Legs {
		if l.Ride {
			continue
		}
		a, b := point(l.From, from), point(l.To, to)
		res, err := s.routes.Route(ctx, a.Lon, a.Lat, b.Lon, b.Lat, routing.Options{Mode: routing.Foot})
		if err != nil {
			continue
		}
		// The router starts and ends on the nearest path; add the steps to and from it.
		offsets := res.From.DistM + res.To.DistM
		j.SetWalk(i, res.DistanceM+offsets, res.DurationS+offsets/journey.WalkSpeed)
		path := append([][2]float64{{a.Lon, a.Lat}}, res.Path...)
		paths[i] = append(path, [2]float64{b.Lon, b.Lat})
	}
	return paths
}

// describeJourney turns a planned journey into the API's response; walks are the measured walking paths by leg.
func describeJourney(n *journey.Network, j *journey.Journey, from, to geo.Point, walks map[int][][2]float64) journeyResponse {
	stop := func(i int, end geo.Point) journeyStop {
		if i < 0 {
			return journeyStop{Lat: end.Lat, Lon: end.Lon}
		}
		st := n.Stations[i]
		return journeyStop{Name: st.Name, Lat: round6(st.Lat), Lon: round6(st.Lon), Place: st.Place}
	}
	out := journeyResponse{
		From:      journeyStop{Lat: from.Lat, Lon: from.Lon},
		To:        journeyStop{Lat: to.Lat, Lon: to.Lon},
		DurationS: math.Round(j.DurationS),
		StraightM: math.Round(geo.Haversine(from, to)),
		Legs:      []journeyLeg{},
		Note:      journeyNote,
	}
	bbox := []float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for i, l := range j.Legs {
		leg := journeyLeg{Type: "walk", From: stop(l.From, from), To: stop(l.To, to),
			DistanceM: math.Round(l.DistanceM), DurationS: math.Round(l.DurationS)}
		var path [][2]float64
		if l.Ride {
			route, express := n.RouteOf(l)
			line := route.Line
			leg.Type, leg.Line, leg.Route, leg.Headsign, leg.Express = "ride", &line, route.ID, route.Headsign, express
			leg.WaitS, leg.Stops, leg.Via = math.Round(l.WaitS), l.ToStop-l.FromStop, []journeyStop{}
			for _, st := range n.Via(l) {
				leg.Via = append(leg.Via, stop(st, geo.Point{}))
			}
			path = n.RideGeometry(l)
			out.Rides++
		} else {
			path = walks[i]
			if path == nil {
				path = [][2]float64{{leg.From.Lon, leg.From.Lat}, {leg.To.Lon, leg.To.Lat}}
			}
			out.WalkM += leg.DistanceM
		}
		out.DistanceM += leg.DistanceM
		leg.Geometry = lineString{Type: "LineString", Coordinates: make([][2]float64, len(path))}
		for i, c := range path {
			leg.Geometry.Coordinates[i] = [2]float64{round6(c[0]), round6(c[1])}
			bbox[0], bbox[1] = math.Min(bbox[0], c[0]), math.Min(bbox[1], c[1])
			bbox[2], bbox[3] = math.Max(bbox[2], c[0]), math.Max(bbox[3], c[1])
		}
		out.Legs = append(out.Legs, leg)
	}
	out.Changes = max(0, out.Rides-1)
	out.BBox = []float64{round6(bbox[0]), round6(bbox[1]), round6(bbox[2]), round6(bbox[3])}
	return out
}

// journeyOutcome classifies a planning error for metrics.
func journeyOutcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, journey.ErrNoStation):
		return "no_station"
	case errors.Is(err, journey.ErrNoJourney):
		return "no_route"
	}
	return "error"
}

// formatMetres writes a distance for messages: "800 m", "1.5 km".
func formatMetres(m float64) string {
	if m < 1000 {
		return strconv.FormatFloat(math.Round(m), 'f', -1, 64) + " m"
	}
	return strconv.FormatFloat(math.Round(m/100)/10, 'f', -1, 64) + " km"
}
