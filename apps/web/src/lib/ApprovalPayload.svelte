<script lang="ts">
  // Paperclip's BoardApprovalPayload (ui/src/components/ApprovalPayload.tsx;
  // MIT, see NOTICE): the Title, the Summary as Markdown, the amber
  // "Recommended action" box, "On approval" and the Risks as a dotted list.
  // hideTitle leaves the Title out where a heading already shows it. A
  // hire_agent shows Paperclip's HireAgentPayload instead: Name (linking the
  // Agent), Job, Title, Reports to, Capabilities and the Roles it would get
  // as chips, in place of Paperclip's adapter and skills, which Agents here
  // do not have. Left out until budgets bring it: the budget payload.
  import type { ApprovalPayload, HirePayload } from './approvals'
  import { jobLabel } from './approvals'
  import Markdown from './Markdown.svelte'
  import { href } from './router.svelte'

  let {
    type = 'request_board_approval',
    payload,
    hideTitle = false,
  }: { type?: string; payload: Partial<ApprovalPayload & HirePayload>; hideTitle?: boolean } = $props()

  const text = (v: unknown) => (typeof v === 'string' && v.trim() ? v.trim() : null)
  // Risks render in a bullet row of their own, so one leading list marker goes.
  const risks = $derived(
    (Array.isArray(payload.risks) ? payload.risks : [])
      .filter((r): r is string => typeof r === 'string')
      .map((r) => r.trim().replace(/^(?:[-*•]|\d+[.)])\s+/, ''))
      .filter(Boolean),
  )
  const title = $derived(hideTitle ? null : text(payload.title))
  const summary = $derived(text(payload.summary))
  const recommended = $derived(text(payload.recommended_action))
  const onApproval = $derived(text(payload.next_action_on_approval))
</script>

{#snippet label(name: string, tone = 'text-muted-foreground')}
  <p class="text-(length:--text-micro) font-medium tracking-(--tracking-label) uppercase {tone}">{name}</p>
{/snippet}

{#snippet field(name: string, value: string | null | undefined)}
  {#if value}
    <div class="flex items-center gap-2">
      <span class="w-20 shrink-0 text-xs text-muted-foreground sm:w-24">{name}</span>
      <span>{value}</span>
    </div>
  {/if}
{/snippet}

{#if type === 'hire_agent'}
  {@const roles = (Array.isArray(payload.roles) ? payload.roles : []).filter((r) => typeof r === 'string' && r.trim())}
  <div class="mt-3 space-y-1.5 text-sm" data-slot="approval-payload" data-type="hire_agent">
    <div class="flex items-center gap-2">
      <span class="w-20 shrink-0 text-xs text-muted-foreground sm:w-24">Name</span>
      {#if payload.agent_id}
        <a href={href(`/agents/${payload.agent_id}`)} class="font-medium text-inherit hover:underline" data-testid="hire-agent-link">{payload.name || '—'}</a>
      {:else}
        <span class="font-medium">{payload.name || '—'}</span>
      {/if}
    </div>
    {@render field('Job', payload.job ? jobLabel(payload.job) : null)}
    {@render field('Title', text(payload.title))}
    {@render field('Reports to', payload.reports_to?.name)}
    {#if text(payload.capabilities)}
      <div class="flex items-start gap-2">
        <span class="w-20 shrink-0 pt-0.5 text-xs text-muted-foreground sm:w-24">Capabilities</span>
        <span class="text-muted-foreground">{text(payload.capabilities)}</span>
      </div>
    {/if}
    {#if roles.length > 0}
      <div class="flex items-start gap-2">
        <span class="w-20 shrink-0 pt-0.5 text-xs text-muted-foreground sm:w-24">Roles</span>
        <div class="flex flex-wrap gap-1.5">
          {#each roles as role (role)}
            <span class="rounded bg-muted px-1.5 py-0.5 font-mono text-(length:--text-micro) text-muted-foreground">{role}</span>
          {/each}
        </div>
      </div>
    {/if}
  </div>
{:else}
  <div class="mt-4 space-y-3.5 text-sm" data-slot="approval-payload">
    {#if title}
      <div class="space-y-1">
        {@render label('Title')}
        <p class="leading-6 font-medium text-foreground">{title}</p>
      </div>
    {/if}
    {#if summary}
      <div class="space-y-1">
        {@render label('Summary')}
        <Markdown source={summary} class="leading-6 text-foreground/90" />
      </div>
    {/if}
    {#if recommended}
      <div class="rounded-lg border border-amber-500/20 bg-amber-500/10 px-3.5 py-3">
        {@render label('Recommended action', 'text-amber-700 dark:text-amber-300')}
        <Markdown source={recommended} class="mt-1 leading-6 text-foreground" />
      </div>
    {/if}
    {#if onApproval}
      <div class="rounded-lg border border-border/60 bg-background/60 px-3.5 py-3">
        {@render label('On approval')}
        <Markdown source={onApproval} class="mt-1 leading-6 text-foreground" />
      </div>
    {/if}
    {#if risks.length > 0}
      <div class="space-y-1.5">
        {@render label('Risks')}
        <ul class="space-y-1 text-sm text-muted-foreground">
          {#each risks as risk, n (n)}
            <li class="flex items-start gap-2">
              <span class="mt-2 size-1.5 shrink-0 rounded-full bg-muted-foreground/60"></span>
              <Markdown source={risk} class="leading-6" />
            </li>
          {/each}
        </ul>
      </div>
    {/if}
  </div>
{/if}
