// The Desktop app through its frontend in headless Chromium, against its
// `serve` mode, one section each, every flow once in the dark theme at
// 1440×900 (the light theme and phone width wait for the guilds goal's final
// sweep). Every section connects to the dev Bakery itself and disconnects at
// the end, so each runs on its own:
//   shell       the guild rail, the sidebar with the lockup and the Desktop
//               app's version (read through /rpc), and "Connect a Bakery";
//               /rpc answers Version and refuses an unknown method with 404.
//   connect     "Connect a Bakery" to the dev Bakery: the approve link opened
//               in a second page signed in as the owner, Approve, the Bakery
//               shown connected and bakeries.json mode 0600; a second
//               connect cancelled in the desktop shows cancelled on the
//               approve page.
//   guilds      the rail shows the owner's Guilds and opens on one; a Guild
//               created in the dashboard appears on the rail without a
//               reload, and leaves it once deleted.
//   agents      Default lists its Agents with the owner's hires marked "Runs
//               on this desktop"; a scratch Agent hired and approved in the
//               dashboard appears without a reload, its read-only page shows
//               its Roles, and it moves to Paused and then Terminated.
//   disconnect  Disconnect removes the Bakery, the dashboard's Desktops page
//               lists it no more and its key answers 401; a Desktop signed
//               out on the Desktops page shows "Signed out" and "Connect
//               again" in the desktop, which connects it again.
//
//   bun e2e/desktop.ts [section ...]   (task desktop:e2e; needs task dev)
//
// It starts `go run . serve` on 127.0.0.1:4991 itself, with a fresh
// BAKERY_DESKTOP_HOME, and stops it at the end; the frontend must be built
// (task desktop:e2e builds it). BAKERY_DESKTOP points it at a serve already
// running instead (then bakeries.json is not checked), BAKERY_WEB at another
// dev Bakery, CHROMIUM at another browser. The owner comes from
// BAKERY_OWNER_EMAIL and BAKERY_OWNER_PASSWORD or infra/dev/state/owner.env.
import { spawn } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync, statSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { chromium, type APIRequestContext, type Page } from 'playwright-core'

const DESKTOP = (process.env.BAKERY_DESKTOP ?? 'http://127.0.0.1:4991').replace(/\/$/, '')
// The serve mode's home when this script starts it; null for BAKERY_DESKTOP.
const HOME = process.env.BAKERY_DESKTOP ? null : mkdtempSync(join(tmpdir(), 'bakery-desktop-e2e-'))
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

/** The ids of the owner's signed-in Desktops, as the Bakery lists them. */
async function desktopIds(request: APIRequestContext): Promise<number[]> {
  const r = await request.get(`${WEB}/api/desktops`)
  return ((await r.json()) as { id: number }[]).map((d) => d.id)
}

let failed = 0
function expect(what: string, ok: boolean, got?: unknown) {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : ` (got ${JSON.stringify(got)})`}`)
  if (!ok) failed++
}

const browser = await chromium.launch({ executablePath: CHROMIUM })

async function page() {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  return ctx.newPage()
}

/** Connects the desktop page p to the dev Bakery, approving it in a second
 * page signed in as the owner, which is returned with the new Desktop's id. */
async function connectTo(
  p: Page,
  open = () => p.getByTestId('connect').getByRole('button', { name: 'Connect a Bakery' }).click(),
): Promise<{ browserPage: Page; desktopID: number }> {
  await open()
  const dialog = p.getByTestId('connect-dialog')
  await dialog.getByLabel('Address').fill(WEB)
  await dialog.getByRole('button', { name: 'Connect' }).click()
  const link = dialog.getByTestId('approval-link')
  await link.waitFor()
  const approval = (await link.getAttribute('href')) ?? ''
  expect('the dialog asks to approve in the browser, with the link', approval.startsWith(`${WEB}/#/desktop-sign-in/`), approval)

  // The person's browser: signed in as the owner, on the approve page.
  const browserPage = await page()
  const login = await browserPage.request.post(`${WEB}/api/login`, { data: owner() })
  expect('the owner signs in to the Bakery', login.ok(), login.status())
  const before = await desktopIds(browserPage.request)
  await browserPage.goto(approval)
  await browserPage.getByRole('button', { name: 'Approve' }).click()
  await browserPage.getByRole('heading', { name: 'Desktop app approved' }).waitFor()

  await dialog.getByText('Connected', { exact: true }).waitFor({ timeout: 15_000 })
  expect('the dialog says connected', true)
  await dialog.getByRole('button', { name: 'Done' }).click()
  const added = (await desktopIds(browserPage.request)).filter((id) => !before.includes(id))
  expect('the Bakery lists one new Desktop', added.length === 1, added)
  return { browserPage, desktopID: added[0] }
}

