package migrations

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// M20260930000040RenameWwwRedirectToRedirect gives the routing context's Www
// redirect Coolify's name and values: off → both, to_apex → non-www,
// to_www → www.
type M20260930000040RenameWwwRedirectToRedirect struct{}

func (r *M20260930000040RenameWwwRedirectToRedirect) Signature() string {
	return "20260930000040_rename_www_redirect_to_redirect"
}

func (r *M20260930000040RenameWwwRedirectToRedirect) Up() error {
	for _, q := range []string{
		`ALTER TABLE route_settings RENAME COLUMN www_redirect TO redirect`,
		`ALTER TABLE route_settings ALTER COLUMN redirect SET DEFAULT 'both'`,
		`UPDATE route_settings SET redirect = CASE redirect
			WHEN 'to_apex' THEN 'non-www' WHEN 'to_www' THEN 'www' ELSE 'both' END`,
	} {
		if err := facades.Schema().Sql(q); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260930000040RenameWwwRedirectToRedirect) Down() error {
	for _, q := range []string{
		`UPDATE route_settings SET redirect = CASE redirect
			WHEN 'non-www' THEN 'to_apex' WHEN 'www' THEN 'to_www' ELSE 'off' END`,
		`ALTER TABLE route_settings ALTER COLUMN redirect SET DEFAULT 'off'`,
		`ALTER TABLE route_settings RENAME COLUMN redirect TO www_redirect`,
	} {
		if err := facades.Schema().Sql(q); err != nil {
			return err
		}
	}
	return nil
}
