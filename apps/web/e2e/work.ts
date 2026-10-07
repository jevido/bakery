// The Guild's work pages (Goals; Issues and the Issue page follow) in
// headless Chromium, one section each, every flow once in the dark theme at
// 1440×900 (the light theme and phone width wait for the guilds goal's final
// sweep):
//   goals   the Goals page shows its empty state or its tree; New Goal
//           creates "Ship guilds" and its page a Sub-goal under it; the tree
//           shows both; the Goal page edits the title in place and changes
//           the status; a Markdown description renders a heading and a link
//           (opening in a new tab) and no <script>; the scratch Goals are
//           deleted again, the last one through the page's Delete
//
//   bun e2e/work.ts [section ...]   (task web:work; needs task dev)
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

async function signedIn(width = 1440, height = 900, theme: 'dark' | 'light' = 'dark'): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  await ctx.addInitScript((t) => localStorage.setItem('theme', t), theme)
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

type Goal = { id: number; title: string; status: string; parent_id: number | null; description: string }
async function goals(page: Page): Promise<Goal[]> {
  return ((await (await page.request.get(`${WEB}/api/goals`)).json()) as { goals: Goal[] }).goals
}

const scratch = ['Ship guilds', 'Ship guilds now', 'Guild rail']

/** Deletes the scratch Goals a run before may have left, Sub-goals first. */
async function clean(page: Page) {
  const left = (await goals(page)).filter((g) => scratch.includes(g.title))
  left.sort((a, b) => (b.parent_id ?? 0) - (a.parent_id ?? 0))
  for (const g of left) await page.request.delete(`${WEB}/api/goals/${g.id}`)
}

const sections: Record<string, () => Promise<void>> = {
  async goals() {
    const page = await signedIn()
    await clean(page)

    await page.goto(`${WEB}/#/goals`)
    const empty = page.getByText('No goals yet.')
    const tree = page.locator('[data-goal]').first()
    await Promise.race([empty.waitFor(), tree.waitFor()])
    expect('the Goals page shows its empty state or its tree', (await empty.isVisible()) || (await tree.isVisible()))
    expect('Goals is in the sidebar', await page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Goals' }).isVisible())

    // New Goal, or Add Goal on the empty state.
    await page.getByRole('button', { name: /^(New Goal|Add Goal)$/ }).first().click()
    const dialog = page.getByRole('dialog', { name: 'New goal' })
    await dialog.getByLabel('Goal title').fill('Ship guilds')
    await dialog.getByRole('button', { name: 'Level' }).click()
    await page.getByRole('option', { name: 'Guild' }).click()
    await dialog.getByRole('button', { name: 'Create goal' }).click()
    await page.locator('[data-goal="Ship guilds"]').waitFor()
    const parent = (await goals(page)).find((g) => g.title === 'Ship guilds')
    expect('New Goal creates "Ship guilds"', !!parent)

    await page.locator('[data-goal="Ship guilds"]').click()
    await page.getByRole('button', { name: 'Sub Goal' }).click()
    const sub = page.getByRole('dialog', { name: 'New sub-goal' })
    await sub.getByLabel('Goal title').fill('Guild rail')
    await sub.getByRole('button', { name: 'Create sub-goal' }).click()
    await page.locator('[data-goal="Guild rail"]').waitFor()
    const child = (await goals(page)).find((g) => g.title === 'Guild rail')
    expect('a Sub-goal is created under it', child?.parent_id === parent?.id, child)

    await page.goto(`${WEB}/#/goals`)
    await page.locator('[data-goal="Guild rail"]').waitFor()
    expect('the tree shows both', await page.locator('[data-goal="Ship guilds"]').isVisible())

    await page.goto(`${WEB}/#/goals/${parent!.id}`)
    await page.locator('[data-inline-editor="Title"]').click()
    await page.getByLabel('Title', { exact: true }).fill('Ship guilds now')
    await page.keyboard.press('Enter')
    await page.locator('[data-inline-editor="Title"]', { hasText: 'Ship guilds now' }).waitFor()
    await page.waitForTimeout(300)
    expect('the title is edited in place', (await goals(page)).find((g) => g.id === parent!.id)?.title === 'Ship guilds now')

    const properties = page.getByRole('complementary', { name: 'Properties' })
    await properties.getByRole('button', { name: 'Status' }).click()
    await page.getByRole('option', { name: 'active' }).click()
    await properties.getByText('active').waitFor()
    await page.waitForTimeout(300)
    expect('the status changes', (await goals(page)).find((g) => g.id === parent!.id)?.status === 'active')

    await page.locator('[data-inline-editor="Description"]').click()
    await page
      .getByLabel('Description', { exact: true })
      .fill('# The plan\n\nRead [the docs](https://example.com/docs).\n\n<script>window.pwned = 1</script><img src=x onerror="window.pwned = 1">')
    await page.locator('[data-inline-editor="Title"]').focus()
    const body = page.locator('[data-inline-editor="Description"]')
    await body.getByRole('heading', { name: 'The plan' }).waitFor()
    const link = body.getByRole('link', { name: 'the docs' })
    expect('a Markdown description renders a heading', await body.locator('h1').isVisible())
    expect(
      'and a link opening in a new tab',
      (await link.getAttribute('href')) === 'https://example.com/docs' && (await link.getAttribute('target')) === '_blank' && (await link.getAttribute('rel')) === 'noreferrer',
    )
    expect('and no <script> or handler', (await body.locator('script').count()) === 0 && (await body.locator('[onerror]').count()) === 0)
    expect('nothing ran', (await page.evaluate(() => (window as unknown as { pwned?: number }).pwned)) === undefined)

    await page.request.delete(`${WEB}/api/goals/${child!.id}`)
    await page.reload()
    await properties.getByRole('button', { name: 'Delete goal' }).click()
    await page.getByRole('alertdialog').or(page.getByRole('dialog')).getByRole('button', { name: 'Delete', exact: true }).click()
    await page.waitForURL(/#\/goals$/)
    await page.waitForTimeout(300)
    const left = (await goals(page)).filter((g) => scratch.includes(g.title))
    expect('the scratch Goals are deleted again', left.length === 0, left)

    await page.close()
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
console.log('the work flows work')
