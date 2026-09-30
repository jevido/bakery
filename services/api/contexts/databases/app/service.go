// Package app holds the databases use cases.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

var ErrNotFound = errors.New("not found")

// Store keeps Databases; passwords are encrypted by the Store.
type Store interface {
	Create(ctx context.Context, d domain.Database) (domain.Database, error)
	Get(ctx context.Context, id uint64) (domain.Database, bool, error)
	ForProject(ctx context.Context, projectID uint64) ([]domain.Database, error)
	// Wanted lists the Databases whose desired state is running.
	Wanted(ctx context.Context) ([]domain.Database, error)
	Update(ctx context.Context, d domain.Database) error
	Delete(ctx context.Context, id uint64) error
	SlugTaken(ctx context.Context, slug string) (bool, error)
	PublicPortTaken(ctx context.Context, port int, exceptID uint64) (bool, error)
	CountForProject(ctx context.Context, projectID uint64) (int64, error)
	// ScheduledDatabases lists the Databases whose Backup schedule is on.
	ScheduledDatabases(ctx context.Context) ([]domain.Database, error)

	CreateBackup(ctx context.Context, b domain.Backup) (domain.Backup, error)
	SaveBackup(ctx context.Context, b domain.Backup) error
	Backup(ctx context.Context, id uint64) (domain.Backup, bool, error)
	// Backups lists a Database's Backups, newest first.
	Backups(ctx context.Context, databaseID uint64) ([]domain.Backup, error)
	LastScheduledStart(ctx context.Context, databaseID uint64) (time.Time, error)
	DeleteBackup(ctx context.Context, id uint64) error
	DeleteBackups(ctx context.Context, databaseID uint64) error
	FailRunningBackups(ctx context.Context, reason string, at time.Time) (int64, error)

	// S3 storages; the secret key is encrypted by the Store.
	S3Storages(ctx context.Context) ([]domain.S3Storage, error)
	S3Storage(ctx context.Context, id uint64) (domain.S3Storage, bool, error)
	S3StorageNameTaken(ctx context.Context, name string, exceptID uint64) (bool, error)
	S3StorageInUse(ctx context.Context, id uint64) (bool, error)
	CreateS3Storage(ctx context.Context, st domain.S3Storage) (domain.S3Storage, error)
	SaveS3Storage(ctx context.Context, st domain.S3Storage) error
	DeleteS3Storage(ctx context.Context, id uint64) error
}

// Runtime runs Database Containers. A Database that is stopped has no
// Container.
type Runtime interface {
	Start(ctx context.Context, d domain.Database) error
	Stop(ctx context.Context, d domain.Database) error
	Recreate(ctx context.Context, d domain.Database) error
	// Remove removes the Container and the volume.
	Remove(ctx context.Context, d domain.Database) error
	Status(ctx context.Context, d domain.Database) (domain.Status, string, error)
	Logs(ctx context.Context, d domain.Database, follow bool, tail int, out func(stream, line string)) (bool, error)
	// Dump runs the Database's DumpCommand in its Container, writing its
	// stdout to w; it returns the exit code and stderr.
	Dump(ctx context.Context, d domain.Database, w io.Writer) (int, string, error)
	// CopyIn streams a file of size bytes into the Container's RestoreDir.
	CopyIn(ctx context.Context, d domain.Database, name string, size int64, r io.Reader) error
	// Exec runs cmd in the Container: exit code and output.
	Exec(ctx context.Context, d domain.Database, cmd []string) (int, string, error)
}

// Environment is where a Database is placed, as projects told us.
type Environment struct {
	ID        uint64
	ProjectID uint64
}

// Environments looks up an Environment, returning ErrNotFound for an
// unknown one.
type Environments func(ctx context.Context, id uint64) (Environment, error)

// View is a Database as the Owner sees it: with its status and URLs.
type View struct {
	domain.Database
	Status domain.Status
	// Error is why the last start failed, or why the Container exited.
	Error       string
	InternalURL string
	PublicURL   string
	// NextBackupAt is when the Backup schedule fires next; zero when off.
	NextBackupAt time.Time
	Restoring    bool
	LastRestore  *RestoreOutcome
}

// opTimeout bounds one background start, pull included.
const opTimeout = 10 * time.Minute

// Service runs every change to a Container in the background, one at a
// time per Database, because a first start pulls an image and can take
// longer than a request. Stop and Delete cancel whatever is running first.
type Service struct {
	store        Store
	runtime      Runtime
	environments Environments
	publicHost   string
	NewPassword  func() string
	// Log reports background failures; nil discards them.
	Log func(format string, args ...any)
	// Files keeps Backup files; S3 makes a client for an S3 storage.
	Files BackupFiles
	S3    func(domain.S3Storage) S3Client
	// Now is the clock of Backups and schedules.
	Now func() time.Time

	mu      sync.Mutex
	locks   map[uint64]*sync.Mutex
	pending map[uint64]*op
	errs    map[uint64]string
	jobs    map[uint64]*job
	// restoring and restores: which Databases a Restore runs for, and how
	// the last one went.
	restoring map[uint64]bool
	restores  map[uint64]RestoreOutcome
	wg        sync.WaitGroup
}

