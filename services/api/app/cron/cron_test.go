package cron

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	for _, expr := range []string{"* * * * *", "0 9 * * 1", "*/5 0-6 1,15 * MON-FRI", "daily", "@weekly", " hourly "} {
		if _, err := Parse(expr); err != nil {
			t.Errorf("Parse(%q): %v", expr, err)
		}
	}
	for _, expr := range []string{"", "nope", "* * * *", "0 0 * * * *", "@every 1h", "TZ=UTC 0 0 * * *", "61 * * * *"} {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) accepted", expr)
		}
	}
}

func TestNextInTimeZone(t *testing.T) {
	cases := []struct{ after, want string }{
		// Before the change to summer time (31 March 2025): 09:00 CET.
		{"2025-03-25T12:00:00Z", "2025-03-31T07:00:00Z"},
		{"2025-03-20T12:00:00Z", "2025-03-24T08:00:00Z"},
		// Before the change back to winter time (26 October 2025): 09:00 CEST, then CET.
		{"2025-10-21T12:00:00Z", "2025-10-27T08:00:00Z"},
		{"2025-10-14T12:00:00Z", "2025-10-20T07:00:00Z"},
	}
	for _, c := range cases {
		after, _ := time.Parse(time.RFC3339, c.after)
		got, err := Next("0 9 * * 1", "Europe/Amsterdam", after)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format(time.RFC3339) != c.want {
			t.Errorf("Next after %s = %s, want %s", c.after, got.Format(time.RFC3339), c.want)
		}
	}
}

func TestNextDefaultsToUTC(t *testing.T) {
	after, _ := time.Parse(time.RFC3339, "2025-01-01T10:30:00Z")
	got, err := Next("daily", "", after)
	if err != nil || got.Format(time.RFC3339) != "2025-01-02T00:00:00Z" {
		t.Errorf("Next = %v, %v", got, err)
	}
	if _, err := Next("daily", "Mars/Base", after); err == nil {
		t.Error("Mars/Base accepted")
	}
}
