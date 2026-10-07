<script lang="ts" module>
  export type StatusType = 'neutral' | 'success' | 'warning' | 'error'

  function headline(s: string): string {
    return s
      .replace(/[_-]+/g, ' ')
      .replace(/([a-z])([A-Z])/g, '$1 $2')
      .trim()
      .split(/\s+/)
      .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ')
  }

  /**
   * What Coolify's status/index.blade.php shows for a container status such
   * as `running:healthy`: running is success, degraded, restarting and
   * starting are warning, anything else reads "Stopped".
   */
  export function containerStatus(status: string): { status: string; type: StatusType } {
    const s = status.toLowerCase()
    let type: StatusType
    if (s.startsWith('running')) type = 'success'
    else if (s.startsWith('degraded') || s.startsWith('restarting') || s.startsWith('starting')) type = 'warning'
    else return { status: 'Stopped', type: 'neutral' }
    if (status.includes('(')) return { status, type }
    const [state, health] = status.split(':')
    return { status: health ? `${headline(state)} (${health})` : headline(state), type }
  }
</script>

<script lang="ts">
  // Paperclip's StatusBadge (components/StatusBadge.tsx): a pill tinted by
  // its status, here the success, warning and error tokens. `href` renders a
  // link, `onclick` a button.
  import type { Snippet } from 'svelte'

  let {
    label,
    status,
    type = 'neutral',
    href,
    onclick,
    title,
    class: className = '',
    children,
  }: {
    label?: string
    status?: string
    type?: StatusType
    href?: string
    onclick?: () => void
    title?: string
    class?: string
    children?: Snippet
  } = $props()

  const tone: Record<StatusType, string> = {
    neutral: 'bg-muted text-muted-foreground',
    success: 'bg-success/10 text-success',
    warning: 'bg-warning/10 text-warning',
    error: 'bg-destructive/10 text-destructive',
  }

  const base = 'chrome inline-flex max-w-full shrink-0 items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap'
  const text = $derived([label, status].filter(Boolean).join(' '))
</script>

{#snippet body()}
  {#if children}
    {@render children()}
  {:else}
    <span class="truncate">{text}</span>
  {/if}
{/snippet}

{#if href}
  <a {href} {title} class={[base, tone[type], 'transition-opacity hover:opacity-80', className]}>{@render body()}</a>
{:else if onclick}
  <button type="button" {onclick} {title} class={[base, tone[type], 'cursor-pointer transition-opacity hover:opacity-80', className]}>{@render body()}</button>
{:else}
  <span {title} class={[base, tone[type], className]}>{@render body()}</span>
{/if}
