package httpapi

import (
	"math"
	"net/http"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

// ist is India Standard Time, which decides whether it is monsoon season.
var ist = time.FixedZone("IST", 5*3600+30*60)

func (s *Server) handleHeritage(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	status := p.oneOf("status", "", "unesco", "precinct", "monument")
	format := p.oneOf("format", "json", "json", "geojson")
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	items := []store.HeritageSite{}
	for _, h := range s.store.HeritageSites() {
		if status == "" || h.Status == status {
			items = append(items, h)
		}
	}
	if format == "geojson" {
		fc := featureCollection{Type: "FeatureCollection", Features: make([]feature, 0, len(items))}
		for _, h := range items {
			fc.Features = append(fc.Features, feature{Type: "Feature", ID: h.ID, Geometry: pointGeometry(h.Lon, h.Lat), Properties: h})
		}
		s.writeGeoJSON(w, r, fc)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

type seasonInfo struct {
	store.Season
	// Active is whether the season is on today (India Standard Time).
	Active bool `json:"active"`
}

func (s *Server) season() seasonInfo {
	season := s.store.WaterloggingSeason()
	return seasonInfo{Season: season, Active: season.Contains(time.Now().In(ist))}
}

func (s *Server) handleHotspots(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	format := p.oneOf("format", "json", "json", "geojson")
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	items := s.store.Hotspots()
	if items == nil {
		items = []store.Hotspot{}
	}
	if format == "geojson" {
		fc := featureCollection{Type: "FeatureCollection", Features: make([]feature, 0, len(items))}
		for _, h := range items {
			fc.Features = append(fc.Features, feature{Type: "Feature", ID: h.ID, Geometry: pointGeometry(h.Lon, h.Lat), Properties: h})
		}
		s.writeGeoJSON(w, r, fc)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"season": s.season(), "count": len(items), "items": items})
}

// routeHotspot is a waterlogging hotspot a route passes.
type routeHotspot struct {
	store.Hotspot
	DistanceM float64 `json:"distance_m"` // closest approach of the route
}

// hotspotsAlong returns the hotspots within their warning radius of a path (lon, lat pairs).
func (s *Server) hotspotsAlong(path [][2]float64) []routeHotspot {
	out := []routeHotspot{}
	if len(path) == 0 {
		return out
	}
	minLon, minLat, maxLon, maxLat := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range path {
		minLon, minLat = math.Min(minLon, c[0]), math.Min(minLat, c[1])
		maxLon, maxLat = math.Max(maxLon, c[0]), math.Max(maxLat, c[1])
	}
	for _, h := range s.store.Hotspots() {
		pad := h.RadiusM / 100_000 // generous: 1° is more than 100 km here
		if h.Lon < minLon-pad || h.Lon > maxLon+pad || h.Lat < minLat-pad || h.Lat > maxLat+pad {
			continue
		}
		if d := distanceToPath(h.Lon, h.Lat, path); d <= h.RadiusM {
			out = append(out, routeHotspot{Hotspot: h, DistanceM: math.Round(d)})
		}
	}
	return out
}

// distanceToPath is the distance in metres from a point to a polyline, in a local equirectangular projection.
func distanceToPath(lon, lat float64, path [][2]float64) float64 {
	kx := 111_320 * math.Cos(lat*math.Pi/180)
	const ky = 110_574.0
	best := math.Inf(1)
	for i := range path {
		ax, ay := (path[i][0]-lon)*kx, (path[i][1]-lat)*ky
		if i == 0 {
			best = math.Hypot(ax, ay)
			continue
		}
		bx, by := (path[i-1][0]-lon)*kx, (path[i-1][1]-lat)*ky
		dx, dy := ax-bx, ay-by
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, -(bx*dx+by*dy)/l))
		}
		best = math.Min(best, math.Hypot(bx+t*dx, by+t*dy))
	}
	return best
}
