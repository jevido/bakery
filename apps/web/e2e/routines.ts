// Routines in the dashboard, one section each, every flow once in the dark
// theme at 1440×900:
//   page    a scratch Agent and a scratch Project; #/routines has Routines
//           current in the sidebar; Create routine with the Agent, the
//           Project and a Weekly schedule (Mon 9 AM, Europe/Amsterdam) lands
//           on the Routine page, and the API holds its "0 9 * * 1" trigger;
//           back on #/routines the row shows the Agent and the Project, Run
//           now creates a Routine run that Recent Runs lists with its
//           Execution Issue; a viewer sees the Routine but no Create
//           routine, Run now or toggle; the scratch data is removed again
//
//   bun e2e/routines.ts [section ...]   (task web:routines; needs task dev)
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

async function signedIn(): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

/** A person invited as role for the run, signed in in a context of their own; leave removes them. */
async function invited(owner: Page, role: 'viewer' | 'member'): Promise<{ page: Page; leave: () => Promise<void> }> {
  const { members } = (await (await owner.request.get(`${WEB}/api/members`)).json()) as { members: { id: number; email: string }[] }
  for (const m of members.filter((m) => m.email.startsWith(`routines-${role}-`))) await owner.request.delete(`${WEB}/api/members/${m.id}`)
  const inv = await owner.request.post(`${WEB}/api/invitations`, { data: { email: `routines-${role}-${Date.now()}@example.test`, role } })
  if (!inv.ok()) throw new Error(`invite: ${inv.status()}`)
  const token = ((await inv.json()) as { path: string }).path.split('/').pop()
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  const accept = await ctx.request.post(`${WEB}/api/invitations/by-token/${token}/accept`, { data: { name: `Routines ${role}`, password: 'a long enough password' } })
  if (!accept.ok()) throw new Error(`accept: ${accept.status()} ${await accept.text()}`)
  const { member } = (await accept.json()) as { member: { id: number } }
  return {
    page: await ctx.newPage(),
    leave: async () => {
      await ctx.close()
      await owner.request.delete(`${WEB}/api/members/${member.id}`)
    },
  }
}

type Trigger = { kind: string; cron_expression: string | null; timezone: string | null; next_run_at: string | null }
type Run = { status: string; routine: { id: number }; issue: { id: number; identifier: string } | null }
type Routine = { id: number; title: string; status: string; triggers: Trigger[] }

/** Archives the Routines titled title and cancels their open Execution Issues. */
async function clearRoutines(page: Page, title: string) {
  const { routines } = (await (await page.request.get(`${WEB}/api/routines`)).json()) as { routines: Routine[] }
  for (const r of routines.filter((r) => r.title === title)) {
    const { routine_runs } = (await (await page.request.get(`${WEB}/api/routines/${r.id}/runs`)).json()) as { routine_runs: Run[] }
    for (const run of routine_runs) if (run.issue) await page.request.patch(`${WEB}/api/issues/${run.issue.id}`, { data: { status: 'cancelled' } })
    if (r.status !== 'archived') await page.request.patch(`${WEB}/api/routines/${r.id}`, { data: { status: 'archived' } })
  }
}

