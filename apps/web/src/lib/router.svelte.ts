import { api } from './api'
import type { Application, Project } from './types'

// Hash router, with Coolify's paths for Projects and Environments: #/ (the Dashboard), #/projects, #/project/{id} (its Environments), #/project/{id}/edit, #/project/{id}/environment/{envId} (its Resources), #/project/{id}/environment/{envId}/new[?type=…&server=…], #/project/{id}/environment/{envId}/edit, #/project/{id}/environment/{envId}/application/{appId}[/{page}] (its sub-pages are applicationPages; …/deployment/{deploymentId} opens one Deployment), #/applications/{id} (old links, moved to the Application's path once it loads), #/databases/{id}, #/services/{id}, #/servers, #/servers/{id}, #/storages, #/settings, #/members, #/notifications, #/api-tokens, #/profile (#/account opens it too), #/invite/{token}, #/login, and #/dev/components in dev builds.
export type Route =
  | { name: 'dashboard' }
  | { name: 'projects' }
  | { name: 'project'; id: number }
  | { name: 'project-edit'; id: number }
  | { name: 'environment'; projectId: number; id: number }
  // type and server are Coolify's query: the card picked and the Server it
  // goes on (…/new?type=public&server=1).
  | { name: 'environment-new'; projectId: number; id: number; type: string; server: number | null }
  | { name: 'environment-edit'; projectId: number; id: number }
  // #/projects/{id}?new from before Coolify's paths: the New Resource page of
  // the Project's first Environment, found once the Project has loaded.
  | { name: 'project-first-environment-new'; id: number }
  // page is the sub-page slug of Coolify's routes/web.php, '' for General.
  | { name: 'application'; projectId: number; environmentId: number; id: number; page: ApplicationPage; deploymentId: number | null }
  | { name: 'application-legacy'; id: number }
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

/** The Application page's sub-pages, by their slug in Coolify's URLs. */
export const applicationPages = [
  '',
  'domains',
  'advanced',
  'environment-variables',
  'persistent-storage',
  'deployment',
  'logs',
  'source',
  'servers',
  'webhooks',
  'preview-deployments',
  'healthcheck',
  'rollback',
  'resource-limits',
  'danger',
] as const
export type ApplicationPage = (typeof applicationPages)[number]

function isApplicationPage(s: string): s is ApplicationPage {
  return (applicationPages as readonly string[]).includes(s)
}

/** The path of an Application's page, or one of its sub-pages. */
export function applicationPath(a: { project_id: number; environment_id: number; id: number }, page: ApplicationPage = ''): string {
  const base = `/project/${a.project_id}/environment/${a.environment_id}/application/${a.id}`
  return page ? `${base}/${page}` : base
}

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
        if (parts.length === 5 && parts[4] === 'new') {
          const server = Number(flags.get('server') ?? '')
          return {
            name: 'environment-new',
            projectId: id,
            id: envId,
            type: flags.get('type') ?? '',
            server: flags.has('server') && Number.isInteger(server) && server > 0 ? server : null,
          }
        }
        if (parts.length === 5 && parts[4] === 'edit') return { name: 'environment-edit', projectId: id, id: envId }
        const appId = Number(parts[5])
        if (parts[4] === 'application' && Number.isInteger(appId)) {
          const application = { name: 'application', projectId: id, environmentId: envId, id: appId } as const
          const page = parts[6] ?? ''
          if (parts.length <= 7 && isApplicationPage(page)) return { ...application, page, deploymentId: null }
          const deploymentId = Number(parts[7])
          if (parts.length === 8 && page === 'deployment' && Number.isInteger(deploymentId)) return { ...application, page, deploymentId }
        }
      }
    }
  }
  if (parts[0] === 'applications' && parts.length === 2) {
    const id = Number(parts[1])
    if (Number.isInteger(id)) return { name: 'application-legacy', id }
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
    if (route.name === 'application-legacy') {
      api<{ application: Application }>('GET', `/applications/${route.id}`)
        .then(({ application }) => {
          if (this.route === route) this.route = redirect(applicationPath(application))
        })
        .catch(() => {
          if (this.route === route) this.route = { name: 'notfound' }
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
