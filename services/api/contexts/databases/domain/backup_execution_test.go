package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func fieldOf(err error) string {
	var fe *FieldError
	if errors.As(err, &fe) {
		return fe.Field
	}
	return ""
}

func TestScheduledBackupCheck(t *testing.T) {
	for _, tc := range []struct {
		s     ScheduledBackupInput
		field string
	}{
		{ScheduledBackupInput{Cron: "0 3 * * *", Retention: 7}, ""},
		{ScheduledBackupInput{Cron: "*/15 * * * 1-5", Retention: 100}, ""},
		{ScheduledBackupInput{Cron: "@daily", Retention: 7}, ""},
		{ScheduledBackupInput{Cron: "daily", Retention: 7}, ""},
		{ScheduledBackupInput{Cron: "every_minute", Retention: 7}, ""},
		{ScheduledBackupInput{Cron: "@every 1h", Retention: 7}, "scheduled_backup.cron"},
		{ScheduledBackupInput{Cron: "nightly", Retention: 7}, "scheduled_backup.cron"},
		{ScheduledBackupInput{Cron: "TZ=Europe/Amsterdam 0 3 * * *", Retention: 7}, "scheduled_backup.cron"},
		{ScheduledBackupInput{Cron: "0 3 * *", Retention: 7}, "scheduled_backup.cron"},
		{ScheduledBackupInput{Cron: "61 3 * * *", Retention: 7}, "scheduled_backup.cron"},
		{ScheduledBackupInput{Cron: "0 3 * * *", Retention: 0}, "scheduled_backup.retention"},
		{ScheduledBackupInput{Cron: "0 3 * * *", Retention: 101}, "scheduled_backup.retention"},
	} {
		if got := fieldOf(tc.s.Check()); got != tc.field {
			t.Errorf("%+v: field %q, want %q", tc.s, got, tc.field)
		}
	}
}

