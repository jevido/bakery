<script lang="ts">
  // Coolify's Database configuration sidebar
  // (resources/views/components/database/configuration-sidebar.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, drawn by
  // ResourceNav as a column on wide screens and a select on narrower ones. Only the
  // sub-pages The Bakery has something behind are listed.
  import type { IconName } from '../../lib/Icon.svelte'
  import ResourceNav, { type ResourceNavItem } from '../../lib/ResourceNav.svelte'
  import { databasePath, go, type DatabasePage } from '../../lib/router.svelte'
  import { scrollToSettingsSection, scrollToSettingsSectionLater } from '../../lib/settingsSection.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Database } from '../../lib/types'

  let { database, page }: { database: Database; page: DatabasePage } = $props()

  type Item = { label: string; page: DatabasePage; icon: IconName; visible?: boolean }

  const all = $derived<Item[]>([
    { label: 'General', page: '', icon: 'settings' },
    { label: 'Persistent Storage', page: 'persistent-storage', icon: 'storages' },
    { label: 'Backups', page: 'backups', icon: 'database', visible: database.backups_supported },
    { label: 'Servers', page: 'servers', icon: 'servers' },
    { label: 'Runtime Logs', page: 'logs', icon: 'unordered-list' },
    { label: 'Resource Limits', page: 'resource-limits', icon: 'cpu' },
    { label: 'Danger Zone', page: 'danger', icon: 'shield-alert', visible: projectAccess.can('manage_applications') },
  ])
  const items = $derived(all.filter((i) => i.visible ?? true))

  const groups: [string, string[]][] = [
    ['Settings', ['General', 'Persistent Storage']],
    ['Observe & troubleshoot', ['Runtime Logs']],
    ['Deploy', ['Servers']],
    ['Automation', ['Backups']],
    ['Operations', ['Resource Limits', 'Danger Zone']],
  ]

  // Coolify's $pageSections: the sections of General, listed under it.
  const sections: Partial<Record<DatabasePage, { id: string; label: string }[]>> = {
    '': [
      { id: 'database-details-section', label: 'Database details' },
      { id: 'credentials-section', label: 'Credentials' },
      { id: 'runtime-network-section', label: 'Runtime and network' },
      { id: 'public-access-section', label: 'Public access' },
    ],
  }

  let activeSection = $state('')
  $effect(() => {
    void page
    activeSection = ''
  })

  function openSection(item: ResourceNavItem, id: string) {
    activeSection = id
    if (item.active) {
      scrollToSettingsSection(id)
      return
    }
    scrollToSettingsSectionLater(id)
    go(item.path)
  }

  const grouped = $derived(
    groups
      .map(([label, labels]) => ({
        label,
        items: labels
          .map((l) => items.find((i) => i.label === l))
          .filter((i): i is Item => !!i)
          .map((i) => ({
            label: i.label,
            path: databasePath(database, i.page),
            icon: i.icon,
            active: i.page === page,
            // Coolify lists General's sections only while General is open.
            sections: i.page === page ? sections[i.page] : undefined,
          })),
      }))
      .filter((g) => g.items.length > 0),
  )
</script>

<ResourceNav groups={grouped} label="Database settings" {activeSection} onsection={openSection} />
