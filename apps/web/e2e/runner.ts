// The owner's Desktop for the e2e files that need one (agents.ts, costs.ts,
// work.ts): `login` of apps/desktop connected to the dev Bakery, its approve
// link approved through the API with the signed-in page.
import { spawn, type ChildProcess } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { Page } from 'playwright-core'

const WEB = (process.env.BAKERY_WEB ?? 'http://127.0.0.1:4930').replace(/\/$/, '')
const DESKTOP_DIR = new URL('../../desktop/', import.meta.url).pathname

/** Signs a Desktop in under home with env and answers its id and Desktop key. */
async function signIn(page: Page, home: string, env: NodeJS.ProcessEnv): Promise<{ id: number; key: string }> {
  const login = spawn('go', ['run', '.', 'login', '--server', WEB, '--no-browser'], { cwd: DESKTOP_DIR, env, stdio: ['ignore', 'pipe', 'inherit'] })
  let out = ''
  const link = await new Promise<URL>((resolve, reject) => {
    login.stdout!.on('data', (b: Buffer) => {
      out += b.toString()
      const m = out.match(/https?:\/\/\S+desktop-sign-in\S+/)
      if (m) resolve(new URL(m[0]))
    })
    login.on('exit', (code) => reject(new Error(`login exited ${code}: ${out}`)))
  })
  const [, id] = link.hash.match(/desktop-sign-in\/(\d+)/)!
  const token = new URLSearchParams(link.hash.split('?')[1]).get('token')
  const approved = await page.request.post(`${WEB}/api/desktop-sign-ins/${id}/approve`, { data: { token } })
  if (!approved.ok()) throw new Error(`approve the desktop: ${approved.status()}`)
  const { desktop_id } = (await approved.json()) as { desktop_id: number }
  await new Promise((resolve) => login.on('exit', resolve))
  const { bakeries } = JSON.parse(readFileSync(join(home, 'bakeries.json'), 'utf8')) as { bakeries: { key: string }[] }
  return { id: desktop_id, key: bakeries[0].key }
}

/**
 * A headless Desktop runner: `runner` with the claude stand-in (extra adds
 * to or overrides its environment), in its own process group. key is its
 * Desktop key; stop() ends it and signs its Desktop out again.
 */
export async function desktopRunner(page: Page, extra: Record<string, string> = {}): Promise<{ key: string; stop: () => Promise<void> }> {
  const home = mkdtempSync(join(tmpdir(), 'bakery-e2e-runner-'))
  const env = {
    ...process.env,
    BAKERY_DESKTOP_HOME: home,
    BAKERY_CLAUDE: process.env.BAKERY_CLAUDE ?? join(DESKTOP_DIR, 'bin/claude-standin'),
    ...extra,
  }
  const desktop = await signIn(page, home, env)
  const runner: ChildProcess = spawn('go', ['run', '.', 'runner'], { cwd: DESKTOP_DIR, env, detached: true, stdio: ['ignore', 'inherit', 'inherit'] })
  return {
    key: desktop.key,
    stop: async () => {
      try {
        process.kill(-runner.pid!, 'SIGTERM')
      } catch {}
      await page.request.delete(`${WEB}/api/desktops/${desktop.id}`)
      rmSync(home, { recursive: true, force: true })
    },
  }
}
