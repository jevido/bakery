<script lang="ts">
  // Paperclip's RunTranscriptView (ui/src/components/transcript/
  // RunTranscriptView.tsx; MIT, see NOTICE) for what `claude` reports: the
  // assistant's text as Markdown, thinking folded, each tool call with its
  // result as one row (red when it errored), stderr and system lines as
  // muted monospace, and the result as a footer with tokens, turns, run time
  // and the CLI's cost as an equivalent. While `live`, it keeps to the
  // bottom unless the person scrolled up. Its density and raw mode, command
  // and diff groups and tool decisions are left out: the Runner reports
  // none of them.
  import { Brain, Check, ChevronDown, ChevronRight, CircleAlert, SquareTerminal, Wrench } from '@lucide/svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import Markdown from './Markdown.svelte'
  import { compactCount, formatToolPayload, runDuration, summarizeToolInput, transcriptBlocks, type RunEvent } from './runTranscript'

  let {
    events,
    live = false,
    emptyMessage = 'No transcript yet.',
    class: className = '',
  }: { events: readonly RunEvent[]; live?: boolean; emptyMessage?: string; class?: string } = $props()

  const blocks = $derived(transcriptBlocks(events))
  /** The keys of the rows the person unfolded (or folded, for an errored tool). */
  const toggled = new SvelteSet<string>()
  const toggle = (key: string) => (toggled.has(key) ? toggled.delete(key) : toggled.add(key))

  let scroller = $state<HTMLDivElement>()
  let stuck = true
  const onscroll = () => {
    if (scroller) stuck = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 24
  }
  // New blocks scroll into view while live and the person has not scrolled up.
  $effect(() => {
    void blocks.length
    if (live && stuck && scroller) scroller.scrollTop = scroller.scrollHeight
  })

  const caps = 'text-[11px] font-semibold tracking-wider uppercase'
</script>

