// Package cron parses the cron expressions two contexts take: a Scheduled
// backup's Frequency (databases) and a Routine's Schedule trigger (work).
// Only plain five-field expressions and Coolify's shortcuts are allowed.
package cron

import (
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// shortcuts are the words Coolify takes for a Frequency
// (VALID_CRON_STRINGS), each with its five-field expression.
var shortcuts = map[string]string{
	"every_minute": "* * * * *",
	"hourly":       "0 * * * *",
	"daily":        "0 0 * * *",
	"weekly":       "0 0 * * 0",
	"monthly":      "0 0 1 * *",
	"yearly":       "0 0 1 1 *",
	"@hourly":      "0 * * * *",
	"@daily":       "0 0 * * *",
	"@weekly":      "0 0 * * 0",
	"@monthly":     "0 0 1 * *",
	"@yearly":      "0 0 1 1 *",
}

// Parse reads expr, a five-field expression or a shortcut. The schedule it
// answers evaluates in the location of the time it is given.
func Parse(expr string) (cron.Schedule, error) {
	expr = strings.TrimSpace(expr)
	if e, ok := shortcuts[expr]; ok {
		expr = e
	}
	// ParseStandard also takes other descriptors and a TZ= prefix; only
	// plain five fields are allowed, so the time zone is always explicit.
	if len(strings.Fields(expr)) != 5 {
		return nil, errors.New("five fields expected")
	}
	return cron.ParseStandard(expr)
}

// Location loads the time zone tz; empty is UTC.
func Location(tz string) (*time.Location, error) {
	if strings.TrimSpace(tz) == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(tz)
}

// Next is the first time expr fires after after, read as wall-clock time in
// tz (so "0 9 * * 1" is 09:00 there on both sides of a DST change), in UTC.
func Next(expr, tz string, after time.Time) (time.Time, error) {
	s, err := Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := Location(tz)
	if err != nil {
		return time.Time{}, err
	}
	return s.Next(after.In(loc)).UTC(), nil
}
