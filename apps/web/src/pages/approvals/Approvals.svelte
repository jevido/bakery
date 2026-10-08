<script lang="ts">
  // Paperclip's Approvals page (ui/src/pages/Approvals.tsx; MIT, see NOTICE):
  // the Guild's Approvals as cards, newest first, under the tabs Pending
  // (the Actionable ones, with their count) and All. Approve opens the
  // Approval's page with its "Approval confirmed" banner, as Paperclip does;
  // Reject stays on the list. Only a Member with approve gets the buttons.
  import { ShieldCheck } from '@lucide/svelte'
  import { refreshBadges } from '../../lib/inbox.svelte'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import ApprovalCard from '../../lib/ApprovalCard.svelte'
  import { decide, isActionable, listApprovals, type Approval } from '../../lib/approvals'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { go, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'

  let { tab }: { tab: 'pending' | 'all' } = $props()

  let approvals = $state.raw<Approval[] | null>(null)
  let loadError = $state('')
  let actionError = $state('')
  let pending = $state<{ id: number; decision: 'approve' | 'reject' } | null>(null)

  const load = () =>
    listApprovals()
      .then((as) => {
        approvals = as
        loadError = ''
      })
      .catch((e) => (loadError = e.message))
  load()

  $effect(() => breadcrumb.set({ label: 'Approvals' }))

  const canDecide = $derived(session.can('approve'))
  const shown = $derived(
    (approvals ?? []).filter((a) => tab === 'all' || isActionable(a)).toSorted((a, b) => b.created_at.localeCompare(a.created_at)),
  )
  const pendingCount = $derived((approvals ?? []).filter(isActionable).length)

  async function run(a: Approval, decision: 'approve' | 'reject') {
    pending = { id: a.id, decision }
    try {
      await decide(a.id, decision)
      refreshBadges()
      actionError = ''
      if (decision === 'approve') go(`/approvals/${a.id}?resolved=approved`)
      else await load()
    } catch (e) {
      actionError = e instanceof Error ? e.message : decision === 'approve' ? 'Failed to approve' : 'Failed to reject'
    } finally {
      pending = null
    }
  }
</script>

<div class="chrome space-y-4">
  <div class="flex items-center justify-between">
    <Tabs.Root value={tab} onValueChange={(v) => go(`/approvals/${v}`)}>
      <Tabs.List variant="line" class="justify-start">
        <Tabs.Trigger value="pending">
          Pending
          {#if pendingCount > 0}
            <span class="ml-1.5 rounded-full bg-yellow-500/20 px-1.5 text-(length:--text-nano) font-medium text-yellow-500" data-testid="approvals-pending-count">{pendingCount}</span>
          {/if}
        </Tabs.Trigger>
        <Tabs.Trigger value="all">All</Tabs.Trigger>
      </Tabs.List>
    </Tabs.Root>
  </div>

  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}
  {#if actionError}<p class="text-sm text-destructive">{actionError}</p>{/if}

  {#if approvals === null && !loadError}
    <PageSkeleton />
  {:else if approvals && shown.length === 0}
    <div class="flex flex-col items-center justify-center py-16 text-center">
      <ShieldCheck class="mb-3 size-8 text-muted-foreground/30" />
      <p class="text-sm text-muted-foreground">{tab === 'pending' ? 'No pending approvals.' : 'No approvals yet.'}</p>
    </div>
  {:else if approvals}
    <div class="grid gap-3" aria-label="Approvals" role="list">
      {#each shown as approval (approval.id)}
        <div role="listitem">
          <ApprovalCard
            {approval}
            {canDecide}
            onapprove={() => run(approval, 'approve')}
            onreject={() => run(approval, 'reject')}
            detailLink={href(`/approvals/${approval.id}`)}
            pending={pending?.id === approval.id ? pending.decision : null}
          />
        </div>
      {/each}
    </div>
  {/if}
</div>
