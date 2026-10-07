package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// positions lists the Guild's Roles as name=Position, top first.
func positions(t *testing.T, s *Service, guildID uint64) []string {
	t.Helper()
	roles, err := s.RolesIn(context.Background(), guildID)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for i := len(roles) - 1; i >= 0; i-- {
		out = append(out, roles[i].Name+"="+string(rune('0'+roles[i].Position)))
	}
	return out
}

// TestTheDeployerScenario is the goal's: Al, an Admin of Bakers, makes
// "Deployer" below Admin and gives it to Dev with a second Role that has
// manage_roles and manage_members; Dev still cannot touch Admin or Al.
func TestTheDeployerScenario(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	m.add(2, 6, "admin") // Al
	m.add(2, 7, "viewer")
	perms := func(id uint64) domain.Permissions {
		p, _, err := s.PermissionsIn(ctx, 2, id)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	al := perms(6)

	deployer, err := s.CreateRole(ctx, 2, 6, al, "Deployer", "#E67E22", domain.Of(domain.PermissionViewResources, domain.PermissionDeploy))
	if err != nil || deployer.ID == 0 || deployer.Position != 1 || deployer.Color != "#e67e22" {
		t.Fatalf("create Deployer: %+v %v", deployer, err)
	}
	if got := positions(t, s, 2); !slices.Equal(got, []string{"Admin=4", "Member=3", "Viewer=2", "Deployer=1", "@everyone=0"}) {
		t.Errorf("after create: %v", got)
	}
	admin, member, viewer := m.seeded(2, "admin"), m.seeded(2, "member"), m.seeded(2, "viewer")
	if _, err := s.ReorderRoles(ctx, 2, 6, al, []uint64{admin, deployer.ID, member, viewer}); err != nil {
		t.Fatal(err)
	}
	if got := positions(t, s, 2); !slices.Equal(got, []string{"Admin=4", "Deployer=3", "Member=2", "Viewer=1", "@everyone=0"}) {
		t.Errorf("after reorder: %v", got)
	}
	keeper, err := s.CreateRole(ctx, 2, 6, al, "Keeper", "", domain.Of(domain.PermissionManageRoles, domain.PermissionManageMembers))
	if err != nil || keeper.Color != domain.DefaultRoleColor {
		t.Fatalf("create Keeper: %+v %v", keeper, err)
	}
	for _, r := range []uint64{deployer.ID, keeper.ID} {
		if _, err := s.AssignRole(ctx, 2, 6, al, 3, r); err != nil {
			t.Fatalf("assign %d to Dev: %v", r, err)
		}
	}
	dev := perms(3)
	if !dev.Has(domain.PermissionDeploy) || dev.Has(domain.PermissionSeeSecrets) {
		t.Errorf("Dev's Permissions: %v", dev.Keys())
	}

	name := "Boss"
	if _, err := s.EditRole(ctx, 2, 3, dev, admin, RoleEdit{Name: &name}); !errors.Is(err, domain.ErrRoleNotBelow) {
		t.Errorf("Dev edits Admin: %v", err)
	}
	if _, err := s.AssignRole(ctx, 2, 3, dev, 7, admin); !errors.Is(err, domain.ErrRoleNotBelow) {
		t.Errorf("Dev assigns Admin: %v", err)
	}
	if _, err := s.AssignRole(ctx, 2, 3, dev, 7, member); err != nil {
		t.Errorf("Dev assigns Member, below Deployer: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 3, dev, 6); !errors.Is(err, domain.ErrMemberNotBelow) {
		t.Errorf("Dev removes Al: %v", err)
	}
	if err := s.RemoveMembership(ctx, 2, 6, al, 2); !errors.Is(err, domain.ErrGuildMaster) {
		t.Errorf("Al removes Ann, the Guild Master: %v", err)
	}
	if _, err := s.CreateRole(ctx, 2, 3, dev, "Peeker", "", domain.Of(domain.PermissionSeeSecrets)); !errors.Is(err, domain.ErrNotHeld) {
		t.Errorf("Dev grants see_secrets: %v", err)
	}
	if _, err := s.ReorderRoles(ctx, 2, 3, dev, []uint64{deployer.ID, admin, member, viewer, keeper.ID}); !errors.Is(err, domain.ErrRoleNotBelow) {
		t.Errorf("Dev moves Deployer above Admin: %v", err)
	}
	base := domain.Of(domain.PermissionViewResources)
	roles, _ := s.RolesIn(ctx, 2)
	if r, err := s.EditRole(ctx, 2, 3, dev, roles[0].ID, RoleEdit{Permissions: &base}); err != nil || r.Permissions != base {
		t.Errorf("Dev edits @everyone's Permissions: %+v %v", r, err)
	}
	if _, err := s.EditRole(ctx, 2, 3, dev, roles[0].ID, RoleEdit{Name: &name}); !errors.Is(err, domain.ErrBaseRoleFixed) {
		t.Errorf("Dev renames @everyone: %v", err)
	}
	if err := s.DeleteRole(ctx, 2, 6, al, roles[0].ID); !errors.Is(err, domain.ErrBaseRoleFixed) {
		t.Errorf("Al deletes @everyone: %v", err)
	}

	if err := s.DeleteRole(ctx, 2, 6, al, deployer.ID); err != nil {
		t.Fatal(err)
	}
	if got := positions(t, s, 2); !slices.Equal(got, []string{"Admin=4", "Member=3", "Viewer=2", "Keeper=1", "@everyone=0"}) {
		t.Errorf("after delete: %v", got)
	}
	if perms(3).Has(domain.PermissionDeploy) {
		t.Error("Dev still deploys with Deployer deleted")
	}
}

func TestInviteWithRoles(t *testing.T) {
	ctx := context.Background()
	s, m, _ := invitingService(t)
	m.add(2, 6, "admin")
	ops, err := s.CreateRole(ctx, 2, 6, adminPerms, "Ops", "", domain.Of(domain.PermissionDeploy))
	if err != nil {
		t.Fatal(err)
	}
	viewer := m.seeded(2, "viewer")
	_, token, err := s.Invite(ctx, 2, 6, adminPerms, "new@example.com", []uint64{ops.ID, viewer, ops.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, id, err := s.AcceptAsNewMember(ctx, token, "New", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if ms, _, _ := m.Of(ctx, 2, id); !slices.Equal(ms.RoleIDs, []uint64{ops.ID, viewer}) {
		t.Errorf("Roles held: %v", ms.RoleIDs)
	}

	// Someone with manage_members but Viewer as their highest Role cannot
	// invite with Admin.
	limited := domain.Of(domain.PermissionManageMembers)
	m.add(2, 8, "viewer")
	if _, _, err := s.Invite(ctx, 2, 8, limited, "x@example.com", []uint64{m.seeded(2, "admin")}); !errors.Is(err, domain.ErrRoleNotBelow) {
		t.Errorf("invite with Admin from below it: %v", err)
	}
	if _, _, err := s.Invite(ctx, 2, 8, limited, "x@example.com", nil); err != nil {
		t.Errorf("invite with the Base role only: %v", err)
	}
	if _, _, err := s.Invite(ctx, 2, 3, viewerPerms, "y@example.com", nil); !errors.As(err, new(domain.ErrMissing)) {
		t.Errorf("invite without manage_members: %v", err)
	}

	// A Role deleted before the Invitation is accepted is skipped.
	_, token, _ = s.Invite(ctx, 2, 6, adminPerms, "late@example.com", []uint64{ops.ID})
	if err := s.DeleteRole(ctx, 2, 6, adminPerms, ops.ID); err != nil {
		t.Fatal(err)
	}
	_, id, err = s.AcceptAsNewMember(ctx, token, "Late", "correct horse")
	if ms, _, _ := m.Of(ctx, 2, id); err != nil || len(ms.RoleIDs) != 0 {
		t.Errorf("Roles held: %v %v", ms.RoleIDs, err)
	}
}
