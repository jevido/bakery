package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
)

// TestAnAgentStaysBelowItsHirer is the goal's: Mo, a Member of Bakers,
// hires an Agent with only "Deployer"; it can do exactly what that Role
// allows, is never given a Role at or above Mo's highest, and loses Roles
// whenever Mo drops below them.
func TestAnAgentStaysBelowItsHirer(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	m.add(2, 6, "admin")  // Al
	m.add(2, 7, "member") // Mo
	perms := func(id uint64) domain.Permissions {
		p, _, err := s.PermissionsIn(ctx, 2, id)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	al := perms(6)
	deployer, err := s.CreateRole(ctx, 2, 6, al, "Deployer", "", domain.Of(domain.PermissionDeploy))
	if err != nil {
		t.Fatal(err)
	}
	viewer, member := m.seeded(2, "viewer"), m.seeded(2, "member")
	before, _ := s.MembershipsIn(ctx, 2)

	if _, err := s.JoinAgent(ctx, 2, 7, 51, perms(7), []uint64{member}); !errors.Is(err, domain.ErrRoleNotBelow) {
		t.Errorf("Mo hires with Member, their own highest: %v", err)
	}
	if _, err := s.JoinAgent(ctx, 2, 6, 52, al, []uint64{viewer}); err != nil {
		t.Errorf("Al hires with Viewer: %v", err)
	}
	a, err := s.JoinAgent(ctx, 2, 7, 50, perms(7), []uint64{deployer.ID})
	if err != nil || !a.IsAgent() || a.ID == 0 || a.HirerID != 7 {
		t.Fatalf("Mo hires with Deployer: %+v %v", a, err)
	}
	if _, err := s.JoinAgent(ctx, 2, 7, 50, perms(7), nil); !errors.Is(err, ErrAgentMemberAlready) {
		t.Errorf("a second Agent membership: %v", err)
	}
	if _, err := s.JoinAgent(ctx, 2, 1, 53, domain.AllPermissions, nil); !errors.Is(err, ErrMembershipNotFound) {
		t.Errorf("the Instance admin, not in Bakers, hires: %v", err)
	}

	roles, _ := s.RolesIn(ctx, 2)
	base := roles[0].Permissions
	if p, err := s.AgentPermissions(ctx, 2, 50); err != nil || p != base.Union(domain.Of(domain.PermissionDeploy)) {
		t.Errorf("the Agent's Permissions: %v %v", p.Keys(), err)
	}
	if in, _ := s.IsMember(ctx, 2, 50); in {
		t.Error("the Agent counts as a Member")
	}
	if after, _ := s.MembershipsIn(ctx, 2); len(after) != len(before) {
		t.Errorf("Members listed: %d, before %d", len(after), len(before))
	}

	if _, err := s.AssignAgentRole(ctx, 2, 6, al, 50, member); !errors.Is(err, domain.ErrAboveHirer) {
		t.Errorf("Al gives the Agent Member, Mo's highest: %v", err)
	}
	if _, err := s.AssignAgentRole(ctx, 2, 7, perms(7), 50, viewer); err != nil {
		t.Errorf("Mo gives the Agent Viewer: %v", err)
	}
	if _, err := s.AssignRole(ctx, 2, 6, al, 7, viewer); err != nil {
		t.Fatal(err)
	}
	held := func(agentID uint64) []string {
		rs, err := s.AgentRoles(ctx, 2, agentID)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, r := range rs {
			out = append(out, r.Name)
		}
		return out
	}
	if got := held(50); !slices.Equal(got, []string{"Viewer", "Deployer"}) {
		t.Errorf("the Agent's Roles: %v", got)
	}

	// Mo drops to Viewer: the Agent loses Viewer, at Mo's new highest.
	if _, err := s.RemoveRole(ctx, 2, 6, al, 7, member); err != nil {
		t.Fatal(err)
	}
	if got := held(50); !slices.Equal(got, []string{"Deployer"}) {
		t.Errorf("after Mo drops: %v", got)
	}
	// Deployer moves above Viewer, Mo's highest: the Agent loses it.
	admin := m.seeded(2, "admin")
	if _, err := s.ReorderRoles(ctx, 2, 6, al, []uint64{admin, member, deployer.ID, viewer}); err != nil {
		t.Fatal(err)
	}
	if got := held(50); len(got) != 0 {
		t.Errorf("after the reorder: %v", got)
	}
	if _, err := s.RemoveAgentRole(ctx, 2, 3, perms(3), 52, viewer); !errors.Is(err, domain.ErrRoleNotBelow) {
		t.Errorf("Dev, a Viewer, takes Viewer from Al's Agent: %v", err)
	}
	if _, err := s.RemoveAgentRole(ctx, 2, 6, al, 52, viewer); err != nil || len(held(52)) != 0 {
		t.Errorf("Al takes Viewer from their Agent: %v", err)
	}

	if err := s.LeaveAgent(ctx, 2, 50); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AgentPermissions(ctx, 2, 50); !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("after leaving: %v", err)
	}
	if err := s.LeaveAgent(ctx, 2, 50); err != nil {
		t.Errorf("leaving twice: %v", err)
	}
}

func TestRemovingTheHirerTellsOnMemberLeaving(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	m.add(2, 6, "admin")
	m.add(2, 7, "member")
	mo, _, _ := s.PermissionsIn(ctx, 2, 7)
	if _, err := s.JoinAgent(ctx, 2, 7, 50, mo, []uint64{m.seeded(2, "viewer")}); err != nil {
		t.Fatal(err)
	}
	var left []uint64
	s.OnMemberLeaving(func(_ context.Context, guildID, memberID uint64) error {
		left = append(left, guildID, memberID)
		return nil
	})
	if err := s.RemoveMembership(ctx, 2, 6, domain.AllPermissions, 7); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(left, []uint64{2, 7}) {
		t.Errorf("told: %v", left)
	}
	if rs, _ := s.AgentRoles(ctx, 2, 50); len(rs) != 0 {
		t.Errorf("the Agent of a Hirer who left keeps %v", rs)
	}
}

func TestATransferLowersTheOldGuildMastersAgents(t *testing.T) {
	ctx := context.Background()
	s, m, _ := twoGuilds(t)
	// Ann (2), the Guild Master, holds Admin; her Agent holds Admin too.
	admin := m.seeded(2, "admin")
	if _, err := s.JoinAgent(ctx, 2, 2, 50, domain.AllPermissions, []uint64{admin}); err != nil {
		t.Fatal(err)
	}
	o, err := s.OfferGuildMaster(ctx, 2, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptOffer(ctx, o.ID, 3); err != nil {
		t.Fatal(err)
	}
	if rs, _ := s.AgentRoles(ctx, 2, 50); len(rs) != 0 {
		t.Errorf("Ann's Agent keeps %v at Ann's own highest", rs)
	}
	if up, _ := s.RankAbove(ctx, 2, 3, 2); !up {
		t.Error("the new Guild Master does not rank above Ann")
	}
	if up, _ := s.RankAbove(ctx, 2, 2, 3); up {
		t.Error("Ann ranks above the new Guild Master")
	}
}
