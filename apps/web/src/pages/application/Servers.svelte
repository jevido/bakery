<script lang="ts">
  // Coolify's Servers page (resources/views/livewire/project/shared/destination.blade.php,
  // Apache-2.0, see NOTICE): the Primary server card with Open server and the
  // Application's status. The Bakery fixes the Target server when the
  // Application is created and has no Destinations yet, so the Additional
  // servers and Add another server sections are left out, and the card names
  // the server's address instead of a Docker network.
  import { href, serverPath } from '../../lib/router.svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import Icon from '../../lib/Icon.svelte'
  import type { Server } from '../../lib/types'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import StatusSummary from '../../lib/ui/StatusSummary.svelte'

  let { server, status }: { server: Server | null; status: string | null } = $props()

  const reachability = $derived(
    server?.status === 'reachable'
      ? { label: 'Reachable', type: 'success' as const }
      : server?.status === 'unreachable'
        ? { label: 'Unreachable', type: 'error' as const }
        : { label: 'Not validated', type: 'neutral' as const },
  )
</script>

<SettingsGroup id="primary-server-section" label="Primary server" hint="The server this application is built and runs on.">
  {#if !server}
    <Spinner text="Loading…" />
  {:else}
    <div class="flex flex-col gap-4 rounded-md border border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex min-w-0 items-center gap-3">
        <div class="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <Icon name="servers" class="size-4" />
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h4 class="truncate text-sm font-medium text-foreground">{server.name}</h4>
            <span class="rounded-full border border-border bg-muted/50 px-2 py-0.5 text-xs text-muted-foreground">Primary</span>
          </div>
          <p class="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            {#if server.kind === 'remote'}
              <span>Host</span>
              <code class="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground">{server.host}</code>
            {:else}
              <span>The server The Bakery runs on</span>
            {/if}
          </p>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2 sm:justify-end">
        <a href={href(serverPath(server.id))} class={buttonVariants({ variant: 'outline', size: 'sm' })}>Open server</a>
        <StatusBadge status={reachability.label} type={reachability.type} title="Server" />
        {#if status}<StatusSummary {status} align="right" />{/if}
      </div>
    </div>
    {#if server.kind === 'remote'}
      <p class="text-sm text-muted-foreground">Its domains must point at {server.host}, where this server's proxy serves them.</p>
    {/if}
    <p class="text-sm text-muted-foreground">The server is chosen when the application is created and cannot be changed.</p>
  {/if}
</SettingsGroup>
