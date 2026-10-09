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
//   detail  on a Routine page (a scratch Agent, Project and Routine): add a
//           Schedule trigger and see its Next run, switch it off and on,
//           edit the title and save it, press Run and follow the toast to
//           the Execution Issue, whose properties link back to the Routine;
//           Runs lists the run as "issue created"; Activity shows the
//           creation and the trigger events
//   run     Run on a Routine page: the toast opens the Execution Issue,
//           assigned to the Agent, with its Routine row; Runs shows "issue
//           created"; Run again (the Issue's Run still queued, no Desktop
//           runs it) coalesces into the same Issue; the Issue set done
//           makes its Routine run "completed"
//   pause   the toggle pauses the Routine (the trigger's Next run goes) and
//           resumes it; Archive from the list's row menu takes it out of
//           the list and leaves its page read-only
//   schedule  a "* * * * *" trigger fires by itself within 90 s (slow)
//   webhook a GitHub Webhook trigger added on the Triggers tab shows its URL
//           and 48-hex secret once; after a reload the card has the URL and
//           no secret; a delivery signed with the secret is "accepted" and
//           Runs lists a Webhook run with its Issue; after Rotate secret the
//           old secret is 401 and the card says "rejected"
//   variables  "Triage {{repo}} alert" being edited shows repo in the
//           Variables editor; made a Select (shop, blog, default shop) and
//           saved, Run opens the Run dialog with shop picked; blog is run
//           and Runs links an Issue "triage blog alert"; with repo's
//           default cleared, Add schedule is refused naming repo
//
//   bun e2e/routines.ts [section ...]   (task web:routines; needs task dev)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { createHmac } from 'node:crypto'
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

