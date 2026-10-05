import type { Server } from '../../lib/types'

/**
 * A Server's status in the words of Coolify's server/index.blade.php: a
 * reachable Server is "Ready", any other needs validating. The Bakery has no
 * proxy or Sentinel per Server and cannot disable one, so Coolify's
 * "Attention required", "Disabled" and "Transferred away" never show. An
 * unreachable Server carries the first failing check's detail as `detail`.
 */
export function serverStatus(s: Server): { label: string; type: 'success' | 'error'; detail: string } {
  if (s.status === 'reachable') return { label: 'Ready', type: 'success', detail: '' }
  const failed = s.status === 'unreachable' ? s.validation.checks.find((c) => !c.ok && c.required) : undefined
  return { label: 'Validation required', type: 'error', detail: failed?.detail ?? '' }
}

/** Coolify's isFunctional(): the Server answers and can run Resources. */
export function isFunctional(s: Server): boolean {
  return s.status === 'reachable'
}
