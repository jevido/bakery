<script lang="ts" module>
  import type { ApprovalStatus } from './approvals'

  /** Paperclip's status icon tones, the same on the card and the Approval page. */
  export const approvalStatusTone: Record<ApprovalStatus, string> = {
    approved: 'text-green-600 dark:text-green-400',
    rejected: 'text-red-600 dark:text-red-400',
    revision_requested: 'text-amber-600 dark:text-amber-400',
    pending: 'text-yellow-600 dark:text-yellow-400',
  }
</script>

<script lang="ts">
  // Paperclip's ApprovalCard (ui/src/components/ApprovalCard.tsx; MIT, see
  // NOTICE): the type icon in a circle, the kind badge, who asked (a Member
  // here; only agents ask in Paperclip), the subject as heading, when it was
  // asked, the status pill, the request, the Decision note, and Approve and
  // Reject while the Approval is Actionable and the Member may approve.
  import { CircleCheck, CircleX, Clock } from '@lucide/svelte'
  import { Badge } from '$lib/components/ui/badge'
  import { Button, buttonVariants } from '$lib/components/ui/button'
  import { Card } from '$lib/components/ui/card'
  import { approvalSubject, approvalTypeIcon, approvalTypeLabel, isActionable, type Approval } from './approvals'
  import ApprovalPayload from './ApprovalPayload.svelte'
  import { ago } from './format'
  import Identity from './Identity.svelte'

  let {
    approval,
    canDecide,
    onapprove,
    onreject,
    detailLink,
    pending = null,
  }: {
    approval: Approval
    /** Whether the Member holds the approve Permission. */
    canDecide: boolean
    onapprove?: () => void
    onreject?: () => void
    detailLink?: string
    /** The Decision on its way, so both buttons wait for it. */
    pending?: 'approve' | 'reject' | null
  } = $props()

  const Icon = $derived(approvalTypeIcon(approval.type))
  const kind = $derived(approvalTypeLabel(approval.type))
  const subject = $derived(approvalSubject(approval.payload))
  const decidable = $derived(canDecide && !!onapprove && !!onreject && isActionable(approval))
</script>

<Card class="block gap-0 border-border/70 p-4" data-approval={approval.id} data-status={approval.status}>
  <div class="flex items-start justify-between gap-4">
    <div class="flex min-w-0 flex-1 items-start gap-3">
      <div class="flex size-9 shrink-0 items-center justify-center rounded-full border border-border/70 bg-background/80">
        <Icon class="size-4 text-muted-foreground" />
      </div>
      <div class="min-w-0 flex-1 space-y-2">
        <div class="flex flex-wrap items-center gap-2">
          <Badge variant="outline" class="border-border/70 bg-background/70 px-2 py-0.5 text-(length:--text-micro) font-medium tracking-(--tracking-label) text-muted-foreground uppercase">
            {kind}
          </Badge>
          {#if approval.requester}
            <span class="inline-flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
              <span>Requested by</span>
              <Identity name={approval.requester.name} size="sm" />
            </span>
          {/if}
        </div>
        <div class="space-y-1">
          <h3 class="text-base leading-6 font-semibold text-foreground">{subject ?? kind}</h3>
          <p class="text-xs leading-5 text-muted-foreground">Approval request created {ago(approval.created_at)}</p>
        </div>
      </div>
    </div>
    <div class="shrink-0">
      <span class="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-background/80 px-2.5 py-1 text-xs text-muted-foreground" data-slot="approval-status">
        {#if approval.status === 'approved'}<CircleCheck class="size-3.5 {approvalStatusTone.approved}" />
        {:else if approval.status === 'rejected'}<CircleX class="size-3.5 {approvalStatusTone.rejected}" />
        {:else}<Clock class="size-3.5 {approvalStatusTone[approval.status]}" />{/if}
        <span class="capitalize">{approval.status.replace(/_/g, ' ')}</span>
      </span>
    </div>
  </div>

  <div class="mt-4 border-t border-border/60 pt-4">
    <ApprovalPayload payload={approval.payload} hideTitle={!!subject} />
  </div>

  {#if approval.decision_note}
    <div class="mt-4 rounded-lg border border-border/60 bg-muted/30 px-3.5 py-3 text-xs leading-5 text-muted-foreground">
      <span class="font-medium text-foreground">Decision note.</span>
      {approval.decision_note}
    </div>
  {/if}

  {#if decidable || detailLink}
    <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-border/60 pt-4">
      <div class="flex flex-wrap items-center gap-2">
        {#if decidable}
          <Button size="sm" class="bg-green-700 text-white hover:bg-green-600" disabled={!!pending} onclick={onapprove}>
            {pending === 'approve' ? 'Approving...' : 'Approve'}
          </Button>
          <Button variant="destructive" size="sm" disabled={!!pending} onclick={onreject}>
            {pending === 'reject' ? 'Rejecting...' : 'Reject'}
          </Button>
        {/if}
      </div>
      {#if detailLink}
        <a href={detailLink} class={[buttonVariants({ variant: 'ghost', size: 'sm' }), 'h-auto px-2 text-xs text-muted-foreground no-underline']}>View details</a>
      {/if}
    </div>
  {/if}
</Card>
