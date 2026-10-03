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
  // Coolify's status-badge (resources/views/components/status-badge.blade.php):
  // a pill with a coloured dot. `href` renders a link, `onclick` a button.
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

  const dot: Record<StatusType, string> = {
    neutral: 'bg-neutral-400 dark:bg-neutral-500',
    success: 'bg-emerald-500',
    warning: 'bg-warning',
    error: 'bg-red-500',
  }

  const base =
    'chrome inline-flex h-6 max-w-full items-center gap-1.5 rounded-full border border-neutral-200 bg-neutral-100 px-2 text-xs leading-none font-medium whitespace-nowrap text-neutral-700 dark:border-white/[0.12] dark:bg-white/[0.07] dark:text-white'
  const text = $derived([label, status].filter(Boolean).join(' '))
</script>

{#snippet body()}
  {#if children}
    {@render children()}
  {:else}
    <span class={['size-1.5 shrink-0 rounded-full', dot[type]]}></span>
    <span class="truncate">{text}</span>
  {/if}
{/snippet}

{#if href}
  <a {href} {title} class={[base, 'transition-colors', className]}>{@render body()}</a>
{:else if onclick}
  <button type="button" {onclick} {title} class={[base, 'transition-colors', className]}>{@render body()}</button>
{:else}
  <span {title} class={[base, className]}>{@render body()}</span>
{/if}
