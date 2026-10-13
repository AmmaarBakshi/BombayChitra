package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"name":    "BombayChitra API",
		"version": "v1",
		"docs":    base + "/docs/api/",
		"openapi": base + "/openapi.json",
		"endpoints": []string{
			"/api/v1/health", "/api/v1/meta", "/api/v1/categories", "/api/v1/areas", "/api/v1/areas/{id}", "/api/v1/stats",
			"/api/v1/search", "/api/v1/reverse", "/api/v1/places", "/api/v1/places/{id}", "/api/v1/nearby",
			"/api/v1/streets", "/api/v1/streets/{id}", "/api/v1/roads/{id}", "/api/v1/bridges",
			"/api/v1/bridges/{id}", "/api/v1/bridges/named", "/api/v1/bridges/named/{id}", "/api/v1/buildings",
			"/api/v1/buildings/{id}", "/api/v1/transit/lines", "/api/v1/transit/routes/{id}",
			"/api/v1/transit/stations", "/api/v1/route", "/tiles/bombaychitra.json", "/styles/light.json", "/styles/dark.json",
		},
	})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	m := s.store.Meta()
	base := s.baseURL(r)
	var bounds, center []float64
	_ = s.store.MetaJSON("bounds", &bounds)
	_ = s.store.MetaJSON("center", &center)
	zoom, _ := strconv.Atoi(m["zoom"])
	maxzoom, _ := strconv.Atoi(m["maxzoom"])
	counts := map[string]int64{}
	for k, v := range m {
		if name, ok := strings.CutPrefix(k, "count_"); ok {
			counts[name], _ = strconv.ParseInt(v, 10, 64)
		}
	}
	links := map[string]any{
		"tilejson": base + "/tiles/bombaychitra.json",
		"tiles":    base + "/tiles/{z}/{x}/{y}.mvt",
		"styles":   map[string]string{"light": base + "/styles/light.json", "dark": base + "/styles/dark.json"},
		"openapi":  base + "/openapi.json",
		"docs":     base + "/docs/",
	}
	if s.cfg.Downloads {
		links["downloads"] = map[string]string{
			"pmtiles": base + "/data/bombaychitra.pmtiles",
			"sqlite":  base + "/data/bombaychitra.db",
		}
	}
	// Routing modes come from the build metadata, so answering does not load the graph.
	routingInfo := map[string]any{"available": m["routing_version"] != ""}
	if v, err := strconv.Atoi(m["routing_version"]); err == nil {
		routingInfo["version"] = v
		modes := []string{"car"}
		if v >= 2 && m["routing_profile"] != "" {
			modes = strings.Split(m["routing_profile"], ",")
		}
		routingInfo["modes"] = modes
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"name":           "BombayChitra",
		"server":         Version,
		"routing":        routingInfo,
		"region":         map[string]any{"id": m["region_id"], "name": m["region_name"], "description": m["description"], "bounds": bounds, "center": center, "zoom": zoom, "maxzoom": maxzoom},
		"dataset":        map[string]any{"built_at": m["built_at"], "source": m["source_name"], "source_url": m["source_url"], "source_timestamp": m["source_timestamp"], "schema_version": m["schema_version"], "generator": m["generator"]},
		"counts":         counts,
		"attribution":    m["attribution"],
		"license":        m["license"],
		"links":          links,
		"areas":          s.store.Areas(),
		"category_count": len(s.allCategories()),
	})
}

func (s *Server) allCategories() []store.Category {
	var out []store.Category
	for _, g := range s.store.Groups() {
		out = append(out, g.Categories...)
	}
	return out
}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, r, http.StatusOK, map[string]any{"groups": s.store.Groups()})
}

