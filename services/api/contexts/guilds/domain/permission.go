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

	endOfPermissions
)

// permissionKeys are the wire keys, in the glossary's order.
var permissionKeys = []string{
	"administrator", "view_resources", "see_secrets", "deploy",
	"manage_applications", "manage_servers", "manage_notifications",
	"manage_guild", "manage_members", "manage_roles", "hire_agents",
	"approve", "manage_budgets",
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
		found := false
		for i, known := range permissionKeys {
			if k == known {
				out |= 1 << i
				found = true
				break
			}
		}
		if !found {
			return 0, ErrUnknownPermission
		}
	}
	return out, nil
}
