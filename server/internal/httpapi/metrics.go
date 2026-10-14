package httpapi

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/metrics"
)

// serverMetrics are the Prometheus metrics exposed at /metrics.
type serverMetrics struct {
	registry    *metrics.Registry
	requests    *metrics.CounterVec
	duration    *metrics.HistogramVec
	bytes       *metrics.CounterVec
	rateLimited *metrics.CounterVec
	inFlight    atomic.Int64

	routeSearches *metrics.CounterVec
	routeDuration *metrics.HistogramVec
	routeSettled  *metrics.HistogramVec
}

func (s *Server) newMetrics() *serverMetrics {
	r := metrics.NewRegistry()
	m := &serverMetrics{
		registry:    r,
		requests:    r.NewCounterVec("bombaychitra_http_requests_total", "HTTP requests by route pattern, method and status code.", "route", "method", "code"),
		duration:    r.NewHistogramVec("bombaychitra_http_request_duration_seconds", "Time to serve HTTP requests by route pattern.", metrics.DefaultBuckets, "route"),
		bytes:       r.NewCounterVec("bombaychitra_http_response_bytes_total", "Response body bytes written by route pattern (after compression).", "route"),
		rateLimited: r.NewCounterVec("bombaychitra_http_rate_limited_total", "API requests rejected by the rate limiter.", "route"),

		routeSearches: r.NewCounterVec("bombaychitra_route_searches_total", "Route searches by travel mode (transit for journeys) and outcome (ok, no_road, no_station, no_route, unavailable, error).", "mode", "outcome"),
		routeDuration: r.NewHistogramVec("bombaychitra_route_search_duration_seconds", "Time to compute a route or plan a journey, including waiting for a search slot.", metrics.DefaultBuckets, "mode"),
		routeSettled:  r.NewHistogramVec("bombaychitra_route_settled_vertices", "Graph vertices settled per route search (search effort).", []float64{100, 1000, 5000, 20000, 50000, 100000, 200000}, "mode"),
	}
	r.NewGaugeFunc("bombaychitra_routing_graph_vertices", "Vertices in the loaded routing graph (absent until the first route request).", nil, func() []metrics.Sample {
		if s.routes == nil {
			return nil
		}
		if g, ok, err := s.routes.Peek(); ok && err == nil {
			return []metrics.Sample{{Value: float64(g.Stats().Vertices)}}
		}
		return nil
	})
	r.NewGaugeFunc("bombaychitra_http_in_flight_requests", "Requests being served.", nil, func() []metrics.Sample {
		return []metrics.Sample{{Value: float64(m.inFlight.Load())}}
	})
	r.NewGaugeFunc("bombaychitra_build_info", "Server version and dataset build (always 1).", []string{"version", "dataset", "schema"}, func() []metrics.Sample {
		if s.store == nil {
			return []metrics.Sample{{Labels: []string{Version, "", ""}, Value: 1}}
		}
		return []metrics.Sample{{Labels: []string{Version, s.store.Version(), s.store.Meta()["schema_version"]}, Value: 1}}
	})
	r.NewGaugeFunc("bombaychitra_uptime_seconds", "Seconds since the server started.", nil, func() []metrics.Sample {
		return []metrics.Sample{{Value: time.Since(s.started).Seconds()}}
	})
	r.NewGaugeFunc("bombaychitra_dataset_age_seconds", "Age of the OpenStreetMap data being served.", nil, func() []metrics.Sample {
		if s.store == nil {
			return nil
		}
		stamp := s.store.Meta()["source_timestamp"]
		if stamp == "" {
			stamp = s.store.Meta()["built_at"]
		}
		t, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			return nil
		}
		return []metrics.Sample{{Value: time.Since(t).Seconds()}}
	})
	r.NewGaugeFunc("bombaychitra_rate_limit_clients", "Clients with a partly used rate-limit bucket.", nil, func() []metrics.Sample {
		if s.limiter == nil {
			return nil
		}
		return []metrics.Sample{{Value: float64(s.limiter.Clients())}}
	})
	r.NewGaugeFunc("go_goroutines", "Number of goroutines.", nil, func() []metrics.Sample {
		return []metrics.Sample{{Value: float64(runtime.NumGoroutine())}}
	})
	r.NewGaugeFunc("go_memstats_heap_alloc_bytes", "Bytes of allocated heap objects.", nil, func() []metrics.Sample {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return []metrics.Sample{{Value: float64(ms.HeapAlloc)}}
	})
	return m
}

// observe records a finished request. The route label is the ServeMux pattern that matched (bounded cardinality),
// not the raw path.
func (m *serverMetrics) observe(r *http.Request, status, bytes int, elapsed time.Duration) {
	route := r.Pattern
	if _, p, ok := strings.Cut(route, " "); ok {
		route = p
	}
	if route == "" {
		route = "unmatched"
	}
	m.requests.Inc(route, r.Method, strconv.Itoa(status))
	m.duration.Observe(elapsed.Seconds(), route)
	m.bytes.Add(float64(bytes), route)
	if status == http.StatusTooManyRequests {
		m.rateLimited.Inc(route)
	}
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", metrics.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	s.metrics.registry.WriteTo(w)
}

// MetricsHandler serves /metrics on its own (for --metrics-addr).
func (s *Server) MetricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	return mux
}
