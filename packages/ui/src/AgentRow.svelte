<script lang="ts">
  // One Agent in Paperclip's Agents list (ui/src/pages/Agents.tsx; MIT, see
  // NOTICE), as the dashboard and the Desktop app both draw it: an EntityRow
  // with the Agent icon, name, Job and Title, its Manager and Hirer, and its
  // status badge. `badges` adds the app's own marks before the status.
  import type { Snippet } from 'svelte'
  import AgentIcon from './AgentIcon.svelte'
  import EntityRow from './EntityRow.svelte'
  import StatusBadge from './StatusBadge.svelte'
  import { agentStatusText, agentStatusTones, type AgentStatus, type PauseReason } from './agentStatus'

  let {
    agent,
    href,
    dimmed = false,
    badges,
  }: {
    agent: {
      name: string
      job_label: string
      title: string
      icon: string
      status: AgentStatus
      pause_reason?: PauseReason | null
      reports_to: { name: string } | null
      hirer: { name: string } | null
    }
    href: string
    dimmed?: boolean
    badges?: Snippet
  } = $props()
</script>

<EntityRow
  title={agent.name}
  subtitle={`${agent.job_label}${agent.title ? ` - ${agent.title}` : ''}`}
  {href}
  class={['group py-3', dimmed && 'opacity-50'].filter(Boolean).join(' ')}
  data-testid="agent-row"
>
  {#snippet leading()}
    <span class="flex size-8 items-center justify-center rounded-full bg-accent"><AgentIcon icon={agent.icon} class="size-4" /></span>
  {/snippet}
  {#snippet trailing()}
    <div class="hidden items-center gap-3 text-xs text-muted-foreground lg:flex">
      <span class="w-36 truncate" title="Reports to">{agent.reports_to ? `Reports to ${agent.reports_to.name}` : ''}</span>
      <span class="w-36 truncate" title="Hirer">{agent.hirer ? `Hired by ${agent.hirer.name}` : ''}</span>
    </div>
    {@render badges?.()}
    <span class="flex w-32 justify-end"><StatusBadge type={agentStatusTones[agent.status]} label={agentStatusText(agent)} /></span>
  {/snippet}
</EntityRow>
