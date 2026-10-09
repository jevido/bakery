// The Guild's work pages (Goals, Issues and the Issue page) in
// headless Chromium, one section each, every flow once in the dark theme at
// 1440×900 (the light theme and phone width wait for the guilds goal's final
// sweep):
//   goals   the Goals page shows its empty state or its tree; New Goal
//           creates "Ship guilds" and its page a Sub-goal under it; the tree
//           shows both; the Goal page edits the title in place and changes
//           the status; a Markdown description renders a heading and a link
//           (opening in a new tab) and no <script>; the scratch Goals are
//           deleted again, the last one through the page's Delete
//   issues  New Issue creates two Issues (one in a scratch Project, one
//           assigned to "Me" at priority high); both rows show their
//           identifier, icons and Assignee; search by an identifier finds
//           one; the status filter todo hides a backlog one and survives a
//           reload; the Project filter keeps only the first; grouping shows
//           the counts; the scratch Issues and Project are deleted again
//   issue   a scratch Issue opens by its identifier; its title is renamed in
//           place and a reload keeps it; status In Progress shows the
//           Started time, priority and the Assignee and Goal change from the
//           properties panel; a scratch Project's Application is picked as
//           the Issue's Application and let go of when the Issue leaves the
//           Project; a scratch Agent picked as the Assignee shows its
//           name and icon, and terminating it unassigns the Issue; Add sub-issue creates one under it, listed and
//           opening its own page with the parent link; a Markdown comment
//           with a code block is posted, edited and deleted (the placeholder
//           stays); the Goal's page and the Issues list open it from
//           their rows; #/issues/DEF-99999 shows the not-found state; the scratch
//           Issues, Goal and Project are deleted again
//   viewer  a Viewer, invited for the run and removed again, sees the Issues
//           list, Goals and an Issue's page but no New Issue, New Goal,
//           composer, pickers, Add sub-issue or Delete, and the API answers
//           403 to their Issue and Comment
//   runs    an Issue assigned to a scratch Agent the owner hired: Run queues
//           a Run that waits for the owner's desktop; a headless Desktop
//           runner (apps/desktop `runner`, connected through `login` and
//           running the claude stand-in) claims it, the Transcript grows
//           live on the page, the Run ends Succeeded in Runs with its result
//           footer and tokens, and the Agent page lists it; a [slow] Issue's
//           Run is cancelled from the page and ends Cancelled
//   wakes   a scratch Issue in todo assigned to a scratch Agent lists a
//           queued "Assignment" Run without Run being pressed; a comment
//           joins it ("×2", still one Run); after it is cancelled, another
//           comment queues an "Automation" Run
//   hidden  a Member, invited for the run, sees an Issue in a scratch
//           Project until the Project's permissions page denies the Member
//           Role View resources there: then it is gone from the list, by its
//           identifier (404), on its page, from the Activity feed and its own
//           Activity (404); removing the override shows it again
//   blockers  three scratch Issues: on the second, "Blocked by" picks the
//           first through the search picker; the notice names it and the
//           Issues list marks the row blocked; the first lists the second
//           under "Blocking"; picking the second as the first's Blocker
//           shows the cycle error; once the first is done the notice is
//           gone; × removes the Blocker; a Viewer sees both rows without
//           the picker or ×
//   documents  on a scratch Issue, New document "plan" is created (rev 1),
//           edited and saved (rev 2); History opens rev 1 read-only,
//           Compare shows a removed and an added line, Restore makes rev 3
//           with rev 1's text; a second page saves first, so the first's
//           save shows the conflict notice and Reload shows rev 4; Download
//           gives plan.md; a Viewer sees the document and History without
//           Edit or Delete; Delete removes it
//   activity  the sidebar shows Guild → Activity; after a scratch Issue is
//           created and commented on, #/activity lists "<you> commented on
//           <title> DEF-n" first; the Goals filter hides it and a reload
//           keeps entity=goal; clicking the Issue's row opens the Issue;
//           its Activity tab, after a status, priority, Blocker and document
//           change, lists those and the comment oldest first in Paperclip's
//           words, ?tab=activity
//           opens on it after a reload, and the Comments tab still posts;
//           the Actor filter set to you keeps the Issue; a Viewer, invited
//           for the run, sees the feed but not an Issue in a scratch Project
//           whose Viewer Role is denied View resources there
//   inbox   with no last tab kept, #/inbox opens Mine, listing a scratch
//           Issue the owner created; after a Member, invited for the run,
//           comments on it the row shows the unread dot; clicking it clears
//           the dot and a reload keeps it cleared; Archive takes the row out
//           of Mine and the toast's Undo brings it back; archived again,
//           Recent shows it dimmed with Unarchive; Unread lists only a second
//           scratch Issue the Member commented on, and "Mark all as read"
//           empties it; on Recent, search by the second's identifier
//           keeps only it; #/inbox after Recent opens Recent; after the
//           Member comments on the second again the sidebar's Inbox shows 1,
//           and on the rail a dot; opening that Issue clears both, and
//           /api/sidebar-badges agrees; after the Member sets the archived
//           first to In Review it is back on Mine; an Issue in a scratch
//           Project whose Member Role is denied View resources, assigned to
//           the Member, is on neither their Mine nor Recent
//   approvals  #/approvals opens Pending, listing a Board Approval made
//           through the API for a scratch Issue, with the count; Approve on
//           its card opens its page with "Approval confirmed" and "Review
//           linked issue"; All lists it approved and Pending no longer does;
//           a second one gets Request revision with a Decision note, which
//           its page shows, a comment, and Resubmit opens the request
//           dialog prefilled and makes it pending again with the edited
//           title; on the scratch Issue's page "Request approval" in the
//           More actions menu asks for a third through the dialog, whose
//           card appears above the tabs and is approved there; and
//           #/activity?entity=approval lists approval.created and
//           approval.approved, linking to the Approval page; a fourth,
//           pending, is counted by /api/sidebar-badges and the sidebar's
//           Inbox badge and has its row on Mine and Unread; Approve on that
//           row takes it off Unread and the badge drops; the Dashboard's
//           "Pending Approvals" card shows the count and opens
//           #/approvals/pending; the first's Linked issue links back to the
//           Issue and the resubmitted second is rejected from its card on
//           Pending; a Member, invited for the run, sees a fifth on Pending
//           and its page without Approve, Reject or Request revision,
//           comments on it, and does not see its Linked issue in a scratch
//           Project whose Member Role is denied View resources
//   agent-actor  an Agent acting through its Run key without a Runner: a
//           scratch Agent's Run is claimed with a Desktop key minted as the
//           Desktop app does, and the Run key checks the Issue out, writes a
//           Comment and saves a plan document; the Issue page shows "Checked
//           out by" the Agent, the Comment under the Agent's icon and name
//           (linking to its page, no Edit or Delete), the revision by it,
//           and the Activity tab and the Guild's Activity (filtered to
//           agent:<id>) name it; after the Run finishes, the Checkout row
//           is gone
//   agent-api  the whole path through the MCP server: a scratch Agent with
//           the Member Role is assigned an Issue whose description asks the
//           claude stand-in for [mcp bakeryCheckoutIssue] and
//           [mcp bakeryAddComment] with a [slow] tail; Run and the headless
//           Desktop runner start it with The Bakery's MCP server; while it
//           runs the Issue is In progress and "Checked out by" the Agent, the
//           Comment is in the thread under the Agent's name and the Activity
//           names it; the Transcript shows both tool calls; once the Run is
//           final the Checkout row is gone
//   work-products  a scratch Issue's page has no Work products section;
//           with a Pull request (open, opened by the owner) and its Preview
//           (deploying) written straight into the dev Postgres, the page
//           shows both cards with their pills, the Pull request's number,
//           title, git host and link opening in a new tab, the Preview's
//           link and its Preview deployments link; merged and ready, a
//           reload shows Merged and Ready
//   pull-request  the whole path against the Forgejo stand-in, started for
//           the run and removed again: an Application on a public Forgejo
//           repository deploys with Previews on; a scratch Agent's Issue
//           names it and asks the claude stand-in to check out, commit
//           index.html, push and open the Pull request through the MCP
//           server; with Forgejo credentials only in the Runner's
//           environment, the assignment's Run succeeds, Forgejo has the
//           Pull request from bakery/<identifier>, the Issue page's
//           Pull request card says Open and the Preview card Deploying,
//           then Ready, its address serving the Agent's change; closing
//           the Pull request on Forgejo turns them Closed and Removed
//
//   bun e2e/work.ts [section ...]   (task web:work; needs task dev)
//
// The same environment as e2e/walk.ts overrides what it uses.
import { spawnSync } from 'node:child_process'
import { createHash, randomBytes } from 'node:crypto'
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { chromium, type Page } from 'playwright-core'
import { desktopRunner as sharedRunner } from './runner.ts'

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

async function signedIn(width = 1440, height = 900, theme: 'dark' | 'light' = 'dark'): Promise<Page> {
  const ctx = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' })
  await ctx.addInitScript((t) => localStorage.setItem('theme', t), theme)
  const login = await ctx.request.post(`${WEB}/api/login`, { data: who })
  if (!login.ok()) throw new Error(`sign in: ${login.status()} (is task dev running?)`)
  return ctx.newPage()
}

type Goal = { id: number; title: string; status: string; parent_id: number | null; description: string }
async function goals(page: Page): Promise<Goal[]> {
  return ((await (await page.request.get(`${WEB}/api/goals`)).json()) as { goals: Goal[] }).goals
}

const scratch = ['Ship guilds', 'Ship guilds now', 'Guild rail']

/** Deletes the scratch Goals a run before may have left, Sub-goals first. */
async function clean(page: Page) {
  const left = (await goals(page)).filter((g) => scratch.includes(g.title))
  left.sort((a, b) => (b.parent_id ?? 0) - (a.parent_id ?? 0))
  for (const g of left) await page.request.delete(`${WEB}/api/goals/${g.id}`)
}

type Issue = { id: number; identifier: string; title: string; status: string; priority: string; assignee: { id: number; name: string; kind: 'member' | 'agent' } | null; project: { id: number } | null; application?: { id: number; name: string } | null }
async function issues(page: Page): Promise<Issue[]> {
  return ((await (await page.request.get(`${WEB}/api/issues`)).json()) as { issues: Issue[] }).issues
}

const scratchIssues = ['Wire the guild rail', 'Port the Issues list']
const scratchProject = 'Work e2e scratch'

/** Deletes the scratch Issues and Project a run before may have left. */
async function cleanIssues(page: Page) {
  for (const i of (await issues(page)).filter((i) => scratchIssues.includes(i.title))) await page.request.delete(`${WEB}/api/issues/${i.id}`)
  const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
  for (const p of projects.filter((p) => p.name === scratchProject)) await page.request.delete(`${WEB}/api/projects/${p.id}`)
}

/** The shared runner, slow enough that the Transcript is seen growing. */
const desktopRunner = (page: Page, extra: Record<string, string> = {}) =>
  sharedRunner(page, { BAKERY_STANDIN_DELAY: process.env.BAKERY_STANDIN_DELAY ?? '1s', ...extra })

/** Picks an option of a dialog chip or popover by its accessible names. */
async function pick(page: Page, scope: ReturnType<Page['getByRole']>, chip: string, option: string) {
  await scope.getByRole('button', { name: chip, exact: true }).click()
  await page.getByRole('listbox', { name: chip }).getByRole('option', { name: option, exact: true }).click()
}


/**
 * A real Member of the owner's Current guild through an Invitation with
 * the seeded Role `role`, signed in in a context of its own. `leave`
 * removes the Membership again.
 */
async function invited(owner: Page, role: 'viewer' | 'member'): Promise<{ page: Page; id: number; leave: () => Promise<void> }> {
  const { members } = (await (await owner.request.get(`${WEB}/api/members`)).json()) as { members: { id: number; email: string }[] }
  for (const m of members.filter((m) => m.email.startsWith(`work-${role}-`))) await owner.request.delete(`${WEB}/api/members/${m.id}`)
  const inv = await owner.request.post(`${WEB}/api/invitations`, { data: { email: `work-${role}-${Date.now()}@example.test`, role } })
  if (!inv.ok()) throw new Error(`invite: ${inv.status()}`)
  const token = ((await inv.json()) as { path: string }).path.split('/').pop()
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  const accept = await ctx.request.post(`${WEB}/api/invitations/by-token/${token}/accept`, {
    data: { name: `Work ${role}`, password: 'a long enough password' },
  })
  if (!accept.ok()) throw new Error(`accept: ${accept.status()} ${await accept.text()}`)
  const { member } = (await accept.json()) as { member: { id: number } }
  return {
    page: await ctx.newPage(),
    id: member.id,
    leave: async () => {
      await ctx.close()
      await owner.request.delete(`${WEB}/api/members/${member.id}`)
    },
  }
}

/**
 * Denies the seeded Role `role` View resources in a Project through its
 * permissions page; the returned function removes the override again.
 */
