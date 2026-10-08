// The Guild's Agents pages in headless Chromium, one section each, every
// flow once in the dark theme at 1440×900 (the light theme and phone width
// wait for the guilds goal's final sweep):
//   hire    the sidebar's Guild section shows Agents; #/agents opens All,
//           which shows the empty state or the list; Hire agent hires "Ada"
//           (CTO, rocket icon) and says "Agent submitted for approval"; the
//           list shows Ada as Pending approval; after Approve on her card
//           on #/approvals/pending, All and Active show Ada as Idle. A run
//           before is cleaned up by terminating its Ada first.
//   org     with Org Ada (CTO) and Org Bob reporting to her, hired and
//           approved through the API, the Org chart view shows Bob's card
//           below Ada's joined by a line; dragging moves the chart; zoom in
//           changes the scale; Fit brings both into view; ?view=org survives
//           a reload; the Paused tab hides both; clicking Bob opens his page.
//
//   bun e2e/agents.ts [section ...]   (task web:agents; needs task dev)
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

const browser = await chromium.launch({ executablePath: CHROMIUM })
const who = owner()

async function signedIn(width = 1440, height = 900): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

type Agent = { id: number; name: string; status: string; approval_id: number | null }

async function agents(page: Page, status = 'all'): Promise<Agent[]> {
  return ((await (await page.request.get(`${WEB}/api/agents?status=${status}`)).json()) as { agents: Agent[] }).agents
}

/** Terminates the Agents a run before left behind under these names. */
async function terminate(page: Page, names: string[]) {
  for (const a of (await agents(page)).filter((a) => names.includes(a.name))) {
    const r = await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
    if (!r.ok()) throw new Error(`terminate ${a.name}: ${r.status()} ${await r.text()}`)
  }
}

/** Hires an Agent through the API and approves its hire. */
async function hired(page: Page, hire: { name: string; job: string; reports_to?: number }): Promise<Agent> {
  const r = await page.request.post(`${WEB}/api/agents`, { data: { title: '', icon: 'bot', capabilities: '', role_ids: [], reports_to: null, ...hire } })
  if (!r.ok()) throw new Error(`hire ${hire.name}: ${r.status()} ${await r.text()}`)
  const { agent, approval_id } = (await r.json()) as { agent: Agent; approval_id: number }
  const ok = await page.request.post(`${WEB}/api/approvals/${approval_id}/approve`, { data: {} })
  if (!ok.ok()) throw new Error(`approve ${hire.name}: ${ok.status()} ${await ok.text()}`)
  return agent
}

/** The card layer's transform: its translate and scale. */
async function cardLayer(page: Page): Promise<string> {
  return (await page.getByTestId('org-chart-card-layer').getAttribute('style')) ?? ''
}

const row = (page: Page, name: string) => page.getByTestId('agent-row').filter({ hasText: name })

