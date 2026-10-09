// Costs and Budgets of the Guild's Agents' Runs, against Runs a headless
// Desktop runner finishes with the claude stand-in, one section each, every
// flow once in the dark theme at 1440×900:
//   api     a scratch Agent and a scratch Project; two Runs on an Issue in
//           the Project (its assignment's and Run) and one without an Issue
//           (Run heartbeat) run to the end; the
//           summary, by-agent and by-project routes over the range since
//           the section started add up the Runs' usage as GET /api/runs
//           shows it, the Project's row has two Runs and "No project" one;
//           a range after now is all zeros; from=bad and from ≥ to are 422;
//           the scratch Issue, Agent and Project are removed again
//   hard-stop  an agent Budget of 2 Runs warning at 50%: its first Run opens
//           a soft incident; with a Run and a Run heartbeat queued, the one
//           that finishes opens a hard incident with a pending
//           budget_override_required Approval (approve is 422), pauses the
//           Agent by budget and cancels the other with the reason; Run and
//           Resume are 422; a project Budget of 1 Run stops a second Agent's
//           Runs on that Project's Issue (422 naming it) and not on another
//           Project's; raising the agent Budget resumes the Agent, approves
//           the Approval and lets a Run start; deleting the stopped Project
//           cancels its Approval; the scratch data is removed again
//   resolve  an agent Budget of 1 Run reached by its assignment's Run: a
//           member without manage_budgets gets 403; raising to the Observed
//           amount, an unknown action and a raise without an amount are
//           422; raising to 2 resolves the incident, resumes the Agent,
//           approves the Approval and lets a Run start, whose finish opens
//           a new hard incident; keeping it paused dismisses that one,
//           rejects its Approval and leaves the Agent paused; resolving it
//           again is 422; the scratch data is removed again
//
//   bun e2e/costs.ts [section ...]   (task web:costs; needs task dev)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { spawn, type ChildProcess } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
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

const DESKTOP_DIR = new URL('../../desktop/', import.meta.url).pathname

/**
 * A headless Desktop runner for the owner, as in e2e/work.ts: `login`
 * approved through the API, then `runner` with the claude stand-in, in its
 * own process group. stop() ends it and signs its Desktop out again.
 */
async function desktopRunner(page: Page): Promise<{ stop: () => Promise<void> }> {
  const home = mkdtempSync(join(tmpdir(), 'bakery-costs-e2e-'))
  const env = {
    ...process.env,
    BAKERY_DESKTOP_HOME: home,
    BAKERY_CLAUDE: process.env.BAKERY_CLAUDE ?? join(DESKTOP_DIR, 'bin/claude-standin'),
  }
  const login = spawn('go', ['run', '.', 'login', '--server', WEB, '--no-browser'], { cwd: DESKTOP_DIR, env, stdio: ['ignore', 'pipe', 'inherit'] })
  let out = ''
  const link = await new Promise<URL>((resolve, reject) => {
    login.stdout!.on('data', (b: Buffer) => {
      out += b.toString()
      const m = out.match(/https?:\/\/\S+desktop-sign-in\S+/)
      if (m) resolve(new URL(m[0]))
    })
    login.on('exit', (code) => reject(new Error(`login exited ${code}: ${out}`)))
  })
  const [, id] = link.hash.match(/desktop-sign-in\/(\d+)/)!
  const token = new URLSearchParams(link.hash.split('?')[1]).get('token')
  const approved = await page.request.post(`${WEB}/api/desktop-sign-ins/${id}/approve`, { data: { token } })
  if (!approved.ok()) throw new Error(`approve the desktop: ${approved.status()}`)
  const { desktop_id } = (await approved.json()) as { desktop_id: number }
  await new Promise((resolve) => login.on('exit', resolve))
  const runner: ChildProcess = spawn('go', ['run', '.', 'runner'], { cwd: DESKTOP_DIR, env, detached: true, stdio: ['ignore', 'inherit', 'inherit'] })
  return {
    stop: async () => {
      try {
        process.kill(-runner.pid!, 'SIGTERM')
      } catch {}
      await page.request.delete(`${WEB}/api/desktops/${desktop_id}`)
      rmSync(home, { recursive: true, force: true })
    },
  }
}

