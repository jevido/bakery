package migrations

// M20260930000108AddRoutineWebhookTriggers adds what a Webhook trigger
// (work) keeps: its Public id, encrypted secret, Signing mode, Replay
// window and last Webhook delivery; and on a Routine run its Idempotency
// key, unique per trigger, and the Routine variables' values it ran with.
type M20260930000108AddRoutineWebhookTriggers struct{}

func (r *M20260930000108AddRoutineWebhookTriggers) Signature() string {
	return "20260930000108_add_routine_webhook_triggers"
}

func (r *M20260930000108AddRoutineWebhookTriggers) Up() error {
	return sqls(
		`ALTER TABLE routine_triggers
			ADD COLUMN public_id text NULL,
			ADD COLUMN secret_encrypted text NULL,
			ADD COLUMN signing_mode text NULL,
			ADD COLUMN replay_window_sec integer NULL,
			ADD COLUMN last_rotated_at timestamptz NULL,
			ADD COLUMN last_delivery_status text NULL,
			ADD COLUMN last_delivery_at timestamptz NULL`,
		`CREATE UNIQUE INDEX routine_triggers_public_id_unique ON routine_triggers (public_id) WHERE public_id IS NOT NULL`,
		`ALTER TABLE routine_runs
			ADD COLUMN idempotency_key text NULL,
			ADD COLUMN variables jsonb NULL`,
		`CREATE UNIQUE INDEX routine_runs_trigger_idempotency_key_unique ON routine_runs (trigger_id, idempotency_key) WHERE idempotency_key IS NOT NULL`,
	)
}

func (r *M20260930000108AddRoutineWebhookTriggers) Down() error {
	return sqls(
		`DROP INDEX IF EXISTS routine_runs_trigger_idempotency_key_unique`,
		`ALTER TABLE routine_runs DROP COLUMN IF EXISTS variables, DROP COLUMN IF EXISTS idempotency_key`,
		`DROP INDEX IF EXISTS routine_triggers_public_id_unique`,
		`ALTER TABLE routine_triggers
			DROP COLUMN IF EXISTS last_delivery_at, DROP COLUMN IF EXISTS last_delivery_status, DROP COLUMN IF EXISTS last_rotated_at,
			DROP COLUMN IF EXISTS replay_window_sec, DROP COLUMN IF EXISTS signing_mode, DROP COLUMN IF EXISTS secret_encrypted,
			DROP COLUMN IF EXISTS public_id`,
	)
}
