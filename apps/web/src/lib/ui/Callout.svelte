<script lang="ts">
  // Coolify's callout (resources/views/components/callout.blade.php) in
  // Paperclip's tokens.
  import type { Snippet } from 'svelte'
  import Icon, { type IconName } from '../Icon.svelte'

  type Type = 'warning' | 'danger' | 'info' | 'success'

  let {
    type = 'warning',
    title = 'Warning',
    class: className = '',
    ondismiss,
    children,
  }: { type?: Type; title?: string; class?: string; ondismiss?: () => void; children?: Snippet } = $props()

  // Paperclip has no alert; these follow its status chips: a tint of the
  // status token, its border and its text.
  const styles: Record<Type, { icon: IconName; shell: string; iconClass: string; titleClass: string; textClass: string }> = {
    warning: {
      icon: 'alert-triangle',
      shell: 'border-warning/30 bg-warning/10',
      iconClass: 'text-warning',
      titleClass: 'text-warning',
      textClass: 'text-foreground/80',
    },
    danger: {
      icon: 'alert-circle',
      shell: 'border-destructive/30 bg-destructive/10',
      iconClass: 'text-destructive',
      titleClass: 'text-destructive',
      textClass: 'text-foreground/80',
    },
    info: {
      icon: 'info-circle',
      shell: 'border-border bg-muted/50',
      iconClass: 'text-muted-foreground',
      titleClass: 'text-foreground',
      textClass: 'text-muted-foreground',
    },
    success: {
      icon: 'check-circle',
      shell: 'border-success/30 bg-success/10',
      iconClass: 'text-success',
      titleClass: 'text-success',
      textClass: 'text-foreground/80',
    },
  }

  const style = $derived(styles[type])
</script>

<div class={['chrome relative rounded-md border px-3 py-2.5', style.shell, className]}>
  <div class="flex items-start gap-2.5">
    <Icon name={style.icon} class={['mt-0.5 size-4 shrink-0', style.iconClass].join(' ')} />
    <div class={['min-w-0 flex-1', ondismiss && 'pr-7']}>
      <div class={['text-sm leading-5 font-medium', style.titleClass]}>{title}</div>
      <div class={['mt-0.5 text-xs leading-5', style.textClass]}>{@render children?.()}</div>
    </div>
    {#if ondismiss}
      <button
        type="button"
        onclick={(e) => {
          e.stopPropagation()
          ondismiss()
        }}
        class="absolute top-1.5 right-1.5 flex size-7 items-center justify-center rounded-md opacity-70 transition-colors hover:bg-accent hover:opacity-100"
        aria-label="Dismiss"
      >
        <Icon name="x" class={['size-3.5', style.iconClass].join(' ')} />
      </button>
    {/if}
  </div>
</div>
