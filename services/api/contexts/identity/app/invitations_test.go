package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

type memInvitation struct {
	inv  domain.Invitation
	hash string
}

type memInvitations struct {
	mu      sync.Mutex
	members *memMembers
	all     []memInvitation
}

func (m *memInvitations) Add(_ context.Context, inv domain.Invitation, hash string) (domain.Invitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.all {
		if x.inv.Email == inv.Email && x.inv.Open(inv.CreatedAt) {
			return domain.Invitation{}, ErrAlreadyInvited
		}
	}
	inv.ID = uint64(len(m.all) + 1)
	m.all = append(m.all, memInvitation{inv, hash})
	return inv, nil
}

func (m *memInvitations) Open(_ context.Context, now time.Time) ([]domain.Invitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Invitation
	for _, x := range m.all {
		if x.inv.Open(now) {
			out = append(out, x.inv)
		}
	}
	return out, nil
}

func (m *memInvitations) find(match func(memInvitation) bool) (*memInvitation, bool) {
	for i := range m.all {
		if match(m.all[i]) {
			return &m.all[i], true
		}
	}
	return nil, false
}

func (m *memInvitations) ByID(_ context.Context, id uint64) (domain.Invitation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.find(func(x memInvitation) bool { return x.inv.ID == id })
	if !ok {
		return domain.Invitation{}, false, nil
	}
	return x.inv, true, nil
}

func (m *memInvitations) ByTokenHash(_ context.Context, hash string) (domain.Invitation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.find(func(x memInvitation) bool { return x.hash == hash })
	if !ok {
		return domain.Invitation{}, false, nil
	}
	return x.inv, true, nil
}

func (m *memInvitations) Revoke(_ context.Context, id uint64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if x, ok := m.find(func(x memInvitation) bool { return x.inv.ID == id }); ok {
		x.inv.RevokedAt = &now
	}
	return nil
}

func (m *memInvitations) Accept(ctx context.Context, hash string, member domain.Member, now time.Time) (domain.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.find(func(x memInvitation) bool { return x.hash == hash })
	if !ok {
		return domain.Member{}, ErrInvitationNotFound
	}
	if err := x.inv.Accept(now); err != nil {
		return domain.Member{}, err
	}
	if _, found, _ := m.members.ByEmail(ctx, member.Email); found {
		return domain.Member{}, ErrAlreadyMember
	}
	return m.members.add(member), nil
}

func setUpOwner(t *testing.T) (*Service, domain.Member, *time.Time) {
	t.Helper()
	s := newTestService()
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	owner, err := s.SetupOwner(context.Background(), "Ada", "ada@example.com", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	return s, owner, &now
}

func TestInviteAndAccept(t *testing.T) {
	ctx := context.Background()
	s, owner, now := setUpOwner(t)

	inv, token, err := s.Invite(ctx, owner.ID, "dev@example.com", domain.RoleMember)
	if err != nil || token == "" || inv.Email != "dev@example.com" {
		t.Fatalf("invite: %+v %q %v", inv, token, err)
	}
	if _, _, err := s.Invite(ctx, owner.ID, "dev@example.com", domain.RoleViewer); !errors.Is(err, ErrAlreadyInvited) {
		t.Errorf("second invite: %v", err)
	}
	if _, _, err := s.Invite(ctx, owner.ID, "ada@example.com", domain.RoleViewer); !errors.Is(err, ErrAlreadyMember) {
		t.Errorf("inviting a member: %v", err)
	}
	if _, _, err := s.Invite(ctx, owner.ID, "x@example.com", domain.RoleOwner); !errors.Is(err, domain.ErrInvitationRole) {
		t.Errorf("inviting an owner: %v", err)
	}
	if _, err := s.InvitationByToken(ctx, "wrong"); !errors.Is(err, ErrInvitationNotFound) {
		t.Errorf("unknown token: %v", err)
	}

	*now = now.Add(time.Hour)
	m, err := s.AcceptInvitation(ctx, token, "Dev", "correct horse")
	if err != nil || m.Role != domain.RoleMember || m.Email != "dev@example.com" {
		t.Fatalf("accept: %+v %v", m, err)
	}
	if _, err := s.AcceptInvitation(ctx, token, "Dev", "correct horse"); !errors.Is(err, domain.ErrInvitationUsed) {
		t.Errorf("second accept: %v", err)
	}
	if _, err := s.Login(ctx, "dev@example.com", "correct horse"); err != nil {
		t.Errorf("invited member cannot log in: %v", err)
	}
}

func TestInvitationExpiresAndRevokes(t *testing.T) {
	ctx := context.Background()
	s, owner, now := setUpOwner(t)
	_, late, _ := s.Invite(ctx, owner.ID, "late@example.com", domain.RoleViewer)
	revoked, revokedToken, _ := s.Invite(ctx, owner.ID, "gone@example.com", domain.RoleViewer)
	if err := s.RevokeInvitation(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptInvitation(ctx, revokedToken, "Gone", "correct horse"); !errors.Is(err, domain.ErrInvitationRevoked) {
		t.Errorf("revoked: %v", err)
	}
	*now = now.Add(domain.InvitationLifetime)
	if _, err := s.AcceptInvitation(ctx, late, "Late", "correct horse"); !errors.Is(err, domain.ErrInvitationExpired) {
		t.Errorf("expired: %v", err)
	}
	if open, _ := s.OpenInvitations(ctx); len(open) != 0 {
		t.Errorf("open invitations: %v", open)
	}
}

func TestChangeRoleAndRemove(t *testing.T) {
	ctx := context.Background()
	s, owner, _ := setUpOwner(t)
	_, token, _ := s.Invite(ctx, owner.ID, "admin@example.com", domain.RoleAdmin)
	admin, _ := s.AcceptInvitation(ctx, token, "Admin", "correct horse")
	_, token, _ = s.Invite(ctx, owner.ID, "dev@example.com", domain.RoleViewer)
	dev, _ := s.AcceptInvitation(ctx, token, "Dev", "correct horse")

	if m, err := s.ChangeRole(ctx, admin.ID, dev.ID, domain.RoleMember); err != nil || m.Role != domain.RoleMember {
		t.Fatalf("admin promotes viewer: %+v %v", m, err)
	}
	if _, err := s.ChangeRole(ctx, admin.ID, owner.ID, domain.RoleAdmin); !errors.Is(err, domain.ErrOwnerIsFixed) {
		t.Errorf("admin demotes owner: %v", err)
	}
	if _, err := s.ChangeRole(ctx, admin.ID, admin.ID, domain.RoleMember); !errors.Is(err, domain.ErrSelf) {
		t.Errorf("admin demotes self: %v", err)
	}
	if _, err := s.ChangeRole(ctx, owner.ID, dev.ID, domain.RoleOwner); !errors.Is(err, domain.ErrGrantOwner) {
		t.Errorf("make owner: %v", err)
	}
	if _, err := s.ChangeRole(ctx, dev.ID, admin.ID, domain.RoleViewer); !errors.Is(err, domain.ErrNotAdmin) {
		t.Errorf("member demotes admin: %v", err)
	}
	if err := s.RemoveMember(ctx, owner.ID, owner.ID); !errors.Is(err, domain.ErrOwnerIsFixed) {
		t.Errorf("owner removes self: %v", err)
	}
	if err := s.RemoveMember(ctx, admin.ID, dev.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CurrentMember(ctx, dev.ID); !errors.Is(err, ErrMemberNotFound) {
		t.Errorf("removed member still found: %v", err)
	}
}
