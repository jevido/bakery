<script lang="ts">
  // One Run's Transcript, as apps/web/src/lib/RunView.svelte reads it for
  // the dashboard: a final Run's events read once, a queued or running one
  // followed live until it ends. A Run this desktop executes is read
  // straight from the Runner (localRuns, no round trip to the Bakery); one
  // running on another of the person's connected desktops is followed with
  // FollowRun, whose `run-events` grow it instead.
  import RunTranscript from '@bakery/ui/RunTranscript.svelte'
  import { runIsFinal } from '@bakery/ui/runStatus'
  import { followRun, localRuns, onEvent, runEvents, unfollowRun, type Run, type RunEvent, type RunEventsUpdate, type RunsEvent } from './desktop'

  let {
    address,
    guildID,
    run,
    class: className = '',
  }: { address: string; guildID: number; run: Pick<Run, 'id' | 'status'>; class?: string } = $props()

  let events = $state.raw<RunEvent[]>([])
  let live = $state(false)
  let error = $state('')

  const id = $derived(run.id)
  const final = $derived(runIsFinal(run.status))

  $effect(() => {
    const [a, g, runID] = [address, guildID, id]
    error = ''
    if (final) {
      live = false
      runEvents(a, g, runID).then(
        (es) => {
          if (a === address && g === guildID && runID === id) events = es
        },
        (e: Error) => (error = e.message),
      )
      return
    }
    live = true
    events = []

    let stopped = false
    let following = false
    let offRuns: (() => void) | undefined
    let offFollow: (() => void) | undefined

    const fromLocal = async () => {
      const local = (await localRuns()).find((l) => l.address === a && l.run_id === runID)
      if (stopped || !local) return false
      events = local.events ?? []
      if (runIsFinal(local.status)) live = false
      return true
    }

    void fromLocal().then((found) => {
      if (stopped) return
      if (found) {
        offRuns = onEvent<RunsEvent>('runs', (e) => {
          if (e.address === a && e.run_id === runID) void fromLocal()
        })
        return
      }
      following = true
      void followRun(a, g, runID)
      offFollow = onEvent<RunEventsUpdate>('run-events', (e) => {
        if (e.address !== a || e.run_id !== runID) return
        if (e.kind === 'event' && e.event) events = [...events, e.event]
        else if (e.kind === 'end') live = false
      })
    })

    return () => {
      stopped = true
      offRuns?.()
      offFollow?.()
      if (following) void unfollowRun(a, g, runID)
    }
  })
</script>

{#if error}
  <p class="text-sm text-destructive">{error}</p>
{:else}
  <RunTranscript {events} {live} class={className} />
{/if}
