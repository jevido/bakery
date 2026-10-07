<script lang="ts" module>
  import type { IconName } from './Icon.svelte'

  /** An in-page section of a sub-page, scrolled to from the nav. */
  export type ResourceNavSection = { id: string; label: string }
  /** `brandIcon` names a brand mark in /svgs/ drawn in place of `icon`; `testid` marks the link for the e2e. */
  export type ResourceNavItem = {
    label: string
    path: string
    icon?: IconName
    brandIcon?: string
    active: boolean
    sections?: ResourceNavSection[]
    testid?: string
  }
  export type ResourceNavGroup = { label: string; items: ResourceNavItem[] }
</script>

<script lang="ts">
  // A Resource's configuration nav: Coolify's grouped sub-pages
  // (components/application/configuration-sidebar.blade.php, Apache-2.0, see
  // NOTICE) drawn with Paperclip's sidebar pieces (SidebarSection,
  // SidebarNavItem; MIT) as a column from `xl`, and below it as the phone
  // select of Paperclip's PageTabBar (ui/src/components/PageTabBar.tsx), one
  // optgroup per group. A sub-page's in-page sections are indented under it.
  import { ChevronDown } from '@lucide/svelte'
  import { icons } from './Icon.svelte'
  import { go, href } from './router.svelte'
  import SidebarNavItem from './SidebarNavItem.svelte'
  import SidebarSection from './SidebarSection.svelte'
  import { cn } from './utils'

  let {
    groups,
    label,
    activeSection = '',
    onsection,
  }: {
    groups: ResourceNavGroup[]
    /** The nav's accessible name, e.g. "Configuration sections". */
    label: string
    activeSection?: string
    onsection?: (item: ResourceNavItem, id: string) => void
  } = $props()

  const current = $derived(groups.flatMap((g) => g.items).find((i) => i.active)?.path ?? '')
</script>

<aside class="min-w-0 xl:self-start">
  <div class="relative inline-flex w-full xl:hidden">
    <select
      value={current}
      onchange={(e) => go(e.currentTarget.value)}
      class="h-9 w-full appearance-none rounded-md border border-border bg-background py-1 pr-9 pl-3 text-base focus:ring-1 focus:ring-ring focus:outline-none"
      aria-label={label}
    >
      {#each groups as group (group.label)}
        <optgroup label={group.label}>
          {#each group.items as item (item.label)}
            <option value={item.path}>{item.label}</option>
          {/each}
        </optgroup>
      {/each}
    </select>
    <ChevronDown aria-hidden="true" class="pointer-events-none absolute top-1/2 right-3 size-3.5 -translate-y-1/2 text-muted-foreground" />
  </div>

  <nav aria-label={label} class="hidden flex-col gap-4 xl:flex">
    {#each groups as group (group.label)}
      <SidebarSection label={group.label} inline>
        {#each group.items as item (item.label)}
          <SidebarNavItem
            href={href(item.path)}
            label={item.label}
            icon={item.icon && icons[item.icon]}
            brandIcon={item.brandIcon}
            active={item.active}
            testid={item.testid}
            inline
          />
          {#if item.sections?.length}
            <div class="ml-6 flex flex-col gap-0.5 border-l border-border py-1 pl-2">
              {#each item.sections as section (section.id)}
                <button
                  type="button"
                  title={section.label}
                  class={cn(
                    'truncate rounded-md px-2 py-1 text-left text-(length:--text-compact) transition-colors',
                    item.active && activeSection === section.id
                      ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
                      : 'text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
                  )}
                  onclick={() => onsection?.(item, section.id)}
                >
                  {section.label}
                </button>
              {/each}
            </div>
          {/if}
        {/each}
      </SidebarSection>
    {/each}
  </nav>
</aside>
