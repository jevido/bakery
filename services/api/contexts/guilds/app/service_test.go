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
	roles       []domain.Role
}

// The Permissions of the seeded Roles, for actors in the tests.
var (
	viewerPerms = domain.Of(domain.PermissionViewResources)
	memberPerms = domain.Of(domain.PermissionViewResources, domain.PermissionSeeSecrets, domain.PermissionDeploy, domain.PermissionManageApplications)
	adminPerms  = domain.Of(domain.PermissionAdministrator)
)

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

func (m *memStore) Create(_ context.Context, g domain.Guild) (domain.Guild, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.create(g), nil
}

func (m *memStore) MasteredBy(_ context.Context, memberID uint64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.ContainsFunc(m.guilds, func(g domain.Guild) bool { return g.MasterID == memberID }), nil
}

func (m *memStore) create(g domain.Guild) domain.Guild {
	g.ID = uint64(len(m.guilds) + 1)
	m.guilds = append(m.guilds, g)
	for _, r := range domain.SeedRoles(g.ID) {
		r.ID = uint64(len(m.roles) + 1)
		m.roles = append(m.roles, r)
	}
	m.add(g.ID, g.MasterID, "admin")
	return g
}

func (m *memStore) CreateFirstIfNone(_ context.Context, g domain.Guild) (domain.Guild, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.guilds) > 0 {
		return domain.Guild{}, false, nil
	}
	return m.create(g), true, nil
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

func (m *memStore) Of(_ context.Context, guildID, memberID uint64) (domain.Membership, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.memberships {
		if x.GuildID == guildID && x.MemberID == memberID {
			return x, true, nil
		}
	}
	return domain.Membership{}, false, nil
}

func (m *memStore) ForGuild(_ context.Context, guildID uint64) ([]domain.Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Role
	for _, r := range m.roles {
		if r.GuildID == guildID {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b domain.Role) int { return a.Position - b.Position })
	return out, nil
}

// seeded is the id of the seeded Role a former role names in the Guild.
func (m *memStore) seeded(guildID uint64, former string) uint64 {
	var roles []domain.Role
	for _, r := range m.roles {
		if r.GuildID == guildID {
			roles = append(roles, r)
		}
	}
	r, _, _ := domain.SeededRole(roles, former)
	return r.ID
}

