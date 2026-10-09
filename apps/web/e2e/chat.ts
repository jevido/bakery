// The Chat pages in the dashboard, one section each, every flow once in the
// dark theme at 1440×900:
//   start   #/chats asks "Who would you like to talk to?"; picking an Agent
//           hired for the run lands on #/chats/{id} with the empty thread,
//           which opened no Conversation; "hello" sent with Enter shows on
//           the right, the Conversation is Active and its Live run appears
//
//   bun e2e/chat.ts [section ...]   (task web:chat; needs task dev)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { readFileSync } from 'node:fs'
import { chromium, type Page } from 'playwright-core'
import { desktopRunner } from './runner.ts'

const WEB = (process.env.BAKERY_WEB ?? 'http://127.0.0.1:4930').replace(/\/$/, '')
const CHROMIUM = process.env.CHROMIUM ?? '/usr/bin/chromium'

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

type Agent = { id: number; name: string; status: string }

/** Hires an Agent through the API and approves its hire, as e2e/agents.ts does. */
async function hired(page: Page, name: string): Promise<Agent> {
  const r = await page.request.post(`${WEB}/api/agents`, {
    data: {
      name,
      job: 'general',
      title: 'Chat partner',
      icon: 'bot',
      capabilities: '',
      role_ids: [],
      reports_to: null,
    },
  })
  if (!r.ok()) throw new Error(`hire ${name}: ${r.status()} ${await r.text()}`)
  const { agent, approval_id } = (await r.json()) as {
    agent: Agent
    approval_id: number
  }
  const ok = await page.request.post(`${WEB}/api/approvals/${approval_id}/approve`, { data: {} })
  if (!ok.ok()) throw new Error(`approve ${name}: ${ok.status()} ${await ok.text()}`)
  return agent
}

/** Terminates the scratch Agent, which stays as a terminated record. */
async function terminate(page: Page, agent: Agent) {
  const r = await page.request.post(`${WEB}/api/agents/${agent.id}/terminate`)
  if (!r.ok()) throw new Error(`terminate ${agent.name}: ${r.status()} ${await r.text()}`)
}

type Run = { id: number; status: string }

/** The Agent's Runs, newest first. */
async function runsOf(page: Page, agent: Agent): Promise<Run[]> {
  const { runs } = (await (await page.request.get(`${WEB}/api/runs?agent=${agent.id}`)).json()) as { runs: Run[] }
  return runs.sort((a, b) => b.id - a.id)
}

/**
 * Sends text with Enter and waits for the Agent's reply that holds answer,
 * answering the Run that wrote it (the Live run's, read while it shows).
 */
async function converse(page: Page, agent: Agent, text: string, answer: string): Promise<number> {
  const box = page.getByRole('textbox', { name: `Message ${agent.name}` })
  await box.fill(text)
  await box.press('Enter')
  const live = page.getByTestId('live-run')
  await live.waitFor({ timeout: 15_000 })
  const run = Number(await live.getAttribute('data-run'))
  await page.locator('[data-testid="chat-message"][data-from="agent"]').filter({ hasText: answer }).last().waitFor({ timeout: 90_000 })
  return run
}

/** What the claude stand-in said the Run's prompt was ([prompt]). */
async function promptOf(page: Page, run: number): Promise<string> {
  return await (await page.request.get(`${WEB}/api/runs/${run}/events`)).text()
}

