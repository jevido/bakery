package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000013AddRouteDomains lets a Route serve every Domain of its
// Application: routes.domain becomes routes.domains, a JSON array. That the
// Domains are unique is projects' rule, so no index replaces the old one.
type M20260930000013AddRouteDomains struct{}

func (r *M20260930000013AddRouteDomains) Signature() string {
	return "20260930000013_add_route_domains"
}

func (r *M20260930000013AddRouteDomains) Up() error {
	if err := facades.Schema().Table("routes", func(t schema.Blueprint) {
		t.Text("domains").Default("[]")
	}); err != nil {
		return err
	}
	if _, err := facades.Orm().Query().Exec(`UPDATE routes SET domains = json_build_array(domain)::text`); err != nil {
		return err
	}
	// Dropping the column drops its unique index with it.
	return facades.Schema().DropColumns("routes", []string{"domain"})
}

func (r *M20260930000013AddRouteDomains) Down() error {
	if err := facades.Schema().Table("routes", func(t schema.Blueprint) {
		t.String("domain").Nullable()
	}); err != nil {
		return err
	}
	if _, err := facades.Orm().Query().Exec(`UPDATE routes SET domain = domains::json->>0`); err != nil {
		return err
	}
	return facades.Schema().DropColumns("routes", []string{"domains"})
}
