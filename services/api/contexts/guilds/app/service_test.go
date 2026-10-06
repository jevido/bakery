package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

type memStore struct {
	mu          sync.Mutex
	guilds      []domain.Guild
	memberships []domain.Membership
}

func (m *memStore) ByID(_ context.Context, id uint64) (domain.Guild, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, g := range m.guilds {
		if g.ID == id {
			return g, true, nil
		}
	}
	return domain.Guild{}, false, nil
}

func (m *memStore) All(context.Context) ([]domain.Guild, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Guild(nil), m.guilds...), nil
}

func (m *memStore) Create(_ context.Context, g domain.Guild, adminID uint64) (domain.Guild, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.create(g, adminID), nil
}

func (m *memStore) create(g domain.Guild, adminID uint64) domain.Guild {
	g.ID = uint64(len(m.guilds) + 1)
	m.guilds = append(m.guilds, g)
	m.memberships = append(m.memberships, domain.Membership{ID: uint64(len(m.memberships) + 1), GuildID: g.ID, MemberID: adminID, Role: domain.RoleAdmin})
	return g
}

func (m *memStore) CreateFirstIfNone(_ context.Context, g domain.Guild, adminID uint64) (domain.Guild, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.guilds) > 0 {
		return domain.Guild{}, false, nil
	}
	return m.create(g, adminID), true, nil
}

func (m *memStore) Update(_ context.Context, g domain.Guild) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.guilds {
		if m.guilds[i].ID == g.ID {
			m.guilds[i] = g
		}
	}
	return nil
}

func (m *memStore) Delete(_ context.Context, id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.guilds = slices.DeleteFunc(m.guilds, func(g domain.Guild) bool { return g.ID == id })
	m.memberships = slices.DeleteFunc(m.memberships, func(x domain.Membership) bool { return x.GuildID == id })
	return nil
}

func (m *memStore) ListForMember(_ context.Context, memberID uint64) ([]domain.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Membership
	for _, x := range m.memberships {
		if x.MemberID == memberID {
			out = append(out, x)
		}
	}
	return out, nil
}

func (m *memStore) RoleOf(_ context.Context, guildID, memberID uint64) (domain.Role, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.memberships {
		if x.GuildID == guildID && x.MemberID == memberID {
			return x.Role, true, nil
		}
	}
	return "", false, nil
}

func (m *memStore) ListForGuild(_ context.Context, guildID uint64) ([]domain.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Membership
	for _, x := range m.memberships {
		if x.GuildID == guildID {
			out = append(out, x)
		}
	}
	return out, nil
}

func (m *memStore) Add(_ context.Context, ms domain.Membership) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.memberships {
		if x.GuildID == ms.GuildID && x.MemberID == ms.MemberID {
			return nil
		}
	}
	ms.ID = uint64(len(m.memberships) + 1)
	m.memberships = append(m.memberships, ms)
	return nil
}

func (m *memStore) Change(ctx context.Context, guildID, memberID uint64, to domain.Role, check func([]domain.Membership) error) (domain.Membership, error) {
	ms, _ := m.ListForGuild(ctx, guildID)
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.memberships, func(x domain.Membership) bool { return x.GuildID == guildID && x.MemberID == memberID })
	if i < 0 {
		return domain.Membership{}, ErrMembershipNotFound
	}
	if err := check(ms); err != nil {
		return domain.Membership{}, err
	}
	changed := m.memberships[i]
	if to == "" {
		m.memberships = slices.Delete(m.memberships, i, i+1)
		return changed, nil
	}
	m.memberships[i].Role = to
	changed.Role = to
	return changed, nil
}

// memMembers is identity as the tests need it.
type memMembers struct {
	instanceAdmin uint64
	// emails are the Members by email; CreateMember adds to it.
	emails  map[string]uint64
	revoked [][2]uint64
	reset   []uint64
}

func (m *memMembers) IsInstanceAdmin(_ context.Context, id uint64) (bool, error) {
	return id == m.instanceAdmin, nil
}

func (m *memMembers) MemberByEmail(_ context.Context, email string) (uint64, bool, error) {
	id, found := m.emails[email]
	return id, found, nil
}

