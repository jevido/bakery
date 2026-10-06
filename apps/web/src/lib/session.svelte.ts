import { api, ApiError } from './api'

export type Role = 'viewer' | 'member' | 'admin' | 'owner'
export type Member = { id: number; name: string; email: string; role: Role; two_factor: boolean }
/** A Member as sign-in and the Profile answer them: the Role is per Guild, so only /me has it. */
export type Account = Omit<Member, 'role'>

type State = 'loading' | 'setup' | 'signed-out' | 'signed-in'

class Session {
  state = $state<State>('loading')
  member = $state.raw<Member | null>(null)

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
      const { member } = await api<{ member: Member }>('GET', '/me')
      this.signedIn(member)
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
    const { member } = await api<{ member: Member }>('GET', '/me')
    this.member = member
    this.state = 'signed-in'
  }

  /** Asks /me again, after the Current guild (and with it the Role) changed, as accepting an Invitation does. */
  async refresh() {
    const { member } = await api<{ member: Member }>('GET', '/me')
    this.member = member
    this.state = 'signed-in'
  }

  signedOut() {
    this.member = null
    this.state = 'signed-out'
  }

  async logout() {
    await api('POST', '/logout')
    this.signedOut()
  }
}

export const session = new Session()
