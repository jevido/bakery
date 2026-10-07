// The Servers pages and S3 Storage in headless Chromium, one section each,
// every flow once in the light theme on a desktop:
//   list       the Local server shows Ready, search narrows and clears, the
//              table/grid switch persists, New server validates an empty
//              submit, a Viewer sees no "New server"
//   frame      the header, status pill, breadcrumb and nav to every sub-page
//   resources  the Local server's resources (or the empty state) within 2 s,
//              and search narrows them
//   metrics    the overview and a chart sample
//   cleanup    Docker Cleanup's confirmation opens and cancels
//   remote     the switcher, Private Key, Danger and a Viewer on the Remote
//              server stand-in (task remote:up), else skipped
//   general    rename the Local server and back; Validate's checkpoints
//   storages   S3 Storage add, test, rename, delete, and a Viewer
//   render     each page in dark on a desktop and both themes on a phone:
//              it loads and nothing scrolls sideways
//
//   bun e2e/servers.ts [section ...]   (task web:servers; needs task dev)
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

async function signedIn(width = 1440, height = 900, theme: 'dark' | 'light' = 'light'): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  await ctx.addInitScript((t) => localStorage.setItem('theme', t), theme)
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

// A Viewer: the owner with manage_servers and administrator taken out of
// what /api/me says.
async function viewer(): Promise<Page> {
  const page = await signedIn()
  await page.route('**/api/me', async (route) => {
    const r = await route.fetch()
    const body = await r.json()
    body.permissions = body.permissions.filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
    for (const g of body.guilds ?? []) g.permissions = (g.permissions ?? []).filter((p: string) => p !== 'manage_servers' && p !== 'administrator')
    await route.fulfill({ response: r, json: body })
  })
  return page
}

function listening(port: number): Promise<boolean> {
  return new Promise((resolve) => {
    const socket = connect(port, '127.0.0.1')
    socket.once('connect', () => (socket.destroy(), resolve(true)))
    socket.once('error', () => resolve(false))
  })
}

const probe = await signedIn()
const localServer = ((await (await probe.request.get(`${WEB}/api/servers`)).json()) as { servers: Server[] }).servers.find(
  (s) => s.kind === 'local',
)!
await probe.close()