func (s *Server) handleAreas(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	kind := p.str("kind")
	if kind == "" {
		kind = p.str("type") // the name the roadmap and early clients used
	}
	withStats := p.boolean("stats")
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	if kind == "pincode" {
		pins, err := s.store.PinCodes(r.Context())
		if err != nil {
			s.handleErr(w, r, err)
			return
		}
		if withStats {
			pins = slices.Clone(pins)
			for i := range pins {
				full, err := s.store.PinCode(r.Context(), pins[i].ID)
				if err != nil {
					s.handleErr(w, r, err)
					return
				}
				pins[i].Stats = full.Stats
			}
		}
		s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(pins), "items": pins})
		return
	}
	items := s.store.Areas()
	if kind != "" {
		kinds := []string{"pincode"}
		filtered := []store.Area{}
		for _, a := range items {
			if !contains(kinds, a.Kind) {
				kinds = append(kinds, a.Kind)
			}
			if a.Kind == kind {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) == 0 {
			s.badRequest(w, r, "kind must be one of "+strings.Join(kinds, ", "))
			return
		}
		items = filtered
	}
	if withStats {
		// Statistics for every listed area at once, e.g. to compare the wards.
		items = slices.Clone(items)
		for i := range items {
			stats, err := s.store.Stats(r.Context(), items[i].ID)
			if err != nil {
				s.handleErr(w, r, err)
				return
			}
			items[i].Stats = stats
		}
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s *Server) handleArea(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	area := s.store.Area
	if store.IsPinCode(id) {
		area = s.store.PinCode
	}
	a, err := area(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	if newParams(r.URL).str("format") == "geojson" {
		props := *a
		props.Geometry = nil
		s.writeGeoJSON(w, r, feature{Type: "Feature", ID: a.ID, Geometry: a.Geometry, Properties: props})
		return
	}
	s.writeJSON(w, r, http.StatusOK, a)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	area := newParams(r.URL).str("area")
	if area != "" && !s.validArea(r, area) {
		s.badRequest(w, r, "unknown area "+area)
		return
	}
	var stats map[string]float64
	var err error
	if store.IsPinCode(area) {
		var pin *store.Area
		if pin, err = s.store.PinCode(r.Context(), area); err == nil {
			stats = pin.Stats
		}
	} else {
		stats, err = s.store.Stats(r.Context(), area)
	}
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	places := map[string]float64{}
	totals := map[string]float64{}
	for k, v := range stats {
		if c, ok := strings.CutPrefix(k, "places:"); ok {
			places[c] = v
		} else {
			totals[k] = v
		}
	}
	if area == "" {
		area = "*"
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"area": area, "totals": totals, "places_by_category": places})
}

// ---------------------------------------------------------------- search & reverse

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	q := p.str("q")
	limit := p.intRange("limit", 10, 1, 50)
	near := p.point(false)
	kinds := p.list("kind")
	for _, k := range kinds {
		if !contains(store.SearchKinds, k) {
			p.fail("kind must be any of %s", strings.Join(store.SearchKinds, ", "))
		}
	}
	if q == "" {
		p.fail("q is required")
	}
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	results, err := s.store.Search(r.Context(), q, near, kinds, limit)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"query": q, "count": len(results), "items": results})
}

func (s *Server) handleReverse(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	pt := p.point(true)
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	res, err := s.store.Reverse(r.Context(), *pt)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, res)
}

// ---------------------------------------------------------------- places

func (s *Server) placeQuery(p *params) store.PlaceQuery {
	q := store.PlaceQuery{
		BBox:       p.bbox(),
		Categories: p.list("category"),
		Groups:     p.list("group"),
		Area:       p.str("area"),
		Text:       p.str("q"),
		NamedOnly:  p.boolean("named"),
	}
	q.Limit, q.Offset = p.limitOffset(50, 500)
	for _, c := range q.Categories {
		if _, ok := s.store.Category(c); !ok {
			p.fail("unknown category %q (see /api/v1/categories)", c)
		}
	}
	for _, g := range q.Groups {
		if !s.store.GroupExists(g) {
			p.fail("unknown group %q (see /api/v1/categories)", g)
		}
	}
	if q.Area != "" && !s.validArea(p.r, q.Area) {
		p.fail("unknown area %q (see /api/v1/areas)", q.Area)
	}
	return q
}

// validArea accepts configured areas and, for place queries and statistics, PIN codes.
func (s *Server) validArea(r *http.Request, id string) bool {
	return s.store.AreaExists(id) || s.store.PinCodeExists(r.Context(), id)
}

