package app

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

var (
	// ErrBusy is returned while a Backup execution or Restore of the Database runs.
	ErrBusy = errors.New("a backup or restore of this database is already running")
	// ErrNotRunning is returned for a Database that is not running.
	ErrNotRunning = errors.New("the database is not running: start it first")
	// ErrBackupRunning is returned for a Backup execution that has not finished.
	ErrBackupRunning = errors.New("the backup is still running")
	// ErrNotRestorable is returned for a Backup execution that failed or whose file
	// is gone.
	ErrNotRestorable = errors.New("only a succeeded backup whose file still exists can be restored")
)

// BackupFiles keeps Backup execution files in the Backups directory, one directory
// per Database.
type BackupFiles interface {
	// Write creates the file through write; a failed write leaves no file.
	Write(databaseID uint64, name string, write func(w io.Writer) error) (size int64, err error)
	// Open opens a Backup execution file; an error satisfying errors.Is(err,
	// fs.ErrNotExist) means it is gone.
	Open(databaseID uint64, name string) (BackupFile, error)
	// Remove removes one file; a missing file is not an error.
	Remove(databaseID uint64, name string) error
	// RemoveAll removes the Database's directory.
	RemoveAll(databaseID uint64) error
}

// BackupFile is an open Backup execution file.
type BackupFile interface {
	io.Reader
	io.ReaderAt
	io.Closer
	Size() int64
}

// S3Client talks to one S3 storage.
type S3Client interface {
	Check(ctx context.Context) error
	Put(ctx context.Context, key string, r io.ReaderAt, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
	Delete(ctx context.Context, key string) error
}

// backupTimeout bounds one Backup execution or Restore.
const backupTimeout = 2 * time.Hour

// job is a running Backup execution or Restore of one Database.
type job struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// claim reserves the Database for one Backup execution or Restore.
func (s *Service) claim(id uint64) (*job, context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobs[id] != nil {
		return nil, nil, ErrBusy
	}
	ctx, cancel := context.WithTimeout(context.Background(), backupTimeout)
	j := &job{cancel: cancel, done: make(chan struct{})}
	s.jobs[id] = j
	return j, ctx, nil
}

func (s *Service) release(id uint64, j *job) {
	s.mu.Lock()
	if s.jobs[id] == j {
		delete(s.jobs, id)
	}
	s.mu.Unlock()
	j.cancel()
	close(j.done)
}

// endJob cancels whatever Backup execution or Restore runs for the Database and
// waits for it to end.
func (s *Service) endJob(id uint64) {
	s.mu.Lock()
	j := s.jobs[id]
	s.mu.Unlock()
	if j != nil {
		j.cancel()
		<-j.done
	}
}

func (s *Service) busy(id uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id] != nil
}

func (s *Service) logf(format string, args ...any) {
	if s.Log != nil {
		s.Log(format, args...)
	}
}

// checkRunning is what BackUp and Restore need of the Database: a Database type
// with backups, running.
func (s *Service) checkRunning(ctx context.Context, d domain.Database) error {
	if !d.Type.Spec().Backups {
		return domain.ErrNoBackups
	}
	status, _, err := s.runtime.Status(ctx, d)
	if err != nil {
		return err
	}
	if status != domain.StatusRunning {
		return ErrNotRunning
	}
	return nil
}

// BackUp starts a Backup execution for the Scheduled backup and returns it
// while it runs.
func (s *Service) BackUp(ctx context.Context, scheduledBackupID uint64, trigger domain.ExecutionTrigger) (domain.BackupExecution, error) {
	sb, d, err := s.scheduledBackup(ctx, scheduledBackupID)
	if err != nil {
		return domain.BackupExecution{}, err
	}
	return s.backUp(ctx, d, sb, trigger)
}

// BackUpDatabase starts a Backup execution for the Database's oldest
// Scheduled backup, making the default one if it has none.
func (s *Service) BackUpDatabase(ctx context.Context, databaseID uint64, trigger domain.ExecutionTrigger) (domain.BackupExecution, error) {
	d, err := s.get(ctx, databaseID)
	if err != nil {
		return domain.BackupExecution{}, err
	}
	sb, err := s.oldestScheduledBackup(ctx, d)
	if err != nil {
		return domain.BackupExecution{}, err
	}
	return s.backUp(ctx, d, sb, trigger)
}

