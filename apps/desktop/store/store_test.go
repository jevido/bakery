package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTripAndMode(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "nested", "bakeries.json"))
	f, err := s.Load()
	if err != nil || len(f.Bakeries) != 0 {
		t.Fatalf("missing file: %+v, %v", f, err)
	}
	a := Bakery{Address: "https://a.example", DesktopID: 1, Key: "bky_desk_a", Member: Member{ID: 7, Name: "Ann", Email: "ann@example.com"}, ConnectedAt: time.Now().UTC().Truncate(time.Second)}
	b := Bakery{Address: "https://b.example", DesktopID: 2, Key: "bky_desk_b"}
	if err := s.Put(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(b); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode %o, want 600", mode)
	}
	f, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := f.Find(a.Address)
	if !ok || got.Key != a.Key || got.Member != a.Member || !got.ConnectedAt.Equal(a.ConnectedAt) {
		t.Fatalf("round trip: %+v", got)
	}
	if f.Active != b.Address || f.Version != 1 {
		t.Fatalf("active %q version %d", f.Active, f.Version)
	}
}

func TestPutReplacesSameAddress(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "bakeries.json"))
	_ = s.Put(Bakery{Address: "https://a.example", Key: "old", SignedOut: true})
	_ = s.Put(Bakery{Address: "https://a.example", Key: "new"})
	f, _ := s.Load()
	if len(f.Bakeries) != 1 || f.Bakeries[0].Key != "new" || f.Bakeries[0].SignedOut {
		t.Fatalf("%+v", f.Bakeries)
	}
}

func TestRemoveMovesActive(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "bakeries.json"))
	_ = s.Put(Bakery{Address: "https://a.example"})
	_ = s.Put(Bakery{Address: "https://b.example"})
	if err := s.Remove("https://b.example"); err != nil {
		t.Fatal(err)
	}
	f, _ := s.Load()
	if len(f.Bakeries) != 1 || f.Active != "https://a.example" {
		t.Fatalf("%+v", f)
	}
	_ = s.Remove("https://a.example")
	f, _ = s.Load()
	if len(f.Bakeries) != 0 || f.Active != "" {
		t.Fatalf("%+v", f)
	}
}

func TestActivateAndMarkSignedOut(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "bakeries.json"))
	_ = s.Put(Bakery{Address: "https://a.example"})
	_ = s.Put(Bakery{Address: "https://b.example"})
	if err := s.Activate("https://nope.example"); err != ErrUnknown {
		t.Fatalf("unknown: %v", err)
	}
	_ = s.Activate("https://a.example")
	_ = s.MarkSignedOut("https://a.example")
	f, _ := s.Load()
	a, _ := f.Find("https://a.example")
	if f.Active != "https://a.example" || !a.SignedOut {
		t.Fatalf("%+v", f)
	}
}

func TestRefusesOtherVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bakeries.json")
	_ = os.WriteFile(path, []byte(`{"version":2,"bakeries":[]}`), 0o600)
	if _, err := New(path).Load(); err == nil {
		t.Fatal("want an error for version 2")
	}
}
