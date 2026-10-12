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
	"time"

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
	cfg := config.Config{Database: db, Tiles: tiles, Downloads: true, CORSOrigin: "*", Metrics: true}
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

func TestHealth(t *testing.T) {
	h := testServer(t)
	rec, body := get(t, h, "/api/v1/health")
	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("health: %d %v", rec.Code, body)
	}
	checks, _ := body["checks"].(map[string]any)
	for _, name := range []string{"database", "tiles", "routing"} {
		if _, ok := checks[name]; !ok {
			t.Errorf("health is missing the %s check", name)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("health must not be cached: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestHealthFailsWithoutDatabase(t *testing.T) {
	db := filepath.Join("..", "..", "..", "data", "build", "bombaychitra.db")
	if _, err := os.Stat(db); err != nil {
		t.Skip("dataset not built")
	}
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	st.Close() // a database that has gone away, e.g. deleted or unmounted under a running server
	h := New(config.Config{}, st, nil, nil).Handler()
	rec, body := get(t, h, "/api/v1/health")
	if rec.Code != http.StatusServiceUnavailable || body["status"] != "error" {
		t.Fatalf("health without a database: %d %v", rec.Code, body)
	}
	checks, _ := body["checks"].(map[string]any)
	if tiles, _ := checks["tiles"].(map[string]any); tiles["status"] != "missing" {
		t.Errorf("tiles check: %v", checks["tiles"])
	}
}

func TestMetrics(t *testing.T) {
	h := testServer(t)
	get(t, h, "/api/v1/meta")
	get(t, h, "/api/v1/places/1")
	rec, _ := get(t, h, "/metrics")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain; version=0.0.4") {
		t.Fatalf("metrics: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	for _, want := range []string{
		`bombaychitra_http_requests_total{route="/api/v1/meta",method="GET",code="200"} 1`,
		`bombaychitra_http_requests_total{route="/api/v1/places/{id}",method="GET",code="404"} 1`,
		`bombaychitra_http_request_duration_seconds_count{route="/api/v1/meta"} 1`,
		"bombaychitra_build_info{",
		"go_goroutines ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}

func TestSearchFindsStationsAndReverseFindsRoad(t *testing.T) {
	h := testServer(t)
	_, body := get(t, h, "/api/v1/search?q=andheri&limit=5")
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatal("no search results for andheri")
	}
	// Station codes find the station, not businesses that happen to contain the letters.
	for _, code := range []string{"CST", "VT", "CSMT"} {
		_, body := get(t, h, "/api/v1/search?limit=3&q="+code)
		items, _ := body["items"].([]any)
		if len(items) == 0 || items[0].(map[string]any)["category"] != "railway_station" {
			t.Errorf("search %s: first result %v, want the CSMT railway station", code, items)
		}
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

func TestRouteByRoad(t *testing.T) {
	h := testServer(t)
	trips := []struct {
		name, from, to string
	}{
		{"Andheri → BKC", "19.1197,72.8464", "19.0660,72.8680"},
		{"Thane → Vashi across the creek", "19.1860,72.9750", "19.0750,72.9980"},
		{"Colaba → Panvel", "18.9100,72.8200", "18.9900,73.1100"},
		{"Kalamboli → Taloja", "19.0300,73.1000", "19.0650,73.1150"},
		{"Virar → Vasai", "19.4550,72.8050", "19.3650,72.8150"},
	}
	for _, trip := range trips {
		start := time.Now()
		rec, body := get(t, h, "/api/v1/route?from="+trip.from+"&to="+trip.to)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: %d %s", trip.name, rec.Code, rec.Body.String())
			continue
		}
		road, straight := body["distance_m"].(float64), body["straight_m"].(float64)
		coords := body["geometry"].(map[string]any)["coordinates"].([]any)
		t.Logf("%s: %.1f km by road (%.1f km straight), %.0f min, %d points, %v", trip.name, road/1000, straight/1000,
			body["duration_s"].(float64)/60, len(coords), time.Since(start).Round(time.Millisecond))
		if road < straight*0.98 || road > straight*3 || len(coords) < 10 || body["duration_s"].(float64) <= 0 {
			t.Errorf("%s: implausible route %.0f m by road vs %.0f m straight, %d points", trip.name, road, straight, len(coords))
		}
		_, short := get(t, h, "/api/v1/route?optimize=distance&from="+trip.from+"&to="+trip.to)
		if d, _ := short["distance_m"].(float64); d > road+1 {
			t.Errorf("%s: shortest route %.0f m is longer than fastest %.0f m", trip.name, d, road)
		}
	}

	rec, body := get(t, h, "/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680&format=geojson")
	if features, _ := body["features"].([]any); rec.Code != http.StatusOK || len(features) != 3 {
		t.Errorf("geojson route: %d, %d features", rec.Code, len(features))
	}
	if rec, _ := get(t, h, "/api/v1/route?from=18.85,72.71&to=19.0660,72.8680"); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("route from the sea: %d, want 422", rec.Code)
	}
	for _, url := range []string{"/api/v1/route", "/api/v1/route?from=19.1&to=19.2,72.9", "/api/v1/route?from=19.1,72.8&to=19.2,72.9&optimize=scenic"} {
		if rec, _ := get(t, h, url); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: %d, want 400", url, rec.Code)
		}
	}
}

func TestMetaListsRoutingModes(t *testing.T) {
	h := testServer(t)
	_, body := get(t, h, "/api/v1/meta")
	routing, _ := body["routing"].(map[string]any)
	modes, _ := routing["modes"].([]any)
	if routing["available"] != true || len(modes) == 0 || modes[0] != "car" {
		t.Errorf("meta routing: %v", routing)
	}
}

func TestRouteModes(t *testing.T) {
	h := testServer(t)
	const trip = "from=18.9985,72.8580&to=18.9310,73.0300" // Sewri to Ulwe, across Atal Setu by car
	rec, body := get(t, h, "/api/v1/route?mode=foot&"+trip)
	if rec.Code == http.StatusServiceUnavailable {
		t.Skip("dataset built before travel modes")
	}
	results := map[string]map[string]any{}
	for _, q := range []string{"mode=car", "mode=car&avoid=toll", "mode=two_wheeler", "mode=walk"} {
		rec, body = get(t, h, "/api/v1/route?"+q+"&"+trip)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", q, rec.Code, rec.Body.String())
		}
		results[q] = body
	}
	if toll := results["mode=car"]["toll_m"].(float64); toll < 15000 {
		t.Errorf("by car the fastest way is over Atal Setu, got %.0f m of toll road", toll)
	}
	for _, q := range []string{"mode=car&avoid=toll", "mode=two_wheeler", "mode=walk"} {
		if toll := results[q]["toll_m"].(float64); toll != 0 {
			t.Errorf("%s: %.0f m of toll road", q, toll)
		}
	}
	if m := results["mode=walk"]["mode"]; m != "foot" {
		t.Errorf("mode=walk answered as %v", m)
	}
	walk := results["mode=walk"]["duration_s"].(float64)
	if car := results["mode=car"]["duration_s"].(float64); walk < 5*car {
		t.Errorf("walking %.0f s is implausibly close to driving %.0f s", walk, car)
	}

	steps, _ := results["mode=car"]["steps"].([]any)
	if len(steps) < 3 {
		t.Fatalf("steps: %v", steps)
	}
	first, last := steps[0].(map[string]any), steps[len(steps)-1].(map[string]any)
	if first["type"] != "depart" || last["type"] != "arrive" || first["instruction"] == "" {
		t.Errorf("steps run %v … %v", first, last)
	}
	if _, body := get(t, h, "/api/v1/route?steps=false&"+trip); body["steps"] != nil {
		t.Errorf("steps=false still returned steps")
	}
	for _, q := range []string{"mode=plane", "avoid=ferry", "steps=maybe"} {
		if rec, _ := get(t, h, "/api/v1/route?"+q+"&"+trip); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", q, rec.Code)
		}
	}
}
