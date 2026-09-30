// Hash router: #/projects, #/projects/{id}, #/applications/{id}, #/databases/{id}, #/services/{id}, #/servers, #/servers/{id}, #/settings, #/members, #/notifications, #/api-tokens, #/invite/{token}, #/login.
export type Route =
  | { name: 'projects' }
  | { name: 'project'; id: number }
  | { name: 'application'; id: number }
  | { name: 'database'; id: number }
  | { name: 'service'; id: number }
  | { name: 'servers' }
  | { name: 'server'; id: number }
  | { name: 'settings' }
  | { name: 'members' }
  | { name: 'notifications' }
  | { name: 'api-tokens' }
  | { name: 'invite'; token: string }
  | { name: 'login' }
  | { name: 'notfound' }

function parse(hash: string): Route {
  const parts = hash.replace(/^#\/?/, '').split('/').filter(Boolean).map(decodeURIComponent)
  if (parts.length === 0) return { name: 'projects' }
  if (parts[0] === 'login' && parts.length === 1) return { name: 'login' }
  if (parts[0] === 'settings' && parts.length === 1) return { name: 'settings' }
  if (parts[0] === 'members' && parts.length === 1) return { name: 'members' }
  if (parts[0] === 'notifications' && parts.length === 1) return { name: 'notifications' }
  if (parts[0] === 'api-tokens' && parts.length === 1) return { name: 'api-tokens' }
  if (parts[0] === 'invite' && parts.length === 2) return { name: 'invite', token: parts[1] }
  if (parts[0] === 'servers') {
    if (parts.length === 1) return { name: 'servers' }
    const id = Number(parts[1])
    if (parts.length === 2 && Number.isInteger(id)) return { name: 'server', id }
  }
  if (parts[0] === 'projects') {
    if (parts.length === 1) return { name: 'projects' }
    const id = Number(parts[1])
    if (parts.length === 2 && Number.isInteger(id)) return { name: 'project', id }
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
