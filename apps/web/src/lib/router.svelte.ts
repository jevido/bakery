// Hash router: #/ (the Dashboard), #/projects, #/projects/{id} (#/projects/{id}?new opens its New Resource chooser), #/applications/{id}, #/databases/{id}, #/services/{id}, #/servers, #/servers/{id}, #/storages, #/settings, #/members, #/notifications, #/api-tokens, #/profile (#/account opens it too), #/invite/{token}, #/login, and #/dev/components in dev builds.
export type Route =
  | { name: 'dashboard' }
  | { name: 'projects' }
  | { name: 'project'; id: number; new: boolean }
  | { name: 'application'; id: number }
  | { name: 'database'; id: number }
  | { name: 'service'; id: number }
  | { name: 'servers' }
  | { name: 'server'; id: number }
  | { name: 'storages' }
  | { name: 'settings' }
  | { name: 'members' }
  | { name: 'notifications' }
  | { name: 'api-tokens' }
  | { name: 'profile' }
  | { name: 'invite'; token: string }
  | { name: 'login' }
  | { name: 'dev-components' }
  | { name: 'notfound' }

function parse(hash: string): Route {
  const [path, query = ''] = hash.replace(/^#\/?/, '').split('?', 2)
  const flags = new URLSearchParams(query)
  const parts = path.split('/').filter(Boolean).map(decodeURIComponent)
  if (parts.length === 0) return { name: 'dashboard' }
  if (parts[0] === 'login' && parts.length === 1) return { name: 'login' }
  if (import.meta.env.DEV && parts[0] === 'dev' && parts[1] === 'components' && parts.length === 2) return { name: 'dev-components' }
  if (parts[0] === 'storages' && parts.length === 1) return { name: 'storages' }
  if (parts[0] === 'settings' && parts.length === 1) return { name: 'settings' }
  if (parts[0] === 'members' && parts.length === 1) return { name: 'members' }
  if (parts[0] === 'notifications' && parts.length === 1) return { name: 'notifications' }
  if (parts[0] === 'api-tokens' && parts.length === 1) return { name: 'api-tokens' }
  // #/account was the page's name before it took Coolify's; old links still open it.
  if ((parts[0] === 'profile' || parts[0] === 'account') && parts.length === 1) return { name: 'profile' }
  if (parts[0] === 'invite' && parts.length === 2) return { name: 'invite', token: parts[1] }
  if (parts[0] === 'servers') {
    if (parts.length === 1) return { name: 'servers' }
    const id = Number(parts[1])
    if (parts.length === 2 && Number.isInteger(id)) return { name: 'server', id }
  }
  if (parts[0] === 'projects') {
    if (parts.length === 1) return { name: 'projects' }
    const id = Number(parts[1])
    if (parts.length === 2 && Number.isInteger(id)) return { name: 'project', id, new: flags.has('new') }
  }
  if (parts[0] === 'applications' && parts.length === 2) {
    const id = Number(parts[1])
    if (Number.isInteger(id)) return { name: 'application', id }
  }
  if (parts[0] === 'databases' && parts.length === 2) {
    const id = Number(parts[1])
    if (Number.isInteger(id)) return { name: 'database', id }
  }
  if (parts[0] === 'services' && parts.length === 2) {
    const id = Number(parts[1])
    if (Number.isInteger(id)) return { name: 'service', id }
  }
  return { name: 'notfound' }
}

class Router {
  route = $state<Route>(parse(location.hash))

  constructor() {
    window.addEventListener('hashchange', () => {
      this.route = parse(location.hash)
    })
  }
}

export const router = new Router()

export function go(path: string) {
  location.hash = path
}

export function href(path: string): string {
  return '#' + path
}
