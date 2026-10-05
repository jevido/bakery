<script lang="ts">
  // Coolify's Service configuration sidebar (the navigation of
  // resources/views/livewire/project/service/configuration.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, one column on
  // wide screens and a grid of links above the page on narrower ones. Only the
  // sub-pages The Bakery has something behind are listed.
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { href, servicePath, type ServicePage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Service } from '../../lib/types'

  let { service, page }: { service: Service; page: ServicePage } = $props()

  type Item = { label: string; page: ServicePage; icon: IconName; visible?: boolean }

  const all = $derived<Item[]>([
    { label: 'General', page: '', icon: 'settings' },
    { label: 'Domains', page: 'domains', icon: 'globe' },
    { label: 'Environment Variables', page: 'environment-variables', icon: 'variables' },
    { label: 'Persistent Storage', page: 'storages', icon: 'storages' },
    { label: 'Runtime Logs', page: 'logs', icon: 'unordered-list' },
    { label: 'Danger Zone', page: 'danger', icon: 'shield-alert', visible: session.canWrite },
  ])
  const items = $derived(all.filter((i) => i.visible ?? true))

  const groups: [string, string[]][] = [
    ['Settings', ['General', 'Domains', 'Environment Variables', 'Persistent Storage']],
    ['Observe & troubleshoot', ['Runtime Logs']],
    ['Operations', ['Danger Zone']],
  ]

  const grouped = $derived(
    groups
      .map(([label, labels]) => ({ label, items: labels.map((l) => items.find((i) => i.label === l)).filter((i): i is Item => !!i) }))
      .filter((g) => g.items.length > 0),
  )
</script>

<aside class="chrome min-w-0 xl:self-start">
  <nav
    aria-label="Service settings"
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
          href={href(servicePath(service, item.page))}
          aria-current={item.page === page ? 'page' : undefined}
        >
          <Icon name={item.icon} class="menu-item-icon" />
          <span class="menu-item-label">{item.label}</span>
        </a>
      {/each}
    {/each}
  </nav>
</aside>
