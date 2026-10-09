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
