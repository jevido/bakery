<script lang="ts">
  // "Connect a Bakery": the address, then "Approve in your browser" with the
  // approve link (the window opens it; it is shown as a link too, for when no
  // browser opened, and in `serve`), a spinner while the Go side polls, and
  // the outcome. Paperclip's CLI does the same in a terminal
  // (cli/src/client/board-auth.ts; MIT, see NOTICE).
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Dialog from '@bakery/ui/components/ui/dialog'
  import { Input } from '@bakery/ui/components/ui/input'
  import { Label } from '@bakery/ui/components/ui/label'
  import { untrack } from 'svelte'
  import { CircleCheck, CircleX, ExternalLink, LoaderCircle } from '@lucide/svelte'
  import { connected, connectDialog } from './bakeries.svelte'
  import { cancelConnect, connect, connectStatus, onEvent, type ConnectStart, type ConnectState } from './desktop'

  let address = $state('')
  let starting = $state(false)
  let error = $state('')
  let started = $state<ConnectStart | null>(null)
  let progress = $state<ConnectState | null>(null)

  const outcome: Record<string, string> = {
    expired: 'The sign-in expired before it was approved. Start again.',
    cancelled: 'The sign-in was cancelled.',
  }

  // The `connect` event settles the attempt; a slow poll of ConnectStatus
  // catches one the event stream missed.
  $effect(() => {
    if (!started) return
    const id = started.id
    const off = onEvent<ConnectState>('connect', (s) => {
      if (s.id === id) progress = s
    })
    const timer = setInterval(async () => {
      try {
        const s = await connectStatus(id)
        if (s.status !== 'pending') progress = s
      } catch {
        // The next tick tries again.
      }
    }, 2000)
    return () => {
      off()
      clearInterval(timer)
    }
  })

  $effect(() => {
    if (progress?.status === 'approved') void connected.load()
  })

  function reset() {
    address = ''
    error = ''
    started = null
    progress = null
    starting = false
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    starting = true
    error = ''
    try {
      started = await connect(address)
      progress = { id: started.id, address: started.address, status: 'pending' }
    } catch (err) {
      error = (err as Error).message
    }
    starting = false
  }

  // However the dialog closes (Cancel, Escape, outside), a pending connect
  // is cancelled on the Bakery and the next open starts afresh.
  $effect(() => {
    if (connectDialog.open) return
    untrack(() => {
      if (started && progress?.status === 'pending') void cancelConnect(started.id).catch(() => {})
      reset()
    })
  })
</script>

<Dialog.Root bind:open={connectDialog.open}>
  <Dialog.Content class="sm:max-w-md" data-testid="connect-dialog">
    {#if !started}
      <form class="flex flex-col gap-4" onsubmit={submit}>
        <Dialog.Header>
          <Dialog.Title>Connect a Bakery</Dialog.Title>
          <Dialog.Description>The Bakery's address, as you open it in the browser.</Dialog.Description>
        </Dialog.Header>
        <div class="flex flex-col gap-2">
          <Label for="bakery-address">Address</Label>
          <Input id="bakery-address" bind:value={address} placeholder="https://bakery.example.com" autocomplete="url" required />
          {#if error}
            <p class="text-sm text-destructive" role="alert">{error}</p>
          {/if}
        </div>
        <Dialog.Footer>
          <Button type="button" variant="outline" onclick={() => (connectDialog.open = false)}>Cancel</Button>
          <Button type="submit" disabled={starting || !address.trim()}>
            {#if starting}<LoaderCircle class="animate-spin" />{/if}
            Connect
          </Button>
        </Dialog.Footer>
      </form>
    {:else if progress?.status === 'approved'}
      <Dialog.Header>
        <Dialog.Title class="flex items-center gap-2"><CircleCheck class="size-5 text-green-500" /> Connected</Dialog.Title>
        <Dialog.Description>This desktop is signed in to {started.address}.</Dialog.Description>
      </Dialog.Header>
      <Dialog.Footer>
        <Button onclick={() => (connectDialog.open = false)}>Done</Button>
      </Dialog.Footer>
    {:else if progress && progress.status !== 'pending'}
      <Dialog.Header>
        <Dialog.Title class="flex items-center gap-2"><CircleX class="size-5 text-destructive" /> Not connected</Dialog.Title>
        <Dialog.Description role="alert">{progress.error || outcome[progress.status] || `The sign-in ended: ${progress.status}.`}</Dialog.Description>
      </Dialog.Header>
      <Dialog.Footer>
        <Button variant="outline" onclick={() => (connectDialog.open = false)}>Close</Button>
        <Button onclick={reset}>Start again</Button>
      </Dialog.Footer>
    {:else}
      <Dialog.Header>
        <Dialog.Title>Approve in your browser</Dialog.Title>
        <Dialog.Description>
          Approve The Bakery desktop app at {started.address}, signing in there if you need to. This window carries on by itself.
        </Dialog.Description>
      </Dialog.Header>
      <a
        href={started.approval_url}
        target="_blank"
        rel="noreferrer"
        class="flex items-center gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs break-all text-foreground hover:bg-accent"
        data-testid="approval-link"
      >
        <ExternalLink class="size-4 shrink-0 text-muted-foreground" />
        {started.approval_url}
      </a>
      <p class="flex items-center gap-2 text-sm text-muted-foreground">
        <LoaderCircle class="size-4 animate-spin" aria-hidden="true" /> Waiting for approval…
      </p>
      <Dialog.Footer>
        <Button variant="outline" onclick={() => (connectDialog.open = false)}>Cancel</Button>
      </Dialog.Footer>
    {/if}
  </Dialog.Content>
</Dialog.Root>
