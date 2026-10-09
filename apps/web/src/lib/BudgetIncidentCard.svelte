<script lang="ts">
  // Paperclip's BudgetIncidentCard (ui/src/components/BudgetIncidentCard.tsx;
  // MIT, see NOTICE): a hard Budget incident with its state, the Observed
  // amount against the limit, the pause note, then "Raise budget & resume"
  // with a new amount or "Keep paused". Amounts are in the Budget metric,
  // run time in minutes on screen; the new amount starts at the Observed
  // amount plus 10% or one above the limit, whichever is more, as
  // Paperclip's adds $10. Resolving needs manage_budgets.
  import { AlertOctagon, ArrowUpRight, PauseCircle } from '@lucide/svelte'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Card from '@bakery/ui/components/ui/card'
  import { Input } from '@bakery/ui/components/ui/input'
  import { ApiError } from './api'
  import {
    budgetAmount,
    budgetInputUnit,
    fromBudgetInput,
    resolveBudgetIncident,
    toBudgetInput,
    type BudgetIncident,
  } from './costs'
  import { session } from './session.svelte'

  let { incident, onresolved }: { incident: BudgetIncident; onresolved?: (i: BudgetIncident) => void } = $props()

  const editable = $derived(session.can('manage_budgets'))
  const suggested = (i: BudgetIncident) =>
    String(toBudgetInput(i.metric, Math.max(Math.ceil(i.observed * 1.1), i.amount + 1)))
  let draft = $state('')
  $effect(() => {
    draft = suggested(incident)
  })
  let busy = $state(false)
  let error = $state('')

  const parsed = $derived(fromBudgetInput(incident.metric, draft))
  const tooLow = $derived(parsed !== null && parsed <= incident.observed)
  const stateLabel = $derived(
    incident.status === 'resolved'
      ? 'Resolved'
      : incident.status === 'dismissed'
        ? 'Dismissed'
        : incident.approval_id
          ? 'Pending approval'
          : 'Open',
  )

  async function resolve(action: 'raise_budget_and_resume' | 'keep_paused') {
    busy = true
    error = ''
    try {
      onresolved?.(await resolveBudgetIncident(incident.id, action, action === 'raise_budget_and_resume' ? parsed! : undefined))
    } catch (e) {
      error = e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e)
    } finally {
      busy = false
    }
  }

  const caps = 'text-(length:--text-micro) tracking-(--tracking-caps) uppercase'
</script>

<Card.Root
  class="gap-0 overflow-hidden border-red-500/20 bg-[linear-gradient(180deg,rgba(255,70,70,0.10),rgba(255,255,255,0.02))] py-0"
  data-incident={incident.id}
>
  <Card.Header class="px-5 pt-5 pb-3">
    <div class="flex items-start justify-between gap-3">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <div class="{caps} text-red-700/90 dark:text-red-200/80">{incident.scope.type} hard stop</div>
          <Badge variant={incident.status === 'resolved' ? 'outline' : 'secondary'}>{stateLabel}</Badge>
        </div>
        <Card.Title class="mt-1 text-base text-red-950 dark:text-red-50">{incident.scope.name}</Card.Title>
        <Card.Description class="mt-1 text-red-900/75 dark:text-red-100/70">
          Usage reached {budgetAmount(incident.metric, incident.observed)} against a limit of {budgetAmount(incident.metric, incident.amount)}.
        </Card.Description>
      </div>
      <div class="rounded-full border border-red-400/30 bg-red-500/10 p-2 text-red-600 dark:text-red-200">
        <AlertOctagon class="size-4" />
      </div>
    </div>
  </Card.Header>
  <Card.Content class="space-y-4 px-5 pt-0 pb-5">
    <div class="flex items-start gap-2 rounded-xl border border-red-400/20 bg-red-500/10 px-3 py-2 text-sm text-red-950/90 dark:text-red-50/90">
      <PauseCircle class="mt-0.5 size-4 shrink-0" />
      <div>
        {incident.scope.type === 'project'
          ? 'Runs on this project are paused. New work in this project will not start until you resolve the budget incident.'
          : 'This scope is paused. New runs will not start until you resolve the budget incident.'}
      </div>
    </div>

    {#if editable && incident.status === 'open'}
      <div class="rounded-xl border border-border/60 bg-background/60 p-3">
        <label for="incident-{incident.id}-amount" class="{caps} text-muted-foreground">New budget ({budgetInputUnit(incident.metric)})</label>
        <div class="mt-2 flex flex-col gap-3 sm:flex-row">
          <Input id="incident-{incident.id}-amount" bind:value={draft} inputmode="decimal" placeholder="0" />
          <Button class="gap-2" disabled={busy || parsed === null || tooLow} onclick={() => resolve('raise_budget_and_resume')}>
            <ArrowUpRight class="size-4" />{busy ? 'Applying...' : 'Raise budget & resume'}
          </Button>
        </div>
        {#if tooLow}
          <p class="mt-2 text-xs text-red-700 dark:text-red-200/80">The new budget must exceed the current observed usage.</p>
        {/if}
      </div>

      <div class="flex items-center justify-between gap-3">
        <p class="text-xs text-destructive">{error}</p>
        <Button variant="ghost" class="text-muted-foreground" disabled={busy} onclick={() => resolve('keep_paused')}>Keep paused</Button>
      </div>
    {/if}
  </Card.Content>
</Card.Root>
