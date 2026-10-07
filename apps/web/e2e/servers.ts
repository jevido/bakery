// The Servers list and New server in headless Chromium: the Local server
// shows with its Ready status, search narrows and clears, the table/grid
// switch persists across a reload, New server validates an empty submit,
// and a Viewer sees no "New server". A Server's frame, in dark and light on
// a desktop and a phone: the header with its name and status pill, the nav
// to every sub-page (the select on a phone), and the switcher to a second
// Server (the Remote server stand-in, when task remote:up runs). General
// renames the Local server and back and opens Validate with its checkpoints;
// Private Key and Danger open on the Remote server stand-in, where Danger's
// button stays disabled until the name is typed; a Viewer sees no save
// button and no Danger. Resources lists the Local server's resources (or its
// empty state) and search narrows them; Metrics shows the overview and draws
// a chart sample; Docker Cleanup's confirmation opens and cancels.
//
//   bun e2e/servers.ts            (task web:servers; needs task dev running)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { readFileSync } from 'node:fs'
import { connect } from 'node:net'
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

type Server = { id: number; name: string; kind: 'local' | 'remote' }
const browser = await chromium.launch({ executablePath: CHROMIUM })
const who = owner()

async function signedIn(width: number, height: number, theme: 'dark' | 'light' = 'light'): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  await ctx.addInitScript((t) => localStorage.setItem('theme', t), theme)
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

const probe = await signedIn(1440, 900)
const localServer = ((await (await probe.request.get(`${WEB}/api/servers`)).json()) as { servers: Server[] }).servers.find(
  (s) => s.kind === 'local',
)!

for (const theme of ['light', 'dark'] as const) {
  const page = await signedIn(1440, 900, theme)
  await page.goto(`${WEB}/#/servers`)
  await page.getByTestId('servers-count').waitFor()

  const rows = page.getByTestId('server')
  expect(`${theme}: the list has the Local server`, (await rows.filter({ hasText: localServer.name }).count()) === 1)
  expect(`${theme}: it is Ready`, (await rows.filter({ hasText: localServer.name }).getByTestId('server-status').textContent()) === 'Ready')
  const dark = await page.evaluate(() => document.documentElement.classList.contains('dark'))
  expect(`${theme}: the page is in the ${theme} theme`, dark === (theme === 'dark'), dark)
  await page.close()
}

// The Server frame on the Local server, in each theme and size.
const subPages = ['General', 'Resources', 'Docker Cleanup', 'Metrics']
for (const theme of ['light', 'dark'] as const) {
  for (const [width, height] of [
    [1440, 900],
    [390, 844],
  ] as const) {
    const at = `${theme} ${width}px`
    const page = await signedIn(width, height, theme)
    await page.goto(`${WEB}/#/server/${localServer.id}`)
    await page.getByTestId('server-subtitle').waitFor()
    expect(`${at}: the header shows the name`, (await page.getByTestId('server-subtitle').textContent())?.trim() === localServer.name)
    const pill = page.getByTestId('server-status-summary')
    expect(`${at}: the status pill says Ready`, ((await pill.textContent()) ?? '').includes('Ready'))
    await pill.click()
    expect(`${at}: the pill opens System status`, await page.getByText('System status').isVisible())
    await page.keyboard.press('Escape')
    const crumbs = await page.getByTestId('breadcrumb-bar').textContent()
    expect(`${at}: the breadcrumb is Servers › the name`, !!crumbs?.includes(localServer.name), crumbs)
    expect(`${at}: the top bar holds no server context`, (await page.locator('#server-topbar-context, #resource-action-hud-slot').count()) === 0)

    for (const label of subPages) {
      if (width >= 1280) {
        await page.getByRole('navigation', { name: 'Server configuration sections' }).getByRole('link', { name: label, exact: true }).click()
      } else {
        await page.getByRole('combobox', { name: 'Server configuration sections' }).selectOption({ label })
      }
      await page.waitForTimeout(300)
      const want = label === 'General' ? `#/server/${localServer.id}` : `#/server/${localServer.id}/${label.toLowerCase().replace(' ', '-')}`
      expect(`${at}: the nav opens ${label}`, page.url().endsWith(want), page.url())
    }
    // Resources: at least one resource or the empty state; search narrows.
    await page.goto(`${WEB}/#/server/${localServer.id}/resources`)
    await page.locator('#server-resources-section').waitFor()
    await page.waitForFunction(
      () => document.querySelector('[data-testid="server-resource"], [data-testid="resources-empty"]'),
      null,
      // Composed from one request per Project and Application; the dev
      // installation's many e2e Projects make that several seconds.
      { timeout: 30000 },
    )
    const resources = page.getByTestId('server-resource')
    const listed = await resources.count()
    if (listed === 0) {
      expect(`${at}: Resources shows its empty state`, (await page.getByTestId('resources-empty').count()) === 1)
    } else {
      const first = ((await resources.first().locator('a').textContent()) ?? '').trim()
      await page.getByPlaceholder('Search resources by name').fill(first)
      await page.waitForTimeout(500)
      const narrowed = await resources.count()
      expect(`${at}: Resources search narrows to "${first}"`, narrowed >= 1 && narrowed <= listed, { narrowed, listed })
      await page.getByPlaceholder('Search resources by name').fill('no-such-resource-xyz')
      await page.waitForTimeout(500)
      expect(`${at}: Resources search shows no match`, (await page.getByTestId('resources-empty').count()) === 1)
    }

    // Metrics: the overview figures and a chart sample within 12 s.
    await page.goto(`${WEB}/#/server/${localServer.id}/metrics`)
    // Resources' requests may still hold the browser's connections here.
    await page.getByTestId('metrics-overview').waitFor({ timeout: 30000 }).catch(() => {})
    expect(`${at}: Metrics shows the overview`, (await page.getByTestId('metrics-overview').count()) === 1)
    const sampled = await page
      .waitForFunction(
        () => [...document.querySelectorAll('[data-testid="usage-chart"] svg')].some((svg) => svg.querySelector('path, circle')),
        null,
        { timeout: 12000 },
      )
      .then(() => true)
      .catch(() => false)
    expect(`${at}: Metrics draws a chart sample`, sampled)

    // Docker Cleanup: its confirmation opens and cancels.
    await page.goto(`${WEB}/#/server/${localServer.id}/docker-cleanup`)
    await page.locator('#docker-cleanup-overview-section').waitFor()
    await page.getByRole('button', { name: 'Run cleanup' }).click()
    const dialog = page.getByRole('dialog', { name: 'Confirm Docker Cleanup?' })
    await dialog.waitFor()
    expect(`${at}: Docker Cleanup asks to confirm`, await dialog.isVisible())
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await page.waitForTimeout(200)
    expect(`${at}: Cancel closes the confirmation`, (await dialog.count()) === 0)

    if (width < 1280) {
      const sideways = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
      expect(`${at}: nothing scrolls sideways`, !sideways)
    }
    await page.close()
  }
}

