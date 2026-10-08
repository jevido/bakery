<script lang="ts">
  // The "Blocked by" picker of Paperclip's IssueProperties
  // (ui/src/components/issue-properties/IssueProperties.tsx; MIT, see
  // NOTICE): an "Add blocker" chip opening a search over the Guild's other
  // Issues. Picking one toggles it in the set and keeps the list open, as
  // there; "No blockers" clears the set.
  import { Check, Plus } from '@lucide/svelte'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import StatusIcon from './StatusIcon.svelte'
  import { listIssues, type Issue } from './work'

  let {
    self,
    chosen,
    issues,
    onchange,
  }: {
    /** The Issue being blocked, never offered. */
    self: number
    /** The ids of its current Blockers. */
    chosen: number[]
    /** The Guild's Issues, shown before anything is typed. */
    issues: Issue[]
    onchange: (ids: number[]) => void
  } = $props()

  let open = $state(false)
  let query = $state('')
  let found = $state.raw<Issue[] | null>(null)
  let searching = 0

  const options = $derived((query.trim() === '' || found === null ? issues : found).filter((i) => i.id !== self))

  function search(q: string) {
    query = q
    const n = ++searching
    if (q.trim() === '') {
      found = null
      return
    }
    listIssues({ q: q.trim(), limit: 50 })
      .then((is) => {
        if (n === searching) found = is
      })
      .catch(() => {})
  }

  function toggle(id: number) {
    onchange(chosen.includes(id) ? chosen.filter((c) => c !== id) : [...chosen, id])
  }
</script>

<Popover.Root
  bind:open
  onOpenChange={(o) => {
    if (!o) search('')
  }}
>
  <Popover.Trigger
    class="inline-flex items-center gap-1 rounded-full border border-border px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-accent/50 hover:text-foreground"
  >
    <Plus class="size-3" />Add blocker
  </Popover.Trigger>
  <Popover.Content align="end" class="w-72 p-1">
    <input
      data-slot="blocker-search"
      class="mb-1 w-full border-0 border-b border-border bg-transparent px-2 py-1.5 text-xs shadow-none outline-none placeholder:text-muted-foreground/50 focus:border-border focus:ring-0"
      placeholder="Search issues..."
      aria-label="Search issues to add as blockers"
      value={query}
      oninput={(e) => search(e.currentTarget.value)}
    />
    <div class="max-h-48 overflow-y-auto overscroll-contain" role="listbox" aria-label="Blocked by" aria-multiselectable="true">
      <button
        type="button"
        class={['flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs hover:bg-accent/50', chosen.length === 0 && 'bg-accent']}
        onclick={() => {
          onchange([])
          open = false
          search('')
        }}
      >
        No blockers
      </button>
      {#each options as candidate (candidate.id)}
        {@const selected = chosen.includes(candidate.id)}
        <button
          type="button"
          role="option"
          aria-selected={selected}
          data-candidate={candidate.identifier}
          class={['flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-accent/50', selected && 'bg-accent']}
          onclick={() => toggle(candidate.id)}
        >
          <StatusIcon status={candidate.status} size="sm" />
          <span class="truncate">{candidate.identifier} {candidate.title}</span>
          {#if selected}<Check class="ml-auto size-3.5 shrink-0 text-foreground" aria-hidden="true" />{/if}
        </button>
      {/each}
      {#if options.length === 0}
        <div class="px-2 py-2 text-xs text-muted-foreground">No matching issues.</div>
      {/if}
    </div>
  </Popover.Content>
</Popover.Root>
