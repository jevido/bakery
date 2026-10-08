// What an Environment's Resources page shows of each Application, Database
// and Service, with the helpers from the Alpine script in Coolify's
// project/resource/index.blade.php (Apache-2.0, see NOTICE).
import { statusType, type StatusType } from '@bakery/ui/statusColors'

export type ResourceType = 'application' | 'database' | 'service'

/** One row or card on the page. */
export type ResourceItem = {
  key: string
  name: string
  type: ResourceType
  typeLabel: string
  description: string
  /** The address it is served on, '' when it has none. */
  fqdn: string
  /** An Application's latest Deployment's state, a Database's or Service's status. */
  status: string
  server: string
  href: string
}

export const typeLabels: Record<ResourceType, string> = {
  application: 'Application',
  database: 'Database',
  service: 'Service',
}

/** The state before any `:health` suffix, "unknown" when there is none. */
export function statusState(item: Pick<ResourceItem, 'status'>): string {
  return String(item.status || 'unknown').split(':')[0].toLowerCase()
}

export function statusLabel(item: Pick<ResourceItem, 'status'>): string {
  const state = statusState(item)
  return state.charAt(0).toUpperCase() + state.slice(1)
}

export const statusTitle = statusLabel

/**
 * Coolify's tones for container states, with Bakery's own: a finished
 * Deployment is running, one in flight (and a Service deploying) is starting,
 * a cancelled one is neutral.
 */
export function statusTone(item: Pick<ResourceItem, 'status'>): StatusType {
  return statusType(statusState(item))
}

export function firstDomain(fqdn: string): string {
  return String(fqdn).split(',')[0].trim()
}

export function displayDomain(fqdn: string): string {
  if (!fqdn) return ''
  return firstDomain(fqdn).replace(/^https?:\/\//, '')
}
