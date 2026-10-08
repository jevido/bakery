// The Desktop app's sign-in and the Desktops page in headless Chromium, one
// section each, every flow once in the dark theme at 1440×900 (the light
// theme and phone width wait for the guilds goal's final sweep). The
// Desktop app's side (minting the secret and the Desktop key, polling) is
// played here with ctx.request:
//   approve   a sign-in for "test-laptop" opened signed out shows the login
//             page; signing in there comes back to "Approve The Bakery
//             desktop app" with the client and "Signed in as"; Approve shows
//             "Desktop app approved", the poll answers approved, and the
//             Desktop key reads /api/me.
//   cancel    Cancel shows the cancelled state and the poll answers
//             cancelled; a wrong token and an unknown id show not-found.
//   expired   the expired state, with the sign-in's answer replaced by an
//             expired one (the API has no way to age a sign-in short of
//             waiting 10 minutes).
//   desktops  Keys & Tokens' Desktops tab lists an approved Desktop with
//             when it was last seen; Sign out after confirming removes it
//             and its key then answers 401.
//
//   bun e2e/desktop.ts [section ...]   (task web:desktop; needs task dev)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { createHash, randomBytes } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { chromium, type APIRequestContext, type Page } from 'playwright-core'

const WEB = (process.env.BAKERY_WEB ?? 'http://127.0.0.1:4930').replace(/\/$/, '')
const CHROMIUM = process.env.CHROMIUM ?? '/usr/bin/chromium'

function owner(): { email: string; password: string } {
  if (process.env.BAKERY_OWNER_EMAIL && process.env.BAKERY_OWNER_PASSWORD)
    return { email: process.env.BAKERY_OWNER_EMAIL, password: process.env.BAKERY_OWNER_PASSWORD }
  const file = new URL('../../../infra/dev/state/owner.env', import.meta.url)
  const env = Object.fromEntries(
    readFileSync(file, 'utf8')
      .trim()
      .split('\n')
      .map((l) => [l.slice(0, l.indexOf('=')), l.slice(l.indexOf('=') + 1)]),
  )
  return { email: env.OWNER_EMAIL, password: env.OWNER_PASSWORD }
}

