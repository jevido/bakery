package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/app/secret"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type memDesktopSignIns struct {
	mu       sync.Mutex
	signIns  []domain.DesktopSignIn
	hashes   []string
	desktops []domain.Desktop
}

func (m *memDesktopSignIns) Add(_ context.Context, s domain.DesktopSignIn, secretHash string) (domain.DesktopSignIn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.ID = uint64(len(m.signIns) + 1)
	m.signIns = append(m.signIns, s)
	m.hashes = append(m.hashes, secretHash)
	return s, nil
}

func (m *memDesktopSignIns) ByID(_ context.Context, id uint64) (domain.DesktopSignIn, string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == 0 || id > uint64(len(m.signIns)) {
		return domain.DesktopSignIn{}, "", false, nil
	}
	return m.signIns[id-1], m.hashes[id-1], true, nil
}

func (m *memDesktopSignIns) Change(_ context.Context, id uint64, change func(*domain.DesktopSignIn, string) (*domain.Desktop, error)) (domain.DesktopSignIn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == 0 || id > uint64(len(m.signIns)) {
		return domain.DesktopSignIn{}, ErrDesktopSignInNotFound
	}
	s := m.signIns[id-1]
	d, err := change(&s, m.hashes[id-1])
	if err != nil {
		return domain.DesktopSignIn{}, err
	}
	if d != nil {
		d.ID = uint64(len(m.desktops) + 1)
		m.desktops = append(m.desktops, *d)
		s.DesktopID = &d.ID
	}
	m.signIns[id-1] = s
	return s, nil
}

func (m *memDesktopSignIns) DesktopByKeyHash(_ context.Context, keyHash string) (domain.Desktop, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.desktops {
		if d.KeyHash == keyHash {
			return d, true, nil
		}
	}
	return domain.Desktop{}, false, nil
}

func (m *memDesktopSignIns) DesktopNames(_ context.Context, ids []uint64) (map[uint64]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[uint64]string{}
	for _, d := range m.desktops {
		if slices.Contains(ids, d.ID) {
			out[d.ID] = d.Name
		}
	}
	return out, nil
}

func (m *memDesktopSignIns) DesktopsOf(_ context.Context, memberID uint64) ([]domain.Desktop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Desktop
	for i := len(m.desktops) - 1; i >= 0; i-- {
		if m.desktops[i].MemberID == memberID {
			out = append(out, m.desktops[i])
		}
	}
	return out, nil
}

func (m *memDesktopSignIns) TouchDesktop(_ context.Context, id uint64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.desktops[id-1].LastSeenAt = &at
	return nil
}

func (m *memDesktopSignIns) SignOutDesktop(_ context.Context, id, memberID uint64, at time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == 0 || id > uint64(len(m.desktops)) || m.desktops[id-1].MemberID != memberID {
		return false, nil
	}
	m.desktops[id-1].SignOut(at)
	return true, nil
}

var (
	testSignInSecret = "bky_signin_" + strings.Repeat("1a", 24)
	testKeyHash      = strings.Repeat("cd", 32)
)

