// The Servers list and New server in headless Chromium: the Local server
// shows with its Ready status, search narrows and clears, the table/grid
// switch persists across a reload, New server validates an empty submit,
// and a Viewer sees no "New server". A Server's frame, in dark and light on
// a desktop and a phone: the header with its name and status pill, the nav
// to every sub-page (the select on a phone), and the switcher to a second
// Server (the Remote server stand-in, when task remote:up runs).
//
//   bun e2e/servers.ts            (task web:servers; needs task dev running)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { readFileSync } from 'node:fs'
import { connect } from 'node:net'
import { chromium, type Page } from 'playwright-core'

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

type Server = { id: number; name: string; kind: 'local' | 'remote' }
const browser = await chromium.launch({ executablePath: CHROMIUM })
const who = owner()

async function signedIn(width: number, height: number, theme: 'dark' | 'light' = 'light'): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  await ctx.addInitScript((t) => localStorage.setItem('theme', t), theme)
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

const probe = await signedIn(1440, 900)
const localServer = ((await (await probe.request.get(`${WEB}/api/servers`)).json()) as { servers: Server[] }).servers.find(
  (s) => s.kind === 'local',
)!

for (const theme of ['light', 'dark'] as const) {
  const page = await signedIn(1440, 900, theme)
  await page.goto(`${WEB}/#/servers`)
  await page.getByTestId('servers-count').waitFor()

  const rows = page.getByTestId('server')
  expect(`${theme}: the list has the Local server`, (await rows.filter({ hasText: localServer.name }).count()) === 1)
  expect(`${theme}: it is Ready`, (await rows.filter({ hasText: localServer.name }).getByTestId('server-status').textContent()) === 'Ready')
  const dark = await page.evaluate(() => document.documentElement.classList.contains('dark'))
  expect(`${theme}: the page is in the ${theme} theme`, dark === (theme === 'dark'), dark)
  await page.close()
}

// The Server frame on the Local server, in each theme and size.
const subPages = ['General', 'Resources', 'Docker Cleanup', 'Metrics']
for (const theme of ['light', 'dark'] as const) {
  for (const [width, height] of [
    [1440, 900],
    [390, 844],
  ] as const) {
    const at = `${theme} ${width}px`
    const page = await signedIn(width, height, theme)
    await page.goto(`${WEB}/#/server/${localServer.id}`)
    await page.getByTestId('server-subtitle').waitFor()
    expect(`${at}: the header shows the name`, (await page.getByTestId('server-subtitle').textContent())?.trim() === localServer.name)
    const pill = page.getByTestId('server-status-summary')
    expect(`${at}: the status pill says Ready`, ((await pill.textContent()) ?? '').includes('Ready'))
    await pill.click()
    expect(`${at}: the pill opens System status`, await page.getByText('System status').isVisible())
    await page.keyboard.press('Escape')
    const crumbs = await page.getByTestId('breadcrumb-bar').textContent()
    expect(`${at}: the breadcrumb is Servers › the name`, !!crumbs?.includes(localServer.name), crumbs)
    expect(`${at}: the top bar holds no server context`, (await page.locator('#server-topbar-context, #resource-action-hud-slot').count()) === 0)

    for (const label of subPages) {
      if (width >= 1280) {
        await page.getByRole('navigation', { name: 'Server configuration sections' }).getByRole('link', { name: label, exact: true }).click()
      } else {
        await page.getByRole('combobox', { name: 'Server configuration sections' }).selectOption({ label })
      }
      await page.waitForTimeout(300)
      const want = label === 'General' ? `#/server/${localServer.id}` : `#/server/${localServer.id}/${label.toLowerCase().replace(' ', '-')}`
      expect(`${at}: the nav opens ${label}`, page.url().endsWith(want), page.url())
    }
    if (width < 1280) {
      const sideways = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
      expect(`${at}: nothing scrolls sideways`, !sideways)
    }
    await page.close()
  }
}

