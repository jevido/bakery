package domain

import (
	"errors"
	"slices"
	"time"
)

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
	// ScheduledBackupID is the Scheduled backup it was made for, whose
	// Retention prunes it.
	ScheduledBackupID uint64
	Status            ExecutionStatus
	Trigger           ExecutionTrigger
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

// NewBackupExecution starts a Backup execution of d for its Scheduled backup
// s at now, with the file name it will be written to and s's S3 storage.
func NewBackupExecution(d Database, s ScheduledBackup, trigger ExecutionTrigger, now time.Time) (BackupExecution, error) {
	if !d.Type.Spec().Backups {
		return BackupExecution{}, ErrNoBackups
	}
	if s.DatabaseID != d.ID {
		return BackupExecution{}, errors.New("the scheduled backup belongs to another database")
	}
	// Whole seconds: that is what the store keeps.
	now = now.UTC().Truncate(time.Second)
	return BackupExecution{
		DatabaseID: d.ID, ScheduledBackupID: s.ID, Status: ExecutionRunning, Trigger: trigger,
		FileName:    now.Format("20060102T150405Z") + d.BackupExt(),
		S3StorageID: s.S3StorageID,
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
