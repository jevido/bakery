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
  import type { Server } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
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

<div class="chrome application-settings-form flex w-full flex-col gap-6">
  <SettingsSection
    id="docker-cleanup-overview-section"
    title="Docker cleanup"
    helper="Remove unused Podman images and keep disk usage under control."
  >
    {#snippet actions()}
      {#if session.isAdmin && (server.kind === 'remote' || session.instanceAdmin)}
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

    <div class="flex items-start gap-3">
      <div
        class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-neutral-100 text-neutral-500 dark:bg-white/[0.06] dark:text-fg-dim"
      >
        <Icon name="storages" class="size-4" />
      </div>
      <div>
        <p class="text-sm font-medium text-neutral-950 dark:text-fg">Scheduled maintenance</p>
        <p class="mt-1 text-xs leading-5 text-neutral-500 dark:text-fg-dim">
          Cleanup runs automatically every night at 03:00, The Bakery's local time.
        </p>
      </div>
    </div>
  </SettingsSection>

  <SettingsSection
    id="docker-cleanup-executions-section"
    title="Recent executions"
    helper="Review the last cleanup and what it freed."
    flush
  >
    {#if server.last_cleanup.at}
      <div class="border-b border-neutral-200 last:border-b-0 dark:border-white/[0.08]" data-testid="cleanup-execution">
        <div class="flex w-full items-center gap-4 px-4 py-3 text-left">
          <StatusBadge status="Success" type="success" />
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-neutral-950 dark:text-fg">{when.format(new Date(server.last_cleanup.at))}</p>
            <p class="mt-0.5 text-xs text-neutral-500 dark:text-fg-dim">
              Freed {size(server.last_cleanup.reclaimed_bytes)} · finished {ago(server.last_cleanup.at)}
            </p>
          </div>
        </div>
      </div>
    {:else}
      <div class="p-6" data-testid="cleanup-executions-empty">
        <Empty
          size="sm"
          title="No cleanup executions"
          description="Run a manual cleanup or wait for the next scheduled execution."
          icon="storages"
        />
      </div>
    {/if}
  </SettingsSection>
</div>
