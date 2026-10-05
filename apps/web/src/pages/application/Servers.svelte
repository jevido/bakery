<script lang="ts">
  // Coolify's Servers page (resources/views/livewire/project/shared/destination.blade.php,
  // Apache-2.0, see NOTICE): the Primary server card with Open server and the
  // Application's status. The Bakery fixes the Target server when the
  // Application is created and has no Destinations yet, so the Additional
  // servers and Add another server sections are left out, and the card names
  // the server's address instead of a Docker network.
  import { href, serverPath } from '../../lib/router.svelte'
  import Icon from '../../lib/Icon.svelte'
  import type { Server } from '../../lib/types'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
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

<div class="flex flex-col gap-6">
  <SettingsSection id="primary-server-section" title="Primary server" helper="The server this application is built and runs on.">
    {#if !server}
      <Spinner text="Loading…" />
    {:else}
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-center gap-3">
          <div
            class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-neutral-100 text-neutral-500 ring-1 ring-neutral-200 dark:bg-white/[0.05] dark:text-fg-dim dark:ring-white/[0.07]"
          >
            <Icon name="servers" class="size-5" />
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h4 class="truncate text-sm font-semibold text-black dark:text-fg">{server.name}</h4>
              <span
                class="rounded-sm bg-coollabs/10 px-1.5 py-0.5 text-[11px] font-medium text-coollabs dark:bg-warning/10 dark:text-warning"
              >
                Primary
              </span>
            </div>
            <p class="mt-1 flex flex-wrap items-center gap-1.5 text-[13px] text-neutral-500 dark:text-fg-dim">
              {#if server.kind === 'remote'}
                <span>Host</span>
                <code
                  class="rounded bg-neutral-100 px-1.5 py-0.5 font-mono text-xs text-neutral-700 dark:bg-white/[0.05] dark:text-fg-dim"
                  >{server.host}</code
                >
              {:else}
                <span>The server The Bakery runs on</span>
              {/if}
            </p>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2 sm:justify-end">
          <a href={href(serverPath(server.id))} class="button">Open server</a>
          <StatusBadge status={reachability.label} type={reachability.type} title="Server" />
          {#if status}<StatusSummary {status} align="right" />{/if}
        </div>
      </div>
      {#if server.kind === 'remote'}
        <p class="mt-4 text-[13px] leading-5 text-neutral-500 dark:text-fg-dim">
          Its domains must point at {server.host}, where this server's proxy serves them.
        </p>
      {/if}
      <p class="mt-4 text-[13px] leading-5 text-neutral-500 dark:text-fg-dim">
        The server is chosen when the application is created and cannot be changed.
      </p>
    {/if}
  </SettingsSection>
</div>
