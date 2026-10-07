package domain

import "testing"

func TestResolve(t *testing.T) {
	const base, deployer, ops, me = 1, 2, 3, 7
	member := Of(PermissionViewResources, PermissionSeeSecrets, PermissionDeploy, PermissionManageApplications)
	cases := []struct {
		name      string
		guild     Permissions
		overrides []Override
		held      []uint64
		want      Permissions
	}{
		{"no overrides inherit the guild's", member, nil, []uint64{deployer}, member},
		{"a held Role's deny takes it away", member,
			[]Override{{RoleID: deployer, Deny: Of(PermissionDeploy)}}, []uint64{deployer},
			member.Without(Of(PermissionDeploy))},
		{"a Role not held changes nothing", member,
			[]Override{{RoleID: ops, Deny: Of(PermissionDeploy)}}, []uint64{deployer}, member},
		{"Role deny beats Role allow", Of(PermissionViewResources),
			[]Override{{RoleID: deployer, Deny: Of(PermissionDeploy)}, {RoleID: ops, Allow: Of(PermissionDeploy)}},
			[]uint64{deployer, ops}, Of(PermissionViewResources)},
		{"a Role allow beats the Base role's deny", member,
			[]Override{{RoleID: base, Deny: Of(PermissionViewResources)}, {RoleID: deployer, Allow: Of(PermissionViewResources)}},
			[]uint64{deployer}, member},
		{"the Base role's deny counts for everyone", member,
			[]Override{{RoleID: base, Deny: Of(PermissionViewResources)}}, nil,
			member.Without(Of(PermissionViewResources))},
		{"the Member's allow beats a Role deny", member,
			[]Override{{RoleID: deployer, Deny: Of(PermissionDeploy)}, {MemberID: me, Allow: Of(PermissionDeploy)}},
			[]uint64{deployer}, member},
		{"the Member's deny beats a Role allow", Of(PermissionViewResources),
			[]Override{{RoleID: deployer, Allow: Of(PermissionDeploy)}, {MemberID: me, Deny: Of(PermissionDeploy, PermissionViewResources)}},
			[]uint64{deployer}, 0},
		{"another Member's override changes nothing", member,
			[]Override{{MemberID: me + 1, Deny: Of(PermissionDeploy)}}, nil, member},
		{"an allow grants what the guild does not", Of(PermissionViewResources),
			[]Override{{RoleID: deployer, Allow: Of(PermissionDeploy)}}, []uint64{deployer},
			Of(PermissionViewResources, PermissionDeploy)},
		{"administrator ignores overrides", Of(PermissionAdministrator),
			[]Override{{RoleID: deployer, Deny: Of(PermissionDeploy)}}, []uint64{deployer}, Of(PermissionAdministrator)},
		{"guild-wide Permissions are never overridden", Of(PermissionManageServers),
			[]Override{{RoleID: deployer, Deny: Of(PermissionManageServers)}}, []uint64{deployer}, Of(PermissionManageServers)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Resolve(c.guild, c.overrides, base, c.held, me); got != c.want {
				t.Errorf("Resolve = %v, want %v", got.Keys(), c.want.Keys())
			}
		})
	}
}

func TestNewOverride(t *testing.T) {
	if err := NewOverride(Of(PermissionDeploy), Of(PermissionSeeSecrets)); err != nil {
		t.Errorf("deploy/see_secrets: %v", err)
	}
	if err := NewOverride(Of(PermissionManageServers), 0); err != ErrNotOverridable {
		t.Errorf("manage_servers: %v", err)
	}
	if err := NewOverride(Of(PermissionDeploy), Of(PermissionDeploy)); err != ErrNotOverridable {
		t.Errorf("deploy both ways: %v", err)
	}
}
