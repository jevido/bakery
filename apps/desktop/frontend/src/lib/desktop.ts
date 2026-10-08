/**
 * The bridge to the Desktop app's Go methods: one typed function per method
 * of the `Desktop` service (desktop.go). Inside the Wails window it calls the
 * generated bindings; in a plain browser (`serve`, for the headless checks)
 * it POSTs /rpc/<Method> with a JSON array of the arguments and reads
 * {result} or {error}. Live updates arrive as Wails events in the window and
 * as the SSE stream GET /rpc/events in the browser.
 */

type Webview = { webkit?: { messageHandlers?: { external?: unknown } }; chrome?: { webview?: unknown } }

/** Inside the Wails window: the webview's message channel that Wails' runtime posts to. */
export const inWindow = (() => {
  const w = window as unknown as Webview
  return Boolean(w.webkit?.messageHandlers?.external || w.chrome?.webview)
})()

// Loaded only in the window, so the browser never starts Wails' runtime.
const bindings = inWindow ? import('../../bindings/github.com/jevido/bakery/apps/desktop/index.js') : null

async function rpc<T>(method: string, ...args: unknown[]): Promise<T> {
  const res = await fetch(`/rpc/${method}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(args),
  })
  const body = (await res.json()) as { result?: T; error?: string }
  if (!res.ok || body.error !== undefined) throw new Error(body.error ?? `${method}: ${res.status}`)
  return body.result as T
}

/** The Desktop app's version. */
export async function version(): Promise<string> {
  if (bindings) return (await bindings).Desktop.Version()
  return rpc<string>('Version')
}

/** A connected Bakery; its key never leaves the Go side. */
export type Bakery = {
  address: string
  member: { id: number; name: string; email: string }
  connected_at: string
  active: boolean
  signed_out: boolean
}

/** A started connect: the link the person approves this desktop at. */
export type ConnectStart = { id: number; address: string; approval_url: string; expires_at: string }

export type ConnectStatus = 'pending' | 'approved' | 'expired' | 'cancelled' | 'failed'

/** Where a connect stands; also the `connect` event's data. */
export type ConnectState = { id: number; address: string; status: ConnectStatus; error?: string }

/** The person a Desktop key acts as. */
export type Member = { id: number; name: string; email: string }

/** A Guild the person is in. */
export type Guild = { id: number; name: string; issue_prefix: string }

import type { AgentStatus } from '@bakery/ui/agentStatus'

/** The Agents list's tabs, as GET /api/agents?status= filters them; all leaves out the terminated ones. */
export type AgentsTab = 'all' | 'active' | 'paused' | 'terminated'

/** An Agent as the Bakery shows it (contexts/agents/http/agents.go), less what only the dashboard uses to manage it. */
export type Agent = {
  id: number
  name: string
  job: string
  job_label: string
  title: string
  icon: string
  capabilities: string
  status: AgentStatus
  reports_to: { id: number; name: string } | null
  hirer: { id: number; name: string } | null
  roles: { id: number; name: string; color: string; position: number }[] | null
  created_at: string
  terminated_at: string | null
}

/** The `guilds` event: the Guilds of the Bakery at address, sent when a refresh changed them. */
export type GuildsEvent = { address: string; guilds: Guild[] | null }

/** The `agents` event: one Guild's Agents on one tab, sent when a refresh changed them. */
export type AgentsEvent = { address: string; guild_id: number; status: AgentsTab; agents: Agent[] | null }

// The bindings' generated models carry the same JSON fields as these types.
type Methods = {
  Bakeries(): Promise<Bakery[]>
  Activate(address: string): Promise<void>
  Connect(address: string): Promise<ConnectStart>
  ConnectStatus(id: number): Promise<ConnectState>
  CancelConnect(id: number): Promise<void>
  Disconnect(address: string): Promise<void>
  Me(address: string): Promise<Member>
  Guilds(address: string): Promise<Guild[] | null>
  Agents(address: string, guildID: number, status: AgentsTab): Promise<Agent[] | null>
  Agent(address: string, guildID: number, id: number): Promise<Agent>
}

async function call<K extends keyof Methods>(method: K, ...args: Parameters<Methods[K]>): Promise<Awaited<ReturnType<Methods[K]>>> {
  if (bindings) {
    const fn = (await bindings).Desktop[method] as unknown as (...a: unknown[]) => Promise<Awaited<ReturnType<Methods[K]>>>
    return fn(...args)
  }
  return rpc<Awaited<ReturnType<Methods[K]>>>(method, ...args)
}

/** The connected Bakeries, the active one marked. */
export const bakeries = () => call('Bakeries')
/** Makes the Bakery at address the one the window shows. */
export const activate = (address: string) => call('Activate', address)
/** Starts connecting to the Bakery at address; the window also opens the approve link in the browser. */
export const connect = (address: string) => call('Connect', address)
/** Where the connect id stands. */
export const connectStatus = (id: number) => call('ConnectStatus', id)
/** Cancels a pending connect. */
export const cancelConnect = (id: number) => call('CancelConnect', id)
/** Signs this desktop out of the Bakery at address and forgets its key. */
export const disconnect = (address: string) => call('Disconnect', address)

/** The person this desktop acts as on the Bakery at address. */
export const me = (address: string) => call('Me', address)
/** The person's Guilds on the Bakery at address; kept current with `guilds` events while shown. */
export const guilds = (address: string) => call('Guilds', address).then((g) => g ?? [])
/** A Guild's Agents on a tab; kept current with `agents` events while shown. */
export const agents = (address: string, guildID: number, status: AgentsTab) => call('Agents', address, guildID, status).then((a) => a ?? [])
/** One Agent of a Guild. */
export const agent = (address: string, guildID: number, id: number) => call('Agent', address, guildID, id)

/** Opens url in the system browser: through Wails in the window, a new tab in the browser. */
export async function openInBrowser(url: string) {
  if (inWindow) {
    const { Browser } = await import('@wailsio/runtime')
    await Browser.OpenURL(url)
    return
  }
  window.open(url, '_blank', 'noopener')
}

/** Calls handler with each `name` event's data until the returned function is called. */
export function onEvent<T>(name: string, handler: (data: T) => void): () => void {
  if (inWindow) {
    let off: (() => void) | undefined
    let stopped = false
    void import('@wailsio/runtime').then(({ Events }) => {
      if (!stopped) off = Events.On(name, (e: { data: T }) => handler(e.data))
    })
    return () => {
      stopped = true
      off?.()
    }
  }
  return stream().subscribe(name, handler as (data: unknown) => void)
}

// One EventSource for every listener in the browser.
let source: { subscribe: (name: string, handler: (data: unknown) => void) => () => void } | null = null
function stream() {
  if (source) return source
  const es = new EventSource('/rpc/events')
  source = {
    subscribe(name, handler) {
      const listener = (e: MessageEvent<string>) => handler(JSON.parse(e.data))
      es.addEventListener(name, listener)
      return () => es.removeEventListener(name, listener)
    },
  }
  return source
}
