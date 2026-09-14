package handler

import (
	"testing"
	"time"
)

func TestParseWeekStart(t *testing.T) {
	// 2026-09-07 and 2026-09-14 are Mondays.
	sundayNight := time.Date(2026, 9, 13, 22, 0, 0, 0, time.UTC)
	mondayMorning := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	thisMonday := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	nextMonday := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		raw     string
		now     time.Time
		want    time.Time
		wantErr bool
	}{
		{"empty is the current week", "", sundayNight, thisMonday, false},
		{"current week", "2026-09-07", sundayNight, thisMonday, false},
		{"next week", "2026-09-14", sundayNight, nextMonday, false},
		{"page from Sunday submitted on Monday keeps its week", "2026-09-14", mondayMorning, nextMonday, false},
		{"a week that is over is refused", "2026-09-07", mondayMorning, time.Time{}, true},
		{"two weeks ahead is refused", "2026-09-21", sundayNight, time.Time{}, true},
		{"not a Monday", "2026-09-15", mondayMorning, time.Time{}, true},
		{"not a date", "next", sundayNight, time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWeekStart(tt.raw, tt.now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseWeekStart(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseWeekStart(%q) = %s, want %s", tt.raw, got, tt.want)
			}
		})
	}
}
