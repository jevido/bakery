<script lang="ts">
  // One Run's Transcript: a final Run's events read once, a queued or
  // running one followed live until it ends, as Paperclip's
  // useLiveRunTranscripts does (MIT, see NOTICE). The stream replays the
  // stored events first, so a running Run is only followed, never read too.
  import RunTranscript from '@bakery/ui/RunTranscript.svelte'
  import { runIsFinal } from '@bakery/ui/runStatus'
  import { followRun, runEvents, type Run, type RunEvent } from './runs'

  let { run, onstatus, class: className = '' }: { run: Pick<Run, 'id' | 'status'>; onstatus?: (r: Run) => void; class?: string } = $props()

  let events = $state.raw<RunEvent[]>([])
  let live = $state(false)
  let error = $state('')

  // Followed again only when the Run changes or ends.
  const id = $derived(run.id)
  const final = $derived(runIsFinal(run.status))
  $effect(() => {
    error = ''
    if (final) {
      // Kept on screen until read again, so a Run that just ended does not blink.
      live = false
      runEvents(id).then(
        (es) => (events = es),
        (e) => (error = e instanceof Error ? e.message : String(e)),
      )
      return
    }
    live = true
    events = []
    let seen = 0
    let pending: RunEvent[] = []
    let frame = 0
    const flush = () => {
      frame = 0
      events = events.concat(pending)
      pending = []
    }
    const stop = followRun(
      id,
      (e) => {
        if (e.seq <= seen) return
        seen = e.seq
        pending.push(e)
        frame ||= requestAnimationFrame(flush)
      },
      (r) => {
        if (runIsFinal(r.status)) live = false
        onstatus?.(r)
      },
    )
    return () => {
      stop()
      cancelAnimationFrame(frame)
    }
  })
</script>

{#if error}
  <p class="text-sm text-destructive">{error}</p>
{:else}
  <RunTranscript {events} {live} class={className} />
{/if}
