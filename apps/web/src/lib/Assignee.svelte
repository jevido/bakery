<script lang="ts">
  // An Issue's Assignee: a Member as their Identity, an Agent as its Agent
  // icon and name, as Paperclip's assignee cell shows one
  // (ui/src/components/IssueProperties.tsx; MIT, see NOTICE).
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import Identity from './Identity.svelte'
  import type { IssueAssignee } from './work'

  let { assignee, size = 'sm' }: { assignee: IssueAssignee; size?: 'xs' | 'sm' } = $props()
</script>

{#if assignee.kind === 'agent'}
  <span class={['inline-flex min-w-0 items-center gap-1.5', size === 'xs' && 'gap-1']} title={assignee.name} data-assignee-agent={assignee.id}>
    <span class={['inline-flex shrink-0 items-center justify-center rounded-full bg-muted', size === 'xs' ? 'size-5' : 'size-6']}>
      <AgentIcon icon={assignee.icon} class={size === 'xs' ? 'size-3' : 'size-3.5'} />
    </span>
    <span class={['truncate', size === 'sm' ? 'text-xs' : 'text-sm']}>{assignee.name}</span>
  </span>
{:else}
  <Identity name={assignee.name} {size} />
{/if}
