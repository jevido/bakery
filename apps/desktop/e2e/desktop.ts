// The Desktop app through its frontend in headless Chromium, against its
// `serve` mode, one section each, every flow once in the dark theme at
// 1440×900 (the light theme and phone width wait for the guilds goal's final
// sweep):
//   shell   the guild rail, the sidebar with the lockup and the Desktop
//           app's version (read through /rpc), and "Connect a Bakery";
//           /rpc answers Version and refuses an unknown method with 404.
//
//   bun e2e/desktop.ts [section ...]   (needs task desktop:serve)
//
// BAKERY_DESKTOP overrides the serve address, CHROMIUM the browser.
import { chromium } from 'playwright-core'

const DESKTOP = (process.env.BAKERY_DESKTOP ?? 'http://127.0.0.1:4991').replace(/\/$/, '')
const CHROMIUM = process.env.CHROMIUM ?? '/usr/bin/chromium'

let failed = 0
function expect(what: string, ok: boolean, got?: unknown) {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}${ok ? '' : ` (got ${JSON.stringify(got)})`}`)
  if (!ok) failed++
}

const browser = await chromium.launch({ executablePath: CHROMIUM })

async function page() {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
  await ctx.addInitScript(() => localStorage.setItem('theme', 'dark'))
  return ctx.newPage()
}

const sections: Record<string, () => Promise<void>> = {
  async shell() {
    const p = await page()
    const errors: string[] = []
    p.on('pageerror', (e) => errors.push(e.message))
    await p.goto(DESKTOP)
    await p.getByTestId('connect').waitFor()
    expect('the title is "The Bakery"', (await p.title()) === 'The Bakery', await p.title())
    expect('the dark theme is on', await p.evaluate(() => document.documentElement.classList.contains('dark')))
    expect('the guild rail is there', await p.getByRole('navigation', { name: 'Guilds' }).isVisible())
    expect('the sidebar shows the lockup', await p.getByTestId('sidebar').getByRole('img', { name: 'The Bakery' }).isVisible())
    const version = p.getByTestId('version')
    await version.filter({ hasText: 'Desktop app' }).waitFor()
    expect('the sidebar shows the version from /rpc', /^Desktop app \S+$/.test(await version.innerText()), await version.innerText())
    const connect = p.getByTestId('connect').getByRole('button', { name: 'Connect a Bakery' })
    expect('"Connect a Bakery" is shown, its button not yet enabled', (await connect.isVisible()) && (await connect.isDisabled()))
    expect('no page errors', errors.length === 0, errors)

    const v = await p.request.post(`${DESKTOP}/rpc/Version`, { data: [] })
    expect('POST /rpc/Version answers {result}', v.ok() && typeof ((await v.json()) as { result: unknown }).result === 'string', await v.text())
    const unknown = await p.request.post(`${DESKTOP}/rpc/Nope`, { data: [] })
    expect('an unknown method answers 404', unknown.status() === 404, unknown.status())
    await p.context().close()
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
console.log('the desktop app works')
