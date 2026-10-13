package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalisePin(t *testing.T) {
	cases := map[string]string{"400050": "400050", "400 050": "400050", "Mumbai - 400050": "400050",
		"40005": "", "040005": "", "4000501": "", "": "", "MH 400053.": "400053"}
	for in, want := range cases {
		if got := normalisePin(in); got != want {
			t.Errorf("normalisePin(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDescribeLocalities(t *testing.T) {
	got := describeLocalities(map[string]int{"Bandra West": 40, "Pali Hill": 12, "Khar": 9, "Chapel Road": 2})
	if got != "Bandra West, Pali Hill and Khar" {
		t.Errorf("describeLocalities = %q", got)
	}
	if got := describeLocalities(map[string]int{"Juhu": 3}); got != "Juhu" {
		t.Errorf("one locality: %q", got)
	}
}

func TestPinAreaDropsOutliers(t *testing.T) {
	var places []pinPlace
	for i := range 20 {
		places = append(places, pinPlace{lon: 72.83 + float64(i%5)*0.002, lat: 19.05 + float64(i/5)*0.002, locality: "Bandra West", category: "cafe"})
	}
	// A branch in Thane carrying the Bandra head office's PIN.
	places = append(places, pinPlace{lon: 72.97, lat: 19.19, locality: "Thane", category: "bank"})
	a := buildPinArea("400050", places)
	if a == nil {
		t.Fatal("no area")
	}
	if a.stats["places"] != 20 || a.stats["places:bank"] != 0 {
		t.Errorf("outlier kept: %v", a.stats)
	}
	if a.area.AreaKm2 < 0.5 || a.area.AreaKm2 > 1 { // 0.008° × 0.006° ≈ 0.55 km²
		t.Errorf("area %.2f km²", a.area.AreaKm2)
	}
	if a.area.Kind != "pincode" || a.area.Description != "Bandra West" {
		t.Errorf("area %+v", a.area)
	}
	if buildPinArea("400051", places[:5]) != nil {
		t.Error("five places are too few for an area")
	}
}

func TestPinCodesFromDataset(t *testing.T) {
	db := filepath.Join("..", "..", "..", "data", "build", "bombaychitra.db")
	if _, err := os.Stat(db); err != nil {
		t.Skip("dataset not built")
	}
	s, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	pins, err := s.PinCodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) < 100 {
		t.Fatalf("only %d PIN code areas", len(pins))
	}
	bandra, err := s.PinCode(ctx, "400050")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("400050: %s, %.1f km², %v places", bandra.Description, bandra.AreaKm2, bandra.Stats["places"])
	if bandra.AreaKm2 < 0.5 || bandra.AreaKm2 > 25 || len(bandra.Geometry) == 0 {
		t.Errorf("400050 looks wrong: %+v", bandra)
	}
	places, err := s.Places(ctx, PlaceQuery{Area: "400050", Categories: []string{"hospital"}, Limit: 50})
	if err != nil || len(places) == 0 {
		t.Errorf("hospitals in 400050: %d, %v", len(places), err)
	}
	if s.PinCodeExists(ctx, "999999") || !s.PinCodeExists(ctx, "400050") {
		t.Error("PinCodeExists")
	}
}
