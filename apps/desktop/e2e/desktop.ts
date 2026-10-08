// The Desktop app through its frontend in headless Chromium, against its
// `serve` mode, one section each, every flow once in the dark theme at
// 1440×900 (the light theme and phone width wait for the guilds goal's final
// sweep):
//   shell    the guild rail, the sidebar with the lockup and the Desktop
//            app's version (read through /rpc), and "Connect a Bakery";
//            /rpc answers Version and refuses an unknown method with 404.
//   connect  "Connect a Bakery" to the dev Bakery: the approve link opened in
//            a second page signed in as the owner, Approve, the Bakery shown
//            connected; Disconnect removes it and the Bakery lists the
//            Desktop no more.
//   guilds   connected again: the rail shows the owner's Guilds and opens on
//            one; Default lists its Agents with the owner's hires marked "Runs
//            on this desktop"; a Guild created in the dashboard appears on
//            the rail and an Agent hired there appears in the list, both
//            without a reload; an Agent's page shows its Roles; a deleted
//            Guild leaves the rail. Disconnects at the end.
//
//   bun e2e/desktop.ts [section ...]   (needs task desktop:serve with no
//   Bakery connected, e.g. BAKERY_DESKTOP_HOME=$(mktemp -d), and task dev)
//
// BAKERY_DESKTOP overrides the serve address, BAKERY_WEB the dev Bakery's,
// CHROMIUM the browser. The owner comes from BAKERY_OWNER_EMAIL and
// BAKERY_OWNER_PASSWORD or infra/dev/state/owner.env.
import { readFileSync } from 'node:fs'
import { chromium, type APIRequestContext, type Page } from 'playwright-core'

const DESKTOP = (process.env.BAKERY_DESKTOP ?? 'http://127.0.0.1:4991').replace(/\/$/, '')
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
async function connectTo(p: Page): Promise<{ browserPage: Page; desktopID: number }> {
  await p.getByTestId('connect').getByRole('button', { name: 'Connect a Bakery' }).click()
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

const sections: Record<string, () => Promise<void>> = {
  async shell() {
    const p = await page()
    const errors: string[] = []
    p.on('pageerror', (e) => errors.push(e.message))
    await p.goto(DESKTOP)
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
    const p = await page()
    const errors: string[] = []
    p.on('pageerror', (e) => errors.push(e.message))
    await p.goto(DESKTOP)
    const { browserPage, desktopID } = await connectTo(p)
    const bakery = p.getByTestId('sidebar').getByTestId('bakery')
    await bakery.waitFor()
    expect('the sidebar lists the Bakery with the person', (await bakery.innerText()).includes('Owner'), await bakery.innerText())
    await p.getByTestId('guild-rail-item').first().waitFor()
    expect('the page shows the Bakery\'s guilds on the rail', true)

    await disconnectFrom(p)
    expect('Disconnect removes the Bakery', await p.getByTestId('connect').isVisible())
    const after = await desktopIds(browserPage.request)
    expect('the Bakery lists the Desktop no more', !after.includes(desktopID), after)
    expect('no page errors', errors.length === 0, errors)
    await browserPage.context().close()
    await p.context().close()
  },

  async guilds() {
    const p = await page()
    const errors: string[] = []
    p.on('pageerror', (e) => errors.push(e.message))
    await p.goto(DESKTOP)
    const { browserPage: web } = await connectTo(p)
    const me = (await (await web.request.get(`${WEB}/api/me`)).json()) as { member: { id: number }; guilds: Guild[] }
    const rail = p.getByTestId('guild-rail')
    const items = rail.getByTestId('guild-rail-item')
    await items.first().waitFor()
    expect('the rail shows the owner\'s guilds', (await items.count()) === me.guilds.length, [await items.count(), me.guilds.length])
    expect('one guild is marked', (await rail.locator('[aria-current="true"]').count()) === 1)

    const def = me.guilds.find((g) => g.name === 'Default') ?? me.guilds[0]
    await rail.getByRole('link', { name: def.name, exact: true }).click()
    await p.getByTestId('guild-name').filter({ hasText: def.name }).waitFor()
    expect(`picking "${def.name}" shows its name in the sidebar`, true)
    const sidebarAgents = p.getByTestId('sidebar').getByRole('link', { name: 'Agents' })
    expect('the sidebar has "Agents"', await sidebarAgents.isVisible())
    // Agents the owner hired are marked; others are not.
    const listed = (await (await web.request.get(`${WEB}/api/agents?status=all`, { headers: { 'Bakery-Guild': String(def.id) } })).json()) as {
      agents: { name: string; hirer: { id: number } | null }[]
    }
    const rows = p.getByTestId('agent-row')
    if (listed.agents.length) await rows.first().waitFor()
    expect('the list shows the guild\'s agents', (await rows.count()) === listed.agents.length, [await rows.count(), listed.agents.length])
    const owned = listed.agents.filter((a) => a.hirer?.id === me.member.id).length
    expect('the owner\'s hires are marked "Runs on this desktop"', (await p.getByTestId('runs-here').count()) === owned, [await p.getByTestId('runs-here').count(), owned])

    // A guild created in the dashboard appears on the rail without a reload.
    const name = `desk-${Date.now()}`
    const made = await web.request.post(`${WEB}/api/guilds`, { data: { name, description: 'made by the desktop e2e' } })
    expect('the dashboard creates a guild', made.ok(), made.status())
    await rail.getByRole('link', { name, exact: true }).waitFor({ timeout: 25_000 })
    expect('the new guild appears on the rail within 15 seconds', true)

    // An Agent hired in Default appears in the list without a reload, marked.
    await web.request.post(`${WEB}/api/guilds/${def.id}/switch`)
    const hireName = `Desk ${Date.now()}`
    const hire = await web.request.post(`${WEB}/api/agents`, {
      data: { name: hireName, job: 'engineer', title: 'Runner', icon: 'bot', capabilities: '', role_ids: [], reports_to: null },
    })
    expect('the dashboard hires an agent', hire.ok(), hire.status())
    const { approval_id } = (await hire.json()) as { approval_id: number }
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
    expect('the agent page shows its Roles', await agentPage.getByTestId('agent-roles').isVisible())
    expect('the agent page links to The Bakery', await agentPage.getByRole('button', { name: 'Open in The Bakery' }).isVisible())
    expect('the agent page has nothing to edit', (await agentPage.getByRole('textbox').count()) === 0)

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
console.log('the desktop app works')