func (s *Service) backUp(ctx context.Context, d domain.Database, sb domain.ScheduledBackup, trigger domain.ExecutionTrigger) (domain.BackupExecution, error) {
	if err := s.checkRunning(ctx, d); err != nil {
		return domain.BackupExecution{}, err
	}
	j, jctx, err := s.claim(d.ID)
	if err != nil {
		return domain.BackupExecution{}, err
	}
	b, err := domain.NewBackupExecution(d, sb, trigger, s.Now())
	if err == nil {
		b, err = s.store.CreateBackupExecution(ctx, b)
	}
	if err != nil {
		s.release(d.ID, j)
		return domain.BackupExecution{}, err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(d.ID, j)
		s.runBackup(jctx, d, sb, b)
	}()
	return b, nil
}

// runBackup dumps, uploads and prunes. Its outcome is stored on the Backup execution.
func (s *Service) runBackup(ctx context.Context, d domain.Database, sb domain.ScheduledBackup, b domain.BackupExecution) {
	finish := func(err error, local bool, size int64, s3 bool) {
		if err != nil {
			b.Fail(err.Error(), local, size, s.Now())
		} else {
			b.Succeed(size, s3, s.Now())
		}
		// The job's context may be cancelled; the outcome is still stored.
		sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if serr := s.store.SaveBackupExecution(sctx, b); serr != nil {
			s.logf("databases: saving backup %d: %v", b.ID, serr)
		}
		if s.BackupExecutionFinished != nil {
			s.BackupExecutionFinished(sctx, d, b)
		}
		if err != nil {
			s.logf("databases: backup %d of database %d: %v", b.ID, d.ID, err)
		}
	}

	size, err := s.Files.Write(d.ID, b.FileName, func(w io.Writer) error {
		out := w
		var gz *gzip.Writer
		if d.DumpGzip() {
			gz = gzip.NewWriter(w)
			out = gz
		}
		code, stderr, err := s.runtime.Dump(ctx, d, out)
		switch {
		case ctx.Err() != nil:
			return fmt.Errorf("the dump did not finish: %w", context.Cause(ctx))
		case err != nil:
			return fmt.Errorf("dumping: %w", err)
		case code != 0:
			return fmt.Errorf("the dump exited with code %d: %s", code, stderr)
		}
		if gz != nil {
			return gz.Close()
		}
		return nil
	})
	if err != nil {
		finish(err, false, 0, false)
		return
	}

	uploaded := false
	if b.S3StorageID != 0 {
		if err := s.upload(ctx, d, b, size); err != nil {
			finish(err, true, size, false)
			s.prune(d, sb)
			return
		}
		uploaded = true
	}
	finish(nil, true, size, uploaded)
	s.prune(d, sb)
}

func (s *Service) upload(ctx context.Context, d domain.Database, b domain.BackupExecution, size int64) error {
	st, found, err := s.store.S3Storage(ctx, b.S3StorageID)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("upload failed: the S3 storage no longer exists; the backup is on local disk only")
	}
	f, err := s.Files.Open(d.ID, b.FileName)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := s.S3(st).Put(ctx, st.Key(d.Slug, d.ID, b.FileName), f, size); err != nil {
		return fmt.Errorf("upload to %s failed, the backup is on local disk only: %w", st.Name, err)
	}
	return nil
}

// prune deletes the Scheduled backup's Backup executions its Retention no
// longer keeps. Failures are logged; they never fail a Backup execution.
func (s *Service) prune(d domain.Database, sb domain.ScheduledBackup) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// The Owner may have changed the Retention while the dump ran.
	if cur, found, err := s.store.ScheduledBackup(ctx, sb.ID); err == nil && found {
		sb = cur
	}
	backups, err := s.store.ScheduledBackupExecutions(ctx, sb.ID)
	if err != nil {
		s.logf("databases: pruning backups of scheduled backup %d: %v", sb.ID, err)
		return
	}
	for _, b := range domain.Prune(backups, sb.Retention) {
		if err := s.removeExecution(ctx, d, b, Copies{Local: true, S3: true}); err != nil {
			s.logf("databases: pruning backup %d: %v", b.ID, err)
		}
	}
}

