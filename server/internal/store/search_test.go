package store

import "testing"

func TestMatchExpression(t *testing.T) {
	cases := map[string]string{
		"andheri sta":  `"andheri" "sta"*`,
		"  Lilavati  ": `"lilavati"*`,
		`he said "hi"`: `"he" "said" "hi"*`,
		"छत्रपती शिवाजी": `"छत्रपती" "शिवाजी"*`,
		"--- !!!":           ``,
		"a b c d e f g h i": `"a" "b" "c" "d" "e" "f" "g" "h"*`,
	}
	for in, want := range cases {
		if got := MatchExpression(in); got != want {
			t.Errorf("MatchExpression(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestNameBoost(t *testing.T) {
	cases := []struct {
		kind, name, alt, query string
		want                   float64
	}{
		{"place", "dadar", "", "dadar", 2.2},
		{"place", "dadar west", "", "dadar", 1.6},
		{"place", "mumbai csmt", "", "csmt", 1.4},
		{"place", "andheri east", "", "heri", 1.15},
		{"place", "hallow pul", "", "dadar", 1},
		// Codes in capitals among the other names count as exact matches.
		{"place", "mumbai csmt", "Victoria Terminus CSMT;CST;VT", "cst", 5.5},
		{"place", "mumbai csmt", "Victoria Terminus CSMT;CST;VT", "victoria terminus", 5},
		// A word that merely appears in another name, or in lower case, is not a code.
		{"place", "shishu vihar", "Dadar Shishu Vihar", "dadar", 1},
		{"place", "some school", "near Cst road", "cst", 1},
		// Lines mention their termini in their other names; those are not aliases.
		{"line", "harbour line", "Harbour Line: CSMT => Panvel", "csmt", 1},
	}
	for _, c := range cases {
		if got := nameBoost(c.kind, c.name, c.alt, c.query); got != c.want {
			t.Errorf("nameBoost(%s %q, alt %q, %q) = %v, want %v", c.kind, c.name, c.alt, c.query, got, c.want)
		}
	}
}

func TestNormaliseDropsAccents(t *testing.T) {
	if got := normalise("Thāne  (West)"); got != "thane west" {
		t.Errorf("normalise = %q", got)
	}
}

func TestDedupeKeepsDistinctPlaces(t *testing.T) {
	results := []SearchResult{
		{Kind: "place", Name: "Lilavati Hospital", Lon: 72.8263, Lat: 19.0510},
		{Kind: "place", Name: "Lilavati Hospital", Lon: 72.8265, Lat: 19.0512},   // its bus stop across the road
		{Kind: "place", Name: "Lilavati Hospital", Lon: 72.95, Lat: 19.2},        // a different place far away
		{Kind: "street", Name: "Kherwadi Flyover", Lon: 72.8480, Lat: 19.0600},   //
		{Kind: "bridge", Name: "Kherwadi Flyover", Lon: 72.8482, Lat: 19.0601},   // the same flyover as a bridge
		{Kind: "locality", Name: "Kherwadi Flyover", Lon: 72.8482, Lat: 19.0601}, // different kind: kept
	}
	got := dedupe(results, 10)
	if len(got) != 4 {
		t.Fatalf("dedupe kept %d results: %+v", len(got), got)
	}
	if got := dedupe(results, 2); len(got) != 2 {
		t.Errorf("limit not applied: %d", len(got))
	}
}
