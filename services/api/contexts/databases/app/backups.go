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
	// ErrBusy is returned while a Backup or Restore of the Database runs.
	ErrBusy = errors.New("a backup or restore of this database is already running")
	// ErrNotRunning is returned for a Database that is not running.
	ErrNotRunning = errors.New("the database is not running: start it first")
	// ErrBackupRunning is returned for a Backup that has not finished.
	ErrBackupRunning = errors.New("the backup is still running")
	// ErrNotRestorable is returned for a Backup that failed or whose file
	// is gone.
	ErrNotRestorable = errors.New("only a succeeded backup whose file still exists can be restored")
)

// BackupFiles keeps Backup files in the Backups directory, one directory
// per Database.
type BackupFiles interface {
	// Write creates the file through write; a failed write leaves no file.
	Write(databaseID uint64, name string, write func(w io.Writer) error) (size int64, err error)
	// Open opens a Backup file; an error satisfying errors.Is(err,
	// fs.ErrNotExist) means it is gone.
	Open(databaseID uint64, name string) (BackupFile, error)
	// Remove removes one file; a missing file is not an error.
	Remove(databaseID uint64, name string) error
	// RemoveAll removes the Database's directory.
	RemoveAll(databaseID uint64) error
}

// BackupFile is an open Backup file.
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

// backupTimeout bounds one Backup or Restore.
const backupTimeout = 2 * time.Hour

// job is a running Backup or Restore of one Database.
type job struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// claim reserves the Database for one Backup or Restore.
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

// endJob cancels whatever Backup or Restore runs for the Database and
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

