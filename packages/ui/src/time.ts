// Times as both apps write them.

/** A moment as a clock reads it, "14:30", with the day before it when it is
 * not today: "Oct 9, 2026 14:30". */
export function clockTime(at: string | Date, now = new Date()): string {
  const d = new Date(at)
  const time = d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
  if (d.toDateString() === now.toDateString()) return time
  return `${d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })} ${time}`
}