// Copies says which copies of a Backup execution a delete removes with
// its row, as Coolify's delete asks: the file on this server, the object in
// its S3 storage, or both. A copy left behind is no longer listed.
type Copies struct {
	Local bool
	S3    bool
}

// removeExecution removes the Backup execution's row and the copies files names.
func (s *Service) removeExecution(ctx context.Context, d domain.Database, b domain.BackupExecution, files Copies) error {
	if b.Local && files.Local {
		if err := s.Files.Remove(d.ID, b.FileName); err != nil {
			return err
		}
	}
	if b.S3 && b.S3StorageID != 0 && files.S3 {
		st, found, err := s.store.S3Storage(ctx, b.S3StorageID)
		if err != nil {
			return err
		}
		if found {
			if err := s.S3(st).Delete(ctx, st.Key(d.Slug, d.ID, b.FileName)); err != nil {
				return err
			}
		}
	}
	return s.store.DeleteBackupExecution(ctx, b.ID)
}

// BackupExecutions lists the Database's Backup executions, newest first.
func (s *Service) BackupExecutions(ctx context.Context, databaseID uint64) ([]domain.BackupExecution, error) {
	if _, err := s.get(ctx, databaseID); err != nil {
		return nil, err
	}
	return s.store.BackupExecutions(ctx, databaseID)
}

