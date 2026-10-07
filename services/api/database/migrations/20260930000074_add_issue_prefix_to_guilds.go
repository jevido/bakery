package migrations

// M20260930000074AddIssuePrefixToGuilds gives every Guild the Issue prefix
// its Issues are numbered with, derived as the guilds domain does
// (DeriveIssuePrefix, FreeIssuePrefix), in id order: the first three of
// the name's letters A to Z, GLD without any, then A, B, … Z, AA, … until
// no earlier Guild has it. "Default" becomes DEF.
type M20260930000074AddIssuePrefixToGuilds struct{}

func (r *M20260930000074AddIssuePrefixToGuilds) Signature() string {
	return "20260930000074_add_issue_prefix_to_guilds"
}

func (r *M20260930000074AddIssuePrefixToGuilds) Up() error {
	return sqls(
		`ALTER TABLE guilds ADD COLUMN issue_prefix varchar(5)`,
		`DO $$
		DECLARE
			g record;
			base text;
			candidate text;
			suffix text;
			n int;
			k int;
		BEGIN
			FOR g IN SELECT id, name FROM guilds ORDER BY id LOOP
				base := left(regexp_replace(upper(g.name), '[^A-Z]', '', 'g'), 3);
				IF base = '' THEN
					base := 'GLD';
				END IF;
				n := 0;
				LOOP
					suffix := '';
					k := n;
					WHILE k > 0 LOOP
						k := k - 1;
						suffix := chr(65 + k % 26) || suffix;
						k := k / 26;
					END LOOP;
					candidate := right(base || suffix, 5);
					EXIT WHEN NOT EXISTS (SELECT 1 FROM guilds WHERE issue_prefix = candidate);
					n := n + 1;
				END LOOP;
				UPDATE guilds SET issue_prefix = candidate WHERE id = g.id;
			END LOOP;
		END
		$$`,
		`ALTER TABLE guilds ALTER COLUMN issue_prefix SET NOT NULL`,
		`CREATE UNIQUE INDEX guilds_issue_prefix_unique ON guilds (issue_prefix)`,
	)
}

func (r *M20260930000074AddIssuePrefixToGuilds) Down() error {
	return sqls(`ALTER TABLE guilds DROP COLUMN issue_prefix`)
}