func TestApproveDesktopSignIn(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	signIns := s.desktopSignIns.(*memDesktopSignIns)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	admin, err := s.SetUp(ctx, "Owner", "owner@example.com", "a long password")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.StartDesktopSignIn(ctx, "laptop", "bky_signin_short", testKeyHash); !errors.Is(err, domain.ErrInvalidDesktopSignInKey) {
		t.Fatalf("short secret: %v", err)
	}
	in, err := s.StartDesktopSignIn(ctx, "laptop", testSignInSecret, testKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	if signIns.hashes[0] == testSignInSecret {
		t.Fatal("secret stored as is")
	}
	if _, err := s.DescribeDesktopSignIn(ctx, in.ID, "bky_signin_"+strings.Repeat("00", 24)); !errors.Is(err, ErrDesktopSignInNotFound) {
		t.Fatalf("wrong secret: %v", err)
	}
	if _, err := s.ApproveDesktopSignIn(ctx, in.ID, "wrong", admin.ID); !errors.Is(err, ErrDesktopSignInNotFound) {
		t.Fatalf("approve with wrong secret: %v", err)
	}

	// Approving twice makes one Desktop and answers the same.
	first, err := s.ApproveDesktopSignIn(ctx, in.ID, testSignInSecret, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ApproveDesktopSignIn(ctx, in.ID, testSignInSecret, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(signIns.desktops) != 1 || *first.DesktopID != *second.DesktopID || s.DesktopSignInStatus(second) != domain.DesktopSignInApproved {
		t.Fatalf("desktops %+v, first %+v, second %+v", signIns.desktops, first, second)
	}
	if d := signIns.desktops[0]; d.MemberID != admin.ID || d.KeyHash != testKeyHash || d.Name != "laptop" {
		t.Fatalf("desktop %+v", d)
	}
	v, err := s.DescribeDesktopSignIn(ctx, in.ID, testSignInSecret)
	if err != nil || v.Status != domain.DesktopSignInApproved || v.ApprovedBy == nil || v.ApprovedBy.ID != admin.ID {
		t.Fatalf("describe: %+v %v", v, err)
	}
	if _, err := s.CancelDesktopSignIn(ctx, in.ID, testSignInSecret); !errors.Is(err, domain.ErrDesktopSignInApproved) {
		t.Fatalf("cancel approved: %v", err)
	}

	// An expired one is refused and makes no Desktop.
	late, err := s.StartDesktopSignIn(ctx, "other", testSignInSecret, testKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(domain.DesktopSignInLasts)
	if _, err := s.ApproveDesktopSignIn(ctx, late.ID, testSignInSecret, admin.ID); !errors.Is(err, domain.ErrDesktopSignInExpired) {
		t.Fatalf("approve expired: %v", err)
	}
	if len(signIns.desktops) != 1 {
		t.Fatalf("expired sign-in made a desktop: %+v", signIns.desktops)
	}
}

func TestAuthenticateDesktop(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	signIns := s.desktopSignIns.(*memDesktopSignIns)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	admin, err := s.SetUp(ctx, "Owner", "owner@example.com", "a long password")
	if err != nil {
		t.Fatal(err)
	}
	key := domain.DesktopKeyPrefix + strings.Repeat("ab", 24)
	approve := func() domain.Desktop {
		t.Helper()
		in, err := s.StartDesktopSignIn(ctx, "laptop", testSignInSecret, secret.Hash(key))
		if err != nil {
			t.Fatal(err)
		}
		in, err = s.ApproveDesktopSignIn(ctx, in.ID, testSignInSecret, admin.ID)
		if err != nil {
			t.Fatal(err)
		}
		return signIns.desktops[*in.DesktopID-1]
	}
	d := approve()

	if _, _, err := s.AuthenticateDesktop(ctx, "bky_"+strings.Repeat("ab", 24)); !errors.Is(err, ErrInvalidDesktopKey) {
		t.Fatalf("an API token's shape: %v", err)
	}
	if _, _, err := s.AuthenticateDesktop(ctx, domain.DesktopKeyPrefix+"unknown"); !errors.Is(err, ErrInvalidDesktopKey) {
		t.Fatalf("unknown key: %v", err)
	}
	m, got, err := s.AuthenticateDesktop(ctx, key)
	if err != nil || m.ID != admin.ID || got.ID != d.ID || got.LastSeenAt == nil || !got.LastSeenAt.Equal(now) {
		t.Fatalf("authenticate: %+v %+v %v", m, got, err)
	}
	// Last seen is written at most once a minute.
	seen := now
	now = now.Add(30 * time.Second)
	if _, _, err := s.AuthenticateDesktop(ctx, key); err != nil || !signIns.desktops[d.ID-1].LastSeenAt.Equal(seen) {
		t.Fatalf("touched within a minute: %v %v", signIns.desktops[d.ID-1].LastSeenAt, err)
	}

	// Another Member cannot sign it out; its own Member can.
	if err := s.SignOutDesktop(ctx, admin.ID+1, d.ID); !errors.Is(err, ErrDesktopNotFound) {
		t.Fatalf("another's: %v", err)
	}
	if ds, err := s.Desktops(ctx, admin.ID); err != nil || len(ds) != 1 {
		t.Fatalf("desktops: %+v %v", ds, err)
	}
	if err := s.SignOutDesktop(ctx, admin.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AuthenticateDesktop(ctx, key); !errors.Is(err, ErrInvalidDesktopKey) {
		t.Fatalf("signed out: %v", err)
	}
	if ds, err := s.Desktops(ctx, admin.ID); err != nil || len(ds) != 0 {
		t.Fatalf("a signed-out Desktop is listed: %+v %v", ds, err)
	}

	// "Sign out everywhere else" ends a Desktop made before it.
	key = domain.DesktopKeyPrefix + strings.Repeat("cd", 24)
	approve()
	if _, _, err := s.AuthenticateDesktop(ctx, key); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	if err := s.members.SetSessionsValidFrom(ctx, admin.ID, domain.SessionsValidFromNow(now)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AuthenticateDesktop(ctx, key); !errors.Is(err, ErrInvalidDesktopKey) {
		t.Fatalf("after the Sessions ended: %v", err)
	}
}
