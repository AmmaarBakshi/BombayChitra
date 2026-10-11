package pmtiles

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestZXYToID(t *testing.T) {
	cases := []struct {
		z    uint8
		x, y uint32
		want uint64
	}{
		{0, 0, 0, 0},
		{1, 0, 0, 1},
		{1, 0, 1, 2},
		{1, 1, 1, 3},
		{1, 1, 0, 4},
		{2, 0, 0, 5},
		{20, 0, 0, 366503875925},
	}
	for _, c := range cases {
		got, err := ZXYToID(c.z, c.x, c.y)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("ZXYToID(%d,%d,%d) = %d, want %d", c.z, c.x, c.y, got, c.want)
		}
	}
	if _, err := ZXYToID(1, 2, 0); err == nil {
		t.Error("expected out-of-range error")
	}
}

// TestArchive runs against the locally built archive when it exists.
func TestArchive(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "build", "bombaychitra.pmtiles")
	if _, err := os.Stat(path); err != nil {
		t.Skip("archive not built")
	}
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	h := r.Header()
	if h.TileType != TileTypeMVT || h.MaxZoom < 14 {
		t.Fatalf("unexpected header %+v", h)
	}
	// Chhatrapati Shivaji Maharaj Terminus at every zoom must resolve to a stored tile.
	lon, lat := 72.8355, 18.9398
	for z := uint8(6); z <= h.MaxZoom; z++ { // low zooms over open sea may be empty
		x, y := lonLatToTile(lon, lat, z)
		data, ok, err := r.Tile(z, x, y)
		if err != nil || !ok || len(data) == 0 {
			t.Fatalf("tile %d/%d/%d: ok=%v err=%v", z, x, y, ok, err)
		}
		if h.TileCompression == CompressionGzip {
			if _, err := Gunzip(data); err != nil {
				t.Fatalf("tile %d/%d/%d not gzip: %v", z, x, y, err)
			}
		}
	}
	if _, ok, _ := r.Tile(h.MaxZoom, 0, 0); ok {
		t.Error("tile far outside bounds should be missing")
	}
	if len(r.Metadata()) == 0 || r.Metadata()[0] != '{' {
		t.Error("metadata should be JSON")
	}
}

func lonLatToTile(lon, lat float64, z uint8) (uint32, uint32) {
	n := math.Exp2(float64(z))
	x := (lon + 180) / 360 * n
	latRad := lat * math.Pi / 180
	y := (1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * n
	return uint32(x), uint32(y)
}
