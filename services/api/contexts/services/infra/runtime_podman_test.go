//go:build podman

package infra

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/app/podman"
	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

// An id no real Service has, so the test never touches one.
const testServiceID = 990001

const testCompose = `services:
  web:
    image: docker.io/library/busybox
    command: ["httpd", "-f", "-p", "8080", "-h", "/www"]
    environment:
      - SERVICE_FQDN_WEB_8080
    volumes:
      - www:/www
    depends_on: [db]
  db:
    image: docker.io/library/busybox
    entrypoint: ["sh", "-c"]
    command: ["trap 'exit 0' TERM; while :; do sleep 1; done"]
    working_dir: /tmp
`

func TestRuntimeUpAndRemove(t *testing.T) {
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	c := podman.New(sock)
	r := Runtime{Podman: c, Network: "bakery-test"}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	s, err := domain.NewService(1, 1, "test", "test-svc", testCompose, "localhost", Generate)
	if err != nil {
		t.Fatal(err)
	}
	s.ID = testServiceID
	t.Cleanup(func() { r.Remove(context.Background(), s) })
	resolved, err := s.Resolved()
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Up(ctx, s, resolved, false); err != nil {
		t.Fatalf("Up: %v", err)
	}
	statuses, _, err := r.Statuses(ctx, s)
	if err != nil || statuses["web"] != domain.StatusRunning || statuses["db"] != domain.StatusRunning {
		t.Fatalf("statuses %v %v", statuses, err)
	}
	web := domain.ContainerName(s.ID, "web")
	code, out, err := c.Exec(ctx, web, []string{"sh", "-c", "echo kept > /www/index.html; ping -c1 -W1 db 2>&1 | head -1; echo $SERVICE_FQDN_WEB_8080"})
	if err != nil || code != 0 || !strings.Contains(out, "PING db (") || !strings.Contains(out, "test-svc.localhost") {
		t.Fatalf("exec in web: %d %v\n%s", code, err, out)
	}
	if code, out, _ := c.Exec(ctx, domain.ContainerName(s.ID, "db"), []string{"pwd"}); code != 0 || strings.TrimSpace(out) != "/tmp" {
		t.Errorf("db working dir: %q", out)
	}

	// A second Up replaces the Containers and keeps the volume.
	if err := r.Up(ctx, s, resolved, false); err != nil {
		t.Fatalf("second Up: %v", err)
	}
	if code, out, _ := c.Exec(ctx, web, []string{"cat", "/www/index.html"}); code != 0 || strings.TrimSpace(out) != "kept" {
		t.Errorf("volume content after Up: %d %q", code, out)
	}

	if err := r.Down(ctx, s); err != nil {
		t.Fatal(err)
	}
	s.DesiredState = domain.Stopped
	if statuses, _, _ := r.Statuses(ctx, s); statuses["web"] != domain.StatusStopped {
		t.Errorf("after Down: %v", statuses)
	}

	if err := r.Remove(ctx, s); err != nil {
		t.Fatal(err)
	}
	l := labels(s.ID)
	if list, _ := c.ListContainers(ctx, l); len(list) != 0 {
		t.Errorf("containers left: %v", list)
	}
	if list, _ := c.ListNetworks(ctx, l); len(list) != 0 {
		t.Errorf("networks left: %v", list)
	}
	if list, _ := c.ListVolumes(ctx, l); len(list) != 0 {
		t.Errorf("volumes left: %v", list)
	}
}

func TestRuntimeReportsAComponentThatExits(t *testing.T) {
	sock := podman.DefaultSocket()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no podman socket at %s", sock)
	}
	r := Runtime{Podman: podman.New(sock), Network: "bakery-test"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s, err := domain.NewService(1, 1, "crash", "test-crash", "services:\n  app:\n    image: docker.io/library/busybox\n    command: [\"sh\", \"-c\", \"echo no config; exit 3\"]\n", "localhost", Generate)
	if err != nil {
		t.Fatal(err)
	}
	s.ID = testServiceID + 1
	t.Cleanup(func() { r.Remove(context.Background(), s) })
	resolved, _ := s.Resolved()
	err = r.Up(ctx, s, resolved, false)
	if err == nil || !strings.Contains(err.Error(), "exited with code 3") || !strings.Contains(err.Error(), "no config") {
		t.Fatalf("Up: %v", err)
	}
}