func NewService(store Store, runtime Runtime, environments Environments, publicHost string) *Service {
	return &Service{
		store: store, runtime: runtime, environments: environments, publicHost: publicHost,
		NewPassword: newPassword, Now: time.Now,
		locks: map[uint64]*sync.Mutex{}, pending: map[uint64]*op{}, errs: map[uint64]string{}, jobs: map[uint64]*job{},
		restoring: map[uint64]bool{}, restores: map[uint64]RestoreOutcome{},
	}
}

const passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// newPassword is 32 random letters and digits: no character any Engine's
// URL or configuration treats specially.
func newPassword() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = passwordAlphabet[int(b[i])%len(passwordAlphabet)]
	}
	return string(b)
}

// op is one background change; only the latest one for a Database keeps
// its error.
type op struct{ cancel context.CancelFunc }

func (s *Service) lock(id uint64) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[id]
	if !ok {
		l = &sync.Mutex{}
		s.locks[id] = l
	}
	return l
}

// background runs f for the Database after whatever runs for it now, and
// keeps its error to show.
func (s *Service) background(d domain.Database, what string, f func(ctx context.Context, d domain.Database) error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	me := &op{cancel: cancel}
	s.mu.Lock()
	if prev := s.pending[d.ID]; prev != nil {
		prev.cancel()
	}
	s.pending[d.ID] = me
	delete(s.errs, d.ID)
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		l := s.lock(d.ID)
		l.Lock()
		err := ctx.Err()
		if err == nil {
			err = f(ctx, d)
		}
		l.Unlock()
		s.mu.Lock()
		current := s.pending[d.ID] == me
		if current {
			delete(s.pending, d.ID)
			if err != nil {
				s.errs[d.ID] = fmt.Sprintf("%s failed: %v", what, err)
			}
		}
		s.mu.Unlock()
		if current && err != nil && s.Log != nil {
			s.Log("databases: %s of database %d: %v", what, d.ID, err)
		}
	}()
}

// now cancels whatever runs for the Database and runs f in its place,
// synchronously.
func (s *Service) now(ctx context.Context, d domain.Database, f func(ctx context.Context, d domain.Database) error) error {
	s.mu.Lock()
	if prev := s.pending[d.ID]; prev != nil {
		prev.cancel()
		delete(s.pending, d.ID)
	}
	delete(s.errs, d.ID)
	s.mu.Unlock()
	l := s.lock(d.ID)
	l.Lock()
	defer l.Unlock()
	return f(ctx, d)
}

// Wait waits for every background change to finish (tests).
func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) get(ctx context.Context, id uint64) (domain.Database, error) {
	d, found, err := s.store.Get(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return d, err
}

func (s *Service) freeSlug(ctx context.Context, base string) (string, error) {
	for i := 1; i < 100; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		taken, err := s.store.SlugTaken(ctx, slug)
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
	}
	return "", &domain.FieldError{Field: "name", Message: "too many databases with this name"}
}

func (s *Service) checkPublicPort(ctx context.Context, port int, exceptID uint64) error {
	if port == 0 {
		return nil
	}
	taken, err := s.store.PublicPortTaken(ctx, port, exceptID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "public_port", Message: fmt.Sprintf("public port %d is used by another database", port)}
	}
	return nil
}

// Create stores a new Database in the Environment and starts it in the
// background.
func (s *Service) Create(ctx context.Context, environmentID uint64, in domain.Input) (View, error) {
	env, err := s.environments(ctx, environmentID)
	if err != nil {
		return View{}, err
	}
	slug, err := s.freeSlug(ctx, domain.Slugify(in.Name, in.Engine))
	if err != nil {
		return View{}, err
	}
	d, err := domain.NewDatabase(env.ID, env.ProjectID, in, slug, s.NewPassword)
	if err != nil {
		return View{}, err
	}
	if err := s.checkPublicPort(ctx, d.PublicPort, 0); err != nil {
		return View{}, err
	}
	if d, err = s.store.Create(ctx, d); err != nil {
		return View{}, err
	}
	s.background(d, "start", s.runtime.Start)
	return s.view(ctx, d)
}

