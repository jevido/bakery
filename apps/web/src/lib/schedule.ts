// The Schedule editor's arithmetic, ported from Paperclip's ScheduleEditor
// (ui/src/components/ScheduleEditor.tsx; MIT, see NOTICE): a few presets that
// build a five-field cron expression, the way back from an expression to its
// preset, and the line that describes it. Whether an expression is valid is
// the API's to say (a bad one is 422); here a Custom expression only needs its
// five fields.

export type SchedulePreset = 'every_minute' | 'every_hour' | 'every_day' | 'weekdays' | 'weekly' | 'monthly' | 'custom'

export const presets: { value: SchedulePreset; label: string }[] = [
  { value: 'every_minute', label: 'Every minute' },
  { value: 'every_hour', label: 'Every hour' },
  { value: 'every_day', label: 'Every day' },
  { value: 'weekdays', label: 'Weekdays' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
  { value: 'custom', label: 'Custom (cron)' },
]

export const hours = Array.from({ length: 24 }, (_, i) => ({
  value: String(i),
  label: i === 0 ? '12 AM' : i < 12 ? `${i} AM` : i === 12 ? '12 PM' : `${i - 12} PM`,
}))

export const minutes = Array.from({ length: 12 }, (_, i) => ({
  value: String(i * 5),
  label: String(i * 5).padStart(2, '0'),
}))

export const daysOfWeek = [
  { value: '1', label: 'Mon' },
  { value: '2', label: 'Tue' },
  { value: '3', label: 'Wed' },
  { value: '4', label: 'Thu' },
  { value: '5', label: 'Fri' },
  { value: '6', label: 'Sat' },
  { value: '0', label: 'Sun' },
]

export const daysOfMonth = Array.from({ length: 31 }, (_, i) => ({
  value: String(i + 1),
  label: String(i + 1),
}))

export type Schedule = {
  preset: SchedulePreset
  hour: string
  minute: string
  dayOfWeek: string
  dayOfMonth: string
}

const has = (options: { value: string }[], value: string) => options.some((o) => o.value === value)

/** The preset an expression is, with its picks; anything else is custom. */
export function parseCronToPreset(cron: string): Schedule {
  const defaults = { hour: '10', minute: '0', dayOfWeek: '1', dayOfMonth: '1' }
  if (!cron.trim()) return { preset: 'every_day', ...defaults }
  const parts = cron.trim().split(/\s+/)
  if (parts.length !== 5) return { preset: 'custom', ...defaults }

  const [min, hr, dom, month, dow] = parts
  const minuteOK = has(minutes, min)
  const hourOK = has(hours, hr)
  if (min === '*' && hr === '*' && dom === '*' && month === '*' && dow === '*') return { preset: 'every_minute', ...defaults }
  if (hr === '*' && dom === '*' && month === '*' && dow === '*' && minuteOK) return { preset: 'every_hour', ...defaults, minute: min }
  if (dom === '*' && month === '*' && dow === '*' && hourOK && minuteOK) return { preset: 'every_day', ...defaults, hour: hr, minute: min }
  if (dom === '*' && month === '*' && dow === '1-5' && hourOK && minuteOK) return { preset: 'weekdays', ...defaults, hour: hr, minute: min }
  if (dom === '*' && month === '*' && has(daysOfWeek, dow) && hourOK && minuteOK)
    return { preset: 'weekly', ...defaults, hour: hr, minute: min, dayOfWeek: dow }
  if (month === '*' && has(daysOfMonth, dom) && dow === '*' && hourOK && minuteOK)
    return { preset: 'monthly', ...defaults, hour: hr, minute: min, dayOfMonth: dom }
  return { preset: 'custom', ...defaults }
}

/** The expression of a preset and its picks; custom has none of its own. */
export function buildCron({ preset, hour, minute, dayOfWeek, dayOfMonth }: Schedule): string {
  switch (preset) {
    case 'every_minute':
      return '* * * * *'
    case 'every_hour':
      return `${minute} * * * *`
    case 'every_day':
      return `${minute} ${hour} * * *`
    case 'weekdays':
      return `${minute} ${hour} * * 1-5`
    case 'weekly':
      return `${minute} ${hour} * * ${dayOfWeek}`
    case 'monthly':
      return `${minute} ${hour} ${dayOfMonth} * *`
    case 'custom':
      return ''
  }
}

function ordinal(n: number): string {
  const s = ['th', 'st', 'nd', 'rd']
  const v = n % 100
  return s[(v - 20) % 10] || s[v] || s[0]
}

/** "Every Mon at 9:00 AM", or the expression itself when it is custom. */
export function describeSchedule(cron: string): string {
  const { preset, hour, minute, dayOfWeek, dayOfMonth } = parseCronToPreset(cron)
  const hourLabel = hours.find((h) => h.value === hour)?.label ?? hour
  const time = `${hourLabel.replace(/ (AM|PM)$/, '')}:${minute.padStart(2, '0')} ${hourLabel.match(/(AM|PM)$/)?.[0] ?? ''}`.trim()
  switch (preset) {
    case 'every_minute':
      return 'Every minute'
    case 'every_hour':
      return `Every hour at :${minute.padStart(2, '0')}`
    case 'every_day':
      return `Every day at ${time}`
    case 'weekdays':
      return `Weekdays at ${time}`
    case 'weekly':
      return `Every ${daysOfWeek.find((d) => d.value === dayOfWeek)?.label ?? dayOfWeek} at ${time}`
    case 'monthly':
      return `Monthly on the ${dayOfMonth}${ordinal(Number(dayOfMonth))} at ${time}`
    case 'custom':
      return cron.trim() || 'No schedule set'
  }
}

/** Whether a Custom expression has its five fields; the API checks the rest. */
export function cronFieldsMessage(cron: string): string | null {
  const fields = cron.trim().split(/\s+/).filter(Boolean)
  if (fields.length === 0) return 'Enter a 5-field cron expression.'
  if (fields.length !== 5) return `Use exactly 5 fields; this has ${fields.length}.`
  return null
}

/** The browser's time zone, the Schedule's default. */
export const localTimeZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

/** Every IANA time zone the browser knows, UTC first. */
export function timeZones(): string[] {
  const all = typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : []
  return ['UTC', ...all.filter((z) => z !== 'UTC')]
}
