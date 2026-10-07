<script lang="ts">
  // Coolify's Service configuration sidebar (the navigation of
  // resources/views/livewire/project/service/configuration.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, one column on
  // wide screens and a grid of links above the page on narrower ones. Only the
  // sub-pages The Bakery has something behind are listed. Under an open
  // Persistent Storage, one sub-item per Component scrolls to its section.
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { href, servicePath, type ServicePage } from '../../lib/router.svelte'
  import { scrollToSettingsSection } from '../../lib/settingsSection.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Service } from '../../lib/types'
  import { headline, storageSectionID } from './PersistentStorage.svelte'

  let { service, page }: { service: Service; page: ServicePage } = $props()

  type Item = { label: string; page: ServicePage; icon: IconName; visible?: boolean }

  const all = $derived<Item[]>([
    { label: 'General', page: '', icon: 'settings' },
    { label: 'Domains', page: 'domains', icon: 'globe' },
    { label: 'Environment Variables', page: 'environment-variables', icon: 'variables' },
    { label: 'Persistent Storage', page: 'storages', icon: 'storages' },
    { label: 'Runtime Logs', page: 'logs', icon: 'unordered-list' },
    { label: 'Danger Zone', page: 'danger', icon: 'shield-alert', visible: projectAccess.can('manage_applications') },
  ])
  const items = $derived(all.filter((i) => i.visible ?? true))

  const groups: [string, string[]][] = [
    ['Settings', ['General', 'Domains', 'Environment Variables', 'Persistent Storage']],
    ['Observe & troubleshoot', ['Runtime Logs']],
    ['Operations', ['Danger Zone']],
  ]

  // Coolify's $storageSections.
  const storageSections = $derived(service.components.map((c) => ({ id: storageSectionID(c.name), label: headline(c.name) })))
  let activeSection = $state('')
  $effect(() => {
    void page
    activeSection = ''
  })

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
        {#if item.page === 'storages' && page === 'storages' && storageSections.length > 0}
          <div class="nav-children hidden flex-col gap-0.5 py-1 xl:flex">
            {#each storageSections as section (section.id)}
              <button
                type="button"
                class={['menu-subitem', activeSection === section.id && 'menu-subitem-active']}
                onclick={() => {
                  activeSection = section.id
                  scrollToSettingsSection(section.id)
                }}
              >
                <span class="menu-item-label truncate text-left" title={section.label}>{section.label}</span>
              </button>
            {/each}
          </div>
        {/if}
      {/each}
    {/each}
  </nav>
</aside>
