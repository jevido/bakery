package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000043RenameDiskAlmostFullToHighDiskUsage names the Server
// probe's disk flag as the glossary does: high disk usage.
type M20260930000043RenameDiskAlmostFullToHighDiskUsage struct{}

func (r *M20260930000043RenameDiskAlmostFullToHighDiskUsage) Signature() string {
	return "20260930000043_rename_disk_almost_full_to_high_disk_usage"
}

func (r *M20260930000043RenameDiskAlmostFullToHighDiskUsage) Up() error {
	return facades.Schema().Sql(`ALTER TABLE servers RENAME COLUMN disk_almost_full TO high_disk_usage`)
}

func (r *M20260930000043RenameDiskAlmostFullToHighDiskUsage) Down() error {
	return facades.Schema().Sql(`ALTER TABLE servers RENAME COLUMN high_disk_usage TO disk_almost_full`)
}
