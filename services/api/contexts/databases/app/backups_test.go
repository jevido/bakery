package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

// memFiles keeps Backup files in memory.
type memFiles struct {
	mu    sync.Mutex
	files map[string][]byte
}

func key(id uint64, name string) string { return fmt.Sprintf("%d/%s", id, name) }

func (m *memFiles) Write(id uint64, name string, write func(io.Writer) error) (int64, error) {
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key(id, name)] = buf.Bytes()
	return int64(buf.Len()), nil
}

type memFile struct{ *bytes.Reader }

func (memFile) Close() error { return nil }

func (m *memFiles) Open(id uint64, name string) (BackupFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[key(id, name)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return memFile{bytes.NewReader(b)}, nil
}
func (m *memFiles) Remove(id uint64, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, key(id, name))
	return nil
}
func (m *memFiles) RemoveAll(id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.files {
		if strings.HasPrefix(k, fmt.Sprintf("%d/", id)) {
			delete(m.files, k)
		}
	}
	return nil
}
func (m *memFiles) names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for k := range m.files {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// memBucket is one fake S3 storage.
type memBucket struct {
	mu      sync.Mutex
	objects map[string][]byte
	putErr  error
}

func (b *memBucket) Check(context.Context) error { return nil }
func (b *memBucket) Put(_ context.Context, key string, r io.ReaderAt, size int64) error {
	if b.putErr != nil {
		return b.putErr
	}
	data, err := io.ReadAll(io.NewSectionReader(r, 0, size))
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[key] = data
	return nil
}
func (b *memBucket) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.objects[key]
	if !ok {
		return nil, 0, errors.New("NoSuchKey")
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}
func (b *memBucket) Delete(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.objects, key)
	return nil
}
func (b *memBucket) keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for k := range b.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type backupEnv struct {
	s      *Service
	store  *memStore
	rt     *fakeRuntime
	files  *memFiles
	bucket *memBucket
	clock  time.Time
}

// newBackupEnv has a running PostgreSQL Database (id 1) and an S3 storage.
func newBackupEnv(t *testing.T) (*backupEnv, View, domain.S3Storage) {
	t.Helper()
	s, store, rt := newTestService()
	e := &backupEnv{s: s, store: store, rt: rt, files: &memFiles{files: map[string][]byte{}}, bucket: &memBucket{objects: map[string][]byte{}}}
	e.clock = time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	s.Now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		e.clock = e.clock.Add(time.Second)
		return e.clock
	}
	s.Files = e.files
	s.S3 = func(domain.S3Storage) S3Client { return e.bucket }
	rt.dump = []byte("PGDMP-data")
	ctx := context.Background()
	v, err := s.Create(ctx, 7, domain.Input{Name: "main", Engine: domain.PostgreSQL})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	st, _ := store.CreateS3Storage(ctx, domain.S3Storage{Name: "garage", Bucket: "b", Prefix: "p"})
	return e, v, st
}