/** Disconnects the one connected Bakery from the sidebar. */
async function disconnectFrom(p: Page) {
  const bakery = p.getByTestId('sidebar').getByTestId('bakery')
  await bakery.getByRole('button', { name: `${WEB} menu` }).click()
  await p.getByRole('menuitem', { name: 'Disconnect' }).click()
  await bakery.waitFor({ state: 'detached' })
}


type Guild = { id: number; name: string }
type Me = { member: { id: number }; guilds: Guild[] }

/** The desktop page, opened on the serve mode, with its page errors. */
async function desktop(): Promise<{ p: Page; errors: string[] }> {
  const p = await page()
  const errors: string[] = []
  p.on('pageerror', (e) => errors.push(e.message))
  await p.goto(DESKTOP)
  return { p, errors }
}

/** The connected Bakeries as the serve mode keeps them in bakeries.json. */
function stored(): { address: string; desktop_id: number; key: string }[] {
  if (!HOME) return []
  return (JSON.parse(readFileSync(join(HOME, 'bakeries.json'), 'utf8')) as { bakeries: { address: string; desktop_id: number; key: string }[] }).bakeries
}

/** The owner's Default guild (or their first), made current for the web
 * page's requests. */
async function defaultGuild(web: Page): Promise<{ me: Me; def: Guild }> {
  const me = (await (await web.request.get(`${WEB}/api/me`)).json()) as Me
  const def = me.guilds.find((g) => g.name === 'Default') ?? me.guilds[0]
  await web.request.post(`${WEB}/api/guilds/${def.id}/switch`)
  return { me, def }
}

