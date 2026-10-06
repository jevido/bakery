import { api, ApiError } from './api'

export type Role = 'viewer' | 'member' | 'admin' | 'owner'
export type Member = { id: number; name: string; email: string; role: Role; two_factor: boolean; instance_admin: boolean }
/** A Member as sign-in and the Profile answer them: the Role is per Guild, so only /me has it. */
export type Account = Omit<Member, 'role' | 'instance_admin'>

/** A Guild the signed-in Member may switch to, with their Role there. */
export type GuildPlace = { id: number; name: string; role: Exclude<Role, 'owner'> }
/** The Guild the Session acts in. */
export type CurrentGuild = { id: number; name: string }

type Me = { member: Member; guild: CurrentGuild | null; guilds: GuildPlace[]; instance_admin: boolean }

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

  /** Whether the signed-in Role may change anything (not a viewer). */
  canWrite = $derived(this.member !== null && this.member.role !== 'viewer')
  /** Whether the signed-in Role may read Secrets. */
  canSeeSecrets = $derived(this.canWrite)
  /** Whether the signed-in Role manages Servers, S3 storages, Known hosts and Members. */
  isAdmin = $derived(this.member?.role === 'admin' || this.member?.role === 'owner')

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
    this.state = 'signed-in'
  }

  signedOut() {
    this.member = null
    this.guild = null
    this.guilds = []
    this.instanceAdmin = false
    this.state = 'signed-out'
  }

  async logout() {
    await api('POST', '/logout')
    this.signedOut()
  }
}

export const session = new Session()
