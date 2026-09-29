<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import Field from '../lib/Field.svelte'
  import { go } from '../lib/router.svelte'
  import { session, type Owner } from '../lib/session.svelte'

  let email = $state('')
  let password = $state('')
  let message = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    message = ''
    try {
      const { owner } = await api<{ owner: Owner }>('POST', '/login', { email, password })
      session.signedIn(owner)
      go('/projects')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      message = err.message
    } finally {
      busy = false
    }
  }
</script>

<main class="auth">
  <form class="card" onsubmit={submit}>
    <h1>Sign in to Bakery</h1>
    <Field label="Email" type="email" bind:value={email} autocomplete="email" required />
    <Field label="Password" type="password" bind:value={password} autocomplete="current-password" required />
    {#if message}<p class="error">{message}</p>{/if}
    <button class="primary" disabled={busy}>Sign in</button>
  </form>
</main>
