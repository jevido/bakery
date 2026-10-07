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
  import Callout from '../../lib/ui/Callout.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'

  let { service }: { service: Service } = $props()
</script>

<div class="chrome flex flex-col gap-6">
  <Callout title="Read-only mounts">
    Service volume mounts are read-only here. Edit the Docker Compose file and reload it to change volumes.
  </Callout>
  {#each service.components as c (c.name)}
    <SettingsGroup
      id={storageSectionID(c.name)}
      label={headline(c.name)}
      hint="Volume mounts for this compose service. Compose-managed mounts are read-only in the dashboard."
      wide
    >
      <div class="overflow-hidden rounded-md border border-border">
        {#if c.volumes.length === 0}
          <div class="p-4">
            <Empty size="sm" title="No storage found" description="No volumes, files, or directories are defined for this service." icon="storages" />
          </div>
        {:else}
          <div
            class="hidden grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] gap-3 border-b border-border bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground md:grid"
            data-testid={`component-volumes-${c.name}`}
          >
            <span>Volume Name</span>
            <span>Destination Path</span>
          </div>
          {#each c.volumes as v (v.path)}
            <div class="grid gap-x-3 gap-y-1 border-b border-border px-4 py-2.5 text-sm last:border-b-0 md:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
              <div class="min-w-0">
                <span class="block text-xs text-muted-foreground md:hidden">Volume Name</span>
                <span class="block min-w-0 truncate font-mono text-xs font-medium text-foreground" title={v.name}>{v.name}</span>
              </div>
              <div class="min-w-0">
                <span class="block text-xs text-muted-foreground md:hidden">Destination Path</span>
                <span class="flex min-w-0 items-center gap-2">
                  <span class="min-w-0 truncate font-mono text-xs text-foreground" title={v.path}>{v.path}</span>
                  {#if v.read_only}
                    <span class="shrink-0 rounded-full border border-border bg-muted/50 px-2 py-0.5 text-xs text-muted-foreground">Read-only</span>
                  {/if}
                </span>
              </div>
            </div>
          {/each}
        {/if}
      </div>
    </SettingsGroup>
  {/each}
</div>