const sections: Record<string, () => Promise<void>> = {
  // The list: the Local server Ready, search, the table/grid switch, New
  // server's empty submit, and a Viewer without "New server".
  async list() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/servers`)
    await page.getByTestId('servers-count').waitFor()
    const rows = page.getByTestId('server')
    expect('the list has the Local server', (await rows.filter({ hasText: localServer.name }).count()) === 1)
    expect('it is Ready', (await rows.filter({ hasText: localServer.name }).getByTestId('server-status').textContent()) === 'Ready')

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

    await page.getByLabel('Grid view').click()
    await page.waitForTimeout(100)
    const stored = await page.evaluate(() => localStorage.getItem('bakery-servers-view'))
    expect('grid view is remembered', stored === 'grid', stored)
    await page.reload()
    await page.getByTestId('servers-count').waitFor()
    expect('grid view survives a reload', (await page.getByLabel('Grid view').getAttribute('aria-pressed')) === 'true')
    await page.getByLabel('Table view').click()

    await page.goto(`${WEB}/#/servers/new`)
    await page.getByRole('heading', { name: 'New server' }).waitFor()
    await page.getByRole('button', { name: 'Add server' }).click()
    await page.waitForTimeout(100)
    // The required host field blocks native submission before any server
    // round-trip, so the browser's own constraint validation is what shows.
    expect('empty submit shows field errors', (await page.locator('input:invalid').count()) > 0)
    await page.close()

    const v = await viewer()
    await v.goto(`${WEB}/#/servers`)
    await v.getByTestId('servers-count').waitFor()
    expect('a Viewer sees no "New server"', (await v.getByRole('link', { name: 'New server' }).count()) === 0)
    await v.close()
  },

  // The Server frame on the Local server: header, status pill, breadcrumb
  // and the nav to every sub-page.
  async frame() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/server/${localServer.id}`)
    await page.getByTestId('server-subtitle').waitFor()
    expect('the header shows the name', (await page.getByTestId('server-subtitle').textContent())?.trim() === localServer.name)
    const pill = page.getByTestId('server-status-summary')
    expect('the status pill says Ready', ((await pill.textContent()) ?? '').includes('Ready'))
    await pill.click()
    expect('the pill opens System status', await page.getByText('System status').isVisible())
    await page.keyboard.press('Escape')
    const crumbs = await page.getByTestId('breadcrumb-bar').textContent()
    expect('the breadcrumb is Servers › the name', !!crumbs?.includes(localServer.name), crumbs)
    expect('the top bar holds no server context', (await page.locator('#server-topbar-context, #resource-action-hud-slot').count()) === 0)
    for (const label of ['General', 'Resources', 'Docker Cleanup', 'Metrics']) {
      await page.getByRole('navigation', { name: 'Server configuration sections' }).getByRole('link', { name: label, exact: true }).click()
      await page.waitForTimeout(300)
      const want = label === 'General' ? `#/server/${localServer.id}` : `#/server/${localServer.id}/${label.toLowerCase().replace(' ', '-')}`
      expect(`the nav opens ${label}`, page.url().endsWith(want), page.url())
    }
    await page.close()
  },

  // Resources: at least one resource or the empty state; search narrows.
  async resources() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/server/${localServer.id}/resources`)
    await page.locator('#server-resources-section').waitFor()
    const started = Date.now()
    await page.waitForFunction(
      () => document.querySelector('[data-testid="server-resource"], [data-testid="resources-empty"]'),
      null,
      { timeout: 5000 },
    )
    const took = Date.now() - started
    expect(`Resources lists within 2 s (${took} ms)`, took < 2000, took)
    const resources = page.getByTestId('server-resource')
    const listed = await resources.count()
    if (listed === 0) {
      expect('Resources shows its empty state', (await page.getByTestId('resources-empty').count()) === 1)
    } else {
      const first = ((await resources.first().locator('a').textContent()) ?? '').trim()
      await page.getByPlaceholder('Search resources by name').fill(first)
      await page.waitForTimeout(500)
      const narrowed = await resources.count()
      expect(`Resources search narrows to "${first}"`, narrowed >= 1 && narrowed <= listed, { narrowed, listed })
      await page.getByPlaceholder('Search resources by name').fill('no-such-resource-xyz')
      await page.waitForTimeout(500)
      expect('Resources search shows no match', (await page.getByTestId('resources-empty').count()) === 1)
    }
    await page.close()
  },

  // Metrics: the overview figures and a chart sample within 12 s.
  async metrics() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/server/${localServer.id}/metrics`)
    await page.getByTestId('metrics-overview').waitFor({ timeout: 10000 }).catch(() => {})
    expect('Metrics shows the overview', (await page.getByTestId('metrics-overview').count()) === 1)
    const sampled = await page
      .waitForFunction(
        () => [...document.querySelectorAll('[data-testid="usage-chart"] svg')].some((svg) => svg.querySelector('path, circle')),
        null,
        { timeout: 12000 },
      )
      .then(() => true)
      .catch(() => false)
    expect('Metrics draws a chart sample', sampled)
    await page.close()
  },

  // Docker Cleanup: its confirmation opens and cancels.
  async cleanup() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/server/${localServer.id}/docker-cleanup`)
    await page.locator('#docker-cleanup-overview-section').waitFor()
    await page.getByRole('button', { name: 'Run cleanup' }).click()
    const dialog = page.getByRole('dialog', { name: 'Confirm Docker Cleanup?' })
    await dialog.waitFor()
    expect('Docker Cleanup asks to confirm', await dialog.isVisible())
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await page.waitForTimeout(200)
    expect('Cancel closes the confirmation', (await dialog.count()) === 0)
    await page.close()
  },

  // The switcher, Private Key, Danger and a Viewer, with the Remote server
  // stand-in as a second Server.
  async remote() {
    if (!(await listening(4972))) {
      console.log('skip remote: the Remote server stand-in is not running (task remote:up)')
      return
    }
    const page = await signedIn()
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

      const v = await viewer()
      await v.goto(`${WEB}/#/server/${other.id}`)
      await v.locator('#server-connection-section').waitFor()
      expect('a Viewer sees the inputs disabled', await v.locator('#server-connection-section input').first().isDisabled())
      expect('a Viewer sees no save button', (await v.getByRole('button', { name: 'Save changes' }).count()) === 0)
      expect('a Viewer sees no Validate', (await v.getByTestId('validate').count()) === 0)
      expect('a Viewer sees no Danger nav item', (await v.getByRole('link', { name: 'Danger' }).count()) === 0)
      await v.close()
    } finally {
      await page.request.delete(`${WEB}/api/servers/${other.id}`)
      await page.close()
    }
  },

  // General: rename the Local server and back; Validate lists its checkpoints.
  async general() {
    const page = await signedIn()
    await page.goto(`${WEB}/#/server/${localServer.id}`)
    await page.locator('#server-connection-section').waitFor()
    const field = page.locator('#server-connection-section input').first()
    for (const to of [`${localServer.name}-e2e`, localServer.name]) {
      await field.fill(to)
      await page.getByRole('button', { name: 'Save changes' }).click()
      await page
        .waitForFunction((n) => document.querySelector('[data-testid="server-subtitle"]')?.textContent?.trim() === n, to, { timeout: 5000 })
        .catch(() => {})
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
  },

  // S3 Storage: add (the Garage stand-in when task s3:up runs, else dummy
  // values that fail the connection test with a readable message), edit its
  // name, delete it, and a Viewer sees the Empty sentence.
  async storages() {
    const garageUp = await listening(4960)
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

    const page = await signedIn()
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
    if (garageUp) expect('the connection test succeeds', (await checkResult.textContent())?.includes('Connected') ?? false)
    else expect('the connection test fails with a readable message', ((await checkResult.textContent()) ?? '').trim().length > 0)

    await page.getByTestId('s3-storage-save').click()
    const row = page.getByTestId('s3-storage').filter({ hasText: name })
    await row.waitFor()
    expect('the new storage is listed', (await row.count()) === 1)

    const renamed = `${name}-edit`
    await row.click()
    await page.getByLabel('Name').fill(renamed)
    await page.getByTestId('s3-storage-save').click()
    const renamedRow = page.getByTestId('s3-storage').filter({ hasText: renamed })
    await renamedRow.waitFor()
    expect('the rename is listed', (await renamedRow.count()) === 1)

    await renamedRow.getByLabel(`Delete ${renamed}`).click()
    await page.getByRole('button', { name: 'Delete', exact: true }).click()
    await page.waitForFunction((n) => !document.body.textContent?.includes(n), renamed, { timeout: 5000 }).catch(() => {})
    expect('the storage is gone after delete', (await page.getByTestId('s3-storage').filter({ hasText: renamed }).count()) === 0)
    await page.close()

    const v = await viewer()
    await v.goto(`${WEB}/#/storages`)
    await v.getByRole('heading', { name: 'S3 Storage', exact: true, level: 1 }).last().waitFor()
    expect('a Viewer sees the Empty sentence', (await v.getByText('S3 Storage needs the Manage servers permission').count()) === 1)
    expect('a Viewer sees no "Add"', (await v.getByRole('button', { name: 'Add' }).count()) === 0)
    await v.close()
  },

  // Dark and phone: every page loads in its theme and nothing scrolls
  // sideways. The flows above run once, light on a desktop.
  async render() {
    const pages: [string, string][] = [
      ['Servers', '/servers'],
      ['New server', '/servers/new'],
      ['General', `/server/${localServer.id}`],
      ['Resources', `/server/${localServer.id}/resources`],
      ['Metrics', `/server/${localServer.id}/metrics`],
      ['Docker Cleanup', `/server/${localServer.id}/docker-cleanup`],
      ['S3 Storage', '/storages'],
    ]
    for (const [theme, width, height] of [
      ['dark', 1440, 900],
      ['light', 390, 844],
      ['dark', 390, 844],
    ] as const) {
      const page = await signedIn(width, height, theme)
      for (const [label, path] of pages) {
        const at = `${theme} ${width}px ${label}`
        await page.goto(`${WEB}/#${path}`)
        await page.locator('main').first().waitFor()
        await page.waitForTimeout(400)
        const dark = await page.evaluate(() => document.documentElement.classList.contains('dark'))
        const sideways = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
        expect(`${at}: loads in the ${theme} theme, nothing scrolls sideways`, dark === (theme === 'dark') && !sideways, { dark, sideways })
      }
      await page.close()
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
console.log('the Servers flows work')