func (e *backupEnv) backUp(t *testing.T, id uint64) domain.Backup {
	t.Helper()
	b, err := e.s.BackUp(context.Background(), id, domain.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	e.s.Wait()
	b, _, _ = e.store.Backup(context.Background(), b.ID)
	return b
}

func setSchedule(t *testing.T, e *backupEnv, id uint64, sched domain.BackupSchedule) {
	t.Helper()
	d, _, _ := e.store.Get(context.Background(), id)
	if err := d.SetBackupSchedule(sched, e.s.Now()); err != nil {
		t.Fatal(err)
	}
	e.store.Update(context.Background(), d)
}

func TestBackUpLocalAndS3(t *testing.T) {
	e, v, st := newBackupEnv(t)
	setSchedule(t, e, v.ID, domain.BackupSchedule{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	b := e.backUp(t, v.ID)
	if b.Status != domain.BackupSucceeded || !b.Local || !b.S3 || b.SizeBytes != int64(len("PGDMP-data")) || !strings.HasSuffix(b.FileName, ".dump") {
		t.Fatalf("backup %+v", b)
	}
	if got := e.files.names(); len(got) != 1 || got[0] != key(v.ID, b.FileName) {
		t.Fatalf("files %v", got)
	}
	if got := e.bucket.keys(); len(got) != 1 || got[0] != "p/main-1/"+b.FileName {
		t.Fatalf("objects %v", got)
	}
}

func TestBackUpGzipsMySQL(t *testing.T) {
	e, _, _ := newBackupEnv(t)
	v, _ := e.s.Create(context.Background(), 7, domain.Input{Name: "my", Engine: domain.MySQL})
	e.s.Wait()
	e.rt.dump = []byte("CREATE TABLE t;")
	b := e.backUp(t, v.ID)
	f, _ := e.files.Open(v.ID, b.FileName)
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(zr); string(got) != "CREATE TABLE t;" {
		t.Fatalf("gunzipped %q", got)
	}
}

func TestBackUpFailures(t *testing.T) {
	e, v, st := newBackupEnv(t)
	ctx := context.Background()

	e.rt.dumpCode, e.rt.dumpStderr = 1, "pg_dump: error: connection refused"
	b := e.backUp(t, v.ID)
	if b.Status != domain.BackupFailed || b.Local || !strings.Contains(b.Error, "connection refused") || len(e.files.names()) != 0 {
		t.Fatalf("failed dump: %+v, files %v", b, e.files.names())
	}

	e.rt.dumpCode = 0
	e.bucket.putErr = errors.New("s3: AccessDenied")
	setSchedule(t, e, v.ID, domain.BackupSchedule{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	b = e.backUp(t, v.ID)
	if b.Status != domain.BackupFailed || !b.Local || b.S3 || !strings.Contains(b.Error, "AccessDenied") || len(e.files.names()) != 1 {
		t.Fatalf("failed upload: %+v, files %v", b, e.files.names())
	}

	r, _ := e.s.Create(ctx, 7, domain.Input{Name: "r", Engine: domain.Redis})
	e.s.Wait()
	if _, err := e.s.BackUp(ctx, r.ID, domain.TriggerManual); !errors.Is(err, domain.ErrNoBackups) {
		t.Fatalf("redis: %v", err)
	}
	e.s.Stop(ctx, v.ID)
	if _, err := e.s.BackUp(ctx, v.ID, domain.TriggerManual); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("stopped: %v", err)
	}
}

func TestBackUpOneAtATime(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	ctx := context.Background()
	e.rt.dumpGate = make(chan struct{})
	first, err := e.s.BackUp(ctx, v.ID, domain.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.BackUp(ctx, v.ID, domain.TriggerManual); !errors.Is(err, ErrBusy) {
		t.Fatalf("second: %v", err)
	}
	if err := e.s.DeleteBackup(ctx, first.ID); !errors.Is(err, ErrBackupRunning) {
		t.Fatalf("delete running: %v", err)
	}
	close(e.rt.dumpGate)
	e.s.Wait()
	if _, err := e.s.BackUp(ctx, v.ID, domain.TriggerManual); err != nil {
		t.Fatalf("after the first: %v", err)
	}
	e.s.Wait()
}

func TestRetentionPrunes(t *testing.T) {
	e, v, st := newBackupEnv(t)
	setSchedule(t, e, v.ID, domain.BackupSchedule{Cron: "0 3 * * *", Retention: 2, S3StorageID: st.ID})
	var all []domain.Backup
	for range 3 {
		all = append(all, e.backUp(t, v.ID))
	}
	list, _ := e.s.Backups(context.Background(), v.ID)
	if len(list) != 2 || list[0].ID != all[2].ID || list[1].ID != all[1].ID {
		t.Fatalf("kept %+v", list)
	}
	if len(e.files.names()) != 2 || len(e.bucket.keys()) != 2 {
		t.Fatalf("files %v objects %v", e.files.names(), e.bucket.keys())
	}
	for _, k := range e.bucket.keys() {
		if strings.HasSuffix(k, all[0].FileName) {
			t.Fatalf("oldest still in S3: %v", k)
		}
	}
}

func TestDeleteBackupAndDatabase(t *testing.T) {
	e, v, st := newBackupEnv(t)
	ctx := context.Background()
	setSchedule(t, e, v.ID, domain.BackupSchedule{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	a := e.backUp(t, v.ID)
	e.backUp(t, v.ID)
	if err := e.s.DeleteBackup(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if len(e.files.names()) != 1 || len(e.bucket.keys()) != 1 {
		t.Fatalf("after delete backup: files %v objects %v", e.files.names(), e.bucket.keys())
	}
	if err := e.s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	// Local Backups go with the Database; S3 copies stay.
	if len(e.files.names()) != 0 || len(e.bucket.keys()) != 1 {
		t.Fatalf("after delete database: files %v objects %v", e.files.names(), e.bucket.keys())
	}
	if list, _ := e.store.Backups(ctx, v.ID); len(list) != 0 {
		t.Fatalf("rows left %+v", list)
	}
}

func TestRecoverFailsInterrupted(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	ctx := context.Background()
	d, _, _ := e.store.Get(ctx, v.ID)
	b, _ := domain.NewBackup(d, domain.TriggerScheduled, e.s.Now())
	b, _ = e.store.CreateBackup(ctx, b)
	if err := e.s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	e.s.Wait()
	got, _, _ := e.store.Backup(ctx, b.ID)
	if got.Status != domain.BackupFailed || !strings.Contains(got.Error, "interrupted") {
		t.Fatalf("after recover %+v", got)
	}
}
