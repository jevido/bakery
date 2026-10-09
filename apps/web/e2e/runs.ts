// An Agent's Runs for the e2e files that read them (board.ts, chat.ts).
import type { Page } from 'playwright-core'

const WEB = (process.env.BAKERY_WEB ?? 'http://127.0.0.1:4930').replace(/\/$/, '')

export type Run = { id: number; status: string; wake_reason: string }

/** The Agent's Runs, newest first. */
export async function runsOf(page: Page, agent: { id: number }): Promise<Run[]> {
  const { runs } = (await (await page.request.get(`${WEB}/api/runs?agent=${agent.id}`)).json()) as { runs: Run[] }
  return runs.sort((a, b) => b.id - a.id)
}

/** What the claude stand-in said the Run's prompt was ([prompt]): its assistant text. */
export async function promptOf(page: Page, run: number): Promise<string> {
  const { events } = (await (await page.request.get(`${WEB}/api/runs/${run}/events`)).json()) as { events: { kind: string; payload: { text?: string } }[] }
  return events
    .filter((e) => e.kind === 'assistant')
    .map((e) => e.payload.text ?? '')
    .join('\n')
}
