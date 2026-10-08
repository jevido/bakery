// A Run's Transcript as blocks to draw, from its Run events in seq order,
// as Paperclip's normalizeTranscript does for its RunTranscriptView
// (ui/src/components/transcript/RunTranscriptView.tsx; MIT, see NOTICE),
// trimmed to what the Runner reports from `claude --output-format
// stream-json`: `init`, `assistant`, `thinking`, `tool_call` + `tool_result`,
// `stderr`, `system` and `result` (the payloads are in the agents document).

/** A Run event as the API answers it. */
export type RunEvent = { seq: number; kind: string; payload: unknown; created_at: string }

/** The CLI's result line, as the `result` event carries it. */
export type RunResult = {
  subtype: string
  is_error: boolean
  result: string
  num_turns: number
  duration_ms: number
  total_cost_usd: number
  usage: { input_tokens: number; output_tokens: number; cache_read_input_tokens: number }
}

export type TranscriptBlock =
  | { type: 'message'; key: string; text: string }
  | { type: 'thinking'; key: string; text: string }
  | { type: 'tool'; key: string; name: string; input: unknown; result: string | null; status: 'running' | 'completed' | 'error' }
  | { type: 'log'; key: string; kind: 'stderr' | 'system'; lines: string[] }
  | { type: 'event'; key: string; label: 'init'; text: string }
  | { type: 'result'; key: string; result: RunResult }

const record = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {})
const str = (v: unknown) => (typeof v === 'string' ? v : '')
const num = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) ? v : 0)

/** The blocks of a Transcript: a tool call and its result are one block, and
 * consecutive stderr or system lines one block each. */
export function transcriptBlocks(events: readonly RunEvent[]): TranscriptBlock[] {
  const out: TranscriptBlock[] = []
  const tools = new Map<string, Extract<TranscriptBlock, { type: 'tool' }>>()
  for (const e of events) {
    const p = record(e.payload)
    const key = String(e.seq)
    switch (e.kind) {
      case 'init': {
        const session = str(p.session_id)
        out.push({ type: 'event', key, label: 'init', text: `model ${str(p.model) || 'unknown'}${session ? ` • session ${session}` : ''}` })
        break
      }
      case 'assistant':
        if (str(p.text)) out.push({ type: 'message', key, text: str(p.text) })
        break
      case 'thinking':
        if (str(p.text)) out.push({ type: 'thinking', key, text: str(p.text) })
        break
      case 'tool_call': {
        const block = { type: 'tool' as const, key, name: str(p.name) || 'tool', input: p.input ?? {}, result: null, status: 'running' as const }
        out.push(block)
        if (str(p.id)) tools.set(str(p.id), block)
        break
      }
      case 'tool_result': {
        const tool = tools.get(str(p.tool_use_id))
        const content = str(p.content)
        const status = p.is_error === true ? 'error' : 'completed'
        if (tool) {
          tool.result = content
          tool.status = status
        } else {
          out.push({ type: 'tool', key, name: 'tool', input: {}, result: content, status })
        }
        break
      }
      case 'stderr':
      case 'system': {
        const last = out.at(-1)
        if (last?.type === 'log' && last.kind === e.kind) last.lines.push(str(p.text))
        else out.push({ type: 'log', key, kind: e.kind, lines: [str(p.text)] })
        break
      }
      case 'result': {
        const u = record(p.usage)
        out.push({
          type: 'result',
          key,
          result: {
            subtype: str(p.subtype),
            is_error: p.is_error === true,
            result: str(p.result),
            num_turns: num(p.num_turns),
            duration_ms: num(p.duration_ms),
            total_cost_usd: num(p.total_cost_usd),
            usage: { input_tokens: num(u.input_tokens), output_tokens: num(u.output_tokens), cache_read_input_tokens: num(u.cache_read_input_tokens) },
          },
        })
        break
      }
    }
  }
  return out
}

const squash = (s: string) => s.replace(/\s+/g, ' ').trim()
const cut = (s: string, max: number) => (s.length > max ? `${s.slice(0, max - 1)}…` : s)

/** A tool call's input in one line: its command, path, pattern or query
 * when it has one, as Paperclip's summarizeToolInput picks them. */
export function summarizeToolInput(name: string, input: unknown, max = 120): string {
  if (typeof input === 'string') return cut(squash(input), max)
  const r = record(input)
  for (const k of ['command', 'cmd', 'file_path', 'path', 'query', 'url', 'pattern', 'prompt', 'description', 'name']) {
    if (typeof r[k] === 'string' && r[k]) return cut(squash(r[k] as string), max)
  }
  const keys = Object.keys(r)
  if (keys.length === 0) return `No ${name} input`
  return cut(keys.length === 1 ? `${keys[0]} payload` : `${keys.length} fields: ${keys.slice(0, 3).join(', ')}`, max)
}

/** A tool's input or result as it reads unfolded. */
export const formatToolPayload = (v: unknown) => (typeof v === 'string' ? v : JSON.stringify(v, null, 2))

/** Tokens as the footer shows them: 1234 → "1.2k". */
export function compactCount(n: number): string {
  if (n < 1000) return String(n)
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0)}k`
  return `${(n / 1_000_000).toFixed(1)}M`
}

/** A run time in milliseconds as "4.2 s" or "3 min 5 s". */
export function runDuration(ms: number): string {
  if (ms < 60_000) return `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)} s`
  const s = Math.round(ms / 1000)
  return `${Math.floor(s / 60)} min ${s % 60} s`
}