<div bind:this={scroller} {onscroll} class={['chrome overflow-y-auto', className]} data-testid="run-transcript" data-live={live}>
  {#if blocks.length === 0}
    <div class="rounded-2xl border border-dashed border-border/70 bg-background/40 p-4 text-sm text-muted-foreground">
      {live ? 'Waiting for output…' : emptyMessage}
    </div>
  {:else}
    <div class="space-y-3">
      {#each blocks as block (block.key)}
        {#if block.type === 'message'}
          <div data-block="assistant"><Markdown source={block.text} class="text-sm leading-6" /></div>
        {:else if block.type === 'thinking'}
          {@const open = toggled.has(block.key)}
          <div data-block="thinking">
            <button type="button" class="flex items-center gap-1.5 text-xs text-muted-foreground/80 hover:text-foreground" aria-expanded={open} onclick={() => toggle(block.key)}>
              {#if open}<ChevronDown class="size-3.5" />{:else}<ChevronRight class="size-3.5" />{/if}
              <Brain class="size-3.5" />Thinking
            </button>
            {#if open}<Markdown source={block.text} class="mt-1.5 pl-5 text-xs leading-5 text-muted-foreground/70 italic" />{/if}
          </div>
        {:else if block.type === 'tool'}
          {@const errored = block.status === 'error'}
          {@const open = errored !== toggled.has(block.key)}
          <div data-block="tool" data-tool-status={block.status} class={[errored && 'rounded-xl border border-destructive/20 bg-destructive/[0.04] p-3']}>
            <div class="flex items-start gap-2">
              {#if errored}
                <CircleAlert class="mt-0.5 size-3.5 shrink-0 text-destructive" />
              {:else if block.status === 'completed'}
                <Check class="mt-0.5 size-3.5 shrink-0 text-success" />
              {:else}
                <Wrench class="mt-0.5 size-3.5 shrink-0 text-blue-600 dark:text-blue-300" />
              {/if}
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span class={[caps, 'text-muted-foreground']}>{block.name}</span>
                  <span class={['text-[10px] font-semibold tracking-wider uppercase', errored ? 'text-destructive' : block.status === 'completed' ? 'text-success' : 'text-blue-700 dark:text-blue-300']}>
                    {errored ? 'Errored' : block.status === 'completed' ? 'Completed' : 'Running'}
                  </span>
                </div>
                <div class="mt-1 text-sm break-words text-foreground/80">{summarizeToolInput(block.name, block.input)}</div>
              </div>
              <button
                type="button"
                class="mt-0.5 inline-flex size-5 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
                aria-label={open ? 'Collapse tool details' : 'Expand tool details'}
                aria-expanded={open}
                onclick={() => toggle(block.key)}
              >
                {#if open}<ChevronDown class="size-4" />{:else}<ChevronRight class="size-4" />{/if}
              </button>
            </div>
            {#if open}
              <div class="mt-3 grid gap-3 lg:grid-cols-2">
                <div>
                  <div class={['mb-1 text-[10px]', caps, 'text-muted-foreground']}>Input</div>
                  <pre class="overflow-x-auto font-mono text-[11px] break-words whitespace-pre-wrap text-foreground/80">{formatToolPayload(block.input) || '<empty>'}</pre>
                </div>
                <div>
                  <div class={['mb-1 text-[10px]', caps, 'text-muted-foreground']}>Result</div>
                  <pre class={['overflow-x-auto font-mono text-[11px] break-words whitespace-pre-wrap', errored ? 'text-destructive' : 'text-foreground/80']}>{block.result ?? 'Waiting for result…'}</pre>
                </div>
              </div>
            {/if}
          </div>
        {:else if block.type === 'log'}
          <div data-block={block.kind} class="flex items-start gap-2 text-muted-foreground">
            <SquareTerminal class="mt-0.5 size-3.5 shrink-0" />
            <pre class="min-w-0 flex-1 overflow-x-auto font-mono text-[11px] break-words whitespace-pre-wrap">{block.lines.join('\n')}</pre>
          </div>
        {:else if block.type === 'event'}
          <div data-block={block.label} class="flex items-start gap-2 text-sky-700 dark:text-sky-300">
            <span class="mt-[7px] size-1.5 shrink-0 rounded-full bg-current/50"></span>
            <div class="min-w-0 flex-1 text-xs break-words whitespace-pre-wrap">
              <span class="text-[10px] font-semibold tracking-wide text-muted-foreground/70 uppercase">{block.label}</span>
              <span class="ml-2">{block.text}</span>
            </div>
          </div>
        {:else if block.type === 'result'}
          {@const r = block.result}
          <div data-block="result" class={['rounded-xl border px-3 py-2.5 text-xs', r.is_error ? 'border-destructive/30 bg-destructive/[0.05]' : 'border-border/70 bg-muted/30']}>
            <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground">
              <span class={[caps, r.is_error ? 'text-destructive' : 'text-success']}>{r.is_error ? 'Failed' : 'Done'}</span>
              <span data-testid="run-tokens">
                <span class="text-foreground">{compactCount(r.usage.input_tokens + r.usage.cache_read_input_tokens)}</span> in ·
                <span class="text-foreground">{compactCount(r.usage.output_tokens)}</span> out tokens
              </span>
              <span>{r.num_turns} {r.num_turns === 1 ? 'turn' : 'turns'}</span>
              <span>{runDuration(r.duration_ms)}</span>
              <span title="What the claude CLI reports this Run would cost; a subscription is not billed for it">≈ ${r.total_cost_usd.toFixed(4)} equivalent</span>
            </div>
            {#if r.is_error && r.result}
              <pre class="mt-2 font-mono text-[11px] break-words whitespace-pre-wrap text-destructive">{r.result}</pre>
            {/if}
          </div>
        {/if}
      {/each}
      {#if live}
        <div class="inline-flex items-center gap-1 text-[10px] font-medium text-muted-foreground italic">
          <span class="relative flex size-1.5">
            <span class="absolute inline-flex size-full animate-ping rounded-full bg-current opacity-70 motion-reduce:animate-none"></span>
            <span class="relative inline-flex size-1.5 rounded-full bg-current"></span>
          </span>
          Streaming
        </div>
      {/if}
    </div>
  {/if}
</div>