const sections: Record<string, () => Promise<void>> = {
  async start() {
    const page = await signedIn()
    // Scratch Agents stay as terminated records, so each run names its own.
    const agent = await hired(page, `Chatter ${Date.now()}`)
    try {
      await page.goto(`${WEB}/#/chats`)
      await page.getByRole('heading', { name: 'Who would you like to talk to?' }).waitFor()
      expect('#/chats asks who to talk to', true)
      const pick = page.locator(`[data-testid="chat-agents"] [data-agent="${agent.id}"]`)
      if ((await pick.count()) === 0) {
        // Only six are offered; the scratch Agent may be past them.
        await page.goto(`${WEB}/#/chats/${agent.id}`)
      } else {
        await pick.click()
      }
      await page.waitForURL(`**/#/chats/${agent.id}`)
      expect('picking the Agent opens its chat', true)
      await page.getByTestId('chat-empty').waitFor()
      expect('the empty thread says hello', (await page.getByTestId('chat-empty').textContent())?.includes(`Say hello to ${agent.name}.`) ?? false)
      const before = (await (await page.request.get(`${WEB}/api/chats/${agent.id}`)).json()) as { issue: unknown }
      expect('visiting opened no Conversation', before.issue === null, before)

      const box = page.getByRole('textbox', { name: `Message ${agent.name}` })
      await box.fill('hello')
      await box.press('Enter')
      const mine = page.locator('[data-testid="chat-message"][data-from="member"]').filter({ hasText: 'hello' })
      await mine.waitFor()
      expect('the message is right-aligned', (await mine.getAttribute('class'))?.includes('justify-end') ?? false)
      await page.getByTestId('live-run').waitFor({ timeout: 15_000 })
      expect('the Live run appears', true)
      const after = (await (await page.request.get(`${WEB}/api/chats/${agent.id}`)).json()) as { issue: { conversation: { state: string } } | null }
      expect('the first message opened the Conversation, Active', after.issue?.conversation.state === 'active', after)
    } finally {
      await terminate(page, agent)
      await page.context().close()
    }
  },

  async reply() {
    const page = await signedIn()
    const agent = await hired(page, `Replier ${Date.now()}`)
    let desktop: Awaited<ReturnType<typeof desktopRunner>> | undefined
    try {
      await page.goto(`${WEB}/#/chats/${agent.id}`)
      await page.getByTestId('chat-empty').waitFor()
      const box = page.getByRole('textbox', { name: `Message ${agent.name}` })
      await box.fill('What changed?')
      await box.press('Enter')
      await page.getByTestId('run-waiting').waitFor({ timeout: 15_000 })
      expect('the Live run is queued until a Desktop claims it', true)

      desktop = await desktopRunner(page)
      await page.getByTestId('run-waiting').waitFor({ state: 'detached', timeout: 90_000 })
      expect('the Desktop claims it', true)
      await page.getByTestId('live-run').waitFor({ state: 'detached', timeout: 90_000 })
      expect('the Live run goes when it ends', true)
      const reply = page.locator('[data-testid="chat-message"][data-from="agent"]').filter({ hasText: 'Done with' })
      await reply.waitFor()
      expect('the reply is left-aligned', (await reply.getAttribute('class'))?.includes('justify-start') ?? false)
      expect("the reply carries the Agent's name", (await reply.innerText()).includes(agent.name), await reply.innerText())
      const [first] = await runsOf(page, agent)
      expect('the Run succeeded', first?.status === 'succeeded', first)
      const state = async () => ((await (await page.request.get(`${WEB}/api/chats/${agent.id}`)).json()) as { issue: { conversation: { state: string } } }).issue.conversation.state
      expect('the Conversation is Waiting', (await state()) === 'waiting', await state())

      const second = await converse(page, agent, 'And since then? [prompt]', 'Read the prompt.')
      expect('a second message gets a second reply', (await page.locator('[data-testid="chat-message"][data-from="agent"]').count()) === 2)
      expect('a second Run answered it', second !== first.id, second)
      expect("the second Run's prompt holds the first message", (await promptOf(page, second)).includes('What changed?'))
    } finally {
      await desktop?.stop()
      await terminate(page, agent)
      await page.context().close()
    }
  },

  async ['new-session']() {
    const page = await signedIn()
    const agent = await hired(page, `Sessions ${Date.now()}`)
    const desktop = await desktopRunner(page)
    try {
      await page.goto(`${WEB}/#/chats/${agent.id}`)
      await page.getByTestId('chat-empty').waitFor()
      await converse(page, agent, 'Before the break', 'Done with')
      const runs = (await runsOf(page, agent)).length

      const box = page.getByRole('textbox', { name: `Message ${agent.name}` })
      await box.fill('/new')
      await box.press('Enter')
      await page.getByTestId('chat-new-session').waitFor()
      expect('/new shows the New session divider', true)
      await page.waitForTimeout(1_500)
      expect('/new queues no Run', (await runsOf(page, agent)).length === runs, await runsOf(page, agent))

      const run = await converse(page, agent, 'After the break [prompt]', 'Read the prompt.')
      const prompt = await promptOf(page, run)
      expect('the next Run sees the new message', prompt.includes('After the break'))
      expect('it does not see what came before the divider', !prompt.includes('Before the break'))
    } finally {
      await desktop.stop()
      await terminate(page, agent)
      await page.context().close()
    }
  },

  async sidebar() {
    const page = await signedIn()
    // Named to sort first among starred Agents, which go by name.
    const first = await hired(page, `AAA Sidebar ${Date.now()}`)
    const second = await hired(page, `Picked ${Date.now()}`)
    try {
      await page.goto(`${WEB}/#/chats/${first.id}`)
      await page.getByTestId('chat-empty').waitFor()
      const row = page.locator(`[data-testid="sidebar-chats"] [data-chat-agent="${first.id}"]`)
      await row.waitFor()
      expect('the chatted Agent is under Chats', true)
      expect('its row is active on its chat', (await row.getByRole('link').getAttribute('aria-current')) === 'page')

      const star = row.getByRole('button', { name: `Star ${first.name}` })
      await star.click()
      const unstar = row.getByRole('button', { name: `Unstar ${first.name}` })
      await unstar.waitFor()
      expect('starring presses the star', (await unstar.getAttribute('aria-pressed')) === 'true')
      await page.reload()
      await row.waitFor()
      const ids = await page.locator('[data-testid="sidebar-chats"] [data-chat-agent]').evaluateAll((els) => els.map((e) => Number(e.getAttribute('data-chat-agent'))))
      expect('the starred Agent stays first after a reload', ids[0] === first.id, ids)
      expect('it stays starred', (await row.getByRole('button', { name: `Unstar ${first.name}` }).getAttribute('aria-pressed')) === 'true')

      await page.getByRole('button', { name: 'Chat with an agent' }).click()
      const search = page.getByRole('textbox', { name: 'Search agents by name or role' })
      await search.fill(second.name)
      await page.locator(`[data-testid="chat-picker"] [data-agent="${second.id}"]`).waitFor()
      await search.press('Enter')
      await page.waitForURL(`**/#/chats/${second.id}`)
      expect('the picker opens a chat with the second Agent', true)
      await page.locator(`[data-testid="sidebar-chats"] [data-chat-agent="${second.id}"]`).waitFor()
      expect('the second Agent joins Chats', true)

      await page.goto(`${WEB}/#/agents/${first.id}`)
      await page.locator('main').getByRole('link', { name: 'Chat', exact: true }).click()
      await page.waitForURL(`**/#/chats/${first.id}`)
      expect("the Agent page's Chat button lands on its chat", true)
    } finally {
      await terminate(page, first)
      await terminate(page, second)
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
console.log('the chat flows work')
