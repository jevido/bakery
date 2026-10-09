<script lang="ts">
  // Paperclip's BudgetPolicyCard (ui/src/components/BudgetPolicyCard.tsx;
  // MIT, see NOTICE): the scope and its name, the window, the status chip,
  // Observed against Budget, the Remaining bar and the paused note, then the
  // editor. Amounts are in the Budget metric (tokens, Runs, run time in
  // minutes on screen and seconds on the wire), not dollars. Without a
  // Budget the card is empty and its editor also picks the metric and the
  // window; with one they are the Budget's own, since another metric or
  // window is another Budget. The editor shows only with manage_budgets.
  import { AlertTriangle, PauseCircle, ShieldAlert, Wallet } from '@lucide/svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Card from '@bakery/ui/components/ui/card'
  import { Input } from '@bakery/ui/components/ui/input'
  import * as Select from '@bakery/ui/components/ui/select'
  import { ApiError } from './api'
  import {
    budgetAmount,
    budgetInputUnit,
    budgetMetricLabels,
    budgetUtilisation,
    budgetWindowLabels,
    fromBudgetInput,
    setBudget,
    toBudgetInput,
    type Budget,
    type BudgetMetric,
    type BudgetScope,
    type BudgetWindow,
  } from './costs'
  import { session } from './session.svelte'

  let {
    budget,
    scope,
    onsaved,
  }: {
    /** The Budget, or null for an empty card that sets one. */
    budget: Budget | null
    /** Whose Budget it is; a new one is set for it. */
    scope: BudgetScope
    onsaved?: (b: Budget) => void
  } = $props()

  const editable = $derived(session.can('manage_budgets'))
  const defaultWindow = (s: BudgetScope): BudgetWindow => (s.type === 'project' ? 'lifetime' : 'calendar_month_utc')

  // The drafts follow the Budget whenever the server answers a new one.
  let metric = $state<BudgetMetric>('tokens')
  let windowKind = $state<BudgetWindow>('calendar_month_utc')
  let draft = $state('')
  let warn = $state('80')
  $effect(() => {
    metric = budget?.metric ?? 'runs'
    windowKind = budget?.window ?? defaultWindow(scope)
    draft = budget ? String(toBudgetInput(budget.metric, budget.amount)) : ''
    warn = String(budget?.warn_percent ?? 80)
  })
  let saving = $state(false)
  let error = $state('')

  const parsed = $derived(fromBudgetInput(metric, draft))
  const warnParsed = $derived(Number.isInteger(Number(warn)) && Number(warn) >= 1 && Number(warn) <= 99 ? Number(warn) : null)
  const canSave = $derived(
    parsed !== null && warnParsed !== null && (!budget || parsed !== budget.amount || warnParsed !== budget.warn_percent),
  )

  const status = $derived(budget?.status ?? 'ok')
  const paused = $derived(status === 'hard_stop' && !!budget?.hard_stop)
  const chip = $derived(paused ? 'Paused' : status === 'hard_stop' ? 'Hard stop' : status === 'warning' ? 'Warning' : 'Healthy')
  const StatusIcon = $derived(status === 'hard_stop' ? ShieldAlert : status === 'warning' ? AlertTriangle : Wallet)
  const tone = $derived(
    status === 'hard_stop'
      ? 'text-red-700 dark:text-red-300 border-red-500/30 bg-red-500/10'
      : status === 'warning'
        ? 'text-amber-700 dark:text-amber-200 border-amber-500/30 bg-amber-500/10'
        : 'text-emerald-700 dark:text-emerald-200 border-emerald-500/30 bg-emerald-500/10',
  )
  const capped = $derived(!!budget && budget.amount > 0)
  const progress = $derived(capped ? Math.min(100, budgetUtilisation(budget!)) : 0)

  async function save() {
    if (parsed === null || warnParsed === null) return
    saving = true
    error = ''
    try {
      const b = await setBudget({
        scope_type: scope.type,
        scope_id: scope.type === 'guild' ? undefined : scope.id,
        metric,
        window: windowKind,
        amount: parsed,
        warn_percent: warnParsed,
      })
      onsaved?.(b)
    } catch (e) {
      error = e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e)
    } finally {
      saving = false
    }
  }

  const caps = 'text-(length:--text-micro) tracking-(--tracking-caps) text-muted-foreground uppercase'
</script>

