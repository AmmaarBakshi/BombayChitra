package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/config"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func request(h http.Handler, path, remote string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remote
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimit(t *testing.T) {
	s := New(config.Config{RateLimit: 60, RateBurst: 3}, nil, nil, nil)
	h := s.guard(okHandler())
	for i := 0; i < 3; i++ {
		if rec := request(h, "/api/v1/meta", "203.0.113.5:4000", nil); rec.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i, rec.Code)
		}
	}
	rec := request(h, "/api/v1/meta", "203.0.113.5:4001", nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("over the limit: %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("X-RateLimit-Limit") != "3" || rec.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("headers: %v", rec.Header())
	}
	// Other clients, tiles and pages are not affected.
	if rec := request(h, "/api/v1/meta", "203.0.113.6:4000", nil); rec.Code != http.StatusOK {
		t.Errorf("other client: %d", rec.Code)
	}
	if rec := request(h, "/tiles/14/11505/7300.mvt", "203.0.113.5:4000", nil); rec.Code != http.StatusOK {
		t.Errorf("tiles are not limited: %d", rec.Code)
	}
}

func TestRouteCostsMore(t *testing.T) {
	s := New(config.Config{RateLimit: 60, RateBurst: 6}, nil, nil, nil)
	h := s.guard(okHandler())
	if rec := request(h, "/api/v1/route", "198.51.100.1:1", nil); rec.Code != http.StatusOK {
		t.Fatalf("first route: %d", rec.Code)
	}
	if rec := request(h, "/api/v1/route", "198.51.100.1:1", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second route within a second: %d, want 429", rec.Code)
	}
	if rec := request(h, "/api/v1/meta", "198.51.100.1:1", nil); rec.Code != http.StatusOK {
		t.Fatalf("a cheap request still fits: %d", rec.Code)
	}
}

func TestAPIKeys(t *testing.T) {
	const key = "0123456789abcdef-test"
	s := New(config.Config{RateLimit: 60, RateBurst: 1, APIKeys: []string{key}}, nil, nil, nil)
	h := s.guard(okHandler())
	for i := 0; i < 5; i++ {
		if rec := request(h, "/api/v1/meta", "192.0.2.1:1", map[string]string{"X-API-Key": key}); rec.Code != http.StatusOK {
			t.Fatalf("keyed request %d: %d", i, rec.Code)
		}
	}
	if rec := request(h, "/api/v1/meta?key="+key, "192.0.2.1:1", nil); rec.Code != http.StatusOK {
		t.Errorf("key in query: %d", rec.Code)
	}
	if rec := request(h, "/api/v1/meta", "192.0.2.1:1", map[string]string{"X-API-Key": "wrong-key-wrong-key"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong key: %d, want 401", rec.Code)
	}

	s = New(config.Config{APIKeys: []string{key}, RequireKey: true}, nil, nil, nil)
	h = s.guard(okHandler())
	if rec := request(h, "/api/v1/meta", "192.0.2.1:1", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("missing key with require-key: %d, want 401", rec.Code)
	}
	if rec := request(h, "/styles/light.json", "192.0.2.1:1", nil); rec.Code != http.StatusOK {
		t.Errorf("styles do not need a key: %d", rec.Code)
	}
}

func TestClientKey(t *testing.T) {
	cases := []struct {
		trust  bool
		remote string
		header map[string]string
		want   string
	}{
		{false, "203.0.113.9:5555", nil, "203.0.113.9"},
		{false, "203.0.113.9:5555", map[string]string{"X-Forwarded-For": "1.2.3.4"}, "203.0.113.9"},
		{true, "127.0.0.1:1", map[string]string{"X-Forwarded-For": "1.2.3.4, 198.51.100.7"}, "198.51.100.7"},
		{true, "127.0.0.1:1", map[string]string{"X-Real-IP": "198.51.100.8"}, "198.51.100.8"},
		{false, "[2001:db8:1:2:3:4:5:6]:443", nil, "2001:db8:1:2::/64"},
		{false, "[::ffff:203.0.113.4]:80", nil, "203.0.113.4"},
	}
	for _, c := range cases {
		s := New(config.Config{TrustProxy: c.trust}, nil, nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
		req.RemoteAddr = c.remote
		for k, v := range c.header {
			req.Header.Set(k, v)
		}
		if got := s.clientKey(req); got != c.want {
			t.Errorf("clientKey(%s, %v) = %q, want %q", c.remote, c.header, got, c.want)
		}
	}
}
