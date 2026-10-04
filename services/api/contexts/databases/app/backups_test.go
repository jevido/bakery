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
	v, err := s.Create(ctx, 7, domain.Input{Name: "main", Type: domain.PostgreSQL})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	st, _ := store.CreateS3Storage(ctx, domain.S3Storage{Name: "garage", Bucket: "b", Prefix: "p"})
	return e, v, st
}

func (e *backupEnv) backUp(t *testing.T, id uint64) domain.BackupExecution {
	t.Helper()
	b, err := e.s.BackUpDatabase(context.Background(), id, domain.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	e.s.Wait()
	b, _, _ = e.store.BackupExecution(context.Background(), b.ID)
	return b
}

// setSchedule replaces the Database's first Scheduled backup, the one a
// new Database gets.
func setSchedule(t *testing.T, e *backupEnv, id uint64, in domain.ScheduledBackupInput) ScheduledBackupView {
	t.Helper()
	got, err := e.s.UpdateScheduledBackup(context.Background(), firstSchedule(t, e, id), in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func firstSchedule(t *testing.T, e *backupEnv, id uint64) uint64 {
	t.Helper()
	list, err := e.s.ScheduledBackups(context.Background(), id)
	if err != nil || len(list) == 0 {
		t.Fatalf("scheduled backups of %d: %v %v", id, list, err)
	}
	return list[0].ID
}

func TestBackUpLocalAndS3(t *testing.T) {
	e, v, st := newBackupEnv(t)
	setSchedule(t, e, v.ID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	b := e.backUp(t, v.ID)
	if b.Status != domain.ExecutionSucceeded || !b.Local || !b.S3 || b.SizeBytes != int64(len("PGDMP-data")) || !strings.HasSuffix(b.FileName, ".dump") {
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
	v, _ := e.s.Create(context.Background(), 7, domain.Input{Name: "my", Type: domain.MySQL})
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
	if b.Status != domain.ExecutionFailed || b.Local || !strings.Contains(b.Error, "connection refused") || len(e.files.names()) != 0 {
		t.Fatalf("failed dump: %+v, files %v", b, e.files.names())
	}

	e.rt.dumpCode = 0
	e.bucket.putErr = errors.New("s3: AccessDenied")
	setSchedule(t, e, v.ID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	b = e.backUp(t, v.ID)
	if b.Status != domain.ExecutionFailed || !b.Local || b.S3 || !strings.Contains(b.Error, "AccessDenied") || len(e.files.names()) != 1 {
		t.Fatalf("failed upload: %+v, files %v", b, e.files.names())
	}

	r, _ := e.s.Create(ctx, 7, domain.Input{Name: "r", Type: domain.Redis})
	e.s.Wait()
	if _, err := e.s.BackUpDatabase(ctx, r.ID, domain.TriggerManual); !errors.Is(err, domain.ErrNoBackups) {
		t.Fatalf("redis: %v", err)
	}
	e.s.Stop(ctx, v.ID)
	if _, err := e.s.BackUpDatabase(ctx, v.ID, domain.TriggerManual); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("stopped: %v", err)
	}
}

func TestBackUpOneAtATime(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	ctx := context.Background()
	e.rt.dumpGate = make(chan struct{})
	first, err := e.s.BackUpDatabase(ctx, v.ID, domain.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.BackUpDatabase(ctx, v.ID, domain.TriggerManual); !errors.Is(err, ErrBusy) {
		t.Fatalf("second: %v", err)
	}
	if err := e.s.DeleteBackupExecution(ctx, first.ID, true); !errors.Is(err, ErrBackupRunning) {
		t.Fatalf("delete running: %v", err)
	}
	close(e.rt.dumpGate)
	e.s.Wait()
	if _, err := e.s.BackUpDatabase(ctx, v.ID, domain.TriggerManual); err != nil {
		t.Fatalf("after the first: %v", err)
	}
	e.s.Wait()
}

func TestRetentionPrunes(t *testing.T) {
	e, v, st := newBackupEnv(t)
	setSchedule(t, e, v.ID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 2, S3StorageID: st.ID})
	var all []domain.BackupExecution
	for range 3 {
		all = append(all, e.backUp(t, v.ID))
	}
	list, _ := e.s.BackupExecutions(context.Background(), v.ID)
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
	setSchedule(t, e, v.ID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	a := e.backUp(t, v.ID)
	e.backUp(t, v.ID)
	// Deleted without its S3 object, then with it.
	if err := e.s.DeleteBackupExecution(ctx, a.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(e.files.names()) != 1 || len(e.bucket.keys()) != 2 {
		t.Fatalf("after delete backup keeping S3: files %v objects %v", e.files.names(), e.bucket.keys())
	}
	b := e.backUp(t, v.ID)
	if err := e.s.DeleteBackupExecution(ctx, b.ID, true); err != nil {
		t.Fatal(err)
	}
	if len(e.files.names()) != 1 || len(e.bucket.keys()) != 2 {
		t.Fatalf("after delete backup: files %v objects %v", e.files.names(), e.bucket.keys())
	}
	if err := e.s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	// Local Backups go with the Database; S3 copies stay.
	if len(e.files.names()) != 0 || len(e.bucket.keys()) != 2 {
		t.Fatalf("after delete database: files %v objects %v", e.files.names(), e.bucket.keys())
	}
	if list, _ := e.store.BackupExecutions(ctx, v.ID); len(list) != 0 {
		t.Fatalf("rows left %+v", list)
	}
}

func TestRecoverFailsInterrupted(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	ctx := context.Background()
	d, _, _ := e.store.Get(ctx, v.ID)
	sbs, _ := e.store.ScheduledBackups(ctx, v.ID)
	b, _ := domain.NewBackupExecution(d, sbs[0], domain.TriggerScheduled, e.s.Now())
	b, _ = e.store.CreateBackupExecution(ctx, b)
	if err := e.s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	e.s.Wait()
	got, _, _ := e.store.BackupExecution(ctx, b.ID)
	if got.Status != domain.ExecutionFailed || !strings.Contains(got.Error, "interrupted") {
		t.Fatalf("after recover %+v", got)
	}
}

func TestSetBackupSchedule(t *testing.T) {
	e, v, st := newBackupEnv(t)
	ctx := context.Background()
	on := domain.ScheduledBackupInput{Enabled: true, Cron: "0 3 * * *", Retention: 3, S3StorageID: st.ID}
	sID := firstSchedule(t, e, v.ID)
	got, err := e.s.UpdateScheduledBackup(ctx, sID, on)
	if err != nil {
		t.Fatal(err)
	}
	// The clock is just past 03:00 on 1 October: the next run is tomorrow.
	if !got.Enabled || !got.NextBackupAt.Equal(time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("schedule %+v, next %s", got.ScheduledBackup, got.NextBackupAt)
	}
	on.S3StorageID = 999
	var fe *domain.FieldError
	if _, err := e.s.UpdateScheduledBackup(ctx, sID, on); !errors.As(err, &fe) || fe.Field != "scheduled_backup.s3_storage_id" {
		t.Fatalf("unknown storage: %v", err)
	}
	on.S3StorageID, on.Cron = 0, "every night"
	if _, err := e.s.UpdateScheduledBackup(ctx, sID, on); !errors.As(err, &fe) || fe.Field != "scheduled_backup.cron" {
		t.Fatalf("bad cron: %v", err)
	}
	off, _ := e.s.UpdateScheduledBackup(ctx, sID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 3})
	if !off.NextBackupAt.IsZero() {
		t.Fatalf("off has a next backup: %s", off.NextBackupAt)
	}
}

func TestTick(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	ctx := context.Background()
	// Switched on at 03:00:0x on 1 October.
	setSchedule(t, e, v.ID, domain.ScheduledBackupInput{Enabled: true, Cron: "0 3 * * *", Retention: 7})
	day := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, time.UTC) }
	count := func() int {
		e.s.Wait()
		l, _ := e.s.BackupExecutions(ctx, v.ID)
		return len(l)
	}
	e.s.Tick(ctx, day(1, 14, 0))
	if n := count(); n != 0 {
		t.Fatalf("fired the day it was switched on: %d", n)
	}
	// The API was down for three nights; the first tick catches up once.
	e.clock = day(4, 12, 0)
	e.s.Tick(ctx, day(4, 12, 0))
	e.s.Tick(ctx, day(4, 12, 1))
	if n := count(); n != 1 {
		t.Fatalf("after a long outage: %d backups", n)
	}
	l, _ := e.s.BackupExecutions(ctx, v.ID)
	if l[0].Trigger != domain.TriggerScheduled || l[0].Status != domain.ExecutionSucceeded {
		t.Fatalf("scheduled backup %+v", l[0])
	}
	// Stopped when due: a failed scheduled Backup says why, once.
	e.s.Stop(ctx, v.ID)
	e.clock = day(5, 3, 0)
	e.s.Tick(ctx, day(5, 3, 0))
	e.s.Tick(ctx, day(5, 3, 1))
	if n := count(); n != 2 {
		t.Fatalf("while stopped: %d backups", n)
	}
	l, _ = e.s.BackupExecutions(ctx, v.ID)
	if l[0].Status != domain.ExecutionFailed || !strings.Contains(l[0].Error, "not running") {
		t.Fatalf("missed run %+v", l[0])
	}
}

func TestRestore(t *testing.T) {
	e, v, st := newBackupEnv(t)
	ctx := context.Background()
	setSchedule(t, e, v.ID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	b := e.backUp(t, v.ID)
	d, _, _ := e.store.Get(ctx, v.ID)

	if err := e.s.Restore(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	e.s.Wait()
	if got := string(e.rt.copied[d.RestoreFile()]); got != "PGDMP-data" {
		t.Fatalf("copied %q", got)
	}
	if len(e.rt.execs) != 1 || e.rt.execs[0][2] != d.RestoreCommand()[2] {
		t.Fatalf("execs %v", e.rt.execs)
	}
	view, _ := e.s.Get(ctx, v.ID)
	if view.Restoring || view.LastRestore == nil || view.LastRestore.BackupExecutionID != b.ID || view.LastRestore.Error != "" {
		t.Fatalf("after restore: %v %+v", view.Restoring, view.LastRestore)
	}

	// The local file is gone: it comes from S3.
	e.files.Remove(v.ID, b.FileName)
	e.bucket.objects["p/main-1/"+b.FileName] = []byte("PGDMP-from-s3")
	e.s.Restore(ctx, b.ID)
	e.s.Wait()
	if got := string(e.rt.copied[d.RestoreFile()]); got != "PGDMP-from-s3" {
		t.Fatalf("copied from S3 %q", got)
	}

	// A failing restore is reported with its output.
	e.rt.execCode = 1
	e.s.Restore(ctx, b.ID)
	e.s.Wait()
	view, _ = e.s.Get(ctx, v.ID)
	if view.LastRestore == nil || !strings.Contains(view.LastRestore.Error, "restore broke") {
		t.Fatalf("failed restore %+v", view.LastRestore)
	}

	// Not while a Backup runs; not a failed Backup.
	e.rt.dumpGate = make(chan struct{})
	running, _ := e.s.BackUpDatabase(ctx, v.ID, domain.TriggerManual)
	if err := e.s.Restore(ctx, b.ID); !errors.Is(err, ErrBusy) {
		t.Fatalf("restore while backing up: %v", err)
	}
	if err := e.s.Restore(ctx, running.ID); !errors.Is(err, ErrNotRestorable) {
		t.Fatalf("restore of a running backup: %v", err)
	}
	close(e.rt.dumpGate)
	e.s.Wait()
}

func TestS3Storages(t *testing.T) {
	e, v, st := newBackupEnv(t)
	ctx := context.Background()
	in := domain.S3Input{Name: "garage", Endpoint: "http://127.0.0.1:4960", Bucket: "bakery-backups", AccessKey: "a", SecretKey: "s"}
	var fe *domain.FieldError
	if _, err := e.s.CreateS3Storage(ctx, in); !errors.As(err, &fe) || fe.Field != "name" {
		t.Fatalf("duplicate name: %v", err)
	}
	in.Name = "other"
	other, err := e.s.CreateS3Storage(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	in.SecretKey = ""
	if got, err := e.s.UpdateS3Storage(ctx, other.ID, in); err != nil || got.SecretKey != "s" {
		t.Fatalf("update keeps secret: %+v %v", got, err)
	}
	if connErr, err := e.s.CheckS3Storage(ctx, other.ID, in); connErr != nil || err != nil {
		t.Fatalf("check: %v %v", connErr, err)
	}
	if _, err := e.s.CheckS3Storage(ctx, 0, in); !errors.As(err, &fe) || fe.Field != "secret_key" {
		t.Fatalf("check without a secret: %v", err)
	}
	setSchedule(t, e, v.ID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 7, S3StorageID: st.ID})
	if err := e.s.DeleteS3Storage(ctx, st.ID); !errors.Is(err, ErrS3StorageInUse) {
		t.Fatalf("delete in use: %v", err)
	}
	if err := e.s.DeleteS3Storage(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBackupFinishedIsHeard(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	var mu sync.Mutex
	var heard []domain.BackupExecution
	e.s.BackupExecutionFinished = func(_ context.Context, d domain.Database, b domain.BackupExecution) {
		mu.Lock()
		defer mu.Unlock()
		if d.ID != v.ID {
			t.Errorf("database %d, want %d", d.ID, v.ID)
		}
		heard = append(heard, b)
	}
	e.backUp(t, v.ID)
	e.rt.dumpCode, e.rt.dumpStderr = 1, "pg_dump: error: connection refused"
	e.backUp(t, v.ID)
	mu.Lock()
	defer mu.Unlock()
	if len(heard) != 2 || heard[0].Status != domain.ExecutionSucceeded || heard[1].Status != domain.ExecutionFailed || !strings.Contains(heard[1].Error, "connection refused") {
		t.Fatalf("heard %+v", heard)
	}
}

func TestManyScheduledBackups(t *testing.T) {
	e, v, st := newBackupEnv(t)
	ctx := context.Background()

	// A new Database of a type with backups starts with one, off.
	list, err := e.s.ScheduledBackups(ctx, v.ID)
	if err != nil || len(list) != 1 || list[0].Enabled || list[0].Cron != "0 3 * * *" || list[0].Retention != 7 {
		t.Fatalf("default %+v %v", list, err)
	}
	first := list[0]
	second, err := e.s.CreateScheduledBackup(ctx, v.ID, domain.ScheduledBackupInput{Enabled: true, Cron: "0 * * * *", Retention: 1, S3StorageID: st.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !second.NextBackupAt.Equal(time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("next %s", second.NextBackupAt)
	}
	var fe *domain.FieldError
	if _, err := e.s.CreateScheduledBackup(ctx, v.ID, domain.ScheduledBackupInput{Cron: "0 3 * * *", Retention: 1, S3StorageID: 999}); !errors.As(err, &fe) || fe.Field != "scheduled_backup.s3_storage_id" {
		t.Fatalf("unknown storage: %v", err)
	}

	// Back up now on each; each lists only its own, the Database all.
	backUp := func(id uint64) domain.BackupExecution {
		b, err := e.s.BackUp(ctx, id, domain.TriggerManual)
		if err != nil {
			t.Fatal(err)
		}
		e.s.Wait()
		b, _, _ = e.store.BackupExecution(ctx, b.ID)
		return b
	}
	a := backUp(first.ID)
	b1 := backUp(second.ID)
	b2 := backUp(second.ID)
	if a.ScheduledBackupID != first.ID || a.S3 || !b2.S3 {
		t.Fatalf("executions %+v %+v", a, b2)
	}
	// The second keeps 1: its first execution is pruned, the first's stays.
	own, _ := e.s.ScheduledBackupExecutions(ctx, second.ID)
	if len(own) != 1 || own[0].ID != b2.ID {
		t.Fatalf("second's executions %+v", own)
	}
	if _, found, _ := e.store.BackupExecution(ctx, b1.ID); found {
		t.Fatal("pruned execution kept")
	}
	if own, _ := e.s.ScheduledBackupExecutions(ctx, first.ID); len(own) != 1 || own[0].ID != a.ID {
		t.Fatalf("first's executions %+v", own)
	}
	if all, _ := e.s.BackupExecutions(ctx, v.ID); len(all) != 2 {
		t.Fatalf("database's executions %+v", all)
	}

	// Deleting one removes its executions and files; its S3 objects stay
	// unless asked.
	if err := e.s.DeleteScheduledBackup(ctx, second.ID, Copies{Local: true}); err != nil {
		t.Fatal(err)
	}
	if all, _ := e.s.BackupExecutions(ctx, v.ID); len(all) != 1 || all[0].ID != a.ID {
		t.Fatalf("after delete %+v", all)
	}
	if got := e.files.names(); len(got) != 1 || len(e.bucket.keys()) != 1 {
		t.Fatalf("files %v objects %v", got, e.bucket.keys())
	}
	if _, err := e.s.ScheduledBackup(ctx, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}

	// Not on a Database type without backups.
	r, _ := e.s.Create(ctx, 7, domain.Input{Name: "r", Type: domain.Redis})
	e.s.Wait()
	if l, _ := e.s.ScheduledBackups(ctx, r.ID); len(l) != 0 {
		t.Fatalf("redis has %+v", l)
	}
	if _, err := e.s.CreateScheduledBackup(ctx, r.ID, domain.DefaultScheduledBackup); !errors.Is(err, domain.ErrNoBackups) {
		t.Fatalf("redis: %v", err)
	}

	// Deleting the Database deletes its Scheduled backups.
	if err := e.s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := e.store.ScheduledBackups(ctx, v.ID); len(l) != 0 {
		t.Fatalf("left %+v", l)
	}
}

func TestDeleteScheduledBackupWhileRunning(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	ctx := context.Background()
	list, _ := e.s.ScheduledBackups(ctx, v.ID)
	e.rt.dumpGate = make(chan struct{})
	if _, err := e.s.BackUp(ctx, list[0].ID, domain.TriggerManual); err != nil {
		t.Fatal(err)
	}
	if err := e.s.DeleteScheduledBackup(ctx, list[0].ID, Copies{Local: true}); !errors.Is(err, ErrBackupRunning) {
		t.Fatalf("delete while running: %v", err)
	}
	close(e.rt.dumpGate)
	e.s.Wait()
}

func TestTickEachScheduledBackup(t *testing.T) {
	e, v, _ := newBackupEnv(t)
	ctx := context.Background()
	daily, _ := e.s.ScheduledBackups(ctx, v.ID)
	e.s.UpdateScheduledBackup(ctx, daily[0].ID, domain.ScheduledBackupInput{Enabled: true, Cron: "0 3 * * *", Retention: 7})
	hourly, _ := e.s.CreateScheduledBackup(ctx, v.ID, domain.ScheduledBackupInput{Enabled: true, Cron: "0 * * * *", Retention: 7})
	count := func(id uint64) int {
		e.s.Wait()
		l, _ := e.s.ScheduledBackupExecutions(ctx, id)
		return len(l)
	}
	// 04:00 on 1 October: only the hourly one is due.
	e.clock = time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	e.s.Tick(ctx, e.clock)
	if count(daily[0].ID) != 0 || count(hourly.ID) != 1 {
		t.Fatalf("at 04:00: daily %d hourly %d", count(daily[0].ID), count(hourly.ID))
	}
	// 03:00 the next day: both are due; the second finds the Database busy
	// with the first and starts on the next Tick, without a failure.
	e.rt.dumpGate = make(chan struct{})
	e.clock = time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	e.s.Tick(ctx, e.clock)
	close(e.rt.dumpGate)
	if count(daily[0].ID) != 1 || count(hourly.ID) != 1 {
		t.Fatalf("at 03:00: daily %d hourly %d", count(daily[0].ID), count(hourly.ID))
	}
	e.s.Tick(ctx, e.clock.Add(time.Minute))
	n := count(hourly.ID)
	l, _ := e.s.ScheduledBackupExecutions(ctx, hourly.ID)
	if n != 2 || l[0].Status != domain.ExecutionSucceeded {
		t.Fatalf("at 03:01: hourly %+v", l)
	}
}
