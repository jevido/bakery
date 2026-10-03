<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import Field from '../lib/Field.svelte'
  import { go } from '../lib/router.svelte'
  import { session, type Member } from '../lib/session.svelte'

  let email = $state('')
  let password = $state('')
  // With two-factor on, a correct password leads to a second step.
  let step = $state<'password' | 'code'>('password')
  let useRecoveryCode = $state(false)
  let code = $state('')
  let message = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    message = ''
    try {
      const r = await api<{ member?: Member; two_factor_required?: boolean }>('POST', '/login', { email, password })
      if (r.two_factor_required) {
        step = 'code'
        code = ''
        return
      }
      signedIn(r.member!)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      message = err.message
    } finally {
      busy = false
    }
  }

  async function submitCode(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    message = ''
    try {
      const body = useRecoveryCode ? { recovery_code: code } : { code }
      const { member } = await api<{ member: Member }>('POST', '/login/two-factor', body)
      signedIn(member)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      if (err.status === 401) {
        // The Login challenge is over: start again from the password.
        step = 'password'
        password = ''
        message = err.message
      } else {
        const left = err.body.attempts_left
        message = typeof left === 'number' ? `${err.message} (${left} ${left === 1 ? 'try' : 'tries'} left)` : err.message
        code = ''
      }
    } finally {
      busy = false
    }
  }

  function signedIn(member: Member) {
    session.signedIn(member)
    go('/projects')
  }

  function focus(el: HTMLInputElement) {
    el.focus()
  }
</script>

<main class="auth">
  {#if step === 'password'}
    <form class="card" onsubmit={submit}>
      <h1>Sign in to The Bakery</h1>
      <Field label="Email" type="email" bind:value={email} autocomplete="email" required />
      <Field label="Password" type="password" bind:value={password} autocomplete="current-password" required />
      {#if message}<p class="error">{message}</p>{/if}
      <button class="primary" disabled={busy}>Sign in</button>
    </form>
  {:else}
    <form class="card" onsubmit={submitCode}>
      <h1>Two-factor authentication</h1>
      {#key useRecoveryCode}
        <label class="field">
          <span class="muted">{useRecoveryCode ? 'One of your recovery codes' : 'The 6-digit code from your authenticator app'}</span>
          {#if useRecoveryCode}
            <input bind:value={code} autocomplete="off" required aria-label="Recovery code" {@attach focus} />
          {:else}
            <input bind:value={code} inputmode="numeric" autocomplete="one-time-code" maxlength="6" required aria-label="Code" {@attach focus} />
          {/if}
        </label>
      {/key}
      {#if message}<p class="error">{message}</p>{/if}
      <button class="primary" disabled={busy}>Sign in</button>
      <button
        type="button"
        class="link"
        onclick={() => {
          useRecoveryCode = !useRecoveryCode
          code = ''
          message = ''
        }}>{useRecoveryCode ? 'Use the code from your app' : 'Use a recovery code instead'}</button
      >
      <button type="button" class="link" onclick={() => ((step = 'password'), (message = ''))}>Back</button>
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
  }
  .link {
    background: none;
    border: 0;
    color: var(--muted);
    text-decoration: underline;
    padding: 0;
    cursor: pointer;
  }
</style>
