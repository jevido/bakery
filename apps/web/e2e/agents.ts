// The Guild's Agents pages in headless Chromium, one section each, every
// flow once in the dark theme at 1440×900 (the light theme and phone width
// wait for the guilds goal's final sweep):
//   hire    the sidebar's Guild section shows Agents; #/agents opens All,
//           which shows the empty state or the list; Hire agent hires "Ada"
//           (CTO, rocket icon) and says "Agent submitted for approval"; the
//           list shows Ada as Pending approval; after Approve on her card
//           on #/approvals/pending, All and Active show Ada as Idle. A run
//           before is cleaned up by terminating its Ada first.
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