// The switcher, with the Remote server stand-in as a second Server.
const standIn = await new Promise<boolean>((resolve) => {
  const socket = connect(4972, '127.0.0.1')
  socket.once('connect', () => (socket.destroy(), resolve(true)))
  socket.once('error', () => resolve(false))
})
if (!standIn) {
  console.log('skip the switcher: the Remote server stand-in is not running (task remote:up)')
} else {
  const page = await signedIn(1440, 900)
  const name = `e2e-switch-${Date.now()}`
  const added = await page.request.post(`${WEB}/api/servers`, { data: { name, host: '127.0.0.1', port: 4972, user: 'podman' } })
  const other = ((await added.json()) as { server: Server }).server
  try {
    await page.goto(`${WEB}/#/server/${localServer.id}/metrics`)
    await page.getByTestId('server-subtitle').waitFor()
    await page.getByLabel('Switch server').click()
    const options = page.getByTestId('server-switcher-option')
    expect('the switcher lists both Servers', (await options.count()) >= 2, await options.count())
    await page.getByLabel('Filter servers').fill(name)
    await page.waitForTimeout(200)
    expect('the filter narrows to the other Server', (await options.count()) === 1, await options.count())
    await options.first().click()
    await page.waitForTimeout(500)
    expect('the switcher stays on Metrics', page.url().endsWith(`#/server/${other.id}/metrics`), page.url())
    expect('the header shows the other Server', (await page.getByTestId('server-subtitle').textContent())?.trim() === name)
  } finally {
    await page.request.delete(`${WEB}/api/servers/${other.id}`)
    await page.close()
  }
}

const page = await signedIn(1440, 900)
await page.goto(`${WEB}/#/servers`)
await page.getByTestId('servers-count').waitFor()

// Search narrows to a match and the empty state on no match, then clears.
const search = page.getByPlaceholder('Search servers')
await search.fill(localServer.name)
await page.waitForTimeout(200)
expect('search narrows to the match', (await page.getByTestId('server').count()) === 1)
await search.fill('no-such-server-xyz')
await page.waitForTimeout(200)
expect('no match shows "No matching servers"', (await page.getByText('No matching servers').count()) === 1)
await page.getByLabel('Clear search').click()
await page.waitForTimeout(200)
expect('clearing restores the list', (await page.getByTestId('server').count()) >= 1)

// The table/grid switch persists across a reload.
await page.getByLabel('Grid view').click()
await page.waitForTimeout(100)
const storedAfterClick = await page.evaluate(() => localStorage.getItem('bakery-servers-view'))
expect('grid view is remembered', storedAfterClick === 'grid', storedAfterClick)
await page.reload()
await page.getByTestId('servers-count').waitFor()
expect('grid view survives a reload', (await page.getByLabel('Grid view').getAttribute('aria-pressed')) === 'true')
await page.getByLabel('Table view').click()

// New server: empty submit shows the field errors.
await page.goto(`${WEB}/#/servers/new`)
await page.getByRole('heading', { name: 'New server' }).waitFor()
await page.getByRole('button', { name: 'Add server' }).click()
await page.waitForTimeout(100)
// The required host field blocks native submission before any server
// round-trip, so the browser's own constraint validation is what shows.
expect('empty submit shows field errors', (await page.locator('input:invalid').count()) > 0)

// A Viewer sees no "New server".
const viewer = await signedIn(1440, 900)
await viewer.route('**/api/me', async (route) => {
  const r = await route.fetch()
  const body = await r.json()
  body.permissions = body.permissions.filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
  for (const g of body.guilds ?? []) g.permissions = (g.permissions ?? []).filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
  await route.fulfill({ response: r, json: body })
})
await viewer.goto(`${WEB}/#/servers`)
await viewer.getByTestId('servers-count').waitFor()
expect('a Viewer sees no "New server"', (await viewer.getByRole('link', { name: 'New server' }).count()) === 0)

await browser.close()
if (failed) {
  console.log(`${failed} failed`)
  process.exit(1)
}
console.log('the Servers flows work')
