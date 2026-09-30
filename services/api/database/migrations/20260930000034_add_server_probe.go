package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000034AddServerProbe keeps what the Server probe (servers)
// needs between probes: failures in a row and whether the disk was found
// almost full.
type M20260930000034AddServerProbe struct{}

func (r *M20260930000034AddServerProbe) Signature() string {
	return "20260930000034_add_server_probe"
}

func (r *M20260930000034AddServerProbe) Up() error {
	for _, stmt := range []string{
		`ALTER TABLE servers ADD COLUMN IF NOT EXISTS failed_probes integer NOT NULL DEFAULT 0`,
		`ALTER TABLE servers ADD COLUMN IF NOT EXISTS disk_almost_full boolean NOT NULL DEFAULT false`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000034AddServerProbe) Down() error {
	for _, stmt := range []string{
		`ALTER TABLE servers DROP COLUMN IF EXISTS failed_probes`,
		`ALTER TABLE servers DROP COLUMN IF EXISTS disk_almost_full`,
	} {
		if err := facades.Schema().Sql(stmt); err != nil {
			return err
		}
	}
	return nil
}
