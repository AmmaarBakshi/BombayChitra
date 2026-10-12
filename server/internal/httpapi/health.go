package httpapi

import (
	"context"
	"math"
	"net/http"
	"time"
)

type check struct {
	Status    string   `json:"status"`
	LatencyMS *float64 `json:"latency_ms,omitempty"` // measured checks only; may be 0 for a sub-microsecond query
	Error     string   `json:"error,omitempty"`
	Note      string   `json:"note,omitempty"`
	Detail    any      `json:"detail,omitempty"` // always an object
}

// handleHealthDetail reports the state of every component: database, tiles and routing graph, plus dataset age. It
// answers 503 when the database cannot be read, so it doubles as a readiness probe.
func (s *Server) handleHealthDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusOK
	checks := map[string]check{}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	start := time.Now()
	if err := s.store.Ping(ctx); err != nil {
		checks["database"] = check{Status: "error", Error: err.Error()}
		status = http.StatusServiceUnavailable
	} else {
		latency := ms(time.Since(start))
		checks["database"] = check{Status: "ok", LatencyMS: &latency}
	}

	if s.tiles != nil {
		h := s.tiles.Header()
		checks["tiles"] = check{Status: "ok", Detail: map[string]any{"tiles": h.AddressedTiles, "minzoom": h.MinZoom, "maxzoom": h.MaxZoom}}
	} else {
		checks["tiles"] = check{Status: "missing"}
	}

	switch g, ok, err := s.routes.Peek(); {
	case !ok:
		checks["routing"] = check{Status: "not_loaded", Note: "loaded on the first route request"}
	case err != nil:
		checks["routing"] = check{Status: "unavailable", Error: err.Error()}
	default:
		checks["routing"] = check{Status: "ok", Detail: g.Stats()}
	}

	m := s.store.Meta()
	dataset := map[string]any{"built_at": m["built_at"], "source_timestamp": m["source_timestamp"], "schema_version": m["schema_version"]}
	stamp := m["source_timestamp"]
	if stamp == "" {
		stamp = m["built_at"]
	}
	if t, err := time.Parse(time.RFC3339, stamp); err == nil {
		dataset["age_days"] = math.Floor(time.Since(t).Hours()/24*10) / 10
	}
	overall := "ok"
	if status != http.StatusOK {
		overall = "error"
	}
	s.writeJSON(w, r, status, map[string]any{
		"status":   overall,
		"version":  Version,
		"uptime_s": math.Round(time.Since(s.started).Seconds()),
		"dataset":  dataset,
		"checks":   checks,
	})
}

func ms(d time.Duration) float64 { return math.Round(float64(d.Nanoseconds())/1e3) / 1e3 }
