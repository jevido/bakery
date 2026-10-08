<script lang="ts">
  // Coolify's Database heading (resources/views/livewire/project/database/heading.blade.php,
  // Apache-2.0, see NOTICE) as Paperclip's detail header (ui/src/pages/AgentDetail.tsx;
  // MIT): the name, the status and the type and version under it, and an
  // Actions menu on the right with Start, or Restart and Stop. The menu is
  // shown only to those who may change it; Restart and Stop open the page's
  // confirmation modals by their hidden triggers, as Coolify's do.
  import { ChevronDown } from '@lucide/svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import Icon from '../../lib/Icon.svelte'
  import PageHeader from '../../lib/PageHeader.svelte'
  import StatusSummary from '../../lib/ui/StatusSummary.svelte'

  let {
    name,
    status,
    detail,
    error,
    stopped,
    canDeploy,
    busy,
    onstart,
  }: {
    name: string
    /** Coolify's status string, e.g. `running`. */
    status: string
    /** The type and version, as a muted line. */
    detail: string
    /** Why the last start failed, or how the container exited. */
    error?: string
    /** Coolify offers Start for an exited Database; The Bakery for one meant to be stopped. */
    stopped: boolean
    canDeploy: boolean
    busy: boolean
    onstart: () => void
  } = $props()

  const click = (id: string) => document.getElementById(id)?.click()
</script>

<div class="space-y-2">
  <PageHeader title={name}>
    {#snippet meta()}
      <StatusSummary {status} title="Database status" />
      <span>{detail}</span>
    {/snippet}
    {#snippet actions()}
      {#if canDeploy}
        <DropdownMenu.Root>
          <DropdownMenu.Trigger class={buttonVariants({ size: 'sm' })}>
            Actions
            <ChevronDown class="opacity-60" />
          </DropdownMenu.Trigger>
          <DropdownMenu.Content align="end" class="w-60">
            {#if stopped}
              <DropdownMenu.Item disabled={busy} onSelect={onstart}>
                <Icon name="play-circle" />
                Start
              </DropdownMenu.Item>
            {:else}
              <DropdownMenu.Item disabled={busy} onSelect={() => click('database-restart-trigger')}>
                <Icon name="restart" />
                Restart
              </DropdownMenu.Item>
              <DropdownMenu.Item variant="destructive" disabled={busy} onSelect={() => click('database-stop-trigger')}>
                <Icon name="stop" />
                Stop
              </DropdownMenu.Item>
            {/if}
          </DropdownMenu.Content>
        </DropdownMenu.Root>
      {/if}
    {/snippet}
  </PageHeader>
  {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
</div>