let failed = 0
function expect(what: string, ok: boolean, got?: unknown) {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : ` (got ${JSON.stringify(got)})`}`)
  if (!ok) failed++
}

const browser = await chromium.launch({ executablePath: CHROMIUM })
const who = owner()

async function page(signIn: boolean): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  if (signIn) {
    const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
    if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  }
  return ctx.newPage()
}

type SignIn = { id: number; token: string; key: string; path: string }

/** Starts a Desktop sign-in as the Desktop app does: it mints the secret and the Desktop key. */
async function started(request: APIRequestContext, name = 'test-laptop'): Promise<SignIn> {
  const token = 'bky_signin_' + randomBytes(24).toString('hex')
  const key = 'bky_desk_' + randomBytes(24).toString('hex')
  const r = await request.post(`${WEB}/api/desktop-sign-ins`, {
    data: { client_name: name, token, desktop_key_hash: createHash('sha256').update(key).digest('hex') },
  })
  if (!r.ok()) throw new Error(`start: ${r.status()} ${await r.text()}`)
  const { id, approval_path } = (await r.json()) as { id: number; approval_path: string }
  return { id, token, key, path: approval_path }
}

/** The status the Desktop app's poll sees. */
async function polled(request: APIRequestContext, s: SignIn): Promise<string> {
  const r = await request.get(`${WEB}/api/desktop-sign-ins/${s.id}?token=${s.token}`)
  return ((await r.json()) as { status: string }).status
}

/** Approves a sign-in through the API, from a signed-in page. */
async function approved(p: Page, name: string): Promise<SignIn> {
  const s = await started(p.request, name)
  const r = await p.request.post(`${WEB}/api/desktop-sign-ins/${s.id}/approve`, { data: { token: s.token } })
  if (!r.ok()) throw new Error(`approve: ${r.status()} ${await r.text()}`)
  return s
}

const sections: Record<string, () => Promise<void>> = {
  async approve() {
    const p = await page(false)
    const s = await started(p.request)
    await p.goto(WEB + s.path)
    await p.getByRole('button', { name: 'Sign in', exact: true }).waitFor()
    expect('signed out, the approve link shows the login page', true)
    await p.locator('input[name=email]').fill(who.email)
    await p.locator('input[name=password]').fill(who.password)
    await p.getByRole('button', { name: 'Sign in', exact: true }).click()
    await p.getByText('Approve The Bakery desktop app').waitFor()
    expect('signing in comes back to the approve page', p.url().includes(`/desktop-sign-in/${s.id}`), p.url())
    expect('it names the client', (await p.getByTestId('desktop-sign-in-client').textContent()) === 'test-laptop')
    expect('it says who is signed in', (await p.getByTestId('desktop-sign-in-member').textContent()) === who.email)
    expect('the poll answers pending', (await polled(p.request, s)) === 'pending')
    await p.getByRole('button', { name: 'Approve', exact: true }).click()
    await p.getByText('Desktop app approved. You can go back to the app.').waitFor()
    expect('Approve shows the approved state', true)
    expect('the poll answers approved', (await polled(p.request, s)) === 'approved')
    const me = await p.request.get(`${WEB}/api/me`, { headers: { Authorization: `Bearer ${s.key}` } })
    expect('the Desktop key reads /api/me', me.status() === 200, me.status())
    await p.request.post(`${WEB}/api/desktops/current/sign-out`, { headers: { Authorization: `Bearer ${s.key}` } })
    await p.context().close()
  },

  async cancel() {
    const p = await page(true)
    const s = await started(p.request)
    await p.goto(WEB + s.path)
    await p.getByRole('button', { name: 'Cancel', exact: true }).click()
    await p.getByTestId('desktop-sign-in-cancelled').waitFor()
    expect('Cancel shows the cancelled state', (await p.getByText('Start again from the desktop app.').count()) === 1)
    expect('the poll answers cancelled', (await polled(p.request, s)) === 'cancelled')

    await p.goto(`${WEB}/#/desktop-sign-in/${s.id}?token=bky_signin_wrong`)
    await p.getByTestId('desktop-sign-in-missing').waitFor()
    expect('a wrong token shows not-found', true)
    await p.goto(`${WEB}/#/desktop-sign-in/999999?token=${s.token}`)
    await p.reload()
    await p.getByTestId('desktop-sign-in-missing').waitFor()
    expect('an unknown id shows not-found', true)
    await p.context().close()
  },

  async expired() {
    const p = await page(true)
    const s = await started(p.request)
    await p.route(`**/api/desktop-sign-ins/${s.id}?*`, async (route) => {
      const res = await route.fetch()
      await route.fulfill({ response: res, json: { ...(await res.json()), status: 'expired', can_approve: false } })
    })
    await p.goto(WEB + s.path)
    await p.getByTestId('desktop-sign-in-expired').waitFor()
    expect('an expired sign-in shows the expired state', (await p.getByText('This sign-in has expired. Start again from the desktop app.').count()) === 1)
    await p.request.post(`${WEB}/api/desktop-sign-ins/${s.id}/cancel`, { data: { token: s.token } })
    await p.context().close()
  },

  async desktops() {
    const p = await page(true)
    const name = `e2e-laptop-${Date.now()}`
    const s = await approved(p, name)
    // A use of the key sets its last seen.
    await p.request.get(`${WEB}/api/me`, { headers: { Authorization: `Bearer ${s.key}` } })
    await p.goto(`${WEB}/#/security/api-tokens`)
    await p.getByRole('tab', { name: 'Desktops' }).click()
    await p.waitForURL(/#\/security\/desktops$/)
    const row = p.getByTestId('desktop').filter({ hasText: name })
    await row.waitFor()
    expect('the Desktops tab lists the approved Desktop', true)
    expect('it says when it was last seen', ((await row.getByTestId('desktop-last-seen').textContent()) ?? '').startsWith('Last seen'))
    await row.getByTestId('sign-out-desktop').click()
    await p.getByRole('dialog').getByRole('button', { name: 'Sign out', exact: true }).click()
    await row.waitFor({ state: 'detached' })
    expect('Sign out removes it from the list', true)
    const me = await p.request.get(`${WEB}/api/me`, { headers: { Authorization: `Bearer ${s.key}` } })
    expect('its key then answers 401', me.status() === 401, me.status())
    await p.context().close()
  },
}

const asked = process.argv.slice(2)
const unknown = asked.filter((s) => !(s in sections))
if (unknown.length) {
  console.log(`unknown section ${unknown.join(', ')}; the sections are ${Object.keys(sections).join(', ')}`)
  process.exit(2)
}
for (const name of asked.length ? asked : Object.keys(sections)) {
  console.log(`# ${name}`)
  await sections[name]()
}

await browser.close()
if (failed) {
  console.log(`${failed} failed`)
  process.exit(1)
}
console.log('the desktop sign-in flows work')