// roleOf is the former role the Member's Roles in the Guild read as, false
// without a Membership.
func (m *memStore) roleOf(guildID, memberID uint64) (string, bool) {
	ms, ok, _ := m.Of(context.Background(), guildID, memberID)
	if !ok {
		return "", false
	}
	roles, _ := m.ForGuild(context.Background(), guildID)
	switch p := domain.PermissionsOf(roles, ms); {
	case p.Has(domain.PermissionAdministrator):
		return "admin", true
	case p.Has(domain.PermissionManageApplications):
		return "member", true
	case p.Has(domain.PermissionViewResources):
		return "viewer", true
	}
	return "", true
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

// add gives memberID a Membership in the Guild holding the seeded Role a
// former role names, unless they hold one there already. The caller holds
// mu or is the only one using m.
func (m *memStore) add(guildID, memberID uint64, former string) {
	m.addWith(guildID, memberID, []uint64{m.seeded(guildID, former)})
}

// addWith is add with these Roles, those of them still in the Guild.
func (m *memStore) addWith(guildID, memberID uint64, roleIDs []uint64) {
	for _, x := range m.memberships {
		if x.GuildID == guildID && x.MemberID == memberID {
			return
		}
	}
	held := []uint64{}
	for _, id := range roleIDs {
		if slices.ContainsFunc(m.roles, func(r domain.Role) bool { return r.ID == id && r.GuildID == guildID }) {
			held = append(held, id)
		}
	}
	// Ids stay unique after a removal: one past the highest.
	next := uint64(1)
	for _, x := range m.memberships {
		next = max(next, x.ID+1)
	}
	m.memberships = append(m.memberships, domain.Membership{ID: next, GuildID: guildID, MemberID: memberID, RoleIDs: held})
}

func (m *memStore) Change(_ context.Context, guildID uint64, decide func(Hierarchy) (Change, error)) (Change, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := Hierarchy{}
	for _, g := range m.guilds {
		if g.ID == guildID {
			h.Guild = g
		}
	}
	if h.Guild.ID == 0 {
		return Change{}, ErrGuildNotFound
	}
	for _, r := range m.roles {
		if r.GuildID == guildID {
			h.Roles = append(h.Roles, r)
		}
	}
	slices.SortFunc(h.Roles, func(a, b domain.Role) int { return a.Position - b.Position })
	for _, x := range m.memberships {
		if x.GuildID == guildID {
			h.Memberships = append(h.Memberships, x)
		}
	}
	c, err := decide(h)
	if err != nil {
		return Change{}, err
	}
	for i, r := range c.Roles {
		if r.ID == 0 {
			r.ID = uint64(len(m.roles) + 1)
			c.Roles[i] = r
			m.roles = append(m.roles, r)
			continue
		}
		m.roles[slices.IndexFunc(m.roles, func(x domain.Role) bool { return x.ID == r.ID })] = r
	}
	if c.DeletedRole != 0 {
		// Ids stay unique: the slot stays, in no Guild.
		m.roles[slices.IndexFunc(m.roles, func(x domain.Role) bool { return x.ID == c.DeletedRole })].GuildID = 0
		for i := range m.memberships {
			m.memberships[i].RoleIDs = slices.DeleteFunc(slices.Clone(m.memberships[i].RoleIDs), func(id uint64) bool { return id == c.DeletedRole })
		}
	}
	if c.Membership != nil {
		m.memberships[slices.IndexFunc(m.memberships, func(x domain.Membership) bool { return x.ID == c.Membership.ID })].RoleIDs = c.Membership.RoleIDs
	}
	if c.RemovedMembership != 0 {
		m.memberships = slices.DeleteFunc(m.memberships, func(x domain.Membership) bool { return x.ID == c.RemovedMembership })
	}
	return c, nil
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
	return NewService(m, m, m, &memInvitations{store: m}, &memOffers{store: m}, members), m, members
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
	if r, ok := m.roleOf(m.guilds[0].ID, 1); !ok || r != "admin" {
		t.Errorf("Instance admin's Role = %q, %v; want admin", r, ok)
	}
	if _, ok := m.roleOf(m.guilds[0].ID, 2); ok {
		t.Error("the second call made a Membership")
	}
}

func TestCreateGuildMakesTheCreatorAdmin(t *testing.T) {
	ctx := context.Background()
	s, m := newTestService()
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
	if r, _ := m.roleOf(g.ID, 7); len(ms) != 1 || ms[0].GuildID != g.ID || r != "admin" {
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
	m.add(1, 3, "member")
	bakers, err := s.CreateGuild(ctx, "Bakers", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	m.add(bakers.ID, 3, "viewer")
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
		perms                    domain.Permissions
	}{
		{"no cookie: the first Guild", 3, false, 0, 0, 1, memberPerms},
		{"the cookie's Guild", 3, false, 0, 2, 2, viewerPerms},
		{"a cookie of a Guild you are not in falls back; the Guild Master holds every Permission", 2, false, 0, 1, 2, domain.AllPermissions},
		{"a cookie of no Guild falls back", 3, false, 0, 99, 1, memberPerms},
		{"the Instance admin holds every Permission anywhere", 1, true, 0, 2, 2, domain.AllPermissions},
		{"a token acts in its Guild", 3, false, 2, 1, 2, viewerPerms},
		{"a token never acts elsewhere", 2, false, 1, 2, 0, 0},
		{"someone in no Guild", 4, false, 0, 1, 0, 0},
	}
	for _, c := range cases {
		p, ok, err := s.Place(ctx, c.member, c.instanceAdmin, c.tokenGuild, c.wanted)
		if err != nil {
			t.Fatal(err)
		}
		if ok != (c.want != 0) || p.Guild.ID != c.want || p.Permissions != c.perms {
			t.Errorf("%s: %+v %v, want Guild %d with %v", c.name, p, ok, c.want, c.perms.Keys())
		}
	}
}

func TestTheInstanceAdminWithoutMembershipsActsInTheFirstGuild(t *testing.T) {
	ctx := context.Background()
	s, m, _ := newTestServiceWithMembers()
	s.CreateGuild(ctx, "Bakers", "", 2)
	m.memberships = nil
	p, ok, _ := s.Place(ctx, 1, true, 0, 0)
	if !ok || p.Guild.Name != "Bakers" || p.Permissions != domain.AllPermissions {
		t.Errorf("%+v %v", p, ok)
	}
}

func TestAssignRoleAndRemoveMembership(t *testing.T) {
	ctx := context.Background()
	s, m, members := twoGuilds(t)
	member, viewer := m.seeded(2, "member"), m.seeded(2, "viewer")
	if ms, err := s.AssignRole(ctx, 2, 2, adminPerms, 3, member); err != nil || !slices.Equal(ms.RoleIDs, []uint64{viewer, member}) {
		t.Fatalf("Ann gives Dev Member: %+v %v", ms, err)
	}
	if ms, err := s.AssignRole(ctx, 2, 2, adminPerms, 3, member); err != nil || len(ms.RoleIDs) != 2 {
		t.Errorf("assigning a held Role again: %+v %v", ms, err)
	}
	if ms, err := s.RemoveRole(ctx, 2, 2, adminPerms, 3, viewer); err != nil || !slices.Equal(ms.RoleIDs, []uint64{member}) {
		t.Fatalf("Ann takes Viewer: %+v %v", ms, err)
	}
	if r, _ := m.roleOf(1, 3); r != "member" {
		t.Errorf("the Roles in another Guild changed: %s", r)
	}
	if _, err := s.AssignRole(ctx, 2, 3, memberPerms, 2, viewer); !errors.As(err, new(domain.ErrMissing)) {
		t.Errorf("Dev without manage_roles: %v", err)
	}
	if _, err := s.AssignRole(ctx, 2, 2, adminPerms, 3, m.seeded(1, "admin")); !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("another Guild's Role: %v", err)
	}
	if _, err := s.AssignRole(ctx, 2, 2, adminPerms, 2, viewer); !errors.Is(err, domain.ErrSelf) {
		t.Errorf("Ann re-roles herself: %v", err)
	}
	if _, err := s.AssignRole(ctx, 1, 1, adminPerms, 2, viewer); !errors.Is(err, ErrMembershipNotFound) {
		t.Errorf("Ann is not in Default: %v", err)
	}
	if _, err := s.AssignRole(ctx, 1, 3, adminPerms, 1, m.seeded(1, "viewer")); !errors.Is(err, domain.ErrInstanceAdminFixed) {
		t.Errorf("re-role the Instance admin: %v", err)
	}
	// Not even the Instance admin, above every Role anywhere, re-roles or
	// removes Ann, the Guild Master of Bakers.
	if _, err := s.RemoveRole(ctx, 2, 1, domain.AllPermissions, 2, m.seeded(2, "admin")); !errors.Is(err, domain.ErrGuildMaster) {
		t.Errorf("re-role the Guild Master: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 1, domain.AllPermissions, 2); !errors.Is(err, domain.ErrGuildMaster) {
		t.Errorf("remove the Guild Master: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, 2, 1, domain.AllPermissions, 2); !errors.Is(err, domain.ErrGuildMaster) {
		t.Errorf("reset the Guild Master's two-factor: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 2, adminPerms, 3); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.roleOf(2, 3); ok {
		t.Error("the Membership stayed")
	}
	if _, ok := m.roleOf(1, 3); !ok {
		t.Error("the Membership in another Guild went too")
	}
	if len(members.revoked) != 1 || members.revoked[0] != [2]uint64{3, 2} {
		t.Errorf("API tokens revoked: %v, want Dev's in Bakers", members.revoked)
	}
}

func TestResetTwoFactorOfAMemberOfTheGuild(t *testing.T) {
	ctx := context.Background()
	s, _, members := twoGuilds(t)
	if err := s.ResetTwoFactor(ctx, 2, 2, adminPerms, 1); !errors.Is(err, ErrMembershipNotFound) {
		t.Errorf("someone outside the Guild: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, 1, 3, memberPerms, 1); !errors.As(err, new(domain.ErrMissing)) {
		t.Errorf("a member resets: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, 1, 1, adminPerms, 1); !errors.Is(err, domain.ErrInstanceAdminFixed) {
		t.Errorf("the Instance admin's own: %v", err)
	}
	if err := s.ResetTwoFactor(ctx, 2, 2, adminPerms, 3); err != nil || len(members.reset) != 1 || members.reset[0] != 3 {
		t.Errorf("admin resets Dev: %v %v", err, members.reset)
	}
}

func TestGuildsFor(t *testing.T) {
	ctx := context.Background()
	s, _, _ := twoGuilds(t)
	dev, _ := s.GuildsFor(ctx, 3, false)
	if len(dev) != 2 || dev[0].Permissions != memberPerms || dev[1].Permissions != viewerPerms {
		t.Errorf("Dev's Guilds: %+v", dev)
	}
	admin, _ := s.GuildsFor(ctx, 1, true)
	if len(admin) != 2 || admin[1].Guild.Name != "Bakers" || admin[1].Permissions != domain.AllPermissions {
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

func TestPlaceUnitesTheBaseRoleAndEveryRoleHeld(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	deployer := domain.Role{ID: 99, GuildID: 2, Name: "Deployer", Position: 4, Permissions: domain.Of(domain.PermissionDeploy)}
	m.roles = append(m.roles, deployer)
	i := slices.IndexFunc(m.memberships, func(x domain.Membership) bool { return x.GuildID == 2 && x.MemberID == 3 })
	m.memberships[i].RoleIDs = append(m.memberships[i].RoleIDs, deployer.ID)
	m.roles[slices.IndexFunc(m.roles, func(r domain.Role) bool { return r.GuildID == 2 && r.Base })].Permissions = domain.Of(domain.PermissionApprove)

	p, ok, err := s.Place(ctx, 3, false, 2, 0)
	want := viewerPerms.Union(domain.Of(domain.PermissionDeploy, domain.PermissionApprove))
	if err != nil || !ok || p.Permissions != want {
		t.Errorf("Dev in Bakers: %v, want %v", p.Permissions.Keys(), want.Keys())
	}
	if p.Permissions.Has(domain.PermissionSeeSecrets) {
		t.Error("Viewer and Deployer see no Secrets")
	}
}
