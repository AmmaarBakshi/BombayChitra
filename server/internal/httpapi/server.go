// Package httpapi exposes the dataset over HTTP: the REST API (/api/v1), vector tiles (/tiles), map styles (/styles),
// raw downloads (/data) and the embedded web app and docs.
package httpapi

import (
	"crypto/sha256"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/config"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/pmtiles"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/ratelimit"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/routing"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

// Version is the server version, overridden at build time with -ldflags "-X ...httpapi.Version=v1.2.3".
var Version = "dev"

// Server bundles the data sources and the static web assets.
type Server struct {
	cfg     config.Config
	store   *store.Store
	tiles   *pmtiles.Reader
	web     fs.FS
	static  *staticHandler
	routes  *routing.Service
	limiter *ratelimit.Limiter // nil when rate limiting is off
	keys    map[[32]byte]bool  // SHA-256 digests of the API keys
	started time.Time
	metrics *serverMetrics
	styles  sync.Map // base URL + name → rendered style bytes
}

// New creates a server. web is the built web app (embedded or on disk) and may be nil.
func New(cfg config.Config, st *store.Store, tiles *pmtiles.Reader, web fs.FS) *Server {
	s := &Server{cfg: cfg, store: st, tiles: tiles, web: web, started: time.Now()}
	if st != nil {
		s.routes = routing.NewService(st.DB(), 4)
	}
	if web != nil {
		s.static = newStaticHandler(web)
	}
	if cfg.RateLimit > 0 {
		s.limiter = ratelimit.New(cfg.RateLimit, cfg.RateBurst)
	}
	s.metrics = s.newMetrics()
	if len(cfg.APIKeys) > 0 {
		s.keys = make(map[[32]byte]bool, len(cfg.APIKeys))
		for _, k := range cfg.APIKeys {
			s.keys[sha256.Sum256([]byte(k))] = true
		}
	}
	return s
}

// Handler returns the root handler with middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /api/v1", s.handleIndex)
	mux.HandleFunc("GET /api/v1/{$}", s.handleIndex)
	mux.HandleFunc("GET /api/v1/health", s.handleHealthDetail)
	mux.HandleFunc("GET /api/v1/meta", s.handleMeta)
	mux.HandleFunc("GET /api/v1/categories", s.handleCategories)
	mux.HandleFunc("GET /api/v1/areas", s.handleAreas)
	mux.HandleFunc("GET /api/v1/areas/{id}", s.handleArea)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	mux.HandleFunc("GET /api/v1/reverse", s.handleReverse)
	mux.HandleFunc("GET /api/v1/places", s.handlePlaces)
	mux.HandleFunc("GET /api/v1/places/{id}", s.handlePlace)
	mux.HandleFunc("GET /api/v1/nearby", s.handleNearby)
	mux.HandleFunc("GET /api/v1/streets", s.handleStreets)
	mux.HandleFunc("GET /api/v1/streets/{id}", s.handleStreet)
	mux.HandleFunc("GET /api/v1/roads/{id}", s.handleRoad)
	mux.HandleFunc("GET /api/v1/bridges", s.handleBridges)
	mux.HandleFunc("GET /api/v1/bridges/named", s.handleNamedBridges)
	mux.HandleFunc("GET /api/v1/bridges/named/{id}", s.handleNamedBridge)
	mux.HandleFunc("GET /api/v1/bridges/{id}", s.handleBridge)
	mux.HandleFunc("GET /api/v1/buildings", s.handleBuildings)
	mux.HandleFunc("GET /api/v1/buildings/{id}", s.handleBuilding)
	mux.HandleFunc("GET /api/v1/transit/lines", s.handleTransitLines)
	mux.HandleFunc("GET /api/v1/transit/routes/{id}", s.handleTransitRoute)
	mux.HandleFunc("GET /api/v1/transit/stations", s.handleStations)
	mux.HandleFunc("GET /api/v1/route", s.handleRoute)
	mux.HandleFunc("GET /api/v1/heritage", s.handleHeritage)
	mux.HandleFunc("GET /api/v1/hotspots", s.handleHotspots)
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) { s.notFound(w, r) })

	mux.HandleFunc("GET /tiles/bombaychitra.json", s.handleTileJSON)
	mux.HandleFunc("GET /tiles/{z}/{x}/{y}", s.handleTile)
	mux.HandleFunc("GET /styles/{name}", s.handleStyle)
	if s.cfg.Downloads {
		mux.HandleFunc("GET /data/{file}", s.handleDownload)
	}
	if s.cfg.Metrics && s.cfg.MetricsAddr == "" {
		mux.HandleFunc("GET /metrics", s.handleMetrics)
	}
	if s.static != nil {
		mux.Handle("GET /", s.static)
	}
	return s.recover(s.logRequests(s.headers(s.guard(mux))))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	s.writeJSON(w, r, http.StatusOK, map[string]any{"status": "ok", "version": Version, "dataset": s.store.Version()})
}

// baseURL is the absolute origin clients should use for tile and style URLs.
func (s *Server) baseURL(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL
	}
	scheme, host := "http", r.Host
	if r.TLS != nil {
		scheme = "https"
	}
	if s.cfg.TrustProxy {
		if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
			scheme = strings.TrimSpace(strings.Split(v, ",")[0])
		}
		if v := r.Header.Get("X-Forwarded-Host"); v != "" {
			host = strings.TrimSpace(strings.Split(v, ",")[0])
		}
	}
	return scheme + "://" + host
}

// ---------------------------------------------------------------- middleware

var corsPrefixes = []string{"/api/", "/tiles/", "/styles/", "/data/", "/fonts/", "/openapi.json", "/healthz"}

func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if s.cfg.CORSOrigin != "" {
			for _, p := range corsPrefixes {
				if strings.HasPrefix(r.URL.Path, p) {
					h.Set("Access-Control-Allow-Origin", s.cfg.CORSOrigin)
					h.Set("Access-Control-Expose-Headers", "ETag, Content-Length, Content-Range, Retry-After, X-RateLimit-Limit, X-RateLimit-Remaining")
					if r.Method == http.MethodOptions {
						h.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
						h.Set("Access-Control-Allow-Headers", "Range, If-None-Match, If-Match, Content-Type, X-API-Key")
						h.Set("Access-Control-Max-Age", "86400")
						w.WriteHeader(http.StatusNoContent)
						return
					}
					break
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		s.metrics.inFlight.Add(1)
		next.ServeHTTP(rec, r)
		s.metrics.inFlight.Add(-1)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		s.metrics.observe(r, rec.status, rec.bytes, time.Since(start))
		level := slog.LevelInfo
		if strings.HasPrefix(r.URL.Path, "/tiles/") || strings.HasPrefix(r.URL.Path, "/assets/") ||
			strings.HasPrefix(r.URL.Path, "/fonts/") {
			level = slog.LevelDebug
		}
		if rec.status >= 500 {
			level = slog.LevelError
		}
		slog.Log(r.Context(), level, "http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"bytes", rec.bytes, "ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.Error("panic", "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
