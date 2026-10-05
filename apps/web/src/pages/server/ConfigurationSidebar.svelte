<script lang="ts">
  // Coolify's Server configuration sidebar (resources/views/components/server/sidebar.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, one column on
  // wide screens and a grid of links above the page on narrower ones. Only the
  // sub-pages The Bakery has something behind are listed. The Local server is
  // Coolify's `is_coolify_host`: it has no Private Key and cannot be deleted.
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { href, serverPath, type ServerPage } from '../../lib/router.svelte'
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
        { label: 'Danger', page: 'danger', icon: 'shield-alert', group: 'Danger zone', visible: server.kind === 'remote' && session.isAdmin },
      ] satisfies Item[] as Item[]
    ).filter((i) => i.visible ?? true),
  )

  // Coolify's groupBy('group'): groups in the order their first item comes.
  const grouped = $derived(
    [...new Set(items.map((i) => i.group))].map((label) => ({ label, items: items.filter((i) => i.group === label) })),
  )
</script>

<aside class="chrome application-settings-navigation min-w-0 xl:self-start">
  <nav
    aria-label="Server configuration sections"
    class="grid grid-cols-2 gap-0.5 border-y border-neutral-200 py-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-1 xl:border-y-0 xl:py-0 dark:border-white/[0.06]"
  >
    {#each grouped as group, i (group.label)}
      {#if i > 0}
        <div class="my-2 hidden border-t border-neutral-200 xl:block dark:border-white/[0.06]" aria-hidden="true"></div>
      {/if}
      <div class="nav-section hidden xl:block">{group.label}</div>
      {#each group.items as item (item.label)}
        <a
          class={['menu-item', item.page === page && 'menu-item-active']}
          href={href(serverPath(server.id, item.page))}
          aria-current={item.page === page ? 'page' : undefined}
        >
          <Icon name={item.icon} class="menu-item-icon" />
          <span class="menu-item-label">{item.label}</span>
        </a>
      {/each}
    {/each}
  </nav>
</aside>