func (m *memMembers) CreateMember(_ context.Context, _, email, password string) (uint64, error) {
	if _, found := m.emails[email]; found {
		return 0, ErrMemberExists
	}
	if len(password) < 12 {
		return 0, errors.New("password too short")
	}
	if m.emails == nil {
		m.emails = map[string]uint64{}
	}
	id := uint64(100 + len(m.emails))
	m.emails[email] = id
	return id, nil
}

func (m *memMembers) RevokeAPITokens(_ context.Context, memberID, guildID uint64) error {
	m.revoked = append(m.revoked, [2]uint64{memberID, guildID})
	return nil
}

func (m *memMembers) ResetTwoFactor(_ context.Context, id uint64) error {
	m.reset = append(m.reset, id)
	return nil
}

func newTestService() (*Service, *memStore) {
	s, m, _ := newTestServiceWithMembers()
	return s, m
}

func newTestServiceWithMembers() (*Service, *memStore, *memMembers) {
	m := &memStore{}
	members := &memMembers{}
	return NewService(m, m, &memInvitations{store: m}, members), m, members
}

func TestMakeFirstGuildOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s, m := newTestService()
	if err := s.MakeFirstGuild(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.MakeFirstGuild(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if len(m.guilds) != 1 || m.guilds[0].Name != "Default" {
		t.Fatalf("guilds = %+v, want one Default", m.guilds)
	}
	if r, ok, _ := s.RoleOf(ctx, m.guilds[0].ID, 1); !ok || r != domain.RoleAdmin {
		t.Errorf("Instance admin's Role = %q, %v; want admin", r, ok)
	}
	if _, ok, _ := s.RoleOf(ctx, m.guilds[0].ID, 2); ok {
		t.Error("the second call made a Membership")
	}
}

func TestCreateGuildMakesTheCreatorAdmin(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService()
	if _, err := s.CreateGuild(ctx, " ", "", 7); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("empty name: err = %v", err)
	}
	g, err := s.CreateGuild(ctx, " Bakers ", "bread", 7)
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "Bakers" {
		t.Errorf("name = %q", g.Name)
	}
	ms, _ := s.GuildsOf(ctx, 7)
	if len(ms) != 1 || ms[0].GuildID != g.ID || ms[0].Role != domain.RoleAdmin {
		t.Errorf("memberships = %+v", ms)
	}
}

