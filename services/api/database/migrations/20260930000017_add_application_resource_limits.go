package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000017AddApplicationResourceLimits adds the Resource limits of
// an Application; 0 means unlimited, which every existing Application stays.
type M20260930000017AddApplicationResourceLimits struct{}

func (r *M20260930000017AddApplicationResourceLimits) Signature() string {
	return "20260930000017_add_application_resource_limits"
}

func (r *M20260930000017AddApplicationResourceLimits) Up() error {
	return facades.Schema().Table("applications", func(t schema.Blueprint) {
		t.Integer("memory_mb").Default(0)
		t.Float("cpus").Default(0)
	})
}

func (r *M20260930000017AddApplicationResourceLimits) Down() error {
	return facades.Schema().DropColumns("applications", []string{"memory_mb", "cpus"})
}
