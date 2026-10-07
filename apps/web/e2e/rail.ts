// The guild rail in headless Chromium: signed in as the Owner (in two Guilds
// at least; a second one is made if needed), the rail shows every Guild left
// of the sidebar, marks the current one with the tall pill, names a Guild in
// a tooltip, switches on a click (the Guild menu and the Dashboard follow)
// and opens New guild from "+". With one Guild it shows that one and "+". At
// 375px it is only in the slide-over menu.
//
//   bun e2e/rail.ts            (task web:rail; needs task dev running)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { readFileSync } from 'node:fs'
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

type Guild = { id: number; name: string }
const browser = await chromium.launch({ executablePath: CHROMIUM })
const who = owner()

async function signedIn(width: number, height: number): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

const page = await signedIn(1440, 900)
let me = (await (await page.request.get(`${WEB}/api/me`)).json()) as { guild: Guild; guilds: Guild[] }
if (me.guilds.length < 2) {
  const r = await page.request.post(`${WEB}/api/guilds`, { data: { name: `rail-${Date.now()}`, description: 'made by the rail e2e' } })
  if (!r.ok()) throw new Error(`create a Guild: ${r.status()}`)
  await page.request.post(`${WEB}/api/guilds/${me.guild.id}/switch`)
  me = (await (await page.request.get(`${WEB}/api/me`)).json()) as typeof me
}
const start = me.guild
const other = me.guilds.find((g) => g.id !== start.id)!

await page.goto(`${WEB}/#/`)
const rail = page.getByTestId('guild-rail')
await rail.waitFor()
const items = rail.getByTestId('guild-rail-item')
expect('the rail lists every Guild', (await items.count()) === me.guilds.length, await items.count())
const railBox = (await rail.boundingBox())!
const sidebarBox = (await page.getByTestId('sidebar').boundingBox())!
expect('the rail is left of the sidebar', railBox.x + railBox.width <= sidebarBox.x + 1, [railBox, sidebarBox])
expect('the rail is 72px wide', Math.round(railBox.width) === 72, railBox.width)

const current = rail.locator('[aria-current="true"]')
expect('one Guild is current', (await current.count()) === 1, await current.count())
expect('it is the Current guild', (await current.getAttribute('aria-label')) === start.name, await current.getAttribute('aria-label'))
const pill = (await current.getByTestId('guild-rail-pill').boundingBox())!
expect('the current one has the 40px pill', Math.round(pill.height) === 40, pill.height)

const target = rail.getByRole('button', { name: other.name, exact: true })
await target.hover()
const tip = page.locator('[data-slot="tooltip-content"]')
await tip.waitFor()
expect('a hover names the Guild in a tooltip', (await tip.textContent())?.includes(other.name) ?? false, await tip.textContent())

await target.click()
await page.waitForFunction((name) => document.querySelector('[data-testid="guild-menu"]')?.textContent?.includes(name), other.name)
expect('a click switches the Guild menu', true)
expect('and opens the Dashboard', new URL(page.url()).hash === '#/', page.url())
expect('the clicked Guild is now current', (await rail.locator('[aria-current="true"]').getAttribute('aria-label')) === other.name)

await rail.getByTestId('guild-rail-new').click()
const dialog = page.getByRole('dialog')
await dialog.waitFor()
expect('"+" opens New guild', (await dialog.textContent())?.includes('New Guild') ?? false, await dialog.textContent())
await page.keyboard.press('Escape')

// Switch back, so the installation is left as it was found.
await page.request.post(`${WEB}/api/guilds/${start.id}/switch`)

// One Guild: /me answered with only the current one.
const one = await signedIn(1440, 900)
await one.route('**/api/me', async (route) => {
  const r = await route.fetch()
  const body = await r.json()
  body.guilds = body.guilds.filter((g: Guild) => g.id === body.guild.id)
  await route.fulfill({ response: r, json: body })
})
await one.goto(`${WEB}/#/`)
const oneRail = one.getByTestId('guild-rail')
await oneRail.waitFor()
expect('with one Guild the rail shows it', (await oneRail.getByTestId('guild-rail-item').count()) === 1)
expect('and "+"', await oneRail.getByTestId('guild-rail-new').isVisible())

// A phone: the rail is in the slide-over menu only.
const phone = await signedIn(375, 812)
await phone.goto(`${WEB}/#/`)
await phone.getByTestId('sidebar-drawer').waitFor({ state: 'attached' })
const closed = await phone.getByTestId('guild-rail').boundingBox()
expect('at 375px the rail is off screen while the menu is closed', !closed || closed.x + closed.width <= 0, closed)
await phone.getByRole('button', { name: 'Open sidebar', exact: true }).click()
await phone.waitForTimeout(300)
const opened = (await phone.getByTestId('guild-rail').boundingBox())!
const drawerRail = phone.getByTestId('sidebar-drawer').getByTestId('guild-rail')
expect('opening the menu shows the rail inside it', (await drawerRail.count()) === 1 && opened.x >= 0, opened)

await browser.close()
if (failed) {
  console.log(`${failed} failed`)
  process.exit(1)
}
console.log('the guild rail works')
