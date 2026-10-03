<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import Field from '../lib/Field.svelte'
  import { go } from '../lib/router.svelte'
  import { session, type Member, type Role } from '../lib/session.svelte'

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
      const { member } = await api<{ member: Member }>('POST', `/invitations/by-token/${encodeURIComponent(token)}/accept`, {
        name,
        password,
      })
      session.signedIn(member)
      go('/projects')
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
</script>

<main class="auth">
  {#if refusal}
    <div class="card" data-testid="invite-refused">
      <h1>Invitation</h1>
      <p class="error">{refusal}</p>
      <p class="muted">Ask whoever invited you for a new link.</p>
    </div>
  {:else if invitation === null}
    <p class="muted">Loading…</p>
  {:else if session.member}
    <div class="card">
      <h1>Invitation</h1>
      <p>
        This invitation is for <strong>{invitation.email}</strong>, but you are signed in as {session.member.email}. Sign out to
        accept it.
      </p>
      <button onclick={() => session.logout()}>Log out</button>
    </div>
  {:else}
    <form class="card" onsubmit={accept}>
      <h1>Join The Bakery</h1>
      <p class="muted">You were invited as <strong>{invitation.role}</strong>. Pick your name and a password to sign in with.</p>
      <label class="field">
        <span>Email</span>
        <input type="email" value={invitation.email} readonly aria-label="Email" />
      </label>
      <Field label="Name" bind:value={name} error={errors.name} autocomplete="name" required />
      <Field
        label="Password (at least 12 characters)"
        type="password"
        bind:value={password}
        error={errors.password}
        autocomplete="new-password"
        required
      />
      {#if message}<p class="error">{message}</p>{/if}
      <button class="primary" disabled={busy}>Accept invitation</button>
    </form>
  {/if}
</main>

<style>
  .field {
    display: grid;
    gap: 0.3rem;
  }
  .field span {
    font-size: 0.8rem;
    color: var(--muted);
  }
</style>