func (s *Server) handlePlaces(w http.ResponseWriter, r *http.Request) {
	p := requestParams(r)
	q := s.placeQuery(p)
	if near := p.point(false); near != nil {
		q.Near = near
		q.RadiusM = p.float("radius", 1000, 1, 10000)
	}
	format := p.oneOf("format", "json", "json", "geojson")
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	s.servePlaces(w, r, q, format)
}

func (s *Server) handleNearby(w http.ResponseWriter, r *http.Request) {
	p := requestParams(r)
	q := s.placeQuery(p)
	q.Near = p.point(true)
	q.RadiusM = p.float("radius", 500, 1, 10000)
	format := p.oneOf("format", "json", "json", "geojson")
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	s.servePlaces(w, r, q, format)
}

func (s *Server) servePlaces(w http.ResponseWriter, r *http.Request, q store.PlaceQuery, format string) {
	places, err := s.store.Places(r.Context(), q)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	if format == "geojson" {
		s.writeGeoJSON(w, r, placesToGeoJSON(places))
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(places), "limit": q.Limit, "offset": q.Offset, "items": places})
}

func (s *Server) handlePlace(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		s.badRequest(w, r, "id must be a feature id (e.g. 2470123452) or OSM shorthand (n123, w123, r123)")
		return
	}
	d, err := s.store.Place(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	if newParams(r.URL).str("format") == "geojson" {
		s.writeGeoJSON(w, r, feature{Type: "Feature", ID: d.ID, Geometry: pointGeometry(d.Lon, d.Lat), Properties: d})
		return
	}
	s.writeJSON(w, r, http.StatusOK, d)
}

func (s *Server) handleStations(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	mode := p.oneOf("mode", "", "metro", "monorail", "railway")
	q := store.PlaceQuery{BBox: p.bbox(), Area: p.str("area"), Categories: store.StationCategories, Limit: 1000}
	if mode != "" {
		q.Categories = []string{mode + "_station"}
	}
	if q.Area != "" && !s.validArea(r, q.Area) {
		p.fail("unknown area %q", q.Area)
	}
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	places, err := s.store.Places(r.Context(), q)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	items := make([]*store.PlaceDetail, 0, len(places))
	for _, pl := range places {
		d, err := s.store.Place(r.Context(), pl.ID)
		if err != nil {
			s.handleErr(w, r, err)
			return
		}
		d.Tags = nil
		items = append(items, d)
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

// ---------------------------------------------------------------- streets & roads

func (s *Server) handleStreets(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	q := store.StreetQuery{Text: p.str("q"), BBox: p.bbox(), Area: p.str("area"), Kinds: p.list("kind")}
	q.Limit, q.Offset = p.limitOffset(50, 500)
	if q.Area != "" && !s.store.AreaExists(q.Area) {
		p.fail("unknown area %q", q.Area)
	}
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	streets, err := s.store.Streets(r.Context(), q)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(streets), "items": streets})
}

func (s *Server) handleStreet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.badRequest(w, r, "id must be an integer")
		return
	}
	st, err := s.store.Street(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	if newParams(r.URL).str("format") == "geojson" {
		props := *st
		props.Geometry = nil
		s.writeGeoJSON(w, r, feature{Type: "Feature", ID: st.ID, Geometry: st.Geometry, Properties: props})
		return
	}
	s.writeJSON(w, r, http.StatusOK, st)
}

func (s *Server) handleRoad(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		s.badRequest(w, r, "invalid id")
		return
	}
	road, err := s.store.Road(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, road)
}

// ---------------------------------------------------------------- bridges

var bridgeKinds = []string{"road", "rail", "metro", "foot", "construction", "aqueduct", "outline"}

