<script lang="ts">
  // The Desktop app's shell in Paperclip's look, the same frame as the
  // dashboard's (apps/web/src/lib/Layout.svelte): the guild rail, the
  // sidebar, a top bar and `main`. Until a Bakery is connected, main shows
  // "Connect a Bakery"; then the active Bakery.
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import { Plug, Server } from '@lucide/svelte'
  import { connected, connectDialog } from './lib/bakeries.svelte'
  import ConnectDialog from './lib/ConnectDialog.svelte'
  import GuildRail from './lib/GuildRail.svelte'
  import Sidebar from './lib/Sidebar.svelte'

  connected.start()
  const active = $derived(connected.active)
</script>

<Tooltip.Provider delayDuration={0}>
  <div class="flex h-dvh overflow-clip bg-background text-foreground">
    <GuildRail />
    <Sidebar />
    <div class="flex h-full min-w-0 flex-1 flex-col">
      <header class="flex h-12 shrink-0 items-center border-b border-border px-6">
        <h1 class="truncate text-sm font-semibold">{active ? new URL(active.address).host : 'Connect a Bakery'}</h1>
      </header>
      <main class="flex flex-1 items-center justify-center overflow-auto p-6">
        {#if !connected.loaded}
          <span></span>
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
