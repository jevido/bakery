<script lang="ts">
  // The Desktop app's shell in Paperclip's look, the same frame as the
  // dashboard's (apps/web/src/lib/Layout.svelte): the guild rail, the
  // sidebar, a top bar and `main`. Until a Bakery is connected, main shows
  // "Connect a Bakery".
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import { Plug } from '@lucide/svelte'
  import GuildRail from './lib/GuildRail.svelte'
  import Sidebar from './lib/Sidebar.svelte'
</script>

<Tooltip.Provider delayDuration={0}>
  <div class="flex h-dvh overflow-clip bg-background text-foreground">
    <GuildRail />
    <Sidebar />
    <div class="flex h-full min-w-0 flex-1 flex-col">
      <header class="flex h-12 shrink-0 items-center border-b border-border px-6">
        <h1 class="truncate text-sm font-semibold">Connect a Bakery</h1>
      </header>
      <main class="flex flex-1 items-center justify-center overflow-auto p-6">
        <div class="flex max-w-md flex-col items-center px-6 py-16 text-center" data-testid="connect">
          <div class="mb-4 bg-muted/50 p-4">
            <Plug class="size-10 text-muted-foreground/50" />
          </div>
          <p class="mb-1.5 text-base font-semibold">Connect a Bakery</p>
          <p class="text-sm text-muted-foreground">
            Sign this computer in to a Bakery to see your guilds and run your Agents here. You approve it in the browser; no password is typed into the app.
          </p>
          <div class="mt-4">
            <Button disabled>Connect a Bakery</Button>
          </div>
        </div>
      </main>
    </div>
  </div>
</Tooltip.Provider>
