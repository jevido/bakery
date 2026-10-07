<script lang="ts">
  // Coolify's Application configuration sidebar
  // (resources/views/components/application/configuration-sidebar.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, one column on
  // wide screens and a grid of links above the page on narrower ones. Only the
  // sub-pages The Bakery has something behind are listed, and under a page
  // the in-page sections it has been ported with.
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { applicationPath, go, href, type ApplicationPage } from '../../lib/router.svelte'
  import { scrollToSettingsSection, scrollToSettingsSectionLater } from '../../lib/settingsSection.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application } from '../../lib/types'

  let { application, page }: { application: Application; page: ApplicationPage } = $props()

  type Item = { label: string; page: ApplicationPage; icon: IconName; visible?: boolean }

  const gitBased = $derived(application.build_pack !== 'dockerimage')

  const all = $derived<Item[]>([
      { label: 'General', page: '', icon: 'settings' },
      { label: 'Domains', page: 'domains', icon: 'globe' },
      { label: 'Advanced', page: 'advanced', icon: 'grid' },
      { label: 'Environment Variables', page: 'environment-variables', icon: 'variables' },
      { label: 'Persistent Storage', page: 'persistent-storage', icon: 'storages' },
      { label: 'Deployment Logs', page: 'deployment', icon: 'time-back' },
      { label: 'Runtime Logs', page: 'logs', icon: 'unordered-list' },
      { label: 'Git Source', page: 'source', icon: 'sources', visible: gitBased },
      { label: 'Servers', page: 'servers', icon: 'servers' },
      { label: 'Webhooks', page: 'webhooks', icon: 'notifications' },
      // Every Bakery build pack is git based or dockerimage, as the Blade's condition asks.
      { label: 'Preview Deployments', page: 'preview-deployments', icon: 'eye' },
      { label: 'Healthcheck', page: 'healthcheck', icon: 'feedback' },
      { label: 'Rollback', page: 'rollback', icon: 'time-back' },
      { label: 'Resource Limits', page: 'resource-limits', icon: 'cpu' },
    { label: 'Danger Zone', page: 'danger', icon: 'shield-alert', visible: projectAccess.can('manage_applications') },
  ])
  const items = $derived(all.filter((i) => i.visible ?? true))

  const groups: [string, string[]][] = [
    ['Settings', ['General', 'Domains', 'Environment Variables', 'Persistent Storage', 'Advanced', 'Healthcheck']],
    ['Observe & troubleshoot', ['Runtime Logs', 'Deployment Logs']],
    ['Deploy', ['Git Source', 'Servers', 'Preview Deployments']],
    ['Automation', ['Webhooks']],
    ['Operations', ['Resource Limits', 'Rollback', 'Danger Zone']],
  ]

  // Coolify's $pageSections, for the sections each ported page has.
  const sections = $derived<Partial<Record<ApplicationPage, { id: string; label: string }[]>>>({
    '': [
      { id: 'application-details-section', label: 'Application details' },
      { id: 'access-section', label: 'Access' },
      { id: 'build-pipeline-section', label: 'Build pipeline' },
      ...(gitBased ? [] : [{ id: 'container-image-section', label: 'Container image' }]),
      { id: 'networking-section', label: 'Networking' },
      { id: 'security-section', label: 'Security' },
    ],
    advanced: [
      ...(gitBased ? [{ id: 'advanced-deployment-section', label: 'Deployment' }] : []),
      { id: 'advanced-proxy-section', label: 'Proxy' },
    ],
  })

  let activeSection = $state('')
  $effect(() => {
    void page
    activeSection = ''
  })

  function openSection(item: Item, id: string) {
    activeSection = id
    if (item.page === page) {
      scrollToSettingsSection(id)
      return
    }
    scrollToSettingsSectionLater(id)
    go(applicationPath(application, item.page))
  }

  const grouped = $derived(
    groups
      .map(([label, labels]) => ({ label, items: labels.map((l) => items.find((i) => i.label === l)).filter((i): i is Item => !!i) }))
      .filter((g) => g.items.length > 0),
  )
</script>

<aside class="chrome min-w-0 xl:self-start">
  <nav
    aria-label="Configuration sections"
    class="grid grid-cols-2 gap-0.5 border-y border-neutral-200 py-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-1 xl:border-y-0 xl:py-0 dark:border-white/[0.06]"
  >
    {#each grouped as group, i (group.label)}
      {#if i > 0}
        <div class="my-2 hidden border-t border-neutral-200 xl:block dark:border-white/[0.06]" aria-hidden="true"></div>
      {/if}
      <div class="nav-section hidden xl:block">{group.label}</div>
      {#each group.items as item (item.label)}
        <div>
          <a
            class={['menu-item', item.page === page && 'menu-item-active']}
            href={href(applicationPath(application, item.page))}
            aria-current={item.page === page ? 'page' : undefined}
          >
            <Icon name={item.icon} class="menu-item-icon" />
            <span class="menu-item-label">{item.label}</span>
          </a>
          {#if sections[item.page]?.length}
            <div class="nav-children hidden flex-col gap-0.5 py-1 xl:flex">
              {#each sections[item.page] ?? [] as section (section.id)}
                <button
                  type="button"
                  class={['menu-subitem', item.page === page && activeSection === section.id && 'menu-subitem-active']}
                  onclick={() => openSection(item, section.id)}
                >
                  <span class="menu-item-label text-left">{section.label}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    {/each}
  </nav>
</aside>
