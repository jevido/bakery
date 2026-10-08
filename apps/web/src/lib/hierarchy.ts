// The Guild's hierarchy as the API's domain applies it (RankOf, CanAssign,
// CanManage in contexts/guilds/domain/hierarchy.go), so the Roles and
// Members pages only offer what the server will allow. The server stays
// the judge; these only hide what it would refuse.
import { session } from './session.svelte'
import type { Permission } from './session.svelte'
import type { GuildRole, Member } from './types'

/** Above every Role, for the Instance admin. */
const instanceAdminRank = Number.MAX_SAFE_INTEGER - 1
/** Above the Instance admin, for the Guild Master. */
const guildMasterRank = Number.MAX_SAFE_INTEGER

type Ranked = Pick<Member, 'guild_master' | 'instance_admin' | 'roles'>

/** Where m stands among roles: the Position of their highest Role, 0 with only @everyone. */
export function rankOf(m: Ranked, roles: GuildRole[]): number {
  if (m.guild_master) return guildMasterRank
  if (m.instance_admin) return instanceAdminRank
  const held = new Set((m.roles ?? []).map((r) => r.id))
  return Math.max(0, ...roles.filter((r) => !r.base && held.has(r.id)).map((r) => r.position))
}

/** The signed-in Member's Rank among roles. */
export function myRank(roles: GuildRole[]): number {
  return session.member ? rankOf(session.member, roles) : 0
}

/** Whether the signed-in Member may edit role: one below their highest, or @everyone's Permissions. */
export function canEditRole(role: GuildRole, roles: GuildRole[]): boolean {
  return session.can('manage_roles') && (role.base || myRank(roles) > role.position)
}

/** Whether the signed-in Member may give or take role: below their highest, never @everyone. */
export function canAssign(role: GuildRole, roles: GuildRole[]): boolean {
  return session.can('manage_roles') && !role.base && myRank(roles) > role.position
}

/** Whether the signed-in Member, with need, may change target: never themselves, the Guild Master or the Instance admin, and only below their own highest Role. */
export function canManage(target: Member, need: Permission, roles: GuildRole[]): boolean {
  return (
    session.can(need) &&
    !target.instance_admin &&
    !target.guild_master &&
    target.id !== session.member?.id &&
    myRank(roles) > rankOf(target, roles)
  )
}

/** Whether the signed-in Member may give an Agent they hire role: below their highest, never @everyone (CanAssignToAgent; manage_roles is not needed). */
export function canGiveAgent(role: GuildRole, roles: GuildRole[]): boolean {
  return !role.base && myRank(roles) > role.position
}
