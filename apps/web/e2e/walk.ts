// The route walk: signs in as the Owner and opens every route the router
// knows (src/lib/router.svelte.ts) in headless Chromium, at 1440×900 and
// 390×844, dark and light. A route fails on a page error or console error, a
// horizontal scrollbar at 390px, or a page that opens outside the shell. The
// Projects, Resources and Servers it opens are the first ones it finds in the
// running dev installation; Login and Invite are opened signed out. No page
// may read "Coolify" or "Paperclip" anywhere a person reads it.
//
//   bun e2e/walk.ts            (task web:walk; needs task dev running)
//
// BAKERY_WEB (default http://127.0.0.1:4930), CHROMIUM (default
// /usr/bin/chromium), BAKERY_OWNER_EMAIL and BAKERY_OWNER_PASSWORD (default
// infra/dev/state/owner.env) override what it uses.
import { readFileSync } from 'node:fs'
import { chromium, type BrowserContext, type Page } from 'playwright-core'

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

const browser = await chromium.launch({ executablePath: CHROMIUM })
const who = owner()

// The ids to open, from the API as the Owner reads it.
const probe = await browser.newContext()
async function get<T>(path: string): Promise<T> {
  const r = await probe.request.get(`${WEB}/api${path}`)
  if (!r.ok()) throw new Error(`GET ${path}: ${r.status()}`)
  return (await r.json()) as T
}
const login = await probe.request.post(`${WEB}/api/login`, { data: who })
if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)

type Env = { id: number; applications: { id: number }[] }
type Ref = { id: number; project_id: number; environment_id: number }
const { projects } = await get<{ projects: { id: number }[] }>('/projects')
let app: Ref | undefined, database: Ref | undefined, service: Ref | undefined
let project = projects[0]?.id, environment: number | undefined
for (const { id } of projects) {
  const p = (await get<{ project: { environments: Env[] } }>(`/projects/${id}`)).project
  const e = p.environments.find((x) => x.applications.length)
  if (!app && e) app = { id: e.applications[0].id, project_id: id, environment_id: e.id }
  if (!database) database = (await get<{ databases: Ref[] }>(`/projects/${id}/databases`)).databases[0]
  if (!service) service = (await get<{ services: Ref[] }>(`/projects/${id}/services`)).services[0]
  if (app && database && service) break
}
if (!app || !database || !service) throw new Error('the walk needs an Application, a Database and a Service in some Project')
project = app.project_id
environment = app.environment_id
const deployment = (await get<{ deployments: { id: number }[] }>(`/applications/${app.id}/deployments`)).deployments[0]
const backup = (await get<{ scheduled_backups: { id: number }[] }>(`/databases/${database.id}/scheduled-backups`)).scheduled_backups[0]
const server = (await get<{ servers: { id: number }[] }>('/servers')).servers[0]
const role = (await get<{ roles: { id: number }[] }>('/roles')).roles[0]

const routerSource = readFileSync(new URL('../src/lib/router.svelte.ts', import.meta.url), 'utf8')
function pages(name: string): string[] {
  const m = routerSource.match(new RegExp(`export const ${name} = \\[([^\\]]*)\\]`))
  if (!m) throw new Error(`router.svelte.ts has no ${name}`)
  return [...m[1].matchAll(/'([^']*)'/g)].map((x) => x[1])
}
const sub = (base: string, page: string) => (page ? `${base}/${page}` : base)
const appBase = `/project/${app.project_id}/environment/${app.environment_id}/application/${app.id}`
const dbBase = `/project/${database.project_id}/environment/${database.environment_id}/database/${database.id}`
const svcBase = `/project/${service.project_id}/environment/${service.environment_id}/service/${service.id}`

const routes = [
  '/',
  '/projects',
  `/project/${project}`,
  `/project/${project}/edit`,
  `/project/${project}/permissions`,
  `/project/${project}/environment/${environment}`,
  `/project/${project}/environment/${environment}/edit`,
  `/project/${project}/environment/${environment}/new`,
  ...pages('applicationPages').map((p) => sub(appBase, p)),
  ...(deployment ? [`${appBase}/deployment/${deployment.id}`] : []),
  ...pages('databasePages').map((p) => sub(dbBase, p)),
  ...(backup ? pages('scheduledBackupSections').map((s) => sub(`${dbBase}/backups/${backup.id}`, s)) : []),
  ...pages('servicePages').map((p) => sub(svcBase, p)),
  '/servers',
  '/servers/new',
  ...pages('serverPages').map((p) => sub(`/server/${server.id}`, p)),
  '/storages',
  '/settings',
  ...pages('guildPages').map((p) => sub('/guild', p)),
  `/guild/roles/${role.id}`,
  '/guild/new',
  ...pages('notificationPages').map((p) => `/notifications/${p}`),
  ...pages('securityPages').map((p) => `/security/${p}`),
  '/profile',
]

