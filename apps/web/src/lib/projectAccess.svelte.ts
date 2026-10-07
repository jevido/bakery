// The signed-in Member's Permissions in the Project the page is under, as
// `GET /api/projects/{id}` answers them with its Permission overrides
// applied. Pages under a Project ask projectAccess.can instead of
// session.can, so they only offer what the server allows there. The
// server stays the judge; this only hides what it would refuse.
import { api, ApiError } from './api'
import type { Route } from './router.svelte'
import { canIn, session, type Permission } from './session.svelte'

/** The Permissions a Project can override (Overridable in contexts/guilds/domain/permission.go); the rest are guild-wide. */
const overridable: ReadonlySet<Permission> = new Set(['view_resources', 'see_secrets', 'deploy', 'manage_applications'])

/** The Project a route is under, or null. */
export function projectOf(route: Route): number | null {
  switch (route.name) {
    case 'project':
    case 'project-edit':
    case 'project-permissions':
    case 'project-first-environment-new':
      return route.id
    case 'environment':
    case 'environment-new':
    case 'environment-edit':
    case 'application':
    case 'database':
    case 'service':
      return route.projectId
  }
  return null
}

class ProjectAccess {
  /** The Guild and Project followed, as "guild:project"; '' for none. */
  #key = ''
  #project = $state<number | null>(null)
  /** Null until the Project has answered. */
  #permissions = $state.raw<Permission[] | null>(null)
  /** Whether the followed Project answered 404: gone, elsewhere, or not to be viewed. */
  missing = $state(false)

  /** Follows the Project of the page in the Current guild, asking it once per visit. */
  follow(project: number | null, guild: number | null) {
    const key = project === null ? '' : `${guild}:${project}`
    if (key === this.#key) return
    this.#key = key
    this.#project = project
    this.#permissions = null
    this.missing = false
    if (project !== null) void this.#load(key, project)
  }

  /** Asks the followed Project again, after its Permission overrides changed. */
  async refresh() {
    if (this.#project !== null) await this.#load(this.#key, this.#project)
  }

  async #load(key: string, project: number) {
    try {
      const { project: p } = await api<{ project: { permissions?: Permission[] } }>('GET', `/projects/${project}`)
      if (key === this.#key) this.#permissions = p.permissions ?? []
    } catch (e) {
      if (key !== this.#key) return
      if (!(e instanceof ApiError) || e.status !== 404) throw e
      this.#permissions = []
      this.missing = true
    }
  }

  /**
   * Whether the Member may use p where the page is: in its Project for the
   * four Permissions a Project overrides (false until the Project answers),
   * guild-wide for the rest and outside a Project.
   */
  can(p: Permission): boolean {
    if (this.#project === null || !overridable.has(p)) return session.can(p)
    return this.#permissions !== null && canIn(this.#permissions, p)
  }
}

export const projectAccess = new ProjectAccess()
