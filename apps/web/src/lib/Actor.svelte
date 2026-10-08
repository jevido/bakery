<script lang="ts">
  // Who did something in work: an Agent as its Agent icon and name, linking to
  // its page, else a Member as their Identity, as Paperclip's comment and
  // activity rows show authorAgentId and actorType "agent"
  // (ui/src/components/CommentThread.tsx, ActivityRow.tsx; MIT, see NOTICE).
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import Identity from './Identity.svelte'
  import { href } from './router.svelte'
  import type { WorkAgent, WorkMember } from './work'

  let {
    member,
    agent,
    fallback = 'Someone',
    me,
    size = 'sm',
    class: className = '',
  }: {
    member: WorkMember | null | undefined
    agent?: WorkAgent | null
    /** The name when neither is known. */
    fallback?: string
    /** The signed-in Member, shown as "You". */
    me?: number | null
    size?: 'xs' | 'sm'
    class?: string
  } = $props()
</script>

{#if agent}
  <a
    href={href(`/agents/${agent.id}`)}
    class={['inline-flex min-w-0 items-center gap-1.5 hover:underline', size === 'xs' && 'gap-1', className]}
    title={agent.name}
    data-actor-agent={agent.id}
  >
    <span class={['inline-flex shrink-0 items-center justify-center rounded-full bg-muted', size === 'xs' ? 'size-5' : 'size-6']}>
      <AgentIcon icon={agent.icon} class={size === 'xs' ? 'size-3' : 'size-3.5'} />
    </span>
    <span class={['truncate', size === 'sm' ? 'text-xs' : 'text-sm']}>{agent.name}</span>
  </a>
{:else}
  <Identity name={member ? (me != null && member.id === me ? 'You' : member.name) : fallback} {size} class={className} />
{/if}
