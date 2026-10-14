package httpapi

import (
	"crypto/sha256"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// requestCost is how many rate-limit tokens an API request spends: searches over the road graph (routes, and the
// walks of a journey) cost more than lookups in the database.
func requestCost(path string) float64 {
	switch path {
	case "/api/v1/route", "/api/v1/journey":
		return 5
	}
	return 1
}

// guard checks API keys and applies the per-client rate limit to /api/ requests. A valid key (X-API-Key header or
// key query parameter) bypasses the limit; with --require-key requests without one are rejected.
func (s *Server) guard(next http.Handler) http.Handler {
	if s.limiter == nil && len(s.keys) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("key")
		}
		switch {
		case key != "" && s.validKey(key):
			next.ServeHTTP(w, r)
			return
		case key != "":
			s.fail(w, r, http.StatusUnauthorized, "invalid_key", "unknown API key")
			return
		case s.cfg.RequireKey:
			s.fail(w, r, http.StatusUnauthorized, "missing_key", "an API key is required; send it in the X-API-Key header")
			return
		case s.limiter == nil:
			next.ServeHTTP(w, r)
			return
		}
		d := s.limiter.Allow(s.clientKey(r), requestCost(r.URL.Path))
		h := w.Header()
		h.Set("X-RateLimit-Limit", strconv.Itoa(d.Limit))
		h.Set("X-RateLimit-Remaining", strconv.Itoa(d.Remaining))
		if !d.Allowed {
			wait := int(math.Ceil(d.RetryAfter.Seconds()))
			h.Set("Retry-After", strconv.Itoa(wait))
			s.fail(w, r, http.StatusTooManyRequests, "rate_limited",
				"too many requests; retry in "+strconv.Itoa(wait)+" s")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validKey looks a key up by its SHA-256 digest, so the comparison does not leak key prefixes through timing.
func (s *Server) validKey(key string) bool {
	_, ok := s.keys[sha256.Sum256([]byte(key))]
	return ok
}

// clientKey identifies the client a request counts against: its IP address (IPv6 clients by their /64 network, as a
// single host usually owns the whole prefix). Behind a trusted proxy the address the proxy saw is used.
func (s *Server) clientKey(r *http.Request) string {
	raw := ""
	if s.cfg.TrustProxy {
		if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
			raw = v
		} else if v := r.Header.Get("X-Forwarded-For"); v != "" {
			// The right-most entry was added by our proxy; entries to its left are client-controlled.
			parts := strings.Split(v, ",")
			raw = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	if raw == "" {
		raw = r.RemoteAddr
		if host, _, err := net.SplitHostPort(raw); err == nil {
			raw = host
		}
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return raw
	}
	addr = addr.Unmap()
	if addr.Is6() {
		prefix, _ := addr.Prefix(64)
		return prefix.String()
	}
	return addr.String()
}
