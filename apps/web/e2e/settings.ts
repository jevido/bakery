// The guild's settings pages (Notifications, Keys & Tokens, Profile and the
// instance Settings) in headless Chromium, one section each, every flow once
// in the dark theme at 1440×900 (the light theme and phone width wait for the
// guilds goal's final sweep):
//   notifications  every Channel kind opens from the nav and shows its form;
//                  Discord: an empty save shows the validation, a webhook
//                  URL saves, Enable and Disable toggle and toast, Send test
//                  answers with a toast and adds a Delivery row, an event
//                  toggles and stays toggled after a reload; the Discord
//                  channel is deleted again; the Email kind's Send Test
//                  Email dialog opens
//
//   tokens         create a token with Read and Deploy, expiring in 7 days;
//                  its value shows once with Copy, survives no reload, is
//                  found and cleared by search, authenticates GET /api/me by
//                  Bearer with no cookie, and stops doing so once revoked
//
//   profile        the header card shows the name; a rename shows in it and
//                  is renamed back; a wrong current password shows its error
//                  under the field; Set up two-factor shows the QR code and
//                  key and is left pending (never on), so the owner's
//                  password and two-factor stay as the other scripts expect
//
//   instance       Settings (Known hosts) shows its rows or the empty state;
//                  Forget opens the confirmation and cancels
//
//   viewer         a Viewer sees no Notifications and no Settings in the
//                  settings sidebar; #/settings and #/notifications/email
//                  show the permission Empty; Keys & Tokens and Profile
//                  still open
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

// A Viewer: the owner with manage_servers, manage_notifications and
// administrator taken out of what /api/me says (as e2e/servers.ts makes one).
async function viewer(): Promise<Page> {
  const page = await signedIn()
  const strip = ['manage_servers', 'manage_notifications', 'administrator']
  await page.route('**/api/me', async (route) => {
    const r = await route.fetch()
    const body = await r.json()
    body.permissions = body.permissions.filter((p: string) => !strip.includes(p))
    for (const g of body.guilds ?? []) g.permissions = (g.permissions ?? []).filter((p: string) => !strip.includes(p))
    await route.fulfill({ response: r, json: body })
  })
  return page
}

