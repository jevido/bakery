import { api, ApiError } from './api'

export type Owner = { id: number; name: string; email: string }

type State = 'loading' | 'setup' | 'signed-out' | 'signed-in'

class Session {
  state = $state<State>('loading')
  owner = $state.raw<Owner | null>(null)

  /** Works out which screen to show: Setup, Login or the dashboard. */
  async load() {
    const { needed } = await api<{ needed: boolean }>('GET', '/setup')
    if (needed) {
      this.state = 'setup'
      return
    }
    try {
      const { owner } = await api<{ owner: Owner }>('GET', '/me')
      this.signedIn(owner)
    } catch (e) {
      if (!(e instanceof ApiError) || e.status !== 401) throw e
      this.state = 'signed-out'
    }
  }

  signedIn(owner: Owner) {
    this.owner = owner
    this.state = 'signed-in'
  }

  signedOut() {
    this.owner = null
    this.state = 'signed-out'
  }

  async logout() {
    await api('POST', '/logout')
    this.signedOut()
  }
}

export const session = new Session()
