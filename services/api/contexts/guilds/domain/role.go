package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxRoleNameLength is in characters.
const MaxRoleNameLength = 100

var (
	ErrInvalidRole      = errors.New("role must be viewer, member or admin")
	ErrInvalidRoleName  = errors.New("a role's name is required and at most 100 characters")
	ErrInvalidRoleColor = errors.New("a role's color is #rrggbb")
)

// The seeded Roles every Guild starts with.
const (
	BaseRoleName = "@everyone"
	ViewerRole   = "Viewer"
	MemberRole   = "Member"
	AdminRole    = "Admin"
)

// Role is a named set of Permissions in one Guild, with a color and a
// Position. The Base role is held by every Member and never stored on a
// Membership.
type Role struct {
	ID          uint64
	GuildID     uint64
	Name        string
	Color       string
	Position    int
	Permissions Permissions
	Base        bool
}

var colorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Rename trims name and refuses an empty or too long one, and any new
// name for the Base role.
func (r *Role) Rename(name string) error {
	name = strings.TrimSpace(name)
	if r.Base && name != r.Name {
		return ErrBaseRoleFixed
	}
	if name == "" || utf8.RuneCountInString(name) > MaxRoleNameLength {
		return ErrInvalidRoleName
	}
	r.Name = name
	return nil
}

// Recolor sets the color, #rrggbb in either case, stored lower case; the
// Base role keeps its own.
func (r *Role) Recolor(color string) error {
	color = strings.ToLower(strings.TrimSpace(color))
	if r.Base && color != r.Color {
		return ErrBaseRoleFixed
	}
	if !colorPattern.MatchString(color) {
		return ErrInvalidRoleColor
	}
	r.Color = color
	return nil
}

// SeedRoles are the Roles a new Guild starts with, by Position: the Base
// role, Viewer, Member and Admin, which keep the meaning of the former
// viewer, member and admin.
func SeedRoles(guildID uint64) []Role {
	return []Role{
		{GuildID: guildID, Name: BaseRoleName, Color: "#99aab5", Position: 0, Base: true},
		{GuildID: guildID, Name: ViewerRole, Color: "#95a5a6", Position: 1, Permissions: Of(PermissionViewResources)},
		{GuildID: guildID, Name: MemberRole, Color: "#3498db", Position: 2,
			Permissions: Of(PermissionViewResources, PermissionSeeSecrets, PermissionDeploy, PermissionManageApplications)},
		{GuildID: guildID, Name: AdminRole, Color: "#e74c3c", Position: 3, Permissions: Of(PermissionAdministrator)},
	}
}

// SeededRoleName is the name of the seeded Role a former role ("viewer",
// "member" or "admin") became; ErrInvalidRole for anything else.
func SeededRoleName(former string) (string, error) {
	switch former {
	case "viewer":
		return ViewerRole, nil
	case "member":
		return MemberRole, nil
	case "admin":
		return AdminRole, nil
	}
	return "", ErrInvalidRole
}

// SeededRole finds the seeded Role a former role names among a Guild's
// Roles; false when it was renamed or deleted.
func SeededRole(roles []Role, former string) (Role, bool, error) {
	name, err := SeededRoleName(former)
	if err != nil {
		return Role{}, false, err
	}
	for _, r := range roles {
		if !r.Base && r.Name == name {
			return r, true, nil
		}
	}
	return Role{}, false, nil
}

// DefaultRoleColor is a new Role's color when none is given, Discord's.
const DefaultRoleColor = "#99aab5"

// NewRole validates a new Role of the Guild, without an ID or Position.
func NewRole(guildID uint64, name, color string, permissions Permissions) (Role, error) {
	r := Role{GuildID: guildID, Permissions: permissions}
	if err := r.Rename(name); err != nil {
		return Role{}, err
	}
	if strings.TrimSpace(color) == "" {
		color = DefaultRoleColor
	}
	if err := r.Recolor(color); err != nil {
		return Role{}, err
	}
	return r, nil
}
