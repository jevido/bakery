import { api } from './api'
import type { Project } from './types'

// Hash router, with Coolify's paths for Projects and Environments: #/ (the Dashboard), #/projects, #/project/{id} (its Environments), #/project/{id}/edit, #/project/{id}/environment/{envId} (its Resources), #/project/{id}/environment/{envId}/new, #/project/{id}/environment/{envId}/edit, #/applications/{id}, #/databases/{id}, #/services/{id}, #/servers, #/servers/{id}, #/storages, #/settings, #/members, #/notifications, #/api-tokens, #/profile (#/account opens it too), #/invite/{token}, #/login, and #/dev/components in dev builds.
export type Route =
  | { name: 'dashboard' }
  | { name: 'projects' }
  | { name: 'project'; id: number }
  | { name: 'project-edit'; id: number }
  | { name: 'environment'; projectId: number; id: number }
  | { name: 'environment-new'; projectId: number; id: number }
  | { name: 'environment-edit'; projectId: number; id: number }
  // #/projects/{id}?new from before Coolify's paths: the New Resource page of
  // the Project's first Environment, found once the Project has loaded.
  | { name: 'project-first-environment-new'; id: number }
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
    if (parts.length === 2 && Number.isInteger(id)) {
      if (flags.has('new')) return { name: 'project-first-environment-new', id }
      return redirect(`/project/${id}`)
    }
  }
  if (parts[0] === 'project') {
    const id = Number(parts[1])
    if (Number.isInteger(id)) {
      if (parts.length === 2) return { name: 'project', id }
      if (parts.length === 3 && parts[2] === 'edit') return { name: 'project-edit', id }
      const envId = Number(parts[3])
      if (parts[2] === 'environment' && Number.isInteger(envId)) {
        if (parts.length === 4) return { name: 'environment', projectId: id, id: envId }
        if (parts.length === 5 && parts[4] === 'new') return { name: 'environment-new', projectId: id, id: envId }
        if (parts.length === 5 && parts[4] === 'edit') return { name: 'environment-edit', projectId: id, id: envId }
      }
    }
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

// An old link opens its new path in place, so Back does not return to it and
// bounce forward again. replaceState fires no hashchange; the caller parses.
function redirect(path: string): Route {
  history.replaceState(null, '', '#' + path)
  return parse(location.hash)
}

class Router {
  route = $state.raw<Route>(parse(location.hash))

  constructor() {
    window.addEventListener('hashchange', () => this.update())
    this.update()
  }

  private update() {
    const route = parse(location.hash)
    this.route = route
    if (route.name === 'project-first-environment-new') {
      api<{ project: Project }>('GET', `/projects/${route.id}`)
        .then(({ project }) => {
          // The person may have moved on while the Project was on its way.
          if (this.route !== route) return
          const first = project.environments?.[0]
          this.route = redirect(first ? `/project/${route.id}/environment/${first.id}/new` : `/project/${route.id}`)
        })
        .catch(() => {
          if (this.route === route) this.route = redirect(`/project/${route.id}`)
        })
    }
  }
}

export const router = new Router()

export function go(path: string) {
  location.hash = path
}

export function href(path: string): string {
  return '#' + path
}
