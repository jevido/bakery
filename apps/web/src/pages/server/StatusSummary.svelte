<script lang="ts">
  // Coolify's Server status summary (resources/views/components/server/status-summary.blade.php,
  // Apache-2.0, see NOTICE): a pill saying whether the Server is Ready, which
  // opens the System status. Only the Server row: Caddy is one proxy for
  // every Server here, and there is no Sentinel.
  import Icon from '../../lib/Icon.svelte'
  import type { Server } from '../../lib/types'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { isFunctional } from './status'

  let { server }: { server: Server } = $props()

  const ready = $derived(isFunctional(server))
  let open = $state(false)
  let root = $state<HTMLDivElement>()
</script>

<svelte:window
  onclick={(e) => open && root && !root.contains(e.target as Node) && (open = false)}
  onkeydown={(e) => e.key === 'Escape' && (open = false)}
/>

<div class="relative shrink-0" bind:this={root} data-testid="server-status-summary">
  <StatusBadge onclick={() => (open = !open)} class="cursor-pointer">
    <span class={['size-1.5 shrink-0 rounded-full', ready ? 'bg-success' : 'bg-error']}></span>
    <span class="truncate">{ready ? 'Ready' : 'Unavailable'}</span>
    <span class={['inline-flex transition-transform', open && 'rotate-180']}>
      <Icon name="chevron-down" class="size-3 opacity-55" />
    </span>
  </StatusBadge>
  {#if open}
    <div class="listbox-panel top-8! right-0! left-auto! z-[90]! w-64! min-w-64!" role="menu">
      <div class="flex items-center gap-1 px-3 py-2 text-[11px] font-medium text-neutral-400 dark:text-fg-faint">
        <span>System status</span>
      </div>
      <div class="listbox-option cursor-default! gap-2.5!">
        <span class={['size-1.5 shrink-0 rounded-full', ready ? 'bg-success' : 'bg-error']}></span>
        <span class="flex-1">Server</span>
        <span>{ready ? 'Ready' : 'Unavailable'}</span>
      </div>
    </div>
  {/if}
</div>
