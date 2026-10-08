<script lang="ts">
  // Paperclip's BoardApprovalPayload (ui/src/components/ApprovalPayload.tsx;
  // MIT, see NOTICE): the Title, the Summary as Markdown, the amber
  // "Recommended action" box, "On approval" and the Risks as a dotted list.
  // hideTitle leaves the Title out where a heading already shows it. Left
  // out until agents bring them: the hire and budget payloads, and the
  // proposed comment.
  import type { ApprovalPayload } from './approvals'
  import Markdown from './Markdown.svelte'

  let { payload, hideTitle = false }: { payload: Partial<ApprovalPayload>; hideTitle?: boolean } = $props()

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
