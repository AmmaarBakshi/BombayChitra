package journey

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
	_ "modernc.org/sqlite"
)

// dataset opens the locally built dataset, skipping when it has not been built.
func dataset(t *testing.T) *Network {
	t.Helper()
	path := filepath.Join("..", "..", "..", "data", "build", "bombaychitra.db")
	if _, err := os.Stat(path); err != nil {
		t.Skip("dataset not built")
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	n, err := Load(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDatasetJourneys(t *testing.T) {
	n := dataset(t)
	stats := n.Stats()
	if stats.Stations < 150 || stats.Lines < 10 {
		t.Fatalf("network too small: %+v", stats)
	}
	linked := 0
	for _, st := range n.Stations {
		if st.Place != 0 {
			linked++
		}
	}
	if linked < stats.Stations*3/4 {
		t.Errorf("only %d of %d stations linked to their place", linked, stats.Stations)
	}

	cases := []struct {
		name     string
		from, to geo.Point
		lines    []string // lines ridden, in order
		maxMin   float64
	}{
		// Churchgate to Borivali: the Western line all the way.
		{"churchgate-borivali", geo.Point{Lon: 72.8268, Lat: 18.9352}, geo.Point{Lon: 72.8566, Lat: 19.2290},
			[]string{"Mumbai Suburban Railway:W"}, 90},
		// Versova to Ghatkopar: Metro Line 1 end to end.
		{"versova-ghatkopar", geo.Point{Lon: 72.8214, Lat: 19.1303}, geo.Point{Lon: 72.9087, Lat: 19.0864},
			[]string{"Mumbai Metro:1"}, 35},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			j, err := n.Plan(c.from, c.to, Options{MaxWalkM: 1500})
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, l := range j.Legs {
				if l.Ride {
					r, _ := n.RouteOf(l)
					lines = append(lines, r.Line.Key)
				}
			}
			if len(lines) != len(c.lines) {
				t.Fatalf("rides %v, want %v", lines, c.lines)
			}
			for i := range lines {
				if lines[i] != c.lines[i] {
					t.Fatalf("rides %v, want %v", lines, c.lines)
				}
			}
			if j.DurationS/60 > c.maxMin {
				t.Errorf("%.0f min, want under %.0f", j.DurationS/60, c.maxMin)
			}
		})
	}
}
