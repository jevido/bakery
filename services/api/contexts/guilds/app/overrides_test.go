package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// TestProjectOverrides is the goal's: in Bakers, Al (Admin) denies deploy
// to Member on Project 10; Dev (Member) can deploy in Project 11 only,
// until Al allows it to Dev personally on 10. A deny of view_resources
// hides Project 11 from Dev, not from Al.
func TestProjectOverrides(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	m.add(2, 6, "admin")  // Al
	m.add(2, 7, "member") // Dev
	place := func(id uint64) Place {
		p, ok, err := s.Place(ctx, id, false, 0, 2)
		if err != nil || !ok {
			t.Fatalf("place of %d: %v %v", id, ok, err)
		}
		return p
	}
	in := func(id, project uint64) domain.Permissions {
		p, err := s.InProject(ctx, place(id), project)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	al := place(6).Permissions
	member := m.seeded(2, "member")

	if _, err := s.SetOverride(ctx, 2, 6, al, 10, member, 0, 0, domain.Of(domain.PermissionDeploy)); err != nil {
		t.Fatal(err)
	}
	if in(7, 10).Has(domain.PermissionDeploy) || !in(7, 11).Has(domain.PermissionDeploy) {
		t.Errorf("Dev deploys in 10: %v, in 11: %v", in(7, 10).Keys(), in(7, 11).Keys())
	}
	if !in(6, 10).Has(domain.PermissionDeploy) {
		t.Error("Al lost deploy in 10")
	}
	if _, err := s.SetOverride(ctx, 2, 6, al, 10, 0, 7, domain.Of(domain.PermissionDeploy), 0); err != nil {
		t.Fatal(err)
	}
	if !in(7, 10).Has(domain.PermissionDeploy) {
		t.Error("Dev's own allow does not beat Member's deny")
	}

	if _, err := s.SetOverride(ctx, 2, 6, al, 11, member, 0, 0, domain.Of(domain.PermissionViewResources)); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.VisibleProjects(ctx, place(7), []uint64{10, 11, 12}); !slices.Equal(got, []uint64{10, 12}) {
		t.Errorf("Dev sees %v", got)
	}
	if got, _ := s.VisibleProjects(ctx, place(6), []uint64{10, 11, 12}); !slices.Equal(got, []uint64{10, 11, 12}) {
		t.Errorf("Al sees %v", got)
	}

	// Both empty deletes it.
	if _, err := s.SetOverride(ctx, 2, 6, al, 11, member, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if os, _ := s.Overrides(ctx, 2, 11); len(os) != 0 {
		t.Errorf("overrides of 11 after clearing: %+v", os)
	}
	if err := s.ForgetProject(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if os, _ := s.Overrides(ctx, 2, 10); len(os) != 0 {
		t.Errorf("overrides of a forgotten project: %+v", os)
	}
}

func TestSetOverrideFollowsTheHierarchy(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	m.add(2, 6, "admin")
	m.add(2, 7, "member")
	member, admin := m.seeded(2, "member"), m.seeded(2, "admin")
	keeper, err := s.CreateRole(ctx, 2, 2, domain.AllPermissions, "Keeper", "", domain.Of(domain.PermissionManageRoles, domain.PermissionViewResources))
	if err != nil {
		t.Fatal(err)
	}
	m.addWith(2, 8, []uint64{keeper.ID})
	m.addWith(2, 9, nil)
	kp := domain.Of(domain.PermissionManageRoles, domain.PermissionViewResources)
	cases := []struct {
		name         string
		actor        uint64
		perms        domain.Permissions
		role, member uint64
		allow, deny  domain.Permissions
		want         error
	}{
		{"a Role above", 8, kp, admin, 0, 0, domain.Of(domain.PermissionDeploy), domain.ErrRoleNotBelow},
		{"a Member above", 8, kp, 0, 6, 0, domain.Of(domain.PermissionViewResources), domain.ErrMemberNotBelow},
		{"a Permission not held", 8, kp, 0, 9, domain.Of(domain.PermissionDeploy), 0, domain.ErrNotHeld},
		{"a guild-wide Permission", 6, domain.AllPermissions, member, 0, domain.Of(domain.PermissionManageServers), 0, domain.ErrNotOverridable},
		{"without manage_roles", 7, memberPerms, 0, 3, 0, domain.Of(domain.PermissionDeploy), domain.ErrMissing{Permission: domain.PermissionManageRoles}},
		{"another Guild's Role", 6, domain.AllPermissions, m.seeded(1, "member"), 0, 0, domain.Of(domain.PermissionDeploy), ErrRoleNotFound},
		{"the Base role", 8, kp, 0, 0, 0, 0, nil},
	}
	base, _ := s.RolesIn(ctx, 2)
	cases[len(cases)-1].role = base[0].ID
	cases[len(cases)-1].deny = domain.Of(domain.PermissionViewResources)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.SetOverride(ctx, 2, c.actor, c.perms, 10, c.role, c.member, c.allow, c.deny)
			if !errors.Is(err, c.want) && err != c.want {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}
