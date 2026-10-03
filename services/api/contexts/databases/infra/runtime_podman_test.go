//go:build podman

package infra

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

func runtime(t *testing.T) Runtime {
	t.Helper()
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	c := podman.New(sock)
	if err := c.EnsureNetwork(context.Background(), "bakery-test"); err != nil {
		t.Fatal(err)
	}
	return Runtime{Podman: c, Network: "bakery-test", PublicBind: "127.0.0.1"}
}

func testDatabase(t *testing.T, id uint64, typ domain.DatabaseType) domain.Database {
	t.Helper()
	slug := "test-" + string(typ)
	d, err := domain.NewDatabase(1, 1, domain.Input{Name: slug, Type: typ}, slug, func() string { return "secret-" + slug })
	if err != nil {
		t.Fatal(err)
	}
	d.ID = id
	return d
}

func waitStatus(t *testing.T, ctx context.Context, r Runtime, d domain.Database, want domain.Status) {
	t.Helper()
	var got domain.Status
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
		s, detail, err := r.Status(ctx, d)
		if err != nil {
			t.Fatal(err)
		}
		if got = s; got == want {
			return
		}
		if s == domain.StatusExited {
			t.Fatalf("%s %s", d.Type, detail)
		}
	}
	t.Fatalf("%s: status %s, want %s", d.Type, got, want)
}

func TestPostgresLifecycle(t *testing.T) {
	r := runtime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	d := testDatabase(t, 990101, domain.PostgreSQL)
	defer r.Remove(context.Background(), d)

	if s, _, err := r.Status(ctx, d); err != nil || s != domain.StatusMissing {
		t.Fatalf("before start: %s %v", s, err)
	}
	if err := r.Start(ctx, d); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, ctx, r, d, domain.StatusRunning)

	psql := func(sql string) string {
		t.Helper()
		code, out, err := r.Podman.Exec(ctx, domain.ContainerName(d.Slug), []string{"sh", "-c", `PGPASSWORD="$POSTGRES_PASSWORD" psql -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "$0"`, sql})
		if err != nil || code != 0 {
			t.Fatalf("psql %q: %d %v %s", sql, code, err, out)
		}
		return strings.TrimSpace(out)
	}
	psql("CREATE TABLE kept (v text); INSERT INTO kept VALUES ('still here')")

	d.PublicPort = 54990
	if err := r.Recreate(ctx, d); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, ctx, r, d, domain.StatusRunning)
	if got := psql("SELECT v FROM kept"); got != "still here" {
		t.Fatalf("after recreate: %q", got)
	}
	info, err := r.Podman.InspectContainer(ctx, domain.ContainerName(d.Slug))
	if err != nil {
		t.Fatal(err)
	}
	if b := info.HostConfig.PortBindings["5432/tcp"]; len(b) != 1 || b[0].HostPort != "54990" || b[0].HostIP != "127.0.0.1" {
		t.Fatalf("port bindings %+v", info.HostConfig.PortBindings)
	}
	var lines int
	if found, err := r.Logs(ctx, d, false, 10, func(string, string) { lines++ }); !found || err != nil || lines == 0 {
		t.Fatalf("logs: found %v, %d lines, %v", found, lines, err)
	}

	d.DesiredState = domain.Stopped
	if err := r.Stop(ctx, d); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := r.Status(ctx, d); s != domain.StatusStopped {
		t.Fatalf("after stop: %s", s)
	}
	if found, _ := r.Logs(ctx, d, false, 10, func(string, string) {}); found {
		t.Fatal("logs of a stopped database")
	}

	if err := r.Remove(ctx, d); err != nil {
		t.Fatal(err)
	}
	vols, err := r.Podman.ListVolumes(ctx, map[string]string{"bakery.database": "990101"})
	if err != nil || len(vols) != 0 {
		t.Fatalf("volumes left: %v %v", vols, err)
	}
}

func TestRedisRuns(t *testing.T) {
	r := runtime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	d := testDatabase(t, 990102, domain.Redis)
	defer r.Remove(context.Background(), d)
	if err := r.Start(ctx, d); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, ctx, r, d, domain.StatusRunning)
	info, err := r.Podman.InspectContainer(ctx, domain.ContainerName(d.Slug))
	if err != nil {
		t.Fatal(err)
	}
	// The entrypoint drops root: redis-server does not run as uid 0.
	code, out, err := r.Podman.Exec(ctx, info.ID, []string{"sh", "-c", "cat /proc/1/status | grep '^Uid:'"})
	if err != nil || code != 0 || strings.Contains(out, "\t0\t") {
		t.Fatalf("redis runs as root: %s %v", out, err)
	}
}
