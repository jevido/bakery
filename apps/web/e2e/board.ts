// The Conference Room (#/board-chat) in the dashboard, one section each,
// every flow once in the dark theme at 1440×900:
//   welcome  the header names the Guild's CEO over the Guild, the welcome
//            bubble comes from the CEO and the four chips are offered; a
//            chip fills the composer
//   reply    "Create a hiring plan" sent lands on the right, the typing
//            bubble and status line show while the CEO's Run waits or runs,
//            and the CEO's answer lands on the left, through the Desktop's
//            headless runner and the claude stand-in
//   two-members  a second Member sees the first one's message with their
//            name and the answer, and their follow-up's prompt holds both
//            Members' turns and the CEO's own answer as role="you"
//   new-chat "New chat" shows the New session divider, wakes nobody, and
//            the next prompt no longer holds the turns before it
//   no-ceo   in a scratch Guild with no CEO the page says so, and "Hire a
//            CEO" opens the hire dialog with Job CEO
//   feed     the Activity feed beside it: a new Issue's card lands by
//            polling, "In Review" filters, "Show all activity" brings back
//            hidden events, group by Issue folds them, and the divider drags
//            and keeps its place over a reload
//
//   bun e2e/board.ts [section ...]   (task web:board; needs task dev)
//
// The same environment as e2e/walk.ts overrides what it uses; SHOTS names a
// directory for screenshots.
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { chromium, type Page } from 'playwright-core'
import { desktopRunner } from './runner.ts'
import { promptOf, runsOf } from './runs.ts'

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

type Agent = { id: number; name: string; job: string; status: string }

/**
 * The Guild's CEO, picked as the API does (its oldest CEO neither pending
 * approval nor terminated); one is hired and approved when it has none.
 */