const sections: Record<string, () => Promise<void>> = {
  async page() {
    const page = await signedIn()
    const title = 'Routines e2e: weekly dependency check'
    const agentName = 'Routines e2e agent'
    const projectName = 'Routines e2e project'
    await clearRoutines(page, title)
    const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
    for (const a of agents.filter((a) => a.name === agentName)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
    const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
    for (const p of projects.filter((p) => p.name === projectName)) await page.request.delete(`${WEB}/api/projects/${p.id}`)

    const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: projectName } })).json()) as { project: { id: number } }
    const hired = (await (await page.request.post(`${WEB}/api/agents`, { data: { name: agentName, job: 'engineer', icon: 'bot' } })).json()) as { agent: { id: number; approval_id: number } }
    await page.request.post(`${WEB}/api/approvals/${hired.agent.approval_id}/approve`, { data: {} })
    const agent = hired.agent.id

    try {
      await page.goto(`${WEB}/#/routines`)
      const nav = page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Routines' })
      await nav.waitFor()
      expect('the sidebar has Routines, current', (await nav.getAttribute('aria-current')) === 'page', await nav.getAttribute('aria-current'))

      await page.getByRole('button', { name: 'Create routine' }).first().click()
      const dialog = page.getByRole('dialog')
      await dialog.getByLabel('Routine title').fill(title)
      await dialog.getByRole('button', { name: 'Agent' }).click()
      await page.getByRole('option', { name: agentName }).click()
      await dialog.getByRole('button', { name: 'Project' }).click()
      await page.getByRole('option', { name: projectName }).click()
      await dialog.getByRole('switch', { name: 'Add a schedule' }).click()
      await dialog.getByLabel('Schedule frequency').click()
      await page.getByRole('option', { name: 'Weekly' }).click()
      await dialog.getByLabel('Hour').click()
      await page.getByRole('option', { name: '9 AM', exact: true }).click()
      await dialog.getByRole('button', { name: 'Mon' }).click()
      await dialog.getByLabel('Time zone').click()
      await page.getByRole('option', { name: 'Europe/Amsterdam', exact: true }).click()
      const description = await dialog.getByTestId('schedule-description').textContent()
      expect('the schedule says it in words', description === 'Every Mon at 9:00 AM (Europe/Amsterdam)', description)
      await dialog.getByRole('button', { name: 'Create routine' }).click()

      await page.waitForURL(/#\/routines\/\d+$/)
      const id = Number(page.url().split('/').pop())
      await page.getByRole('heading', { name: title }).waitFor()
      expect('Create routine lands on the Routine page', true)
      const { routine } = (await (await page.request.get(`${WEB}/api/routines/${id}`)).json()) as { routine: Routine }
      const t = routine.triggers[0]
      expect(
        'the Routine has its weekly schedule',
        routine.triggers.length === 1 && t.kind === 'schedule' && t.cron_expression === '0 9 * * 1' && t.timezone === 'Europe/Amsterdam' && t.next_run_at !== null,
        routine.triggers,
      )

      await page.goto(`${WEB}/#/routines`)
      const row = page.getByRole('list', { name: 'Routines' }).getByRole('listitem').filter({ hasText: title })
      await row.waitFor()
      const text = (await row.textContent()) ?? ''
      expect('the row shows the Agent and the Project', text.includes(agentName) && text.includes(projectName) && text.includes('Never'), text)
      await row.getByRole('button', { name: 'Run now' }).click()
      await row.getByText(/· issue created/).waitFor()
      expect('Run now shows the last run', true)

      await page.getByRole('tab', { name: 'Recent Runs' }).click()
      await page.waitForURL(/#\/routines\/runs$/)
      const { routine_runs } = (await (await page.request.get(`${WEB}/api/routines/${id}/runs`)).json()) as { routine_runs: Run[] }
      const run = routine_runs[0]
      expect('the Routine run created an Execution Issue', run?.status === 'issue_created' && run.issue !== null, run)
      const runRow = page.getByRole('list', { name: 'Recent Runs' }).getByRole('listitem').filter({ hasText: title }).first()
      await runRow.waitFor()
      const runText = (await runRow.textContent()) ?? ''
      expect('Recent Runs lists it with its Issue', runText.includes('issue created') && runText.includes(run.issue!.identifier), runText)

      const viewer = await invited(page, 'viewer')
      try {
        await viewer.page.goto(`${WEB}/#/routines`)
        const vrow = viewer.page.getByRole('list', { name: 'Routines' }).getByRole('listitem').filter({ hasText: title })
        await vrow.waitFor()
        expect('a viewer sees the Routine', true)
        expect('a viewer has no Create routine', (await viewer.page.getByRole('button', { name: 'Create routine' }).count()) === 0)
        expect('a viewer has no Run now or toggle', (await vrow.getByRole('button', { name: 'Run now' }).count()) === 0 && (await vrow.getByRole('switch').count()) === 0)
      } finally {
        await viewer.leave()
      }
    } finally {
      await clearRoutines(page, title)
      await page.request.post(`${WEB}/api/agents/${agent}/terminate`)
      await page.request.delete(`${WEB}/api/projects/${project.id}`)
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
console.log('the routines flows work')
