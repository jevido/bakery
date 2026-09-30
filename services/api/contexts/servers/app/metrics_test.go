package app

import (
	"context"
	"errors"
	"testing"
)

func TestMetrics(t *testing.T) {
	conn := healthy()
	conn.cpuErr = errors.New("no /proc/stat")
	s := NewService(newMemStore(), fakeKey, fakeConnector{conn: conn})
	var logged []string
	s.Log = func(format string, _ ...any) { logged = append(logged, format) }
	local, _ := s.EnsureLocal(context.Background())
	m, err := s.Metrics(context.Background(), local.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Server.MemUsed != 6<<30 || m.Server.DiskUsed != 60<<30 || m.Server.CPUs != 4 || m.Server.CPUPercent != 0 {
		t.Fatalf("%+v", m.Server)
	}
	if len(logged) != 1 {
		t.Fatalf("a failed part was not logged: %v", logged)
	}
	if len(m.Containers) != 2 || m.Containers[0].Name != "bakery-app-3-7" || m.Containers[0].Owner != "application" || m.Containers[0].OwnerID != "3" || m.Containers[1].Owner != "proxy" {
		t.Fatalf("%+v", m.Containers)
	}
	if !conn.closed {
		t.Fatal("connection left open")
	}
}

func TestMetricsUnreachable(t *testing.T) {
	s := NewService(newMemStore(), fakeKey, fakeConnector{err: errors.New("refused")})
	local, _ := s.EnsureLocal(context.Background())
	_, err := s.Metrics(context.Background(), local.ID)
	var u *ErrUnreachable
	if !errors.As(err, &u) {
		t.Fatalf("err %v", err)
	}
}
