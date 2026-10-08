// The identity context's Desktops: the Desktop app's sign-in, approved in
// the browser, and the Desktops signed in as the person.
import { api } from './api'

/** Where a Desktop sign-in is; derived by the API from its times. */
export type DesktopSignInStatus = 'pending' | 'approved' | 'cancelled' | 'expired'

export type DesktopSignIn = {
  id: number
  client_name: string
  status: DesktopSignInStatus
  expires_at: string
  approved_at: string | null
  approved_by: { id: number; name: string } | null
  requires_sign_in: boolean
  can_approve: boolean
}

/** One signed-in copy of the Desktop app, as The Bakery knows it. */
export type Desktop = {
  id: number
  name: string
  created_at: string
  last_seen_at: string | null
  /** True for the Desktop making the request; never from the dashboard. */
  current: boolean
}

export const getDesktopSignIn = (id: number, token: string) =>
  api<DesktopSignIn>('GET', `/desktop-sign-ins/${id}?token=${encodeURIComponent(token)}`)

export const approveDesktopSignIn = (id: number, token: string) =>
  api<{ status: DesktopSignInStatus; desktop_id: number }>('POST', `/desktop-sign-ins/${id}/approve`, { token })

export const cancelDesktopSignIn = (id: number, token: string) =>
  api<{ status: DesktopSignInStatus }>('POST', `/desktop-sign-ins/${id}/cancel`, { token })

export const listDesktops = () => api<Desktop[]>('GET', '/desktops')

export const signOutDesktop = (id: number) => api('DELETE', `/desktops/${id}`)
