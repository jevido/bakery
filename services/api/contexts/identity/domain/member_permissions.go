package domain

import "slices"

// MemberPermissions are the wire keys of the Permissions a Member holds in
// the Guild a request acts in (`administrator`, `see_secrets`, …). Guilds
// holds the Roles that give them; identity is told them and uses them only
// to cap the Token permissions an API token may carry.
type MemberPermissions []string

func (m MemberPermissions) has(key string) bool {
	return slices.Contains(m, key) || slices.Contains(m, "administrator")
}

// MayGrant reports whether a Member with these Permissions may put p on a
// token: root needs administrator, write manage_applications, deploy
// deploy, read:sensitive see_secrets; read anyone may.
func (m MemberPermissions) MayGrant(p Permission) bool {
	switch p {
	case PermissionRoot:
		return m.has("administrator")
	case PermissionWrite:
		return m.has("manage_applications")
	case PermissionDeploy:
		return m.has("deploy")
	case PermissionReadSensitive:
		return m.has("see_secrets")
	}
	return true
}
