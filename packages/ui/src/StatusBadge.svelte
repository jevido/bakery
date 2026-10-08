<script lang="ts" module>
  import type { StatusType } from './statusColors'
  export type { StatusType }

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
  // its status from the one palette in statusColors.ts. Without a `type` the
  // tone comes from `status` itself (`failed`, `running:healthy`). `href`
  // renders a link, `onclick` a button.
  import type { Snippet } from 'svelte'
  import { statusBadgeClasses, statusType } from './statusColors'

  let {
    label,
    status,
    type: typeProp,
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

  const tone = $derived(statusBadgeClasses[typeProp ?? statusType(status ?? '')])

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
  <a {href} {title} class={[base, tone, 'transition-opacity hover:opacity-80', className]}>{@render body()}</a>
{:else if onclick}
  <button type="button" {onclick} {title} class={[base, tone, 'cursor-pointer transition-opacity hover:opacity-80', className]}>{@render body()}</button>
{:else}
  <span {title} class={[base, tone, className]}>{@render body()}</span>
{/if}
