<script lang="ts">
  // Coolify's auth alert (resources/views/components/auth/alert.blade.php).
  import type { Snippet } from 'svelte'
  import Icon, { type IconName } from '../Icon.svelte'

  type Type = 'info' | 'success' | 'error' | 'warning'

  let { type = 'info', class: className = '', children }: { type?: Type; class?: string; children?: Snippet } = $props()

  const styles: Record<Type, string> = {
    success: 'border-success/35 bg-success/10 text-success',
    error: 'border-error/35 bg-error/10 text-error',
    warning: 'border-warning/35 bg-warning/10 text-warning',
    info: 'border-neutral-300 bg-neutral-100 text-neutral-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-fg-dim',
  }
  const icons: Record<Type, IconName> = {
    success: 'check-circle',
    error: 'alert-circle',
    warning: 'alert-triangle',
    info: 'info-circle',
  }
</script>

<div role={type === 'error' ? 'alert' : undefined} class={['chrome flex items-start gap-3 rounded-lg border px-3 py-2.5 text-sm', styles[type], className]}>
  <Icon name={icons[type]} class="mt-0.5 size-4 shrink-0" />
  <div class="min-w-0 flex-1 leading-5">{@render children?.()}</div>
</div>
