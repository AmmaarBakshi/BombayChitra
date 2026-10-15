package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestCoverageLevels(t *testing.T) {
	cases := []struct {
		osm, overture int
		level         string
	}{
		{0, 0, "few"}, {2, 2, "few"}, {1, 9, "thin"}, {5, 5, "partial"}, {9, 1, "good"}, {7, 3, "good"},
	}
	for _, c := range cases {
		share, level := coverageLevel(c.osm, c.overture)
		if level != c.level {
			t.Errorf("%d OSM, %d Overture: %s, want %s", c.osm, c.overture, level, c.level)
		}
		if (share == nil) != (c.osm+c.overture == 0) {
			t.Errorf("%d OSM, %d Overture: share %v", c.osm, c.overture, share)
		}
	}
}

func TestCoverage(t *testing.T) {
	h := testServer(t)
	rec, _ := get(t, h, "/api/v1/coverage?cell=2000")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var fc struct {
		Features []struct {
			Geometry struct {
				Type        string         `json:"type"`
				Coordinates [][][2]float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatal(err)
	}
	if len(fc.Features) < 100 {
		t.Fatalf("only %d cells", len(fc.Features))
	}
	levels := map[string]int{}
	for _, f := range fc.Features {
		if f.Geometry.Type != "Polygon" || len(f.Geometry.Coordinates[0]) != 5 {
			t.Fatalf("cell is not a closed square: %v", f.Geometry)
		}
		levels[f.Properties["level"].(string)]++
		osm, overture := f.Properties["osm_places"].(float64), f.Properties["overture_places"].(float64)
		if share, ok := f.Properties["osm_share"].(float64); ok && (share < 0 || share > 1 || osm+overture == 0) {
			t.Errorf("share %v with %v/%v places", share, osm, overture)
		}
	}
	if levels["good"] == 0 || levels["thin"]+levels["partial"] == 0 {
		t.Errorf("levels %v: expected well-mapped and thin cells", levels)
	}
	// The second request is served from the cache, identically.
	if again, _ := get(t, h, "/api/v1/coverage?cell=2000"); again.Body.String() != rec.Body.String() {
		t.Error("a repeated request differs")
	}
	for _, q := range []string{"cell=100", "cell=9000", "cell=big"} {
		if rec, _ := get(t, h, "/api/v1/coverage?"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", q, rec.Code)
		}
	}
}
