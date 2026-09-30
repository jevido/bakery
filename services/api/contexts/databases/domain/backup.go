package domain

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// BackupSchedule is when a Database is backed up by itself. Times are UTC.
type BackupSchedule struct {
	Enabled bool
	// Cron is a five-field cron expression.
	Cron      string
	Retention int
	// S3StorageID is the S3 storage Backups are uploaded to; 0 is local
	// disk only.
	S3StorageID uint64
	// EnabledAt is when the schedule was last switched on; it first fires
	// at the next time after it.
	EnabledAt time.Time
}

// DefaultBackupSchedule is what a new Database starts with: off, daily at
// 03:00 UTC, keeping 7.
var DefaultBackupSchedule = BackupSchedule{Cron: "0 3 * * *", Retention: 7}

func parseCron(expr string) (cron.Schedule, error) {
	// ParseStandard also takes descriptors (@daily) and a TZ= prefix; only
	// plain five-field expressions are allowed, so times stay UTC.
	if len(strings.Fields(expr)) != 5 {
		return nil, errors.New("five fields expected")
	}
	return cron.ParseStandard(expr)
}

// Check validates the cron expression and the Retention.
func (s BackupSchedule) Check() error {
	if _, err := parseCron(s.Cron); err != nil {
		return invalid("backup_schedule.cron", "%q is not a five-field cron expression (minute hour day month weekday)", s.Cron)
	}
	if s.Retention < 1 || s.Retention > 100 {
		return invalid("backup_schedule.retention", "keep between 1 and 100 backups")
	}
	return nil
}

// Next is the first scheduled time after after, in UTC; the zero time for
// an invalid expression.
func (s BackupSchedule) Next(after time.Time) time.Time {
	c, err := parseCron(s.Cron)
	if err != nil {
		return time.Time{}
	}
	return c.Next(after.UTC())
}

// Due reports whether a scheduled Backup should start now: the schedule is
// on and its next time after the last scheduled Backup (or after it was
// switched on, whichever is later) has come. However many times were
// missed, one Backup catches up.
func (s BackupSchedule) Due(lastScheduledStart, now time.Time) bool {
	if !s.Enabled {
		return false
	}
	from := s.EnabledAt
	if lastScheduledStart.After(from) {
		from = lastScheduledStart
	}
	next := s.Next(from)
	return !next.IsZero() && !next.After(now)
}

// ErrNoBackups is returned for an Engine that is not backed up.
var ErrNoBackups = errors.New("backups are not available for this engine: Redis and Valkey keep their data in an append-only file")

// SetBackupSchedule replaces the Backup schedule. Switching it on at now
// remembers now, so it does not fire for a time that already passed.
func (d *Database) SetBackupSchedule(s BackupSchedule, now time.Time) error {
	s.Cron = strings.TrimSpace(s.Cron)
	if err := s.Check(); err != nil {
		return err
	}
	if s.Enabled && !d.Engine.Spec().Backups {
		return ErrNoBackups
	}
	switch {
	case s.Enabled && !d.BackupSchedule.Enabled:
		s.EnabledAt = now.UTC()
	case s.Enabled:
		s.EnabledAt = d.BackupSchedule.EnabledAt
	}
	d.BackupSchedule = s
	return nil
}

// BackupStatus is where a Backup is.
type BackupStatus string

const (
	BackupRunning   BackupStatus = "running"
	BackupSucceeded BackupStatus = "succeeded"
	BackupFailed    BackupStatus = "failed"
)

// BackupTrigger is what started a Backup.
type BackupTrigger string

const (
	TriggerManual    BackupTrigger = "manual"
	TriggerScheduled BackupTrigger = "scheduled"
)

// Backup is one dump of a Database.
type Backup struct {
	ID         uint64
	DatabaseID uint64
	Status     BackupStatus
	Trigger    BackupTrigger
	// FileName is relative to the Database's directory in the Backups
	// directory, and the last part of its S3 key.
	FileName  string
	SizeBytes int64
	// Local and S3 are where the Backup is.
	Local       bool
	S3          bool
	S3StorageID uint64
	Error       string
	StartedAt   time.Time
	FinishedAt  time.Time
}

// ErrBackupFinished is returned when a finished Backup is finished again.
var ErrBackupFinished = errors.New("backup already finished")

// NewBackup starts a Backup of d at now, with the file name it will be
// written to.
func NewBackup(d Database, trigger BackupTrigger, now time.Time) (Backup, error) {
	if !d.Engine.Spec().Backups {
		return Backup{}, ErrNoBackups
	}
	now = now.UTC()
	return Backup{
		DatabaseID: d.ID, Status: BackupRunning, Trigger: trigger,
		FileName:    now.Format("20060102T150405Z") + d.BackupExt(),
		S3StorageID: d.BackupSchedule.S3StorageID,
		StartedAt:   now,
	}, nil
}

// Succeed records a written Backup of size bytes, in S3 too when s3.
func (b *Backup) Succeed(size int64, s3 bool, at time.Time) error {
	if b.Status != BackupRunning {
		return ErrBackupFinished
	}
	b.Status, b.SizeBytes, b.Local, b.S3, b.FinishedAt = BackupSucceeded, size, true, s3, at.UTC()
	if !s3 {
		b.S3StorageID = 0
	}
	return nil
}

// Fail records why the Backup did not succeed. local is whether a file was
// still written (an upload failed after the dump).
func (b *Backup) Fail(reason string, local bool, size int64, at time.Time) error {
	if b.Status != BackupRunning {
		return ErrBackupFinished
	}
	b.Status, b.Error, b.Local, b.SizeBytes, b.S3, b.FinishedAt = BackupFailed, reason, local, size, false, at.UTC()
	return nil
}

// Restorable reports whether the Backup can be restored from somewhere.
func (b Backup) Restorable() bool {
	return b.Status == BackupSucceeded && (b.Local || b.S3)
}

// Prune returns the Backups Retention no longer keeps: newest first, once
// retention succeeded Backups are counted, every finished Backup older
// than the last of them. Running Backups are never pruned; failed ones
// newer than the oldest kept Backup stay, so a failure is seen.
func Prune(backups []Backup, retention int) []Backup {
	sorted := slices.Clone(backups)
	slices.SortStableFunc(sorted, func(a, b Backup) int { return b.StartedAt.Compare(a.StartedAt) })
	var out []Backup
	kept := 0
	for _, b := range sorted {
		switch {
		case b.Status == BackupRunning:
		case kept >= retention:
			out = append(out, b)
		case b.Status == BackupSucceeded:
			kept++
		}
	}
	return out
}
