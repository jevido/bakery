<script lang="ts">
  // Paperclip's IssueBlockedNotice (ui/src/components/IssueBlockedNotice.tsx;
  // MIT, see NOTICE), only its "blocked by unresolved issues" case: the amber
  // notice above the description while any Blocker is not done, naming each.
  // Left out with what the Issue does not have yet: parked and stalled
  // chains, run handoffs and retries, and the agent it notifies.
  import { TriangleAlert } from '@lucide/svelte'
  import { href } from './router.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import type { Blocker } from './work'

  let { blockers }: { blockers: Blocker[] } = $props()

  const unresolved = $derived(blockers.filter((b) => b.status !== 'done'))
</script>

{#if unresolved.length > 0}
  <div
    role="status"
    data-slot="issue-blocked-notice"
    data-testid="issue-blocked-notice"
    class="rounded-md border border-amber-300/70 bg-amber-50/90 px-3 py-2.5 text-sm text-amber-950 shadow-sm dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-100"
  >
    <div class="flex items-start gap-2">
      <TriangleAlert class="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-300" />
      <div class="min-w-0 space-y-1.5">
        <p class="leading-5">
          Work on this issue is blocked by {unresolved.length === 1 ? 'the linked issue' : 'the linked issues'} until
          {unresolved.length === 1 ? 'it is' : 'they are'} done.
        </p>
        <div class="flex flex-wrap gap-1.5">
          {#each unresolved as b (b.id)}
            <a
              href={href(`/issues/${b.identifier}`)}
              data-blocker={b.identifier}
              class="inline-flex max-w-full items-center gap-1 rounded-md no-underline border border-amber-300/70 bg-background/80 px-2 py-1 font-mono text-xs text-amber-950 transition-colors hover:border-amber-500 hover:bg-amber-100 hover:underline dark:border-amber-500/40 dark:bg-background/40 dark:text-amber-100 dark:hover:bg-amber-500/15"
            >
              <StatusIcon status={b.status} size="sm" />
              <span>{b.identifier}</span>
              <span class="max-w-72 truncate font-sans text-[11px] text-amber-800 dark:text-amber-200">{b.title}</span>
            </a>
          {/each}
        </div>
      </div>
    </div>
  </div>
{/if}
