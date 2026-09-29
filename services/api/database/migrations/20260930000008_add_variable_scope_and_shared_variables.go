package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000008AddVariableScopeAndSharedVariables gives every variable a
// scope (build, runtime; existing ones stay runtime only) and adds the
// Shared variables of Projects and Environments.
type M20260930000008AddVariableScopeAndSharedVariables struct{}

func (r *M20260930000008AddVariableScopeAndSharedVariables) Signature() string {
	return "20260930000008_add_variable_scope_and_shared_variables"
}

func (r *M20260930000008AddVariableScopeAndSharedVariables) Up() error {
	s := facades.Schema()
	if err := s.Table("env_vars", func(t schema.Blueprint) {
		t.Boolean("build").Default(false)
		t.Boolean("runtime").Default(true)
	}); err != nil {
		return err
	}
	for table, owner := range map[string]string{"project_variables": "project", "environment_variables": "environment"} {
		if err := s.Create(table, func(t schema.Blueprint) {
			t.ID()
			t.UnsignedBigInteger(owner + "_id")
			t.String("name")
			t.Text("value_encrypted")
			t.Boolean("build").Default(false)
			t.Boolean("runtime").Default(true)
			t.TimestampsTz()
			t.Foreign(owner + "_id").References("id").On(owner + "s").CascadeOnDelete()
			t.Unique(owner+"_id", "name")
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000008AddVariableScopeAndSharedVariables) Down() error {
	for _, table := range []string{"project_variables", "environment_variables"} {
		if err := facades.Schema().DropIfExists(table); err != nil {
			return err
		}
	}
	return facades.Schema().DropColumns("env_vars", []string{"build", "runtime"})
}
