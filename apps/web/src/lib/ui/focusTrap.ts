// Keeps Tab inside an open dialog and focuses its first field, as Alpine's
// x-trap does for Coolify's modals; the page behind stops scrolling meanwhile.
const focusable = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

export function focusTrap(node: HTMLElement) {
  const previous = document.activeElement as HTMLElement | null
  const overflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  const first = node.querySelector<HTMLElement>('input:not([readonly]), textarea, select') ?? node.querySelector<HTMLElement>(focusable)
  first?.focus()

  function keydown(e: KeyboardEvent) {
    if (e.key !== 'Tab') return
    const items = [...node.querySelectorAll<HTMLElement>(focusable)].filter((el) => el.offsetParent !== null)
    if (items.length === 0) return
    const [head, tail] = [items[0], items[items.length - 1]]
    if (e.shiftKey && document.activeElement === head) {
      e.preventDefault()
      tail.focus()
    } else if (!e.shiftKey && document.activeElement === tail) {
      e.preventDefault()
      head.focus()
    }
  }

  node.addEventListener('keydown', keydown)
  return () => {
    node.removeEventListener('keydown', keydown)
    document.body.style.overflow = overflow
    previous?.focus()
  }
}