type ApiToken = { id: number; name: string }
async function apiTokens(page: Page): Promise<ApiToken[]> {
  return ((await (await page.request.get(`${WEB}/api/api-tokens`)).json()) as { api_tokens: ApiToken[] }).api_tokens
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
    const deliveries = page.getByTestId('delivery')
    await deliveries.first().waitFor({ timeout: 5000 }).catch(() => {})
    expect('a Delivery row appears after Send test', (await deliveries.count()) > 0, await deliveries.count())

    await clearToasts(page)
    await page.locator('#discord-deployments-events-trigger').click()
    const checkbox = page.getByTestId('event-option').first().locator('[data-slot="checkbox"]')
    const checkedBefore = await checkbox.getAttribute('data-state')
    await page.getByTestId('event-option').first().click()
    expect('toggling an event toasts', /saved/i.test(await toastText(page)))
    const checkedAfter = await checkbox.getAttribute('data-state')
    expect('the event toggle flips the checkbox', checkedAfter !== checkedBefore, { checkedBefore, checkedAfter })
    await page.reload()
    await page.locator('#discord-deployments-events-trigger').click()
    const checkedAfterReload = await page.getByTestId('event-option').first().locator('[data-slot="checkbox"]').getAttribute('data-state')
    expect('the checkbox state survives a reload', checkedAfterReload === checkedAfter, { checkedAfter, checkedAfterReload })

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

    // Email's Send test asks for a recipient first, starting at the signed-in
    // owner's address; Send test is disabled until the channel is enabled.
    const strayEmail = (await channels(page)).filter((c) => c.kind === 'email')
    for (const c of strayEmail) await page.request.delete(`${WEB}/api/notification-channels/${c.id}`)
    await page.goto(`${WEB}/#/notifications/email`)
    await page.locator('#email-settings').waitFor()
    await page.getByTestId('email-from-name').fill('The Bakery')
    await page.getByTestId('email-from').fill('bakery@example.com')
    await page.getByTestId('email-to').fill('ops@example.com')
    await page.getByTestId('email-host').fill('127.0.0.1')
    await page.getByTestId('email-port').fill('4980')
    await page.getByTestId('email-encryption').selectOption('none')
    await clearToasts(page)
    await page.getByTestId('channel-toggle').click()
    expect('enabling email toasts', /updated|saved/i.test(await toastText(page)))
    await page.getByTestId('channel-toggle').filter({ hasText: 'Disable' }).waitFor()

    await page.getByTestId('channel-test').click()
    const dialog = page.getByRole('dialog', { name: 'Send Test Email' })
    await dialog.waitFor()
    const recipient = await dialog.getByTestId('test-recipient').inputValue()
    expect('Send test opens with the owner email as recipient', recipient === who.email, recipient)
    await dialog.getByRole('button', { name: 'Close' }).click()
    await dialog.waitFor({ state: 'hidden' })

    const email = (await channels(page)).find((c) => c.kind === 'email')
    if (email) await page.request.delete(`${WEB}/api/notification-channels/${email.id}`)

    await page.close()
  },

  async tokens() {
    const page = await signedIn()
    const name = 'E2E token'
    for (const t of (await apiTokens(page)).filter((t) => t.name === name)) await page.request.delete(`${WEB}/api/api-tokens/${t.id}`)

    await page.goto(`${WEB}/#/security/api-tokens`)
    await page.locator('#main-content').getByRole('heading', { name: 'Keys & Tokens', level: 1 }).waitFor()

    await page.getByTestId('token-description').fill(name)
    await page.getByTestId('token-expires').selectOption('7')

    // Clicking Deploy takes it alone, as Coolify's updatedPermissions does;
    // Read is clicked again after, to land on both.
    await page.getByTestId('permissions-trigger').click()
    await page.getByTestId('permission-deploy').click()
    await page.getByTestId('permission-read').click()
    await page.keyboard.press('Escape')

    await page.getByTestId('create-token').click()
    await page.getByTestId('new-token').waitFor()
    const token = await page.getByTestId('new-token-value').inputValue()
    expect('the new token value is shown once', token.length > 0, token)

    const row = page.getByTestId('api-token').filter({ hasText: name })
    await row.waitFor()
    const perms = (await row.getByTestId('token-permission').allInnerTexts()).sort()
    expect('the token has Read and Deploy', perms.join(',') === 'deploy,read', perms)

    const meBefore = await fetch(`${WEB}/api/me`, { headers: { Authorization: `Bearer ${token}` } })
    expect('the token authenticates GET /api/me with no cookie', meBefore.status === 200, meBefore.status)

    await page.reload()
    expect('the token value is gone from the page after a reload', (await page.getByTestId('new-token').count()) === 0)
    await page.getByTestId('api-token').filter({ hasText: name }).waitFor()

    await page.getByLabel('Search tokens').fill(name)
    expect('search finds the token', (await page.getByTestId('api-token').count()) === 1)
    await page.getByLabel('Search tokens').fill('no-such-token-zzz')
    expect('search filters out what does not match', (await page.getByTestId('api-token').count()) === 0)
    await page.getByLabel('Clear search').click()
    expect('clearing the search shows the token again', (await page.getByTestId('api-token').count()) >= 1)

    await page.getByTestId('api-token').filter({ hasText: name }).getByTestId('revoke-token').click()
    await page.getByLabel('Token description').fill(name)
    await page.getByRole('dialog').getByRole('button', { name: 'Revoke token' }).click()
    await page.waitForTimeout(500)
    expect('Revoke removes the token', (await apiTokens(page)).filter((t) => t.name === name).length === 0)

    const meAfter = await fetch(`${WEB}/api/me`, { headers: { Authorization: `Bearer ${token}` } })
    expect('the revoked token no longer authenticates', meAfter.status === 401, meAfter.status)

    await page.close()
  },

  async profile() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/profile`)
    const card = page.getByTestId('profile-card')
    await card.waitFor()
    const original = (await (await page.request.get(`${WEB}/api/me`)).json()).member.name as string
    expect('the header card shows the name', (await page.getByTestId('profile-name').innerText()).trim() === original.trim())
    expect('the header card shows the email', (await card.innerText()).includes(who.email))

    const renamed = `${original} e2e`
    await page.locator('#name').fill(renamed)
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    expect('Save confirms with a toast', (await toastText(page)).includes('Saved'))
    await page.getByTestId('profile-name').filter({ hasText: renamed }).waitFor({ timeout: 5000 }).catch(() => {})
    expect('the header card shows the new name', (await page.getByTestId('profile-name').innerText()).trim() === renamed)
    await clearToasts(page)
    await page.locator('#name').fill(original)
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    await toastText(page)
    expect('the name is back', (await page.getByTestId('profile-name').innerText()).trim() === original.trim())

    await page.locator('#current_password').fill('not-the-password-e2e')
    await page.locator('#new_password').fill('a-new-password-e2e')
    await page.locator('#repeat_password').fill('a-new-password-e2e')
    await page.getByRole('button', { name: 'Change password' }).click()
    // FieldError is the <p> right after the field's input row.
    const fieldError = page.locator('#current_password').locator('xpath=ancestor::div[contains(@class,"chrome")][1]').locator('p')
    await fieldError.first().waitFor({ timeout: 5000 }).catch(() => {})
    const errorText = (await fieldError.allInnerTexts()).join(' ')
    expect('a wrong current password shows its error under the field', errorText.includes('the password is wrong'), errorText)
    const login = await fetch(`${WEB}/api/login`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(who) })
    expect('the owner still signs in with the old password', login.status === 200, login.status)

    const twoFactor = page.getByTestId('two-factor')
    const setUp = twoFactor.getByRole('button', { name: 'Set up two-factor' })
    if ((await setUp.count()) === 0) {
      expect('two-factor is off for the owner, so it can be set up', false, await twoFactor.innerText())
    } else {
      await setUp.click()
      await page.getByTestId('two-factor-secret').waitFor()
      expect('setup shows the QR code', (await twoFactor.getByRole('img', { name: /QR code/ }).locator('svg').count()) === 1)
      expect('setup shows the key in groups of four', /^([A-Z2-7]{1,4} )+[A-Z2-7]{1,4}$/.test((await page.getByTestId('two-factor-secret').innerText()).trim()))
      await twoFactor.getByRole('button', { name: 'Cancel' }).click()
      const status = (await (await page.request.get(`${WEB}/api/me/two-factor`)).json()) as { state: string }
      expect('two-factor stays off until it is confirmed', status.state !== 'on', status.state)
    }

    await page.close()
  },

  // Settings (Known hosts): its rows or the empty state; Forget opens the
  // confirmation and cancels.
  async instance() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/settings`)
    await page.locator('#main-content').getByRole('heading', { name: 'Settings', level: 1 }).waitFor()
    await page.waitForFunction(() => document.querySelector('[data-testid="known-host"], [data-testid="known-hosts-empty"]'), null, { timeout: 5000 })
    const rows = page.getByTestId('known-host')
    const count = await rows.count()
    if (count === 0) {
      expect('Settings shows the empty state', (await page.getByTestId('known-hosts-empty').count()) === 1)
    } else {
      await rows.first().getByTestId('forget-known-host').click()
      const dialog = page.getByRole('dialog', { name: 'Forget this host key?' })
      await dialog.waitFor()
      expect('Forget opens a confirmation', await dialog.isVisible())
      await dialog.getByRole('button', { name: 'Cancel' }).click()
      await page.waitForTimeout(200)
      expect('Cancel closes the confirmation', (await dialog.count()) === 0)
    }
    await page.close()
  },

  // A Viewer sees no Notifications and no Settings in the settings sidebar;
  // #/settings and #/notifications/email show the permission Empty; Keys &
  // Tokens and Profile still open.
  async viewer() {
    const v = await viewer()
    await v.goto(`${WEB}/#/settings`)
    const sidebar = v.getByTestId('settings-sidebar')
    await sidebar.waitFor()
    expect('a Viewer sees no Notifications in the settings sidebar', (await sidebar.getByRole('link', { name: 'Notifications' }).count()) === 0)
    expect('a Viewer sees no Settings in the settings sidebar', (await sidebar.getByRole('link', { name: 'Settings', exact: true }).count()) === 0)
    expect('#/settings shows the permission Empty', (await v.getByText('Settings need the Manage servers permission').count()) === 1)

    await v.goto(`${WEB}/#/notifications/email`)
    expect(
      '#/notifications/email shows the permission Empty',
      (await v.getByText('Notifications need the Manage notifications permission').count()) === 1,
    )

    await v.goto(`${WEB}/#/security/api-tokens`)
    await v.locator('#main-content').getByRole('heading', { name: 'Keys & Tokens', level: 1 }).waitFor()
    expect('Keys & Tokens still opens for a Viewer', true)

    await v.goto(`${WEB}/#/profile`)
    await v.getByTestId('profile-card').waitFor()
    expect('Profile still opens for a Viewer', true)

    await v.close()
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
