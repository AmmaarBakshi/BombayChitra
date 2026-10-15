package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageCountsEveryPlace(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "build", "bombaychitra.db")
	if _, err := os.Stat(path); err != nil {
		t.Skip("dataset not built")
	}
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	// A coarse grid over the region: every place falls in some cell.
	cells, err := st.Coverage(ctx, 72.7, 18.8, 0.1, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	var osm, overture, buildings int
	for _, c := range cells {
		if c.X < 0 || c.Y < 0 {
			t.Errorf("cell %d,%d west or south of the origin", c.X, c.Y)
		}
		osm += c.OSMPlaces
		overture += c.OverturePlaces
		buildings += c.Buildings
	}
	var wantOSM, wantOverture, wantBuildings int
	db := st.DB()
	if err := db.QueryRow(`SELECT SUM(osm_type != 'overture'), SUM(osm_type = 'overture') FROM places WHERE lon >= 72.7 AND lat >= 18.8`).Scan(&wantOSM, &wantOverture); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM buildings WHERE lon >= 72.7 AND lat >= 18.8`).Scan(&wantBuildings); err != nil {
		t.Fatal(err)
	}
	if osm != wantOSM || overture != wantOverture || buildings != wantBuildings {
		t.Errorf("cells hold %d/%d places and %d buildings, the tables %d/%d and %d", osm, overture, buildings, wantOSM, wantOverture, wantBuildings)
	}
	if overture == 0 {
		t.Log("dataset built without Overture Maps")
	}
}
