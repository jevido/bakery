import { api } from './api'
import type { Database, Project, Service } from './types'

/** What a Project card or row shows beside its name. */
export type ProjectCounts = { environments: number; firstEnvironment?: number; resources: number }

/** A Project with its Databases and Services, which are their own contexts with their own endpoints. */
export type ProjectResources = { project: Project; databases: Database[]; services: Service[] }

export async function projectResources(id: number): Promise<ProjectResources> {
  const [p, d, s] = await Promise.all([
    api<{ project: Project }>('GET', `/projects/${id}`),
    api<{ databases: Database[] }>('GET', `/projects/${id}/databases`),
    api<{ services: Service[] }>('GET', `/projects/${id}/services`),
  ])
  return { project: p.project, databases: d.databases, services: s.services }
}

/** The Applications, Databases and Services in one Environment. */
export function environmentResourceCount(r: ProjectResources, environmentId: number): number {
  const applications = r.project.environments?.find((e) => e.id === environmentId)?.applications.length ?? 0
  return (
    applications +
    r.databases.filter((d) => d.environment_id === environmentId).length +
    r.services.filter((s) => s.environment_id === environmentId).length
  )
}

/**
 * A Project's Environments and Resources. Applications come with the Project.
 * Null when any of them cannot be read.
 */
export async function projectCounts(id: number): Promise<ProjectCounts | null> {
  const r = await projectResources(id).catch(() => null)
  if (!r) return null
  const environments = r.project.environments ?? []
  const applications = environments.reduce((n, e) => n + e.applications.length, 0)
  return {
    environments: environments.length,
    firstEnvironment: environments[0]?.id,
    resources: applications + r.databases.length + r.services.length,
  }
}
