// Stands in for Discord, Slack, Telegram's Bot API, Pushover, ntfy and webhook
// receivers in `task notifications:test`: every request is appended to the
// file named by the first argument as one JSON line, and answered the way
// the real service answers a good request. Listens on 127.0.0.1:4988.
import { appendFileSync } from 'node:fs'

const log = process.argv[2]
if (!log) throw new Error('usage: bun receiver.ts LOGFILE')

Bun.serve({
  hostname: '127.0.0.1',
  port: 4988,
  async fetch(req) {
    const url = new URL(req.url)
    appendFileSync(
      log,
      JSON.stringify({
        method: req.method,
        path: url.pathname,
        headers: Object.fromEntries(req.headers),
        body: await req.text(),
      }) + '\n',
    )
    if (url.pathname.includes('/sendMessage')) return Response.json({ ok: true, result: {} })
    if (url.pathname.endsWith('/1/messages.json')) return Response.json({ status: 1, request: crypto.randomUUID() })
    return new Response(null, { status: 204 })
  },
})