// twoGuilds is an installation with the Instance admin (1) in "Default" (1),
// Ann (2) admin of "Bakers" (2), and Dev (3) a member of Default and a
// viewer of Bakers.
func twoGuilds(t *testing.T) (*Service, *memStore, *memMembers) {
	t.Helper()
	ctx := context.Background()
	s, m, members := newTestServiceWithMembers()
	members.instanceAdmin = 1
	if err := s.MakeFirstGuild(ctx, 1); err != nil {
		t.Fatal(err)
	}
	m.Add(ctx, domain.Membership{GuildID: 1, MemberID: 3, Role: domain.RoleMember})
	bakers, err := s.CreateGuild(ctx, "Bakers", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	m.Add(ctx, domain.Membership{GuildID: bakers.ID, MemberID: 3, Role: domain.RoleViewer})
	return s, m, members
}

func TestPlace(t *testing.T) {
	ctx := context.Background()
	s, _, _ := twoGuilds(t)
	cases := []struct {
		name                     string
		member                   uint64
		instanceAdmin            bool
		tokenGuild, wanted, want uint64
		role                     domain.Role
	}{
		{"no cookie: the first Guild", 3, false, 0, 0, 1, domain.RoleMember},
		{"the cookie's Guild", 3, false, 0, 2, 2, domain.RoleViewer},
		{"a cookie of a Guild you are not in falls back", 2, false, 0, 1, 2, domain.RoleAdmin},
		{"a cookie of no Guild falls back", 3, false, 0, 99, 1, domain.RoleMember},
		{"the Instance admin is admin anywhere", 1, true, 0, 2, 2, domain.RoleAdmin},
		{"a token acts in its Guild", 3, false, 2, 1, 2, domain.RoleViewer},
		{"a token never acts elsewhere", 2, false, 1, 2, 0, ""},
		{"someone in no Guild", 4, false, 0, 1, 0, ""},
	}
	for _, c := range cases {
		p, ok, err := s.Place(ctx, c.member, c.instanceAdmin, c.tokenGuild, c.wanted)
		if err != nil {
			t.Fatal(err)
		}
		if ok != (c.want != 0) || p.Guild.ID != c.want || p.Role != c.role {
			t.Errorf("%s: %+v %v, want Guild %d as %q", c.name, p, ok, c.want, c.role)
		}
	}
}

func TestTheInstanceAdminWithoutMembershipsActsInTheFirstGuild(t *testing.T) {
	ctx := context.Background()
	s, m, _ := newTestServiceWithMembers()
	s.CreateGuild(ctx, "Bakers", "", 2)
	m.memberships = nil
	p, ok, _ := s.Place(ctx, 1, true, 0, 0)
	if !ok || p.Guild.Name != "Bakers" || p.Role != domain.RoleAdmin {
		t.Errorf("%+v %v", p, ok)
	}
}

func TestChangeRoleAndRemoveMembership(t *testing.T) {
	ctx := context.Background()
	s, m, members := twoGuilds(t)
	if _, err := s.ChangeRole(ctx, 2, 2, domain.RoleAdmin, 3, "owner"); !errors.Is(err, domain.ErrInvalidRole) {
		t.Errorf("make owner: %v", err)
	}
	if ms, err := s.ChangeRole(ctx, 2, 2, domain.RoleAdmin, 3, domain.RoleMember); err != nil || ms.Role != domain.RoleMember {
		t.Fatalf("admin promotes viewer: %+v %v", ms, err)
	}
	if r, _, _ := m.RoleOf(ctx, 1, 3); r != domain.RoleMember {
		t.Errorf("the Role in another Guild changed: %s", r)
	}
	if _, err := s.ChangeRole(ctx, 2, 3, domain.RoleMember, 2, domain.RoleViewer); !errors.Is(err, domain.ErrNotAdmin) {
		t.Errorf("member demotes admin: %v", err)
	}
	if _, err := s.ChangeRole(ctx, 2, 2, domain.RoleAdmin, 2, domain.RoleMember); !errors.Is(err, domain.ErrSelf) {
		t.Errorf("admin demotes self: %v", err)
	}
	if _, err := s.ChangeRole(ctx, 1, 1, domain.RoleAdmin, 2, domain.RoleMember); !errors.Is(err, ErrMembershipNotFound) {
		t.Errorf("Ann is not in Default: %v", err)
	}
	if _, err := s.ChangeRole(ctx, 1, 3, domain.RoleAdmin, 1, domain.RoleMember); !errors.Is(err, domain.ErrInstanceAdminFixed) {
		t.Errorf("demote the Instance admin: %v", err)
	}
	// The Instance admin, admin anywhere, takes Ann's admin away: Bakers
	// would have none.
	if _, err := s.ChangeRole(ctx, 2, 1, domain.RoleAdmin, 2, domain.RoleMember); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("demote the last admin: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 1, domain.RoleAdmin, 2); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("remove the last admin: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 2, domain.RoleAdmin, 3); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := m.RoleOf(ctx, 2, 3); ok {
		t.Error("the Membership stayed")
	}
	if _, ok, _ := m.RoleOf(ctx, 1, 3); !ok {
		t.Error("the Membership in another Guild went too")
	}
	if len(members.revoked) != 1 || members.revoked[0] != [2]uint64{3, 2} {
		t.Errorf("API tokens revoked: %v, want Dev's in Bakers", members.revoked)
	}
}

func TestResetTwoFactorOfAMemberOfTheGuild(t *testing.T) {
	ctx := context.Background()
	s, _, members := twoGuilds(t)
	if err := s.ResetTwoFactor(ctx, 2, 2, domain.RoleAdmin, 1); !errors.Is(err, ErrMembershipNotFound) {
		t.Errorf("someone outside the Guild: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, 1, 3, domain.RoleMember, 1); !errors.Is(err, domain.ErrNotAdmin) {
		t.Errorf("a member resets: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, 1, 1, domain.RoleAdmin, 1); !errors.Is(err, domain.ErrInstanceAdminFixed) {
		t.Errorf("the Instance admin's own: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, 2, 2, domain.RoleAdmin, 3); err != nil || len(members.reset) != 1 || members.reset[0] != 3 {
		t.Errorf("admin resets Dev: %v %v", err, members.reset)
	}
}

func TestGuildsFor(t *testing.T) {
	ctx := context.Background()
	s, _, _ := twoGuilds(t)
	dev, _ := s.GuildsFor(ctx, 3, false)
	if len(dev) != 2 || dev[0].Role != domain.RoleMember || dev[1].Role != domain.RoleViewer {
		t.Errorf("Dev's Guilds: %+v", dev)
	}
	admin, _ := s.GuildsFor(ctx, 1, true)
	if len(admin) != 2 || admin[1].Guild.Name != "Bakers" || admin[1].Role != domain.RoleAdmin {
		t.Errorf("the Instance admin's Guilds: %+v", admin)
	}
}

func TestUpdateGuild(t *testing.T) {
	ctx := context.Background()
	s, _, _ := twoGuilds(t)
	g, err := s.UpdateGuild(ctx, 2, "  Pastry  ", "Cakes")
	if err != nil || g.Name != "Pastry" || g.Description != "Cakes" {
		t.Fatalf("got %+v, %v", g, err)
	}
	if got, _ := s.Guild(ctx, 2); got != g {
		t.Errorf("stored %+v, want %+v", got, g)
	}
	if _, err := s.UpdateGuild(ctx, 2, " ", ""); !errors.Is(err, domain.ErrInvalidName) {
		t.Errorf("empty name: %v", err)
	}
	if _, err := s.UpdateGuild(ctx, 9, "X", ""); !errors.Is(err, ErrGuildNotFound) {
		t.Errorf("unknown guild: %v", err)
	}
}

func TestDeleteGuildOnlyWhenItOwnsNothing(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	owns := map[uint64]bool{2: true}
	s.OnGuildDeleting("projects", func(_ context.Context, guildID uint64) (bool, error) { return owns[guildID], nil })
	s.OnGuildDeleting("servers", func(context.Context, uint64) (bool, error) { return false, nil })
	s.OnGuildDeleting("s3 storages", func(_ context.Context, guildID uint64) (bool, error) { return owns[guildID], nil })

	var inUse ErrGuildInUse
	if err := s.DeleteGuild(ctx, 2); !errors.As(err, &inUse) || !slices.Equal(inUse.Blocking, []string{"projects", "s3 storages"}) {
		t.Fatalf("delete while in use: %v", err)
	}
	owns[2] = false
	if b, _ := s.Blocking(ctx, 2); len(b) != 0 {
		t.Errorf("blocking %v", b)
	}
	if err := s.DeleteGuild(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Guild(ctx, 2); !errors.Is(err, ErrGuildNotFound) {
		t.Errorf("guild still there: %v", err)
	}
	if ms, _ := m.ListForMember(ctx, 3); len(ms) != 1 || ms[0].GuildID != 1 {
		t.Errorf("memberships of member 3: %+v", ms)
	}
	if err := s.DeleteGuild(ctx, 2); !errors.Is(err, ErrGuildNotFound) {
		t.Errorf("delete twice: %v", err)
	}
}

func TestCanActIn(t *testing.T) {
	ctx := context.Background()
	s, _, _ := twoGuilds(t)
	for _, c := range []struct {
		guild, member uint64
		admin, want   bool
	}{
		{1, 3, false, true},
		{2, 3, false, true},
		{1, 2, false, false},
		{2, 1, true, true},
		{9, 1, true, false},
	} {
		if got, err := s.CanActIn(ctx, c.guild, c.member, c.admin); err != nil || got != c.want {
			t.Errorf("CanActIn(%d, %d, %v) = %v, %v; want %v", c.guild, c.member, c.admin, got, err, c.want)
		}
	}
}
