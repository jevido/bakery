package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000020CreateS3StoragesTable holds the S3 storages Backups are
// uploaded to; the secret key is encrypted with the application key.
type M20260930000020CreateS3StoragesTable struct{}

func (r *M20260930000020CreateS3StoragesTable) Signature() string {
	return "20260930000020_create_s3_storages_table"
}

func (r *M20260930000020CreateS3StoragesTable) Up() error {
	return facades.Schema().Create("s3_storages", func(t schema.Blueprint) {
		t.ID()
		t.String("name", 100)
		t.String("endpoint", 255)
		t.String("region", 100)
		t.String("bucket", 63)
		t.String("prefix", 200).Default("")
		t.String("access_key", 255)
		t.Text("secret_key_encrypted")
		t.TimestampsTz()
		t.Unique("name")
	})
}

func (r *M20260930000020CreateS3StoragesTable) Down() error {
	return facades.Schema().DropIfExists("s3_storages")
}
