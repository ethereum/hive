package api

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	start := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		end  time.Time
		want string
	}{
		{"unknown end", time.Time{}, "-"},
		{"end before start", start.Add(-time.Second), "-"},
		{"seconds", start.Add(42 * time.Second), "42s"},
		{"minutes", start.Add(17*time.Minute + 5*time.Second), "17m 5s"},
		{"hours", start.Add(4*time.Hour + 12*time.Minute + 30*time.Second), "4h 12m"},
		{"days", start.Add(51*time.Hour + 20*time.Minute), "2d 3h"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatDuration(start, tc.end); got != tc.want {
				t.Errorf("FormatDuration() = %q, want %q", got, tc.want)
			}
		})
	}
}
