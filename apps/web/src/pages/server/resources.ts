// The Resources on a Server, shared by its Resources page (Coolify's
// managed tab) and its Danger page (Coolify's definedResources()).
import { api } from '../../lib/api'
import type { ResourceType } from '../../lib/resources'
import { applicationPath, databasePath, href, servicePath } from '../../lib/router.svelte'
import type { Database, Deployment, Project, Server, Service } from '../../lib/types'

export type Row = {
  key: string
  name: string
  project: string
  environment: string
  type: ResourceType
  status: string
  href: string
}

// Composed from the dashboard API's Projects (the list leaves their
// Environments out, a single Project has them); Databases and Services run
// on the Local server only.
export async function serverResources(target: Server): Promise<Row[]> {
  const { projects } = await api<{ projects: Project[] }>('GET', '/projects')
  const local = target.kind === 'local'
  const perProject = await Promise.all(
    projects.map(async ({ id }) => {
      const { project: p } = await api<{ project: Project }>('GET', `/projects/${id}`)
      const environments = p.environments ?? []
      const envName = (id: number) => environments.find((e) => e.id === id)?.name ?? ''
      const applications = environments.flatMap((e) => e.applications).filter((a) => a.server_id === target.id)
      const [databases, services, statuses] = await Promise.all([
        local ? api<{ databases: Database[] }>('GET', `/projects/${p.id}/databases`).then((r) => r.databases) : [],
        local ? api<{ services: Service[] }>('GET', `/projects/${p.id}/services`).then((r) => r.services) : [],
        // An Application's status is its latest own Deployment's state, as
        // the Environment page shows it.
        Promise.all(
          applications.map((a) =>
            api<{ deployments: Deployment[] }>('GET', `/applications/${a.id}/deployments`)
              .then((r) => r.deployments.find((x) => x.preview === 0)?.status ?? '')
              .catch(() => ''),
          ),
        ),
      ])
      return [
        ...applications.map(
          (a, i): Row => ({
            key: `application-${a.id}`,
            name: a.name,
            project: p.name,
            environment: envName(a.environment_id),
            type: 'application',
            status: statuses[i],
            href: href(applicationPath(a)),
          }),
        ),
        ...databases.map(
          (d): Row => ({
            key: `database-${d.id}`,
            name: d.name,
            project: p.name,
            environment: envName(d.environment_id),
            type: 'database',
            status: d.status,
            href: href(databasePath(d)),
          }),
        ),
        ...services.map(
          (s): Row => ({
            key: `service-${s.id}`,
            name: s.name,
            project: p.name,
            environment: envName(s.environment_id),
            type: 'service',
            status: s.status,
            href: href(servicePath(s)),
          }),
        ),
      ]
    }),
  )
  return perProject.flat().sort((a, b) => a.name.localeCompare(b.name))
}
