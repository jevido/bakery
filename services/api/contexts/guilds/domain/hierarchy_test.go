package domain

import (
	"errors"
	"slices"
	"testing"
)

// hierarchy is a Guild with the goal's Deployer between Viewer and Admin:
// @everyone 0, Viewer 1, Deployer 2, Admin 3. Ann (4) is the Guild Master,
// Ada (1) the Instance admin, Al (2) holds Admin, Dee (3) holds Deployer,
// which has manage_roles and manage_members, and Vic (5) holds Viewer.
func hierarchy() (Guild, []Role) {
	return Guild{ID: 1, MasterID: 4}, []Role{
		{ID: 1, Name: BaseRoleName, Position: 0, Base: true},
		{ID: 2, Name: ViewerRole, Position: 1, Permissions: Of(PermissionViewResources)},
		{ID: 5, Name: "Deployer", Position: 2, Permissions: Of(PermissionViewResources, PermissionDeploy, PermissionManageRoles, PermissionManageMembers)},
		{ID: 4, Name: AdminRole, Position: 3, Permissions: Of(PermissionAdministrator)},
	}
}

func TestRankOf(t *testing.T) {
	g, roles := hierarchy()
	cases := []struct {
		name          string
		m             Membership
		instanceAdmin bool
		want          Rank
	}{
		{"the Guild Master", Membership{MemberID: 4, RoleIDs: []uint64{4}}, false, GuildMasterRank},
		{"the Guild Master who is also the Instance admin", Membership{MemberID: 4}, true, GuildMasterRank},
		{"the Instance admin", Membership{MemberID: 1}, true, InstanceAdminRank},
		{"Admin and Viewer", Membership{MemberID: 2, RoleIDs: []uint64{2, 4}}, false, 3},
		{"Deployer", Membership{MemberID: 3, RoleIDs: []uint64{5}}, false, 2},
		{"only @everyone", Membership{MemberID: 6}, false, 0},
	}
	for _, c := range cases {
		if got := RankOf(g, roles, c.m, c.m.MemberID, c.instanceAdmin); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	if !GuildMasterRank.Above(int(InstanceAdminRank)) || InstanceAdminRank.Above(int(GuildMasterRank)) {
		t.Error("the Guild Master is above the Instance admin")
	}
}

func TestRoleRules(t *testing.T) {
	_, roles := hierarchy()
	base, viewer, deployer, admin := roles[0], roles[1], roles[2], roles[3]
	cases := []struct {
		name  string
		check func(Rank, Role) error
		actor Rank
		role  Role
		want  error
	}{
		{"Deployer cannot edit Admin", CanEditRole, 2, admin, ErrRoleNotBelow},
		{"Deployer cannot edit Deployer", CanEditRole, 2, deployer, ErrRoleNotBelow},
		{"Deployer edits Viewer", CanEditRole, 2, viewer, nil},
		{"anyone edits @everyone's Permissions", CanEditRole, 0, base, nil},
		{"the Instance admin edits Admin", CanEditRole, InstanceAdminRank, admin, nil},
		{"Deployer cannot assign Admin", CanAssign, 2, admin, ErrRoleNotBelow},
		{"Deployer assigns Viewer", CanAssign, 2, viewer, nil},
		{"nobody assigns @everyone", CanAssign, GuildMasterRank, base, ErrBaseRoleFixed},
		{"nobody deletes @everyone", CanDeleteRole, GuildMasterRank, base, ErrBaseRoleFixed},
		{"Deployer cannot delete Admin", CanDeleteRole, 2, admin, ErrRoleNotBelow},
		{"Admin deletes Deployer", CanDeleteRole, 3, deployer, nil},
	}
	for _, c := range cases {
		if err := c.check(c.actor, c.role); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if err := base.Rename("everyone"); !errors.Is(err, ErrBaseRoleFixed) {
		t.Errorf("rename @everyone: %v", err)
	}
	if err := base.Rename(BaseRoleName); err != nil {
		t.Errorf("@everyone keeps its name: %v", err)
	}
}

func TestCanGrant(t *testing.T) {
	deployer := Of(PermissionViewResources, PermissionDeploy, PermissionManageRoles)
	cases := []struct {
		name                string
		held, before, after Permissions
		want                error
	}{
		{"grant what one holds", deployer, 0, Of(PermissionDeploy), nil},
		{"grant see_secrets without it", deployer, 0, Of(PermissionSeeSecrets), ErrNotHeld},
		{"grant administrator without it", deployer, 0, Of(PermissionAdministrator), ErrNotHeld},
		{"take away what one lacks", deployer, Of(PermissionSeeSecrets, PermissionDeploy), Of(PermissionDeploy), ErrNotHeld},
		{"leave what one lacks as it is", deployer, Of(PermissionSeeSecrets), Of(PermissionSeeSecrets, PermissionDeploy), nil},
		{"administrator grants administrator", Of(PermissionAdministrator), 0, Of(PermissionAdministrator), nil},
	}
	for _, c := range cases {
		if err := CanGrant(c.held, c.before, c.after); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

func TestCanManage(t *testing.T) {
	g, roles := hierarchy()
	rank := func(m Membership) Rank { return RankOf(g, roles, m, m.MemberID, false) }
	al := Membership{MemberID: 2, RoleIDs: []uint64{4}}
	dee := Membership{MemberID: 3, RoleIDs: []uint64{5}}
	vic := Membership{MemberID: 5, RoleIDs: []uint64{2}}
	ann := Membership{MemberID: 4, RoleIDs: []uint64{4}}
	deeActs := Actor{MemberID: 3, Permissions: roles[2].Permissions, Rank: 2}
	alActs := Actor{MemberID: 2, Permissions: AllPermissions, Rank: 3}
	adaActs := Actor{MemberID: 1, Permissions: AllPermissions, Rank: InstanceAdminRank}
	cases := []struct {
		name          string
		actor         Actor
		need          Permission
		target        Membership
		instanceAdmin bool
		want          error
	}{
		{"Al removes Dee", alActs, PermissionManageMembers, dee, false, nil},
		{"Dee re-roles Vic", deeActs, PermissionManageRoles, vic, false, nil},
		{"Dee cannot remove Al", deeActs, PermissionManageMembers, al, false, ErrMemberNotBelow},
		{"Dee cannot re-role someone at her own Rank", deeActs, PermissionManageRoles, Membership{MemberID: 6, RoleIDs: []uint64{5}}, false, ErrMemberNotBelow},
		{"Dee without approve", deeActs, PermissionApprove, vic, false, ErrMissing{Permission: PermissionApprove}},
		{"nobody manages the Instance admin", alActs, PermissionManageMembers, Membership{MemberID: 1}, true, ErrInstanceAdminFixed},
		{"nobody manages themselves", alActs, PermissionManageRoles, al, false, ErrSelf},
		{"not even the Instance admin manages the Guild Master", adaActs, PermissionManageMembers, ann, false, ErrGuildMaster},
		{"the Instance admin manages Al", adaActs, PermissionManageMembers, al, false, nil},
		{"the Guild Master manages Al", Actor{MemberID: 4, Permissions: AllPermissions, Rank: GuildMasterRank}, PermissionManageMembers, al, false, nil},
	}
	for _, c := range cases {
		if err := CanManage(g, c.actor, c.need, c.target, rank(c.target), c.instanceAdmin); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

func TestReorder(t *testing.T) {
	_, roles := hierarchy()
	positions := func(rs []Role) []int {
		out := []int{}
		for _, r := range rs {
			out = append(out, int(r.ID), r.Position)
		}
		return out
	}
	// Admin drags Deployer below Viewer.
	got, err := Reorder(roles, []uint64{4, 2, 5}, 3)
	if err != nil || !slices.Equal(positions(got), []int{4, 3, 2, 2, 5, 1}) {
		t.Errorf("Deployer below Viewer: %v %v", positions(got), err)
	}
	if _, err := Reorder(roles, []uint64{5, 4, 2}, 2); !errors.Is(err, ErrRoleNotBelow) {
		t.Errorf("Deployer moves itself above Admin: %v", err)
	}
	if _, err := Reorder(roles, []uint64{4, 2, 5}, 2); !errors.Is(err, ErrRoleNotBelow) {
		t.Errorf("Deployer moves itself down: %v", err)
	}
	if _, err := Reorder(roles, []uint64{4, 5, 2}, 2); err != nil {
		t.Errorf("Deployer leaves everything in place: %v", err)
	}
	if got, err := Reorder(roles, []uint64{2, 5, 4}, GuildMasterRank); err != nil || !slices.Equal(positions(got), []int{2, 3, 5, 2, 4, 1}) {
		t.Errorf("the Guild Master turns it around: %v %v", positions(got), err)
	}
	for _, order := range [][]uint64{{4, 5}, {4, 5, 5}, {4, 5, 1}, {4, 5, 2, 1}, {4, 5, 9}} {
		if _, err := Reorder(roles, order, GuildMasterRank); !errors.Is(err, ErrInvalidOrder) {
			t.Errorf("%v: %v", order, err)
		}
	}
}

func TestPlaceNewAndCloseGap(t *testing.T) {
	_, roles := hierarchy()
	up := PlaceNew(roles)
	if len(up) != 3 || up[0].Position != 2 || up[2].Position != 4 {
		t.Errorf("place new: %+v", up)
	}
	down := CloseGap(roles, roles[1])
	if len(down) != 2 || down[0].Position != 1 || down[1].Position != 2 {
		t.Errorf("close gap: %+v", down)
	}
}

func TestEveryPermissionIsDescribed(t *testing.T) {
	overridable := []string{}
	for _, p := range All() {
		if p.Key() == "" || p.Name() == "" || p.Description() == "" {
			t.Errorf("%d: %q %q %q", p, p.Key(), p.Name(), p.Description())
		}
		if p.Overridable() {
			overridable = append(overridable, p.Key())
		}
	}
	if len(All()) != len(AllPermissions.Keys()) {
		t.Errorf("All lists %d, the set holds %d", len(All()), len(AllPermissions.Keys()))
	}
	if !slices.Equal(overridable, []string{"view_resources", "see_secrets", "deploy", "manage_applications"}) {
		t.Errorf("overridable: %v", overridable)
	}
}

func TestCanAssignToAgent(t *testing.T) {
	_, roles := hierarchy()
	viewer, deployer, admin := roles[1], roles[2], roles[3]
	cases := []struct {
		name   string
		actor  Rank
		hirer  int
		role   Role
		wanted error
	}{
		{"a Role below the actor and the Hirer", 3, 3, deployer, nil},
		{"a Role at the Hirer's highest", 3, 2, deployer, ErrAboveHirer},
		{"a Role above the Hirer, below the actor", GuildMasterRank, 1, deployer, ErrAboveHirer},
		{"a Role at the actor's highest", 2, 3, deployer, ErrRoleNotBelow},
		{"the Base role", 3, 3, roles[0], ErrBaseRoleFixed},
		{"a Guild Master's Agent", GuildMasterRank, int(GuildMasterRank), admin, nil},
		{"an Instance admin's Agent", InstanceAdminRank, int(InstanceAdminRank), viewer, nil},
	}
	for _, c := range cases {
		if err := CanAssignToAgent(c.actor, c.hirer, c.role); !errors.Is(err, c.wanted) {
			t.Errorf("%s: %v, want %v", c.name, err, c.wanted)
		}
	}
}

func TestAgentRolesAbove(t *testing.T) {
	_, roles := hierarchy()
	agent := Membership{AgentID: 9, HirerID: 2, RoleIDs: []uint64{2, 5}}
	if !agent.IsAgent() || (Membership{MemberID: 2}).IsAgent() {
		t.Error("IsAgent")
	}
	for hirer, want := range map[int][]uint64{3: nil, 2: {5}, 1: {2, 5}, 0: {2, 5}} {
		if got := AgentRolesAbove(roles, agent, hirer); !slices.Equal(got, want) {
			t.Errorf("Hirer at %d: %v, want %v", hirer, got, want)
		}
	}
}
