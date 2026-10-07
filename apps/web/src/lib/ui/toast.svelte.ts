// Coolify's toasts (resources/views/components/toast.blade.php): newest
// first, at most four, each leaving after 4 s; hovering one holds it, and
// leaving it gives it 2 s more. Toaster.svelte renders them once, in App.svelte.
export type ToastType = 'default' | 'success' | 'info' | 'warning' | 'danger'

export type Toast = {
  id: number
  type: ToastType
  title: string
  text: string
  persistent: boolean
  /** A link under the text, as Paperclip's toast action ("Open DEF-4"). */
  action?: ToastAction
}

export type ToastAction = { label: string; href: string }

const limit = 4
let next = 1
const timers = new Map<number, ReturnType<typeof setTimeout>>()

export const toasts = $state<Toast[]>([])

function show(type: ToastType, title: string, text = '', persistent = false, action?: ToastAction) {
  const item: Toast = { id: next++, type, title, text, persistent, action }
  toasts.unshift(item)
  if (toasts.length > limit) {
    const index = toasts.findLastIndex((t) => !t.persistent)
    if (index !== -1) dismiss(toasts[index].id)
  }
  schedule(item.id, 4000)
  return item.id
}

export function schedule(id: number, delay = 2000) {
  clearTimeout(timers.get(id))
  if (toasts.find((t) => t.id === id)?.persistent) return
  timers.set(
    id,
    setTimeout(() => dismiss(id), delay),
  )
}

export function hold(id: number) {
  clearTimeout(timers.get(id))
}

export function dismiss(id: number) {
  clearTimeout(timers.get(id))
  timers.delete(id)
  const index = toasts.findIndex((t) => t.id === id)
  if (index !== -1) toasts.splice(index, 1)
}

export const toast = {
  show: (title: string, text?: string) => show('default', title, text),
  success: (title: string, text?: string, action?: ToastAction) => show('success', title, text, false, action),
  info: (title: string, text?: string) => show('info', title, text),
  warning: (title: string, text?: string) => show('warning', title, text),
  error: (title: string, text?: string) => show('danger', title, text),
}
