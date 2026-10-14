package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestJourney(t *testing.T) {
	h := testServer(t)
	const trip = "from=19.0190,72.8430&to=19.0864,72.9087" // Dadar to Ghatkopar
	rec, body := get(t, h, "/api/v1/journey?"+trip)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	legs := body["legs"].([]any)
	if len(legs) < 3 || body["rides"].(float64) < 1 {
		t.Fatalf("legs %v", legs)
	}
	first, last := legs[0].(map[string]any), legs[len(legs)-1].(map[string]any)
	if first["type"] != "walk" || last["type"] != "walk" {
		t.Errorf("a journey starts and ends on foot: %v … %v", first["type"], last["type"])
	}
	total := 0.0
	for _, l := range legs {
		leg := l.(map[string]any)
		total += leg["duration_s"].(float64)
		if w, ok := leg["wait_s"].(float64); ok {
			total += w
		}
		if coords := leg["geometry"].(map[string]any)["coordinates"].([]any); len(coords) < 2 {
			t.Errorf("leg without a line: %v", leg)
		}
		if leg["type"] != "ride" {
			continue
		}
		line := leg["line"].(map[string]any)
		if line["key"] == "" || line["label"] == "" || leg["stops"].(float64) < 1 || leg["headsign"] == "" {
			t.Errorf("ride without its line, stops or direction: %v", leg)
		}
		for _, end := range []string{"from", "to"} {
			if place, _ := leg[end].(map[string]any)["place"].(float64); place == 0 {
				t.Errorf("ride %s station is not linked to its place: %v", end, leg[end])
			}
		}
	}
	if d := body["duration_s"].(float64); math.Abs(total-d) > float64(len(legs)) {
		t.Errorf("legs add up to %.0f s, journey says %.0f", total, d)
	}
	if d := body["duration_s"].(float64); d < 15*60 || d > 50*60 {
		t.Errorf("Dadar to Ghatkopar in %.0f min", d/60)
	}

	// Metro only: Line 3 from Dadar and Line 1 into Ghatkopar.
	rec, body = get(t, h, "/api/v1/journey?modes=metro&"+trip)
	if rec.Code != http.StatusOK {
		t.Fatalf("metro only: %d %s", rec.Code, rec.Body.String())
	}
	for _, l := range body["legs"].([]any) {
		if leg := l.(map[string]any); leg["type"] == "ride" && leg["line"].(map[string]any)["mode"] != "subway" {
			t.Errorf("metro only, but rode %v", leg["line"])
		}
	}

	// Next door: walk.
	_, body = get(t, h, "/api/v1/journey?from=19.0190,72.8430&to=19.0200,72.8440")
	if legs := body["legs"].([]any); len(legs) != 1 || body["rides"].(float64) != 0 {
		t.Errorf("a short trip should be a walk: %v", legs)
	}

	if rec, body := get(t, h, "/api/v1/journey?from=19.0190,72.8430&to=19.9000,73.3000"); rec.Code != http.StatusUnprocessableEntity ||
		body["error"].(map[string]any)["code"] != "no_station" {
		t.Errorf("no station near the destination: %d %v", rec.Code, body)
	}
	for _, q := range []string{"modes=bus&" + trip, "max_walk=50&" + trip, "from=19.0190,72.8430", "format=kml&" + trip} {
		if rec, _ := get(t, h, "/api/v1/journey?"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", q, rec.Code)
		}
	}
}

func TestJourneyGeoJSON(t *testing.T) {
	h := testServer(t)
	rec, _ := get(t, h, "/api/v1/journey?from=19.0190,72.8430&to=19.0864,72.9087&format=geojson")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/geo+json") {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	var fc struct {
		Features []struct {
			Geometry   struct{ Type string }
			Properties map[string]any
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatal(err)
	}
	roles := map[string]int{}
	for _, f := range fc.Features {
		roles[f.Properties["role"].(string)]++
		if f.Properties["role"] == "leg" && f.Properties["type"] == "ride" && f.Properties["colour"] == nil {
			t.Errorf("ride without a colour: %v", f.Properties)
		}
	}
	if roles["leg"] < 3 || roles["board"] < 1 || roles["alight"] < 1 || roles["from"] != 1 || roles["to"] != 1 {
		t.Errorf("feature roles %v", roles)
	}
}

func TestHealthReportsTheJourneyNetwork(t *testing.T) {
	h := testServer(t)
	journeys := func() map[string]any {
		_, body := get(t, h, "/api/v1/health")
		check, _ := body["checks"].(map[string]any)["journeys"].(map[string]any)
		return check
	}
	if c := journeys(); c["status"] != "not_loaded" {
		t.Errorf("before any journey: %v", c)
	}
	get(t, h, "/api/v1/journey?from=19.0190,72.8430&to=19.0864,72.9087")
	c := journeys()
	detail, _ := c["detail"].(map[string]any)
	if c["status"] != "ok" || detail["stations"].(float64) < 150 || detail["lines"].(float64) < 10 {
		t.Errorf("after a journey: %v", c)
	}
}
