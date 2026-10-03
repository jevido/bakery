<script lang="ts">
  // Coolify's auth/login.blade.php and, as the second step, its
  // auth/two-factor-challenge.blade.php. Left out until instance Settings
  // has them: "Forgot password?", the "Register" footer link and the OAuth
  // buttons.
  import { api, ApiError } from '../lib/api'
  import Icon from '../lib/Icon.svelte'
  import { go } from '../lib/router.svelte'
  import { session, type Member } from '../lib/session.svelte'
  import AuthAlert from '../lib/ui/AuthAlert.svelte'
  import AuthShell from '../lib/ui/AuthShell.svelte'
  import Button from '../lib/ui/Button.svelte'
  import Input from '../lib/ui/Input.svelte'

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
        useRecoveryCode = false
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

  async function submitCode(e?: SubmitEvent) {
    e?.preventDefault()
    if (busy) return
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

  // Coolify submits the authenticator code as soon as it has six digits.
  function authenticatorInput(e: Event & { currentTarget: HTMLInputElement }) {
    code = e.currentTarget.value.replace(/\D/g, '').slice(0, 6)
    e.currentTarget.value = code
    if (code.length === 6) void submitCode()
  }

  function switchCode(recovery: boolean) {
    useRecoveryCode = recovery
    code = ''
    message = ''
  }

  function signedIn(member: Member) {
    session.signedIn(member)
    go('/')
  }

  function focus(el: HTMLElement) {
    ;(el.querySelector('input') ?? el).focus()
  }
</script>

{#if step === 'password'}
  <AuthShell description="Sign in to manage your applications and infrastructure.">
    <div class="flex flex-col gap-4">
      {#if message}<AuthAlert type="error"><p>{message}</p></AuthAlert>{/if}
      <form class="flex flex-col gap-4" onsubmit={submit}>
        <div {@attach focus}>
          <Input label="Email" type="email" name="email" bind:value={email} autocomplete="email" required />
        </div>
        <Input label="Password" type="password" name="password" bind:value={password} autocomplete="current-password" required />
        <Button class="w-full justify-center" type="submit" variant="highlighted" loading={busy}>Login</Button>
      </form>
    </div>
  </AuthShell>
{:else}
  <AuthShell description="Verify your identity to finish signing in.">
    <div class="flex flex-col gap-4">
      {#if message}<AuthAlert type="error"><p>{message}</p></AuthAlert>{/if}
      <div class="auth-guidance">
        <Icon name="info-circle" class="mt-0.5 size-4 shrink-0" />
        {#if useRecoveryCode}
          <p>Enter one of the recovery codes you saved when setting up two-factor authentication.</p>
        {:else}
          <p>Enter the 6-digit code from your authenticator app.</p>
        {/if}
      </div>
      <form class="flex flex-col gap-4" onsubmit={submitCode}>
        {#if useRecoveryCode}
          <div class="flex flex-col gap-3" {@attach focus}>
            <Input label="Recovery code" name="recovery_code" bind:value={code} autocomplete="one-time-code" required />
            <button type="button" class="auth-text-link self-center" onclick={() => switchCode(false)}>Use an authenticator code</button>
          </div>
        {:else}
          <div class="flex flex-col gap-3">
            <input
              type="text"
              name="code"
              value={code}
              inputmode="numeric"
              pattern="[0-9]*"
              maxlength="6"
              autocomplete="one-time-code"
              aria-label="Two-factor authentication code"
              required
              oninput={authenticatorInput}
              {@attach focus}
              class="mx-auto h-14 w-64 rounded-md border border-neutral-300 bg-white px-4 text-center text-xl font-semibold tracking-[0.5em] text-neutral-900 transition-colors focus:border-warning focus:ring-1 focus:ring-warning focus:outline-none dark:border-white/10 dark:bg-coolgray-100 dark:text-white"
            />
            <button type="button" class="auth-text-link self-center" onclick={() => switchCode(true)}>Use a recovery code</button>
          </div>
        {/if}
        <Button class="w-full justify-center" type="submit" variant="highlighted" loading={busy}>Verify and continue</Button>
      </form>
    </div>
    {#snippet footer()}
      <span>Not your account?</span>
      <button type="button" class="auth-text-link" onclick={() => ((step = 'password'), (password = ''), (message = ''))}>Back to login</button>
    {/snippet}
  </AuthShell>
{/if}
