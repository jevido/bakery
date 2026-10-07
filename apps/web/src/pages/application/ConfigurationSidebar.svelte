<script lang="ts">
  // Coolify's Application configuration sidebar
  // (resources/views/components/application/configuration-sidebar.blade.php,
  // Apache-2.0, see NOTICE): the sub-pages in Coolify's groups, drawn by
  // ResourceNav as a column on wide screens and a select on narrower ones. Only the
  // sub-pages The Bakery has something behind are listed, and under a page
  // the in-page sections it has been ported with.
  import type { IconName } from '../../lib/Icon.svelte'
  import ResourceNav, { type ResourceNavItem } from '../../lib/ResourceNav.svelte'
  import { applicationPath, go, type ApplicationPage } from '../../lib/router.svelte'
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
            path: applicationPath(application, i.page),
            icon: i.icon,
            active: i.page === page,
            sections: sections[i.page],
          })),
      }))
      .filter((g) => g.items.length > 0),
  )
</script>

<ResourceNav groups={grouped} label="Configuration sections" {activeSection} onsection={openSection} />