const sections: Record<string, () => Promise<void>> = {
  async hire() {
    const page = await signedIn()
    await terminate(page, ['Ada'])

    await page.goto(`${WEB}/#/agents`)
    const empty = page.getByText('No agents yet.', { exact: false })
    const list = page.getByTestId('agent-row').first()
    await Promise.race([empty.waitFor(), list.waitFor()])
    expect('#/agents opens All', page.url().endsWith('#/agents/all'), page.url())
    expect('All shows the empty state or the list', (await empty.isVisible()) || (await list.isVisible()))
    if ((await agents(page)).length === 0) expect('with no Agents, All shows the empty state', await empty.isVisible())
    const nav = page.getByRole('navigation', { name: 'Main' })
    expect('Agents is in the sidebar, current', (await nav.getByRole('link', { name: 'Agents' }).getAttribute('aria-current')) === 'page')

    await page.getByRole('button', { name: 'Hire agent' }).first().click()
    const dialog = page.getByRole('dialog', { name: 'Hire agent' })
    await dialog.getByText('Meet your next agent').waitFor()
    await dialog.getByLabel('Agent name').fill('Ada')
    await dialog.getByLabel('Job').selectOption('cto')
    await dialog.getByRole('button', { name: 'Agent icon' }).click()
    await page.getByRole('option', { name: 'rocket' }).click()
    await dialog.getByRole('button', { name: 'Hire', exact: true }).click()
    await dialog.getByText('Agent submitted for approval').waitFor()
    expect('the hire says "Agent submitted for approval"', true)
    expect('it links to the Approval', (await dialog.getByRole('link', { name: 'View approval' }).getAttribute('href'))!.startsWith('#/approvals/'))
    await dialog.getByRole('button', { name: 'Done' }).click()

    await row(page, 'Ada').waitFor()
    expect('the list shows Ada as Pending approval', (await row(page, 'Ada').textContent())!.includes('Pending approval'), await row(page, 'Ada').textContent())
    expect('her row shows the Job', (await row(page, 'Ada').textContent())!.includes('CTO'))
    const ada = (await agents(page)).find((a) => a.name === 'Ada')!

    await page.goto(`${WEB}/#/approvals/pending`)
    const card = page.locator(`[data-slot="card"][data-approval="${ada.approval_id}"]`)
    await card.waitFor()
    await card.getByRole('button', { name: 'Approve' }).click()
    await page.waitForURL(/#\/approvals\/\d+\?resolved=approved/)

    for (const tab of ['all', 'active']) {
      await page.goto(`${WEB}/#/agents/${tab}`)
      await row(page, 'Ada').waitFor()
      expect(`${tab} shows Ada as Idle`, (await row(page, 'Ada').textContent())!.includes('Idle'), await row(page, 'Ada').textContent())
    }
    await page.context().close()
  },

  async org() {
    const page = await signedIn()
    await terminate(page, ['Org Ada', 'Org Bob'])
    const ada = await hired(page, { name: 'Org Ada', job: 'cto' })
    const bob = await hired(page, { name: 'Org Bob', job: 'engineer', reports_to: ada.id })

    await page.goto(`${WEB}/#/agents/all`)
    await page.getByRole('button', { name: 'Org chart view' }).click()
    const card = (name: string) => page.getByTestId('org-chart-card').filter({ hasText: name })
    await card('Org Bob').waitFor()
    expect('the view is in the hash query', page.url().endsWith('#/agents/all?view=org'), page.url())
    const [a, b] = [await card('Org Ada').boundingBox(), await card('Org Bob').boundingBox()]
    expect("Bob's card is below Ada's", !!a && !!b && b.y > a.y + a.height, { a, b })
    expect('a line joins them', (await page.getByTestId('org-chart-edge').count()) > 0)
    expect("Bob's card shows his Job", (await card('Org Bob').textContent())!.includes('Engineer'))

    const viewport = (await page.getByTestId('org-chart-viewport').boundingBox())!
    const inside = (box: { x: number; y: number; width: number; height: number } | null) =>
      !!box && box.x >= viewport.x && box.y >= viewport.y && box.x + box.width <= viewport.x + viewport.width && box.y + box.height <= viewport.y + viewport.height
    expect('it fits both on first show', inside(await card('Org Ada').boundingBox()) && inside(await card('Org Bob').boundingBox()))

    const before = (await card('Org Ada').boundingBox())!
    await page.mouse.move(viewport.x + 20, viewport.y + viewport.height - 20)
    await page.mouse.down()
    await page.mouse.move(viewport.x + 220, viewport.y + viewport.height - 120, { steps: 5 })
    await page.mouse.up()
    const after = (await card('Org Ada').boundingBox())!
    expect('dragging moves the chart', Math.abs(after.x - before.x - 200) < 2 && Math.abs(after.y - before.y + 100) < 2, { before, after })

    const unzoomed = await cardLayer(page)
    await page.getByRole('button', { name: 'Zoom in' }).click()
    const zoomed = await cardLayer(page)
    expect('zoom in changes the scale', zoomed !== unzoomed && /scale\(/.test(zoomed), { unzoomed, zoomed })
    for (let i = 0; i < 4; i++) await page.getByRole('button', { name: 'Zoom in' }).click()
    await page.getByRole('button', { name: 'Fit chart to screen' }).click()
    expect('Fit brings both into view', inside(await card('Org Ada').boundingBox()) && inside(await card('Org Bob').boundingBox()))

    await page.reload()
    await card('Org Bob').waitFor()
    expect('?view=org survives a reload', page.url().endsWith('?view=org'))

    await page.getByRole('tab', { name: 'Paused' }).click()
    await page.waitForURL(/#\/agents\/paused\?view=org$/)
    await page.getByRole('tab', { name: 'Paused', selected: true }).waitFor()
    await page.waitForTimeout(300)
    expect('the Paused tab hides both', (await card('Org Ada').count()) === 0 && (await card('Org Bob').count()) === 0)

    await page.getByRole('tab', { name: 'All' }).click()
    await card('Org Bob').click()
    await page.waitForURL(new RegExp(`#/agents/${bob.id}$`))
    expect('clicking Bob opens his page', true)
    await terminate(page, ['Org Ada', 'Org Bob'])
    await page.context().close()
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
console.log('the agents flows work')
