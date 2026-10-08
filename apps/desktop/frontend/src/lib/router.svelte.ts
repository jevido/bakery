/**
 * The Desktop app's own small hash router: which connected Bakery (by its
 * place in the list), which of its Guilds, and which page.
 *
 *   #/b/{bakery}                          the Bakery, before a Guild is picked
 *   #/b/{bakery}/g/{guild}/agents[/{tab}]  the Guild's Agents on a tab
 *   #/b/{bakery}/g/{guild}/agents/{id}     one Agent
 */
import type { AgentsTab } from './desktop'

export const agentsTabs: readonly AgentsTab[] = ['all', 'active', 'paused', 'terminated']

export type Route =
  | { page: 'home'; bakery: number | null }
  | { page: 'guild'; bakery: number; guild: number }
  | { page: 'agents'; bakery: number; guild: number; tab: AgentsTab }
  | { page: 'agent'; bakery: number; guild: number; id: number }

export function parse(hash: string): Route {
  const parts = hash.replace(/^#\/?/, '').split('?')[0].split('/').filter(Boolean)
  const num = (s: string | undefined) => (s && /^\d+$/.test(s) ? Number(s) : null)
  const bakery = parts[0] === 'b' ? num(parts[1]) : null
  if (bakery === null) return { page: 'home', bakery: null }
  const guild = parts[2] === 'g' ? num(parts[3]) : null
  if (guild === null) return { page: 'home', bakery }
  if (parts[4] !== 'agents') return { page: 'guild', bakery, guild }
  const id = num(parts[5])
  if (id !== null) return { page: 'agent', bakery, guild, id }
  const tab = agentsTabs.find((t) => t === parts[5]) ?? 'all'
  return { page: 'agents', bakery, guild, tab }
}

class Router {
  route = $state<Route>(parse(location.hash))

  constructor() {
    window.addEventListener('hashchange', () => (this.route = parse(location.hash)))
  }
}

export const router = new Router()

/** The hash for a path such as `/b/0/g/1/agents`. */
export const href = (path: string) => `#${path}`

/** Goes to path; `replace` leaves no history entry, for redirects. */
export function go(path: string, replace = false) {
  if (location.hash === href(path)) return
  if (replace) {
    history.replaceState(history.state, '', href(path))
    router.route = parse(location.hash)
  } else {
    location.hash = href(path)
  }
}

/** The path of a Guild's Agents page or one Agent. */
export const agentsPath = (bakery: number, guild: number, tab: AgentsTab | number = 'all') =>
  `/b/${bakery}/g/${guild}/agents${tab === 'all' ? '' : `/${tab}`}`
