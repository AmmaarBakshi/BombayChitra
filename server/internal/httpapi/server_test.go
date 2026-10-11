package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/config"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/pmtiles"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

// TestRoutesRegister catches ServeMux pattern conflicts, which panic at startup.
func TestRoutesRegister(t *testing.T) {
	web := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}
	s := New(config.Config{Downloads: true, CORSOrigin: "*"}, nil, nil, web)
	if s.Handler() == nil {
		t.Fatal("nil handler")
	}
}

func TestStaticFallbackAndPrecompressed(t *testing.T) {
	web := fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>app</title>")},
		"assets/app-123.js":    {Data: []byte(strings.Repeat("console.log(1);", 200))},
		"assets/app-123.js.br": {Data: []byte("BR")},
		"docs/index.html":      {Data: []byte("<h1>docs</h1>")},
	}
	h := newStaticHandler(web)
	cases := []struct {
		path, encoding, wantBody, wantEncoding string
		wantStatus                             int
	}{
		{"/", "", "<!doctype html><title>app</title>", "", 200},
		{"/some/client/route", "", "<!doctype html><title>app</title>", "", 200},
		{"/docs", "", "<h1>docs</h1>", "", 200},
		{"/assets/app-123.js", "br, gzip", "BR", "br", 200},
		{"/missing.js", "", "", "", 404},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		if c.encoding != "" {
			req.Header.Set("Accept-Encoding", c.encoding)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.wantStatus {
			t.Errorf("%s: status %d, want %d", c.path, rec.Code, c.wantStatus)
			continue
		}
		if c.wantBody != "" && rec.Body.String() != c.wantBody {
			t.Errorf("%s: body %q, want %q", c.path, rec.Body.String(), c.wantBody)
		}
		if got := rec.Header().Get("Content-Encoding"); got != c.wantEncoding {
			t.Errorf("%s: encoding %q, want %q", c.path, got, c.wantEncoding)
		}
	}
}

func TestParseID(t *testing.T) {
	cases := map[string]int64{"311442962": 311442962, "w31144296": 311442962, "n5": 51, "r7": 73}
	for in, want := range cases {
		if got, ok := parseID(in); !ok || got != want {
			t.Errorf("parseID(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "x1", "w", "-5", "n-1"} {
		if _, ok := parseID(bad); ok {
			t.Errorf("parseID(%q) should fail", bad)
		}
	}
}

// testServer opens the locally built dataset, skipping when it has not been built.
func testServer(t *testing.T) http.Handler {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "data", "build")
	db, tiles := filepath.Join(dir, "bombaychitra.db"), filepath.Join(dir, "bombaychitra.pmtiles")
	for _, f := range []string{db, tiles} {
		if _, err := os.Stat(f); err != nil {
			t.Skip("dataset not built")
		}
	}
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	tr, err := pmtiles.Open(tiles)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tr.Close() })
	cfg := config.Config{Database: db, Tiles: tiles, Downloads: true, CORSOrigin: "*"}
	return New(cfg, st, tr, fstest.MapFS{"index.html": {Data: []byte("app")}}).Handler()
}

func get(t *testing.T, h http.Handler, url string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body map[string]any
	if strings.Contains(rec.Header().Get("Content-Type"), "json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: invalid JSON: %v", url, err)
		}
	}
	return rec, body
}

func TestAPI(t *testing.T) {
	h := testServer(t)
	ok := []string{
		"/healthz", "/api/v1", "/api/v1/meta", "/api/v1/categories", "/api/v1/areas", "/api/v1/areas/makabo",
		"/api/v1/stats?area=thane", "/api/v1/search?q=andheri", "/api/v1/reverse?lat=19.0545&lon=72.8405",
		"/api/v1/places?category=hospital&limit=5", "/api/v1/places?bbox=72.82,19.05,72.85,19.07&format=geojson",
		"/api/v1/nearby?lat=19.0178&lon=72.8478&radius=800&category=atm", "/api/v1/streets?q=linking",
		"/api/v1/bridges/named?kind=road&limit=3", "/api/v1/buildings?limit=3", "/api/v1/transit/lines",
		"/api/v1/transit/stations?mode=metro", "/tiles/bombaychitra.json",
	}
	for _, url := range ok {
		if rec, _ := get(t, h, url); rec.Code != http.StatusOK {
			t.Errorf("GET %s: %d %s", url, rec.Code, rec.Body.String())
		}
	}
	bad := []string{
		"/api/v1/places?category=nope", "/api/v1/places?bbox=1,2,3", "/api/v1/places?limit=100000",
		"/api/v1/search", "/api/v1/reverse?lat=999&lon=0", "/api/v1/nearby?lat=19", "/api/v1/places/abc",
	}
	for _, url := range bad {
		rec, body := get(t, h, url)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: %d, want 400", url, rec.Code)
		}
		if _, ok := body["error"]; !ok {
			t.Errorf("GET %s: missing error envelope", url)
		}
	}
	if rec, _ := get(t, h, "/api/v1/places/1"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown place: %d, want 404", rec.Code)
	}
	if rec, _ := get(t, h, "/api/v1/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown endpoint: %d, want 404", rec.Code)
	}
}

func TestSearchFindsStationsAndReverseFindsRoad(t *testing.T) {
	h := testServer(t)
	_, body := get(t, h, "/api/v1/search?q=andheri&limit=5")
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatal("no search results for andheri")
	}
	_, rev := get(t, h, "/api/v1/reverse?lat=19.0545&lon=72.8405")
	if rev["road"] == nil || rev["label"] == "" {
		t.Errorf("reverse geocode returned no road: %v", rev)
	}
}

func TestTilesAndConditionalRequests(t *testing.T) {
	h := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/tiles/14/11505/7300.mvt", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("tile: %d %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
	etag := rec.Header().Get("ETag")
	req = httptest.NewRequest(http.MethodGet, "/tiles/14/11505/7300.mvt", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("conditional tile: %d, want 304", rec.Code)
	}
	plain := httptest.NewRecorder()
	h.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/tiles/14/11505/7300.mvt", nil))
	if plain.Header().Get("Content-Encoding") != "" || plain.Body.Len() <= rec.Body.Len() {
		t.Errorf("tile without gzip support should be decompressed")
	}
	if rec, _ := get(t, h, "/tiles/15/0/0.mvt"); rec.Code != http.StatusNoContent {
		t.Errorf("empty tile: %d, want 204", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := New(config.Config{CORSOrigin: "*"}, nil, nil, nil).Handler()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/meta", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("preflight: %d %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
