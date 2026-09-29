package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000006AddApplicationHealthChecks holds an Application's Health
// check; off by default, times in seconds.
type M20260930000006AddApplicationHealthChecks struct{}

func (r *M20260930000006AddApplicationHealthChecks) Signature() string {
	return "20260930000006_add_application_health_checks"
}

func (r *M20260930000006AddApplicationHealthChecks) Up() error {
	return facades.Schema().Table("applications", func(t schema.Blueprint) {
		t.Boolean("health_check_enabled").Default(false)
		t.String("health_check_path").Default("/")
		t.Integer("health_check_interval").Default(5)
		t.Integer("health_check_timeout").Default(5)
		t.Integer("health_check_retries").Default(10)
		t.Integer("health_check_start_period").Default(0)
	})
}

func (r *M20260930000006AddApplicationHealthChecks) Down() error {
	return facades.Schema().DropColumns("applications", []string{
		"health_check_enabled", "health_check_path", "health_check_interval",
		"health_check_timeout", "health_check_retries", "health_check_start_period",
	})
}
