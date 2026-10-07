// A transition's duration, or 0 when the person asked for reduced motion, so
// modals, menus and toasts appear at once. Headless browsers in the e2e
// scripts ask for it too: they stop producing frames on some pages, and an
// intro that never runs would leave a dialog at opacity 0.
export function motion(duration: number): number {
  if (typeof matchMedia === 'undefined') return duration
  return matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : duration
}