type Usage = { input_tokens: number; cached_input_tokens: number; output_tokens: number; cost_equivalent_usd: number; duration_ms: number }
type Run = { id: number; status: string; started_at: string | null; finished_at: string | null; usage: Usage; issue: { id: number } | null }
type Figures = { input_tokens: number; cached_input_tokens: number; output_tokens: number; tokens: number; runs: number; run_time_ms: number; cost_equivalent_usd: number }

async function runsOf(page: Page, agent: number): Promise<Run[]> {
  return ((await (await page.request.get(`${WEB}/api/runs?agent=${agent}&limit=200`)).json()) as { runs: Run[] }).runs
}

/** Waits until the Agent has no queued or running Run left. */
async function settled(page: Page, agent: number) {
  for (let i = 0; i < 120; i++) {
    const rs = await runsOf(page, agent)
    if (rs.every((r) => r.status !== 'queued' && r.status !== 'running')) return
    await new Promise((r) => setTimeout(r, 1000))
  }
  throw new Error(`the runs of agent ${agent} did not finish`)
}

/** Adds up the claimed, finished Runs as Costs does. */
function added(rs: Run[]): Figures {
  const f: Figures = { input_tokens: 0, cached_input_tokens: 0, output_tokens: 0, tokens: 0, runs: 0, run_time_ms: 0, cost_equivalent_usd: 0 }
  for (const r of rs.filter((r) => r.started_at && r.finished_at)) {
    f.input_tokens += r.usage.input_tokens
    f.cached_input_tokens += r.usage.cached_input_tokens
    f.output_tokens += r.usage.output_tokens
    f.tokens += r.usage.input_tokens + r.usage.output_tokens
    f.runs++
    f.run_time_ms += r.usage.duration_ms
    f.cost_equivalent_usd += r.usage.cost_equivalent_usd
  }
  return f
}

function same(a: Figures, b: Figures): boolean {
  const keys = ['input_tokens', 'cached_input_tokens', 'output_tokens', 'tokens', 'runs', 'run_time_ms'] as const
  return keys.every((k) => a[k] === b[k]) && Math.abs(a.cost_equivalent_usd - b.cost_equivalent_usd) < 1e-6
}

type Agent = { id: number; status: string; pause_reason: string | null }
type Incident = { id: number; threshold: string; status: string; approval_id: number | null; scope: { type: string; id: number; name: string } }
type Approval = { id: number; type: string; status: string; payload: { scope_name: string; threshold: string; guidance: string } }

async function hire(page: Page, name: string): Promise<number> {
  const hired = (await (await page.request.post(`${WEB}/api/agents`, { data: { name, job: 'engineer', icon: 'bot' } })).json()) as { agent: { id: number; approval_id: number } }
  await page.request.post(`${WEB}/api/approvals/${hired.agent.approval_id}/approve`, { data: {} })
  return hired.agent.id
}

async function issueFor(page: Page, title: string, project: number, agent: number): Promise<number> {
  const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, project_id: project, status: 'todo' } })).json()) as { issue: { id: number } }
  await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { assignee_agent_id: agent } })
  return issue.id
}

async function incidents(page: Page, budget: { type: string; id: number }): Promise<Incident[]> {
  const o = (await (await page.request.get(`${WEB}/api/budgets/overview`)).json()) as { incidents: Incident[] }
  return o.incidents.filter((i) => i.scope.type === budget.type && i.scope.id === budget.id)
}

async function approval(page: Page, id: number): Promise<Approval> {
  return ((await (await page.request.get(`${WEB}/api/approvals/${id}`)).json()) as { approval: Approval }).approval
}

async function agentOf(page: Page, id: number): Promise<Agent> {
  return ((await (await page.request.get(`${WEB}/api/agents/${id}`)).json()) as { agent: Agent }).agent
}

