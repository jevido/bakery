package domain

import (
	"errors"
	"math/bits"
)

var ErrUnknownPermission = errors.New("unknown permission")

// Permission is one thing a Role allows in its Guild, from the glossary's
// fixed list. The bit values are stored in roles.permissions: append new
// Permissions at the end and never renumber one.
type Permission uint64

const (
	PermissionAdministrator Permission = 1 << iota
	PermissionViewResources
	PermissionSeeSecrets
	PermissionDeploy
	PermissionManageApplications
	PermissionManageServers
	PermissionManageNotifications
	PermissionManageGuild
	PermissionManageMembers
	PermissionManageRoles
	PermissionHireAgents
	PermissionApprove
	PermissionManageBudgets
	PermissionManageWork
	PermissionManageSkills

	endOfPermissions
)

// permissionKeys are the wire keys, in the glossary's order.
var permissionKeys = []string{
	"administrator", "view_resources", "see_secrets", "deploy",
	"manage_applications", "manage_servers", "manage_notifications",
	"manage_guild", "manage_members", "manage_roles", "hire_agents",
	"approve", "manage_budgets", "manage_work", "manage_skills",
}

// permissionNames are the glossary's names, in the same order.
var permissionNames = []string{
	"Administrator", "View resources", "See secrets", "Deploy",
	"Manage applications", "Manage servers", "Manage notifications",
	"Manage guild", "Manage members", "Manage roles", "Hire agents",
	"Approve", "Manage budgets", "Manage work", "Manage skills",
}

// permissionDescriptions say what each Permission allows, as the glossary
// does, in the same order.
var permissionDescriptions = []string{
	"Every permission. Ignores permission overrides.",
	"Read the guild's projects and everything in them, servers, S3 storages and notification channels, without secrets.",
	"Read secrets: environment variables, database passwords and private keys.",
	"Deploy, restart, stop, start, cancel and roll back.",
	"Create, change and delete projects, environments, applications, databases, scheduled backups, services, their variables and routing settings.",
	"Manage servers, S3 storages and known hosts.",
	"Manage notification channels.",
	"Change the guild's name, description and issue prefix.",
	"Invite and remove members and reset their two-factor authentication.",
	"Manage roles below your highest role, who holds them, and permission overrides.",
	"Hire agents into the guild.",
	"Answer approvals.",
	"Set the guild's budgets.",
	"Create and change goals and issues, and comment on issues.",
	"Create, change and delete the guild's skills.",
}

// overridable are the Permissions a Permission override can set on a
// Project.
const overridable = Permissions(PermissionViewResources | PermissionSeeSecrets | PermissionDeploy | PermissionManageApplications)

// All lists every Permission in the glossary's order.
func All() []Permission {
	out := make([]Permission, len(permissionKeys))
	for i := range out {
		out[i] = Permission(1) << i
	}
	return out
}

// Description says what the Permission allows, "" for no single known
// Permission.
func (p Permission) Description() string {
	if p.Key() == "" {
		return ""
	}
	return permissionDescriptions[bits.TrailingZeros64(uint64(p))]
}

// Overridable reports whether a Permission override on a Project can set
// p.
func (p Permission) Overridable() bool { return Permissions(p)&overridable != 0 && p.Key() != "" }

// ParsePermission reads one Permission from its wire key;
// ErrUnknownPermission for a key that is not on the list.
func ParsePermission(key string) (Permission, error) {
	for i, known := range permissionKeys {
		if key == known {
			return Permission(1) << i, nil
		}
	}
	return 0, ErrUnknownPermission
}

// Name is the Permission's name in the glossary, "" for no single known
// Permission.
func (p Permission) Name() string {
	if p.Key() == "" {
		return ""
	}
	return permissionNames[bits.TrailingZeros64(uint64(p))]
}

// Key is the Permission's wire key, "" for no single known Permission.
func (p Permission) Key() string {
	if bits.OnesCount64(uint64(p)) != 1 || p >= endOfPermissions {
		return ""
	}
	return permissionKeys[bits.TrailingZeros64(uint64(p))]
}

// Permissions is a set of Permissions.
type Permissions uint64

// AllPermissions holds every Permission.
const AllPermissions = Permissions(endOfPermissions - 1)

// Of makes the set of these Permissions.
func Of(ps ...Permission) Permissions {
	var out Permissions
	for _, p := range ps {
		out |= Permissions(p)
	}
	return out
}

// Has reports whether the set allows p: it holds p, or administrator,
// which allows everything.
func (s Permissions) Has(p Permission) bool {
	return s&Permissions(PermissionAdministrator) != 0 || s&Permissions(p) == Permissions(p)
}

// Expand is the set with administrator spelled out as every Permission,
// so taking Permissions away from it means something.
func (s Permissions) Expand() Permissions {
	if s&Permissions(PermissionAdministrator) != 0 {
		return AllPermissions
	}
	return s
}

// Without is s with every Permission in o taken away. Take administrator
// away from an Expand-ed set, or the rest still counts as granted.
func (s Permissions) Without(o Permissions) Permissions { return s &^ o }

// Union is every Permission in s or o.
func (s Permissions) Union(o Permissions) Permissions { return s | o }

// Keys lists the wire keys of the Permissions the set holds, in the
// glossary's order; never nil.
func (s Permissions) Keys() []string {
	out := []string{}
	for i, k := range permissionKeys {
		if s&(1<<i) != 0 {
			out = append(out, k)
		}
	}
	return out
}

// ParsePermissions reads a set from wire keys; ErrUnknownPermission for a
// key that is not on the list.
func ParsePermissions(keys []string) (Permissions, error) {
	var out Permissions
	for _, k := range keys {
		p, err := ParsePermission(k)
		if err != nil {
			return 0, err
		}
		out |= Permissions(p)
	}
	return out, nil
}
