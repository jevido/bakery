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

type memAPIToken struct {
	t    domain.APIToken
	hash string
}

type memAPITokens struct {
	mu  sync.Mutex
	all []memAPIToken
}

func (m *memAPITokens) Add(_ context.Context, t domain.APIToken, hash string) (domain.APIToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.all {
		if x.t.MemberID == t.MemberID && x.t.Name == t.Name {
			return domain.APIToken{}, ErrTokenNameTaken
		}
	}
	t.ID = uint64(len(m.all) + 1)
	m.all = append(m.all, memAPIToken{t, hash})
	return t, nil
}

func (m *memAPITokens) ForMember(_ context.Context, memberID uint64) ([]domain.APIToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.APIToken
	for _, x := range m.all {
		if x.t.MemberID == memberID {
			out = append(out, x.t)
		}
	}
	return out, nil
}

func (m *memAPITokens) ByHash(_ context.Context, hash string) (domain.APIToken, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.all {
		if x.hash == hash {
			return x.t, true, nil
		}
	}
	return domain.APIToken{}, false, nil
}

func (m *memAPITokens) Revoke(_ context.Context, memberID, id uint64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, x := range m.all {
		if x.t.ID == id && x.t.MemberID == memberID {
			m.all = append(m.all[:i], m.all[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (m *memAPITokens) Touch(_ context.Context, id uint64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.all {
		if m.all[i].t.ID == id {
			m.all[i].t.LastUsedAt = &now
		}
	}
	return nil
}

func TestAPITokens(t *testing.T) {
	ctx := context.Background()
	s, owner, now := setUpOwner(t)

	tok, value, err := s.CreateAPIToken(ctx, owner.ID, "ci deploy", nil, nil)
	if err != nil || !strings.HasPrefix(value, "bky_") || len(value) < 40 || !tok.ReadOnly() || tok.ExpiresAt != nil {
		t.Fatalf("create: %+v %v", tok, err)
	}
	if _, _, err := s.CreateAPIToken(ctx, owner.ID, "ci deploy", nil, nil); !errors.Is(err, ErrTokenNameTaken) {
		t.Errorf("same name: %v", err)
	}
	m, got, err := s.Authenticate(ctx, value)
	if err != nil || m.ID != owner.ID || m.Role != domain.RoleOwner || got.ID != tok.ID {
		t.Fatalf("authenticate: %+v %+v %v", m, got, err)
	}
	listed, _ := s.APITokensOf(ctx, owner.ID)
	if len(listed) != 1 || listed[0].LastUsedAt == nil || !listed[0].LastUsedAt.Equal(*now) {
		t.Errorf("last used not recorded: %+v", listed)
	}

	for _, bad := range []string{"", "bky_nope", strings.TrimPrefix(value, "bky_")} {
		if _, _, err := s.Authenticate(ctx, bad); !errors.Is(err, ErrInvalidAPIToken) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if err := s.RevokeAPIToken(ctx, owner.ID+1, tok.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("revoking someone else's token: %v", err)
	}
	if err := s.RevokeAPIToken(ctx, owner.ID, tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, value); !errors.Is(err, ErrInvalidAPIToken) {
		t.Errorf("revoked token: %v", err)
	}
}

func TestAPITokenExpiry(t *testing.T) {
	ctx := context.Background()
	s, owner, now := setUpOwner(t)

	week := 7
	tok, value, err := s.CreateAPIToken(ctx, owner.ID, "a week", []domain.Permission{domain.PermissionRoot}, &week)
	if err != nil || tok.ExpiresAt == nil || !tok.ExpiresAt.Equal(now.AddDate(0, 0, 7)) {
		t.Fatalf("create: %+v %v", tok, err)
	}
	odd := 3
	if _, _, err := s.CreateAPIToken(ctx, owner.ID, "three days", nil, &odd); !errors.Is(err, ErrInvalidExpiry) {
		t.Errorf("3 days: %v", err)
	}

	*now = now.AddDate(0, 0, 7).Add(-time.Second)
	if _, _, err := s.Authenticate(ctx, value); err != nil {
		t.Fatalf("just before expiry: %v", err)
	}
	listed, _ := s.APITokensOf(ctx, owner.ID)
	if listed[0].LastUsedAt == nil || !listed[0].LastUsedAt.Equal(*now) {
		t.Errorf("last used not recorded: %+v", listed)
	}
	*now = now.Add(time.Second)
	if _, _, err := s.Authenticate(ctx, value); !errors.Is(err, ErrInvalidAPIToken) {
		t.Errorf("expired token: %v", err)
	}
}

func TestAPITokenRoleCaps(t *testing.T) {
	ctx := context.Background()
	s, owner, _ := setUpOwner(t)
	_, invite, _ := s.Invite(ctx, owner.ID, "dev@example.com", domain.RoleMember)
	dev, _ := s.AcceptInvitation(ctx, invite, "Dev", "correct horse")
	if _, _, err := s.CreateAPIToken(ctx, dev.ID, "root", []domain.Permission{domain.PermissionRoot}, nil); !errors.Is(err, domain.ErrRoleCannotGrant) {
		t.Errorf("member granting root: %v", err)
	}
	if _, _, err := s.CreateAPIToken(ctx, dev.ID, "deploy", []domain.Permission{domain.PermissionDeploy}, nil); err != nil {
		t.Errorf("member granting deploy: %v", err)
	}
}

func TestAPITokenOfRemovedMember(t *testing.T) {
	ctx := context.Background()
	s, owner, _ := setUpOwner(t)
	_, invite, _ := s.Invite(ctx, owner.ID, "dev@example.com", domain.RoleMember)
	dev, _ := s.AcceptInvitation(ctx, invite, "Dev", "correct horse")
	_, value, _ := s.CreateAPIToken(ctx, dev.ID, "ci deploy", []domain.Permission{domain.PermissionWrite}, nil)
	if err := s.RemoveMember(ctx, owner.ID, dev.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, value); !errors.Is(err, ErrInvalidAPIToken) {
		t.Errorf("token of removed member: %v", err)
	}
}
