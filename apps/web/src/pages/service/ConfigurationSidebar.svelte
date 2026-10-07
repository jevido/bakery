<script lang="ts">
  // Coolify's Service configuration sidebar (the navigation of
  // resources/views/livewire/project/service/configuration.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, drawn by
  // ResourceNav as a column on wide screens and a select on narrower ones. Only the
  // sub-pages The Bakery has something behind are listed. Under an open
  // Persistent Storage, one sub-item per Component scrolls to its section.
  import type { IconName } from '../../lib/Icon.svelte'
  import ResourceNav from '../../lib/ResourceNav.svelte'
  import { servicePath, type ServicePage } from '../../lib/router.svelte'
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
      .map(([label, labels]) => ({
        label,
        items: labels
          .map((l) => items.find((i) => i.label === l))
          .filter((i): i is Item => !!i)
          .map((i) => ({
            label: i.label,
            path: servicePath(service, i.page),
            icon: i.icon,
            active: i.page === page,
            sections: i.page === 'storages' && page === 'storages' ? storageSections : undefined,
          })),
      }))
      .filter((g) => g.items.length > 0),
  )
</script>

<ResourceNav
  groups={grouped}
  label="Service settings"
  {activeSection}
  onsection={(_, id) => {
    activeSection = id
    scrollToSettingsSection(id)
  }}
/>