const sections: Record<string, () => Promise<void>> = {
  async shell() {
    const { p, errors } = await desktop()
    await p.getByTestId('connect').waitFor()
    expect('the title is "The Bakery"', (await p.title()) === 'The Bakery', await p.title())
    expect('the dark theme is on', await p.evaluate(() => document.documentElement.classList.contains('dark')))
    expect('the guild rail is there', await p.getByRole('navigation', { name: 'Guilds' }).isVisible())
    expect('the sidebar shows the lockup', await p.getByTestId('sidebar').getByRole('img', { name: 'The Bakery' }).isVisible())
    const version = p.getByTestId('version')
    await version.filter({ hasText: 'Desktop app' }).waitFor()
    expect('the sidebar shows the version from /rpc', /^Desktop app \S+$/.test(await version.innerText()), await version.innerText())
    const connect = p.getByTestId('connect').getByRole('button', { name: 'Connect a Bakery' })
    expect('"Connect a Bakery" is shown and enabled', (await connect.isVisible()) && (await connect.isEnabled()))
    expect('no page errors', errors.length === 0, errors)

    const v = await p.request.post(`${DESKTOP}/rpc/Version`, { data: [] })
    expect('POST /rpc/Version answers {result}', v.ok() && typeof ((await v.json()) as { result: unknown }).result === 'string', await v.text())
    const unknown = await p.request.post(`${DESKTOP}/rpc/Nope`, { data: [] })
    expect('an unknown method answers 404', unknown.status() === 404, unknown.status())
    await p.context().close()
  },

  async connect() {
    const { p, errors } = await desktop()
    const { browserPage: web } = await connectTo(p)
    const bakery = p.getByTestId('sidebar').getByTestId('bakery')
    await bakery.waitFor()
    expect('the sidebar lists the Bakery with the person', (await bakery.innerText()).includes('Owner'), await bakery.innerText())
    await p.getByTestId('guild-rail-item').first().waitFor()
    expect("the page shows the Bakery's guilds on the rail", true)
    if (HOME) {
      const mode = statSync(join(HOME, 'bakeries.json')).mode & 0o777
      expect('bakeries.json is mode 0600', mode === 0o600, mode.toString(8))
      expect('bakeries.json holds the Bakery', stored().some((b) => b.address === WEB), stored().map((b) => b.address))
    }

    // A second connect cancelled in the desktop is cancelled on the Bakery.
    await p.getByTestId('sidebar').getByRole('button', { name: 'Connect a Bakery' }).click()
    const dialog = p.getByTestId('connect-dialog')
    await dialog.getByLabel('Address').fill(WEB)
    await dialog.getByRole('button', { name: 'Connect' }).click()
    const link = dialog.getByTestId('approval-link')
    await link.waitFor()
    const approval = (await link.getAttribute('href')) ?? ''
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await dialog.waitFor({ state: 'detached' })
    await web.goto(approval)
    await web.getByTestId('desktop-sign-in-cancelled').waitFor({ timeout: 10_000 })
    expect('a connect cancelled in the desktop shows cancelled on the approve page', true)

    await disconnectFrom(p)
    expect('no page errors', errors.length === 0, errors)
    await web.context().close()
    await p.context().close()
  },

  async guilds() {
    const { p, errors } = await desktop()
    const { browserPage: web } = await connectTo(p)
    const { me, def } = await defaultGuild(web)
    const rail = p.getByTestId('guild-rail')
    const items = rail.getByTestId('guild-rail-item')
    await items.first().waitFor()
    expect("the rail shows the owner's guilds", (await items.count()) === me.guilds.length, [await items.count(), me.guilds.length])
    expect('one guild is marked', (await rail.locator('[aria-current="true"]').count()) === 1)

    await rail.getByRole('link', { name: def.name, exact: true }).click()
    await p.getByTestId('guild-name').filter({ hasText: def.name }).waitFor()
    expect(`picking "${def.name}" shows its name in the sidebar`, true)
    expect('the sidebar has "Agents"', await p.getByTestId('sidebar').getByRole('link', { name: 'Agents' }).isVisible())

    // A guild created in the dashboard appears on the rail without a reload.
    const name = `desk-${Date.now()}`
    const made = await web.request.post(`${WEB}/api/guilds`, { data: { name, description: 'made by the desktop e2e' } })
    expect('the dashboard creates a guild', made.ok(), made.status())
    await rail.getByRole('link', { name, exact: true }).waitFor({ timeout: 25_000 })
    expect('the new guild appears on the rail within 15 seconds', true)

    // A deleted guild leaves the rail.
    const madeID = ((await made.json()) as { guild: Guild }).guild.id
    await web.request.post(`${WEB}/api/guilds/${madeID}/switch`)
    const gone = await web.request.delete(`${WEB}/api/guilds/current`)
    expect('the dashboard deletes the new guild', gone.ok(), gone.status())
    await rail.getByRole('link', { name, exact: true }).waitFor({ state: 'detached', timeout: 25_000 })
    expect('the deleted guild leaves the rail', true)
    await web.request.post(`${WEB}/api/guilds/${def.id}/switch`)

    await disconnectFrom(p)
    expect('no page errors', errors.length === 0, errors)
    await web.context().close()
    await p.context().close()
  },

  async agents() {
    const { p, errors } = await desktop()
    const { browserPage: web } = await connectTo(p)
    const { me, def } = await defaultGuild(web)
    await p.getByTestId('guild-rail').getByRole('link', { name: def.name, exact: true }).click()
    await p.getByTestId('guild-name').filter({ hasText: def.name }).waitFor()

    // Agents the owner hired are marked; others are not.
    const listed = (await (await web.request.get(`${WEB}/api/agents`)).json()) as { agents: { name: string; hirer: { id: number } | null }[] }
    const rows = p.getByTestId('agent-row')
    if (listed.agents.length) await rows.first().waitFor()
    expect("the list shows the guild's agents", (await rows.count()) === listed.agents.length, [await rows.count(), listed.agents.length])
    const owned = listed.agents.filter((a) => a.hirer?.id === me.member.id).length
    expect('the owner\'s hires are marked "Runs on this desktop"', (await p.getByTestId('runs-here').count()) === owned, [await p.getByTestId('runs-here').count(), owned])

    // A scratch Agent hired with the Viewer Role appears without a reload, marked.
    const roles = (await (await web.request.get(`${WEB}/api/roles`)).json()) as { roles: { id: number; name: string }[] }
    const viewer = roles.roles.find((r) => r.name === 'Viewer')
    const hireName = `Desk ${Date.now()}`
    const hire = await web.request.post(`${WEB}/api/agents`, {
      data: { name: hireName, job: 'engineer', title: 'Runner', icon: 'bot', capabilities: '', role_ids: viewer ? [viewer.id] : [], reports_to: null },
    })
    expect('the dashboard hires an agent', hire.ok(), hire.status())
    const { agent, approval_id } = (await hire.json()) as { agent: { id: number }; approval_id: number }
    await web.request.post(`${WEB}/api/approvals/${approval_id}/approve`, { data: {} })
    const row = rows.filter({ hasText: hireName })
    await row.waitFor({ timeout: 25_000 })
    expect('the hire appears in the list without a reload', true)
    expect('the hire is marked "Runs on this desktop"', await row.getByTestId('runs-here').isVisible())

    // Its page shows its Roles, read-only.
    await row.getByRole('link', { name: `Open ${hireName}` }).click({ position: { x: 8, y: 8 } })
    const agentPage = p.getByTestId('agent')
    await agentPage.waitFor()
    expect('the agent page shows its name', await agentPage.getByRole('heading', { name: hireName }).isVisible())
    expect('the agent page shows its Roles', await agentPage.getByTestId('agent-roles').locator('[data-role="Viewer"]').isVisible())
    expect('the agent page links to The Bakery', await agentPage.getByRole('button', { name: 'Open in The Bakery' }).isVisible())
    expect('the agent page has nothing to edit', (await agentPage.getByRole('textbox').count()) === 0)

    // Paused, then Terminated, each under its tab.
    await p.getByTestId('sidebar').getByRole('link', { name: 'Agents' }).click()
    const paused = await web.request.post(`${WEB}/api/agents/${agent.id}/pause`)
    expect('the dashboard pauses it', paused.ok(), paused.status())
    await p.getByRole('tab', { name: 'Paused' }).click()
    await rows.filter({ hasText: hireName }).waitFor({ timeout: 25_000 })
    expect('it is listed under Paused', true)
    const ended = await web.request.post(`${WEB}/api/agents/${agent.id}/terminate`)
    expect('the dashboard terminates it', ended.ok(), ended.status())
    await p.getByRole('tab', { name: 'Terminated' }).click()
    await rows.filter({ hasText: hireName }).waitFor({ timeout: 25_000 })
    expect('it is listed under Terminated', true)
    await p.getByRole('tab', { name: 'All' }).click()
    await p.getByRole('tab', { name: 'All', selected: true }).waitFor()
    // Within 5 seconds: the list is asked again, not kept from before.
    const left = await rows.filter({ hasText: hireName }).waitFor({ state: 'detached', timeout: 5_000 }).then(() => true, () => false)
    expect('it leaves All', left)

    await disconnectFrom(p)
    expect('no page errors', errors.length === 0, errors)
    await web.context().close()
    await p.context().close()
  },

  async disconnect() {
    const { p, errors } = await desktop()
    const { browserPage: web, desktopID } = await connectTo(p)
    const key = stored().find((b) => b.desktop_id === desktopID)?.key
    await disconnectFrom(p)
    expect('Disconnect removes the Bakery', await p.getByTestId('connect').isVisible())
    await web.goto(`${WEB}/#/security/api-tokens`)
    await web.getByRole('tab', { name: 'Desktops' }).click()
    await web.getByRole('tab', { name: 'Desktops', selected: true }).waitFor()
    await web.waitForLoadState('networkidle')
    expect('the Desktops page lists it no more', (await web.locator(`[data-testid="desktop"][data-id="${desktopID}"]`).count()) === 0)
    if (key) {
      const r = await fetch(`${WEB}/api/me`, { headers: { Authorization: `Bearer ${key}` } })
      expect('its key answers 401', r.status === 401, r.status)
    }

    // Signed out from the dashboard's Desktops page: the desktop says so.
    const again = await connectTo(p)
    await p.getByTestId('guild-rail-item').first().waitFor()
    await web.reload()
    await web.getByRole('tab', { name: 'Desktops' }).click()
    const row = web.locator(`[data-testid="desktop"][data-id="${again.desktopID}"]`)
    await row.getByTestId('sign-out-desktop').click()
    await web.getByRole('dialog').getByRole('button', { name: 'Sign out', exact: true }).click()
    await row.waitFor({ state: 'detached' })
    expect('the Desktops page signs it out', true)
    const bakery = p.getByTestId('sidebar').getByTestId('bakery')
    await bakery.filter({ hasText: 'Signed out' }).waitFor({ timeout: 25_000 })
    expect('the desktop shows the Bakery signed out', true)
    const connectAgain = p.getByTestId('connected').getByRole('button', { name: 'Connect again' })
    expect('with "Connect again"', await connectAgain.isVisible())
    await again.browserPage.context().close()

    const third = await connectTo(p, () => connectAgain.click())
    await bakery.filter({ hasText: 'Owner' }).waitFor()
    expect('"Connect again" connects it again', true)
    await disconnectFrom(p)
    expect('no page errors', errors.length === 0, errors)
    await third.browserPage.context().close()
    await web.context().close()
    await p.context().close()
  },
}

