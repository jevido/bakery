package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000012CreateApplicationDomainsTable lets an Application have
// several Domains: each existing Domain becomes the primary (position 0) of
// its Application, and applications.domain goes.
type M20260930000012CreateApplicationDomainsTable struct{}

func (r *M20260930000012CreateApplicationDomainsTable) Signature() string {
	return "20260930000012_create_application_domains_table"
}

func (r *M20260930000012CreateApplicationDomainsTable) Up() error {
	if err := facades.Schema().Create("application_domains", func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("application_id")
		t.String("domain")
		t.Integer("position")
		t.TimestampsTz()
		t.Foreign("application_id").References("id").On("applications").CascadeOnDelete()
		t.Unique("domain")
		t.Unique("application_id", "position")
	}); err != nil {
		return err
	}
	if _, err := facades.Schema().Orm().Query().Exec(`
		INSERT INTO application_domains (application_id, domain, position, created_at, updated_at)
		SELECT id, domain, 0, now(), now() FROM applications`); err != nil {
		return err
	}
	return facades.Schema().DropColumns("applications", []string{"domain"})
}

func (r *M20260930000012CreateApplicationDomainsTable) Down() error {
	if err := facades.Schema().Table("applications", func(t schema.Blueprint) {
		t.String("domain").Nullable()
	}); err != nil {
		return err
	}
	if _, err := facades.Schema().Orm().Query().Exec(`
		UPDATE applications a SET domain = d.domain
		FROM application_domains d WHERE d.application_id = a.id AND d.position = 0`); err != nil {
		return err
	}
	return facades.Schema().DropIfExists("application_domains")
}
