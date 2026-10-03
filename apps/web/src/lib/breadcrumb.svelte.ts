import { api } from './api'
import type { Project } from './types'

/** One step of the top bar's breadcrumb; the last one is the page itself and has no link. */
export type Crumb = { label: string; href?: string }

class Breadcrumb {
  crumbs = $state.raw<Crumb[]>([])

  set(...crumbs: Crumb[]) {
    this.crumbs = crumbs
  }

  clear() {
    this.crumbs = []
  }

  /**
   * Project › Resource, for an Application, Database or Service page. The
   * Resource only knows its Project's id, so the name follows when it loads.
   */
  resource(projectId: number, name: string) {
    const project: Crumb = { label: 'Project', href: `#/project/${projectId}` }
    this.set(project, { label: name })
    api<{ project: Project }>('GET', `/projects/${projectId}`)
      .then(({ project: p }) => {
        // The page may have moved on while the name was on its way.
        if (this.crumbs[0] === project) this.set({ ...project, label: p.name }, { label: name })
      })
      .catch(() => {})
  }
}

export const breadcrumb = new Breadcrumb()
