// Hash router: #/projects, #/projects/{id}, #/applications/{id}, #/settings, #/login.
export type Route =
  | { name: 'projects' }
  | { name: 'project'; id: number }
  | { name: 'application'; id: number }
  | { name: 'settings' }
  | { name: 'login' }
  | { name: 'notfound' }

function parse(hash: string): Route {
  const parts = hash.replace(/^#\/?/, '').split('/').filter(Boolean).map(decodeURIComponent)
  if (parts.length === 0) return { name: 'projects' }
  if (parts[0] === 'login' && parts.length === 1) return { name: 'login' }
  if (parts[0] === 'settings' && parts.length === 1) return { name: 'settings' }
  if (parts[0] === 'projects') {
    if (parts.length === 1) return { name: 'projects' }
    const id = Number(parts[1])
    if (parts.length === 2 && Number.isInteger(id)) return { name: 'project', id }
  }
  if (parts[0] === 'applications' && parts.length === 2) {
    const id = Number(parts[1])
    if (Number.isInteger(id)) return { name: 'application', id }
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
