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

import type { RunEvent } from '@bakery/ui/runTranscript'
import type { RunStatus } from '@bakery/ui/runStatus'

export type { RunEvent } from '@bakery/ui/runTranscript'
export type { RunStatus } from '@bakery/ui/runStatus'

/** A Run's usage as the claude CLI reported it; the cost is an equivalent only. */
export type RunUsage = {
  input_tokens: number
  cached_input_tokens: number
  output_tokens: number
  turns: number
  cost_equivalent_usd: number
  duration_ms: number
}

/** A Run as the Bakery shows it (contexts/agents/http/runs.go), for an Agent's Runs on its page. */
export type Run = {
  id: number
  agent: { id: number; name: string; icon: string }
  issue: { id: number; identifier: string; title: string } | null
  invocation_source: string
  status: RunStatus
  requested_by: { id: number; name: string } | null
  desktop: { id: number; name: string } | null
  retry_of_run_id: number | null
  usage: RunUsage
  exit_code: number | null
  error: string
  created_at: string
  started_at: string | null
  finished_at: string | null
  can_cancel: boolean
}

/** A Run this desktop executes right now, from the Runner, with its events so far. */
export type LocalRun = {
  address: string
  guild: { id: number; name: string }
  agent: { id: number; name: string; icon: string }
  run_id: number
  issue: { id: number; identifier: string; title: string } | null
  status: RunStatus
  started_at: string
  events: RunEvent[] | null
}

/** The `runs` event: a Run on this desktop and where it stands. */
export type RunsEvent = { address: string; guild_id: number; agent_id: number; run_id: number; status: RunStatus; events: number }

/** The `run-events` event: one message off a Run's stream this desktop did
 * not claim, forwarded by FollowRun. */
export type RunEventsUpdate = { address: string; guild_id: number; run_id: number; kind: 'event' | 'end'; event?: RunEvent; status?: RunStatus }

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
  LocalRuns(): Promise<LocalRun[] | null>
  Runs(address: string, guildID: number, id: number): Promise<Run[] | null>
  RunEvents(address: string, guildID: number, id: number, after: number): Promise<RunEvent[] | null>
  FollowRun(address: string, guildID: number, id: number): Promise<void>
  UnfollowRun(address: string, guildID: number, id: number): Promise<void>
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

/** The Runs this desktop executes right now, across every connected Bakery. */
export const localRuns = () => call('LocalRuns').then((r) => r ?? [])
/** An Agent's last Runs, newest first. */
export const runs = (address: string, guildID: number, id: number) => call('Runs', address, guildID, id).then((r) => r ?? [])
/** A Run's stored events after seq after: a final Run's Transcript, read once. */
export const runEvents = (address: string, guildID: number, id: number, after = 0) => call('RunEvents', address, guildID, id, after).then((e) => e ?? [])
/** Follows a Run this desktop did not claim; its events arrive as `run-events`. */
export const followRun = (address: string, guildID: number, id: number) => call('FollowRun', address, guildID, id)
/** Stops a FollowRun started earlier. */
export const unfollowRun = (address: string, guildID: number, id: number) => call('UnfollowRun', address, guildID, id)

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