// The switcher, with the Remote server stand-in as a second Server.
const standIn = await new Promise<boolean>((resolve) => {
  const socket = connect(4972, '127.0.0.1')
  socket.once('connect', () => (socket.destroy(), resolve(true)))
  socket.once('error', () => resolve(false))
})
if (!standIn) {
  console.log('skip the switcher: the Remote server stand-in is not running (task remote:up)')
  console.log('skip Private Key, Danger and the Viewer on a Remote server: no stand-in')
} else {
  const page = await signedIn(1440, 900)
  const name = `e2e-switch-${Date.now()}`
  const added = await page.request.post(`${WEB}/api/servers`, { data: { name, host: '127.0.0.1', port: 4972, user: 'podman' } })
  const other = ((await added.json()) as { server: Server }).server
  try {
    await page.goto(`${WEB}/#/server/${localServer.id}/metrics`)
    await page.getByTestId('server-subtitle').waitFor()
    await page.getByLabel('Switch server').click()
    const options = page.getByTestId('server-switcher-option')
    expect('the switcher lists both Servers', (await options.count()) >= 2, await options.count())
    await page.getByLabel('Filter servers').fill(name)
    await page.waitForTimeout(200)
    expect('the filter narrows to the other Server', (await options.count()) === 1, await options.count())
    await options.first().click()
    await page.waitForTimeout(500)
    expect('the switcher stays on Metrics', page.url().endsWith(`#/server/${other.id}/metrics`), page.url())
    expect('the header shows the other Server', (await page.getByTestId('server-subtitle').textContent())?.trim() === name)

    // Private Key and Danger on a Remote server.
    await page.goto(`${WEB}/#/server/${other.id}/private-key`)
    await page.getByTestId('private-key').waitFor()
    expect('Private Key shows the key type', ((await page.getByTestId('key-type').textContent()) ?? '').startsWith('ssh-'))
    await page.goto(`${WEB}/#/server/${other.id}/danger`)
    await page.locator('#server-danger-section').waitFor()
    await page.getByRole('button', { name: 'Delete server' }).click()
    const confirm = page.getByRole('button', { name: 'Confirm' })
    await confirm.waitFor()
    expect("Danger's button is disabled before the name", await confirm.isDisabled())
    await page.getByRole('textbox', { name: 'Server Name' }).fill(name)
    expect("Danger's button is enabled once the name is typed", await confirm.isEnabled())
    await page.getByRole('button', { name: 'Cancel' }).click()

    // A Viewer: no save button and no Danger nav item.
    const viewer = await signedIn(1440, 900)
    await viewer.route('**/api/me', async (route) => {
      const r = await route.fetch()
      const body = await r.json()
      body.permissions = body.permissions.filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
      for (const g of body.guilds ?? []) g.permissions = (g.permissions ?? []).filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
      await route.fulfill({ response: r, json: body })
    })
    await viewer.goto(`${WEB}/#/server/${other.id}`)
    await viewer.locator('#server-connection-section').waitFor()
    expect('a Viewer sees the inputs disabled', await viewer.locator('#server-connection-section input').first().isDisabled())
    expect('a Viewer sees no save button', (await viewer.getByRole('button', { name: 'Save changes' }).count()) === 0)
    expect('a Viewer sees no Validate', (await viewer.getByTestId('validate').count()) === 0)
    expect('a Viewer sees no Danger nav item', (await viewer.getByRole('link', { name: 'Danger' }).count()) === 0)
    await viewer.close()
  } finally {
    await page.request.delete(`${WEB}/api/servers/${other.id}`)
    await page.close()
  }
}