/** A person invited as role for the run, signed in in a context of their own; leave removes them. */
async function invited(owner: Page, role: 'viewer' | 'member'): Promise<{ page: Page; leave: () => Promise<void> }> {
  const { members } = (await (await owner.request.get(`${WEB}/api/members`)).json()) as { members: { id: number; email: string }[] }
  for (const m of members.filter((m) => m.email.startsWith(`costs-${role}-`))) await owner.request.delete(`${WEB}/api/members/${m.id}`)
  const inv = await owner.request.post(`${WEB}/api/invitations`, { data: { email: `costs-${role}-${Date.now()}@example.test`, role } })
  if (!inv.ok()) throw new Error(`invite: ${inv.status()}`)
  const token = ((await inv.json()) as { path: string }).path.split('/').pop()
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  const accept = await ctx.request.post(`${WEB}/api/invitations/by-token/${token}/accept`, { data: { name: `Costs ${role}`, password: 'a long enough password' } })
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

const sections: Record<string, () => Promise<void>> = {
  async api() {
    const page = await signedIn()
    const name = 'Costs e2e agent'
    const projectName = 'Costs e2e project'
    const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
    for (const a of agents.filter((a) => a.name === name)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
    const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
    for (const p of projects.filter((p) => p.name === projectName)) await page.request.delete(`${WEB}/api/projects/${p.id}`)

    // The range reaches a second back: let a Run another section just
    // finished fall out of it.
    await new Promise((r) => setTimeout(r, 2000))
    // Whole seconds, so the range starts before the first Run finishes.
    const from = new Date(Math.floor(Date.now() / 1000) * 1000 - 1000).toISOString()
    const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: projectName } })).json()) as { project: { id: number } }
    const hire = (await (await page.request.post(`${WEB}/api/agents`, { data: { name, job: 'engineer', icon: 'bot' } })).json()) as { agent: { id: number; approval_id: number } }
    await page.request.post(`${WEB}/api/approvals/${hire.agent.approval_id}/approve`, { data: {} })
    const agent = hire.agent.id
    const { issue } = (await (
      await page.request.post(`${WEB}/api/issues`, { data: { title: 'Costs e2e: say hello', project_id: project.id, status: 'todo' } })
    ).json()) as { issue: { id: number } }
    await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { assignee_agent_id: agent } })

    const desktop = await desktopRunner(page)
    try {
      // The assignment queued a Run already; Run on the Issue once more,
      // then Run heartbeat for one without an Issue.
      for (const path of ['runs', 'heartbeat']) {
        await settled(page, agent)
        const started = await page.request.post(`${WEB}/api/agents/${agent}/${path}`, { data: path === 'runs' ? { issue_id: issue.id } : {} })
        if (!started.ok()) throw new Error(`start a run: ${started.status()} ${await started.text()}`)
        await settled(page, agent)
      }
      const rs = (await runsOf(page, agent)).filter((r) => r.finished_at && r.finished_at >= from)
      const onIssue = rs.filter((r) => r.issue?.id === issue.id && r.started_at)
      const without = rs.filter((r) => !r.issue && r.started_at)
      expect('two or more Runs on the Issue and one without finished', onIssue.length >= 2 && without.length === 1, rs.map((r) => [r.status, r.issue?.id]))

      const range = `from=${encodeURIComponent(from)}`
      const summary = (await (await page.request.get(`${WEB}/api/costs/summary?${range}`)).json()) as Figures & { from: string }
      // Only the scratch Agent ran since the section started.
      expect("the summary adds up the Runs' usage", same(summary, added(rs)), { summary, runs: added(rs) })
      expect('it used tokens', summary.tokens > 0, summary)
      const byAgent = (await (await page.request.get(`${WEB}/api/costs/by-agent?${range}`)).json()) as { agents: (Figures & { agent: { id: number; name: string; status: string } })[] }
      const mine = byAgent.agents.find((a) => a.agent.id === agent)
      expect('by-agent has the Agent with the same figures', !!mine && mine.agent.name === name && same(mine, added(rs)), byAgent)
      const byProject = (await (await page.request.get(`${WEB}/api/costs/by-project?${range}`)).json()) as { projects: (Figures & { project: { id: number; name: string } | null })[] }
      const row = byProject.projects.find((p) => p.project?.id === project.id)
      expect('the Project row has the Runs on its Issue', row?.project?.name === projectName && row.runs === onIssue.length && same(row, added(onIssue)), byProject)
      const none = byProject.projects.find((p) => p.project === null)
      expect('"No project" has the Run without an Issue', none?.runs === 1 && same(none, added(without)), byProject)

      const later = encodeURIComponent(new Date(Date.now() + 3_600_000).toISOString())
      const empty = (await (await page.request.get(`${WEB}/api/costs/summary?from=${later}`)).json()) as Figures
      expect('a range after now is all zeros', empty.runs === 0 && empty.tokens === 0 && empty.run_time_ms === 0, empty)
      expect('from=bad is 422', (await page.request.get(`${WEB}/api/costs/summary?from=bad`)).status() === 422)
      expect('from after to is 422', (await page.request.get(`${WEB}/api/costs/by-agent?from=${later}&to=${encodeURIComponent(from)}`)).status() === 422)
    } finally {
      await desktop.stop()
      await page.request.delete(`${WEB}/api/issues/${issue.id}`)
      await page.request.post(`${WEB}/api/agents/${agent}/terminate`)
      await page.request.delete(`${WEB}/api/projects/${project.id}`)
      await page.close()
    }
  },
}

