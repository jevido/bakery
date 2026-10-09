<script lang="ts">
  // Paperclip's ActivitySection with its RoutineActivityRow
  // (ui/src/components/RoutineActivityRow.tsx; MIT, see NOTICE): every
  // event about the Routine, newest first, as the Activity page's rows: who
  // did it, what (Paperclip's label for the Action) and a line of what
  // changed. A row with details opens them as JSON.
  import { ChevronRight } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import * as Avatar from '@bakery/ui/components/ui/avatar'
  import { ago } from '../../lib/format'
  import { initials } from '../../lib/Identity.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { routineActionLabel, routineEventSummary } from '../../lib/routines'
  import Empty from '../../lib/ui/Empty.svelte'
  import { listActivity, type ActivityEvent } from '../../lib/work'

  let { id, version }: { id: number; version: number } = $props()

  let events = $state.raw<ActivityEvent[] | null>(null)
  let loadError = $state('')
  let open = $state<number | null>(null)

  $effect(() => {
    void version
    listActivity({ entity: 'routine', entity_id: id, limit: 200 })
      .then((es) => (events = es))
      .catch((e) => (loadError = e.message))
  })
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if events === null}
  <PageSkeleton />
{:else if events.length === 0}
  <div class="py-12"><Empty title="No activity yet." icon="activity" /></div>
{:else}
  <ul class="overflow-hidden rounded-lg border border-border" aria-label="Routine activity">
    {#each events as event (event.id)}
      {@const name = event.actor_agent?.name ?? event.actor?.name ?? 'Board'}
      {@const summary = routineEventSummary(event.action, event.details)}
      {@const payload = Object.keys(event.details).length > 0}
      <li class="border-b border-border last:border-b-0" data-activity={event.action}>
        <button
          type="button"
          disabled={!payload}
          aria-expanded={payload ? open === event.id : undefined}
          onclick={() => (open = open === event.id ? null : event.id)}
          class={['dashboard-list-row flex w-full items-center gap-2 text-left text-sm', payload ? 'transition-colors hover:bg-accent/50' : 'cursor-default']}
        >
          {#if event.actor_agent}
            <span class="inline-flex size-6 shrink-0 items-center justify-center rounded-full bg-muted" aria-hidden="true">
              <AgentIcon icon={event.actor_agent.icon} class="size-3.5" />
            </span>
          {:else}
            <Avatar.Root size="sm" aria-hidden="true"><Avatar.Fallback>{initials(name)}</Avatar.Fallback></Avatar.Root>
          {/if}
          <span class="max-w-1/2 shrink-0 truncate"><span>{name}</span> <span class="font-medium">{routineActionLabel(event.action)}</span></span>
          <span class="min-w-0 flex-1 truncate text-muted-foreground" title={summary}>{summary}</span>
          <span class="w-28 shrink-0 text-right text-xs whitespace-nowrap text-muted-foreground">{ago(event.created_at)}</span>
          {#if payload}<ChevronRight class={['size-3.5 shrink-0 text-muted-foreground transition-transform', open === event.id && 'rotate-90']} />{/if}
        </button>
        {#if open === event.id}
          <pre class="mx-2 mb-2 overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">{JSON.stringify(event.details, null, 2)}</pre>
        {/if}
      </li>
    {/each}
  </ul>
{/if}
