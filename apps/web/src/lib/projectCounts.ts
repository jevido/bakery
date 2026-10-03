import { api } from './api'
import type { Database, Project, Service } from './types'

/** What a Project card or row shows beside its name. */
export type ProjectCounts = { environments: number; firstEnvironment?: number; resources: number }

/**
 * A Project's Environments and Resources. Applications come with the Project;
 * Databases and Services are their own contexts with their own endpoints.
 * Null when any of them cannot be read.
 */
export async function projectCounts(id: number): Promise<ProjectCounts | null> {
  const [p, d, s] = await Promise.all([
    api<{ project: Project }>('GET', `/projects/${id}`),
    api<{ databases: Database[] }>('GET', `/projects/${id}/databases`),
    api<{ services: Service[] }>('GET', `/projects/${id}/services`),
  ]).catch(() => [])
  if (!p || !d || !s) return null
  const environments = p.project.environments ?? []
  const applications = environments.reduce((n, e) => n + e.applications.length, 0)
  return {
    environments: environments.length,
    firstEnvironment: environments[0]?.id,
    resources: applications + d.databases.length + s.services.length,
  }
}
