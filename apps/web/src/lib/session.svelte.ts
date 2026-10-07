import { api, ApiError } from './api'

export type Role = 'viewer' | 'member' | 'admin' | 'owner'
export type Member = {
  id: number
  name: string
  email: string
  role: Role
  two_factor: boolean
  instance_admin: boolean
  /** Whether they are the Current guild's Guild Master. */
  guild_master: boolean
}
/** A Member as sign-in and the Profile answer them: the Role is per Guild, so only /me has it. */
export type Account = Omit<Member, 'role' | 'instance_admin' | 'guild_master'>
/** A Member named on a Guild Master or a Transfer offer. */
export type Person = { id: number; name: string; email: string }
/** An open Transfer offer of a Guild's Guild Master; `guild` only on the offers to the signed-in Member. */
export type Offer = {
  id: number
  guild?: { id: number; name: string }
  from: Person | null
  to: Person | null
  created_at: string
  expires_at: string
}

/** A Permission's wire key, from the glossary's fixed list. */
export type Permission =
  | 'administrator'
  | 'view_resources'
  | 'see_secrets'
  | 'deploy'
  | 'manage_applications'
  | 'manage_servers'
  | 'manage_notifications'
  | 'manage_guild'
  | 'manage_members'
  | 'manage_roles'
  | 'hire_agents'
  | 'approve'
  | 'manage_budgets'

/** A Guild the signed-in Member may switch to, with their former role and Permissions there. */
export type GuildPlace = { id: number; name: string; role: Exclude<Role, 'owner'>; permissions: Permission[] }
/** The Guild the Session acts in. */
export type CurrentGuild = { id: number; name: string }

type Me = {
  member: Member
  guild: CurrentGuild | null
  guilds: GuildPlace[]
  instance_admin: boolean
  guild_master: boolean
  offers: Offer[]
  permissions: Permission[]
}

type State = 'loading' | 'setup' | 'signed-out' | 'signed-in'

class Session {
  state = $state<State>('loading')
  member = $state.raw<Member | null>(null)
  /** The Current guild; null for a Member in no Guild. */
  guild = $state.raw<CurrentGuild | null>(null)
  /** The Guilds the Member may switch to. */
  guilds = $state.raw<GuildPlace[]>([])
  /** Whether the Member runs the installation (the Local server among it). */
  instanceAdmin = $state(false)
  /** Whether the Member is the Current guild's Guild Master. */
  guildMaster = $state(false)
  /** The open Transfer offers to the Member, in every Guild. */
  offers = $state.raw<Offer[]>([])

  /** The Member's Permissions in the Current guild. */
  permissions = $state.raw<Permission[]>([])

  /** Whether the Member may use p in the Current guild; administrator allows everything. */
  can(p: Permission): boolean {
    return this.permissions.includes('administrator') || this.permissions.includes(p)
  }

  /** Works out which screen to show: Setup, Login or the dashboard. */
  async load() {
    const { needed } = await api<{ needed: boolean }>('GET', '/setup')
    if (needed) {
      this.state = 'setup'
      return
    }
    try {
      this.apply(await api<Me>('GET', '/me'))
    } catch (e) {
      if (!(e instanceof ApiError) || e.status !== 401) throw e
      this.state = 'signed-out'
    }
  }

  /** Takes the Member a sign-in or a Profile change answered, and their Role in the Current guild from /me. */
  async signedIn(account: Account) {
    if (this.member?.id === account.id) {
      this.member = { ...this.member, ...account }
      this.state = 'signed-in'
      return
    }
    this.apply(await api<Me>('GET', '/me'))
  }

  /** Asks /me again, after the Current guild (and with it the Role) changed, as accepting an Invitation does. */
  async refresh() {
    this.apply(await api<Me>('GET', '/me'))
  }

  /** Makes the Guild the Current guild; every page shows that Guild's things from then on. */
  async switchGuild(id: number) {
    await api('POST', `/guilds/${id}/switch`)
    await this.refresh()
  }

  private apply(me: Me) {
    this.member = me.member
    this.guild = me.guild
    this.guilds = me.guilds
    this.instanceAdmin = me.instance_admin
    this.guildMaster = me.guild_master
    this.offers = me.offers
    this.permissions = me.permissions
    this.state = 'signed-in'
  }

  signedOut() {
    this.member = null
    this.guild = null
    this.guilds = []
    this.instanceAdmin = false
    this.guildMaster = false
    this.offers = []
    this.permissions = []
    this.state = 'signed-out'
  }

  async logout() {
    await api('POST', '/logout')
    this.signedOut()
  }
}

export const session = new Session()
