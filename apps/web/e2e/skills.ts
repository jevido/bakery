// The Skills pages in the dashboard, one section each, every flow once in the
// dark theme at 1440×900:
//   library  #/skills has Skills current in the sidebar; New skill with a
//            name derives its slug and lands on the Skill page, whose
//            Overview shows the generated SKILL.md and its frontmatter;
//            on Files, SKILL.md edited with a new description and saved
//            renames nothing but changes the header's description; Add file
//            templates/notes.md opens empty in the editor and saves into the
//            tree under templates; Delete file takes it out again; the
//            Activity holds the file's update with its path; Delete deletes
//            the Skill and lands on #/skills
//   viewer   a Member (no manage_skills) sees the Skill in the list and on
//            its page, its files too, but no New skill, Edit, Add file,
//            Delete file or Delete
//
//   bun e2e/skills.ts [section ...]   (task web:skills; needs task dev)
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

/** A Member invited for the run, signed in in a context of their own; leave removes them. */
async function invitedMember(owner: Page): Promise<{ page: Page; leave: () => Promise<void> }> {
  const { members } = (await (await owner.request.get(`${WEB}/api/members`)).json()) as { members: { id: number; email: string }[] }
  for (const m of members.filter((m) => m.email.startsWith('skills-member-'))) await owner.request.delete(`${WEB}/api/members/${m.id}`)
  const inv = await owner.request.post(`${WEB}/api/invitations`, { data: { email: `skills-member-${Date.now()}@example.test`, role: 'member' } })
  if (!inv.ok()) throw new Error(`invite: ${inv.status()}`)
  const token = ((await inv.json()) as { path: string }).path.split('/').pop()
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  const accept = await ctx.request.post(`${WEB}/api/invitations/by-token/${token}/accept`, { data: { name: 'Skills member', password: 'a long enough password' } })
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

type Skill = { id: number; slug: string; name: string; description: string; file_count: number; files?: { path: string }[] }

/** Deletes the Skills with this slug left by an earlier run. */
async function clearSkill(page: Page, slug: string) {
  const { skills } = (await (await page.request.get(`${WEB}/api/skills`)).json()) as { skills: Skill[] }
  for (const k of skills.filter((k) => k.slug === slug)) await page.request.delete(`${WEB}/api/skills/${k.id}`)
}

const skillOf = async (page: Page, id: number) => ((await (await page.request.get(`${WEB}/api/skills/${id}`)).json()) as { skill: Skill }).skill

const sections: Record<string, () => Promise<void>> = {
  async library() {
    const page = await signedIn()
    const name = 'Release notes e2e'
    const slug = 'release-notes-e2e'
    await clearSkill(page, slug)

    await page.goto(`${WEB}/#/skills`)
    const nav = page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Skills' })
    await nav.waitFor()
    expect('the sidebar has Skills, current', (await nav.getAttribute('aria-current')) === 'page', await nav.getAttribute('aria-current'))

    await page.getByRole('button', { name: 'New skill' }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByLabel('Name').fill(name)
    const derived = await dialog.getByLabel('Slug').inputValue()
    expect('the slug is derived from the name', derived === slug, derived)
    await dialog.getByLabel('Description').fill('Drafts release notes.')
    await dialog.getByRole('button', { name: 'Create skill' }).click()

    await page.waitForURL(/#\/skills\/\d+$/)
    const id = Number(page.url().split('/').pop())
    try {
      await page.getByRole('heading', { name }).waitFor()
      expect('New skill lands on the Skill page', true)
      const overview = page.getByTestId('skill-overview')
      await overview.waitFor()
      const heading = await overview.getByRole('heading', { level: 1 }).textContent()
      expect('Overview renders the generated SKILL.md', heading === name, heading)
      const front = await page.getByLabel('Frontmatter').first().textContent()
      expect('Overview shows the frontmatter', !!front?.includes(name) && front.includes('Drafts release notes.'), front)

      await page.getByRole('tab', { name: 'Files' }).click()
      await page.waitForURL(/tab=files/)
      const filePath = page.getByTestId('skill-file-path')
      expect('Files opens SKILL.md', (await filePath.textContent()) === 'SKILL.md', await filePath.textContent())
      const pane = page.getByRole('region', { name: 'File' })
      await pane.getByRole('button', { name: 'Edit' }).click()
      await pane.getByLabel('Content').fill(`---\nname: ${name}\ndescription: Writes the release notes.\n---\n\n# ${name}\n\nFill in templates/notes.md.\n`)
      await pane.getByRole('button', { name: 'Save' }).click()
      await pane.getByRole('button', { name: 'Edit' }).waitFor()
      await page.getByText('Writes the release notes.').first().waitFor()
      const saved = await skillOf(page, id)
      expect('saving SKILL.md changes the description from its frontmatter', saved.description === 'Writes the release notes.' && saved.name === name, saved)

      await page.getByRole('button', { name: 'Add file' }).click()
      const add = page.getByRole('dialog')
      await add.getByLabel('Path').fill('templates/notes.md')
      await add.getByRole('button', { name: 'Add file' }).click()
      expect('Add file opens the path in the editor', (await filePath.textContent()) === 'templates/notes.md', await filePath.textContent())
      await pane.getByLabel('Content').fill('## Notes\n\n- one change\n')
      await pane.getByRole('button', { name: 'Save' }).click()
      await page.waitForURL(/path=templates%2Fnotes\.md/)
      const tree = page.getByRole('navigation', { name: 'Skill files' })
      await tree.getByRole('link', { name: 'notes.md' }).waitFor()
      expect('the new file sits in the tree under templates', await tree.getByRole('button', { name: 'templates' }).isVisible())
      const added = await skillOf(page, id)
      expect('the Skill holds templates/notes.md', added.file_count === 2 && !!added.files?.some((f) => f.path === 'templates/notes.md'), added.files)

      await pane.getByRole('button', { name: 'Delete file' }).click()
      await page.getByRole('alertdialog').getByRole('button', { name: 'Delete file' }).click()
      await page.waitForURL(/tab=files$/)
      await tree.getByRole('link', { name: 'notes.md' }).waitFor({ state: 'detached' })
      const left = await skillOf(page, id)
      expect('Delete file takes it out of the Skill', left.file_count === 1, left.files)

      const { activity } = (await (await page.request.get(`${WEB}/api/activity?entity=skill&entity_id=${id}`)).json()) as {
        activity: { action: string; details: { path?: string } }[]
      }
      expect(
        'the Activity holds the file update with its path',
        activity.some((a) => a.action === 'skill.file_updated' && a.details.path === 'templates/notes.md'),
        activity.map((a) => a.action),
      )

      await page.getByRole('button', { name: 'Delete', exact: true }).click()
      await page.getByRole('alertdialog').getByRole('button', { name: 'Delete skill' }).click()
      await page.waitForURL(/#\/skills$/)
      const gone = await page.request.get(`${WEB}/api/skills/${id}`)
      expect('Delete deletes the Skill', gone.status() === 404, gone.status())
    } finally {
      await page.request.delete(`${WEB}/api/skills/${id}`)
      await page.context().close()
    }
  },

  async viewer() {
    const owner = await signedIn()
    const slug = 'viewer-e2e-skill'
    await clearSkill(owner, slug)
    const created = await owner.request.post(`${WEB}/api/skills`, { data: { name: 'Viewer e2e skill', slug, description: 'Read only for Members.' } })
    const { skill } = (await created.json()) as { skill: Skill }
    await owner.request.put(`${WEB}/api/skills/${skill.id}/files`, { data: { path: 'templates/notes.md', content: '## Notes\n' } })
    const { page, leave } = await invitedMember(owner)
    try {
      await page.goto(`${WEB}/#/skills`)
      const row = page.getByRole('list', { name: 'Skills' }).getByRole('listitem').filter({ hasText: 'Viewer e2e skill' })
      await row.waitFor()
      expect('a Member sees the Skill in the list', true)
      expect('a Member gets no New skill', (await page.getByRole('button', { name: 'New skill' }).count()) === 0)

      await page.goto(`${WEB}/#/skills/${skill.id}?tab=files&path=templates%2Fnotes.md`)
      const pane = page.getByRole('region', { name: 'File' })
      await pane.getByText('## Notes').or(pane.getByRole('heading', { name: 'Notes' })).first().waitFor()
      expect('a Member reads the Skill file', true)
      const buttons = await Promise.all(['Edit', 'Add file', 'Delete file', 'Delete'].map((n) => page.getByRole('button', { name: n, exact: true }).count()))
      expect('a Member sees no Edit, Add file, Delete file or Delete', buttons.every((c) => c === 0), buttons)
    } finally {
      await leave()
      await owner.request.delete(`${WEB}/api/skills/${skill.id}`)
      await owner.context().close()
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
console.log('the skills flows work')