func TestScheduledBackup(t *testing.T) {
	d, _ := NewDatabase(1, 1, Input{Name: "x", Type: PostgreSQL}, "x", pw)
	d.ID = 5
	def, err := NewScheduledBackup(d, DefaultScheduledBackup, time.Now())
	if err != nil || def.DatabaseID != 5 || def.Enabled || def.Cron != "0 3 * * *" || def.Retention != 7 || !def.EnabledAt.IsZero() {
		t.Fatalf("default %+v %v", def, err)
	}
	on := ScheduledBackupInput{Enabled: true, Cron: " 0 3 * * * ", Retention: 3}
	s, err := NewScheduledBackup(d, on, at("2026-09-30T14:00:00+02:00"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Cron != "0 3 * * *" || !s.EnabledAt.Equal(at("2026-09-30T12:00:00Z")) {
		t.Fatalf("got %+v", s)
	}
	// Saving it again while on keeps when it was switched on.
	on.Retention = 5
	s.Update(on, at("2026-10-05T00:00:00Z"))
	if !s.EnabledAt.Equal(at("2026-09-30T12:00:00Z")) || s.Retention != 5 {
		t.Fatalf("got %+v", s)
	}
	// Off forgets it; a bad input changes nothing.
	s.Update(ScheduledBackupInput{Cron: "0 3 * * *", Retention: 5}, time.Now())
	if s.Enabled || !s.EnabledAt.IsZero() {
		t.Fatalf("off %+v", s)
	}
	before := s
	if err := s.Update(ScheduledBackupInput{Cron: "nope", Retention: 5, Enabled: true}, time.Now()); fieldOf(err) != "scheduled_backup.cron" || s != before {
		t.Fatalf("bad input %+v %v", s, err)
	}

	r, _ := NewDatabase(1, 1, Input{Name: "r", Type: Redis}, "r", pw)
	if _, err := NewScheduledBackup(r, DefaultScheduledBackup, time.Now()); !errors.Is(err, ErrNoBackups) {
		t.Fatalf("redis scheduled backup: %v", err)
	}
}

func TestDue(t *testing.T) {
	s := ScheduledBackup{Enabled: true, Cron: "0 3 * * *", Retention: 7, EnabledAt: at("2026-09-30T14:00:00Z")}
	for _, tc := range []struct {
		last, now string
		due       bool
	}{
		// Switched on after today's 03:00: not until tomorrow.
		{"", "2026-09-30T14:01:00Z", false},
		{"", "2026-10-01T02:59:59Z", false},
		{"", "2026-10-01T03:00:00Z", true},
		// Ran tonight: not again until tomorrow.
		{"2026-10-01T03:00:00Z", "2026-10-01T03:01:00Z", false},
		{"2026-10-01T03:00:00Z", "2026-10-02T03:00:30Z", true},
		// Down for three days: due now, once (the next is after this run).
		{"2026-10-01T03:00:00Z", "2026-10-04T12:00:00Z", true},
		{"2026-10-04T12:00:00Z", "2026-10-04T12:01:00Z", false},
	} {
		var last time.Time
		if tc.last != "" {
			last = at(tc.last)
		}
		if got := s.Due(last, at(tc.now)); got != tc.due {
			t.Errorf("last %s now %s: due %v, want %v", tc.last, tc.now, got, tc.due)
		}
	}
	s.Enabled = false
	if s.Due(time.Time{}, at("2027-01-01T00:00:00Z")) {
		t.Fatal("disabled schedule is due")
	}
	// Coolify's shortcuts: daily is midnight UTC.
	for _, c := range []string{"daily", "@daily"} {
		s := ScheduledBackup{Enabled: true, Cron: c, EnabledAt: at("2026-09-30T14:00:00Z")}
		if got := s.NextAt(time.Time{}); !got.Equal(at("2026-10-01T00:00:00Z")) {
			t.Errorf("%s: next %s", c, got)
		}
	}
}

func TestBackupLifecycle(t *testing.T) {
	d, _ := NewDatabase(1, 1, Input{Name: "x", Type: MySQL}, "x", pw)
	d.ID = 9
	sb := ScheduledBackup{ID: 3, DatabaseID: 9, S3StorageID: 4}
	b, err := NewBackupExecution(d, sb, TriggerManual, at("2026-09-30T14:05:06+02:00"))
	if err != nil {
		t.Fatal(err)
	}
	if b.FileName != "20260930T120506Z.sql.gz" || b.Status != ExecutionRunning || b.DatabaseID != 9 || b.ScheduledBackupID != 3 || b.S3StorageID != 4 {
		t.Fatalf("got %+v", b)
	}
	if err := b.Succeed(123, false, at("2026-09-30T12:06:00Z")); err != nil {
		t.Fatal(err)
	}
	if !b.Local || b.S3 || b.S3StorageID != 0 || b.SizeBytes != 123 || !b.Restorable() {
		t.Fatalf("got %+v", b)
	}
	if err := b.Fail("late", false, 0, time.Now()); !errors.Is(err, ErrExecutionFinished) {
		t.Fatalf("fail after succeed: %v", err)
	}
	if _, err := NewBackupExecution(d, ScheduledBackup{ID: 4, DatabaseID: 10}, TriggerManual, time.Now()); err == nil {
		t.Fatal("a scheduled backup of another database")
	}
	f, _ := NewBackupExecution(d, sb, TriggerScheduled, time.Now())
	f.Fail("upload failed", true, 50, time.Now())
	if f.Restorable() || !f.Local || f.S3 {
		t.Fatalf("got %+v", f)
	}
	if err := f.Succeed(1, true, time.Now()); !errors.Is(err, ErrExecutionFinished) {
		t.Fatalf("succeed after fail: %v", err)
	}
	r, _ := NewDatabase(1, 1, Input{Name: "r", Type: Valkey}, "r", pw)
	if _, err := NewBackupExecution(r, ScheduledBackup{}, TriggerManual, time.Now()); !errors.Is(err, ErrNoBackups) {
		t.Fatalf("valkey backup: %v", err)
	}
}

func TestPrune(t *testing.T) {
	day := func(n int, s ExecutionStatus) BackupExecution {
		return BackupExecution{ID: uint64(n), Status: s, StartedAt: at("2026-10-01T03:00:00Z").AddDate(0, 0, n)}
	}
	backups := []BackupExecution{
		day(1, ExecutionSucceeded), day(2, ExecutionFailed), day(3, ExecutionSucceeded),
		day(4, ExecutionSucceeded), day(5, ExecutionFailed), day(6, ExecutionSucceeded), day(7, ExecutionRunning),
	}
	ids := func(bs []BackupExecution) (out []uint64) {
		for _, b := range bs {
			out = append(out, b.ID)
		}
		return
	}
	// Keep 2: 6 and 4 stay, the failure 5 between them stays, 7 runs.
	if got := ids(Prune(backups, 2)); len(got) != 3 || got[0] != 3 || got[1] != 2 || got[2] != 1 {
		t.Fatalf("prune 2: %v", got)
	}
	if got := Prune(backups, 7); len(got) != 0 {
		t.Fatalf("prune 7: %v", ids(got))
	}
}

func TestDatabaseTypeBackupCommands(t *testing.T) {
	for _, e := range DatabaseTypes {
		d, _ := NewDatabase(1, 1, Input{Name: "x", Type: e}, "x", pw)
		if !e.Spec().Backups {
			if d.DumpCommand() != nil || d.RestoreCommand() != nil || d.BackupExt() != "" {
				t.Errorf("%s has backup commands", e)
			}
			continue
		}
		if d.BackupExt() == "" {
			t.Errorf("%s has no extension", e)
		}
		for _, cmd := range [][]string{d.DumpCommand(), d.RestoreCommand()} {
			all := strings.Join(cmd, " ")
			if len(cmd) != 3 || cmd[0] != "sh" || strings.Contains(all, pw()) {
				t.Errorf("%s: %q", e, cmd)
			}
		}
		if !strings.Contains(d.RestoreCommand()[2], "/tmp/"+d.RestoreFile()) {
			t.Errorf("%s restore does not read %s", e, d.RestoreFile())
		}
	}
	if (Database{Type: PostgreSQL}).DumpGzip() || !(Database{Type: MariaDB}).DumpGzip() {
		t.Fatal("DumpGzip")
	}
}
