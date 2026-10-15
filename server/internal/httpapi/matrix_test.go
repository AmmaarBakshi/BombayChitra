package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(t *testing.T, h http.Handler, url, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("POST %s: %d, not JSON: %s", url, rec.Code, rec.Body.String())
	}
	return rec, out
}

func TestMatrix(t *testing.T) {
	h := testServer(t)
	// Andheri station, Bandra station and CSMT, as a square matrix; plus a point at sea as a destination.
	const places = `{"lat": 19.1197, "lon": 72.8464}, {"lat": 19.0544, "lon": 72.8402}, {"lat": 18.9400, "lon": 72.8355}`
	rec, body := post(t, h, "/api/v1/matrix", `{"sources": [`+places+`]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %v", rec.Code, body)
	}
	durations := body["durations"].([]any)
	distances := body["distances"].([]any)
	if len(durations) != 3 || len(durations[0].([]any)) != 3 {
		t.Fatalf("want a 3×3 matrix, got %v", durations)
	}
	for i := range 3 {
		if d := durations[i].([]any)[i].(float64); d != 0 {
			t.Errorf("place %d to itself: %.0f s", i, d)
		}
	}
	// Andheri → CSMT as the route gives it.
	_, route := get(t, h, "/api/v1/route?from=19.1197,72.8464&to=18.9400,72.8355&steps=false")
	if d, r := durations[0].([]any)[2].(float64), route["duration_s"].(float64); math.Abs(d-r) > 1 {
		t.Errorf("matrix %.0f s, route %.0f s", d, r)
	}
	if d, r := distances[0].([]any)[2].(float64), route["distance_m"].(float64); math.Abs(d-r) > 1 {
		t.Errorf("matrix %.0f m, route %.0f m", d, r)
	}

	_, body = post(t, h, "/api/v1/matrix", `{"sources": [{"lat": 19.1197, "lon": 72.8464}], "destinations": [{"lat": 18.5, "lon": 72.0}], "mode": "walk"}`)
	if body["mode"] != "foot" || body["durations"].([]any)[0].([]any)[0] != nil || body["destinations"].([]any)[0].(map[string]any)["snapped"] != nil {
		t.Errorf("a destination at sea should be null and unsnapped: %v", body)
	}
}

func TestMatrixRejectsBadRequests(t *testing.T) {
	h := testServer(t)
	many := strings.Repeat(`{"lat": 19.1, "lon": 72.85},`, 11)
	for _, body := range []string{
		`not json`,
		`{"sources": []}`,
		`{"sources": [` + strings.TrimSuffix(many, ",") + `]}`,
		`{"sources": [{"lat": 19.1, "lon": 72.85}], "mode": "plane"}`,
		`{"sources": [{"lat": 19.1, "lon": 72.85}], "optimize": "cheapest"}`,
		`{"sources": [{"lat": 19.1, "lon": 72.85}], "avoid": ["ferry"]}`,
		`{"sources": [{"lat": 99, "lon": 72.85}]}`,
		`{"origins": [{"lat": 19.1, "lon": 72.85}]}`,
	} {
		if rec, _ := post(t, h, "/api/v1/matrix", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%.60s: %d, want 400", body, rec.Code)
		}
	}
}
