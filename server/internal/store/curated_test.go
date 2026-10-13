package store

import (
	"testing"
	"time"
)

func TestSeasonContains(t *testing.T) {
	monsoon := Season{From: 6, To: 9}
	for month, want := range map[time.Month]bool{time.May: false, time.June: true, time.September: true, time.October: false} {
		if got := monsoon.Contains(time.Date(2026, month, 15, 0, 0, 0, 0, time.UTC)); got != want {
			t.Errorf("June-September contains %s: %v", month, got)
		}
	}
	winter := Season{From: 11, To: 2}
	if !winter.Contains(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)) || winter.Contains(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("a season across the new year")
	}
}
