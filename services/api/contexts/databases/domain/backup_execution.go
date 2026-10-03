package domain

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// ScheduledBackup is when a Database is backed up by itself. Times are UTC.
type ScheduledBackup struct {
	Enabled bool
	// Cron is a five-field cron expression.
	Cron      string
	Retention int
	// S3StorageID is the S3 storage Backup executions are uploaded to; 0 is local
	// disk only.
	S3StorageID uint64
	// EnabledAt is when the schedule was last switched on; it first fires
	// at the next time after it.
	EnabledAt time.Time
}

// DefaultScheduledBackup is what a new Database starts with: off, daily at
// 03:00 UTC, keeping 7.
var DefaultScheduledBackup = ScheduledBackup{Cron: "0 3 * * *", Retention: 7}

func parseCron(expr string) (cron.Schedule, error) {
	// ParseStandard also takes descriptors (@daily) and a TZ= prefix; only
	// plain five-field expressions are allowed, so times stay UTC.
	if len(strings.Fields(expr)) != 5 {
		return nil, errors.New("five fields expected")
	}
	return cron.ParseStandard(expr)
}

// Check validates the cron expression and the Retention.
func (s ScheduledBackup) Check() error {
	if _, err := parseCron(s.Cron); err != nil {
		return invalid("scheduled_backup.cron", "%q is not a five-field cron expression (minute hour day month weekday)", s.Cron)
	}
	if s.Retention < 1 || s.Retention > 100 {
		return invalid("scheduled_backup.retention", "keep between 1 and 100 backups")
	}
	return nil
}

// Next is the first scheduled time after after, in UTC; the zero time for
// an invalid expression.
func (s ScheduledBackup) Next(after time.Time) time.Time {
	c, err := parseCron(s.Cron)
	if err != nil {
		return time.Time{}
	}
	return c.Next(after.UTC())
}

// Due reports whether a scheduled Backup execution should start now: the schedule is
// on and its next time after the last scheduled Backup execution (or after it was
// switched on, whichever is later) has come. However many times were
// missed, one Backup execution catches up.
func (s ScheduledBackup) Due(lastScheduledStart, now time.Time) bool {
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

// ErrNoBackups is returned for a Database type that is not backed up.
var ErrNoBackups = errors.New("backups are not available for this database type: Redis and Valkey keep their data in an append-only file")

// SetScheduledBackup replaces the Scheduled backup. Switching it on at now
// remembers now, so it does not fire for a time that already passed.
func (d *Database) SetScheduledBackup(s ScheduledBackup, now time.Time) error {
	s.Cron = strings.TrimSpace(s.Cron)
	if err := s.Check(); err != nil {
		return err
	}
	if s.Enabled && !d.Type.Spec().Backups {
		return ErrNoBackups
	}
	switch {
	case s.Enabled && !d.ScheduledBackup.Enabled:
		s.EnabledAt = now.UTC()
	case s.Enabled:
		s.EnabledAt = d.ScheduledBackup.EnabledAt
	}
	d.ScheduledBackup = s
	return nil
}

// ExecutionStatus is where a Backup execution is.
type ExecutionStatus string

const (
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
)

// ExecutionTrigger is what started a Backup execution.
type ExecutionTrigger string

const (
	TriggerManual    ExecutionTrigger = "manual"
	TriggerScheduled ExecutionTrigger = "scheduled"
)

// BackupExecution is one dump of a Database.
type BackupExecution struct {
	ID         uint64
	DatabaseID uint64
	Status     ExecutionStatus
	Trigger    ExecutionTrigger
	// FileName is relative to the Database's directory in the Backup executions
	// directory, and the last part of its S3 key.
	FileName  string
	SizeBytes int64
	// Local and S3 are where the Backup execution is.
	Local       bool
	S3          bool
	S3StorageID uint64
	Error       string
	StartedAt   time.Time
	FinishedAt  time.Time
}

// ErrExecutionFinished is returned when a finished Backup execution is finished again.
var ErrExecutionFinished = errors.New("backup already finished")

// NewBackupExecution starts a Backup execution of d at now, with the file name it will be
// written to.
func NewBackupExecution(d Database, trigger ExecutionTrigger, now time.Time) (BackupExecution, error) {
	if !d.Type.Spec().Backups {
		return BackupExecution{}, ErrNoBackups
	}
	// Whole seconds: that is what the store keeps.
	now = now.UTC().Truncate(time.Second)
	return BackupExecution{
		DatabaseID: d.ID, Status: ExecutionRunning, Trigger: trigger,
		FileName:    now.Format("20060102T150405Z") + d.BackupExt(),
		S3StorageID: d.ScheduledBackup.S3StorageID,
		StartedAt:   now,
	}, nil
}

// Succeed records a written Backup execution of size bytes, in S3 too when s3.
func (b *BackupExecution) Succeed(size int64, s3 bool, at time.Time) error {
	if b.Status != ExecutionRunning {
		return ErrExecutionFinished
	}
	b.Status, b.SizeBytes, b.Local, b.S3, b.FinishedAt = ExecutionSucceeded, size, true, s3, at.UTC().Truncate(time.Second)
	if !s3 {
		b.S3StorageID = 0
	}
	return nil
}

// Fail records why the Backup execution did not succeed. local is whether a file was
// still written (an upload failed after the dump).
func (b *BackupExecution) Fail(reason string, local bool, size int64, at time.Time) error {
	if b.Status != ExecutionRunning {
		return ErrExecutionFinished
	}
	b.Status, b.Error, b.Local, b.SizeBytes, b.S3, b.FinishedAt = ExecutionFailed, reason, local, size, false, at.UTC().Truncate(time.Second)
	return nil
}

// Restorable reports whether the Backup execution can be restored from somewhere.
func (b BackupExecution) Restorable() bool {
	return b.Status == ExecutionSucceeded && (b.Local || b.S3)
}

// Prune returns the Backup executions Retention no longer keeps: newest first, once
// retention succeeded Backup executions are counted, every finished Backup execution older
// than the last of them. Running Backup executions are never pruned; failed ones
// newer than the oldest kept Backup execution stay, so a failure is seen.
func Prune(backups []BackupExecution, retention int) []BackupExecution {
	sorted := slices.Clone(backups)
	slices.SortStableFunc(sorted, func(a, b BackupExecution) int { return b.StartedAt.Compare(a.StartedAt) })
	var out []BackupExecution
	kept := 0
	for _, b := range sorted {
		switch {
		case b.Status == ExecutionRunning:
		case kept >= retention:
			out = append(out, b)
		case b.Status == ExecutionSucceeded:
			kept++
		}
	}
	return out
}