sections['hard-stop'] = async () => {
  const page = await signedIn()
  const names = ['Hard stop e2e agent', 'Hard stop e2e second agent']
  const projectNames = ['Hard stop e2e shop', 'Hard stop e2e lab']
  const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
  for (const a of agents.filter((a) => names.includes(a.name))) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
  const listed = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
  for (const p of listed.projects.filter((p) => projectNames.includes(p.name))) await page.request.delete(`${WEB}/api/projects/${p.id}`)

  const projects: number[] = []
  for (const name of projectNames)
    projects.push(((await (await page.request.post(`${WEB}/api/projects`, { data: { name } })).json()) as { project: { id: number } }).project.id)
  const [shop, lab] = projects
  const ada = await hire(page, names[0])
  const bob = await hire(page, names[1])
  const issues: number[] = []
  let desktop: { stop: () => Promise<void> } | undefined
  try {
    const set = await page.request.put(`${WEB}/api/budgets`, { data: { scope_type: 'agent', scope_id: ada, metric: 'runs', amount: 2, warn_percent: 50 } })
    expect('the agent Budget is set', set.ok(), await set.text())
    const adaScope = { type: 'agent', id: ada }

    // Run 1: the assignment's Run.
    issues.push(await issueFor(page, 'Hard stop e2e: say hello', shop, ada))
    desktop = await desktopRunner(page)
    await settled(page, ada)
    await desktop.stop()
    desktop = undefined
    let open = await incidents(page, adaScope)
    expect('the first Run opens a soft incident', open.length === 1 && open[0].threshold === 'soft', open)

    // Runs 2 and 3 queued while no Desktop is online: the one claimed
    // first reaches the Hard stop and the other is cancelled.
    for (const path of ['runs', 'heartbeat']) {
      const started = await page.request.post(`${WEB}/api/agents/${ada}/${path}`, { data: path === 'runs' ? { issue_id: issues[0] } : {} })
      expect(`${path} queues a Run`, started.ok(), await started.text())
    }
    desktop = await desktopRunner(page)
    await settled(page, ada)
    const rs = await runsOf(page, ada)
    const cancelled = rs.filter((r) => r.status === 'cancelled') as (Run & { error: string | null })[]
    expect('two Runs succeeded', rs.filter((r) => r.status === 'succeeded').length === 2, rs.map((r) => r.status))
    expect("the queued Run is cancelled with the budget's reason", cancelled.length === 1 && cancelled[0].error === "Cancelled because the budget's hard stop was reached.", cancelled)
    open = await incidents(page, adaScope)
    const hard = open.find((i) => i.threshold === 'hard')
    expect('a hard incident replaces the soft one', open.length === 1 && !!hard && hard.approval_id !== null, open)
    const override = await approval(page, hard!.approval_id!)
    expect('its Approval is a pending budget_override_required', override.type === 'budget_override_required' && override.status === 'pending' && override.payload.scope_name === names[0], override)
    expect('the generic approve is 422', (await page.request.post(`${WEB}/api/approvals/${override.id}/approve`, { data: {} })).status() === 422)
    const paused = await agentOf(page, ada)
    expect('the Agent is paused by budget', paused.status === 'paused' && paused.pause_reason === 'budget', paused)
    expect('Run is 422', (await page.request.post(`${WEB}/api/agents/${ada}/runs`, { data: { issue_id: issues[0] } })).status() === 422)
    const resume = await page.request.post(`${WEB}/api/agents/${ada}/resume`)
    expect('Resume is 422 while the Budget is exceeded', resume.status() === 422 && (await resume.text()).includes('budget still exceeded'))

    // A project Budget stops that Project's Runs only.
    const labSet = await page.request.put(`${WEB}/api/budgets`, { data: { scope_type: 'project', scope_id: lab, metric: 'runs', amount: 1 } })
    expect('the project Budget is set', labSet.ok(), await labSet.text())
    issues.push(await issueFor(page, 'Hard stop e2e: in the lab', lab, bob))
    await settled(page, bob)
    const labStop = await incidents(page, { type: 'project', id: lab })
    expect("the lab's Run reaches its Hard stop", labStop.length === 1 && labStop[0].threshold === 'hard', labStop)
    expect('Bob is not paused', (await agentOf(page, bob)).status === 'idle')
    const refused = await page.request.post(`${WEB}/api/agents/${bob}/runs`, { data: { issue_id: issues[1] } })
    const body = (await refused.json()) as { message: string; scope: { type: string; name: string } }
    expect('a Run on the lab is 422 naming it', refused.status() === 422 && body.scope?.type === 'project' && body.scope?.name === projectNames[1], body)
    issues.push(await issueFor(page, 'Hard stop e2e: in the shop', shop, bob))
    const allowed = await page.request.post(`${WEB}/api/agents/${bob}/runs`, { data: { issue_id: issues[2] } })
    expect("a Run on another Project's Issue starts", allowed.ok(), await allowed.text())
    await settled(page, bob)

    // Raising the agent Budget lifts its Hard stop.
    const raised = await page.request.put(`${WEB}/api/budgets`, { data: { scope_type: 'agent', scope_id: ada, metric: 'runs', amount: 5, warn_percent: 50 } })
    expect('the raised Budget is ok', raised.ok() && ((await raised.json()) as { budget: { status: string } }).budget.status !== 'hard_stop')
    const resumed = await agentOf(page, ada)
    expect('the Agent is resumed', resumed.status === 'idle' && resumed.pause_reason === null, resumed)
    expect('the Approval is approved', (await approval(page, override.id)).status === 'approved')
    expect('no incident of the Agent is open', (await incidents(page, adaScope)).length === 0)
    const again = await page.request.post(`${WEB}/api/agents/${ada}/runs`, { data: { issue_id: issues[0] } })
    expect('a Run starts again', again.status() === 201, await again.text())
    await settled(page, ada)

    // Deleting the stopped Project cancels its waiting Approval.
    await page.request.delete(`${WEB}/api/issues/${issues[1]}`)
    await page.request.delete(`${WEB}/api/projects/${lab}`)
    projects.pop()
    expect("the lab's Approval is cancelled", (await approval(page, labStop[0].approval_id!)).status === 'cancelled')
  } finally {
    await desktop?.stop()
    for (const i of issues) await page.request.delete(`${WEB}/api/issues/${i}`)
    for (const a of [ada, bob]) await page.request.post(`${WEB}/api/agents/${a}/terminate`)
    for (const p of projects) await page.request.delete(`${WEB}/api/projects/${p}`)
    await page.close()
  }
}

