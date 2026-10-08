<script lang="ts" module>
  import type { IconName } from '../../lib/Icon.svelte'

  /** One row of the Actions menu. */
  export type Action = { label: string; icon: IconName; run: () => void; disabled?: boolean; danger?: boolean }
</script>

<script lang="ts">
  // Coolify's Application heading (resources/views/livewire/project/application/heading.blade.php,
  // Apache-2.0, see NOTICE) as Paperclip's detail header (ui/src/pages/AgentDetail.tsx;
  // MIT): the name, the status and the Links under it, and an Actions menu on
  // the right. Actions is shown only to those who may deploy; below `sm` it
  // lists `mobileActions`, Coolify's phone order. The Service page
  // (project/service/heading.blade.php) uses it too, with `resource="service"`.
  import { ChevronDown } from '@lucide/svelte'
  import { MediaQuery } from 'svelte/reactivity'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import Icon from '../../lib/Icon.svelte'
  import PageHeader from '../../lib/PageHeader.svelte'
  import StatusSummary from '../../lib/ui/StatusSummary.svelte'
  import Links from './Links.svelte'

  let {
    name,
    urls,
    status,
    actions: menu,
    mobileActions = menu,
    resource = 'application',
    error = '',
  }: {
    name: string
    urls: string[]
    /** Coolify's status string, e.g. `running:healthy`; null until known. */
    status: string | null
    /** Empty for a viewer: no Actions button at all. */
    actions: Action[]
    /** The phone menu, which Coolify orders a little differently. */
    mobileActions?: Action[]
    /** A Service's heading reuses this one with its own words. */
    resource?: 'application' | 'service'
    /** The last failure, as a line under the heading. */
    error?: string
  } = $props()

  const phone = new MediaQuery('max-width: 639px')
  const list = $derived(phone.current ? mobileActions : menu)
</script>

<div class="space-y-2">
  <PageHeader title={name}>
    {#snippet meta()}
      {#if status}
        <StatusSummary
          {status}
          title={resource === 'service' ? 'Service status' : 'Application status'}
          containerName={resource === 'service' ? 'Containers' : 'Container'}
        />
      {/if}
    {/snippet}
    {#snippet actions()}
      <Links {urls} {resource} />
      {#if list.length > 0}
        <DropdownMenu.Root>
          <DropdownMenu.Trigger class={buttonVariants({ size: 'sm' })}>
            Actions
            <ChevronDown class="opacity-60" />
          </DropdownMenu.Trigger>
          <DropdownMenu.Content align="end" class="w-60">
            {#each list as action (action.label)}
              <DropdownMenu.Item variant={action.danger ? 'destructive' : 'default'} disabled={action.disabled} onSelect={action.run}>
                <Icon name={action.icon} />
                {action.label}
              </DropdownMenu.Item>
            {/each}
          </DropdownMenu.Content>
        </DropdownMenu.Root>
      {/if}
    {/snippet}
  </PageHeader>
  {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
</div>
