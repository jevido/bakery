<script lang="ts">
  // The Desktop app's shell in Paperclip's look, the same frame as the
  // dashboard's (apps/web/src/lib/Layout.svelte): the guild rail, the
  // sidebar, a top bar and `main`. Until a Bakery is connected, main shows
  // "Connect a Bakery"; then the route's Bakery (the active one by default),
  // the Guild picked last there (or its first), and the Guild's Agents.
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import { Plug, Server, Users } from '@lucide/svelte'
  import { connected, connectDialog } from './lib/bakeries.svelte'
  import ConnectDialog from './lib/ConnectDialog.svelte'
  import GuildRail from './lib/GuildRail.svelte'
  import { shownGuilds } from './lib/guilds.svelte'
  import { agentsPath, go, router } from './lib/router.svelte'
  import Sidebar from './lib/Sidebar.svelte'
  import Agent from './pages/Agent.svelte'
  import Agents from './pages/Agents.svelte'

  connected.start()
  const route = $derived(router.route)
  const active = $derived(connected.shown)
  const guild = $derived('guild' in route ? shownGuilds.list?.find((g) => g.id === route.guild) : undefined)

  // No Bakery in the route, or one no longer in the list: show the active one.
  $effect(() => {
    if (!connected.loaded) return
    if (route.bakery === null || !connected.list[route.bakery]) {
      const i = connected.list.findIndex((b) => b.active)
      go(i >= 0 ? `/b/${i}` : '/', true)
    }
  })

  // The shown Bakery's Guilds; a signed-out one loads again once reconnected.
  $effect(() => {
    if (!active) return
    if (active.signed_out) shownGuilds.reset()
    else shownGuilds.show(active.address)
  })

  // No Guild in the route, or one the person is no longer in: the one picked
  // last on this Bakery, else the first. A Guild shown is remembered.
  $effect(() => {
    const gs = shownGuilds.list
    if (!active || route.bakery === null || !gs || shownGuilds.address !== active.address) return
    if ('guild' in route && gs.some((g) => g.id === route.guild)) {
      shownGuilds.remember(active.address, route.guild)
      if (route.page === 'guild') go(agentsPath(route.bakery, route.guild), true)
      return
    }
    const last = shownGuilds.remembered(active.address)
    const pick = gs.find((g) => g.id === last) ?? gs[0]
    if (pick) go(agentsPath(route.bakery, pick.id), true)
    else if (route.page !== 'home') go(`/b/${route.bakery}`, true)
  })
</script>

<Tooltip.Provider delayDuration={0}>
  <div class="flex h-dvh overflow-clip bg-background text-foreground">
    <GuildRail />
    <Sidebar />
    <div class="flex h-full min-w-0 flex-1 flex-col">
      <header class="flex h-12 shrink-0 items-center border-b border-border px-6">
        <h1 class="truncate text-sm font-semibold">
          {#if guild && route.page === 'agent'}{guild.name} · Agent{:else if guild}{guild.name} · Agents{:else if active}{new URL(active.address).host}{:else}Connect a Bakery{/if}
        </h1>
      </header>
      <main class={['flex flex-1 overflow-auto p-6', !(guild && !active?.signed_out) && 'items-center justify-center']}>
        {#if !connected.loaded}
          <span></span>
        {:else if active && !active.signed_out && guild && route.page === 'agents'}
          <div class="w-full"><Agents address={active.address} bakery={route.bakery} guild={route.guild} tab={route.tab} /></div>
        {:else if active && !active.signed_out && guild && route.page === 'agent'}
          <div class="w-full"><Agent address={active.address} bakery={route.bakery} guild={route.guild} id={route.id} /></div>
        {:else if active && !active.signed_out && shownGuilds.list?.length === 0}
          <div class="flex max-w-md flex-col items-center px-6 py-16 text-center" data-testid="no-guilds">
            <div class="mb-4 bg-muted/50 p-4">
              <Users class="size-10 text-muted-foreground/50" />
            </div>
            <p class="mb-1.5 text-base font-semibold">No guilds yet</p>
            <p class="text-sm text-muted-foreground">You are in no guild on {new URL(active.address).host}. Create or join one in The Bakery.</p>
          </div>
        {:else if active && !active.signed_out && shownGuilds.error}
          <p class="text-sm text-destructive" role="alert">{shownGuilds.error}</p>
        {:else if active}
          <div class="flex max-w-md flex-col items-center px-6 py-16 text-center" data-testid="connected">
            <div class="mb-4 bg-muted/50 p-4">
              <Server class="size-10 text-muted-foreground/50" />
            </div>
            <p class="mb-1.5 text-base font-semibold">{active.address}</p>
            {#if active.signed_out}
              <p class="text-sm text-muted-foreground">This desktop was signed out of the Bakery. Connect it again to carry on.</p>
              <div class="mt-4">
                <Button onclick={() => (connectDialog.open = true)}>Connect again</Button>
              </div>
            {:else}
              <p class="text-sm text-muted-foreground">Signed in as {active.member.name} ({active.member.email}).</p>
            {/if}
          </div>
        {:else}
          <div class="flex max-w-md flex-col items-center px-6 py-16 text-center" data-testid="connect">
            <div class="mb-4 bg-muted/50 p-4">
              <Plug class="size-10 text-muted-foreground/50" />
            </div>
            <p class="mb-1.5 text-base font-semibold">Connect a Bakery</p>
            <p class="text-sm text-muted-foreground">
              Sign this computer in to a Bakery to see your guilds and run your Agents here. You approve it in the browser; no password is typed into the
              app.
            </p>
            {#if connected.error}
              <p class="mt-2 text-sm text-destructive" role="alert">{connected.error}</p>
            {/if}
            <div class="mt-4">
              <Button onclick={() => (connectDialog.open = true)}>Connect a Bakery</Button>
            </div>
          </div>
        {/if}
      </main>
    </div>
  </div>
  <ConnectDialog />
</Tooltip.Provider>
