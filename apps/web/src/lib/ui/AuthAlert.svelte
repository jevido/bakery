<script lang="ts">
  // A message on the signed-out pages. Paperclip's Auth.tsx and
  // InviteLanding.tsx show errors as a line of small destructive text and
  // notices as a tinted bordered box; this is that box, an error as role=alert.
  import type { Snippet } from 'svelte'
  import CircleAlert from '@lucide/svelte/icons/circle-alert'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import Info from '@lucide/svelte/icons/info'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'

  type Type = 'info' | 'success' | 'error' | 'warning'

  let { type = 'info', class: className = '', children }: { type?: Type; class?: string; children?: Snippet } = $props()

  const styles: Record<Type, string> = {
    success: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-50',
    error: 'border-destructive/40 bg-destructive/10 text-destructive',
    warning: 'border-amber-500/40 bg-amber-500/10 text-amber-800 dark:text-amber-50',
    info: 'border-border bg-muted/40 text-muted-foreground',
  }
  const icons = { success: CircleCheck, error: CircleAlert, warning: TriangleAlert, info: Info }
  const Icon = $derived(icons[type])
</script>

<div role={type === 'error' ? 'alert' : undefined} class={['flex items-start gap-3 border p-3 text-sm', styles[type], className]}>
  <Icon class="mt-0.5 size-4 shrink-0" />
  <div class="min-w-0 flex-1 leading-5">{@render children?.()}</div>
</div>
