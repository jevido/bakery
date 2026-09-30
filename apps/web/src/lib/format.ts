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