func (s *Service) execution(ctx context.Context, id uint64) (domain.BackupExecution, domain.Database, error) {
	b, found, err := s.store.BackupExecution(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	if err != nil {
		return b, domain.Database{}, err
	}
	d, err := s.get(ctx, b.DatabaseID)
	return b, d, err
}

// DeleteBackupExecution removes a finished Backup execution with its file
// and, when deleteS3, its S3 object.
func (s *Service) DeleteBackupExecution(ctx context.Context, id uint64, deleteS3 bool) error {
	b, d, err := s.execution(ctx, id)
	if err != nil {
		return err
	}
	if b.Status == domain.ExecutionRunning {
		return ErrBackupRunning
	}
	return s.removeExecution(ctx, d, b, Copies{Local: true, S3: deleteS3})
}

// OpenBackupExecution opens the Backup execution's file, from the Backups directory or, when
// it is gone there, from its S3 storage. It returns the file name too.
func (s *Service) OpenBackupExecution(ctx context.Context, id uint64) (io.ReadCloser, int64, string, error) {
	b, d, err := s.execution(ctx, id)
	if err != nil {
		return nil, 0, "", err
	}
	r, size, err := s.openExecution(ctx, d, b)
	return r, size, b.FileName, err
}

func (s *Service) openExecution(ctx context.Context, d domain.Database, b domain.BackupExecution) (io.ReadCloser, int64, error) {
	if !b.Restorable() {
		return nil, 0, ErrNotRestorable
	}
	if b.Local {
		f, err := s.Files.Open(d.ID, b.FileName)
		if err == nil {
			return f, f.Size(), nil
		}
		if !isNotExist(err) {
			return nil, 0, err
		}
	}
	if b.S3 && b.S3StorageID != 0 {
		st, found, err := s.store.S3Storage(ctx, b.S3StorageID)
		if err != nil {
			return nil, 0, err
		}
		if found {
			return s.S3(st).Get(ctx, st.Key(d.Slug, d.ID, b.FileName))
		}
	}
	return nil, 0, ErrNotRestorable
}

func isNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// RestoreOutcome is how the last Restore of a Database went. It is kept in
// memory only: an API restart forgets the note, never data.
type RestoreOutcome struct {
	BackupExecutionID uint64
	StartedAt         time.Time
	FinishedAt        time.Time
	Error             string
}

// Restore replaces the running Database's data with the Backup execution, in the
// background.
func (s *Service) Restore(ctx context.Context, backupID uint64) error {
	b, d, err := s.execution(ctx, backupID)
	if err != nil {
		return err
	}
	if !b.Restorable() {
		return ErrNotRestorable
	}
	if err := s.checkRunning(ctx, d); err != nil {
		return err
	}
	j, jctx, err := s.claim(d.ID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.restoring[d.ID] = true
	s.mu.Unlock()
	started := s.Now().UTC()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(d.ID, j)
		err := s.runRestore(jctx, d, b)
		out := RestoreOutcome{BackupExecutionID: b.ID, StartedAt: started, FinishedAt: s.Now().UTC()}
		if err != nil {
			out.Error = err.Error()
			s.logf("databases: restoring backup %d into database %d: %v", b.ID, d.ID, err)
		}
		s.mu.Lock()
		delete(s.restoring, d.ID)
		s.restores[d.ID] = out
		s.mu.Unlock()
	}()
	return nil
}

func (s *Service) runRestore(ctx context.Context, d domain.Database, b domain.BackupExecution) error {
	r, size, err := s.openExecution(ctx, d, b)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := s.runtime.CopyIn(ctx, d, d.RestoreFile(), size, r); err != nil {
		return fmt.Errorf("copying the backup into the database's container: %w", err)
	}
	code, out, err := s.runtime.Exec(ctx, d, d.RestoreCommand())
	switch {
	case err != nil:
		return fmt.Errorf("restoring: %w", err)
	case code != 0:
		return fmt.Errorf("the restore exited with code %d: %s", code, out)
	}
	return nil
}

// restoreState is whether a Restore of the Database runs, and how the last
// one went.
func (s *Service) restoreState(id uint64) (bool, *RestoreOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, ok := s.restores[id]
	if !ok {
		return s.restoring[id], nil
	}
	return s.restoring[id], &out
}

// ScheduledBackupView is a Scheduled backup with when it fires next (zero
// when off).
type ScheduledBackupView struct {
	domain.ScheduledBackup
	NextBackupAt time.Time
}

func (s *Service) scheduledBackup(ctx context.Context, id uint64) (domain.ScheduledBackup, domain.Database, error) {
	sb, found, err := s.store.ScheduledBackup(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	if err != nil {
		return sb, domain.Database{}, err
	}
	d, err := s.get(ctx, sb.DatabaseID)
	return sb, d, err
}

func (s *Service) scheduledBackupView(ctx context.Context, sb domain.ScheduledBackup) (ScheduledBackupView, error) {
	next, err := s.nextBackup(ctx, sb)
	return ScheduledBackupView{ScheduledBackup: sb, NextBackupAt: next}, err
}

// oldestScheduledBackup is the Database's first Scheduled backup; one with
// the defaults is made when it has none.
func (s *Service) oldestScheduledBackup(ctx context.Context, d domain.Database) (domain.ScheduledBackup, error) {
	if !d.Type.Spec().Backups {
		return domain.ScheduledBackup{}, domain.ErrNoBackups
	}
	list, err := s.store.ScheduledBackups(ctx, d.ID)
	if err != nil {
		return domain.ScheduledBackup{}, err
	}
	if len(list) > 0 {
		return list[0], nil
	}
	sb, err := domain.NewScheduledBackup(d, domain.DefaultScheduledBackup, s.Now())
	if err != nil {
		return sb, err
	}
	return s.store.CreateScheduledBackup(ctx, sb)
}

func (s *Service) checkS3Storage(ctx context.Context, id uint64) error {
	if id == 0 {
		return nil
	}
	_, found, err := s.store.S3Storage(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return &domain.FieldError{Field: "scheduled_backup.s3_storage_id", Message: "that S3 storage does not exist"}
	}
	return nil
}

// ScheduledBackups lists the Database's Scheduled backups, oldest first.
func (s *Service) ScheduledBackups(ctx context.Context, databaseID uint64) ([]ScheduledBackupView, error) {
	if _, err := s.get(ctx, databaseID); err != nil {
		return nil, err
	}
	list, err := s.store.ScheduledBackups(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	out := make([]ScheduledBackupView, len(list))
	for i, sb := range list {
		if out[i], err = s.scheduledBackupView(ctx, sb); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ScheduledBackup returns one Scheduled backup.
func (s *Service) ScheduledBackup(ctx context.Context, id uint64) (ScheduledBackupView, error) {
	sb, _, err := s.scheduledBackup(ctx, id)
	if err != nil {
		return ScheduledBackupView{}, err
	}
	return s.scheduledBackupView(ctx, sb)
}

// CreateScheduledBackup adds a Scheduled backup to the Database.
func (s *Service) CreateScheduledBackup(ctx context.Context, databaseID uint64, in domain.ScheduledBackupInput) (ScheduledBackupView, error) {
	d, err := s.get(ctx, databaseID)
	if err != nil {
		return ScheduledBackupView{}, err
	}
	sb, err := domain.NewScheduledBackup(d, in, s.Now())
	if err != nil {
		return ScheduledBackupView{}, err
	}
	if err := s.checkS3Storage(ctx, sb.S3StorageID); err != nil {
		return ScheduledBackupView{}, err
	}
	if sb, err = s.store.CreateScheduledBackup(ctx, sb); err != nil {
		return ScheduledBackupView{}, err
	}
	return s.scheduledBackupView(ctx, sb)
}

// UpdateScheduledBackup replaces what the Owner sets on a Scheduled backup.
func (s *Service) UpdateScheduledBackup(ctx context.Context, id uint64, in domain.ScheduledBackupInput) (ScheduledBackupView, error) {
	sb, _, err := s.scheduledBackup(ctx, id)
	if err != nil {
		return ScheduledBackupView{}, err
	}
	if err := sb.Update(in, s.Now()); err != nil {
		return ScheduledBackupView{}, err
	}
	if err := s.checkS3Storage(ctx, sb.S3StorageID); err != nil {
		return ScheduledBackupView{}, err
	}
	if err := s.store.SaveScheduledBackup(ctx, sb); err != nil {
		return ScheduledBackupView{}, err
	}
	return s.scheduledBackupView(ctx, sb)
}

// DeleteScheduledBackup removes a Scheduled backup with its Backup
// executions' rows and the copies files names. Not while one of them runs.
func (s *Service) DeleteScheduledBackup(ctx context.Context, id uint64, files Copies) error {
	sb, d, err := s.scheduledBackup(ctx, id)
	if err != nil {
		return err
	}
	list, err := s.store.ScheduledBackupExecutions(ctx, sb.ID)
	if err != nil {
		return err
	}
	for _, b := range list {
		if b.Status == domain.ExecutionRunning {
			return ErrBackupRunning
		}
	}
	for _, b := range list {
		if err := s.removeExecution(ctx, d, b, files); err != nil {
			return err
		}
	}
	return s.store.DeleteScheduledBackup(ctx, sb.ID)
}

// ScheduledBackupExecutions lists the Scheduled backup's Backup executions,
// newest first.
func (s *Service) ScheduledBackupExecutions(ctx context.Context, id uint64) ([]domain.BackupExecution, error) {
	if _, _, err := s.scheduledBackup(ctx, id); err != nil {
		return nil, err
	}
	return s.store.ScheduledBackupExecutions(ctx, id)
}

// nextBackup is when the Scheduled backup fires next; zero when off.
func (s *Service) nextBackup(ctx context.Context, sb domain.ScheduledBackup) (time.Time, error) {
	if !sb.Enabled {
		return time.Time{}, nil
	}
	last, err := s.store.LastScheduledStart(ctx, sb.ID)
	if err != nil {
		return time.Time{}, err
	}
	return sb.NextAt(last), nil
}

// Tick starts a Backup execution for every Scheduled backup that is due at
// now. One that finds its Database busy (another of its Scheduled backups
// due at the same time, a Restore) stays due and starts on a later Tick. A
// due Backup execution that cannot start for another reason (the Database
// is stopped) is recorded as a failed scheduled Backup execution, so the
// Owner sees why a run was missed, and the schedule waits for its next time.
func (s *Service) Tick(ctx context.Context, now time.Time) error {
	list, err := s.store.EnabledScheduledBackups(ctx)
	if err != nil {
		return err
	}
	for _, sb := range list {
		last, err := s.store.LastScheduledStart(ctx, sb.ID)
		if err != nil {
			return err
		}
		if !sb.Due(last, now) {
			continue
		}
		d, err := s.get(ctx, sb.DatabaseID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = s.backUp(ctx, d, sb, domain.TriggerScheduled)
		if errors.Is(err, ErrBusy) {
			continue
		}
		if err != nil {
			s.logf("databases: scheduled backup %d of database %d: %v", sb.ID, d.ID, err)
			b, nerr := domain.NewBackupExecution(d, sb, domain.TriggerScheduled, now)
			if nerr != nil {
				continue
			}
			b.Fail(err.Error(), false, 0, now)
			if _, cerr := s.store.CreateBackupExecution(ctx, b); cerr != nil {
				return cerr
			}
		}
	}
	return nil
}
