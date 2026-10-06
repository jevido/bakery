<script lang="ts">
  // Coolify's invitation/accept.blade.php. Coolify's invitee already has an
  // account; a Bakery invitee may not, so for a new email the card also asks
  // for a name and a password before "Accept invitation". An existing
  // Member joins the guild with one click while signed in as that email.
  import { api, ApiError } from '../lib/api'
  import Icon from '../lib/Icon.svelte'
  import { go, returnAfterLogin } from '../lib/router.svelte'
  import { session, type Account, type Role } from '../lib/session.svelte'
  import AuthAlert from '../lib/ui/AuthAlert.svelte'
  import AuthShell from '../lib/ui/AuthShell.svelte'
  import Button from '../lib/ui/Button.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'

  let { token }: { token: string } = $props()

  type Invitation = { email: string; role: Role; expires_at: string }

  let invitation = $state.raw<Invitation | null>(null)
  let guild = $state('')
  // Whether the email already has a Member, who accepts while signed in.
  let existingMember = $state(false)
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
    api<{ invitation: Invitation; guild: { name: string }; existing_member: boolean }>(
      'GET',
      `/invitations/by-token/${encodeURIComponent(token)}`,
    )
      .then((r) => {
        guild = r.guild.name
        existingMember = r.existing_member
        invitation = r.invitation
      })
      .catch((e) => (refusal = e instanceof ApiError && e.status === 404 ? 'This invitation link is not valid.' : e.message))
  })

  async function accept(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      const body = existingMember ? {} : { name, password }
      await api<{ member: Account }>('POST', `/invitations/by-token/${encodeURIComponent(token)}/accept`, body)
      // The guild joined is now the Current guild, with another Role.
      await session.refresh()
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

  function signInToAccept() {
    returnAfterLogin(`/invite/${token}`)
    go('/login')
  }
</script>

<AuthShell description={guild ? `Review your invitation to join ${guild} on The Bakery.` : 'Review your invitation to join The Bakery.'}>
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
        <p>You have been invited to join <span class="font-medium" data-testid="invite-guild">{guild}</span> on The Bakery.</p>
      </div>
      <dl class="divide-y divide-neutral-200 rounded-lg border border-neutral-200 text-sm dark:divide-white/10 dark:border-white/10">
        <div class="flex items-center justify-between gap-4 px-3 py-2.5">
          <dt class="text-neutral-500 dark:text-fg-dim">Guild</dt>
          <dd class="min-w-0 truncate font-medium text-neutral-900 dark:text-white">{guild}</dd>
        </div>
        <div class="flex items-center justify-between gap-4 px-3 py-2.5">
          <dt class="text-neutral-500 dark:text-fg-dim">Email</dt>
          <dd class="min-w-0 truncate font-medium text-neutral-900 dark:text-white">{invitation.email}</dd>
        </div>
        <div class="flex items-center justify-between gap-4 px-3 py-2.5">
          <dt class="text-neutral-500 dark:text-fg-dim">Role</dt>
          <dd class="font-medium text-neutral-900 dark:text-white">{role(invitation.role)}</dd>
        </div>
      </dl>
      {#if existingMember}
        {#if message}<AuthAlert type="error"><p>{message}</p></AuthAlert>{/if}
        {#if session.member?.email === invitation.email}
          <form onsubmit={accept}>
            <Button class="w-full justify-center" type="submit" variant="highlighted" loading={busy}>Join {guild}</Button>
          </form>
        {:else}
          <AuthAlert type="warning">
            {#if session.member}
              You are signed in as {session.member.email}. Sign in as {invitation.email} to accept this invitation.
            {:else}
              You already have an account. Sign in as {invitation.email} to accept this invitation.
            {/if}
          </AuthAlert>
          {#if session.member}
            <Button class="w-full justify-center" onclick={() => session.logout()}>Logout</Button>
          {:else}
            <Button class="w-full justify-center" variant="highlighted" onclick={signInToAccept}>Sign in to accept</Button>
          {/if}
        {/if}
      {:else if session.member}
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