// General: rename the Local server and back; Validate lists its checkpoints.
{
  const page = await signedIn(1440, 900)
  await page.goto(`${WEB}/#/server/${localServer.id}`)
  await page.locator('#server-connection-section').waitFor()
  const field = page.locator('#server-connection-section input').first()
  const renamed = `${localServer.name}-e2e`
  for (const to of [renamed, localServer.name]) {
    await field.fill(to)
    await page.getByRole('button', { name: 'Save changes' }).click()
    await page.waitForFunction(
      (n) => document.querySelector('[data-testid="server-subtitle"]')?.textContent?.trim() === n,
      to,
      { timeout: 5000 },
    ).catch(() => {})
    expect(`the header shows "${to}"`, (await page.getByTestId('server-subtitle').textContent())?.trim() === to)
  }
  await page.getByTestId('validate').click()
  const dialog = page.getByTestId('validate-dialog')
  await dialog.waitFor()
  const cont = dialog.getByRole('button', { name: 'Continue' })
  if (await cont.count()) await cont.click()
  const points = dialog.locator('[data-checkpoint-status]')
  await points.first().waitFor()
  expect('Validate lists its checkpoints', (await points.count()) >= 4, await points.count())
  await page.waitForFunction(() => !document.querySelector('[data-checkpoint-status="running"]'), null, { timeout: 30000 })
  expect('Validate finishes', (await dialog.getByText(/Validation complete|Validation failed/).count()) === 1)
  await page.close()
}

const page = await signedIn(1440, 900)
await page.goto(`${WEB}/#/servers`)
await page.getByTestId('servers-count').waitFor()

// Search narrows to a match and the empty state on no match, then clears.
const search = page.getByPlaceholder('Search servers')
await search.fill(localServer.name)
await page.waitForTimeout(200)
expect('search narrows to the match', (await page.getByTestId('server').count()) === 1)
await search.fill('no-such-server-xyz')
await page.waitForTimeout(200)
expect('no match shows "No matching servers"', (await page.getByText('No matching servers').count()) === 1)
await page.getByLabel('Clear search').click()
await page.waitForTimeout(200)
expect('clearing restores the list', (await page.getByTestId('server').count()) >= 1)

// The table/grid switch persists across a reload.
await page.getByLabel('Grid view').click()
await page.waitForTimeout(100)
const storedAfterClick = await page.evaluate(() => localStorage.getItem('bakery-servers-view'))
expect('grid view is remembered', storedAfterClick === 'grid', storedAfterClick)
await page.reload()
await page.getByTestId('servers-count').waitFor()
expect('grid view survives a reload', (await page.getByLabel('Grid view').getAttribute('aria-pressed')) === 'true')
await page.getByLabel('Table view').click()

// New server: empty submit shows the field errors.
await page.goto(`${WEB}/#/servers/new`)
await page.getByRole('heading', { name: 'New server' }).waitFor()
await page.getByRole('button', { name: 'Add server' }).click()
await page.waitForTimeout(100)
// The required host field blocks native submission before any server
// round-trip, so the browser's own constraint validation is what shows.
expect('empty submit shows field errors', (await page.locator('input:invalid').count()) > 0)

// A Viewer sees no "New server".
const viewer = await signedIn(1440, 900)
await viewer.route('**/api/me', async (route) => {
  const r = await route.fetch()
  const body = await r.json()
  body.permissions = body.permissions.filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
  for (const g of body.guilds ?? []) g.permissions = (g.permissions ?? []).filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
  await route.fulfill({ response: r, json: body })
})
await viewer.goto(`${WEB}/#/servers`)
await viewer.getByTestId('servers-count').waitFor()
expect('a Viewer sees no "New server"', (await viewer.getByRole('link', { name: 'New server' }).count()) === 0)

