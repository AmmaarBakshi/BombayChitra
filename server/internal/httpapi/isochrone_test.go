package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type isoFeature struct {
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

func isochrones(t *testing.T, query string) []isoFeature {
	t.Helper()
	rec, _ := get(t, testServer(t), "/api/v1/isochrone?"+query)
	if rec.Code == http.StatusServiceUnavailable {
		t.Skip("dataset built before travel modes")
	}
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/geo+json") {
		t.Fatalf("%s: %d %s", query, rec.Code, rec.Body.String())
	}
	var fc struct{ Features []isoFeature }
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatal(err)
	}
	return fc.Features
}

func TestIsochroneBands(t *testing.T) {
	const kharghar = "from=19.0330,73.0700"
	features := isochrones(t, kharghar+"&mode=car&minutes=30,10,20")
	if len(features) != 4 || features[3].Properties["role"] != "from" {
		t.Fatalf("want three bands and the start, got %d features", len(features))
	}
	prev := 1e9
	for i, want := range []float64{30, 20, 10} {
		f := features[i]
		if f.Geometry.Type != "MultiPolygon" || f.Properties["minutes"].(float64) != want {
			t.Fatalf("feature %d: %s %v, want the %v-minute band", i, f.Geometry.Type, f.Properties["minutes"], want)
		}
		area := f.Properties["area_km2"].(float64)
		if area <= 0 || area >= prev {
			t.Errorf("%v minutes: %.1f km², want less than the longer band (%.1f)", want, area, prev)
		}
		prev = area
	}
	var polygons [][][][2]float64
	if err := json.Unmarshal(features[0].Geometry.Coordinates, &polygons); err != nil || len(polygons) == 0 {
		t.Fatalf("30-minute band: %v", err)
	}
	for _, p := range polygons {
		if ring := p[0]; len(ring) < 4 || ring[0] != ring[len(ring)-1] {
			t.Fatalf("ring not closed: %v", ring)
		}
	}

	walk := isochrones(t, kharghar+"&mode=walk&minutes=30")
	if w, c := walk[0].Properties["area_km2"].(float64), features[0].Properties["area_km2"].(float64); w*20 > c {
		t.Errorf("30 min on foot covers %.1f km², implausibly close to 30 min by car (%.1f)", w, c)
	}
}

func TestIsochroneErrors(t *testing.T) {
	h := testServer(t)
	for _, q := range []string{"from=19.03,73.07&minutes=0", "from=19.03,73.07&minutes=61", "from=19.03,73.07&minutes=5,10,15,20,25", "from=19.03,73.07&mode=plane", "minutes=10"} {
		if rec, _ := get(t, h, "/api/v1/isochrone?"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", q, rec.Code)
		}
	}
	// Far out in the Arabian Sea.
	if rec, body := get(t, h, "/api/v1/isochrone?from=18.9,72.5"); rec.Code != http.StatusUnprocessableEntity || body["error"].(map[string]any)["code"] != "no_road" {
		t.Errorf("at sea: %d %v", rec.Code, body)
	}
}
