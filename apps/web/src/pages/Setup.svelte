<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import Field from '../lib/Field.svelte'
  import { session, type Owner } from '../lib/session.svelte'

  let name = $state('')
  let email = $state('')
  let password = $state('')
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      const { owner } = await api<{ owner: Owner }>('POST', '/setup', { name, email, password })
      session.signedIn(owner)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      if (err.status === 409) {
        session.signedOut()
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
  <form class="card" onsubmit={submit}>
    <h1>Welcome to Bakery</h1>
    <p class="muted">Create the owner account. This happens once; you will sign in with it from now on.</p>
    <Field label="Name" bind:value={name} error={errors.name} autocomplete="name" required />
    <Field label="Email" type="email" bind:value={email} error={errors.email} autocomplete="email" required />
    <Field
      label="Password (at least 12 characters)"
      type="password"
      bind:value={password}
      error={errors.password}
      autocomplete="new-password"
      required
    />
    {#if message}<p class="error">{message}</p>{/if}
    <button class="primary" disabled={busy}>Create owner</button>
  </form>
</main>
