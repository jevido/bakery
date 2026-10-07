// The guild's settings pages (Notifications, Keys & Tokens, Profile and the
// instance Settings) in headless Chromium, one section each, every flow once
// in the dark theme at 1440×900 (the light theme and phone width wait for the
// guilds goal's final sweep):
//   notifications  every Channel kind opens from the nav and shows its form;
//                  Discord: an empty save shows the validation, a webhook
//                  URL saves, Enable and Disable toggle and toast, Send test
//                  answers with a toast; the Discord channel is deleted again
//
//   bun e2e/settings.ts [section ...]   (task web:settings; needs task dev)
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

type Channel = { id: number; name: string; kind: string; enabled: boolean }
const browser = await chromium.launch({ executablePath: CHROMIUM })
const who = owner()

async function signedIn(width = 1440, height = 900, theme: 'dark' | 'light' = 'dark'): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  await ctx.addInitScript((t) => localStorage.setItem('theme', t), theme)
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

async function channels(page: Page): Promise<Channel[]> {
  return ((await (await page.request.get(`${WEB}/api/notification-channels`)).json()) as { channels: Channel[] }).channels
}

/** The newest toast's text, once one shows. */
async function toastText(page: Page): Promise<string> {
  const toast = page.locator('ul[aria-live="polite"] li').first()
  await toast.waitFor({ timeout: 15000 })
  return (await toast.innerText()).trim()
}

/** Dismisses every toast shown, so the next toastText reads a new one. */
async function clearToasts(page: Page) {
  const shown = page.locator('ul[aria-live="polite"] li')
  while ((await shown.count()) > 0) {
    await shown.first().getByRole('button', { name: 'Dismiss' }).click().catch(() => {})
    await page.waitForTimeout(50)
  }
}

const sections: Record<string, () => Promise<void>> = {
  // Every kind from the nav; Discord's validation, save, Enable/Disable and
  // Send test; the channel is deleted again at the end.
  async notifications() {
    const page = await signedIn()
    const before = (await channels(page)).filter((c) => c.kind === 'discord')
    expect('no Discord channel left behind before the run', before.length === 0, before)
    for (const c of before) await page.request.delete(`${WEB}/api/notification-channels/${c.id}`)

    await page.goto(`${WEB}/#/notifications/email`)
    await page.locator('#main-content').getByRole('heading', { name: 'Notifications', level: 1 }).waitFor()
    const items = page.locator('nav[aria-label="Notification settings"] [data-testid="notification-menu-item"]')
    expect('the nav lists the seven Channel kinds', (await items.count()) === 7, await items.count())
    for (const [kind, label] of [
      ['email', 'Email delivery'],
      ['discord', 'Discord'],
      ['telegram', 'Telegram'],
      ['slack', 'Slack'],
      ['pushover', 'Pushover'],
      ['webhook', 'Webhook'],
      ['ntfy', 'ntfy'],
    ]) {
      const name = label.split(' ')[0]
      await page.locator('nav[aria-label="Notification settings"]').getByRole('link', { name, exact: true }).click()
      const group = page.locator(`#${kind}-settings`)
      await group.waitFor({ timeout: 5000 }).catch(() => {})
      expect(`${kind} opens its form`, (await group.count()) === 1 && (await page.url()).endsWith(`/notifications/${kind}`), page.url())
      const active = await page.locator('nav[aria-label="Notification settings"] a[aria-current="page"]').innerText()
      expect(`${kind} is marked in the nav`, active.trim() === name, active)
    }

    await page.goto(`${WEB}/#/notifications/discord`)
    const form = page.locator('#discord-settings')
    await form.waitFor()
    // An empty Enable asks the form whether it is valid first, as Coolify's
    // reportValidity() does: nothing is stored.
    await page.getByTestId('channel-toggle').click()
    const invalid = await page.getByTestId('webhook-url').evaluate((el) => !(el as HTMLInputElement).validity.valid)
    expect('an empty Enable shows the webhook URL is required', invalid)
    expect('an empty Enable stores nothing', (await channels(page)).filter((c) => c.kind === 'discord').length === 0)

    await page.getByTestId('webhook-url').fill('http://127.0.0.1:9/discord-stand-in')
    await clearToasts(page)
    await page.getByRole('button', { name: 'Save' }).click()
    expect('saving shows a toast', /saved/i.test(await toastText(page)))
    const saved = (await channels(page)).find((c) => c.kind === 'discord')
    expect('the Discord channel is stored disabled', !!saved && !saved.enabled, saved)

    await clearToasts(page)
    await page.getByTestId('channel-toggle').click()
    expect('Enable toasts', /saved/i.test(await toastText(page)))
    await page.getByTestId('channel-toggle').filter({ hasText: 'Disable' }).waitFor()
    expect('Enable stores it enabled', (await channels(page)).find((c) => c.kind === 'discord')?.enabled === true)

    await clearToasts(page)
    await page.getByTestId('channel-test').click()
    const tested = await toastText(page)
    expect('Send test answers with a toast', tested.length > 0, tested)

    await clearToasts(page)
    await page.getByTestId('channel-toggle').click()
    expect('Disable toasts', /saved/i.test(await toastText(page)))
    await page.getByTestId('channel-toggle').filter({ hasText: 'Enable' }).waitFor()
    expect('Disable stores it disabled', (await channels(page)).find((c) => c.kind === 'discord')?.enabled === false)

    // Delete it again, through the page as a person does.
    await page.getByRole('button', { name: 'Delete' }).click()
    await page.getByLabel('Channel name').fill(saved?.name ?? 'Discord')
    await page.getByRole('dialog').getByRole('button', { name: 'Confirm' }).click()
    await page.waitForTimeout(500)
    const after = (await channels(page)).filter((c) => c.kind === 'discord')
    expect('the Discord channel is deleted again', after.length === 0, after)
    for (const c of after) await page.request.delete(`${WEB}/api/notification-channels/${c.id}`)
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
console.log('the settings flows work')