// checkRunning is what BackUp and Restore need of the Database: an Engine
// with backups, running.
func (s *Service) checkRunning(ctx context.Context, d domain.Database) error {
	if !d.Engine.Spec().Backups {
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

// BackUp starts a Backup of the Database and returns it while it runs.
func (s *Service) BackUp(ctx context.Context, id uint64, trigger domain.BackupTrigger) (domain.Backup, error) {
	d, err := s.get(ctx, id)
	if err != nil {
		return domain.Backup{}, err
	}
	if err := s.checkRunning(ctx, d); err != nil {
		return domain.Backup{}, err
	}
	j, jctx, err := s.claim(id)
	if err != nil {
		return domain.Backup{}, err
	}
	b, err := domain.NewBackup(d, trigger, s.Now())
	if err == nil {
		b, err = s.store.CreateBackup(ctx, b)
	}
	if err != nil {
		s.release(id, j)
		return domain.Backup{}, err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(id, j)
		s.runBackup(jctx, d, b)
	}()
	return b, nil
}

// runBackup dumps, uploads and prunes. Its outcome is stored on the Backup.
func (s *Service) runBackup(ctx context.Context, d domain.Database, b domain.Backup) {
	finish := func(err error, local bool, size int64, s3 bool) {
		if err != nil {
			b.Fail(err.Error(), local, size, s.Now())
		} else {
			b.Succeed(size, s3, s.Now())
		}
		// The job's context may be cancelled; the outcome is still stored.
		sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if serr := s.store.SaveBackup(sctx, b); serr != nil {
			s.logf("databases: saving backup %d: %v", b.ID, serr)
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
			s.prune(d)
			return
		}
		uploaded = true
	}
	finish(nil, true, size, uploaded)
	s.prune(d)
}

func (s *Service) upload(ctx context.Context, d domain.Database, b domain.Backup, size int64) error {
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

// prune deletes the Backups Retention no longer keeps. Failures are
// logged; they never fail a Backup.
func (s *Service) prune(d domain.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	backups, err := s.store.Backups(ctx, d.ID)
	if err != nil {
		s.logf("databases: pruning backups of database %d: %v", d.ID, err)
		return
	}
	for _, b := range domain.Prune(backups, d.BackupSchedule.Retention) {
		if err := s.removeBackup(ctx, d, b); err != nil {
			s.logf("databases: pruning backup %d: %v", b.ID, err)
		}
	}
}

// removeBackup removes the Backup's file, its S3 object and its row.
func (s *Service) removeBackup(ctx context.Context, d domain.Database, b domain.Backup) error {
	if b.Local {
		if err := s.Files.Remove(d.ID, b.FileName); err != nil {
			return err
		}
	}
	if b.S3 && b.S3StorageID != 0 {
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
	return s.store.DeleteBackup(ctx, b.ID)
}

// Backups lists the Database's Backups, newest first.
func (s *Service) Backups(ctx context.Context, databaseID uint64) ([]domain.Backup, error) {
	if _, err := s.get(ctx, databaseID); err != nil {
		return nil, err
	}
	return s.store.Backups(ctx, databaseID)
}

func (s *Service) backup(ctx context.Context, id uint64) (domain.Backup, domain.Database, error) {
	b, found, err := s.store.Backup(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	if err != nil {
		return b, domain.Database{}, err
	}
	d, err := s.get(ctx, b.DatabaseID)
	return b, d, err
}

// DeleteBackup removes a finished Backup, its file and its S3 object.
func (s *Service) DeleteBackup(ctx context.Context, id uint64) error {
	b, d, err := s.backup(ctx, id)
	if err != nil {
		return err
	}
	if b.Status == domain.BackupRunning {
		return ErrBackupRunning
	}
	return s.removeBackup(ctx, d, b)
}

// OpenBackup opens the Backup's file, from the Backups directory or, when
// it is gone there, from its S3 storage. It returns the file name too.
func (s *Service) OpenBackup(ctx context.Context, id uint64) (io.ReadCloser, int64, string, error) {
	b, d, err := s.backup(ctx, id)
	if err != nil {
		return nil, 0, "", err
	}
	r, size, err := s.openBackup(ctx, d, b)
	return r, size, b.FileName, err
}

func (s *Service) openBackup(ctx context.Context, d domain.Database, b domain.Backup) (io.ReadCloser, int64, error) {
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
	BackupID   uint64
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
}

// Restore replaces the running Database's data with the Backup, in the
// background.
func (s *Service) Restore(ctx context.Context, backupID uint64) error {
	b, d, err := s.backup(ctx, backupID)
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
	started := s.Now()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(d.ID, j)
		err := s.runRestore(jctx, d, b)
		out := RestoreOutcome{BackupID: b.ID, StartedAt: started, FinishedAt: s.Now()}
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

func (s *Service) runRestore(ctx context.Context, d domain.Database, b domain.Backup) error {
	r, size, err := s.openBackup(ctx, d, b)
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

// SetBackupSchedule replaces the Database's Backup schedule.
func (s *Service) SetBackupSchedule(ctx context.Context, id uint64, sched domain.BackupSchedule) (View, error) {
	d, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if sched.S3StorageID != 0 {
		_, found, err := s.store.S3Storage(ctx, sched.S3StorageID)
		if err != nil {
			return View{}, err
		}
		if !found {
			return View{}, &domain.FieldError{Field: "backup_schedule.s3_storage_id", Message: "that S3 storage does not exist"}
		}
	}
	if err := d.SetBackupSchedule(sched, s.Now()); err != nil {
		return View{}, err
	}
	if err := s.store.Update(ctx, d); err != nil {
		return View{}, err
	}
	return s.view(ctx, d)
}

// nextBackup is when the Database's schedule fires next; zero when off.
func (s *Service) nextBackup(ctx context.Context, d domain.Database) (time.Time, error) {
	sched := d.BackupSchedule
	if !sched.Enabled {
		return time.Time{}, nil
	}
	last, err := s.store.LastScheduledStart(ctx, d.ID)
	if err != nil {
		return time.Time{}, err
	}
	from := sched.EnabledAt
	if last.After(from) {
		from = last
	}
	return sched.Next(from), nil
}

// Tick starts a Backup of every Database whose schedule is due at now. A
// due Backup that cannot start (the Database is stopped, or busy) is
// recorded as a failed scheduled Backup, so the Owner sees why a run was
// missed, and the schedule waits for its next time.
func (s *Service) Tick(ctx context.Context, now time.Time) error {
	list, err := s.store.ScheduledDatabases(ctx)
	if err != nil {
		return err
	}
	for _, d := range list {
		last, err := s.store.LastScheduledStart(ctx, d.ID)
		if err != nil {
			return err
		}
		if !d.BackupSchedule.Due(last, now) {
			continue
		}
		if _, err := s.BackUp(ctx, d.ID, domain.TriggerScheduled); err != nil {
			s.logf("databases: scheduled backup of database %d: %v", d.ID, err)
			b, nerr := domain.NewBackup(d, domain.TriggerScheduled, now)
			if nerr != nil {
				continue
			}
			b.Fail(err.Error(), false, 0, now)
			if _, cerr := s.store.CreateBackup(ctx, b); cerr != nil {
				return cerr
			}
		}
	}
	return nil
}
