// Every call goes to /api on the dashboard's own origin (the Vite proxy in
// dev), so the HttpOnly session cookie rides along without CORS.

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public errors: Record<string, string> = {},
    /** The whole JSON body of the error response. */
    public body: Record<string, unknown> = {},
  ) {
    super(message)
  }
}

/** Called on any 401, so the app can drop back to the login screen. */
let onUnauthorized: () => void = () => {}

export function setUnauthorizedHandler(f: () => void) {
  onUnauthorized = f
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch('/api' + path, {
    method,
    credentials: 'same-origin',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    if (res.status === 401) onUnauthorized()
    throw new ApiError(data.message ?? res.statusText, res.status, data.errors ?? {}, data)
  }
  return data as T
}
