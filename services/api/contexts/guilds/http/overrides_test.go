package http

import (
	"context"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/domain"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

type fakeOverrides []domain.Override

func (f fakeOverrides) ForGuild(context.Context, uint64) ([]domain.Override, error) { return f, nil }

func (f fakeOverrides) ForProject(_ context.Context, _, projectID uint64) ([]domain.Override, error) {
	var out []domain.Override
	for _, o := range f {
		if o.ProjectID == projectID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (fakeOverrides) ForgetProject(context.Context, uint64) error { return nil }

// TestInProject is the goal's Deployer: denied deploy on Project 1 and
// view_resources on Project 3, so of their Guild's Applications they deploy
// only the one in Project 2, see no trace of Project 3, and another Guild's
// Application is not found either.
func TestInProject(t *testing.T) {
	const base, deployer, me = 1, 5, 9
	overrides := fakeOverrides{
		{GuildID: 1, ProjectID: 1, RoleID: deployer, Deny: domain.Of(domain.PermissionDeploy)},
		{GuildID: 1, ProjectID: 3, RoleID: deployer, Deny: domain.Of(domain.PermissionViewResources)},
	}
	// Applications 10, 20 and 30 are in Projects 1, 2 and 3 of Guild 1; 40
	// is in Guild 2.
	m := InProject{
		Service: app.NewService(nil, nil, nil, nil, nil, overrides, nil),
		Name:    "application",
		ProjectOf: func(_ context.Context, id uint64) (uint64, uint64, bool, error) {
			if id == 40 {
				return 4, 2, true, nil
			}
			return id / 10, 1, id >= 10 && id <= 30, nil
		},
	}
	perms := domain.Of(domain.PermissionViewResources, domain.PermissionDeploy)
	pl := place{
		principal: identity.Principal{MemberID: me}, guild: domain.Guild{ID: 1}, permissions: perms, held: perms,
		at: app.Place{Guild: domain.Guild{ID: 1}, Permissions: perms, MemberID: me, Held: []uint64{base, deployer}, BaseRoleID: base},
	}
	cases := []struct {
		app    uint64
		found  bool
		deploy bool
	}{{10, true, false}, {20, true, true}, {30, false, false}, {40, false, false}, {50, false, false}}
	for _, c := range cases {
		got, found, err := m.resolve(context.Background(), pl, c.app)
		if err != nil {
			t.Fatal(err)
		}
		if found != c.found || got.permissions.Has(domain.PermissionDeploy) != c.deploy {
			t.Errorf("application %d: found %v deploy %v, want %v %v", c.app, found, got.permissions.Has(domain.PermissionDeploy), c.found, c.deploy)
		}
		if found && got.project != c.app/10 {
			t.Errorf("application %d: project %d", c.app, got.project)
		}
	}

	// A read token never deploys, whatever the Project allows.
	pl.principal = identity.TokenPrincipal(me, 1, "read")
	if got, _, _ := m.resolve(context.Background(), pl, 20); got.permissions.Has(domain.PermissionDeploy) {
		t.Error("a read token deploys in Project 2")
	}
}
