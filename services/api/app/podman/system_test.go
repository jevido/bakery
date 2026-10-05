package podman

import (
	"testing"
	"time"
)

func TestParseUptime(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want time.Duration
	}{
		{"228h 8m 49.00s (Approximately 9.50 days)", 228*time.Hour + 8*time.Minute + 49*time.Second},
		{"52h 3m 1.04s (Approximately 2.17 days)", 52*time.Hour + 3*time.Minute + 1040*time.Millisecond},
		{"0h 12m 5.5s", 12*time.Minute + 5500*time.Millisecond},
		{"3m 2s", 3*time.Minute + 2*time.Second},
		{"", 0},
		{"soon", 0},
	} {
		if got := parseUptime(c.raw); got != c.want {
			t.Errorf("%q: %v, want %v", c.raw, got, c.want)
		}
	}
}
