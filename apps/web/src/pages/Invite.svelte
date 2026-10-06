<script lang="ts">
  // Coolify's invitation/accept.blade.php. Coolify's invitee already has an
  // account; a Bakery invitee does not, so the card also asks for a name and
  // a password before "Accept invitation".
  import { api, ApiError } from '../lib/api'
  import Icon from '../lib/Icon.svelte'
  import { go } from '../lib/router.svelte'
  import { session, type Account, type Role } from '../lib/session.svelte'
  import AuthAlert from '../lib/ui/AuthAlert.svelte'
  import AuthShell from '../lib/ui/AuthShell.svelte'
  import Button from '../lib/ui/Button.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'

  let { token }: { token: string } = $props()

  type Invitation = { email: string; role: Role; expires_at: string }

  let invitation = $state.raw<Invitation | null>(null)
  // Why the link cannot be used (unknown, expired, used or revoked).
  let refusal = $state('')
  let name = $state('')
  let password = $state('')
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  $effect(() => {
    invitation = null
    refusal = ''
    api<{ invitation: Invitation }>('GET', `/invitations/by-token/${encodeURIComponent(token)}`)
      .then((r) => (invitation = r.invitation))
      .catch((e) => (refusal = e instanceof ApiError && e.status === 404 ? 'This invitation link is not valid.' : e.message))
  })

  async function accept(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      const { member } = await api<{ member: Account }>('POST', `/invitations/by-token/${encodeURIComponent(token)}/accept`, {
        name,
        password,
      })
      session.signedIn(member)
      go('/')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      if (err.status === 410 || err.status === 404) {
        refusal = err.message
        return
      }
      errors = err.errors
      if (Object.keys(err.errors).length === 0) message = err.message
    } finally {
      busy = false
    }
  }

  const role = (r: Role) => r.charAt(0).toUpperCase() + r.slice(1)
</script>

<AuthShell description="Review your invitation to join The Bakery.">
  {#if refusal}
    <div class="flex flex-col gap-4" data-testid="invite-refused">
      <AuthAlert type="error"><p>{refusal}</p></AuthAlert>
      <div class="auth-guidance">
        <Icon name="info-circle" class="mt-0.5 size-4 shrink-0" />
        <p>Ask whoever invited you for a new link.</p>
      </div>
    </div>
  {:else if invitation === null}
    <div class="flex justify-center text-sm text-neutral-500 dark:text-fg-dim"><Spinner text="Loading…" /></div>
  {:else}
    <div class="flex flex-col gap-4">
      <div class="auth-guidance">
        <Icon name="teams" class="mt-0.5 size-4 shrink-0" />
        <p>You have been invited to collaborate on The Bakery.</p>
      </div>
      <dl class="divide-y divide-neutral-200 rounded-lg border border-neutral-200 text-sm dark:divide-white/10 dark:border-white/10">
        <div class="flex items-center justify-between gap-4 px-3 py-2.5">
          <dt class="text-neutral-500 dark:text-fg-dim">Email</dt>
          <dd class="min-w-0 truncate font-medium text-neutral-900 dark:text-white">{invitation.email}</dd>
        </div>
        <div class="flex items-center justify-between gap-4 px-3 py-2.5">
          <dt class="text-neutral-500 dark:text-fg-dim">Role</dt>
          <dd class="font-medium text-neutral-900 dark:text-white">{role(invitation.role)}</dd>
        </div>
      </dl>
      {#if session.member}
        <AuthAlert type="warning">
          You are signed in as {session.member.email}. Log out to accept this invitation.
        </AuthAlert>
        <Button class="w-full justify-center" onclick={() => session.logout()}>Logout</Button>
      {:else}
        {#if message}<AuthAlert type="error"><p>{message}</p></AuthAlert>{/if}
        <form class="flex flex-col gap-4" onsubmit={accept}>
          <Input label="Name" name="name" bind:value={name} error={errors.name} autocomplete="name" required />
          <Input
            label="Password"
            type="password"
            name="password"
            bind:value={password}
            error={errors.password}
            autocomplete="new-password"
            required
          />
          <div class="auth-guidance">
            <Icon name="info-circle" class="mt-0.5 size-4 shrink-0" />
            <p>Use at least 12 characters.</p>
          </div>
          <Button class="w-full justify-center" type="submit" variant="highlighted" loading={busy}>Accept invitation</Button>
        </form>
      {/if}
    </div>
  {/if}
</AuthShell>