func (s *Server) bridgeQuery(p *params) store.BridgeQuery {
	q := store.BridgeQuery{
		BBox:       p.bbox(),
		Kinds:      p.list("kind"),
		MinLengthM: p.float("min_length", 0, 0, 1e6),
		Area:       p.str("area"),
		Text:       p.str("q"),
		Sort:       p.oneOf("sort", "length", "length", "name"),
	}
	q.Limit, q.Offset = p.limitOffset(50, 500)
	for _, k := range q.Kinds {
		if !contains(bridgeKinds, k) {
			p.fail("kind must be any of %s", strings.Join(bridgeKinds, ", "))
		}
	}
	if q.Area != "" && !s.store.AreaExists(q.Area) {
		p.fail("unknown area %q", q.Area)
	}
	return q
}

func (s *Server) handleBridges(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	q := s.bridgeQuery(p)
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	items, err := s.store.Bridges(r.Context(), q)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s *Server) handleBridge(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		s.badRequest(w, r, "invalid id")
		return
	}
	b, err := s.store.Bridge(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, b)
}

func (s *Server) handleNamedBridges(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	q := s.bridgeQuery(p)
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	items, err := s.store.NamedBridges(r.Context(), q)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s *Server) handleNamedBridge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.badRequest(w, r, "id must be an integer")
		return
	}
	b, err := s.store.NamedBridge(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	if newParams(r.URL).str("format") == "geojson" {
		props := *b
		props.Geometry, props.Ways = nil, nil
		s.writeGeoJSON(w, r, feature{Type: "Feature", ID: b.ID, Geometry: b.Geometry, Properties: props})
		return
	}
	s.writeJSON(w, r, http.StatusOK, b)
}

// ---------------------------------------------------------------- buildings

func (s *Server) handleBuildings(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	q := store.BuildingQuery{
		BBox:       p.bbox(),
		Area:       p.str("area"),
		MinLevels:  p.intRange("min_levels", 0, 0, 200),
		MinHeightM: p.float("min_height", 0, 0, 1000),
		NamedOnly:  p.boolean("named"),
		Text:       p.str("q"),
		Sort:       p.oneOf("sort", "height", "height", "levels", "footprint", "name"),
	}
	q.Limit, q.Offset = p.limitOffset(50, 500)
	if q.Area != "" && !s.store.AreaExists(q.Area) {
		p.fail("unknown area %q", q.Area)
	}
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	items, err := s.store.Buildings(r.Context(), q)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s *Server) handleBuilding(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		s.badRequest(w, r, "invalid id")
		return
	}
	b, err := s.store.Building(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, b)
}

// ---------------------------------------------------------------- transit

func (s *Server) handleTransitLines(w http.ResponseWriter, r *http.Request) {
	p := newParams(r.URL)
	modes := p.list("mode")
	for _, m := range modes {
		if !contains([]string{"subway", "light_rail", "monorail", "train", "tram"}, m) {
			p.fail("mode must be any of subway, light_rail, monorail, train, tram")
		}
	}
	if p.err != nil {
		s.badRequest(w, r, p.err.Error())
		return
	}
	lines, err := s.store.TransitLines(r.Context(), modes, p.str("network"))
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(lines), "items": lines})
}

func (s *Server) handleTransitRoute(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.badRequest(w, r, "id must be an OSM relation id")
		return
	}
	route, err := s.store.TransitRoute(r.Context(), id)
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	if newParams(r.URL).str("format") == "geojson" {
		var stops []struct {
			ID   int64   `json:"id"`
			Name string  `json:"name"`
			Lon  float64 `json:"lon"`
			Lat  float64 `json:"lat"`
		}
		_ = json.Unmarshal(route.Stops, &stops)
		props := *route
		props.Geometry, props.Stops = nil, nil
		fc := featureCollection{Type: "FeatureCollection", Features: []feature{{Type: "Feature", ID: route.ID, Geometry: route.Geometry, Properties: props}}}
		for i, st := range stops {
			fc.Features = append(fc.Features, feature{Type: "Feature", ID: st.ID, Geometry: pointGeometry(st.Lon, st.Lat),
				Properties: map[string]any{"name": st.Name, "sequence": i + 1, "route_id": route.ID, "kind": "stop"}})
		}
		s.writeGeoJSON(w, r, fc)
		return
	}
	s.writeJSON(w, r, http.StatusOK, route)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
