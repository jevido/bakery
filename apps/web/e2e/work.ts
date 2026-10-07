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
//           properties panel; Add sub-issue creates one under it, listed and
//           opening its own page with the parent link; a Markdown comment
//           with a code block is posted, edited and deleted (the placeholder
//           stays); the Goal's page and the Issues list open it from
//           their rows; #/issues/DEF-99999 shows the not-found state; the scratch
//           Issues and Goal are deleted again
//
//   bun e2e/work.ts [section ...]   (task web:work; needs task dev)
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

type Issue = { id: number; identifier: string; title: string; status: string; priority: string; assignee: { id: number; name: string } | null; project: { id: number } | null }
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

/** Picks an option of a dialog chip or popover by its accessible names. */
async function pick(page: Page, scope: ReturnType<Page['getByRole']>, chip: string, option: string) {
  await scope.getByRole('button', { name: chip, exact: true }).click()
  await page.getByRole('listbox', { name: chip }).getByRole('option', { name: option, exact: true }).click()
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
    expect('Issues is in the sidebar', await page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Issues' }).isVisible())
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
