package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000004CreateKnownHostsTable holds deployments' Known hosts: the
// SSH host keys of git hosts, trusted on the first clone.
type M20260930000004CreateKnownHostsTable struct{}

func (r *M20260930000004CreateKnownHostsTable) Signature() string {
	return "20260930000004_create_known_hosts_table"
}

func (r *M20260930000004CreateKnownHostsTable) Up() error {
	return facades.Schema().Create("known_hosts", func(t schema.Blueprint) {
		t.ID()
		// The host as known_hosts writes it: example.com or [example.com]:2222.
		t.String("host")
		// known_hosts lines, one per key type.
		t.Text("keys")
		t.TimestampsTz()
		t.Unique("host")
	})
}

func (r *M20260930000004CreateKnownHostsTable) Down() error {
	return facades.Schema().DropIfExists("known_hosts")
}
