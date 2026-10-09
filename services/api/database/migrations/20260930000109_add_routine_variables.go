package migrations

// M20260930000109AddRoutineVariables adds a Routine's variables (work):
// the definitions of the {{name}} placeholders in its title and
// description.
type M20260930000109AddRoutineVariables struct{}

func (r *M20260930000109AddRoutineVariables) Signature() string {
	return "20260930000109_add_routine_variables"
}

func (r *M20260930000109AddRoutineVariables) Up() error {
	return sqls(`ALTER TABLE routines ADD COLUMN variables jsonb NOT NULL DEFAULT '[]'`)
}

func (r *M20260930000109AddRoutineVariables) Down() error {
	return sqls(`ALTER TABLE routines DROP COLUMN IF EXISTS variables`)
}
