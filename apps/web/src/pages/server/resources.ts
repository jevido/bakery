// The Resources on a Server, shared by its Resources page (Coolify's
// managed tab) and its Danger page (Coolify's definedResources()).
import { api } from '../../lib/api'
import type { ResourceType } from '../../lib/resources'
import { applicationPath, databasePath, href, servicePath } from '../../lib/router.svelte'
import type { Server } from '../../lib/types'

export type Row = {
  key: string
  name: string
  project: string
  environment: string
  type: ResourceType
  status: string
  href: string
}

type Resource = {
  type: ResourceType
  id: number
  name: string
  project_id: number
  project: string
  environment_id: number
  environment: string
  status: string
}

const paths = { application: applicationPath, database: databasePath, service: servicePath }

// Databases and Services run on the Local server only; an Application's
// status is its latest own Deployment's state.
export async function serverResources(target: Server): Promise<Row[]> {
  const { resources } = await api<{ resources: Resource[] }>('GET', `/servers/${target.id}/resources`)
  return resources
    .map((r): Row => ({
    key: `${r.type}-${r.id}`,
    name: r.name,
    project: r.project,
    environment: r.environment,
    type: r.type,
    status: r.status,
    href: href(paths[r.type](r)),
  }))
    .sort((a, b) => a.name.localeCompare(b.name))
}
