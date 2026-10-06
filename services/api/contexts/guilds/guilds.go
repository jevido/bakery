// Package guilds is what other contexts, the router and bootstrap may use
// from the guilds context. For now that is Boot, which makes Setup create
// the first Guild; the Current guild and the Role checks come next.
// Nothing else in contexts/guilds is for outside use.
package guilds

import (
	"github.com/jevido/bakery/services/api/contexts/guilds/app"
	"github.com/jevido/bakery/services/api/contexts/guilds/infra"
	"github.com/jevido/bakery/services/api/contexts/identity"
)

var service = app.NewService(infra.Guilds{}, infra.Memberships{})

// Boot subscribes guilds to what identity announces: Setup's Instance admin
// gets the first Guild, "Default", with an admin Membership.
func Boot() {
	identity.OnMemberSetUp(service.MakeFirstGuild)
}
