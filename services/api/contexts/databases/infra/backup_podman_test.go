//go:build podman

package infra

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

// A real PostgreSQL is dumped into a Backups directory and restored from it.
func TestPostgresBackupAndRestore(t *testing.T) {
	r := runtime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	d := testDatabase(t, 990201, domain.PostgreSQL)
	d.Slug = "test-backup-postgresql"
	defer r.Remove(context.Background(), d, true)
	if err := r.Start(ctx, d); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, ctx, r, d, domain.StatusRunning)
	psql := func(sql string) string {
		t.Helper()
		code, out, err := r.Exec(ctx, d, []string{"sh", "-c", `PGPASSWORD="$POSTGRES_PASSWORD" psql -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "$0"`, sql})
		if err != nil || code != 0 {
			t.Fatalf("psql %q: %d %v %s", sql, code, err, out)
		}
		return strings.TrimSpace(out)
	}
	psql("CREATE TABLE notes (body text); INSERT INTO notes VALUES ('kept')")

	files := BackupFiles{Dir: t.TempDir()}
	size, err := files.Write(d.ID, "a"+d.BackupExt(), func(w io.Writer) error {
		code, stderr, err := r.Dump(ctx, d, w)
		if err == nil && code != 0 {
			t.Fatalf("dump exited %d: %s", code, stderr)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := files.Open(d.ID, "a"+d.BackupExt())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	head := make([]byte, 5)
	f.ReadAt(head, 0)
	if !bytes.Equal(head, []byte("PGDMP")) || f.Size() != size || size < 100 {
		t.Fatalf("backup: head %q, size %d/%d", head, f.Size(), size)
	}

	psql("DROP TABLE notes")
	if err := r.CopyIn(ctx, d, d.RestoreFile(), f.Size(), io.NewSectionReader(f, 0, f.Size())); err != nil {
		t.Fatal(err)
	}
	if code, out, err := r.Exec(ctx, d, d.RestoreCommand()); err != nil || code != 0 {
		t.Fatalf("restore: %d %v %s", code, err, out)
	}
	if got := psql("SELECT body FROM notes"); got != "kept" {
		t.Fatalf("after restore: %q", got)
	}
	// The restore removes its copy of the file.
	if code, _, _ := r.Exec(ctx, d, []string{"test", "-e", d.RestoreDir() + "/" + d.RestoreFile()}); code == 0 {
		t.Fatal("restore file left in the container")
	}
	// Restoring over data that is still there works too (--clean).
	psql("INSERT INTO notes VALUES ('after')")
	r.CopyIn(ctx, d, d.RestoreFile(), f.Size(), io.NewSectionReader(f, 0, f.Size()))
	if code, out, err := r.Exec(ctx, d, d.RestoreCommand()); err != nil || code != 0 {
		t.Fatalf("restore over data: %d %v %s", code, err, out)
	}
	if got := psql("SELECT count(*) FROM notes"); got != "1" {
		t.Fatalf("after second restore: %q rows", got)
	}
}
