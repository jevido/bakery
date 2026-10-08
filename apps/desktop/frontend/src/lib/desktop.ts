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
