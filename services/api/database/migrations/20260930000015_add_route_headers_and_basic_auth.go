package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000015AddRouteHeadersAndBasicAuth adds Response headers (a JSON
// array) and Basic auth (the password only as a bcrypt hash) to the Route
// settings.
type M20260930000015AddRouteHeadersAndBasicAuth struct{}

func (r *M20260930000015AddRouteHeadersAndBasicAuth) Signature() string {
	return "20260930000015_add_route_headers_and_basic_auth"
}

func (r *M20260930000015AddRouteHeadersAndBasicAuth) Up() error {
	return facades.Schema().Table("route_settings", func(t schema.Blueprint) {
		t.Text("response_headers").Default("[]")
		t.Boolean("basic_auth_enabled").Default(false)
		t.String("basic_auth_username").Default("")
		t.String("basic_auth_password_hash").Default("")
	})
}

func (r *M20260930000015AddRouteHeadersAndBasicAuth) Down() error {
	return facades.Schema().DropColumns("route_settings", []string{"response_headers", "basic_auth_enabled", "basic_auth_username", "basic_auth_password_hash"})
}
