package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/app/cron"
)

// ScheduledBackup is the aggregate root of when a Database is backed up by
// itself, how many of its Backup executions are kept and where they go. A
// Database has any number of them, as in Coolify. Times are UTC.
type ScheduledBackup struct {
	ID         uint64
	DatabaseID uint64
	Enabled    bool
	// Cron is a five-field cron expression or one of Coolify's shortcuts
	// (daily, @hourly…), kept as a Member typed it.
	Cron      string
	Retention int
	// S3StorageID is the S3 storage Backup executions are uploaded to; 0 is local
	// disk only.
	S3StorageID uint64
	// EnabledAt is when the schedule was last switched on; it first fires
	// at the next time after it.
	EnabledAt time.Time
}

// ScheduledBackupInput is what a Member sets on a Scheduled backup.
type ScheduledBackupInput struct {
	Enabled     bool
	Cron        string
	Retention   int
	S3StorageID uint64
}

// DefaultScheduledBackup is what a new Database starts with: off, daily at
// 03:00 UTC, keeping 7.
var DefaultScheduledBackup = ScheduledBackupInput{Cron: "0 3 * * *", Retention: 7}

// Check validates the cron expression and the Retention.
func (in ScheduledBackupInput) Check() error {
	if _, err := cron.Parse(in.Cron); err != nil {
		return invalid("scheduled_backup.cron", "%q is neither a five-field cron expression (minute hour day month weekday) nor every_minute, hourly, daily, weekly, monthly or yearly", in.Cron)
	}
	if in.Retention < 1 || in.Retention > 100 {
		return invalid("scheduled_backup.retention", "keep between 1 and 100 backups")
	}
	return nil
}

// ErrNoBackups is returned for a Database type that is not backed up.
var ErrNoBackups = errors.New("backups are not available for this database type: Redis and Valkey keep their data in an append-only file")

// NewScheduledBackup makes a Scheduled backup of d. Only a Database type
// with backups has any; switching it on at now remembers now, so it does
// not fire for a time that already passed.
func NewScheduledBackup(d Database, in ScheduledBackupInput, now time.Time) (ScheduledBackup, error) {
	if !d.Type.Spec().Backups {
		return ScheduledBackup{}, ErrNoBackups
	}
	s := ScheduledBackup{DatabaseID: d.ID}
	return s, s.Update(in, now)
}

// Update replaces what a Member sets. Switching it on at now remembers
// now; saving it again while on keeps when it was switched on.
func (s *ScheduledBackup) Update(in ScheduledBackupInput, now time.Time) error {
	in.Cron = strings.TrimSpace(in.Cron)
	if err := in.Check(); err != nil {
		return err
	}
	switch {
	case in.Enabled && !s.Enabled:
		s.EnabledAt = now.UTC()
	case !in.Enabled:
		s.EnabledAt = time.Time{}
	}
	s.Enabled, s.Cron, s.Retention, s.S3StorageID = in.Enabled, in.Cron, in.Retention, in.S3StorageID
	return nil
}

// Next is the first scheduled time after after, in UTC; the zero time for
// an invalid expression.
func (s ScheduledBackup) Next(after time.Time) time.Time {
	c, err := cron.Parse(s.Cron)
	if err != nil {
		return time.Time{}
	}
	return c.Next(after.UTC())
}

// NextAt is when the schedule fires next, given its last scheduled Backup
// execution; zero when off.
func (s ScheduledBackup) NextAt(lastScheduledStart time.Time) time.Time {
	if !s.Enabled {
		return time.Time{}
	}
	from := s.EnabledAt
	if lastScheduledStart.After(from) {
		from = lastScheduledStart
	}
	return s.Next(from)
}

// Due reports whether a scheduled Backup execution should start now: the
// schedule is on and its next time after the last scheduled Backup execution
// (or after it was switched on, whichever is later) has come. However many
// times were missed, one Backup execution catches up.
func (s ScheduledBackup) Due(lastScheduledStart, now time.Time) bool {
	next := s.NextAt(lastScheduledStart)
	return !next.IsZero() && !next.After(now)
}
