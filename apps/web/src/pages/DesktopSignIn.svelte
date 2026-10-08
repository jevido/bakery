<script lang="ts">
  // Paperclip's CLI auth page (ui/src/pages/CliAuth.tsx; MIT, see NOTICE)
  // for the Desktop app's sign-in: the person approves, in the browser they
  // are signed in to, the Desktop app that opened this link. It sits outside
  // the guild shell like the Invitation page; App.svelte shows the login page
  // first when nobody is signed in and comes back here after. Paperclip's
  // Command and Requested access rows go: the Desktop app has no command,
  // and a Desktop always acts as its person. The page does not poll; it
  // reads the sign-in again after Approve or Cancel, as Paperclip's does.
  import { ApiError } from '../lib/api'
  import { approveDesktopSignIn, cancelDesktopSignIn, getDesktopSignIn, type DesktopSignIn } from '../lib/desktops'
  import { session } from '../lib/session.svelte'
  import Button from '../lib/ui/Button.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'

  let { id, token }: { id: number; token: string } = $props()

  let signIn = $state.raw<DesktopSignIn | null>(null)
  // Set when the sign-in is unknown or the token is wrong (the API's 404).
  let missing = $state(false)
  let loadError = $state('')
  let message = $state('')
  let busy = $state<'approve' | 'cancel' | null>(null)

  async function load() {
    try {
      signIn = await getDesktopSignIn(id, token)
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) missing = true
      else loadError = e instanceof Error ? e.message : String(e)
    }
  }

  $effect(() => {
    signIn = null
    missing = false
    loadError = ''
    if (token) load()
    else missing = true
  })

  async function act(what: 'approve' | 'cancel') {
    busy = what
    message = ''
    try {
      await (what === 'approve' ? approveDesktopSignIn(id, token) : cancelDesktopSignIn(id, token))
    } catch (e) {
      // A sign-in that expired or was cancelled meanwhile shows that state below.
      if (!(e instanceof ApiError) || (e.status !== 404 && e.status !== 422)) message = e instanceof Error ? e.message : String(e)
    } finally {
      busy = null
    }
    await load()
  }
</script>

{#snippet card(heading: string, text: string, testid: string)}
  <main class="chrome min-h-screen bg-background px-6 py-12 text-foreground">
    <div class="mx-auto max-w-xl border border-border bg-card p-6" data-testid={testid}>
      <h1 class="text-xl font-semibold">{heading}</h1>
      <p class="mt-2 text-sm text-muted-foreground">{text}</p>
    </div>
  </main>
{/snippet}

{#if missing}
  {@render card(
    'Desktop sign-in not found',
    'This link is not valid. Start again from the desktop app to get a new one.',
    'desktop-sign-in-missing',
  )}
{:else if loadError}
  {@render card('Desktop sign-in unavailable', loadError, 'desktop-sign-in-error')}
{:else if signIn === null}
  <main class="chrome mx-auto max-w-xl px-6 py-10 text-sm text-muted-foreground"><Spinner text="Loading desktop sign-in…" /></main>
{:else if signIn.status === 'approved'}
  {@render card('Desktop app approved', 'Desktop app approved. You can go back to the app.', 'desktop-sign-in-approved')}
{:else if signIn.status === 'expired' || signIn.status === 'cancelled'}
  {@render card(
    signIn.status === 'expired' ? 'Desktop sign-in expired' : 'Desktop sign-in cancelled',
    `This sign-in has ${signIn.status === 'expired' ? 'expired' : 'been cancelled'}. Start again from the desktop app.`,
    `desktop-sign-in-${signIn.status}`,
  )}
{:else}
  <main class="chrome min-h-screen bg-background px-6 py-12 text-foreground">
    <div class="mx-auto max-w-xl border border-border bg-card p-6" data-testid="desktop-sign-in">
      <h1 class="text-xl font-semibold">Approve The Bakery desktop app</h1>
      <p class="mt-2 text-sm text-muted-foreground">
        A desktop app is asking to sign in to this Bakery as you. Approve it only if you just connected it yourself.
      </p>

      <div class="mt-5 space-y-3 text-sm">
        <div>
          <div class="text-muted-foreground">Client</div>
          <div class="text-foreground" data-testid="desktop-sign-in-client">{signIn.client_name}</div>
        </div>
        <div>
          <div class="text-muted-foreground">Signed in as</div>
          <div class="text-foreground" data-testid="desktop-sign-in-member">{session.member?.email}</div>
        </div>
        <div>
          <div class="text-muted-foreground">Expires</div>
          <div class="text-foreground">{new Date(signIn.expires_at).toLocaleString()}</div>
        </div>
      </div>

      {#if message}<p role="alert" class="mt-4 text-sm text-destructive">{message}</p>{/if}

      <div class="mt-5 flex gap-3">
        <Button variant="highlighted" onclick={() => act('approve')} loading={busy === 'approve'} disabled={!signIn.can_approve || busy !== null}>
          Approve
        </Button>
        <Button onclick={() => act('cancel')} loading={busy === 'cancel'} disabled={busy !== null}>Cancel</Button>
      </div>
    </div>
  </main>
{/if}
