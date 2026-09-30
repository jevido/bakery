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

func TestBackupScheduleCheck(t *testing.T) {
	for _, tc := range []struct {
		s     BackupSchedule
		field string
	}{
		{BackupSchedule{Cron: "0 3 * * *", Retention: 7}, ""},
		{BackupSchedule{Cron: "*/15 * * * 1-5", Retention: 100}, ""},
		{BackupSchedule{Cron: "@daily", Retention: 7}, "backup_schedule.cron"},
		{BackupSchedule{Cron: "TZ=Europe/Amsterdam 0 3 * * *", Retention: 7}, "backup_schedule.cron"},
		{BackupSchedule{Cron: "0 3 * *", Retention: 7}, "backup_schedule.cron"},
		{BackupSchedule{Cron: "61 3 * * *", Retention: 7}, "backup_schedule.cron"},
		{BackupSchedule{Cron: "0 3 * * *", Retention: 0}, "backup_schedule.retention"},
		{BackupSchedule{Cron: "0 3 * * *", Retention: 101}, "backup_schedule.retention"},
	} {
		if got := fieldOf(tc.s.Check()); got != tc.field {
			t.Errorf("%+v: field %q, want %q", tc.s, got, tc.field)
		}
	}
}

func TestSetBackupSchedule(t *testing.T) {
	d, _ := NewDatabase(1, 1, Input{Name: "x", Engine: PostgreSQL}, "x", pw)
	if d.BackupSchedule != DefaultBackupSchedule {
		t.Fatalf("new database schedule %+v", d.BackupSchedule)
	}
	on := BackupSchedule{Enabled: true, Cron: " 0 3 * * * ", Retention: 3}
	if err := d.SetBackupSchedule(on, at("2026-09-30T14:00:00+02:00")); err != nil {
		t.Fatal(err)
	}
	if d.BackupSchedule.Cron != "0 3 * * *" || !d.BackupSchedule.EnabledAt.Equal(at("2026-09-30T12:00:00Z")) {
		t.Fatalf("got %+v", d.BackupSchedule)
	}
	// Saving it again while on keeps when it was switched on.
	on.Retention = 5
	d.SetBackupSchedule(on, at("2026-10-05T00:00:00Z"))
	if !d.BackupSchedule.EnabledAt.Equal(at("2026-09-30T12:00:00Z")) || d.BackupSchedule.Retention != 5 {
		t.Fatalf("got %+v", d.BackupSchedule)
	}

	r, _ := NewDatabase(1, 1, Input{Name: "r", Engine: Redis}, "r", pw)
	if err := r.SetBackupSchedule(on, time.Now()); !errors.Is(err, ErrNoBackups) {
		t.Fatalf("redis schedule: %v", err)
	}
	// Off is fine on any Engine.
	if err := r.SetBackupSchedule(DefaultBackupSchedule, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestDue(t *testing.T) {
	s := BackupSchedule{Enabled: true, Cron: "0 3 * * *", Retention: 7, EnabledAt: at("2026-09-30T14:00:00Z")}
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
}

func TestBackupLifecycle(t *testing.T) {
	d, _ := NewDatabase(1, 1, Input{Name: "x", Engine: MySQL}, "x", pw)
	d.ID = 9
	d.BackupSchedule.S3StorageID = 4
	b, err := NewBackup(d, TriggerManual, at("2026-09-30T14:05:06+02:00"))
	if err != nil {
		t.Fatal(err)
	}
	if b.FileName != "20260930T120506Z.sql.gz" || b.Status != BackupRunning || b.DatabaseID != 9 || b.S3StorageID != 4 {
		t.Fatalf("got %+v", b)
	}
	if err := b.Succeed(123, false, at("2026-09-30T12:06:00Z")); err != nil {
		t.Fatal(err)
	}
	if !b.Local || b.S3 || b.S3StorageID != 0 || b.SizeBytes != 123 || !b.Restorable() {
		t.Fatalf("got %+v", b)
	}
	if err := b.Fail("late", false, 0, time.Now()); !errors.Is(err, ErrBackupFinished) {
		t.Fatalf("fail after succeed: %v", err)
	}
	f, _ := NewBackup(d, TriggerScheduled, time.Now())
	f.Fail("upload failed", true, 50, time.Now())
	if f.Restorable() || !f.Local || f.S3 {
		t.Fatalf("got %+v", f)
	}
	if err := f.Succeed(1, true, time.Now()); !errors.Is(err, ErrBackupFinished) {
		t.Fatalf("succeed after fail: %v", err)
	}
	r, _ := NewDatabase(1, 1, Input{Name: "r", Engine: Valkey}, "r", pw)
	if _, err := NewBackup(r, TriggerManual, time.Now()); !errors.Is(err, ErrNoBackups) {
		t.Fatalf("valkey backup: %v", err)
	}
}

func TestPrune(t *testing.T) {
	day := func(n int, s BackupStatus) Backup {
		return Backup{ID: uint64(n), Status: s, StartedAt: at("2026-10-01T03:00:00Z").AddDate(0, 0, n)}
	}
	backups := []Backup{
		day(1, BackupSucceeded), day(2, BackupFailed), day(3, BackupSucceeded),
		day(4, BackupSucceeded), day(5, BackupFailed), day(6, BackupSucceeded), day(7, BackupRunning),
	}
	ids := func(bs []Backup) (out []uint64) {
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

func TestEngineBackupCommands(t *testing.T) {
	for _, e := range Engines {
		d, _ := NewDatabase(1, 1, Input{Name: "x", Engine: e}, "x", pw)
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
	if (Database{Engine: PostgreSQL}).DumpGzip() || !(Database{Engine: MariaDB}).DumpGzip() {
		t.Fatal("DumpGzip")
	}
}
