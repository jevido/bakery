package migrations

// M20260930000111CreateSkillsTables keeps a Guild's Skills (agents), each
// Skill's files by path, and the Agent skills of each Agent. A Skill an
// Agent still has cannot be deleted from under it (on delete restrict);
// agents removes a terminated Agent's rows first.
type M20260930000111CreateSkillsTables struct{}

func (r *M20260930000111CreateSkillsTables) Signature() string {
	return "20260930000111_create_skills_tables"
}

func (r *M20260930000111CreateSkillsTables) Up() error {
	return sqls(
		`CREATE TABLE skills (
			id bigserial PRIMARY KEY,
			guild_id bigint NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
			slug text NOT NULL,
			name text NOT NULL,
			description text NOT NULL DEFAULT '',
			created_by_member_id bigint REFERENCES users (id) ON DELETE SET NULL,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL,
			UNIQUE (guild_id, slug)
		)`,
		`CREATE TABLE skill_files (
			skill_id bigint NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
			path text NOT NULL,
			content bytea NOT NULL,
			executable boolean NOT NULL DEFAULT false,
			updated_at timestamptz NOT NULL,
			PRIMARY KEY (skill_id, path)
		)`,
		`CREATE TABLE agent_skills (
			agent_id bigint NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
			skill_id bigint NOT NULL REFERENCES skills (id) ON DELETE RESTRICT,
			PRIMARY KEY (agent_id, skill_id)
		)`,
		`CREATE INDEX agent_skills_skill_id_index ON agent_skills (skill_id)`,
	)
}

func (r *M20260930000111CreateSkillsTables) Down() error {
	return sqls(
		`DROP TABLE IF EXISTS agent_skills`,
		`DROP TABLE IF EXISTS skill_files`,
		`DROP TABLE IF EXISTS skills`,
	)
}