// view reads the Database's status. A change still running in the
// background shows as starting.
func (s *Service) view(ctx context.Context, d domain.Database) (View, error) {
	status, detail, err := s.runtime.Status(ctx, d)
	if err != nil {
		return View{}, err
	}
	s.mu.Lock()
	pending, lastErr := s.pending[d.ID] != nil, s.errs[d.ID]
	s.mu.Unlock()
	if pending && status != domain.StatusRunning && d.DesiredState == domain.Running {
		status = domain.StatusStarting
	}
	if lastErr != "" {
		detail = lastErr
	}
	next, err := s.nextBackup(ctx, d)
	if err != nil {
		return View{}, err
	}
	restoring, lastRestore := s.restoreState(d.ID)
	return View{
		Database: d, Status: status, Error: detail, InternalURL: d.InternalURL(), PublicURL: d.PublicURL(s.publicHost),
		NextBackupAt: next, Restoring: restoring, LastRestore: lastRestore,
	}, nil
}

func (s *Service) Get(ctx context.Context, id uint64) (View, error) {
	d, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, d)
}

// ForProject lists the Project's Databases with their status.
func (s *Service) ForProject(ctx context.Context, projectID uint64) ([]View, error) {
	list, err := s.store.ForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]View, len(list))
	for i, d := range list {
		if out[i], err = s.view(ctx, d); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Update changes the name, version, Public port and Resource limits; a
// change the Container depends on recreates it if it should run.
func (s *Service) Update(ctx context.Context, id uint64, in domain.Input) (View, error) {
	d, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	recreate, err := d.Update(in)
	if err != nil {
		return View{}, err
	}
	if err := s.checkPublicPort(ctx, d.PublicPort, d.ID); err != nil {
		return View{}, err
	}
	if err := s.store.Update(ctx, d); err != nil {
		return View{}, err
	}
	if recreate && d.DesiredState == domain.Running {
		s.background(d, "restart", s.runtime.Recreate)
	}
	return s.view(ctx, d)
}

func (s *Service) setDesired(ctx context.Context, id uint64, state domain.DesiredState) (domain.Database, error) {
	d, err := s.get(ctx, id)
	if err != nil {
		return d, err
	}
	d.DesiredState = state
	return d, s.store.Update(ctx, d)
}

// Start (and Restart) make a fresh Container from the current settings in
// the background.
func (s *Service) Start(ctx context.Context, id uint64) (View, error) {
	d, err := s.setDesired(ctx, id, domain.Running)
	if err != nil {
		return View{}, err
	}
	s.background(d, "start", s.runtime.Recreate)
	return s.view(ctx, d)
}

func (s *Service) Restart(ctx context.Context, id uint64) (View, error) {
	return s.Start(ctx, id)
}

// Stop stops and removes the Container at once; the data stays.
func (s *Service) Stop(ctx context.Context, id uint64) (View, error) {
	d, err := s.setDesired(ctx, id, domain.Stopped)
	if err != nil {
		return View{}, err
	}
	if err := s.now(ctx, d, s.runtime.Stop); err != nil {
		return View{}, err
	}
	return s.view(ctx, d)
}

// Delete removes the Container, the volume with all data, the Database's
// Backup files and rows, and the Database. Its S3 objects stay.
func (s *Service) Delete(ctx context.Context, id uint64) error {
	d, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	s.endJob(id)
	if err := s.now(ctx, d, s.runtime.Remove); err != nil {
		return err
	}
	if s.Files != nil {
		if err := s.Files.RemoveAll(id); err != nil {
			return err
		}
	}
	if err := s.store.DeleteBackups(ctx, id); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.errs, id)
	delete(s.locks, id)
	delete(s.restores, id)
	s.mu.Unlock()
	return nil
}

// InUse reports whether the Project has Databases, for projects'
// OnProjectDeleting.
func (s *Service) InUse(ctx context.Context, projectID uint64) (bool, error) {
	n, err := s.store.CountForProject(ctx, projectID)
	return n > 0, err
}

// Recover marks Backups a stopped API left running as failed, and starts,
// in the background, every Database that should run but has no Container
// (removed by hand, or lost with an upgrade).
func (s *Service) Recover(ctx context.Context) error {
	if _, err := s.store.FailRunningBackups(ctx, "interrupted: the API stopped during the backup", s.Now()); err != nil {
		return err
	}
	list, err := s.store.Wanted(ctx)
	if err != nil {
		return err
	}
	for _, d := range list {
		status, _, err := s.runtime.Status(ctx, d)
		if err != nil {
			return err
		}
		if status == domain.StatusMissing {
			s.background(d, "start", s.runtime.Start)
		}
	}
	return nil
}

// Logs follows the Database's Container; found is false without one.
func (s *Service) Logs(ctx context.Context, id uint64, tail int, out func(stream, line string)) (bool, error) {
	d, err := s.get(ctx, id)
	if err != nil {
		return false, err
	}
	return s.runtime.Logs(ctx, d, true, tail, out)
}
