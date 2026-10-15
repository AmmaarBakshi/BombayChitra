package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestSearchBatch(t *testing.T) {
	h := testServer(t)
	rec, body := post(t, h, "/api/v1/search/batch", `{"queries": ["CST", "400050", "  ", "Gateway of India"], "near": {"lat": 19.07, "lon": 72.87}}`)
	if rec.Code != http.StatusOK || body["count"].(float64) != 4 {
		t.Fatalf("%d %v", rec.Code, body)
	}
	results := body["results"].([]any)
	first := func(i int) map[string]any {
		items := results[i].(map[string]any)["items"].([]any)
		if len(items) == 0 {
			return nil
		}
		return items[0].(map[string]any)
	}
	if r := first(0); r == nil || !strings.Contains(r["name"].(string), "CSMT") {
		t.Errorf("CST: %v", r)
	}
	if r := first(1); r == nil || r["kind"] != "area" {
		t.Errorf("400050 should find the PIN code area: %v", r)
	}
	if r := results[2].(map[string]any); r["count"].(float64) != 0 || r["query"] != "" {
		t.Errorf("a blank query should have no results: %v", r)
	}
	if len(results[3].(map[string]any)["items"].([]any)) != 1 {
		t.Error("the default limit is one result per query")
	}

	// The same answers as one search at a time.
	_, single := get(t, h, "/api/v1/search?q=Gateway+of+India&lat=19.07&lon=72.87&limit=1")
	if a, b := first(3)["ref"], single["items"].([]any)[0].(map[string]any)["ref"]; a != b {
		t.Errorf("batch found %v, search found %v", a, b)
	}

	_, body = post(t, h, "/api/v1/search/batch", `{"queries": ["Andheri"], "kind": ["line"], "limit": 3}`)
	for _, item := range body["results"].([]any)[0].(map[string]any)["items"].([]any) {
		if item.(map[string]any)["kind"] != "line" {
			t.Errorf("kind=line returned %v", item)
		}
	}
}

func TestSearchBatchRejectsBadRequests(t *testing.T) {
	h := testServer(t)
	many := `"x",` + strings.Repeat(`"x",`, 100)
	for _, body := range []string{
		`[]`,
		`{"queries": []}`,
		`{"queries": [` + strings.TrimSuffix(many, ",") + `]}`,
		`{"queries": ["CST"], "limit": 11}`,
		`{"queries": ["CST"], "kind": ["planet"]}`,
		`{"queries": ["CST"], "near": {"lat": 91, "lon": 72}}`,
		`{"q": "CST"}`,
	} {
		if rec, _ := post(t, h, "/api/v1/search/batch", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%.50s: %d, want 400", body, rec.Code)
		}
	}
}