// An Invitation for the signed-out Invite page, revoked at the end.
const inv = await probe.request.post(`${WEB}/api/invitations`, { data: { email: `walk-${Date.now()}@example.test`, role_ids: [] } })
if (!inv.ok()) throw new Error(`invite: ${inv.status()}`)
const invitation = (await inv.json()) as { invitation: { id: number }; path: string }

const combos = [
  { width: 1440, height: 900, theme: 'dark' },
  { width: 1440, height: 900, theme: 'light' },
  { width: 390, height: 844, theme: 'dark' },
  { width: 390, height: 844, theme: 'light' },
]

let failed = 0
function report(ok: boolean, what: string, why: string[]) {
  if (ok) return console.log(`ok   ${what}`)
  failed++
  console.log(`FAIL ${what}: ${why.join('; ')}`)
}

async function open(ctx: BrowserContext, hash: string, signedIn: boolean): Promise<{ page: Page; why: string[] }> {
  const page = await ctx.newPage()
  const why: string[] = []
  page.on('pageerror', (e) => why.push(`page error: ${e.message}`))
  page.on('console', (m) => {
    // Signed out, the dashboard learns it from /api/me answering 401, which
    // Chromium logs as a failed resource.
    if (!signedIn && m.text().includes('status of 401')) return
    if (m.type() === 'error') why.push(`console error: ${m.text()}`)
  })
  await page.goto(`${WEB}/#${hash}`)
  if (signedIn) await page.getByTestId('breadcrumb-bar').waitFor({ timeout: 10000 }).catch(() => {})
  // Pages load their data after the shell shows; give them time to render it.
  await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {})
  await page.waitForTimeout(400)
  return { page, why }
}

// Words a person reads: the text, titles, labels, placeholders and the tab title.
async function namesASource(page: Page): Promise<string | null> {
  const read = await page.evaluate(() =>
    [
      document.title,
      document.body.innerText,
      ...[...document.querySelectorAll('[title],[aria-label],[placeholder],[alt]')].flatMap((el) =>
        ['title', 'aria-label', 'placeholder', 'alt'].map((a) => el.getAttribute(a) ?? ''),
      ),
    ].join('\n'),
  )
  const m = read.match(/coolify|paperclip/i)
  return m ? `reads "${read.slice(Math.max(0, m.index! - 30), m.index! + 30).replace(/\s+/g, ' ')}"` : null
}

async function overflow(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    for (const el of [document.documentElement, document.querySelector('main')]) {
      if (el && el.scrollWidth > el.clientWidth + 1) return `${el.tagName.toLowerCase()} scrolls sideways (${el.scrollWidth} > ${el.clientWidth})`
    }
    return null
  })
}

for (const { width, height, theme } of combos) {
  const label = `${width}×${height} ${theme}`
  const ctx = await browser.newContext({ viewport: { width, height } })
  await ctx.addInitScript((t) => localStorage.setItem('theme', t), theme)
  await ctx.request.post(`${WEB}/api/login`, { data: who })
  for (const hash of routes) {
    const { page, why } = await open(ctx, hash, true)
    if (!(await page.getByTestId('breadcrumb-bar').isVisible())) why.push('outside the shell (no breadcrumb bar)')
    const dark = await page.evaluate(() => document.documentElement.classList.contains('dark'))
    if (dark !== (theme === 'dark')) why.push(`not in the ${theme} theme`)
    const named = await namesASource(page)
    if (named) why.push(named)
    if (width < 768) {
      const o = await overflow(page)
      if (o) why.push(o)
    }
    report(why.length === 0, `${label} #${hash}`, why)
    await page.close()
  }
  await ctx.close()

  // Signed out: Login, and Invite with a live Invitation.
  const out = await browser.newContext({ viewport: { width, height } })
  await out.addInitScript((t) => localStorage.setItem('theme', t), theme)
  for (const hash of ['/login', invitation.path.replace(/^\/?#/, '')]) {
    const { page, why } = await open(out, hash, false)
    if (!(await page.getByRole('button').first().isVisible())) why.push('nothing to press')
    if (await page.getByTestId('breadcrumb-bar').isVisible()) why.push('signed out but inside the shell')
    const named = await namesASource(page)
    if (named) why.push(named)
    if (width < 768) {
      const o = await overflow(page)
      if (o) why.push(o)
    }
    report(why.length === 0, `${label} #${hash} (signed out)`, why)
    await page.close()
  }
  await out.close()
}

await probe.request.delete(`${WEB}/api/invitations/${invitation.invitation.id}`)
await browser.close()
console.log(failed ? `${failed} failed` : `every route passed (${routes.length + 2} routes × ${combos.length})`)
process.exit(failed ? 1 : 0)
