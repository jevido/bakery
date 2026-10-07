<script lang="ts">
  // Coolify's Server configuration sidebar (resources/views/components/server/sidebar.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, drawn by
  // ResourceNav as a column on wide screens and a select on narrower ones.
  // Only the sub-pages The Bakery has something behind are listed. The Local
  // server is Coolify's `is_coolify_host`: it has no Private Key and cannot
  // be deleted.
  import type { IconName } from '../../lib/Icon.svelte'
  import ResourceNav from '../../lib/ResourceNav.svelte'
  import { serverPath, type ServerPage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'
  import { isFunctional } from './status'

  let { server, page }: { server: Server; page: ServerPage } = $props()

  type Item = { label: string; page: ServerPage; icon: IconName; group: string; visible?: boolean }

  const items = $derived(
    (
      [
        { label: 'General', page: '', icon: 'settings', group: 'Settings' },
        { label: 'Private Key', page: 'private-key', icon: 'keys', group: 'Settings', visible: server.kind === 'remote' },
        { label: 'Resources', page: 'resources', icon: 'projects', group: 'Platform' },
        { label: 'Docker Cleanup', page: 'docker-cleanup', icon: 'broom', group: 'Operations', visible: isFunctional(server) },
        { label: 'Metrics', page: 'metrics', icon: 'graph', group: 'Operations', visible: isFunctional(server) },
        { label: 'Danger', page: 'danger', icon: 'shield-alert', group: 'Danger zone', visible: server.kind === 'remote' && session.can('manage_servers') },
      ] satisfies Item[] as Item[]
    ).filter((i) => i.visible ?? true),
  )

  // Coolify's groupBy('group'): groups in the order their first item comes.
  const grouped = $derived(
    [...new Set(items.map((i) => i.group))].map((label) => ({
      label,
      items: items
        .filter((i) => i.group === label)
        .map((i) => ({ label: i.label, path: serverPath(server.id, i.page), icon: i.icon, active: i.page === page })),
    })),
  )
</script>

<ResourceNav groups={grouped} label="Server configuration sections" />
