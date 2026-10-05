package domain

import (
	"testing"
	"time"
)

func TestNewDetails(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 500, time.UTC)
	d := NewDetails(" debian ", "12", "amd64", "6.1.0", 4, 8<<30, "5.4.2", 49*time.Hour+3*time.Minute+1040*time.Millisecond, now)
	if d.OS != "debian 12" || d.Arch != "amd64" || d.Kernel != "6.1.0" || d.CPUs != 4 || d.Memory != 8<<30 || d.PodmanVersion != "5.4.2" {
		t.Fatalf("%+v", d)
	}
	if want := time.Date(2026, 10, 3, 10, 56, 58, 0, time.UTC); !d.UpSince.Equal(want) {
		t.Errorf("up since %v, want %v", d.UpSince, want)
	}
	if d := NewDetails("", "", "arm64", "6.1", 1, 1, "5", 0, now); d.OS != "" || !d.UpSince.IsZero() {
		t.Errorf("unknown: %+v", d)
	}
}
