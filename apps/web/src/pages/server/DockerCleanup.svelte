<script lang="ts">
  // Coolify's Server Docker Cleanup page (resources/views/livewire/server/docker-cleanup.blade.php,
  // docker-cleanup-executions.blade.php and app/Livewire/Server/DockerCleanup.php;
  // Apache-2.0, see NOTICE), the parts The Bakery has: Run cleanup, the
  // nightly schedule and the one execution it keeps (the last). The cleanup
  // configuration (frequency, threshold, volumes, networks, image retention)
  // is left out; the label stays Coolify's, the words inside say Podman.
  import { api } from '../../lib/api'
  import { ago, size } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import { session } from '../../lib/session.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { Server } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { server, onchange }: { server: Server; onchange: (s: Server) => void } = $props()

  async function cleanUp() {
    try {
      const r = await api<{ cleanup: Server['last_cleanup'] }>('POST', `/servers/${server.id}/cleanup`)
      toast.success(`Freed ${size(r.cleanup.reclaimed_bytes)}.`)
    } catch (err) {
      toast.error('Cleanup failed', err instanceof Error ? err.message : String(err))
      return
    }
    const r = await api<{ server: Server }>('GET', `/servers/${server.id}`)
    onchange(r.server)
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
</script>

<div class="flex w-full flex-col gap-8">
  <SettingsGroup
    id="docker-cleanup-overview-section"
    label="Docker cleanup"
    hint="Remove unused Podman images and keep disk usage under control."
  >
    {#snippet actions()}
      {#if session.can('manage_servers') && (server.kind === 'remote' || session.instanceAdmin)}
        <ConfirmationModal
          title="Confirm Docker Cleanup?"
          buttonTitle="Run cleanup"
          variant="highlighted"
          actions={[
            'Deletes dangling images of The Bakery.',
            'Deletes the images of all but the five newest deployments of each application on this server.',
          ]}
          confirmWithText={false}
          step2ButtonText="Run Podman Cleanup"
          onconfirm={cleanUp}
        />
      {/if}
    {/snippet}

    <div class="flex items-start gap-3 rounded-md border border-border px-4 py-3">
      <Icon name="storages" class="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div>
        <p class="text-sm font-medium text-foreground">Scheduled maintenance</p>
        <p class="mt-1 text-xs text-muted-foreground">Cleanup runs automatically every night at 03:00, The Bakery's local time.</p>
      </div>
    </div>
  </SettingsGroup>

  <SettingsGroup id="docker-cleanup-executions-section" label="Recent executions" hint="Review the last cleanup and what it freed.">
    <div class="overflow-hidden rounded-md border border-border">
      {#if server.last_cleanup.at}
        <div class="flex items-center gap-3 px-4 py-2.5" data-testid="cleanup-execution">
          <StatusBadge status="Success" type="success" />
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-foreground">{when.format(new Date(server.last_cleanup.at))}</p>
            <p class="mt-0.5 text-xs text-muted-foreground">
              Freed {size(server.last_cleanup.reclaimed_bytes)} · finished {ago(server.last_cleanup.at)}
            </p>
          </div>
        </div>
      {:else}
        <div class="p-4" data-testid="cleanup-executions-empty">
          <Empty
            size="sm"
            title="No cleanup executions"
            description="Run a manual cleanup or wait for the next scheduled execution."
            icon="storages"
          />
        </div>
      {/if}
    </div>
  </SettingsGroup>
</div>
