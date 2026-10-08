<script lang="ts">
  // The Issue page's Activity tab, as Paperclip's IssueDetail draws its
  // activity events (ui/src/pages/IssueDetail.tsx; MIT, see NOTICE): oldest
  // first, each a bordered row with the Actor, what they did in Paperclip's
  // words (issueActivitySentence) and how long ago. Nothing here is
  // editable. Issues from before Activity was recorded have none.
  import { ago } from './format'
  import Actor from './Actor.svelte'
  import { href } from './router.svelte'
  import { issueActivitySentence, listIssueActivity, type ActivityEvent } from './work'

  let {
    issue,
    version = 0,
  }: {
    /** The Issue's id. */
    issue: number
    /** Bumped by the page after a change, so the list loads again. */
    version?: number
  } = $props()

  let events = $state.raw<ActivityEvent[] | null>(null)
  let error = $state('')

  $effect(() => {
    void version
    const id = issue
    listIssueActivity(id)
      .then((list) => {
        if (id === issue) {
          events = list
          error = ''
        }
      })
      .catch((e) => (error = e instanceof Error ? e.message : String(e)))
  })
</script>

<section class="space-y-2" aria-label="Activity">
  {#if error}
    <p class="text-sm text-destructive">{error}</p>
  {:else if events === null}
    <p class="text-sm text-muted-foreground">Loading...</p>
  {:else if events.length === 0}
    <p class="text-sm text-muted-foreground">No activity yet.</p>
  {:else}
    {#each events as event (event.id)}
      <div class="flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs" data-activity={event.action}>
        <Actor member={event.actor} agent={event.actor_agent} fallback="Board" size="xs" class="shrink-0 font-medium" />
        <span class="min-w-0 truncate">
          {#each issueActivitySentence(event) as part, n (n)}
            {#if typeof part === 'string'}{part}{:else if 'issue' in part}<a href={href(`/issues/${part.issue}`)} class="font-mono">{part.issue}</a
              >{:else}<span class="text-muted-foreground">{part.muted}</span>{/if}
          {/each}
        </span>
        <span class="ml-auto shrink-0 text-muted-foreground">{ago(event.created_at)}</span>
      </div>
    {/each}
  {/if}
</section>