async function theCEO(page: Page): Promise<Agent> {
  const { agents } = (await (await page.request.get(`${WEB}/api/agents?status=all`)).json()) as { agents: Agent[] }
  const ceo = agents
    .filter((a) => a.job === 'ceo' && a.status !== 'pending_approval' && a.status !== 'terminated')
    .sort((a, b) => a.id - b.id)[0]
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

/** A Member invited for the run, signed in in a context of their own; leave removes them. */
async function invited(owner: Page): Promise<{ page: Page; name: string; leave: () => Promise<void> }> {
  const { members } = (await (await owner.request.get(`${WEB}/api/members`)).json()) as { members: { id: number; email: string }[] }
  for (const m of members.filter((m) => m.email.startsWith('board-member-'))) await owner.request.delete(`${WEB}/api/members/${m.id}`)
  const inv = await owner.request.post(`${WEB}/api/invitations`, { data: { email: `board-member-${Date.now()}@example.test`, role: 'member' } })
  if (!inv.ok()) throw new Error(`invite: ${inv.status()} ${await inv.text()}`)
  const token = ((await inv.json()) as { path: string }).path.split('/').pop()
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  const name = 'Bo Board'
  const accept = await ctx.request.post(`${WEB}/api/invitations/by-token/${token}/accept`, { data: { name, password: 'a long enough password' } })
  if (!accept.ok()) throw new Error(`accept: ${accept.status()} ${await accept.text()}`)
  const { member } = (await accept.json()) as { member: { id: number } }
  return {
    page: await ctx.newPage(),
    name,
    leave: async () => {
      await ctx.close()
      await owner.request.delete(`${WEB}/api/members/${member.id}`)
    },
  }
}

/** Sends text with Enter in the open room and waits for the CEO's next answer. */
async function answered(page: Page, text: string) {
  const answers = '[data-testid="chat-message"][data-from="agent"]'
  const mine = '[data-testid="chat-message"][data-from="member"]'
  const before = await page.locator(answers).count()
  const sent = await page.locator(mine).count()
  const box = page.getByRole('textbox', { name: 'Message the Conference Room' })
  await box.fill(text)
  await box.press('Enter')
  await page.waitForFunction(([sel, n]) => document.querySelectorAll(sel as string).length > (n as number), [mine, sent] as const)
  await page.waitForFunction(([sel, n]) => document.querySelectorAll(sel as string).length > (n as number), [answers, before] as const, { timeout: 90_000 })
  await page.getByTestId('board-status').waitFor({ state: 'detached', timeout: 15_000 })
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
      const { issue } = (await (await page.request.get(`${WEB}/api/board-chat`)).json()) as { issue: { conversation: { state: string } } }
      expect('the Board chat is waiting', issue.conversation.state === 'waiting', issue.conversation)
      const [run] = await runsOf(page, ceo)
      expect('the Run woke the CEO with board_message', run?.wake_reason === 'board_message', run)
      await shot(page, 'reply')
    } finally {
      await desktop.stop()
      await page.context().close()
    }
  },

  async ['two-members']() {
    const page = await signedIn()
    const desktop = await desktopRunner(page, { BAKERY_STANDIN_DELAY: '200ms' })
    const second = await invited(page)
    try {
      const ceo = await theCEO(page)
      const { member: me } = (await (await page.request.get(`${WEB}/api/me`)).json()) as { member: { name: string } }
      await freshRoom(page)
      const question = `Who do we hire first? ${Date.now()}`
      await answered(page, question)

      const other = second.page
      await other.goto(`${WEB}/#/board-chat`)
      const theirs = other.locator('[data-testid="chat-message"][data-from="member"]').filter({ hasText: question })
      await theirs.waitFor()
      expect("the second Member sees the first one's message with their name", ((await theirs.getByTestId('chat-author').textContent()) ?? '').trim() === me.name, await theirs.textContent())
      expect('and the answer to it', (await other.locator('[data-testid="chat-message"][data-from="agent"]').count()) > 0)
      await answered(other, 'And after that? [prompt]')
      const [run] = await runsOf(page, ceo)
      const prompt = await promptOf(page, run.id)
      expect('the follow-up woke the CEO with board_message', run.wake_reason === 'board_message', run)
      expect("the CEO's prompt holds the first Member's turn", prompt.includes(`<turn author="${me.name}" role="board">\n${question}`), prompt)
      expect("and the second Member's", prompt.includes(`<turn author="${second.name}" role="board">\nAnd after that? [prompt]`))
      expect('and its own earlier answer as role="you"', prompt.includes(`<turn author="${ceo.name}" role="you">`))
      await shot(other, 'two-members')
    } finally {
      await second.leave()
      await desktop.stop()
      await page.context().close()
    }
  },

  async ['new-chat']() {
    const page = await signedIn()
    const desktop = await desktopRunner(page, { BAKERY_STANDIN_DELAY: '200ms' })
    try {
      const ceo = await theCEO(page)
      await freshRoom(page)
      await answered(page, 'Before the break')
      const runs = (await runsOf(page, ceo)).length
      const dividers = await page.getByTestId('chat-new-session').count()
      await page.getByRole('button', { name: 'new chat' }).click()
      await page.waitForFunction((n) => document.querySelectorAll('[data-testid="chat-new-session"]').length > n, dividers)
      expect('"New chat" shows the New session divider', true)
      await page.getByTestId('board-chips').waitFor()
      expect('the chips are offered again', true)
      expect('/new wakes nobody', (await runsOf(page, ceo)).length === runs)
      await answered(page, 'After the break [prompt]')
      const [run] = await runsOf(page, ceo)
      const prompt = await promptOf(page, run.id)
      expect('the next prompt holds the new message', prompt.includes('After the break'))
      expect('and not the turns before the divider', !prompt.includes('Before the break'), prompt)
    } finally {
      await desktop.stop()
      await page.context().close()
    }
  },

  async ['no-ceo']() {
    // A scratch Guild of its own, so the dev Guild keeps its CEO.
    const page = await signedIn()
    const made = await page.request.post(`${WEB}/api/guilds`, { data: { name: `no-ceo-${Date.now()}`, description: 'made by the board e2e' } })
    if (!made.ok()) throw new Error(`create a Guild: ${made.status()} ${await made.text()}`)
    const { guild } = (await made.json()) as { guild: { id: number } }
    await page.request.post(`${WEB}/api/guilds/${guild.id}/switch`)
    try {
      await page.goto(`${WEB}/#/board-chat`)
      const none = page.getByTestId('board-no-ceo')
      await none.waitFor()
      expect('the page says the guild has no CEO', ((await none.textContent()) ?? '').includes('This guild has no CEO yet'))
      expect('the composer says nobody will answer', (await page.getByTestId('board-no-ceo-note').count()) === 1)
      await none.getByRole('button', { name: 'Hire a CEO' }).click()
      const dialog = page.getByRole('dialog')
      await dialog.waitFor()
      expect('"Hire a CEO" opens the hire dialog', ((await dialog.textContent()) ?? '').includes('Hire agent'))
      expect('with Job CEO', (await dialog.getByLabel('Job').inputValue()) === 'ceo', await dialog.getByLabel('Job').inputValue())
      await shot(page, 'no-ceo')
    } finally {
      const gone = await page.request.delete(`${WEB}/api/guilds/current`)
      if (!gone.ok()) console.log(`delete the scratch Guild: ${gone.status()} ${await gone.text()}`)
      await page.context().close()
    }
  },

  async feed() {
    const page = await signedIn()
    try {
      await page.goto(`${WEB}/#/board-chat`)
      const feed = page.getByTestId('activity-feed')
      await feed.locator('[data-feed-card]').first().waitFor()
      expect('the feed shows the Guild\'s recent events', (await feed.locator('[data-feed-card]').count()) > 0)

      const title = `Feed check ${Date.now()}`
      const made = await page.request.post(`${WEB}/api/issues`, { data: { title } })
      if (!made.ok()) throw new Error(`create an Issue: ${made.status()} ${await made.text()}`)
      const { issue } = (await made.json()) as { issue: { id: number; identifier: string } }
      const created = feed.locator('[data-feed-card="issue.created"]', { hasText: title })
      await created.waitFor({ timeout: 15_000 })
      expect("a new Issue's card lands within 10 s", (await created.getAttribute('data-tier')) === '1')

      const say = await page.request.post(`${WEB}/api/issues/${issue.id}/comments`, { data: { body: 'to be deleted' } })
      const { comment } = (await say.json()) as { comment: { id: number } }
      await page.request.delete(`${WEB}/api/issues/${issue.id}/comments/${comment.id}`)
      const moved = await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { status: 'in_review' } })
      if (!moved.ok()) throw new Error(`move to in_review: ${moved.status()} ${await moved.text()}`)
      const review = feed.locator('[data-feed-card="issue.updated"]', { hasText: title })
      await review.waitFor({ timeout: 15_000 })
      expect('moving to In Review is a card', (await review.getAttribute('data-tier')) === '1' && ((await review.textContent()) ?? '').includes('moved to in review'), await review.textContent())

      await feed.getByRole('button', { name: 'filter by' }).click()
      await page.getByRole('menuitemradio', { name: 'In Review' }).click()
      await page.keyboard.press('Escape')
      await created.waitFor({ state: 'detached' })
      const kinds = await feed.locator('[data-feed-card]').evaluateAll((els) => els.map((el) => el.getAttribute('data-feed-card')))
      const reviewKinds = ['issue.updated', 'approval.created', 'issue.document_created', 'issue.document_updated', 'issue.pull_request_opened']
      expect('"In Review" keeps only review events', (await review.count()) === 1 && kinds.every((k) => reviewKinds.includes(k ?? '')), kinds)

      const hidden = feed.locator('[data-feed-card="issue.comment_deleted"]', { hasText: title })
      await feed.getByRole('button', { name: 'filter by' }).click()
      await page.getByRole('menuitemradio', { name: 'All' }).click()
      await page.keyboard.press('Escape')
      await created.waitFor()
      expect('a deleted Comment is hidden by default', (await hidden.count()) === 0)
      await feed.getByRole('button', { name: 'filter by' }).click()
      await page.getByRole('menuitemcheckbox', { name: 'Show all activity' }).click()
      await page.keyboard.press('Escape')
      await hidden.waitFor({ timeout: 5_000 })
      expect('"Show all activity" brings it back', (await hidden.getAttribute('data-tier')) === '3')

      await feed.getByRole('button', { name: 'group by issue' }).click()
      const group = feed.locator(`[data-feed-group="issue:${issue.id}"]`)
      await group.waitFor()
      expect('group by Issue puts its events under its header', ((await group.textContent()) ?? '').includes(`${issue.identifier} — ${title}`) && (await group.locator('[data-feed-card]').count()) >= 3)
      await shot(page, 'feed')
      await group.getByRole('button', { expanded: true }).first().click()
      expect('its header folds them', (await group.locator('[data-feed-card]').count()) === 0)

      const pane = page.getByTestId('board-chat-pane')
      const before = (await pane.boundingBox())!.width
      const feedBefore = (await feed.boundingBox())!.width
      const divider = (await page.getByTestId('board-divider').boundingBox())!
      await page.mouse.move(divider.x + divider.width / 2, divider.y + 200)
      await page.mouse.down()
      await page.mouse.move(divider.x + divider.width / 2 - 200, divider.y + 200, { steps: 8 })
      await page.mouse.up()
      const after = (await pane.boundingBox())!.width
      const feedWidth = (await feed.boundingBox())!.width
      expect('dragging the divider resizes both panes', Math.abs(before - 200 - after) <= 2 && Math.abs(feedBefore + 200 - feedWidth) <= 2, { before, after, feedBefore, feedWidth })
      await page.reload()
      await pane.waitFor()
      const reloaded = (await pane.boundingBox())!.width
      expect('the width survives a reload', Math.abs(reloaded - after) <= 2, { after, reloaded })
      await page.evaluate(() => Object.keys(localStorage).filter((k) => k.startsWith('bakery.boardChatSplit.')).forEach((k) => localStorage.removeItem(k)))
    } finally {
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
