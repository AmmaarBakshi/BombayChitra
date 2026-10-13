package httpapi

import (
	"math"
	"net/http"
	"testing"
)

func TestDistanceToPath(t *testing.T) {
	path := [][2]float64{{72.80, 19.00}, {72.81, 19.00}, {72.81, 19.01}}
	// 0.001° of latitude ≈ 110.6 m north of the first leg.
	if d := distanceToPath(72.805, 19.001, path); math.Abs(d-110.6) > 1 {
		t.Errorf("distance %.1f m, want ≈ 110.6", d)
	}
	if d := distanceToPath(72.81, 19.005, path); d > 0.01 {
		t.Errorf("a point on the path: %.2f m", d)
	}
	if d := distanceToPath(72.80, 19.00, path[:1]); d != 0 {
		t.Errorf("single point: %.2f m", d)
	}
}

func TestHeritageAndHotspots(t *testing.T) {
	h := testServer(t)
	_, heritage := get(t, h, "/api/v1/heritage?status=unesco")
	if heritage["count"] == float64(0) {
		t.Skip("dataset built before the curated tables")
	}
	if heritage["count"] != float64(3) {
		t.Errorf("%v World Heritage Sites, want 3", heritage["count"])
	}
	if rec, fc := get(t, h, "/api/v1/heritage?format=geojson"); rec.Code != http.StatusOK || fc["type"] != "FeatureCollection" {
		t.Errorf("heritage GeoJSON: %d", rec.Code)
	}
	_, csmt := get(t, h, "/api/v1/places/1074415162")
	if site, _ := csmt["heritage"].(map[string]any); site["status"] != "unesco" || site["year"] != float64(2004) {
		t.Errorf("CSMT heritage: %v", csmt["heritage"])
	}

	_, hot := get(t, h, "/api/v1/hotspots")
	season, _ := hot["season"].(map[string]any)
	if items, _ := hot["items"].([]any); len(items) < 10 || season["from"] != float64(6) || season["to"] != float64(9) {
		t.Errorf("hotspots: %v", hot)
	}

	// Parel to Dadar along Dr Ambedkar Road passes Hindmata.
	_, route := get(t, h, "/api/v1/route?from=19.0035,72.8402&to=19.0175,72.8455&steps=false")
	found := false
	for _, it := range route["hotspots"].([]any) {
		found = found || it.(map[string]any)["id"] == "hindmata"
	}
	if !found {
		t.Errorf("route through Hindmata: hotspots %v", route["hotspots"])
	}
	if _, ok := route["monsoon"].(bool); !ok {
		t.Errorf("route has no monsoon flag")
	}
	if rec, _ := get(t, h, "/api/v1/heritage?status=famous"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown status: %d", rec.Code)
	}
}
