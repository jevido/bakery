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

// The bindings' generated models carry the same JSON fields as these types.
type Methods = {
  Bakeries(): Promise<Bakery[]>
  Activate(address: string): Promise<void>
  Connect(address: string): Promise<ConnectStart>
  ConnectStatus(id: number): Promise<ConnectState>
  CancelConnect(id: number): Promise<void>
  Disconnect(address: string): Promise<void>
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
