package migrations

// M20260930000092AddRunKey adds a Run's Run key (agents), kept only as its
// SHA-256: ClaimRun mints the key and answers it once, and identity finds
// the Run by this hash while it is running.
type M20260930000092AddRunKey struct{}

func (r *M20260930000092AddRunKey) Signature() string {
	return "20260930000092_add_run_key"
}

func (r *M20260930000092AddRunKey) Up() error {
	return sqls(
		`ALTER TABLE runs ADD COLUMN key_hash text`,
		`CREATE UNIQUE INDEX runs_key_hash_unique ON runs (key_hash) WHERE key_hash IS NOT NULL`,
	)
}

func (r *M20260930000092AddRunKey) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS runs_key_hash_unique`,
		`ALTER TABLE runs DROP COLUMN IF EXISTS key_hash`,
	)
}
