package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

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
