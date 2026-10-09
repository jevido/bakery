// The Conference Room (#/board-chat) in the dashboard, one section each,
// every flow once in the dark theme at 1440×900:
//   welcome  the header names the Guild's CEO over the Guild, the welcome
//            bubble comes from the CEO and the four chips are offered; a
//            chip fills the composer
//   reply    "Create a hiring plan" sent lands on the right, the typing
//            bubble and status line show while the CEO's Run waits or runs,
//            and the CEO's answer lands on the left, through the Desktop's
//            headless runner and the claude stand-in
//
//   bun e2e/board.ts [section ...]   (task web:board; needs task dev)
//
// The same environment as e2e/walk.ts overrides what it uses; SHOTS names a
// directory for screenshots.
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { chromium, type Page } from 'playwright-core'
import { desktopRunner } from './runner.ts'

const WEB = (process.env.BAKERY_WEB ?? 'http://127.0.0.1:4930').replace(/\/$/, '')
const CHROMIUM = process.env.CHROMIUM ?? '/usr/bin/chromium'
const SHOTS = process.env.SHOTS

function owner(): { email: string; password: string } {
  if (process.env.BAKERY_OWNER_EMAIL && process.env.BAKERY_OWNER_PASSWORD)
    return {
      email: process.env.BAKERY_OWNER_EMAIL,
      password: process.env.BAKERY_OWNER_PASSWORD,
    }
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

async function signedIn(): Promise<Page> {
  const ctx = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    reducedMotion: 'reduce',
  })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

type Agent = { id: number; name: string; job: string; status: string; created_at: string }

/**
 * The Guild's CEO, picked as the API does (its oldest CEO neither pending
 * approval nor terminated); one is hired and approved when it has none.
 */
async function theCEO(page: Page): Promise<Agent> {
  const { agents } = (await (await page.request.get(`${WEB}/api/agents?status=all`)).json()) as { agents: Agent[] }
  const ceo = agents
    .filter((a) => a.job === 'ceo' && a.status !== 'pending_approval' && a.status !== 'terminated')
    .sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id - b.id)[0]
  if (ceo) return ceo
  const r = await page.request.post(`${WEB}/api/agents`, {
    data: { name: 'Release Lead', job: 'ceo', title: 'CEO', icon: 'crown', capabilities: '', role_ids: [], reports_to: null },
  })
  if (!r.ok()) throw new Error(`hire the CEO: ${r.status()} ${await r.text()}`)
  const { agent, approval_id } = (await r.json()) as { agent: Agent; approval_id: number }
  const ok = await page.request.post(`${WEB}/api/approvals/${approval_id}/approve`, { data: {} })
  if (!ok.ok()) throw new Error(`approve the CEO: ${ok.status()} ${await ok.text()}`)
  return agent
}

/** Opens #/board-chat in a fresh session, so the chips are offered again. */
async function freshRoom(page: Page) {
  await page.goto(`${WEB}/#/board-chat`)
  await page.getByTestId('board-welcome').waitFor()
  const { issue } = (await (await page.request.get(`${WEB}/api/board-chat`)).json()) as { issue: { id: number } | null }
  if (issue && (await page.getByTestId('board-chips').count()) === 0) {
    const r = await page.request.post(`${WEB}/api/issues/${issue.id}/comments`, { data: { body: '/new' } })
    if (!r.ok()) throw new Error(`/new: ${r.status()} ${await r.text()}`)
    await page.reload()
  }
  await page.getByTestId('board-chips').waitFor()
}

const shot = (page: Page, name: string) => (SHOTS ? page.screenshot({ path: join(SHOTS, `board-${name}.png`) }) : Promise.resolve())

const sections: Record<string, () => Promise<void>> = {
  async welcome() {
    const page = await signedIn()
    try {
      const ceo = await theCEO(page)
      const { guild } = (await (await page.request.get(`${WEB}/api/me`)).json()) as { guild: { name: string } }
      await freshRoom(page)
      expect('the header names the CEO', (await page.getByTestId('board-chat-title').textContent())?.trim() === ceo.name)
      expect('the welcome bubble comes from the CEO', (await page.getByTestId('board-welcome').textContent())?.includes(`Welcome to ${guild.name}! I'm ${ceo.name}, your team lead`) ?? false)
      const chips = await page.getByTestId('board-chips').getByRole('button').allTextContents()
      expect(
        'the four chips are offered',
        JSON.stringify(chips.map((c) => c.trim())) === JSON.stringify(['Draft an Organization Brief', 'Create a hiring plan', 'Outline our first 30 days', 'Write an intro pitch']),
        chips,
      )
      await page.getByRole('button', { name: 'Write an intro pitch' }).click()
      const box = page.getByRole('textbox', { name: 'Message the Conference Room' })
      expect('a chip fills the composer', (await box.inputValue()).startsWith(`Write a short intro pitch for ${guild.name}`))
      expect('the composer has the focus', await box.evaluate((el) => el === document.activeElement))
      await shot(page, 'welcome')
    } finally {
      await page.context().close()
    }
  },

  async reply() {
    const page = await signedIn()
    const desktop = await desktopRunner(page, { BAKERY_STANDIN_DELAY: '600ms' })
    try {
      const ceo = await theCEO(page)
      await freshRoom(page)
      const answers = page.locator('[data-testid="chat-message"][data-from="agent"]')
      const before = await answers.count()
      const mine = page.locator('[data-testid="chat-message"][data-from="member"]')
      const sent = await mine.count()
      await page.getByRole('button', { name: 'Create a hiring plan' }).click()
      await page.getByRole('textbox', { name: 'Message the Conference Room' }).press('Enter')
      await page.waitForFunction((n) => document.querySelectorAll('[data-testid="chat-message"][data-from="member"]').length > n, sent)
      expect('the message lands on the right', (await mine.last().textContent())?.includes('Create a hiring plan for') ?? false)
      expect('the chips are gone', (await page.getByTestId('board-chips').count()) === 0)
      const status = page.getByTestId('board-status')
      await status.waitFor({ timeout: 15_000 })
      const text = (await status.textContent()) ?? ''
      expect('the status line waits for the Desktop or thinks', /Waiting for .+'s Desktop…|Thinking…/.test(text), text)
      await page.locator('[data-testid="board-status"][data-status="running"]').waitFor({ timeout: 30_000 })
      await page.waitForFunction(() => /Thinking…\s*\d+\.\ds/.test(document.querySelector('[data-testid="board-status"]')?.textContent ?? ''), null, { timeout: 10_000 })
      expect('a running Run says Thinking… with its seconds', true)
      await shot(page, 'thinking')
      await page.waitForFunction((n) => document.querySelectorAll('[data-testid="chat-message"][data-from="agent"]').length > n, before, { timeout: 90_000 })
      const last = answers.last()
      expect("the CEO's answer lands on the left", (await last.textContent())?.includes(ceo.name) ?? false)
      await status.waitFor({ state: 'detached', timeout: 15_000 })
      expect('the status line is gone', true)
      await shot(page, 'reply')
    } finally {
      await desktop.stop()
      await page.context().close()
    }
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
console.log('the Conference Room works')