sections.resolve = async () => {
  const page = await signedIn()
  const name = 'Resolve e2e agent'
  const projectName = 'Resolve e2e project'
  const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
  for (const a of agents.filter((a) => a.name === name)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
  const listed = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
  for (const p of listed.projects.filter((p) => p.name === projectName)) await page.request.delete(`${WEB}/api/projects/${p.id}`)

  const project = ((await (await page.request.post(`${WEB}/api/projects`, { data: { name: projectName } })).json()) as { project: { id: number } }).project.id
  const ada = await hire(page, name)
  const scope = { type: 'agent', id: ada }
  const resolve = (id: number, data: Record<string, unknown>, as: Page = page) =>
    as.request.post(`${WEB}/api/budget-incidents/${id}/resolve`, { data })
  let issue = 0
  let desktop: { stop: () => Promise<void> } | undefined
  let member: { page: Page; leave: () => Promise<void> } | undefined
  try {
    const set = await page.request.put(`${WEB}/api/budgets`, { data: { scope_type: 'agent', scope_id: ada, metric: 'runs', amount: 1 } })
    expect('the agent Budget is set', set.ok(), await set.text())
    issue = await issueFor(page, 'Resolve e2e: say hello', project, ada)
    desktop = await desktopRunner(page)
    await settled(page, ada)
    let hard = (await incidents(page, scope)).find((i) => i.threshold === 'hard')
    expect('the first Run reaches the Hard stop', !!hard && hard.status === 'open', hard)

    member = await invited(page, 'member')
    expect('a member without manage_budgets gets 403', (await resolve(hard!.id, { action: 'keep_paused' }, member.page)).status() === 403)
    const equal = await resolve(hard!.id, { action: 'raise_budget_and_resume', amount: 1 })
    expect('raising to the Observed amount is 422', equal.status() === 422 && (await equal.text()).includes('must exceed the observed amount'))
    expect('an unknown action is 422', (await resolve(hard!.id, { action: 'shrug' })).status() === 422)
    expect('a raise without an amount is 422', (await resolve(hard!.id, { action: 'raise_budget_and_resume' })).status() === 422)

    const raised = await resolve(hard!.id, { action: 'raise_budget_and_resume', amount: 2, decision_note: 'one more' })
    const resolved = ((await raised.json()) as { incident: Incident }).incident
    expect('raising answers the resolved incident', raised.ok() && resolved.id === hard!.id && resolved.status === 'resolved', resolved)
    const resumed = await agentOf(page, ada)
    expect('the Agent is idle', resumed.status === 'idle' && resumed.pause_reason === null, resumed)
    expect('the Approval is approved', (await approval(page, hard!.approval_id!)).status === 'approved')
    const again = await page.request.post(`${WEB}/api/agents/${ada}/runs`, { data: { issue_id: issue } })
    expect('a new Run starts', again.status() === 201, await again.text())
    await settled(page, ada)

    const second = (await incidents(page, scope)).find((i) => i.threshold === 'hard')
    expect('reaching the raised Budget opens a new hard incident', !!second && second.id !== hard!.id && second.approval_id !== hard!.approval_id, second)
    hard = second!
    const kept = await resolve(hard.id, { action: 'keep_paused', decision_note: 'enough for now' })
    const dismissed = ((await kept.json()) as { incident: Incident }).incident
    expect('keeping it paused dismisses the incident', kept.ok() && dismissed.status === 'dismissed', dismissed)
    expect('the Approval is rejected', (await approval(page, hard.approval_id!)).status === 'rejected')
    const paused = await agentOf(page, ada)
    expect('the Agent stays paused by budget', paused.status === 'paused' && paused.pause_reason === 'budget', paused)
    expect('Run is still 422', (await page.request.post(`${WEB}/api/agents/${ada}/runs`, { data: { issue_id: issue } })).status() === 422)
    expect('resolving it again is 422', (await resolve(hard.id, { action: 'keep_paused' })).status() === 422)
  } finally {
    await desktop?.stop()
    await member?.leave()
    if (issue) await page.request.delete(`${WEB}/api/issues/${issue}`)
    await page.request.post(`${WEB}/api/agents/${ada}/terminate`)
    await page.request.delete(`${WEB}/api/projects/${project}`)
    await page.close()
  }
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
console.log('the costs flows work')
