package app

import (
	"cmp"
	"context"
	"errors"
	"io"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

type memStore struct {
	mu       sync.Mutex
	dbs      map[uint64]domain.Database
	backups  map[uint64]domain.BackupExecution
	storages map[uint64]domain.S3Storage
	nextID   uint64
}

func newMemStore() *memStore {
	return &memStore{dbs: map[uint64]domain.Database{}, backups: map[uint64]domain.BackupExecution{}, storages: map[uint64]domain.S3Storage{}}
}

func (m *memStore) Create(_ context.Context, d domain.Database) (domain.Database, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d.ID = uint64(len(m.dbs) + 1)
	m.dbs[d.ID] = d
	return d, nil
}
func (m *memStore) Get(_ context.Context, id uint64) (domain.Database, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dbs[id]
	return d, ok, nil
}
func (m *memStore) ForProject(_ context.Context, projectID uint64) ([]domain.Database, error) {
	var out []domain.Database
	for _, d := range m.dbs {
		if d.ProjectID == projectID {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memStore) Wanted(context.Context) ([]domain.Database, error) {
	var out []domain.Database
	for _, d := range m.dbs {
		if d.DesiredState == domain.Running {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memStore) Update(_ context.Context, d domain.Database) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dbs[d.ID] = d
	return nil
}
func (m *memStore) Delete(_ context.Context, id uint64) error {
	delete(m.dbs, id)
	return nil
}
func (m *memStore) SlugTaken(_ context.Context, slug string) (bool, error) {
	for _, d := range m.dbs {
		if d.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}
func (m *memStore) PublicPortTaken(_ context.Context, port int, exceptID uint64) (bool, error) {
	for _, d := range m.dbs {
		if d.PublicPort == port && d.ID != exceptID {
			return true, nil
		}
	}
	return false, nil
}
func (m *memStore) CountForProject(ctx context.Context, projectID uint64) (int64, error) {
	l, _ := m.ForProject(ctx, projectID)
	return int64(len(l)), nil
}
func (m *memStore) CountForEnvironment(_ context.Context, environmentID uint64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for _, d := range m.dbs {
		if d.EnvironmentID == environmentID {
			n++
		}
	}
	return n, nil
}
func (m *memStore) ScheduledDatabases(context.Context) ([]domain.Database, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Database
	for _, d := range m.dbs {
		if d.ScheduledBackup.Enabled {
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(a, b domain.Database) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}
func (m *memStore) CreateBackupExecution(_ context.Context, b domain.BackupExecution) (domain.BackupExecution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	b.ID = m.nextID
	m.backups[b.ID] = b
	return b, nil
}
func (m *memStore) SaveBackupExecution(_ context.Context, b domain.BackupExecution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backups[b.ID] = b
	return nil
}
func (m *memStore) BackupExecution(_ context.Context, id uint64) (domain.BackupExecution, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.backups[id]
	return b, ok, nil
}
func (m *memStore) BackupExecutions(_ context.Context, databaseID uint64) ([]domain.BackupExecution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.BackupExecution
	for _, b := range m.backups {
		if b.DatabaseID == databaseID {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b domain.BackupExecution) int {
		if c := b.StartedAt.Compare(a.StartedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	return out, nil
}
func (m *memStore) LastScheduledStart(ctx context.Context, databaseID uint64) (time.Time, error) {
	bs, _ := m.BackupExecutions(ctx, databaseID)
	for _, b := range bs {
		if b.Trigger == domain.TriggerScheduled {
			return b.StartedAt, nil
		}
	}
	return time.Time{}, nil
}
func (m *memStore) DeleteBackupExecution(_ context.Context, id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.backups, id)
	return nil
}
func (m *memStore) DeleteBackupExecutions(_ context.Context, databaseID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, b := range m.backups {
		if b.DatabaseID == databaseID {
			delete(m.backups, id)
		}
	}
	return nil
}
func (m *memStore) FailRunningBackupExecutions(_ context.Context, reason string, at time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for id, b := range m.backups {
		if b.Status == domain.ExecutionRunning {
			b.Fail(reason, false, 0, at)
			m.backups[id] = b
			n++
		}
	}
	return n, nil
}
func (m *memStore) S3Storages(context.Context) ([]domain.S3Storage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.S3Storage
	for _, st := range m.storages {
		out = append(out, st)
	}
	slices.SortFunc(out, func(a, b domain.S3Storage) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}
func (m *memStore) S3Storage(_ context.Context, id uint64) (domain.S3Storage, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.storages[id]
	return st, ok, nil
}
func (m *memStore) S3StorageNameTaken(_ context.Context, name string, exceptID uint64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, st := range m.storages {
		if st.Name == name && st.ID != exceptID {
			return true, nil
		}
	}
	return false, nil
}
func (m *memStore) S3StorageInUse(_ context.Context, id uint64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.dbs {
		if d.ScheduledBackup.S3StorageID == id {
			return true, nil
		}
	}
	return false, nil
}
func (m *memStore) CreateS3Storage(_ context.Context, st domain.S3Storage) (domain.S3Storage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	st.ID = m.nextID
	m.storages[st.ID] = st
	return st, nil
}
func (m *memStore) SaveS3Storage(_ context.Context, st domain.S3Storage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storages[st.ID] = st
	return nil
}
func (m *memStore) DeleteS3Storage(_ context.Context, id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.storages, id)
	return nil
}

// fakeRuntime records calls and keeps which Databases have a Container.
type fakeRuntime struct {
	mu         sync.Mutex
	calls      []string
	containers map[uint64]bool
	startErr   error

	// dump is what Dump writes and exits with; dumpGate, when set, is
	// waited on before Dump returns.
	dump       []byte
	dumpCode   int
	dumpStderr string
	dumpGate   chan struct{}
	// copied is what CopyIn received, by name; execs every Exec'd command.
	copied   map[string][]byte
	execs    [][]string
	execCode int
}

func (f *fakeRuntime) record(call string, id uint64, has bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	f.containers[id] = has
}
func (f *fakeRuntime) Start(_ context.Context, d domain.Database) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.record("start", d.ID, true)
	return nil
}
func (f *fakeRuntime) Stop(_ context.Context, d domain.Database) error {
	f.record("stop", d.ID, false)
	return nil
}
func (f *fakeRuntime) Recreate(_ context.Context, d domain.Database) error {
	f.record("recreate", d.ID, true)
	return nil
}
func (f *fakeRuntime) Remove(_ context.Context, d domain.Database) error {
	f.record("remove", d.ID, false)
	return nil
}
func (f *fakeRuntime) Status(_ context.Context, d domain.Database) (domain.Status, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.containers[d.ID]:
		return domain.StatusRunning, "", nil
	case d.DesiredState == domain.Stopped:
		return domain.StatusStopped, "", nil
	}
	return domain.StatusMissing, "", nil
}
func (f *fakeRuntime) Logs(context.Context, domain.Database, bool, int, func(string, string)) (bool, error) {
	return false, nil
}
func (f *fakeRuntime) Dump(ctx context.Context, d domain.Database, w io.Writer) (int, string, error) {
	if f.dumpGate != nil {
		select {
		case <-f.dumpGate:
		case <-ctx.Done():
			return 0, "", ctx.Err()
		}
	}
	w.Write(f.dump)
	return f.dumpCode, f.dumpStderr, nil
}
func (f *fakeRuntime) CopyIn(_ context.Context, d domain.Database, name string, size int64, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if int64(len(b)) != size {
		return errors.New("size mismatch")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.copied == nil {
		f.copied = map[string][]byte{}
	}
	f.copied[name] = b
	return nil
}
func (f *fakeRuntime) Exec(_ context.Context, d domain.Database, cmd []string) (int, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.execs = append(f.execs, cmd)
	if f.execCode != 0 {
		return f.execCode, "restore broke", nil
	}
	return 0, "", nil
}
func (f *fakeRuntime) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func newTestService() (*Service, *memStore, *fakeRuntime) {
	store, rt := newMemStore(), &fakeRuntime{containers: map[uint64]bool{}}
	envs := func(_ context.Context, id uint64) (Environment, error) {
		if id != 7 {
			return Environment{}, ErrNotFound
		}
		return Environment{ID: 7, ProjectID: 3}, nil
	}
	s := NewService(store, rt, envs, "db.example.com")
	return s, store, rt
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCreateStartsInTheBackground(t *testing.T) {
	ctx := context.Background()
	s, _, rt := newTestService()
	if _, err := s.Create(ctx, 8, domain.Input{Name: "x", Type: domain.Redis}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown environment: %v", err)
	}
	v, err := s.Create(ctx, 7, domain.Input{Name: "Main", Type: domain.PostgreSQL, PublicPort: 5433})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if v.Slug != "main" || v.ProjectID != 3 || len(v.Credentials.Password) != 32 || v.PublicURL == "" {
		t.Fatalf("created %+v", v)
	}
	if got := rt.Calls(); !equal(got, []string{"start"}) {
		t.Fatalf("calls %v", got)
	}
	if v, _ := s.Get(ctx, v.ID); v.Status != domain.StatusRunning {
		t.Fatalf("status %s", v.Status)
	}
	second, err := s.Create(ctx, 7, domain.Input{Name: "main", Type: domain.Redis})
	if err != nil || second.Slug != "main-2" {
		t.Fatalf("second: %v %v", second.Slug, err)
	}
	_, err = s.Create(ctx, 7, domain.Input{Name: "third", Type: domain.Redis, PublicPort: 5433})
	var fe *domain.FieldError
	if !errors.As(err, &fe) || fe.Field != "public_port" {
		t.Fatalf("taken public port: %v", err)
	}
}

func TestStartErrorIsShown(t *testing.T) {
	ctx := context.Background()
	s, _, rt := newTestService()
	rt.startErr = errors.New("port is already allocated")
	v, err := s.Create(ctx, 7, domain.Input{Name: "x", Type: domain.Redis})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	v, _ = s.Get(ctx, v.ID)
	if v.Status != domain.StatusMissing || v.Error != "start failed: port is already allocated" {
		t.Fatalf("status %s, error %q", v.Status, v.Error)
	}
}

func TestUpdateRecreatesOnlyWhenNeeded(t *testing.T) {
	ctx := context.Background()
	s, _, rt := newTestService()
	v, _ := s.Create(ctx, 7, domain.Input{Name: "x", Type: domain.Redis})
	s.Wait()
	if _, err := s.Update(ctx, v.ID, domain.Input{Name: "renamed"}); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if got := rt.Calls(); !equal(got, []string{"start"}) {
		t.Fatalf("rename recreated: %v", got)
	}
	if _, err := s.Update(ctx, v.ID, domain.Input{Name: "renamed", PublicPort: 6390}); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if got := rt.Calls(); !equal(got, []string{"start", "recreate"}) {
		t.Fatalf("public port: %v", got)
	}
	// A stopped Database only stores the change.
	if _, err := s.Stop(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, v.ID, domain.Input{Name: "renamed", Version: "7"}); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if got := rt.Calls(); !equal(got, []string{"start", "recreate", "stop"}) {
		t.Fatalf("stopped: %v", got)
	}
	if v, _ := s.Get(ctx, v.ID); v.Status != domain.StatusStopped || v.Version != "7" {
		t.Fatalf("after stop: %s %s", v.Status, v.Version)
	}
}

func TestStopStartDeleteAndInUse(t *testing.T) {
	ctx := context.Background()
	s, store, rt := newTestService()
	v, _ := s.Create(ctx, 7, domain.Input{Name: "x", Type: domain.MongoDB})
	s.Wait()
	if used, _ := s.InUse(ctx, 3); !used {
		t.Fatal("project not in use")
	}
	if v, err := s.Stop(ctx, v.ID); err != nil || v.Status != domain.StatusStopped || v.DesiredState != domain.Stopped {
		t.Fatalf("stop: %+v %v", v.Status, err)
	}
	if _, err := s.Start(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if v, _ := s.Get(ctx, v.ID); v.Status != domain.StatusRunning {
		t.Fatalf("after start: %s", v.Status)
	}
	if err := s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, v.ID); found {
		t.Fatal("still stored")
	}
	if got := rt.Calls(); !equal(got, []string{"start", "stop", "recreate", "remove"}) {
		t.Fatalf("calls %v", got)
	}
	if used, _ := s.InUse(ctx, 3); used {
		t.Fatal("project still in use")
	}
	if _, err := s.Get(ctx, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
}

func TestRecoverStartsMissing(t *testing.T) {
	ctx := context.Background()
	s, _, rt := newTestService()
	a, _ := s.Create(ctx, 7, domain.Input{Name: "a", Type: domain.Redis})
	b, _ := s.Create(ctx, 7, domain.Input{Name: "b", Type: domain.Redis})
	s.Wait()
	s.Stop(ctx, b.ID)
	rt.containers[a.ID] = false // removed by hand
	rt.calls = nil
	if err := s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if got := rt.Calls(); !equal(got, []string{"start"}) || !rt.containers[a.ID] || rt.containers[b.ID] {
		t.Fatalf("recover: %v %v", got, rt.containers)
	}
}

func TestCreateWithoutAName(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestService()
	v, err := s.Create(ctx, 7, domain.Input{Type: domain.PostgreSQL})
	s.Wait()
	if err != nil || !regexp.MustCompile(`^postgresql-database-[a-z2-7]{8}$`).MatchString(v.Name) || v.Slug != v.Name {
		t.Fatalf("created %v, name %q, slug %q", err, v.Name, v.Slug)
	}
	var fe *domain.FieldError
	if _, err := s.Create(ctx, 7, domain.Input{Type: "oracle"}); !errors.As(err, &fe) || fe.Field != "type" {
		t.Fatalf("unknown type: %v", err)
	}
}