// S3 Storage: add (the Garage stand-in when task s3:up runs, else dummy
// values that fail the connection test with a readable message), edit its
// name, delete it, and a Viewer sees the Empty sentence.
const garageUp = await new Promise<boolean>((resolve) => {
  const socket = connect(4960, '127.0.0.1')
  socket.once('connect', () => (socket.destroy(), resolve(true)))
  socket.once('error', () => resolve(false))
})
const s3 = garageUp
  ? (() => {
      const env = Object.fromEntries(
        readFileSync(new URL('../../../infra/dev/state/garage.env', import.meta.url), 'utf8')
          .trim()
          .split('\n')
          .map((l) => [l.slice(0, l.indexOf('=')), l.slice(l.indexOf('=') + 1)]),
      )
      return { endpoint: env.GARAGE_ENDPOINT, region: env.GARAGE_REGION, bucket: env.GARAGE_BUCKET, accessKey: env.GARAGE_ACCESS_KEY, secretKey: env.GARAGE_SECRET_KEY }
    })()
  : { endpoint: 'http://127.0.0.1:4960', region: 'garage', bucket: 'no-such-bucket', accessKey: 'dummy', secretKey: 'dummy' }

for (const theme of ['light', 'dark'] as const) {
  for (const [width, height] of [
    [1440, 900],
    [390, 844],
  ] as const) {
    const at = `${theme} ${width}px`
    const page = await signedIn(width, height, theme)
    await page.goto(`${WEB}/#/storages`)
    await page.getByRole('heading', { name: 'S3 Storage', exact: true, level: 1 }).last().waitFor()

    await page.getByRole('button', { name: 'Add' }).click()
    const name = `e2e-storage-${Date.now()}`
    await page.getByLabel('Name').fill(name)
    await page.getByLabel('Endpoint').fill(s3.endpoint)
    await page.getByLabel('Region').fill(s3.region)
    await page.getByLabel('Bucket').fill(s3.bucket)
    await page.getByLabel('Access key').fill(s3.accessKey)
    await page.getByLabel('Secret key').fill(s3.secretKey)

    await page.getByTestId('s3-storage-test').click()
    const checkResult = page.getByTestId('s3-storage-check')
    await checkResult.waitFor()
    if (garageUp) expect(`${at}: the connection test succeeds`, (await checkResult.textContent())?.includes('Connected') ?? false)
    else expect(`${at}: the connection test fails with a readable message`, ((await checkResult.textContent()) ?? '').trim().length > 0)

    await page.getByTestId('s3-storage-save').click()
    const row = page.getByTestId('s3-storage').filter({ hasText: name })
    await row.waitFor()
    expect(`${at}: the new storage is listed`, (await row.count()) === 1)

    // Edit its name.
    const renamed = `${name}-edit`
    await row.click()
    await page.getByLabel('Name').fill(renamed)
    await page.getByTestId('s3-storage-save').click()
    const renamedRow = page.getByTestId('s3-storage').filter({ hasText: renamed })
    await renamedRow.waitFor()
    expect(`${at}: the rename is listed`, (await renamedRow.count()) === 1)

    // Delete it.
    await renamedRow.getByLabel(`Delete ${renamed}`).click()
    await page.getByRole('button', { name: 'Delete', exact: true }).click()
    await page.waitForFunction((n) => !document.body.textContent?.includes(n), renamed, { timeout: 5000 }).catch(() => {})
    expect(`${at}: the storage is gone after delete`, (await page.getByTestId('s3-storage').filter({ hasText: renamed }).count()) === 0)

    if (width < 1280) {
      const sideways = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
      expect(`${at}: nothing scrolls sideways`, !sideways)
    }
    await page.close()
  }
}

// A Viewer sees the Empty sentence, no "Add".
const storagesViewer = await signedIn(1440, 900)
await storagesViewer.route('**/api/me', async (route) => {
  const r = await route.fetch()
  const body = await r.json()
  body.permissions = body.permissions.filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
  for (const g of body.guilds ?? []) g.permissions = (g.permissions ?? []).filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
  await route.fulfill({ response: r, json: body })
})
await storagesViewer.goto(`${WEB}/#/storages`)
await storagesViewer.getByRole('heading', { name: 'S3 Storage', exact: true, level: 1 }).last().waitFor()
expect('a Viewer sees the Empty sentence', (await storagesViewer.getByText('S3 Storage needs the Manage servers permission').count()) === 1)
expect('a Viewer sees no "Add"', (await storagesViewer.getByRole('button', { name: 'Add' }).count()) === 0)
await storagesViewer.close()

await browser.close()
if (failed) {
  console.log(`${failed} failed`)
  process.exit(1)
}
console.log('the Servers flows work')