async function denyView(page: Page, project: number, role: 'Member' | 'Viewer'): Promise<() => Promise<void>> {
  const { roles } = (await (await page.request.get(`${WEB}/api/roles`)).json()) as { roles: { id: number; name: string }[] }
  const target = roles.find((r) => r.name === role)!
  await page.goto(`${WEB}/#/project/${project}/permissions`)
  await page.getByTestId('project-permissions').waitFor()
  await page.getByRole('button', { name: 'Add role or member' }).click()
  await page.locator(`[data-testid="add-target"][data-target="role:${target.id}"]`).click()
  await page.locator('[data-permission="view_resources"]').getByRole('radio', { name: 'Deny' }).click()
  await page.getByRole('button', { name: 'Save changes' }).click()
  await page.getByRole('button', { name: 'Remove override' }).waitFor()
  return async () => {
    await page.goto(`${WEB}/#/project/${project}/permissions`)
    // The page opens on its first target, which need not be this Role's.
    await page.getByTestId('override-target').filter({ hasText: new RegExp(`^\\s*${role}\\s+Role\\s*$`) }).click()
    await page.getByRole('button', { name: 'Remove override' }).click()
    await page.getByRole('button', { name: 'Remove', exact: true }).click()
    await page.getByRole('button', { name: 'Remove override' }).waitFor({ state: 'detached' })
  }
}

/**
 * Runs SQL in the dev Postgres, for state no API path reaches without a git
 * host (Work products come only from Forgejo and the Previews).
 */
function sql(query: string): string {
  const container = process.env.BAKERY_POSTGRES_CONTAINER ?? 'bakery-dev-postgres-1'
  const r = spawnSync('podman', ['exec', '-i', container, 'psql', '-U', 'bakery', '-d', 'bakery', '-tAq', '-v', 'ON_ERROR_STOP=1', '-c', query])
  if (r.status !== 0) throw new Error(`psql: ${r.stderr.toString()}`)
  return r.stdout.toString().trim()
}

const ROOT = new URL('../../../', import.meta.url).pathname
const FORGEJO = 'http://127.0.0.1:4950'
const FORGEJO_COMPOSE = ['compose', '-f', join(ROOT, 'infra/dev/compose.yml'), '--profile', 'git']

/** Runs a command, throwing with its stderr when it fails. */
function run(cmd: string, args: string[], cwd?: string): string {
  const r = spawnSync(cmd, args, { cwd })
  if (r.status !== 0) throw new Error(`${cmd} ${args.slice(0, 3).join(' ')}: ${r.stderr.toString()}`)
  return r.stdout.toString().trim()
}

/** Polls check every 2 s until it answers true or seconds pass. */
async function waitFor(seconds: number, what: string, check: () => Promise<boolean> | boolean) {
  const until = Date.now() + seconds * 1000
  while (!(await check())) {
    if (Date.now() > until) throw new Error(`timed out waiting for ${what}`)
    await new Promise((r) => setTimeout(r, 2000))
  }
}

/**
 * The Forgejo stand-in (compose profile git) with an admin and an access
 * token, as infra/dev/lib/e2e.sh's start_forgejo makes them. down() removes
 * the container and its volumes again.
 */
