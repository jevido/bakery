package domain

import (
	"errors"
	"slices"
)

var ErrNotOverridable = errors.New("a project can only override view resources, see secrets, deploy and manage applications")

// Override is a Permission override on a Project for one Role or one
// Member (exactly one of RoleID and MemberID is set): the Permissions it
// allows and denies there. A Permission in neither inherits.
type Override struct {
	ID        uint64
	GuildID   uint64
	ProjectID uint64
	RoleID    uint64
	MemberID  uint64
	Allow     Permissions
	Deny      Permissions
}

// NewOverride validates what an Override allows and denies: only
// overridable Permissions, and none both allowed and denied.
func NewOverride(allow, deny Permissions) error {
	if (allow|deny)&^overridable != 0 || allow&deny != 0 {
		return ErrNotOverridable
	}
	return nil
}

// Empty reports whether the Override sets nothing, so it may as well not
// exist.
func (o Override) Empty() bool { return o.Allow == 0 && o.Deny == 0 }

// Resolve is what someone holding guild in a Guild may do in one of its
// Projects. overrides are that Project's; held are the ids of the Roles
// they hold there, baseRoleID the Base role's (held by everyone);
// memberID is who they are. With administrator (the Guild Master and the
// Instance admin hold it) nothing is overridden. Otherwise, as Discord
// resolves a channel: the Base role's override first, then the other held
// Roles' (a deny on any of them beats an allow on another), then the
// Member's own, which beats both. Only overridable Permissions change.
func Resolve(guild Permissions, overrides []Override, baseRoleID uint64, held []uint64, memberID uint64) Permissions {
	if guild.Has(PermissionAdministrator) {
		return guild
	}
	out := guild
	apply := func(allow, deny Permissions) {
		out = out.Without(deny & overridable).Union(allow & overridable &^ deny)
	}
	var allow, deny Permissions
	var own *Override
	for i, o := range overrides {
		switch {
		case o.MemberID != 0:
			if o.MemberID == memberID {
				own = &overrides[i]
			}
		case o.RoleID == baseRoleID:
			apply(o.Allow, o.Deny)
		case slices.Contains(held, o.RoleID):
			allow, deny = allow.Union(o.Allow), deny.Union(o.Deny)
		}
	}
	apply(allow, deny)
	if own != nil {
		apply(own.Allow, own.Deny)
	}
	return out
}
