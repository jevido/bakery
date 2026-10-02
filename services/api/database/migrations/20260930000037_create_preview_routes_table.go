package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000037CreatePreviewRoutesTable holds routing's Preview routes,
// one per (Application, Preview number).
type M20260930000037CreatePreviewRoutesTable struct{}

func (r *M20260930000037CreatePreviewRoutesTable) Signature() string {
	return "20260930000037_create_preview_routes_table"
}

func (r *M20260930000037CreatePreviewRoutesTable) Up() error {
	return facades.Schema().Sql(`CREATE TABLE IF NOT EXISTS preview_routes (
		id bigserial PRIMARY KEY,
		application_id bigint NOT NULL,
		preview integer NOT NULL,
		server_id bigint NOT NULL DEFAULT 0,
		domains text NOT NULL,
		container_name varchar(255) NOT NULL,
		container_port integer NOT NULL,
		created_at timestamptz,
		updated_at timestamptz,
		UNIQUE (application_id, preview)
	)`)
}

func (r *M20260930000037CreatePreviewRoutesTable) Down() error {
	return facades.Schema().Sql(`DROP TABLE IF EXISTS preview_routes`)
}