async function forgejoUp(name: string) {
  const user = 'bakery'
  run('podman', [...FORGEJO_COMPOSE, 'up', '-d', 'forgejo'])
  await waitFor(120, 'Forgejo', () => fetch(`${FORGEJO}/api/healthz`).then((r) => r.ok, () => false))
  const password = randomBytes(18).toString('hex')
  const exec = ['podman', ...FORGEJO_COMPOSE, 'exec', '-T', 'forgejo', 'forgejo', 'admin', 'user']
  const made = spawnSync(exec[0], [...exec.slice(1), 'create', '--admin', '--username', user, '--password', password, '--email', 'bakery@example.test', '--must-change-password=false'])
  if (made.status !== 0) run(exec[0], [...exec.slice(1), 'change-password', '--username', user, '--password', password, '--must-change-password=false'])
  const token = run(exec[0], [...exec.slice(1), 'generate-access-token', '--username', user, '--token-name', name, '--scopes', 'all', '--raw'])
  const api = async (method: string, path: string, body?: object) => {
    const r = await fetch(`${FORGEJO}/api/v1${path}`, {
      method,
      headers: { Authorization: `token ${token}`, 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined,
    })
    if (!r.ok) throw new Error(`Forgejo ${method} ${path}: ${r.status} ${await r.text()}`)
    return r.status === 204 ? {} : r.json()
  }
  return {
    user,
    token,
    api,
    /** git pushes over HTTP as the admin. */
    remote: (repo: string) => `http://${user}:${password}@127.0.0.1:4950/${user}/${repo}.git`,
    down: () => {
      spawnSync('podman', [...FORGEJO_COMPOSE, 'rm', '-sf', 'forgejo'])
      spawnSync('podman', ['volume', 'rm', '-f', 'bakery-dev_bakery-forgejo-data', 'bakery-dev_bakery-forgejo-config'])
    },
  }
}

const sections: Record<string, () => Promise<void>> = {
  async goals() {
    const page = await signedIn()
    await clean(page)

    await page.goto(`${WEB}/#/goals`)
    const empty = page.getByText('No goals yet.')
    const tree = page.locator('[data-goal]').first()
    await Promise.race([empty.waitFor(), tree.waitFor()])
    expect('the Goals page shows its empty state or its tree', (await empty.isVisible()) || (await tree.isVisible()))
    expect('Goals is in the sidebar', await page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Goals' }).isVisible())

    // New Goal, or Add Goal on the empty state.
    await page.getByRole('button', { name: /^(New Goal|Add Goal)$/ }).first().click()
    const dialog = page.getByRole('dialog', { name: 'New goal' })
    await dialog.getByLabel('Goal title').fill('Ship guilds')
    await dialog.getByRole('button', { name: 'Level' }).click()
    await page.getByRole('option', { name: 'Guild' }).click()
    await dialog.getByRole('button', { name: 'Create goal' }).click()
    await page.locator('[data-goal="Ship guilds"]').waitFor()
    const parent = (await goals(page)).find((g) => g.title === 'Ship guilds')
    expect('New Goal creates "Ship guilds"', !!parent)

    await page.locator('[data-goal="Ship guilds"]').click()
    await page.getByRole('button', { name: 'Sub Goal' }).click()
    const sub = page.getByRole('dialog', { name: 'New sub-goal' })
    await sub.getByLabel('Goal title').fill('Guild rail')
    await sub.getByRole('button', { name: 'Create sub-goal' }).click()
    await page.locator('[data-goal="Guild rail"]').waitFor()
    const child = (await goals(page)).find((g) => g.title === 'Guild rail')
    expect('a Sub-goal is created under it', child?.parent_id === parent?.id, child)

    await page.goto(`${WEB}/#/goals`)
    await page.locator('[data-goal="Guild rail"]').waitFor()
    expect('the tree shows both', await page.locator('[data-goal="Ship guilds"]').isVisible())

    await page.goto(`${WEB}/#/goals/${parent!.id}`)
    await page.locator('[data-inline-editor="Title"]').click()
    await page.getByLabel('Title', { exact: true }).fill('Ship guilds now')
    await page.keyboard.press('Enter')
    await page.locator('[data-inline-editor="Title"]', { hasText: 'Ship guilds now' }).waitFor()
    await page.waitForTimeout(300)
    expect('the title is edited in place', (await goals(page)).find((g) => g.id === parent!.id)?.title === 'Ship guilds now')

    const properties = page.getByRole('complementary', { name: 'Properties' })
    await properties.getByRole('button', { name: 'Status' }).click()
    await page.getByRole('option', { name: 'active' }).click()
    await properties.getByText('active').waitFor()
    await page.waitForTimeout(300)
    expect('the status changes', (await goals(page)).find((g) => g.id === parent!.id)?.status === 'active')

    await page.locator('[data-inline-editor="Description"]').click()
    await page
      .getByLabel('Description', { exact: true })
      .fill('# The plan\n\nRead [the docs](https://example.com/docs).\n\n<script>window.pwned = 1</script><img src=x onerror="window.pwned = 1">')
    await page.locator('[data-inline-editor="Title"]').focus()
    const body = page.locator('[data-inline-editor="Description"]')
    await body.getByRole('heading', { name: 'The plan' }).waitFor()
    const link = body.getByRole('link', { name: 'the docs' })
    expect('a Markdown description renders a heading', await body.locator('h1').isVisible())
    expect(
      'and a link opening in a new tab',
      (await link.getAttribute('href')) === 'https://example.com/docs' && (await link.getAttribute('target')) === '_blank' && (await link.getAttribute('rel')) === 'noreferrer',
    )
    expect('and no <script> or handler', (await body.locator('script').count()) === 0 && (await body.locator('[onerror]').count()) === 0)
    expect('nothing ran', (await page.evaluate(() => (window as unknown as { pwned?: number }).pwned)) === undefined)

    await page.request.delete(`${WEB}/api/goals/${child!.id}`)
    await page.reload()
    await properties.getByRole('button', { name: 'Delete goal' }).click()
    await page.getByRole('alertdialog').or(page.getByRole('dialog')).getByRole('button', { name: 'Delete', exact: true }).click()
    await page.waitForURL(/#\/goals$/)
    await page.waitForTimeout(300)
    const left = (await goals(page)).filter((g) => scratch.includes(g.title))
    expect('the scratch Goals are deleted again', left.length === 0, left)

    await page.close()
  },

  async issues() {
    const page = await signedIn()
    await cleanIssues(page)
    const project = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: scratchProject } })).json()) as { project: { id: number } }

    await page.goto(`${WEB}/#/issues`)
    const nav = page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Issues' })
    await nav.waitFor()
    expect('Issues is in the sidebar', await nav.isVisible())
    await page.getByRole('button', { name: 'New Issue' }).first().click()
    let dialog = page.getByRole('dialog', { name: 'New issue' })
    await dialog.getByLabel('Issue title').fill(scratchIssues[0])
    await pick(page, dialog, 'Status', 'Backlog')
    await pick(page, dialog, 'Project', scratchProject)
    await dialog.getByRole('button', { name: 'Create issue' }).click()
    await page.locator('[data-issue]', { hasText: scratchIssues[0] }).waitFor()

    await page.getByRole('button', { name: 'New Issue' }).first().click()
    dialog = page.getByRole('dialog', { name: 'New issue' })
    await dialog.getByLabel('Issue title').fill(scratchIssues[1])
    await pick(page, dialog, 'Assignee', 'Me')
    await pick(page, dialog, 'Priority', 'High')
    await dialog.getByRole('button', { name: 'Create issue' }).click()
    await page.locator('[data-issue]', { hasText: scratchIssues[1] }).waitFor()
    expect('the toast names the new identifier', await page.getByText(/^[A-Z]+-\d+ created$/).first().isVisible())

    const made = (await issues(page)).filter((i) => scratchIssues.includes(i.title))
    const first = made.find((i) => i.title === scratchIssues[0])!
    const second = made.find((i) => i.title === scratchIssues[1])!
    expect('New Issue creates both', !!first && !!second, made)
    expect('the first is in the Project, in backlog', first?.project?.id === project.project.id && first?.status === 'backlog', first)
    expect('the second is assigned and high', !!second?.assignee && second?.priority === 'high', second)

    const row1 = page.locator(`[data-issue="${first.identifier}"]`)
    const row2 = page.locator(`[data-issue="${second.identifier}"]`)
    expect('a row shows its identifier', await row1.getByText(first.identifier, { exact: true }).isVisible())
    expect('and its status icon', await row1.getByRole('button', { name: 'Change status (current: Backlog)' }).isVisible())
    expect('and its priority icon', await row2.getByRole('img', { name: 'High priority' }).isVisible())
    expect("and the Assignee's name", await row2.getByText(second.assignee!.name, { exact: true }).isVisible())

    await page.getByRole('searchbox', { name: 'Search issues...' }).fill(second.identifier)
    await row1.waitFor({ state: 'detached' })
    expect('search by the identifier finds one', (await row2.isVisible()) && (await page.locator('[data-issue]').count()) === 1)
    await page.getByRole('button', { name: 'Clear search' }).click()
    await row1.waitFor()

    await page.getByRole('button', { name: /^Filter/ }).click()
    await page.getByRole('listbox', { name: 'Filter issues' }).getByRole('option', { name: 'Todo', exact: true }).click()
    await page.keyboard.press('Escape')
    await row1.waitFor({ state: 'detached' })
    expect('the status filter todo hides the backlog one', await row2.isVisible())
    expect('and the hash keeps it', page.url().includes('status=todo'))
    await page.reload()
    await row2.waitFor()
    expect('it survives a reload', (await row1.count()) === 0)

    await page.goto(`${WEB}/#/issues?project=${project.project.id}`)
    await row1.waitFor()
    expect('the Project filter keeps only the first', (await row2.count()) === 0 && (await page.locator('[data-issue]').count()) === 1)

    await page.goto(`${WEB}/#/issues?project=${project.project.id}&group=priority`)
    await page.reload()
    const medium = page.locator('[data-issue-group="medium"]')
    await medium.waitFor()
    expect('grouping shows the counts', (await medium.locator('[data-count]').textContent()) === '1')

    for (const i of made) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    await page.request.delete(`${WEB}/api/projects/${project.project.id}`)
    const left = (await issues(page)).filter((i) => scratchIssues.includes(i.title))
    expect('the scratch Issues are deleted again', left.length === 0, left)

    await page.close()
  },
  async issue() {
    const page = await signedIn()
    const titles = ['Port the Issue page', 'Port the Issue page now', 'Issue page sub-issue']
    const tidy = async () => {
      for (const i of (await issues(page)).filter((i) => titles.includes(i.title))) await page.request.delete(`${WEB}/api/issues/${i.id}`)
      for (const g of (await goals(page)).filter((g) => g.title === 'Issue page goal')) await page.request.delete(`${WEB}/api/goals/${g.id}`)
      const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
      for (const p of projects.filter((p) => p.name === 'Issue page project')) {
        const { project } = (await (await page.request.get(`${WEB}/api/projects/${p.id}`)).json()) as { project: { environments: { applications: { id: number }[] }[] } }
        for (const a of project.environments.flatMap((e) => e.applications)) await page.request.delete(`${WEB}/api/applications/${a.id}`)
        await page.request.delete(`${WEB}/api/projects/${p.id}`)
      }
    }
    await tidy()
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title: titles[0], status: 'todo' } })).json()) as { issue: Issue }
    await page.request.post(`${WEB}/api/goals`, { data: { title: 'Issue page goal', level: 'guild', status: 'active' } })

    await page.goto(`${WEB}/#/issues/${issue.identifier}`)
    const header = page.getByTestId('issue-detail-header')
    await header.getByText(issue.identifier, { exact: true }).waitFor()
    expect('the page opens by the identifier', await page.locator('[data-inline-editor="Title"]', { hasText: titles[0] }).isVisible())

    await page.locator('[data-inline-editor="Title"]').click()
    await page.getByLabel('Title', { exact: true }).fill(titles[1])
    await page.keyboard.press('Enter')
    await page.locator('[data-inline-editor="Title"]', { hasText: titles[1] }).waitFor()
    await page.reload()
    await page.locator('[data-inline-editor="Title"]', { hasText: titles[1] }).waitFor()
    expect('a renamed title survives a reload', true)

    const properties = page.getByRole('complementary', { name: 'Properties' })
    await properties.getByRole('button', { name: /^Change status/ }).click()
    await page.getByRole('listbox', { name: 'Status' }).getByRole('option', { name: 'In Progress' }).click()
    await properties.locator('[data-property-row="Started"]').waitFor()
    expect('In Progress shows the Started time', true)
    await properties.getByRole('button', { name: /^Change priority/ }).click()
    await page.getByRole('listbox', { name: 'Priority' }).getByRole('option', { name: 'High' }).click()
    await properties.getByRole('button', { name: 'Change priority (current: High)' }).waitFor()
    const me = ((await (await page.request.get(`${WEB}/api/me`)).json()) as { member: { name: string } }).member.name
    await pick(page, properties, 'Assignee', me)
    await pick(page, properties, 'Goal', 'Issue page goal')
    await properties.getByRole('button', { name: 'Goal' }).getByText('Issue page goal').waitFor()
    const saved = ((await (await page.request.get(`${WEB}/api/issues/${issue.identifier}`)).json()) as { issue: Issue & { goal: { title: string } | null; started_at: string | null } }).issue
    expect('status, priority, Assignee and Goal are saved', saved.status === 'in_progress' && saved.priority === 'high' && saved.assignee?.name === me && saved.goal?.title === 'Issue page goal' && !!saved.started_at, saved)

    // The Issue's Application, picked from its Project's once it has one.
    const scratch = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: 'Issue page project' } })).json()) as { project: { id: number } }
    const env = ((await (await page.request.get(`${WEB}/api/projects/${scratch.project.id}`)).json()) as { project: { environments: { id: number }[] } }).project.environments[0]
    await page.request.post(`${WEB}/api/environments/${env.id}/applications`, { data: { name: 'issue-page-web', build_pack: 'dockerimage', docker_image: 'ghcr.io/traefik/whoami:v1.10', port: 80 } })
    await page.reload()
    await header.getByText(issue.identifier, { exact: true }).waitFor()
    expect('no Application row without a Project', (await properties.locator('[data-property-row="Application"]').count()) === 0)
    await pick(page, properties, 'Project', 'Issue page project')
    await properties.locator('[data-property-row="Application"]').waitFor()
    await pick(page, properties, 'Application', 'issue-page-web')
    await properties.getByRole('button', { name: 'Application' }).getByText('issue-page-web').waitFor()
    const withApp = ((await (await page.request.get(`${WEB}/api/issues/${issue.identifier}`)).json()) as { issue: Issue }).issue
    expect("the Issue's Application is saved", withApp.application?.name === 'issue-page-web' && withApp.project?.id === scratch.project.id, withApp)
    await pick(page, properties, 'Project', 'No project')
    await properties.locator('[data-property-row="Application"]').waitFor({ state: 'detached' })
    const moved = ((await (await page.request.get(`${WEB}/api/issues/${issue.identifier}`)).json()) as { issue: Issue }).issue
    expect('leaving the Project lets go of the Application', moved.application === null, moved)

    // An Agent of the Guild as the Assignee, picked under the Members.
    const hire = (await (await page.request.post(`${WEB}/api/agents`, { data: { name: 'Issue page agent', job: 'engineer', icon: 'rocket' } })).json()) as { agent: { id: number; approval_id: number } }
    await page.request.post(`${WEB}/api/approvals/${hire.agent.approval_id}/approve`, { data: {} })
    await page.reload()
    await header.getByText(issue.identifier, { exact: true }).waitFor()
    await pick(page, properties, 'Assignee', 'Issue page agent')
    const agentCell = properties.locator(`[data-assignee-agent="${hire.agent.id}"]`)
    await agentCell.waitFor()
    expect("an Agent Assignee shows its name and icon", (await agentCell.innerText()).trim() === 'Issue page agent' && (await agentCell.locator('svg.lucide-rocket').count()) === 1)
    const toAgent = ((await (await page.request.get(`${WEB}/api/issues/${issue.identifier}`)).json()) as { issue: Issue }).issue
    expect('the API answers the Agent as the Assignee', toAgent.assignee?.kind === 'agent' && toAgent.assignee.id === hire.agent.id, toAgent.assignee)
    await page.request.post(`${WEB}/api/agents/${hire.agent.id}/terminate`)
    const unassigned = ((await (await page.request.get(`${WEB}/api/issues/${issue.identifier}`)).json()) as { issue: Issue }).issue
    expect('terminating the Agent takes it off the Issue', unassigned.assignee === null, unassigned.assignee)

    await page.getByRole('region', { name: 'Sub-issues' }).getByRole('button', { name: 'Add sub-issue' }).click()
    const dialog = page.getByRole('dialog', { name: 'New sub-issue' })
    await dialog.getByLabel('Issue title').fill(titles[2])
    await dialog.getByRole('button', { name: 'Create sub-issue' }).click()
    const child = page.getByRole('region', { name: 'Sub-issues' }).locator('[data-issue]', { hasText: titles[2] })
    await child.waitFor()
    expect('Add sub-issue lists the new one', true)
    await child.click()
    await page.locator('[data-inline-editor="Title"]', { hasText: titles[2] }).waitFor()
    expect('it opens with the parent link', await page.getByRole('navigation', { name: 'Parent issues' }).getByRole('link', { name: titles[1] }).isVisible())
    await page.getByRole('navigation', { name: 'Parent issues' }).getByRole('link', { name: titles[1] }).click()
    await page.locator('[data-inline-editor="Title"]', { hasText: titles[1] }).waitFor()

    const thread = page.getByRole('region', { name: 'Comments' })
    await thread.getByLabel('Comment', { exact: true }).fill('Looks right:\n\n```go\nfmt.Println("guild")\n```')
    await page.keyboard.press('Control+Enter')
    const comment = thread.locator('[data-comment]').last()
    await comment.locator('pre code').waitFor()
    expect('a Markdown comment renders its code block', (await comment.locator('pre code').textContent())?.includes('fmt.Println') ?? false)
    await comment.getByRole('button', { name: 'Edit comment' }).click()
    await comment.getByLabel('Edit comment', { exact: true }).fill('Looks **right** now.')
    await comment.getByRole('button', { name: 'Save' }).click()
    await comment.locator('strong', { hasText: 'right' }).waitFor()
    expect('an edited comment says so', await comment.getByText('· edited').isVisible())
    await comment.getByRole('button', { name: 'Delete comment' }).click()
    await page.getByTestId('confirm-delete-comment').click()
    await comment.getByText('Comment deleted').waitFor()
    expect('a deleted comment leaves its placeholder', true)

    const goal = (await goals(page)).find((g) => g.title === 'Issue page goal')!
    await page.goto(`${WEB}/#/goals/${goal.id}`)
    await page.getByRole('tab', { name: /^Issues/ }).click()
    await page.getByRole('link', { name: new RegExp(titles[1]) }).click()
    await page.waitForURL(new RegExp(`#/issues/${issue.identifier}$`))
    await page.locator('[data-inline-editor="Title"]', { hasText: titles[1] }).waitFor()
    expect("a Goal's Issue row opens the Issue page", true)
    await page.goto(`${WEB}/#/issues?q=${issue.identifier}`)
    await page.locator(`[data-issue="${issue.identifier}"] a`).click()
    await page.locator('[data-inline-editor="Title"]', { hasText: titles[1] }).waitFor()
    expect('a row of the Issues list opens it too', true)

    await page.goto(`${WEB}/#/issues/DEF-99999`)
    await page.getByTestId('not-found').waitFor()
    expect('an unknown identifier shows the not-found state', await page.getByText('This issue does not exist or you cannot see it.').isVisible())

    await tidy()
    const left = (await issues(page)).filter((i) => titles.includes(i.title))
    expect('the scratch Issues are deleted again', left.length === 0, left)
    await page.close()
  },
  async viewer() {
    const page = await signedIn()
    const title = 'Viewer reads this'
    for (const i of (await issues(page)).filter((i) => i.title === title)) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, status: 'todo' } })).json()) as { issue: Issue }
    const v = await invited(page, 'viewer')

    await v.page.goto(`${WEB}/#/issues`)
    await v.page.locator(`[data-issue="${issue.identifier}"]`).waitFor()
    expect('a Viewer sees the Issues list', true)
    expect('a Viewer sees no New Issue', (await v.page.getByRole('button', { name: 'New Issue' }).count()) === 0)
    await v.page.goto(`${WEB}/#/goals`)
    await Promise.race([v.page.getByText('No goals yet.').waitFor(), v.page.locator('[data-goal]').first().waitFor()])
    expect('a Viewer sees Goals but no New Goal', (await v.page.getByRole('button', { name: /^(New Goal|Add Goal)$/ }).count()) === 0)

    await v.page.goto(`${WEB}/#/issues/${issue.identifier}`)
    await v.page.getByRole('heading', { name: title }).waitFor()
    expect('a Viewer opens the Issue, its title not editable', (await v.page.locator('[data-inline-editor]').count()) === 0)
    const thread = v.page.getByRole('region', { name: 'Comments' })
    expect('a Viewer sees no composer', (await thread.getByLabel('Comment', { exact: true }).count()) === 0)
    const properties = v.page.getByRole('complementary', { name: 'Properties' })
    expect('a Viewer sees no pickers', (await properties.getByRole('button', { name: /^Change (status|priority)/ }).count()) === 0)
    expect('a Viewer sees no Delete', (await v.page.getByRole('button', { name: 'More issue actions' }).count()) === 0)
    expect('a Viewer sees no Add sub-issue', (await v.page.getByRole('button', { name: 'Add sub-issue' }).count()) === 0)

    const post = await v.page.request.post(`${WEB}/api/issues`, { data: { title: 'Viewer may not' } })
    expect('the API answers 403 to a Viewer\'s POST', post.status() === 403, post.status())
    const comment = await v.page.request.post(`${WEB}/api/issues/${issue.id}/comments`, { data: { body: 'no' } })
    expect('and to a Viewer\'s Comment', comment.status() === 403, comment.status())

    await v.leave()
    await page.request.delete(`${WEB}/api/issues/${issue.id}`)
    await page.close()
  },
  async blockers() {
    const page = await signedIn()
    const titles = ['Blocks the others', 'Waits on a blocker', 'Unrelated scratch']
    for (const i of (await issues(page)).filter((i) => titles.includes(i.title))) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const [a, b, c] = await Promise.all(
      titles.map(async (title) => ((await (await page.request.post(`${WEB}/api/issues`, { data: { title, status: 'todo' } })).json()) as { issue: Issue }).issue),
    )
    const properties = page.getByRole('complementary', { name: 'Properties' })
    const blockedBy = properties.locator('[data-property-label="Blocked by"]')
    const blocking = properties.locator('[data-property-label="Blocking"]')
    const notice = page.getByTestId('issue-blocked-notice')

    await page.goto(`${WEB}/#/issues/${b.identifier}`)
    await page.getByRole('heading', { name: b.title }).waitFor()
    expect('no notice without Blockers', (await notice.count()) === 0)
    await blockedBy.getByRole('button', { name: 'Add blocker' }).click()
    await page.getByLabel('Search issues to add as blockers').fill(a.title)
    await page.locator(`[data-candidate="${a.identifier}"]`).click()
    await blockedBy.locator(`[data-blocker="${a.identifier}"]`).waitFor()
    expect('the picker adds the Blocker', true)
    expect('and keeps out the Issue itself', (await page.locator(`[data-candidate="${b.identifier}"]`).count()) === 0)
    await page.keyboard.press('Escape')
    await notice.locator(`[data-blocker="${a.identifier}"]`).waitFor()
    expect('the notice names the Blocker', (await notice.textContent())?.includes(a.title) ?? false)

    await page.goto(`${WEB}/#/issues`)
    const row = page.locator(`[data-issue="${b.identifier}"]`)
    await row.waitFor()
    expect('the Issues list marks the row blocked', (await row.locator('[data-blocked-marker]').getAttribute('title')) === 'Blocked by 1 issue')
    expect('and not the Blocker', (await page.locator(`[data-issue="${a.identifier}"] [data-blocked-marker]`).count()) === 0)

    await page.goto(`${WEB}/#/issues/${a.identifier}`)
    await page.getByRole('heading', { name: a.title }).waitFor()
    await blocking.locator(`[data-blocker="${b.identifier}"]`).waitFor()
    expect('the Blocker lists it under Blocking', true)
    expect('Blocking has no controls', (await blocking.getByRole('button').count()) === 0)
    await blockedBy.getByRole('button', { name: 'Add blocker' }).click()
    await page.getByLabel('Search issues to add as blockers').fill(b.title)
    await page.locator(`[data-candidate="${b.identifier}"]`).click()
    await page.getByText('an issue cannot be blocked by an issue it blocks').first().waitFor()
    expect('a cycle shows the API\'s error', true)
    await page.keyboard.press('Escape')
    expect('and adds nothing', (await blockedBy.locator('[data-blocker]').count()) === 0)

    await page.request.patch(`${WEB}/api/issues/${a.id}`, { data: { status: 'done' } })
    await page.goto(`${WEB}/#/issues/${b.identifier}`)
    await page.getByRole('heading', { name: b.title }).waitFor()
    await blockedBy.locator(`[data-blocker="${a.identifier}"]`).waitFor()
    expect('a done Blocker ends the notice', (await notice.count()) === 0)

    const v = await invited(page, 'viewer')
    await v.page.goto(`${WEB}/#/issues/${b.identifier}`)
    const vProps = v.page.getByRole('complementary', { name: 'Properties' })
    await vProps.locator(`[data-property-label="Blocked by"] [data-blocker="${a.identifier}"]`).waitFor()
    expect('a Viewer sees the Blocker', true)
    expect('without the picker or ×', (await vProps.locator('[data-property-label="Blocked by"]').getByRole('button').count()) === 0)
    await v.leave()

    await blockedBy.getByRole('button', { name: `Remove ${a.identifier} as blocker` }).click()
    await blockedBy.locator('[data-blocker]').waitFor({ state: 'detached' })
    expect('× removes the Blocker', ((await (await page.request.get(`${WEB}/api/issues/${b.id}`)).json()) as { issue: { blocked_by: unknown[] } }).issue.blocked_by.length === 0)

    for (const i of [a, b, c]) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    await page.close()
  },
  async documents() {
    const page = await signedIn()
    const title = 'Documents scratch'
    for (const i of (await issues(page)).filter((i) => i.title === title)) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title } })).json()) as { issue: Issue }
    const docs = page.getByRole('region', { name: 'Documents' })
    const card = docs.locator('[data-document="plan"]')
    const body = page.getByRole('textbox', { name: 'Document body' })
    const history = card.getByRole('button', { name: 'Revision history of plan' })
    const rev = async () =>
      ((await (await page.request.get(`${WEB}/api/issues/${issue.id}/documents/plan`)).json()) as { document: { latest_revision_number: number; body: string } }).document

    await page.goto(`${WEB}/#/issues/${issue.identifier}`)
    await page.getByRole('heading', { name: title }).waitFor()
    await docs.getByRole('button', { name: 'New document' }).click()
    await page.getByLabel('Document key').fill('Bad Key')
    expect('a bad key is refused before saving', await page.getByText('Use lowercase letters').isVisible())
    await page.getByLabel('Document key').fill('plan')
    await page.getByLabel('Document title').fill('Plan')
    await body.fill('# Plan\n\nFirst line\nKept line')
    await page.getByRole('button', { name: 'Create document' }).click()
    await card.getByText('First line').waitFor()
    expect('New document creates rev 1', (await history.textContent())?.includes('rev 1') ?? false)

    await card.getByRole('button', { name: 'Document actions' }).click()
    await page.getByRole('menuitem', { name: 'Edit document' }).click()
    await body.fill('# Plan\n\nSecond line\nKept line')
    await page.getByLabel('Change summary').fill('Rewrite the first line')
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    await card.getByText('Second line').waitFor()
    expect('editing saves rev 2', (await rev()).latest_revision_number === 2)

    await history.click()
    await page.locator('[data-revision="1"]').click()
    await page.getByTestId('revision-preview').waitFor()
    expect('History opens rev 1 read-only', (await page.getByTestId('revision-preview').textContent())?.includes('Viewing revision 1') ?? false)
    expect('showing its text', await card.getByText('First line').isVisible())
    await page.getByRole('button', { name: 'Compare with current' }).click()
    const diff = page.getByTestId('document-diff')
    await diff.waitFor()
    const removed = await diff.locator('[data-diff="removed"]').allTextContents()
    const added = await diff.locator('[data-diff="added"]').allTextContents()
    expect('Compare shows a removed line', removed.length === 1 && removed[0].includes('First line'), removed)
    expect('and an added line', added.length === 1 && added[0].includes('Second line'), added)
    await page.keyboard.press('Escape')
    await diff.waitFor({ state: 'detached' })
    await page.getByRole('button', { name: 'Restore this revision' }).click()
    await page.getByTestId('revision-preview').waitFor({ state: 'detached' })
    const restored = await rev()
    expect('Restore makes rev 3 with rev 1\'s text', restored.latest_revision_number === 3 && restored.body.includes('First line'), restored)

    await card.getByRole('button', { name: 'Document actions' }).click()
    await page.getByRole('menuitem', { name: 'Edit document' }).click()
    await body.fill('# Plan\n\nMine')
    const other = await signedIn()
    await other.goto(`${WEB}/#/issues/${issue.identifier}`)
    const otherCard = other.locator('[data-document="plan"]')
    await otherCard.getByRole('button', { name: 'Document actions' }).click()
    await other.getByRole('menuitem', { name: 'Edit document' }).click()
    await other.getByRole('textbox', { name: 'Document body' }).fill('# Plan\n\nTheirs')
    await other.getByRole('button', { name: 'Save', exact: true }).click()
    await otherCard.getByText('Theirs').waitFor()
    await other.context().close()
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    await page.getByTestId('document-conflict').waitFor()
    expect('saving second shows the conflict notice', (await page.getByTestId('document-conflict').textContent())?.includes('Someone saved a newer revision') ?? false)
    expect('and keeps the person\'s text', (await body.inputValue()).includes('Mine'))
    expect('and did not overwrite', (await rev()).body.includes('Theirs'))
    await page.getByTestId('document-conflict').getByRole('button', { name: 'Reload' }).click()
    await card.getByText('Theirs').waitFor()
    expect('Reload shows the newest revision', (await history.textContent())?.includes('rev 4') ?? false)

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      (async () => {
        await card.getByRole('button', { name: 'Document actions' }).click()
        await page.getByRole('menuitem', { name: 'Download document' }).click()
      })(),
    ])
    expect('Download gives plan.md', download.suggestedFilename() === 'plan.md', download.suggestedFilename())

    const v = await invited(page, 'viewer')
    await v.page.goto(`${WEB}/#/issues/${issue.identifier}`)
    const vCard = v.page.locator('[data-document="plan"]')
    await vCard.getByText('Theirs').waitFor()
    expect('a Viewer sees the document', true)
    expect('without New document', (await v.page.getByRole('button', { name: 'New document' }).count()) === 0)
    await vCard.getByRole('button', { name: 'Revision history of plan' }).click()
    await v.page.locator('[data-revision="1"]').waitFor()
    expect('and with History', (await v.page.locator('[data-revision]').count()) === 4)
    await v.page.keyboard.press('Escape')
    await vCard.getByRole('button', { name: 'Document actions' }).click()
    expect('but no Edit', (await v.page.getByRole('menuitem', { name: 'Edit document' }).count()) === 0)
    expect('or Delete', (await v.page.getByRole('menuitem', { name: 'Delete document' }).count()) === 0)
    await v.leave()

    await card.getByRole('button', { name: 'Document actions' }).click()
    await page.getByRole('menuitem', { name: 'Delete document' }).click()
    await page.getByTestId('confirm-delete-document').click()
    await card.waitFor({ state: 'detached' })
    const gone = await page.request.get(`${WEB}/api/issues/${issue.id}/documents/plan`)
    expect('Delete removes it', gone.status() === 404, gone.status())

    await page.request.delete(`${WEB}/api/issues/${issue.id}`)
    await page.close()
  },
  async hidden() {
    const page = await signedIn()
    const name = 'Work e2e hidden'
    const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
    for (const p of projects.filter((p) => p.name === name)) await page.request.delete(`${WEB}/api/projects/${p.id}`)
    for (const i of (await issues(page)).filter((i) => i.title === 'Hidden with its Project')) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name } })).json()) as { project: { id: number } }
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title: 'Hidden with its Project', project_id: project.id } })).json()) as { issue: Issue }
    const m = await invited(page, 'member')
    const sees = async () => (await issues(m.page)).some((i) => i.id === issue.id)
    const inFeed = async () => {
      const { activity } = (await (await m.page.request.get(`${WEB}/api/activity?entity=issue&limit=200`)).json()) as { activity: { entity: { id: number } }[] }
      return activity.some((e) => e.entity.id === issue.id)
    }
    expect('a Member sees the Issue before the override', await sees())
    expect('and its Activity in the feed', await inFeed())

    const undeny = await denyView(page, project.id, 'Member')

    expect('a Member no longer sees the Issue in the list', !(await sees()))
    const byId = await m.page.request.get(`${WEB}/api/issues/${issue.identifier}`)
    expect('nor by its identifier', byId.status() === 404, byId.status())
    expect('nor its Activity in the feed', !(await inFeed()))
    const itsActivity = await m.page.request.get(`${WEB}/api/issues/${issue.identifier}/activity`)
    expect('and its own Activity is 404', itsActivity.status() === 404, itsActivity.status())
    await m.page.goto(`${WEB}/#/issues/${issue.identifier}`)
    await m.page.getByTestId('not-found').waitFor()
    expect('the Issue page shows the not-found state', true)
    await m.page.goto(`${WEB}/#/issues`)
    await m.page.getByRole('button', { name: 'New Issue' }).first().waitFor()
    expect('the Issues list has no row for it', (await m.page.locator(`[data-issue="${issue.identifier}"]`).count()) === 0)

    await undeny()
    expect('removing the override shows it again', await sees())

    await m.leave()
    await page.request.delete(`${WEB}/api/issues/${issue.id}`)
    await page.request.delete(`${WEB}/api/projects/${project.id}`)
    await page.close()
  },
  activity: async () => {
    const page = await signedIn()
    const title = 'Activity e2e scratch'
    for (const i of (await issues(page)).filter((i) => i.title.startsWith(title))) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { member } = (await (await page.request.get(`${WEB}/api/me`)).json()) as { member: { name: string } }
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title } })).json()) as { issue: Issue }
    await page.request.post(`${WEB}/api/issues/${issue.id}/comments`, { data: { body: 'Seen in the feed' } })

    await page.goto(`${WEB}/#/`)
    const nav = page.getByRole('navigation', { name: 'Main' })
    await nav.getByRole('link', { name: 'Activity' }).waitFor()
    expect('the sidebar has Activity', (await nav.getByText('Guild', { exact: true }).count()) === 1)
    await nav.getByRole('link', { name: 'Activity' }).click()
    const rows = page.getByRole('list', { name: 'Activity' }).getByRole('listitem')
    await rows.first().waitFor()
    const first = (await rows.first().innerText()).replace(/\s+/g, ' ')
    expect('the newest row is the comment', first.includes(`${member.name} commented on ${title}`) && first.includes(issue.identifier), first)

    await page.getByRole('button', { name: 'Entity', exact: true }).click()
    await page.getByRole('option', { name: 'Goals', exact: true }).click()
    await page.waitForFunction(() => location.hash === '#/activity?entity=goal')
    await page.waitForFunction((t) => !document.querySelector('ul[aria-label="Activity"]')?.textContent?.includes(t), title)
    expect('the Goals filter hides it', true)
    await page.reload()
    await page.getByRole('button', { name: 'Entity', exact: true }).waitFor()
    expect('a reload keeps entity=goal', (await page.getByRole('button', { name: 'Entity', exact: true }).innerText()).trim() === 'Goals')
    await page.goto(`${WEB}/#/activity?entity=issue`)
    await page.getByRole('button', { name: 'Actor', exact: true }).click()
    await page.getByRole('option', { name: member.name, exact: true }).click()
    await page.waitForFunction(() => /[?&]actor=\d+/.test(location.hash))
    await rows.first().waitFor()
    expect('the Actor filter set to you keeps it', (await rows.first().innerText()).includes(issue.identifier), await rows.first().innerText())

    await page.goto(`${WEB}/#/activity?entity=issue`)
    await rows.first().waitFor()
    await page.locator('[data-activity="issue.comment_added"]').first().getByRole('link').click()
    await page.waitForFunction((id) => location.hash === `#/issues/${id}`, issue.identifier)
    expect('the row opens the Issue', true)

    const { issue: blocker } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title: `${title} blocker` } })).json()) as { issue: Issue }
    await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { status: 'todo' } })
    await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { status: 'in_progress', priority: 'high' } })
    await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { blocked_by_ids: [blocker.id] } })
    const doc = `${WEB}/api/issues/${issue.id}/documents/plan`
    const { document: plan } = (await (await page.request.put(doc, { data: { title: '', body: 'First draft' } })).json()) as { document: { latest_revision_id: number } }
    await page.request.put(doc, { data: { title: '', body: 'Second draft', base_revision_id: plan.latest_revision_id } })
    await page.reload()
    await page.getByRole('tab', { name: 'Activity' }).click()
    await page.waitForFunction(() => location.hash.endsWith('?tab=activity'))
    const tab = page.getByRole('region', { name: 'Activity' })
    await tab.locator('[data-activity="issue.document_updated"]').waitFor()
    const said = (await tab.innerText()).replace(/\s+/g, ' ')
    const sentences = [
      'created the issue',
      'commented Seen in the feed',
      'changed the status from Todo to In Progress',
      'changed the priority from Medium to High',
      `added blocker ${blocker.identifier}`,
      'created document plan',
      'updated document plan (rev 2)',
    ]
    for (const words of sentences) expect(`the Activity tab says "${words}"`, said.includes(words), said)
    const at = sentences.map((w) => said.indexOf(w))
    expect('oldest first, as Paperclip lists them', at.every((i, n) => n === 0 || at[n - 1] < i), at)
    expect('the Blocker links to its Issue', await tab.getByRole('link', { name: blocker.identifier }).isVisible())
    await page.reload()
    await tab.locator('[data-activity="issue.created"]').waitFor()
    expect('?tab=activity opens on Activity after a reload', (await page.getByRole('tab', { name: 'Activity' }).getAttribute('aria-selected')) === 'true')
    await page.getByRole('tab', { name: 'Comments' }).click()
    const thread = page.getByRole('region', { name: 'Comments' })
    await thread.getByLabel('Comment', { exact: true }).fill('Posted from the tab')
    await page.keyboard.press('Control+Enter')
    await thread.locator('[data-comment]', { hasText: 'Posted from the tab' }).waitFor()
    expect('the Comments tab still posts', !page.url().includes('tab='))
    await page.getByRole('tab', { name: 'Activity' }).click()
    await tab.locator('[data-activity="issue.comment_added"]', { hasText: 'Posted from the tab' }).waitFor()
    expect('the Activity tab shows the new comment', true)

    const hiddenName = 'Activity e2e hidden'
    const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
    for (const p of projects.filter((p) => p.name === hiddenName)) await page.request.delete(`${WEB}/api/projects/${p.id}`)
    const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: hiddenName } })).json()) as { project: { id: number } }
    const { issue: hidden } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title: `${title} hidden`, project_id: project.id } })).json()) as { issue: Issue }
    const undeny = await denyView(page, project.id, 'Viewer')
    const v = await invited(page, 'viewer')
    await v.page.goto(`${WEB}/#/activity`)
    const theirs = v.page.getByRole('list', { name: 'Activity' }).getByRole('listitem')
    await theirs.first().waitFor()
    const feed = await v.page.getByRole('list', { name: 'Activity' }).innerText()
    expect('a Viewer sees the feed', feed.includes(issue.identifier), feed)
    expect('without the Issue in a Project they may not view', !feed.includes(hidden.identifier), feed)
    await v.leave()
    await undeny()

    await page.request.delete(`${WEB}/api/issues/${hidden.id}`)
    await page.request.delete(`${WEB}/api/projects/${project.id}`)
    await page.request.delete(`${WEB}/api/issues/${blocker.id}`)
    await page.request.delete(`${WEB}/api/issues/${issue.id}`)
    await page.close()
  },
  inbox: async () => {
    const page = await signedIn()
    const title = (n: string) => `Inbox e2e ${n}`
    for (const i of (await issues(page)).filter((i) => i.title.startsWith('Inbox e2e '))) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    // Waiting Approvals count in the Inbox badge too; the counts below are
    // for Issues alone, so none may be left waiting.
    const waiting = (await (await page.request.get(`${WEB}/api/approvals?status=actionable`)).json()) as { approvals: { id: number }[] }
    for (const a of waiting.approvals) await page.request.post(`${WEB}/api/approvals/${a.id}/reject`, { data: {} })
    const made = async (n: string) =>
      ((await (await page.request.post(`${WEB}/api/issues`, { data: { title: title(n) } })).json()) as { issue: Issue }).issue
    const first = await made('first')
    const second = await made('second')
    const row = (i: Issue) => page.locator(`[data-slot="task-row"][data-issue="${i.identifier}"]`)

    await page.goto(`${WEB}/`)
    await page.evaluate(() => localStorage.removeItem('bakery:inbox:last-tab'))
    await page.goto(`${WEB}/#/inbox`)
    await row(first).waitFor()
    expect('#/inbox opens Mine', page.url().endsWith('#/inbox/mine'), page.url())
    expect('Mine lists an Issue the owner created, read', (await row(first).getAttribute('data-unread')) === null)

    const m = await invited(page, 'member')
    for (const i of [first, second]) {
      const c = await m.page.request.post(`${WEB}/api/issues/${i.id}/comments`, { data: { body: 'Over to you' } })
      if (!c.ok()) throw new Error(`comment: ${c.status()}`)
    }
    await page.reload()
    await row(first).and(page.locator('[data-unread="true"]')).waitFor()
    expect('a Comment from someone else shows the unread dot', true)
    await row(first).getByRole('button', { name: 'Mark as read' }).click()
    await row(first).and(page.locator(':not([data-unread])')).waitFor()
    await page.reload()
    await row(first).waitFor()
    expect('the dot stays cleared after a reload', (await row(first).getAttribute('data-unread')) === null)

    await row(first).hover()
    await row(first).getByRole('button', { name: 'Archive' }).click()
    await row(first).waitFor({ state: 'detached' })
    expect('Archive takes the row out of Mine', true)
    await page.getByRole('button', { name: 'Undo' }).click()
    await row(first).waitFor()
    await page.reload()
    await row(first).waitFor()
    expect('Undo brings it back to Mine', true)
    await row(first).hover()
    await row(first).getByRole('button', { name: 'Archive' }).click()
    await row(first).waitFor({ state: 'detached' })

    await page.getByRole('tab', { name: 'Recent' }).click()
    await page.waitForURL(/#\/inbox\/recent$/)
    await row(first).and(page.locator('[data-archived="true"]')).waitFor()
    expect('Recent shows the archived row with Unarchive', await row(first).getByRole('button', { name: 'Unarchive' }).isVisible())

    await page.getByRole('tab', { name: 'Unread' }).click()
    await page.waitForURL(/#\/inbox\/unread$/)
    await row(first).waitFor({ state: 'detached' })
    await row(second).waitFor()
    const listed = await page.locator('[data-slot="task-row"]').evaluateAll((rows) => rows.map((r) => r.getAttribute('data-issue')))
    expect('Unread lists only the unread one', !listed.includes(first.identifier) && listed.includes(second.identifier), listed)
    await page.getByRole('button', { name: 'Mark all as read' }).click()
    await page.getByRole('alertdialog').getByRole('button', { name: 'Mark all as read' }).click()
    await row(second).waitFor({ state: 'detached' })
    await page.reload()
    await page.getByText('No new inbox items.').waitFor()
    expect('Mark all as read empties Unread', true)

    await page.getByRole('tab', { name: 'Recent' }).click()
    await row(first).waitFor()
    await page.getByRole('searchbox', { name: 'Search inbox…' }).fill(second.identifier)
    await row(first).waitFor({ state: 'detached' })
    expect('search by identifier keeps only that Issue', await row(second).isVisible())

    await page.goto(`${WEB}/#/issues`)
    await page.goto(`${WEB}/#/inbox`)
    await row(first).waitFor()
    expect('#/inbox opens the last tab used', page.url().endsWith('#/inbox/recent'), page.url())

    const review = await m.page.request.patch(`${WEB}/api/issues/${first.id}`, { data: { status: 'in_review' } })
    if (!review.ok()) throw new Error(`status: ${review.status()}`)
    await page.getByRole('tab', { name: 'Mine' }).click()
    await page.waitForURL(/#\/inbox\/mine$/)
    await row(first).and(page.locator(':not([data-archived])')).waitFor()
    const mine = (await (await page.request.get(`${WEB}/api/issues?inbox=me`)).json()) as { issues: (Issue & { archived?: boolean })[] }
    const back = mine.issues.find((i) => i.id === first.id)
    expect('In Review brings the archived Issue back to Mine', !!back && !back.archived, back)
    await page.getByRole('tab', { name: 'Recent' }).click()
    await page.waitForURL(/#\/inbox\/recent$/)

    const inboxItem = page.getByRole('navigation', { name: 'Main' }).locator('a[href="#/inbox"]')
    const badge = inboxItem.getByTestId('sidebar-nav-badge')
    const dot = inboxItem.locator('[data-slot="sidebar-nav-badge-dot"]')
    const again = await m.page.request.post(`${WEB}/api/issues/${second.id}/comments`, { data: { body: 'And again' } })
    if (!again.ok()) throw new Error(`comment: ${again.status()}`)
    await page.reload()
    await badge.waitFor()
    expect('the sidebar counts the unread Issue', (await badge.textContent())?.trim() === '1', await badge.textContent())
    await page.evaluate(() => localStorage.setItem('sidebarCollapsed', 'true'))
    await page.reload()
    await dot.waitFor()
    expect('the rail shows a dot and names the count', (await inboxItem.getAttribute('aria-label')) === 'Inbox, 1 unread', await inboxItem.getAttribute('aria-label'))
    await page.goto(`${WEB}/#/issues/${second.identifier}`)
    await page.getByText(title('second')).first().waitFor()
    await dot.waitFor({ state: 'detached' })
    await page.goBack()
    await page.waitForURL(/#\/inbox\/recent$/)
    expect('opening the Issue clears the rail dot', (await dot.count()) === 0)
    await page.evaluate(() => localStorage.setItem('sidebarCollapsed', 'false'))
    await page.reload()
    await inboxItem.waitFor()
    expect('and the badge', (await badge.count()) === 0)
    const counted = ((await (await page.request.get(`${WEB}/api/sidebar-badges`)).json()) as { inbox: number }).inbox
    expect('/api/sidebar-badges agrees', counted === 0, counted)

    const hiddenName = 'Inbox e2e hidden'
    const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
    for (const p of projects.filter((p) => p.name === hiddenName)) await page.request.delete(`${WEB}/api/projects/${p.id}`)
    const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: hiddenName } })).json()) as { project: { id: number } }
    const { issue: hidden } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title: title('hidden'), project_id: project.id } })).json()) as { issue: Issue }
    const undeny = await denyView(page, project.id, 'Member')
    const assigned = await page.request.patch(`${WEB}/api/issues/${hidden.id}`, { data: { assignee_id: m.id } })
    if (!assigned.ok()) throw new Error(`assign: ${assigned.status()} ${await assigned.text()}`)
    const theirs = m.page.locator(`[data-slot="task-row"][data-issue="${hidden.identifier}"]`)
    for (const tab of ['mine', 'recent']) {
      await m.page.goto(`${WEB}/#/inbox/${tab}`)
      await m.page.locator(`[data-slot="task-row"][data-issue="${first.identifier}"]`).waitFor()
      expect(`an assigned Issue in a Project the Member may not view is not on ${tab}`, (await theirs.count()) === 0)
    }
    const api = (await (await m.page.request.get(`${WEB}/api/issues?touched=me`)).json()) as { issues: Issue[] }
    expect('nor in touched=me', !api.issues.some((i) => i.id === hidden.id))
    await undeny()
    await page.request.delete(`${WEB}/api/issues/${hidden.id}`)
    await page.request.delete(`${WEB}/api/projects/${project.id}`)

    await m.leave()
    for (const i of [first, second]) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    await page.close()
  },
  approvals: async () => {
    const page = await signedIn()
    const prefix = 'Approvals e2e'
    type Approval = { id: number; status: string; payload: { title: string } }
    const all = async () => ((await (await page.request.get(`${WEB}/api/approvals`)).json()) as { approvals: Approval[] }).approvals
    // Approvals cannot be deleted; a run before leaves its own decided.
    for (const a of (await all()).filter((a) => a.payload.title.startsWith(prefix) && (a.status === 'pending' || a.status === 'revision_requested')))
      await page.request.post(`${WEB}/api/approvals/${a.id}/reject`, { data: {} })
    for (const i of (await issues(page)).filter((i) => i.title.startsWith(prefix))) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { projects } = (await (await page.request.get(`${WEB}/api/projects`)).json()) as { projects: { id: number; name: string }[] }
    for (const p of projects.filter((p) => p.name === `${prefix} hidden`)) await page.request.delete(`${WEB}/api/projects/${p.id}`)
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title: `${prefix} issue` } })).json()) as { issue: Issue }
    const ask = async (title: string) => {
      const r = await page.request.post(`${WEB}/api/approvals`, {
        data: {
          type: 'request_board_approval',
          payload: { title, summary: 'Ship the **guild rail**.', recommended_action: 'Approve it.', next_action_on_approval: 'Merge.', risks: ['- Layout shift'] },
          issue_ids: [issue.id],
        },
      })
      if (!r.ok()) throw new Error(`request approval: ${r.status()} ${await r.text()}`)
      return ((await r.json()) as { approval: Approval }).approval
    }
    const first = await ask(`${prefix} first`)
    const card = (a: Approval) => page.locator(`[data-slot="card"][data-approval="${a.id}"]`)

    await page.goto(`${WEB}/#/approvals`)
    await card(first).waitFor()
    expect('#/approvals opens Pending', page.url().endsWith('#/approvals/pending'), page.url())
    expect('Pending shows its count', Number(await page.getByTestId('approvals-pending-count').textContent()) >= 1)
    expect('the card shows the kind, the subject and the recommended action', (await card(first).textContent())!.includes('Board Approval') && (await card(first).getByText('Approve it.').isVisible()))
    expect('a leading list marker is dropped from a Risk', await card(first).getByText('Layout shift', { exact: true }).isVisible())

    await card(first).getByRole('button', { name: 'Approve' }).click()
    await page.waitForURL(new RegExp(`#/approvals/${first.id}\\?resolved=approved$`))
    await page.getByTestId('approval-confirmed').waitFor()
    expect('Approve opens the Approval page with the banner', true)
    expect('the banner offers the linked Issue', await page.getByRole('button', { name: 'Review linked issue' }).isVisible())
    expect('the page lists the Linked issue', await page.getByTestId('linked-issues').getByText(issue.identifier).isVisible())
    await page.getByTestId('linked-issues').getByRole('link', { name: new RegExp(issue.identifier) }).click()
    await page.waitForURL(new RegExp(`#/issues/${issue.identifier}$`))
    expect('the Linked issue links back to the Issue', true)

    await page.goto(`${WEB}/#/approvals/all`)
    await card(first).and(page.locator('[data-status="approved"]')).waitFor()
    expect('All lists it approved', true)
    await page.getByRole('tab', { name: /Pending/ }).click()
    await page.waitForURL(/#\/approvals\/pending$/)
    await card(first).waitFor({ state: 'detached' })
    expect('Pending no longer lists it', true)

    const second = await ask(`${prefix} second`)
    await page.goto(`${WEB}/#/approvals/${second.id}`)
    const detail = page.locator(`[data-approval="${second.id}"]`).first()
    await detail.waitFor()
    await page.getByRole('textbox', { name: 'Decision note' }).fill('Add a rollback plan.')
    await page.getByRole('button', { name: 'Request revision' }).click()
    await page.locator(`[data-approval="${second.id}"][data-status="revision_requested"]`).waitFor()
    expect('Request revision shows the Decision note', !!(await page.getByTestId('decision-note').textContent())?.includes('Add a rollback plan.'), await page.getByTestId('decision-note').textContent())
    await page.getByRole('textbox', { name: 'Comment' }).fill('Rollback plan is in the Issue.')
    await page.getByRole('button', { name: 'Post comment' }).click()
    await page.getByRole('region', { name: 'Comments' }).getByText('Rollback plan is in the Issue.').waitFor()
    await page.reload()
    await page.getByRole('heading', { name: 'Comments (1)' }).waitFor()
    expect('the comment stays after a reload', true)
    await page.getByRole('button', { name: 'Resubmit' }).click()
    const resubmitDialog = page.getByRole('dialog', { name: 'Resubmit approval' })
    const titleField = resubmitDialog.getByRole('textbox', { name: 'Title' })
    expect('Resubmit opens the request prefilled', (await titleField.inputValue()) === `${prefix} second`, await titleField.inputValue())
    await titleField.fill(`${prefix} second, with rollback`)
    await resubmitDialog.getByRole('button', { name: 'Resubmit' }).click()
    await page.locator(`[data-approval="${second.id}"][data-status="pending"]`).waitFor()
    expect('Resubmit makes it pending again', await page.getByRole('button', { name: 'Request revision' }).isVisible())
    expect('Resubmit sends the edited request', await page.getByRole('heading', { name: `Board Approval: ${prefix} second, with rollback` }).isVisible())
    await page.goto(`${WEB}/#/approvals/pending`)
    await card(second).getByRole('button', { name: 'Reject' }).click()
    await card(second).waitFor({ state: 'detached' })
    expect('Reject on its card takes it off Pending', page.url().endsWith('#/approvals/pending'), page.url())
    expect('and it is rejected', (await all()).find((a) => a.id === second.id)?.status === 'rejected')

    await page.goto(`${WEB}/#/issues/${issue.identifier}`)
    await page.getByRole('button', { name: 'More issue actions' }).click()
    await page.getByRole('menuitem', { name: 'Request approval' }).click()
    const dialog = page.getByRole('dialog', { name: 'Request approval' })
    await dialog.getByRole('textbox', { name: 'Title' }).fill(`${prefix} third`)
    await dialog.getByRole('textbox', { name: 'Recommended action' }).fill('Ship it.')
    await dialog.getByRole('textbox', { name: 'Risks' }).fill('Downtime\n\n  Cache misses  ')
    await dialog.getByRole('button', { name: 'Request approval' }).click()
    await dialog.waitFor({ state: 'detached' })
    const third = (await all()).find((a) => a.payload.title === `${prefix} third`)!
    expect('the dialog requests the Approval', !!third && third.status === 'pending')
    const thirdRisks = (third.payload as { risks?: string[] }).risks
    expect('empty lines are dropped from the Risks', JSON.stringify(thirdRisks) === '["Downtime","Cache misses"]', JSON.stringify(thirdRisks))
    const approvalsOnIssue = page.getByRole('region', { name: 'Approvals' })
    await approvalsOnIssue.locator(`[data-approval="${third.id}"]`).waitFor()
    expect('its card appears on the Issue page', true)
    await approvalsOnIssue.locator(`[data-approval="${third.id}"]`).getByRole('button', { name: 'Approve' }).click()
    await approvalsOnIssue.locator(`[data-approval="${third.id}"][data-status="approved"]`).waitFor()
    expect('Approve on the Issue page decides it there', true)

    await page.goto(`${WEB}/#/activity?entity=approval`)
    const created = page.locator('[data-activity="approval.created"]', { hasText: `${prefix} third` }).first()
    await created.waitFor()
    expect('the Activity lists approval.approved for it', await page.locator('[data-activity="approval.approved"]', { hasText: `${prefix} third` }).first().isVisible())
    expect('the Entity filter keeps Approvals', page.url().endsWith('#/activity?entity=approval'), page.url())
    await created.getByRole('link').click()
    await page.waitForURL(new RegExp(`#/approvals/${third.id}$`))
    expect('an approval event links to the Approval page', true)

    type Badges = { inbox: number; approvals: number }
    const counts = async () => (await (await page.request.get(`${WEB}/api/sidebar-badges`)).json()) as Badges
    const fourth = await ask(`${prefix} inbox`)
    const before = await counts()
    expect('/api/sidebar-badges counts the waiting Approval', before.approvals >= 1 && before.inbox >= before.approvals, before)
    const inboxRow = page.locator(`[data-slot="approval-row"][data-approval="${fourth.id}"]`)
    const badge = page.getByRole('navigation', { name: 'Main' }).locator('a[href="#/inbox"]').getByTestId('sidebar-nav-badge')
    await page.goto(`${WEB}/#/inbox/mine`)
    await inboxRow.waitFor()
    expect('Mine shows the waiting Approval', await inboxRow.getByText(`Board Approval: ${prefix} inbox`, { exact: true }).isVisible())
    expect('the Inbox badge counts it', (await badge.textContent())?.trim() === String(before.inbox), await badge.textContent())
    await page.goto(`${WEB}/#/inbox/unread`)
    await inboxRow.waitFor()
    await inboxRow.getByRole('button', { name: 'Approve' }).click()
    await inboxRow.waitFor({ state: 'detached' })
    expect('approving it takes it off Unread', true)
    const left = before.inbox - 1
    if (left > 0) await page.waitForFunction(([n]) => document.querySelector('a[href="#/inbox"] [data-testid="sidebar-nav-badge"]')?.textContent?.trim() === n, [String(left)])
    else await badge.waitFor({ state: 'detached' })
    expect('and the badge drops', true)
    const after = await counts()
    expect('/api/sidebar-badges drops too', after.approvals === before.approvals - 1 && after.inbox === left, after)
    expect('the Approval is approved', (await all()).find((a) => a.id === fourth.id)?.status === 'approved')
    await page.goto(`${WEB}/`)
    const pendingCard = page.getByRole('link', { name: /Pending Approvals/ })
    await pendingCard.waitFor()
    expect('the Dashboard counts Pending Approvals', !!(await pendingCard.textContent())?.includes(String(after.approvals)), await pendingCard.textContent())
    await pendingCard.click()
    await page.waitForURL(/#\/approvals\/pending$/)
    expect('the card opens #/approvals/pending', true)

    // A Member decides nothing, but reads and comments; a Linked issue in a
    // Project their Role may not view is left out for them.
    const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: `${prefix} hidden` } })).json()) as { project: { id: number } }
    const { issue: hidden } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title: `${prefix} hidden`, project_id: project.id } })).json()) as { issue: Issue }
    const linked = await page.request.post(`${WEB}/api/approvals`, {
      data: { type: 'request_board_approval', payload: { title: `${prefix} member` }, issue_ids: [issue.id, hidden.id] },
    })
    if (!linked.ok()) throw new Error(`request approval: ${linked.status()} ${await linked.text()}`)
    const fifth = ((await linked.json()) as { approval: Approval }).approval
    const undeny = await denyView(page, project.id, 'Member')
    const m = await invited(page, 'member')
    await m.page.goto(`${WEB}/#/approvals/pending`)
    await m.page.locator(`[data-slot="card"][data-approval="${fifth.id}"]`).waitFor()
    expect('a Member sees the Approvals', true)
    expect('without Approve or Reject on the card', (await m.page.locator(`[data-slot="card"][data-approval="${fifth.id}"]`).getByRole('button', { name: /^(Approve|Reject)$/ }).count()) === 0)
    await m.page.goto(`${WEB}/#/approvals/${fifth.id}`)
    const memberLinks = m.page.getByTestId('linked-issues')
    await memberLinks.getByText(issue.identifier).waitFor()
    expect('nor Approve, Reject or Request revision on its page', (await m.page.getByRole('button', { name: /^(Approve|Reject|Request revision)$/ }).count()) === 0)
    expect('the Linked issue in the denied Project is left out', !(await memberLinks.getByText(hidden.identifier).isVisible()))
    await m.page.getByRole('textbox', { name: 'Comment' }).fill('Looks right to me.')
    await m.page.getByRole('button', { name: 'Post comment' }).click()
    await m.page.getByRole('region', { name: 'Comments' }).getByText('Looks right to me.').waitFor()
    expect('the Member can comment', true)
    await page.goto(`${WEB}/#/approvals/${fifth.id}`)
    await page.getByTestId('linked-issues').getByText(hidden.identifier).waitFor()
    expect('the owner still sees both Linked issues', true)
    await page.request.post(`${WEB}/api/approvals/${fifth.id}/reject`, { data: {} })
    await m.leave()
    await undeny()
    await page.request.delete(`${WEB}/api/issues/${hidden.id}`)
    await page.request.delete(`${WEB}/api/projects/${project.id}`)

    await page.request.delete(`${WEB}/api/issues/${issue.id}`)
    await page.close()
  },

  runs: async () => {
    const page = await signedIn()
    const name = 'Runs e2e agent'
    const titles = ['Runs e2e: say hello', 'Runs e2e: [slow] count']
    for (const i of (await issues(page)).filter((i) => titles.includes(i.title))) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
    for (const a of agents.filter((a) => a.name === name)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)

    const me = ((await (await page.request.get(`${WEB}/api/me`)).json()) as { member: { name: string } }).member
    const hire = (await (await page.request.post(`${WEB}/api/agents`, { data: { name, job: 'engineer', icon: 'bot' } })).json()) as { agent: { id: number; approval_id: number } }
    await page.request.post(`${WEB}/api/approvals/${hire.agent.approval_id}/approve`, { data: {} })
    const made: Issue[] = []
    for (const title of titles) {
      const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, assignee_agent_id: hire.agent.id } })).json()) as { issue: Issue }
      made.push(issue)
    }
    const [hello, slow] = made

    // Run queues a Run that waits for the owner's desktop, none running yet.
    await page.goto(`${WEB}/#/issues/${hello.identifier}`)
    const header = page.getByTestId('issue-detail-header')
    await header.getByRole('button', { name: 'Run', exact: true }).click()
    const live = page.getByTestId('live-run')
    await live.getByTestId('run-waiting').waitFor()
    const waiting = await live.getByTestId('run-waiting').innerText()
    expect("the queued Run waits for the hirer's desktop", waiting.includes(`Waiting for ${me.name}'s desktop`), waiting)
    expect('it is listed in Runs as Queued', await page.getByTestId('run-ledger').locator('[data-run-status="queued"]').isVisible())

    // The runner claims it and the Transcript grows live.
    const desktop = await desktopRunner(page)
    try {
      const blocks = live.locator('[data-testid="run-transcript"][data-live="true"] [data-block]')
      await blocks.first().waitFor({ timeout: 120_000 })
      const first = await blocks.count()
      const grew = await page
        .waitForFunction((n) => document.querySelectorAll('[data-testid="live-run"] [data-block]').length > n, first, { timeout: 20_000 })
        .then(() => true, () => false)
      expect('the Transcript grows live on the Issue page', grew, first)
      expect('the init line shows the model', (await live.locator('[data-block="init"]').first().innerText()).includes('model '))
      expect('the assistant text shows', await live.locator('[data-block="assistant"]').first().isVisible())
      await live.waitFor({ state: 'detached', timeout: 60_000 })
      const row = page.getByTestId('run-ledger').locator('[data-run]').first()
      await row.locator('[data-run-status="succeeded"]').waitFor({ timeout: 10_000 })
      expect('the Run ends Succeeded in Runs', true)
      await row.getByRole('button', { name: /^Run #/ }).click()
      const footer = row.locator('[data-block="result"]')
      await footer.waitFor()
      const tokens = await footer.getByTestId('run-tokens').innerText()
      expect('its result footer shows the tokens', /[\d.]+k? in · [\d.]+k? out tokens/.test(tokens), tokens)
      expect('and the cost as an equivalent', (await footer.innerText()).includes('equivalent'))
      expect('a tool call shows as one row', (await row.locator('[data-block="tool"]').count()) > 0)

      // The Agent page lists it, linking to the Issue.
      await page.goto(`${WEB}/#/agents/${hire.agent.id}`)
      await page.getByTestId('run-ledger').getByRole('link', { name: new RegExp(hello.identifier) }).waitFor()
      expect('the Agent page lists the Run with its Issue', true)

      // A [slow] Run is cancelled from the page.
      await page.goto(`${WEB}/#/issues/${slow.identifier}`)
      await header.getByRole('button', { name: 'Run', exact: true }).click()
      await live.locator('[data-run-status="running"]').waitFor({ timeout: 60_000 })
      await live.locator('[data-block]').first().waitFor({ timeout: 20_000 })
      await live.getByRole('button', { name: 'Cancel' }).click()
      await page.getByTestId('run-ledger').locator('[data-run]').first().locator('[data-run-status="cancelled"]').waitFor({ timeout: 15_000 })
      expect('the cancelled Run ends Cancelled', true)
      expect('and leaves the live block', !(await live.isVisible()))
    } finally {
      await desktop.stop()
      for (const i of made) await page.request.delete(`${WEB}/api/issues/${i.id}`)
      await page.request.post(`${WEB}/api/agents/${hire.agent.id}/terminate`)
      await page.close()
    }
  },

  'agent-actor': async () => {
    const page = await signedIn()
    const name = 'Actor e2e agent'
    const title = 'Actor e2e: check out and comment'
    for (const i of (await issues(page)).filter((i) => i.title === title)) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
    for (const a of agents.filter((a) => a.name === name)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
    // The seeded Member Role, so its Run key may manage work.
    const { roles } = (await (await page.request.get(`${WEB}/api/roles`)).json()) as { roles: { id: number; name: string }[] }
    const member = roles.find((r) => r.name === 'Member')!
    const hire = (await (await page.request.post(`${WEB}/api/agents`, { data: { name, job: 'engineer', icon: 'bot', role_ids: [member.id] } })).json()) as { agent: { id: number; approval_id: number } }
    await page.request.post(`${WEB}/api/approvals/${hire.agent.approval_id}/approve`, { data: {} })
    // In backlog, so the assignment wakes nobody and only the Run below exists.
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, status: 'backlog', assignee_agent_id: hire.agent.id } })).json()) as { issue: Issue }

    // A Desktop key minted and approved as the Desktop app's sign-in does,
    // then the Run claimed with it: its answer carries the Run key.
    const token = 'bky_signin_' + randomBytes(24).toString('hex')
    const desktopKey = 'bky_desk_' + randomBytes(24).toString('hex')
    const signIn = (await (
      await page.request.post(`${WEB}/api/desktop-sign-ins`, { data: { client_name: 'actor-e2e', token, desktop_key_hash: createHash('sha256').update(desktopKey).digest('hex') } })
    ).json()) as { id: number }
    const approved = (await (await page.request.post(`${WEB}/api/desktop-sign-ins/${signIn.id}/approve`, { data: { token } })).json()) as { desktop_id: number }
    const desktop = { Authorization: `Bearer ${desktopKey}` }
    try {
      const { run } = (await (await page.request.post(`${WEB}/api/agents/${hire.agent.id}/runs`, { data: { issue_id: issue.id } })).json()) as { run: { id: number } }
      const claimed = await page.request.post(`${WEB}/api/runs/${run.id}/claim`, { headers: desktop, data: {} })
      const runKey = ((await claimed.json()) as { run: { run_key?: string } }).run.run_key ?? ''
      expect('the claim answers a Run key', claimed.ok() && runKey.startsWith('bky_run_'), claimed.status())
      const agent = { Authorization: `Bearer ${runKey}` }
      const checkout = await page.request.post(`${WEB}/api/issues/${issue.id}/checkout`, { headers: agent, data: {} })
      expect('the Run key checks the Issue out', checkout.ok(), `${checkout.status()} ${await checkout.text()}`)
      const comment = await page.request.post(`${WEB}/api/issues/${issue.id}/comments`, { headers: agent, data: { body: 'Picked this up from the e2e.' } })
      expect('and writes a Comment', comment.ok(), comment.status())
      const doc = await page.request.put(`${WEB}/api/issues/${issue.id}/documents/plan`, { headers: agent, data: { title: 'Plan', body: '1. Check out\n2. Comment' } })
      expect('and saves a plan document', doc.ok(), `${doc.status()} ${await doc.text()}`)

      await page.goto(`${WEB}/#/issues/${issue.identifier}`)
      const row = page.locator('[data-property-row="Checkout"]')
      await row.waitFor()
      const rowText = await row.innerText()
      expect('the properties show "Checked out by" the Agent and its Run', rowText.includes('Checked out by') && rowText.includes(name) && rowText.includes(`Run #${run.id}`), rowText)
      const card = page.getByRole('region', { name: 'Comments' }).locator('[data-comment]', { hasText: 'Picked this up from the e2e.' })
      await card.waitFor()
      const author = card.locator(`a[data-actor-agent="${hire.agent.id}"]`)
      expect("the Comment shows the Agent's name", (await author.innerText()).includes(name), await card.innerText())
      expect("and links to the Agent's page", ((await author.getAttribute('href')) ?? '').endsWith(`/agents/${hire.agent.id}`))
      expect('and has no Edit or Delete for people', (await card.getByRole('button', { name: /Edit comment|Delete comment/ }).count()) === 0)
      await page.getByRole('button', { name: 'Revision history of plan' }).click()
      const revision = page.locator('[data-revision="1"]')
      await revision.waitFor()
      expect('the revision menu names the Agent', (await revision.innerText()).includes(`• ${name}`), await revision.innerText())
      await page.keyboard.press('Escape')

      await page.goto(`${WEB}/#/issues/${issue.identifier}?tab=activity`)
      const commented = page.locator('[data-activity="issue.comment_added"]').first()
      await commented.waitFor()
      expect("the Issue's Activity names the Agent", (await commented.locator(`[data-actor-agent="${hire.agent.id}"]`).count()) === 1, await commented.innerText())
      const checkedOut = page.locator('[data-activity="issue.checked_out"]').first()
      expect('with "checked out"', (await checkedOut.innerText()).includes('checked out'), await checkedOut.innerText())

      await page.goto(`${WEB}/#/activity?actor=agent:${hire.agent.id}`)
      await page.getByRole('list', { name: 'Activity' }).waitFor()
      const feed = page.getByRole('list', { name: 'Activity' }).locator('li')
      const lines = await feed.allInnerTexts()
      expect("the Guild's Activity filtered to the Agent lists only it", lines.length >= 3 && lines.every((l) => l.includes(name)), lines)
      expect('and the Actor select names it', (await page.locator('[aria-label="Actor"]').innerText()).includes(name))

      const finished = await page.request.post(`${WEB}/api/runs/${run.id}/finish`, { headers: desktop, data: { status: 'succeeded', exit_code: 0, usage: {} } })
      expect('the Desktop finishes the Run', finished.ok(), `${finished.status()} ${await finished.text()}`)
      await page.goto(`${WEB}/#/issues/${issue.identifier}`)
      await page.getByRole('region', { name: 'Comments' }).locator('[data-comment]').first().waitFor()
      expect('the Checkout row is gone once the Run is final', (await page.locator('[data-property-row="Checkout"]').count()) === 0)
    } finally {
      await page.request.delete(`${WEB}/api/desktops/${approved.desktop_id}`)
      await page.request.delete(`${WEB}/api/issues/${issue.id}`)
      await page.request.post(`${WEB}/api/agents/${hire.agent.id}/terminate`)
      await page.close()
    }
  },

  'agent-api': async () => {
    const page = await signedIn()
    const name = 'Agent API e2e agent'
    const title = 'Agent API e2e: check out and comment'
    const said = 'Checked this out through the MCP server.'
    for (const i of (await issues(page)).filter((i) => i.title === title)) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
    for (const a of agents.filter((a) => a.name === name)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)
    // The seeded Member Role holds manage_work, so the Agent may write.
    const { roles } = (await (await page.request.get(`${WEB}/api/roles`)).json()) as { roles: { id: number; name: string }[] }
    const member = roles.find((r) => r.name === 'Member')!
    const hire = (await (await page.request.post(`${WEB}/api/agents`, { data: { name, job: 'engineer', icon: 'bot', role_ids: [member.id] } })).json()) as { agent: { id: number; approval_id: number } }
    await page.request.post(`${WEB}/api/approvals/${hire.agent.approval_id}/approve`, { data: {} })
    // In backlog, so the assignment wakes nobody and only the pressed Run exists;
    // the description names the Issue by its id, known only once it exists.
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, status: 'backlog', assignee_agent_id: hire.agent.id } })).json()) as { issue: Issue }
    const args = (extra: object) => JSON.stringify({ issueId: String(issue.id), ...extra })
    await page.request.patch(`${WEB}/api/issues/${issue.id}`, {
      data: { description: `[mcp bakeryCheckoutIssue ${args({})}] [mcp bakeryAddComment ${args({ body: said })}] [slow]` },
    })

    const desktop = await desktopRunner(page)
    try {
      await page.goto(`${WEB}/#/issues/${issue.identifier}`)
      await page.getByTestId('issue-detail-header').getByRole('button', { name: 'Run', exact: true }).click()
      const live = page.getByTestId('live-run')
      await live.locator('[data-run-status="running"]').waitFor({ timeout: 120_000 })

      // The page re-reads the Issue while the Run is live.
      const row = page.locator('[data-property-row="Checkout"]')
      await row.waitFor({ timeout: 60_000 })
      const rowText = await row.innerText()
      expect('while it runs the Issue is "Checked out by" the Agent', rowText.includes('Checked out by') && rowText.includes(name), rowText)
      const status = (await (await page.request.get(`${WEB}/api/issues/${issue.id}`)).json()) as { issue: Issue }
      expect('and In progress', status.issue.status === 'in_progress', status.issue.status)
      const card = page.getByRole('region', { name: 'Comments' }).locator('[data-comment]', { hasText: said })
      await card.waitFor({ timeout: 30_000 })
      expect("the Comment is in the thread under the Agent's name", (await card.locator(`a[data-actor-agent="${hire.agent.id}"]`).innerText()).includes(name), await card.innerText())
      const tools = live.locator('[data-block="tool"]')
      await tools.nth(1).waitFor({ timeout: 30_000 })
      const toolText = (await tools.allTextContents()).join('\n')
      expect('the Transcript shows both tool calls', toolText.includes('bakeryCheckoutIssue') && toolText.includes('bakeryAddComment'), toolText)
      expect('the Run is still live', await live.locator('[data-run-status="running"]').isVisible())

      await live.waitFor({ state: 'detached', timeout: 90_000 })
      await page.getByTestId('run-ledger').locator('[data-run]').first().locator('[data-run-status="succeeded"]').waitFor({ timeout: 15_000 })
      expect('the Run ends Succeeded', true)
      await page.goto(`${WEB}/#/issues/${issue.identifier}?tab=activity`)
      const commented = page.locator('[data-activity="issue.comment_added"]').first()
      await commented.waitFor()
      expect("the Issue's Activity names the Agent", (await commented.locator(`[data-actor-agent="${hire.agent.id}"]`).count()) === 1, await commented.innerText())
      expect('the Checkout row is gone once the Run is final', (await page.locator('[data-property-row="Checkout"]').count()) === 0)
    } finally {
      await desktop.stop()
      await page.request.delete(`${WEB}/api/issues/${issue.id}`)
      await page.request.post(`${WEB}/api/agents/${hire.agent.id}/terminate`)
      await page.close()
    }
  },

  'work-products': async () => {
    const page = await signedIn()
    const title = 'Work products e2e'
    for (const i of (await issues(page)).filter((i) => i.title === title)) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, status: 'backlog' } })).json()) as { issue: Issue & { created_by: { id: number; name: string } } }
    try {
      await page.goto(`${WEB}/#/issues/${issue.identifier}`)
      await page.getByTestId('issue-detail-header').waitFor()
      expect('no Work products section without any', (await page.getByRole('region', { name: 'Work products' }).count()) === 0)

      const pr = 'https://git.example.test/guild/web/pulls/7'
      const preview = 'https://pr-7.web.example.test'
      sql(`INSERT INTO issue_work_products (guild_id, issue_id, application_id, type, provider, external_id, title, url, status, created_by_member_id, created_at, updated_at)
        SELECT guild_id, id, 4242, 'pull_request', 'forgejo', '7', 'Wire the guild rail', '${pr}', 'open', ${issue.created_by.id}, now(), now() FROM issues WHERE id = ${issue.id};
        INSERT INTO issue_work_products (guild_id, issue_id, application_id, type, provider, external_id, title, url, status, created_at, updated_at)
        SELECT guild_id, id, 4242, 'preview_url', 'forgejo', '7', 'Preview of #7', '${preview}', 'deploying', now(), now() FROM issues WHERE id = ${issue.id}`)
      await page.reload()
      const region = page.getByRole('region', { name: 'Work products' })
      const card = region.locator('[data-work-product="pull_request"]')
      const previewCard = region.locator('[data-work-product="preview_url"]')
      await card.waitFor()
      const text = await card.innerText()
      expect('the Pull request card shows its number, title and git host', text.includes('#7 Wire the guild rail') && text.includes('Forgejo'), text)
      expect('its pill says Open', (await card.locator('[data-pill]').innerText()).trim() === 'Open', await card.locator('[data-pill]').innerText())
      expect('it names who opened it', text.includes(issue.created_by.name), text)
      const open = card.getByRole('link', { name: 'Open pull request' })
      expect('"Open pull request" opens the git host in a new tab', (await open.getAttribute('href')) === pr && (await open.getAttribute('target')) === '_blank')
      expect('the Preview card shows Deploying', (await previewCard.locator('[data-pill]').innerText()).trim() === 'Deploying', await previewCard.innerText())
      expect("the Preview card links the Preview's address", (await previewCard.locator('[data-preview-link]').getAttribute('href')) === preview)
      expect("and the Application's Preview Deployments", (await previewCard.getByRole('link', { name: 'Preview deployments' }).getAttribute('href')) === '#/applications/4242/preview-deployments')

      sql(`UPDATE issue_work_products SET status = CASE type WHEN 'pull_request' THEN 'merged' ELSE 'ready' END WHERE issue_id = ${issue.id}`)
      await page.reload()
      await card.locator('[data-pill]', { hasText: 'Merged' }).waitFor()
      expect('after the merge the pill says Merged', true)
      expect('and the Preview Ready', (await previewCard.locator('[data-pill]').innerText()).trim() === 'Ready', await previewCard.innerText())
    } finally {
      await page.request.delete(`${WEB}/api/issues/${issue.id}`)
      await page.close()
    }
  },

  'pull-request': async () => {
    const page = await signedIn()
    const name = 'Pull request e2e agent'
    const title = 'Pull request e2e: change the page'
    const repo = `pr-e2e-${Date.now()}`
    for (const i of (await issues(page)).filter((i) => i.title === title)) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
    for (const a of agents.filter((a) => a.name === name)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)

    const forgejo = await forgejoUp(repo)
    const work = mkdtempSync(join(tmpdir(), 'bakery-pr-e2e-'))
    const cleanups: (() => Promise<unknown> | unknown)[] = [() => forgejo.down(), () => rmSync(work, { recursive: true, force: true })]
    try {
      // A public repository (the Application clones it over HTTP without a
      // Deploy key) serving one page from busybox, as previews/test.sh's.
      await forgejo.api('POST', '/user/repos', { name: repo, private: false, default_branch: 'main' })
      const clone = join(work, 'repo')
      run('git', ['init', '-q', '-b', 'main', clone])
      writeFileSync(join(clone, 'Dockerfile'), 'FROM docker.io/library/busybox:stable\nCOPY index.html /www/index.html\nEXPOSE 8080\nCMD ["httpd", "-f", "-p", "8080", "-h", "/www"]\n')
      const commit = (text: string) => {
        writeFileSync(join(clone, 'index.html'), `${text}\n`)
        run('git', ['add', '-A'], clone)
        run('git', ['-c', 'user.name=E2E Tester', '-c', 'user.email=e2e@example.test', 'commit', '-qm', text], clone)
        run('git', ['push', '-q', forgejo.remote(repo), 'main'], clone)
      }
      commit('version main')

      const { project } = (await (await page.request.post(`${WEB}/api/projects`, { data: { name: repo } })).json()) as { project: { id: number } }
      cleanups.unshift(() => page.request.delete(`${WEB}/api/projects/${project.id}`))
      const env = ((await (await page.request.get(`${WEB}/api/projects/${project.id}`)).json()) as { project: { environments: { id: number }[] } }).project.environments[0]
      const made = await page.request.post(`${WEB}/api/environments/${env.id}/applications`, {
        data: { name: repo, git_url: `${FORGEJO}/${forgejo.user}/${repo}.git`, git_branch: 'main', port: 8080 },
      })
      if (!made.ok()) throw new Error(`the Application: ${made.status()} ${await made.text()}`)
      const { application: app } = (await made.json()) as { application: { id: number; slug: string } }
      cleanups.unshift(async () => {
        await page.request.delete(`${WEB}/api/applications/${app.id}`)
        const images = run('podman', ['images', '--format', '{{.Repository}}:{{.Tag}}']).split('\n').filter((i) => i.startsWith(`localhost/bakery/${app.slug}:`))
        if (images.length) spawnSync('podman', ['rmi', '-f', ...images])
      })

      // Previews on with the Git host token, Auto-deploy off so only the
      // Deploy below builds main; the hook sends push and pull request events.
      const hooked = await page.request.patch(`${WEB}/api/applications/${app.id}/webhook`, { data: { auto_deploy: false, previews: true, git_host_token: forgejo.token } })
      if (!hooked.ok()) throw new Error(`the webhook: ${hooked.status()} ${await hooked.text()}`)
      const { webhook } = (await hooked.json()) as { webhook: { path: string; secret: string } }
      await forgejo.api('POST', `/repos/${forgejo.user}/${repo}/hooks`, {
        type: 'forgejo',
        active: true,
        events: ['push', 'pull_request', 'pull_request_sync'],
        config: { url: `http://127.0.0.1:4910${webhook.path}`, content_type: 'json', secret: webhook.secret },
      })
      // One verified call, so The Bakery knows the git host is Forgejo.
      commit('version main 2')
      const deployed = async () => {
        const { deployments } = (await (await page.request.get(`${WEB}/api/applications/${app.id}/deployments`)).json()) as { deployments: { status: string; error?: string }[] }
        if (deployments[0]?.status === 'failed') throw new Error(`the deployment failed: ${deployments[0].error}`)
        return deployments[0]?.status === 'finished'
      }
      await page.request.post(`${WEB}/api/applications/${app.id}/deploy`, { data: {} })
      await waitFor(300, 'the first deployment', deployed)
      expect('the Application on the Forgejo stand-in deploys', true)

      // A scratch Agent with the Member Role (manage_work), its Issue naming
      // the Application; the description is set once the Issue's id is known.
      const { roles } = (await (await page.request.get(`${WEB}/api/roles`)).json()) as { roles: { id: number; name: string }[] }
      const member = roles.find((r) => r.name === 'Member')!
      const hire = (await (await page.request.post(`${WEB}/api/agents`, { data: { name, job: 'engineer', icon: 'bot', role_ids: [member.id] } })).json()) as { agent: { id: number; approval_id: number } }
      cleanups.unshift(() => page.request.post(`${WEB}/api/agents/${hire.agent.id}/terminate`))
      await page.request.post(`${WEB}/api/approvals/${hire.agent.approval_id}/approve`, { data: {} })
      const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, status: 'todo', project_id: project.id } })).json()) as { issue: Issue }
      cleanups.unshift(() => page.request.delete(`${WEB}/api/issues/${issue.id}`))
      const id = JSON.stringify({ issueId: String(issue.id) })
      const named = await page.request.patch(`${WEB}/api/issues/${issue.id}`, {
        data: {
          application_id: app.id,
          description: `[mcp bakeryCheckoutIssue ${id}] [git commit index.html version-agent] [git push] [mcp bakeryOpenPullRequest ${id}] [mcp bakeryReleaseIssue ${id}]`,
        },
      })
      expect('the Issue names the Application', named.ok() && ((await named.json()) as { issue: Issue }).issue.application?.id === app.id, named.status())

      // The Runner pushes with Forgejo credentials from its environment
      // only, as a person's git credential setup would give them.
      const desktop = await desktopRunner(page, {
        BAKERY_STANDIN_DELAY: '200ms',
        GIT_CONFIG_COUNT: '1',
        GIT_CONFIG_KEY_0: `url.${forgejo.remote(repo).replace(`${repo}.git`, '')}.insteadOf`,
        GIT_CONFIG_VALUE_0: `${FORGEJO}/${forgejo.user}/`,
      })
      cleanups.unshift(() => desktop.stop())
      const assigned = await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { assignee_agent_id: hire.agent.id } })
      expect('assigning the Issue to the Agent answers 200', assigned.ok(), assigned.status())

      // The page reloaded every half second from the assignment on, so the
      // Preview's short Deploying is seen too; it ends once the Preview is
      // Ready, which it can only be after the Run opened the Pull request.
      const region = page.getByRole('region', { name: 'Work products' })
      const card = region.locator('[data-work-product="pull_request"]')
      const previewCard = region.locator('[data-work-product="preview_url"]')
      const pill = (c: typeof card) => c.locator('[data-pill]').innerText().then((t) => t.trim(), () => '')
      const seen = new Set<string>()
      let prPill = ''
      const until = Date.now() + 300_000
      await page.goto(`${WEB}/#/issues/${issue.identifier}`)
      for (;;) {
        await page.reload()
        await page.getByTestId('issue-detail-header').waitFor()
        if (!prPill && (await card.count())) prPill = await pill(card)
        const p = (await previewCard.count()) ? await pill(previewCard) : ''
        if (p) seen.add(p)
        if (p === 'Failed') throw new Error('the Preview failed')
        if (p === 'Ready') break
        if (Date.now() > until) throw new Error(`timed out waiting for the Preview card to say Ready (saw ${[...seen]})`)
        await new Promise((r) => setTimeout(r, 500))
      }
      const ledger = page.getByTestId('run-ledger')
      await ledger.locator('[data-run]').first().locator('[data-run-status="succeeded"], [data-run-status="failed"]').waitFor({ timeout: 180_000 })
      const ended = await ledger.locator('[data-run]').first().locator('[data-run-status]').getAttribute('data-run-status')
      expect('the Run succeeds', ended === 'succeeded', ended)

      const branch = `bakery/${issue.identifier.toLowerCase()}`
      const pulls = (await forgejo.api('GET', `/repos/${forgejo.user}/${repo}/pulls?state=open`)) as { number: number; head: { ref: string } }[]
      const pull = pulls.find((p) => p.head.ref === branch)
      expect(`Forgejo has the Pull request from ${branch}`, !!pull, pulls)
      expect('the Pull request card said Open from the start', prPill === 'Open', prPill)
      expect('it shows the number', (await card.innerText()).includes(`#${pull?.number}`), await card.innerText())
      expect('the Preview card went Deploying, then Ready', seen.has('Deploying') && seen.has('Ready'), [...seen])
      const address = (await previewCard.locator('[data-preview-link]').getAttribute('href')) ?? ''
      const host = new URL(address).hostname
      const served = () => spawnSync('curl', ['-sf', '-k', '--resolve', `${host}:4943:127.0.0.1`, address]).stdout.toString().trim()
      await waitFor(30, `version-agent on ${address}`, () => served() === 'version-agent')
      expect("the Preview's address serves the Agent's change", true)

      // Closed on Forgejo: the cards turn Closed and Removed.
      await forgejo.api('PATCH', `/repos/${forgejo.user}/${repo}/pulls/${pull!.number}`, { state: 'closed' })
      await waitFor(90, 'the cards to say Closed and Removed', async () => {
        await page.reload()
        await page.getByTestId('issue-detail-header').waitFor()
        await card.waitFor()
        return (await pill(card)) === 'Closed' && (await pill(previewCard)) === 'Removed'
      })
      expect('closing the Pull request turns the cards Closed and Removed', true)
    } finally {
      for (const undo of cleanups) await undo()
      await page.close()
    }
  },

  wakes: async () => {
    const page = await signedIn()
    const name = 'Wakes e2e agent'
    const title = 'Wakes e2e: wake on assignment'
    for (const i of (await issues(page)).filter((i) => i.title === title)) await page.request.delete(`${WEB}/api/issues/${i.id}`)
    const { agents } = (await (await page.request.get(`${WEB}/api/agents`)).json()) as { agents: { id: number; name: string }[] }
    for (const a of agents.filter((a) => a.name === name)) await page.request.post(`${WEB}/api/agents/${a.id}/terminate`)

    const hire = (await (await page.request.post(`${WEB}/api/agents`, { data: { name, job: 'engineer', icon: 'bot' } })).json()) as { agent: { id: number; approval_id: number } }
    await page.request.post(`${WEB}/api/approvals/${hire.agent.approval_id}/approve`, { data: {} })
    const { issue } = (await (await page.request.post(`${WEB}/api/issues`, { data: { title, status: 'todo' } })).json()) as { issue: Issue }
    try {
      const assign = await page.request.patch(`${WEB}/api/issues/${issue.id}`, { data: { assignee_agent_id: hire.agent.id } })
      expect('assigning the Issue to the Agent answers 200', assign.ok(), assign.status())

      await page.goto(`${WEB}/#/issues/${issue.identifier}`)
      const ledger = page.getByTestId('run-ledger')
      const rows = ledger.locator('[data-run]')
      await rows.first().waitFor()
      const first = rows.first()
      expect('a queued Assignment Run is listed without pressing Run', (await rows.count()) === 1 && (await first.locator('[data-run-source="assignment"]').innerText()).includes('Assignment'), await ledger.innerText())
      expect('it is queued', await first.locator('[data-run-status="queued"]').isVisible())

      const thread = page.getByRole('region', { name: 'Comments' })
      await thread.getByLabel('Comment', { exact: true }).fill('Please look at the guild rail too.')
      await page.keyboard.press('Control+Enter')
      await first.locator('[data-wake-count="2"]').waitFor()
      expect('a comment joins the queued Run (×2, still one Run)', (await rows.count()) === 1, await rows.count())

      await page.getByTestId('live-run').getByRole('button', { name: 'Cancel' }).click()
      await first.locator('[data-run-status="cancelled"]').waitFor({ timeout: 15_000 })
      await thread.getByLabel('Comment', { exact: true }).fill('And now the Issues list.')
      await page.keyboard.press('Control+Enter')
      const automation = ledger.locator('[data-run]', { has: page.locator('[data-run-source="automation"]') })
      await automation.waitFor()
      expect('a comment after it ended queues an Automation Run', (await automation.innerText()).includes('Automation') && (await rows.count()) === 2, await ledger.innerText())
    } finally {
      await page.request.delete(`${WEB}/api/issues/${issue.id}`)
      await page.request.post(`${WEB}/api/agents/${hire.agent.id}/terminate`)
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
console.log('the work flows work')
