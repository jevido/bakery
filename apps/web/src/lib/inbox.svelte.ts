/**
 * The sidebar's counts, as Paperclip's useInboxBadge keeps them
 * (ui/src/hooks/useInboxBadge.ts; MIT, see NOTICE): `inbox` is the
 * asking Member's Unread Issues in Mine. Read again on every route change,
 * every 60 s while the tab is visible, and right after a Read mark or an
 * Inbox archive changes.
 */
import { api } from './api'

export const badges = $state({ inbox: 0 })

export function refreshBadges(): Promise<void> {
  return api<{ inbox: number }>('GET', '/sidebar-badges')
    .then((r) => {
      badges.inbox = r.inbox
    })
    .catch(() => {})
}

const POLL_MS = 60_000

/** Polls while the tab is visible and reads again when it comes back; returns the stop. */
export function pollBadges(): () => void {
  let timer: ReturnType<typeof setInterval> | undefined
  const start = () => {
    clearInterval(timer)
    timer = setInterval(refreshBadges, POLL_MS)
  }
  const onVisibility = () => {
    if (document.visibilityState === 'hidden') clearInterval(timer)
    else {
      refreshBadges()
      start()
    }
  }
  if (document.visibilityState !== 'hidden') start()
  document.addEventListener('visibilitychange', onVisibility)
  return () => {
    clearInterval(timer)
    document.removeEventListener('visibilitychange', onVisibility)
  }
}