/** A scratch Agent and Project named after the section, removed again by drop. */
async function scratch(page: Page, name: string): Promise<{ agent: number; project: number; drop: () => Promise<void> }> {
  const agentName = `${name} agent`
  const projectName = `${name} project`
  const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
  for (const a of agents.filter((a) => a.name === agentName)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
  const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
  for (const p of projects.filter((p) => p.name === projectName)) await page.request.delete(`${WEB}/api/projects/${p.id}`)
  const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: projectName } })).json()) as { project: { id: number } }
  const hired = (await (await page.request.post(`${WEB}/api/agents`, { data: { name: agentName, job: 'engineer', icon: 'bot' } })).json()) as { agent: { id: number; approval_id: number } }
  await page.request.post(`${WEB}/api/approvals/${hired.agent.approval_id}/approve`, { data: {} })
  return {
    agent: hired.agent.id,
    project: project.id,
    drop: async () => {
      await page.request.post(`${WEB}/api/agents/${hired.agent.id}/terminate`)
      await page.request.delete(`${WEB}/api/projects/${project.id}`)
    },
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

  async detail() {
    const page = await signedIn()
    const title = 'Routines e2e: routine page'
    const renamed = 'Routines e2e: routine page, renamed'
    await clearRoutines(page, title)
    await clearRoutines(page, renamed)
    const { agent, project, drop } = await scratch(page, 'Routines e2e detail')
    const created = await page.request.post(`${WEB}/api/routines`, { data: { title, assignee_agent_id: agent, project_id: project } })
    const id = ((await created.json()) as { routine: Routine }).routine.id
    const routine = async () => ((await (await page.request.get(`${WEB}/api/routines/${id}`)).json()) as { routine: Routine & { triggers: (Trigger & { enabled: boolean })[] } }).routine

    try {
      await page.goto(`${WEB}/#/routines/${id}/triggers`)
      const sub = page.getByRole('navigation', { name: 'Routine sections' })
      await sub.getByRole('tab', { name: 'Triggers' }).waitFor()
      expect('the sub-sidebar marks Triggers', (await sub.getByRole('tab', { name: 'Triggers' }).getAttribute('aria-current')) === 'page')
      await page.getByRole('button', { name: 'Add schedule' }).click()
      await page.getByRole('group', { name: 'New trigger' }).getByRole('button', { name: 'Add trigger' }).click()
      const card = page.getByRole('group', { name: 'Trigger: Schedule' })
      await card.getByText(/^Next: /).waitFor()
      expect('the Schedule trigger shows its Next run', true)
      await card.getByRole('switch', { name: 'Disable Schedule' }).click()
      await card.getByText('Not scheduled').waitFor()
      expect('switched off, the trigger is not scheduled', (await routine()).triggers[0]?.enabled === false)
      await card.getByRole('switch', { name: 'Enable Schedule' }).click()
      await card.getByText(/^Next: /).waitFor()
      const on = (await routine()).triggers[0]
      expect('switched on again, it has a Next run', on?.enabled === true && on.next_run_at !== null, on)

      await sub.getByRole('tab', { name: 'Overview' }).click()
      await page.getByRole('button', { name: 'Edit routine' }).click()
      await page.getByLabel('Routine title').fill(renamed)
      await page.getByRole('region', { name: 'Unsaved changes' }).getByRole('button', { name: /Save changes/ }).click()
      await page.getByRole('heading', { name: renamed }).waitFor()
      expect('the edited title is saved', (await routine()).title === renamed)

      await page.getByRole('button', { name: 'Run', exact: true }).click()
      const open = page.getByRole('link', { name: /^Open / })
      await open.waitFor()
      await page.waitForURL(new RegExp(`#/routines/${id}/runs$`))
      const runRow = page.getByRole('list', { name: 'Routine runs' }).getByRole('listitem').first()
      await runRow.waitFor()
      const runText = (await runRow.textContent()) ?? ''
      expect('Runs lists the run as issue created', runText.includes('issue created'), runText)
      const identifier = ((await open.textContent()) ?? '').replace('Open ', '')
      await open.click()
      await page.waitForURL(new RegExp(`#/issues/${identifier}$`))
      const back = page.locator('[data-property-row="Routine"]').getByRole('link', { name: renamed })
      await back.waitFor()
      expect('the Execution Issue links back to the Routine', (await back.getAttribute('href')) === `#/routines/${id}`, await back.getAttribute('href'))

      await page.goto(`${WEB}/#/routines/${id}/activity`)
      const feed = page.getByRole('list', { name: 'Routine activity' })
      await feed.waitFor()
      const actions = await feed.locator('[data-activity]').evaluateAll((els) => els.map((e) => e.getAttribute('data-activity')))
      expect(
        'Activity shows the creation and the trigger events',
        ['routine.created', 'routine.trigger_created', 'routine.trigger_updated', 'routine.updated'].every((a) => actions.includes(a)),
        actions,
      )
    } finally {
      await clearRoutines(page, title)
      await clearRoutines(page, renamed)
      await drop()
      await page.context().close()
    }
  },

  async run() {
    const page = await signedIn()
    const title = 'Routines e2e: run and coalesce'
    await clearRoutines(page, title)
    const { agent, project, drop } = await scratch(page, 'Routines e2e run')
    const created = await page.request.post(`${WEB}/api/routines`, { data: { title, assignee_agent_id: agent, project_id: project } })
    const id = ((await created.json()) as { routine: Routine }).routine.id
    const runs = async () => ((await (await page.request.get(`${WEB}/api/routines/${id}/runs`)).json()) as { routine_runs: Run[] }).routine_runs

    try {
      await page.goto(`${WEB}/#/routines/${id}`)
      await page.getByRole('heading', { name: title }).waitFor()
      await page.getByRole('button', { name: 'Run', exact: true }).click()
      const open = page.getByRole('link', { name: /^Open / })
      await open.waitFor()
      const identifier = ((await open.textContent()) ?? '').replace('Open ', '')
      await page.waitForURL(new RegExp(`#/routines/${id}/runs$`))
      const list = page.getByRole('list', { name: 'Routine runs' })
      await list.getByRole('listitem').filter({ hasText: 'issue created' }).waitFor()
      expect('Runs shows the run as issue created', true)

      await open.click()
      await page.waitForURL(new RegExp(`#/issues/${identifier}$`))
      const assignee = page.locator('[data-property-row="Assignee"]')
      await assignee.waitFor()
      expect('the Execution Issue is assigned to the Agent', ((await assignee.textContent()) ?? '').includes('Routines e2e run agent'), await assignee.textContent())
      expect('the Execution Issue has its Routine row', (await page.locator('[data-property-row="Routine"]').getByRole('link', { name: title }).count()) === 1)

      // No Desktop runs the Agent, so its Run stays queued and the Issue live.
      await page.goto(`${WEB}/#/routines/${id}`)
      await page.getByRole('heading', { name: title }).waitFor()
      await page.getByRole('button', { name: 'Run', exact: true }).click()
      await page.getByText(`Coalesced into ${identifier}`).waitFor()
      await page.waitForURL(new RegExp(`#/routines/${id}/runs$`))
      const coalesced = list.getByRole('listitem').filter({ hasText: 'coalesced' })
      await coalesced.waitFor()
      expect('Run again coalesces into the same Issue', ((await coalesced.textContent()) ?? '').includes(identifier), await coalesced.textContent())
      const [second, first] = await runs()
      expect('the API holds both runs on one Issue', second?.status === 'coalesced' && first?.issue?.id === second.issue?.id, [second, first])

      const done = await page.request.patch(`${WEB}/api/issues/${first.issue!.id}`, { data: { status: 'done' } })
      expect('the Execution Issue is set done', done.ok(), done.status())
      await page.reload()
      await list.getByRole('listitem').filter({ hasText: 'completed' }).waitFor()
      expect('its Routine run shows Completed', (await runs()).find((r) => r.issue?.id === first.issue!.id && r.status === 'completed') !== undefined)
    } finally {
      await clearRoutines(page, title)
      await drop()
      await page.context().close()
    }
  },

  async pause() {
    const page = await signedIn()
    const title = 'Routines e2e: pause and archive'
    await clearRoutines(page, title)
    const { agent, project, drop } = await scratch(page, 'Routines e2e pause')
    const created = await page.request.post(`${WEB}/api/routines`, { data: { title, assignee_agent_id: agent, project_id: project } })
    const id = ((await created.json()) as { routine: Routine }).routine.id
    await page.request.post(`${WEB}/api/routines/${id}/triggers`, { data: { kind: 'schedule', cron_expression: '0 9 * * 1', timezone: 'Europe/Amsterdam' } })
    const routine = async () => ((await (await page.request.get(`${WEB}/api/routines/${id}`)).json()) as { routine: Routine }).routine

    try {
      await page.goto(`${WEB}/#/routines/${id}/triggers`)
      const card = page.getByRole('group', { name: 'Trigger: Schedule' })
      await card.getByText(/^Next: /).waitFor()
      await page.getByRole('switch', { name: 'Pause automatic triggers' }).click()
      await card.getByText('Not scheduled').waitFor()
      const paused = await routine()
      expect('the toggle pauses the Routine and clears the Next run', paused.status === 'paused' && paused.triggers[0]?.next_run_at === null, paused)
      await page.getByRole('switch', { name: 'Enable automatic triggers' }).click()
      await card.getByText(/^Next: /).waitFor()
      const resumed = await routine()
      expect('the toggle resumes it with a Next run', resumed.status === 'active' && resumed.triggers[0]?.next_run_at !== null, resumed)

      await page.goto(`${WEB}/#/routines`)
      const row = page.getByRole('list', { name: 'Routines' }).getByRole('listitem').filter({ hasText: title })
      await row.waitFor()
      await row.getByRole('button', { name: `More actions for ${title}` }).click()
      await page.getByRole('menuitem', { name: 'Archive' }).click()
      await page.getByRole('alertdialog').getByRole('button', { name: 'Archive' }).click()
      await row.waitFor({ state: 'detached' })
      expect('Archive takes the Routine out of the list', (await routine()).status === 'archived')

      await page.goto(`${WEB}/#/routines/${id}`)
      await page.getByRole('heading', { name: title }).waitFor()
      await page.getByText('Archived', { exact: true }).first().waitFor()
      expect(
        'an archived Routine is read-only: no Run, Edit or toggle',
        (await page.getByRole('button', { name: 'Run', exact: true }).count()) === 0 &&
          (await page.getByRole('button', { name: 'Edit routine' }).count()) === 0 &&
          (await page.getByRole('switch', { name: /automatic triggers/ }).count()) === 0,
      )
    } finally {
      await clearRoutines(page, title)
      await drop()
      await page.context().close()
    }
  },

  async schedule() {
    const page = await signedIn()
    const title = 'Routines e2e: every minute'
    await clearRoutines(page, title)
    const { agent, project, drop } = await scratch(page, 'Routines e2e schedule')
    const created = await page.request.post(`${WEB}/api/routines`, { data: { title, assignee_agent_id: agent, project_id: project } })
    const id = ((await created.json()) as { routine: Routine }).routine.id
    await page.request.post(`${WEB}/api/routines/${id}/triggers`, { data: { kind: 'schedule', cron_expression: '* * * * *', timezone: 'UTC' } })

    try {
      // The minute's tick plus the scheduler's 15 s ticker: within 90 s.
      await page.goto(`${WEB}/#/routines/runs`)
      const row = page.getByRole('list', { name: 'Recent Runs' }).getByRole('listitem').filter({ hasText: title }).first()
      const until = Date.now() + 90_000
      while (Date.now() < until && !(await row.isVisible())) {
        await page.waitForTimeout(5_000)
        await page.reload()
      }
      const text = (await row.textContent().catch(() => '')) ?? ''
      expect('the every-minute trigger fired by itself: Recent Runs shows a schedule run', text.includes('Schedule') || text.includes('schedule'), text)
    } finally {
      await clearRoutines(page, title)
      await drop()
      await page.context().close()
    }
  },

  async webhook() {
    const page = await signedIn()
    const title = 'Routines e2e: webhook'
    await clearRoutines(page, title)
    const { agent, project, drop } = await scratch(page, 'Routines e2e webhook')
    const created = await page.request.post(`${WEB}/api/routines`, { data: { title, assignee_agent_id: agent, project_id: project, concurrency_policy: 'always_enqueue' } })
    const id = ((await created.json()) as { routine: Routine }).routine.id
    const deliver = (url: string, secret: string, delivery: string) => {
      const body = JSON.stringify({ action: 'opened' })
      const sig = createHmac('sha256', secret).update(body).digest('hex')
      return page.request.post(url, {
        headers: { 'Content-Type': 'application/json', 'X-Hub-Signature-256': `sha256=${sig}`, 'X-GitHub-Delivery': delivery },
        data: body,
      })
    }

    try {
      await page.goto(`${WEB}/#/routines/${id}/triggers`)
      await page.getByRole('button', { name: 'Add webhook' }).click()
      const form = page.getByRole('group', { name: 'New trigger' })
      await form.getByLabel('Signing mode').click()
      await page.getByRole('option', { name: 'GitHub' }).click()
      await form.getByRole('button', { name: 'Add trigger' }).click()
      const reveal = page.getByRole('region', { name: 'Webhook secret' })
      await reveal.waitFor()
      const url = (await reveal.locator('[data-field="Webhook URL"]').textContent()) ?? ''
      const secret = (await reveal.locator('[data-field="Secret key"]').textContent()) ?? ''
      expect('the reveal shows the URL and a 48-hex secret', /\/api\/routine-triggers\/public\/[^/]+\/fire$/.test(url) && /^[0-9a-f]{48}$/.test(secret), { url, secret })
      expect('both have copy buttons', (await reveal.getByRole('button', { name: /^Copy / }).count()) === 2)
      await reveal.getByRole('button', { name: 'Done' }).click()
      await reveal.waitFor({ state: 'detached' })

      await page.reload()
      const card = page.getByRole('group', { name: 'Trigger: Webhook' })
      await card.waitFor()
      const cardText = (await card.textContent()) ?? ''
      expect('after a reload the card has the URL and no secret', cardText.includes(url) && !cardText.includes(secret) && cardText.includes('No deliveries yet'), cardText)

      const ok = await deliver(url, secret, `e2e-${Date.now()}`)
      expect('a signed delivery is accepted', ok.status() === 202, ok.status())
      await page.reload()
      await card.getByText(/^Last delivery: accepted/).waitFor()
      expect('the card says the last delivery was accepted', true)
      await page.getByRole('navigation', { name: 'Routine sections' }).getByRole('tab', { name: 'Runs' }).click()
      const runRow = page.getByRole('list', { name: 'Routine runs' }).getByRole('listitem').first()
      await runRow.waitFor()
      const runText = (await runRow.textContent()) ?? ''
      expect('Runs lists a Webhook run with its Issue', runText.includes('Webhook') && runText.includes('issue created') && runText.includes(title), runText)

      await page.getByRole('navigation', { name: 'Routine sections' }).getByRole('tab', { name: 'Triggers' }).click()
      await card.getByRole('button', { name: 'Rotate secret' }).click()
      await page.getByRole('alertdialog').getByRole('button', { name: 'Rotate secret' }).click()
      await reveal.waitFor()
      const rotated = (await reveal.locator('[data-field="Secret key"]').textContent()) ?? ''
      expect('rotating shows a new secret', /^[0-9a-f]{48}$/.test(rotated) && rotated !== secret, rotated)
      const old = await deliver(url, secret, `e2e-old-${Date.now()}`)
      expect('the old secret is 401', old.status() === 401, old.status())
      await page.reload()
      await card.getByText(/^Last delivery: rejected/).waitFor()
      expect('the card says the last delivery was rejected', true)
    } finally {
      await clearRoutines(page, title)
      await drop()
      await page.context().close()
    }
  },

  async variables() {
    const page = await signedIn()
    const title = 'Routines e2e: triage {{repo}} alert'
    await clearRoutines(page, title)
    const { agent, project, drop } = await scratch(page, 'Routines e2e variables')
    const created = await page.request.post(`${WEB}/api/routines`, { data: { title, assignee_agent_id: agent, project_id: project, concurrency_policy: 'always_enqueue' } })
    const id = ((await created.json()) as { routine: Routine }).routine.id

    try {
      await page.goto(`${WEB}/#/routines/${id}`)
      await page.getByRole('button', { name: 'Edit routine' }).click()
      const row = page.locator('[data-routine-variables] [data-routine-variable="repo"]')
      await row.waitFor()
      expect('the editor shows repo', true)
      await row.getByLabel('Type of repo').click()
      await page.getByRole('option', { name: 'select', exact: true }).click()
      await row.getByLabel('Options').fill('shop, blog')
      await row.getByLabel('Options').press('Tab')
      await row.getByLabel('Default of repo').click()
      await page.getByRole('option', { name: 'shop', exact: true }).click()
      await page.getByRole('button', { name: /^Save changes/ }).click()
      await page.locator('[data-routine-overview-mode="read"]').waitFor()
      const saved = ((await (await page.request.get(`${WEB}/api/routines/${id}`)).json()) as { routine: { variables: { name: string; type: string; options: string[]; default_value: unknown }[] } }).routine.variables
      expect('repo is saved as a Select of shop and blog, default shop', saved.length === 1 && saved[0].type === 'select' && saved[0].options.join() === 'shop,blog' && saved[0].default_value === 'shop', saved)

      await page.getByRole('button', { name: 'Run', exact: true }).click()
      const dialog = page.getByRole('dialog', { name: 'Run routine' })
      const field = dialog.getByLabel('repo', { exact: true })
      await field.waitFor()
      expect('the Run dialog has shop picked', ((await field.textContent()) ?? '').trim() === 'shop', await field.textContent())
      await field.click()
      await page.getByRole('option', { name: 'blog', exact: true }).click()
      await dialog.getByRole('button', { name: 'Run routine' }).click()
      const runRow = page.getByRole('list', { name: 'Routine runs' }).getByRole('listitem').first()
      await runRow.getByText(/triage blog alert/).waitFor()
      expect('Runs links an Issue "triage blog alert"', true)

      await page.request.patch(`${WEB}/api/routines/${id}`, { data: { variables: [{ ...saved[0], default_value: null }] } })
      await page.getByRole('navigation', { name: 'Routine sections' }).getByRole('tab', { name: 'Triggers' }).click()
      await page.getByRole('button', { name: 'Add schedule' }).click()
      await page.getByRole('group', { name: 'New trigger' }).getByRole('button', { name: 'Add trigger' }).click()
      await page.getByText(/require defaults for required variables: repo/).waitFor()
      expect('Add schedule is refused naming repo', true)
    } finally {
      await clearRoutines(page, title)
      await drop()
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
