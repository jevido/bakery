package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type memInvitation struct {
	inv  domain.Invitation
	hash string
}

type memInvitations struct {
	mu    sync.Mutex
	store *memStore
	all   []memInvitation
}

func (m *memInvitations) Add(_ context.Context, inv domain.Invitation, hash string) (domain.Invitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.all {
		if x.inv.GuildID == inv.GuildID && x.inv.Email == inv.Email && x.inv.Open(inv.CreatedAt) {
			return domain.Invitation{}, ErrAlreadyInvited
		}
	}
	inv.ID = uint64(len(m.all) + 1)
	m.all = append(m.all, memInvitation{inv: inv, hash: hash})
	return inv, nil
}

func (m *memInvitations) Open(_ context.Context, guildID uint64, now time.Time) ([]domain.Invitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Invitation
	for _, x := range m.all {
		if x.inv.GuildID == guildID && x.inv.Open(now) {
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
	if x, ok := m.find(func(x memInvitation) bool { return x.inv.ID == id }); ok {
		return x.inv, true, nil
	}
	return domain.Invitation{}, false, nil
}

func (m *memInvitations) ByTokenHash(_ context.Context, hash string) (domain.Invitation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if x, ok := m.find(func(x memInvitation) bool { return x.hash == hash }); ok {
		return x.inv, true, nil
	}
	return domain.Invitation{}, false, nil
}

func (m *memInvitations) Revoke(_ context.Context, id uint64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if x, ok := m.find(func(x memInvitation) bool { return x.inv.ID == id }); ok {
		x.inv.RevokedAt = &now
	}
	return nil
}

func (m *memInvitations) Accept(ctx context.Context, hash string, now time.Time, join func(domain.Invitation) (uint64, error)) (domain.Invitation, uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.find(func(x memInvitation) bool { return x.hash == hash })
	if !ok {
		return domain.Invitation{}, 0, ErrInvitationNotFound
	}
	inv := x.inv
	if err := inv.Accept(now); err != nil {
		return domain.Invitation{}, 0, err
	}
	id, err := join(inv)
	if err != nil {
		return domain.Invitation{}, 0, err
	}
	if err := m.store.Add(ctx, inv.Membership(id)); err != nil {
		return domain.Invitation{}, 0, err
	}
	x.inv = inv
	return inv, id, nil
}

// invitingService is twoGuilds with a settable clock and Ada (1, the
// Instance admin), Ann (2) and Dev (3) known by email.
func invitingService(t *testing.T) (*Service, *memStore, *time.Time) {
	t.Helper()
	s, m, members := twoGuilds(t)
	members.emails = map[string]uint64{"ada@example.com": 1, "ann@example.com": 2, "dev@example.com": 3}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	return s, m, &now
}

func TestInviteANewPerson(t *testing.T) {
	ctx := context.Background()
	s, m, now := invitingService(t)

	inv, token, err := s.Invite(ctx, 2, 2, "New@example.com", domain.RoleMember)
	if err != nil || token == "" || inv.Email != "new@example.com" || inv.GuildID != 2 {
		t.Fatalf("invite: %+v %q %v", inv, token, err)
	}
	if _, _, err := s.Invite(ctx, 2, 2, "new@example.com", domain.RoleViewer); !errors.Is(err, ErrAlreadyInvited) {
		t.Errorf("second invite: %v", err)
	}
	if _, _, err := s.Invite(ctx, 1, 1, "new@example.com", domain.RoleViewer); err != nil {
		t.Errorf("the same email into another Guild: %v", err)
	}
	if _, _, err := s.Invite(ctx, 2, 2, "x@example.com", domain.Role("owner")); !errors.Is(err, domain.ErrInvalidRole) {
		t.Errorf("inviting an owner: %v", err)
	}
	if _, err := s.InvitationByToken(ctx, "wrong"); !errors.Is(err, ErrInvitationNotFound) {
		t.Errorf("unknown token: %v", err)
	}
	in, err := s.InvitationByToken(ctx, token)
	if err != nil || in.Guild.Name != "Bakers" || in.ExistingMember {
		t.Fatalf("by token: %+v %v", in, err)
	}

	*now = now.Add(time.Hour)
	if _, _, err := s.AcceptAsNewMember(ctx, token, "New", "short"); err == nil {
		t.Fatal("a refused new Member accepted the Invitation")
	}
	_, id, err := s.AcceptAsNewMember(ctx, token, "New", "correct horse")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if role, ok, _ := m.RoleOf(ctx, 2, id); !ok || role != domain.RoleMember {
		t.Errorf("membership: %v %v", role, ok)
	}
	if _, _, err := s.AcceptAsNewMember(ctx, token, "New", "correct horse"); !errors.Is(err, domain.ErrInvitationUsed) {
		t.Errorf("second accept: %v", err)
	}
}

func TestInviteAnExistingMember(t *testing.T) {
	ctx := context.Background()
	s, m, _ := invitingService(t)
	// Dev (3) is a member of Default and a viewer of Bakers already.
	if _, _, err := s.Invite(ctx, 2, 2, "dev@example.com", domain.RoleAdmin); !errors.Is(err, ErrAlreadyMember) {
		t.Errorf("inviting a member of the Guild: %v", err)
	}
	// Ann (2) is in Bakers only.
	_, token, err := s.Invite(ctx, 1, 1, "ann@example.com", domain.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	in, err := s.InvitationByToken(ctx, token)
	if err != nil || !in.ExistingMember || in.Guild.Name != "Default" {
		t.Fatalf("by token: %+v %v", in, err)
	}
	if _, _, err := s.AcceptAsNewMember(ctx, token, "Ann", "correct horse"); !errors.Is(err, ErrSignInToAccept) {
		t.Errorf("accepting as a new Member: %v", err)
	}
	if _, err := s.AcceptAsMember(ctx, token, 3, "dev@example.com"); !errors.Is(err, ErrNotInvited) {
		t.Errorf("accepting as someone else: %v", err)
	}
	if _, err := s.AcceptAsMember(ctx, token, 2, "ann@example.com"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if role, ok, _ := m.RoleOf(ctx, 1, 2); !ok || role != domain.RoleViewer {
		t.Errorf("membership in Default: %v %v", role, ok)
	}
	if role, ok, _ := m.RoleOf(ctx, 2, 2); !ok || role != domain.RoleAdmin {
		t.Errorf("Ann's Role in Bakers changed: %v %v", role, ok)
	}
}

func TestInvitationsStayInTheirGuild(t *testing.T) {
	ctx := context.Background()
	s, _, now := invitingService(t)
	bakers, _, _ := s.Invite(ctx, 2, 2, "late@example.com", domain.RoleViewer)
	revoked, revokedToken, _ := s.Invite(ctx, 2, 2, "gone@example.com", domain.RoleViewer)
	if err := s.RevokeInvitation(ctx, 1, bakers.ID); !errors.Is(err, ErrInvitationNotFound) {
		t.Errorf("revoking another Guild's Invitation: %v", err)
	}
	if open, _ := s.OpenInvitations(ctx, 1); len(open) != 0 {
		t.Errorf("Default lists Bakers' Invitations: %v", open)
	}
	if err := s.RevokeInvitation(ctx, 2, revoked.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AcceptAsNewMember(ctx, revokedToken, "Gone", "correct horse"); !errors.Is(err, domain.ErrInvitationRevoked) {
		t.Errorf("revoked: %v", err)
	}
	*now = now.Add(domain.InvitationLifetime)
	if open, _ := s.OpenInvitations(ctx, 2); len(open) != 0 {
		t.Errorf("open invitations: %v", open)
	}
}