const asked = process.argv.slice(2)
const unknown = asked.filter((s) => !(s in sections))
if (unknown.length) {
  console.log(`unknown section ${unknown.join(', ')}; the sections are ${Object.keys(sections).join(', ')}`)
  process.exit(2)
}

// The serve mode, on its own process group so stopping it stops the binary
// `go run` started too.
const serve = HOME
  ? spawn('go', ['run', '.', 'serve', '--addr', new URL(DESKTOP).host], {
      cwd: new URL('..', import.meta.url).pathname,
      env: { ...process.env, BAKERY_DESKTOP_HOME: HOME },
      detached: true,
      stdio: ['ignore', 'inherit', 'inherit'],
    })
  : null
function stop() {
  if (serve?.pid) {
    try {
      process.kill(-serve.pid, 'SIGTERM')
    } catch {}
  }
  if (HOME) rmSync(HOME, { recursive: true, force: true })
}
if (serve) {
  let exited = false
  serve.on('exit', () => (exited = true))
  const deadline = Date.now() + 120_000
  for (;;) {
    if (exited) throw new Error(`serve exited (is ${new URL(DESKTOP).host} taken, or the frontend not built?)`)
    const up = await fetch(`${DESKTOP}/rpc/Version`, { method: 'POST', body: '[]' }).then((r) => r.ok, () => false)
    if (up) break
    if (Date.now() > deadline) {
      stop()
      throw new Error('serve did not answer within 2 minutes')
    }
    await new Promise((r) => setTimeout(r, 500))
  }
}

try {
  for (const name of asked.length ? asked : Object.keys(sections)) {
    console.log(`# ${name}`)
    await sections[name]()
  }
} finally {
  await browser.close()
  stop()
}
if (failed) {
  console.log(`${failed} failed`)
  process.exit(1)
}
console.log('the desktop app works')
