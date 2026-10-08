<script lang="ts">
  // Coolify's Server navbar (resources/views/livewire/server/navbar.blade.php,
  // Apache-2.0, see NOTICE) as Paperclip's detail header (ui/src/pages/AgentDetail.tsx;
  // MIT): the Server's name, its switcher right after it and its status
  // summary under it. Coolify keeps its Configuration and Resources tabs in
  // the markup but hidden, so navigation lives in the sidebar; they are not
  // drawn here at all. The switcher sits in the title row, not the top bar
  // as Coolify's does, so the top bar stays Paperclip's plain bar.
  //
  // Left out: the proxy actions (Caddy is one proxy for every Server here).
  import { Check, ChevronsUpDown, Search } from '@lucide/svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { Input } from '@bakery/ui/components/ui/input'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { api } from '../../lib/api'
  import PageHeader from '../../lib/PageHeader.svelte'
  import { href, serverPath, type ServerPage } from '../../lib/router.svelte'
  import { statusDotClasses } from '@bakery/ui/statusColors'
  import type { Server } from '../../lib/types'
  import { isFunctional } from './status'
  import StatusSummary from './StatusSummary.svelte'

  let { server, page }: { server: Server; page: ServerPage } = $props()

  let servers = $state.raw<Server[]>([])
  $effect(() => {
    void server.id
    api<{ servers: Server[] }>('GET', '/servers')
      .then((r) => (servers = r.servers))
      .catch(() => {})
  })

  let open = $state(false)
  let search = $state('')
  $effect(() => {
    if (!open) search = ''
  })
  const options = $derived(servers.filter((s) => s.name.toLowerCase().includes(search.toLowerCase())))
  // The switcher keeps the sub-page open on the other Server where it has one.
  const switchPath = (s: Server) => serverPath(s.id, (page === 'private-key' || page === 'danger') && s.kind === 'local' ? '' : page)
</script>

<PageHeader title={server.name} titleTestid="server-subtitle">
  {#snippet trailing()}
    <Popover.Root bind:open>
      <Popover.Trigger class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'shrink-0 text-muted-foreground' })} aria-label="Switch server">
        <ChevronsUpDown class="size-4" />
      </Popover.Trigger>
      <Popover.Content align="start" class="w-64 p-0">
        <div class="relative border-b border-border p-1.5">
          <Search class="pointer-events-none absolute top-1/2 left-3.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <!-- svelte-ignore a11y_autofocus -->
          <Input bind:value={search} type="search" autofocus placeholder="Filter servers…" aria-label="Filter servers" class="h-8 pl-7 text-sm" />
        </div>
        <div class="max-h-72 overflow-y-auto p-1">
          {#each options as option (option.id)}
            <a
              href={href(switchPath(option))}
              onclick={() => (open = false)}
              class="flex items-center gap-2.5 rounded-sm px-2 py-1.5 text-sm hover:bg-accent hover:text-accent-foreground"
              data-testid="server-switcher-option"
            >
              <span class={['size-1.5 shrink-0 rounded-full', statusDotClasses[isFunctional(option) ? 'success' : 'error']]}></span>
              <span class="min-w-0 flex-1 truncate">{option.name}</span>
              {#if option.id === server.id}<Check class="size-3.5 shrink-0" />{/if}
            </a>
          {:else}
            <p class="px-2 py-1.5 text-sm text-muted-foreground">No servers found.</p>
          {/each}
        </div>
      </Popover.Content>
    </Popover.Root>
  {/snippet}
  {#snippet meta()}
    <StatusSummary {server} />
  {/snippet}
</PageHeader>
