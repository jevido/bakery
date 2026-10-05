<script lang="ts" module>
  // Laravel's Str::headline, as Coolify titles a Component's section:
  // "uptime-kuma" reads "Uptime Kuma".
  export function headline(name: string): string {
    return name
      .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
      .split(/[\s_-]+/)
      .filter(Boolean)
      .map((w) => w[0].toUpperCase() + w.slice(1))
      .join(' ')
  }

  export const storageSectionID = (component: string) => `storage-service-${component}`
</script>

<script lang="ts">
  // Coolify's Persistent Storage for a Service (the storages route of
  // resources/views/livewire/project/service/configuration.blade.php and the
  // Service branch of project/service/storage.blade.php; Apache-2.0, see
  // NOTICE): the read-only note, then one section per Component with its
  // volume mounts, in the volume row look of the Application's page. Left
  // out: the Files and Directories tabs (a Bakery Component mounts only named
  // volumes) and the Backup column.
  import type { Service } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'

  let { service }: { service: Service } = $props()
</script>

<div class="chrome space-y-6">
  <div
    class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-[13px] leading-5 text-amber-800 dark:border-warning/15 dark:bg-warning/[0.07] dark:text-amber-300/90"
  >
    Service volume mounts are read-only here. Edit the Docker Compose file and reload it to change volumes.
  </div>
  {#each service.components as c (c.name)}
    <SettingsSection
      id={storageSectionID(c.name)}
      title={headline(c.name)}
      helper="Volume mounts for this compose service. Compose-managed mounts are read-only in the dashboard."
      flush
    >
      {#if c.volumes.length === 0}
        <div class="p-4">
          <Empty size="sm" title="No storage found" description="No volumes, files, or directories are defined for this service." icon="storages" />
        </div>
      {:else}
        <div class="data-table w-full" data-testid={`component-volumes-${c.name}`}>
          <div class="data-table-header volumes-table-grid-readonly">
            <span>Volume Name</span>
            <span>Destination Path</span>
          </div>
          {#each c.volumes as v (v.path)}
            <div class="env-table-item">
              <div class="data-table-row volumes-table-grid-readonly text-[13px] text-neutral-700 dark:text-fg-dim">
                <div class="volumes-cell-name min-w-0">
                  <span class="volumes-mobile-label">Volume Name</span>
                  <span class="block min-w-0 truncate font-mono text-[13px] font-medium text-neutral-950 dark:text-fg" title={v.name}>{v.name}</span>
                </div>
                <div class="volumes-cell-dest min-w-0">
                  <span class="volumes-mobile-label">Destination Path</span>
                  <span class="flex min-w-0 items-center gap-2">
                    <span class="min-w-0 truncate text-[13px] text-neutral-950 dark:text-fg" title={v.path}>{v.path}</span>
                    {#if v.read_only}<span class="table-badge shrink-0">Read-only</span>{/if}
                  </span>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </SettingsSection>
  {/each}
</div>
