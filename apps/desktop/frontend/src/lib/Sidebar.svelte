<script lang="ts">
  // Paperclip's Sidebar (ui/src/components/Sidebar.tsx; MIT, see NOTICE) as
  // apps/web/src/lib/Sidebar.svelte lays it out: a 60px header, here the
  // Bakery lockup, the nav with the connected Bakeries (address and the
  // person's name, a menu holding Disconnect) and "Connect a Bakery", then
  // the shown Guild's name and its "Agents", and an account strip at the
  // bottom with the Desktop app's version and the theme toggle.
  import BakeryLockup from '@bakery/ui/BakeryLockup.svelte'
  import ThemeToggle from '@bakery/ui/ThemeToggle.svelte'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import { Bot, Ellipsis, ListChecks, LogOut, Plug, Server } from '@lucide/svelte'
  import { connected, connectDialog } from './bakeries.svelte'
  import { disconnect, localRuns, onEvent, version, type LocalRun, type RunsEvent } from './desktop'
  import { shownGuilds } from './guilds.svelte'
  import { agentsPath, go, href, router } from './router.svelte'

  const route = $derived(router.route)
  const guild = $derived('guild' in route ? shownGuilds.list?.find((g) => g.id === route.guild) : undefined)

  let current = $state('')
  $effect(() => {
    version().then((v) => (current = v), () => (current = ''))
  })

  // What the Runner is doing right now, for "Runs on this desktop"'s badge.
  let runsHere = $state.raw<LocalRun[]>([])
  function loadRuns() {
    localRuns().then((r) => (runsHere = r))
  }
  $effect(() => {
    if (connected.list.length === 0) {
      runsHere = []
      return
    }
    loadRuns()
  })
  $effect(() => onEvent<RunsEvent>('runs', loadRuns))

  async function signOut(address: string) {
    try {
      await disconnect(address)
    } finally {
      // The places in the list moved: show the active Bakery.
      go('/')
      await connected.load()
    }
  }
</script>

<aside class="chrome primary-sidebar-surface flex h-full min-h-0 w-60 shrink-0 flex-col border-r border-border" data-testid="sidebar">
  <div class="flex h-(--sz-60px) shrink-0 items-center px-5">
    <BakeryLockup class="h-5 w-auto" />
  </div>
  <nav class="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-3 py-2" aria-label="Main">
    {#if connected.list.length}
      <div class="px-3 pt-1 pb-1.5 text-[10px] font-medium tracking-widest text-muted-foreground/80 uppercase">Bakeries</div>
      {#each connected.list as bakery, i (bakery.address)}
        {@const shown = i === connected.shownIndex}
        <div
          class={[
            'group flex items-center rounded-md text-[13px]',
            shown ? 'bg-accent text-foreground' : 'text-foreground/80 hover:bg-accent/50 hover:text-foreground',
          ]}
          data-testid="bakery"
        >
          <button
            type="button"
            class="flex min-w-0 flex-1 items-center gap-2.5 px-3 py-2 text-left"
            aria-current={shown ? 'page' : undefined}
            onclick={() => connected.switchTo(i)}
          >
            <Server class="size-4 shrink-0" />
            <span class="flex min-w-0 flex-col">
              <span class="truncate font-medium">{new URL(bakery.address).host}</span>
              <span class="truncate text-xs text-muted-foreground">
                {bakery.signed_out ? 'Signed out' : bakery.member.name}
              </span>
            </span>
          </button>
          <DropdownMenu.Root>
            <DropdownMenu.Trigger
              class="mr-1 flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
              aria-label={`${bakery.address} menu`}
            >
              <Ellipsis class="size-4" />
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="start" class="w-56">
              <DropdownMenu.Label class="truncate text-xs font-normal text-muted-foreground">{bakery.address}</DropdownMenu.Label>
              <DropdownMenu.Separator />
              <DropdownMenu.Item variant="destructive" onSelect={() => signOut(bakery.address)}>
                <LogOut />
                Disconnect
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        </div>
      {/each}
      <div class="my-2 border-t border-border"></div>
    {/if}
    <button
      type="button"
      class="flex items-center gap-2.5 rounded-md px-3 py-2 text-[13px] font-medium text-foreground/80 hover:bg-accent/50 hover:text-foreground"
      onclick={() => (connectDialog.open = true)}
    >
      <Plug class="size-4 shrink-0" />
      Connect a Bakery
    </button>
    {#if connected.list.length > 0}
      <a
        href={href('/runs')}
        class={[
          'flex items-center gap-2.5 rounded-md px-3 py-2 text-[13px] font-medium',
          route.page === 'runs' ? 'bg-accent text-foreground' : 'text-foreground/80 hover:bg-accent/50 hover:text-foreground',
        ]}
        aria-current={route.page === 'runs' ? 'page' : undefined}
        data-testid="runs-on-this-desktop"
      >
        <ListChecks class="size-4 shrink-0" />
        Runs on this desktop
        {#if runsHere.length > 0}
          <span class="ml-auto inline-flex min-w-5 items-center justify-center rounded-full bg-accent px-1.5 text-[11px] font-semibold tabular-nums">
            {runsHere.length}
          </span>
        {/if}
      </a>
    {/if}
    {#if guild && 'guild' in route}
      <div class="my-2 border-t border-border"></div>
      <div class="truncate px-3 pt-1 pb-1.5 text-[13px] font-semibold" data-testid="guild-name">{guild.name}</div>
      <a
        href={href(agentsPath(route.bakery, route.guild))}
        class={[
          'flex items-center gap-2.5 rounded-md px-3 py-2 text-[13px] font-medium',
          route.page === 'agents' || route.page === 'agent' ? 'bg-accent text-foreground' : 'text-foreground/80 hover:bg-accent/50 hover:text-foreground',
        ]}
        aria-current={route.page === 'agents' ? 'page' : undefined}
      >
        <Bot class="size-4 shrink-0" />
        Agents
      </a>
    {/if}
  </nav>
  <div class="flex h-12 shrink-0 items-center justify-between border-t border-border px-3">
    <span class="text-xs text-muted-foreground" data-testid="version">{current ? `Desktop app ${current}` : ''}</span>
    <ThemeToggle />
  </div>
</aside>
