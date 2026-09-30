//go:build garage

package infra

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

// Runs against the Garage stand-in: task s3:up && task api:test:s3.

func garageStorage(t *testing.T) domain.S3Storage {
	t.Helper()
	dir, _ := os.Getwd()
	for dir != "/" && !exists(filepath.Join(dir, ".git")) {
		dir = filepath.Dir(dir)
	}
	f, err := os.Open(filepath.Join(dir, ".claude/ralph/state/garage.env"))
	if err != nil {
		t.Skipf("no garage.env (task s3:up): %v", err)
	}
	defer f.Close()
	env := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "="); ok {
			env[k] = v
		}
	}
	return domain.S3Storage{
		Name: "garage", Endpoint: env["GARAGE_ENDPOINT"], Region: env["GARAGE_REGION"], Bucket: env["GARAGE_BUCKET"],
		AccessKey: env["GARAGE_ACCESS_KEY"], SecretKey: env["GARAGE_SECRET_KEY"],
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestGarage(t *testing.T) {
	st := garageStorage(t)
	c := S3{Storage: st}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := c.Check(ctx); err != nil {
		t.Fatalf("Check: %v", err)
	}

	prefix := "bakery-test/" + strconv.FormatInt(time.Now().UnixNano(), 36) + "/"
	payload := bytes.Repeat([]byte("dump\x00\xff"), 200_000)
	path := filepath.Join(t.TempDir(), "a b.dump")
	os.WriteFile(path, payload, 0o600)
	f, _ := os.Open(path)
	defer f.Close()
	key := prefix + "20260930T030000Z a+b.dump"
	if err := c.Put(ctx, key, f, int64(len(payload))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	keys, err := c.List(ctx, prefix)
	if err != nil || len(keys) != 1 || keys[0] != key {
		t.Fatalf("List: %v %v", keys, err)
	}
	r, size, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, payload) || size != int64(len(payload)) {
		t.Fatalf("Get: %d bytes (size %d), want %d", len(got), size, len(payload))
	}
	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("Delete again: %v", err)
	}
	if keys, err := c.List(ctx, prefix); err != nil || len(keys) != 0 {
		t.Fatalf("List after delete: %v %v", keys, err)
	}
	if _, _, err := c.Get(ctx, key); !IsNoSuchKey(err) {
		t.Fatalf("Get missing: %v", err)
	}

	bad := c
	bad.Storage.SecretKey = "wrong"
	if _, err := bad.List(ctx, prefix); err == nil || !strings.Contains(err.Error(), "Signature") && !strings.Contains(err.Error(), "AccessDenied") {
		t.Fatalf("wrong secret: %v", err)
	}
	if err := bad.Check(ctx); err == nil {
		t.Fatal("Check with a wrong secret passed")
	}
	missing := c
	missing.Storage.Bucket = "bakery-no-such-bucket"
	if err := missing.Check(ctx); err == nil {
		t.Fatal("Check of a missing bucket passed")
	} else {
		t.Logf("missing bucket: %v; wrong secret: %v", err, bad.Check(ctx))
	}
}
