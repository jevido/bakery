<script lang="ts">
  // Coolify's Server status summary (resources/views/components/server/status-summary.blade.php,
  // Apache-2.0, see NOTICE): a pill saying whether the Server is Ready, which
  // opens the System status in Paperclip's popover. Only the Server row: Caddy
  // is one proxy for every Server here, and there is no Sentinel.
  import { ChevronDown } from '@lucide/svelte'
  import * as Popover from '$lib/components/ui/popover'
  import { statusBadgeClasses, statusDotClasses } from '../../lib/statusColors'
  import type { Server } from '../../lib/types'
  import { isFunctional } from './status'

  let { server }: { server: Server } = $props()

  const ready = $derived(isFunctional(server))
  const tone = $derived(ready ? 'success' : 'error')
  let open = $state(false)
</script>

<Popover.Root bind:open>
  <Popover.Trigger
    data-testid="server-status-summary"
    class={[
      'inline-flex max-w-full shrink-0 cursor-pointer items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap transition-opacity outline-none hover:opacity-80 focus-visible:ring-2 focus-visible:ring-ring/50',
      statusBadgeClasses[tone],
    ]}
  >
    <span class={['size-1.5 shrink-0 rounded-full', statusDotClasses[tone]]}></span>
    <span class="truncate">{ready ? 'Ready' : 'Unavailable'}</span>
    <ChevronDown class={['size-3 opacity-55 transition-transform', open && 'rotate-180']} />
  </Popover.Trigger>
  <Popover.Content align="start" class="w-[min(16rem,calc(100vw-1.5rem))] p-1 sm:w-64" role="menu">
    <div class="px-2 py-1.5 text-xs font-medium text-muted-foreground">System status</div>
    <div class="flex items-center gap-2.5 rounded-sm px-2 py-1.5 text-sm">
      <span class={['size-1.5 shrink-0 rounded-full', statusDotClasses[tone]]}></span>
      <span class="flex-1">Server</span>
      <span class="text-muted-foreground">{ready ? 'Ready' : 'Unavailable'}</span>
    </div>
  </Popover.Content>
</Popover.Root>
