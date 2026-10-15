package export

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

func TestMapRouletteWritesOneTaskPerLine(t *testing.T) {
	var buf bytes.Buffer
	w := newWriter("maproulette", &buf)
	for _, id := range []string{"overture:a", "overture:b"} {
		if err := w.feature(feature{Type: "Feature", ID: id, Geometry: point(72.8, 19.1), Properties: map[string]any{"name": id}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.close(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d lines, want 2: %q", len(lines), buf.String())
	}
	for i, line := range lines {
		var fc struct {
			Type     string `json:"type"`
			Features []struct {
				ID string `json:"id"`
			} `json:"features"`
		}
		if err := json.Unmarshal([]byte(line), &fc); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		if fc.Type != "FeatureCollection" || len(fc.Features) != 1 || fc.Features[0].ID == "" {
			t.Errorf("line %d: %s", i, line)
		}
	}
}

func TestOvertureExportFromDataset(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "build", "bombaychitra.db")
	if _, err := os.Stat(path); err != nil {
		t.Skip("dataset not built")
	}
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	opts := Options{Kind: "overture", Format: "maproulette", Area: "makabo", Categories: []string{"hospital"}}
	var buf bytes.Buffer
	n, err := Run(ctx, st, opts, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Skip("dataset built without Overture Maps")
	}
	if lines := strings.Count(buf.String(), "\n"); lines != n {
		t.Errorf("%d tasks for %d places", lines, n)
	}
	first := strings.SplitN(buf.String(), "\n", 2)[0]
	if !strings.Contains(first, `"id":"overture:`) || !strings.Contains(first, `"source":"Overture Maps"`) {
		t.Errorf("first task: %s", first)
	}

	// Only Overture's places: every OpenStreetMap hospital in MaKaBo is left out.
	var all bytes.Buffer
	total, err := Run(ctx, st, Options{Kind: "places", Format: "ndjson", Area: "makabo", Categories: []string{"hospital"}}, &all)
	if err != nil {
		t.Fatal(err)
	}
	if osm := strings.Count(all.String(), `"source":"openstreetmap"`); n+osm != total {
		t.Errorf("%d Overture + %d OpenStreetMap hospitals, but %d in all", n, osm, total)
	}

	buf.Reset()
	if _, err := Run(ctx, st, Options{Kind: "overture", Format: "csv", Area: "makabo", Categories: []string{"hospital"}}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "overture_id,name,category,lon,lat") {
		t.Errorf("CSV header: %.80s", buf.String())
	}
}
