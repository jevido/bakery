/** A byte count for people: 1.5 GB, 320 MB. */
export function size(n: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n < 10 && i > 0 ? n.toFixed(1) : Math.round(n)} ${units[i]}`
}

/** A share, 0–100, with one decimal below 10. */
export function percent(p: number): string {
  return `${p < 10 ? p.toFixed(1) : Math.round(p)}%`
}

/** How long ago, as Carbon's diffForHumans says it: "5 minutes ago". */
export function ago(at: string | Date, now = Date.now()): string {
  const s = Math.round((now - new Date(at).getTime()) / 1000)
  if (s < 1) return 'just now'
  const units: [number, string][] = [
    [365 * 86400, 'year'],
    [30 * 86400, 'month'],
    [7 * 86400, 'week'],
    [86400, 'day'],
    [3600, 'hour'],
    [60, 'minute'],
    [1, 'second'],
  ]
  for (const [secs, unit] of units) {
    const n = Math.floor(s / secs)
    if (n >= 1) return `${n} ${unit}${n === 1 ? '' : 's'} ago`
  }
  return 'just now'
}

/** Coolify's calculateDuration: "01m 05s", "02h 01m 05s", "1d 02h 01m 05s". */
export function duration(from: string | Date, to: string | Date): string {
  const total = Math.max(0, Math.floor((new Date(to).getTime() - new Date(from).getTime()) / 1000))
  const pad = (n: number) => String(n).padStart(2, '0')
  const d = Math.floor(total / 86400)
  const h = Math.floor((total % 86400) / 3600)
  const m = Math.floor((total % 3600) / 60)
  const sec = total % 60
  if (d > 0) return `${d}d ${pad(h)}h ${pad(m)}m ${pad(sec)}s`
  if (h > 0) return `${pad(h)}h ${pad(m)}m ${pad(sec)}s`
  return `${pad(m)}m ${pad(sec)}s`
}
