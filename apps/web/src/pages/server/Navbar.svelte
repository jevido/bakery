<script lang="ts">
  // Coolify's Server navbar (resources/views/livewire/server/navbar.blade.php,
  // Apache-2.0, see NOTICE): the Server switcher and its status summary in
  // the top bar, and below lg the Server's name as the page heading. Coolify
  // keeps its Configuration and Resources tabs in the markup but hidden, so
  // navigation lives in the sidebar; they are not drawn here at all.
  //
  // Left out: the proxy actions (Caddy is one proxy for every Server here).
  import { api } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { href, serverPath, type ServerPage } from '../../lib/router.svelte'
  import type { Server } from '../../lib/types'
  import { portalTo } from '../../lib/ui/portal'
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

  let switcherOpen = $state(false)
  let search = $state('')
  let switcher = $state<HTMLSpanElement>()
  const options = $derived(servers.filter((s) => s.name.toLowerCase().includes(search.toLowerCase())))
  // The switcher keeps the sub-page open on the other Server where it has one.
  const switchPath = (s: Server) => serverPath(s.id, (page === 'private-key' || page === 'danger') && s.kind === 'local' ? '' : page)
</script>

<svelte:window
  onclick={(e) => {
    const target = e.target as Node
    if (switcherOpen && switcher && !switcher.contains(target)) switcherOpen = false
  }}
  onkeydown={(e) => {
    if (e.key === 'Escape') switcherOpen = false
  }}
/>

<nav class="chrome w-full max-w-none pb-3 lg:pb-0">
  <div {@attach portalTo('#server-topbar-context')}>
    <div data-testid="server-topbar-context" class="flex min-w-0 items-center gap-1 text-[13px]">
      <span class="shrink-0 px-0.5 text-neutral-300 dark:text-fg-faint">/</span>
      <span class="relative flex min-w-0 shrink items-center gap-2 px-1" bind:this={switcher}>
        <button
          type="button"
          class="flex h-8 max-w-56 min-w-0 items-center gap-1.5 rounded-md px-2 transition-colors hover:bg-neutral-100 xl:max-w-72 dark:hover:bg-white/[0.05]"
          onclick={() => (switcherOpen = !switcherOpen)}
          aria-expanded={switcherOpen}
          aria-label="Switch server"
        >
          <span class="min-w-0 truncate font-semibold text-black dark:text-fg">{server.name}</span>
          <svg class="size-4 shrink-0 text-neutral-400 dark:text-fg-faint" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M8 9l4-4 4 4M8 15l4 4 4-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
        {#if switcherOpen}
          <div class="listbox-panel top-9! left-1! z-[90]! w-64! min-w-0!">
            <div class="border-b border-neutral-200 p-1.5 dark:border-white/[0.08]">
              <div class="relative">
                <Icon
                  name="search"
                  class="pointer-events-none absolute top-1/2 left-2 size-3 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
                />
                <!-- svelte-ignore a11y_autofocus -->
                <input
                  type="search"
                  bind:value={search}
                  autofocus
                  placeholder="Filter servers…"
                  aria-label="Filter servers"
                  class="input h-7! w-full rounded-md! border-neutral-200! bg-white! py-0! pr-2! pl-7! text-[11px]! text-black! placeholder:text-neutral-400! dark:border-white/[0.1]! dark:bg-coolgray-100! dark:text-white! dark:placeholder:text-fg-faint!"
                />
              </div>
            </div>
            {#each options as option (option.id)}
              <a href={href(switchPath(option))} onclick={() => (switcherOpen = false)} class="listbox-option gap-2.5!" data-testid="server-switcher-option">
                <span class={['size-1.5 shrink-0 rounded-full', isFunctional(option) ? 'bg-success' : 'bg-error']}></span>
                <span class="min-w-0 flex-1 truncate">{option.name}</span>
                {#if option.id === server.id}
                  <Icon name="check-circle" class="size-3.5 shrink-0 text-coollabs" />
                {/if}
              </a>
            {/each}
          </div>
        {/if}
      </span>
      <StatusSummary {server} />
    </div>
  </div>

  <!-- The name lives in the desktop top bar from lg up; below that the mobile
       top bar has no Server context, so the heading shows it. -->
  <div class="mb-3 w-full lg:hidden">
    <div class="flex min-w-0 flex-col items-start gap-2">
      <h1 data-testid="server-subtitle" class="min-w-0 truncate text-[24px]! leading-7! font-semibold! tracking-tight! text-black dark:text-fg">
        {server.name}
      </h1>
      <StatusSummary {server} />
    </div>
  </div>
</nav>