<Card.Root class="gap-0 overflow-hidden border-border/70 bg-card/80 py-0" data-budget-card={budget?.id ?? 'new'} data-scope={scope.type} data-status={budget ? chip : 'none'}>
  <Card.Header class="gap-3 px-5 pt-5 pb-3">
    <div class="flex items-start justify-between gap-3">
      <div>
        <div class={caps}>{scope.type}</div>
        <Card.Title class="mt-1 text-base">{scope.name}</Card.Title>
        <Card.Description class="mt-1">
          {budget ? `${budgetWindowLabels[budget.window]} · ${budgetMetricLabels[budget.metric]}` : 'No budget set'}
        </Card.Description>
      </div>
      {#if budget}
        <div class="inline-flex items-center gap-2 rounded-full border px-3 py-1 {caps} {tone}" data-slot="budget-status">
          <StatusIcon class="size-3.5" />{chip}
        </div>
      {/if}
    </div>
  </Card.Header>
  <Card.Content class="space-y-4 px-5 pt-0 pb-5">
    {#if budget}
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="rounded-xl border border-border/70 bg-black/[0.18] px-4 py-3">
          <div class={caps}>Observed</div>
          <div class="mt-2 text-xl font-semibold tabular-nums" data-slot="observed">{budgetAmount(budget.metric, budget.observed)}</div>
          <div class="mt-1 text-xs text-muted-foreground">{capped ? `${budgetUtilisation(budget)}% of limit` : 'No cap configured'}</div>
        </div>
        <div class="rounded-xl border border-border/70 bg-black/[0.18] px-4 py-3">
          <div class={caps}>Budget</div>
          <div class="mt-2 text-xl font-semibold tabular-nums" data-slot="amount">{capped ? budgetAmount(budget.metric, budget.amount) : 'Disabled'}</div>
          <div class="mt-1 text-xs text-muted-foreground">Warning at {budget.warn_percent}%</div>
        </div>
      </div>

      <div class="space-y-2">
        <div class="flex items-center justify-between text-xs text-muted-foreground">
          <span>Remaining</span>
          <span>{capped ? budgetAmount(budget.metric, Math.max(0, budget.amount - budget.observed)) : 'Unlimited'}</span>
        </div>
        <div class="h-2 overflow-hidden rounded-full bg-muted/70">
          <div
            role="progressbar"
            aria-valuenow={progress}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="Budget utilization: {progress}% used"
            class={[
              'h-full rounded-full transition-[width,background-color] duration-200',
              status === 'hard_stop' ? 'bg-red-500' : status === 'warning' ? 'bg-amber-500' : 'bg-emerald-500',
            ]}
            style:width="{progress}%"
          ></div>
        </div>
      </div>

      {#if paused}
        <div class="flex items-start gap-2 rounded-xl border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-900 dark:text-red-100">
          <PauseCircle class="mt-0.5 size-4 shrink-0" />
          <div>Runs are paused for this scope until the budget is raised or the incident is dismissed.</div>
        </div>
      {/if}
    {:else}
      <p class="text-sm text-muted-foreground">No budget caps this {scope.type} yet.</p>
    {/if}

    {#if editable}
      <div class="flex flex-col gap-3 rounded-xl border border-border/70 bg-background/50 p-3" data-slot="budget-editor">
        {#if !budget}
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <span class={caps}>Metric</span>
              <Select.Root type="single" bind:value={metric}>
                <Select.Trigger class="mt-2 w-full" aria-label="Metric">{budgetMetricLabels[metric]}</Select.Trigger>
                <Select.Content>
                  {#each Object.entries(budgetMetricLabels) as [value, label] (value)}
                    <Select.Item {value} {label} />
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
            <div>
              <span class={caps}>Window</span>
              <Select.Root type="single" bind:value={windowKind}>
                <Select.Trigger class="mt-2 w-full" aria-label="Window">{windowKind === 'lifetime' ? 'Lifetime' : 'Monthly (UTC)'}</Select.Trigger>
                <Select.Content>
                  <Select.Item value="calendar_month_utc" label="Monthly (UTC)" />
                  <Select.Item value="lifetime" label="Lifetime" />
                </Select.Content>
              </Select.Root>
            </div>
          </div>
        {/if}
        <div class="flex flex-col gap-3 sm:flex-row sm:items-end">
          <label class="min-w-0 flex-1">
            <span class={caps}>Budget ({budgetInputUnit(metric)})</span>
            <Input bind:value={draft} class="mt-2" inputmode="decimal" placeholder="0" aria-label="Budget amount" />
          </label>
          <label class="w-full sm:w-28">
            <span class={caps}>Warning %</span>
            <Input bind:value={warn} class="mt-2" inputmode="numeric" aria-label="Warning percent" />
          </label>
          <Button onclick={save} disabled={!canSave || saving}>
            {saving ? 'Saving...' : budget && budget.amount > 0 ? 'Update budget' : 'Set budget'}
          </Button>
        </div>
        {#if parsed === null}
          <p class="text-xs text-destructive">Enter a whole number of {budgetInputUnit(metric)}, 0 or more.</p>
        {:else if warnParsed === null}
          <p class="text-xs text-destructive">The warning is a whole percent from 1 to 99.</p>
        {/if}
        {#if error}<p class="text-xs text-destructive">{error}</p>{/if}
      </div>
    {/if}
  </Card.Content>
</Card.Root>
