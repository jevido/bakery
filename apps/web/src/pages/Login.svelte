<script lang="ts">
  // Paperclip's sign-in page (ui/src/pages/Auth.tsx; MIT, see NOTICE) with
  // Coolify's auth/login.blade.php behind it and, as the second step, its
  // auth/two-factor-challenge.blade.php. Left out until instance Settings
  // has them: "Forgot password?", "Need an account?" (registration) and the
  // OAuth buttons.
  import { api, ApiError } from '../lib/api'
  import { go, takeReturnAfterLogin } from '../lib/router.svelte'
  import { session, type Account } from '../lib/session.svelte'
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
      const r = await api<{ member?: Account; two_factor_required?: boolean }>('POST', '/login', { email, password })
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
      const { member } = await api<{ member: Account }>('POST', '/login/two-factor', body)
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

  async function signedIn(member: Account) {
    await session.signedIn(member)
    go(takeReturnAfterLogin() ?? '/')
  }

  function focus(el: HTMLElement) {
    ;(el.querySelector('input') ?? el).focus()
  }
</script>

{#if step === 'password'}
  <AuthShell title="Sign in to The Bakery" description="Use your email and password to access this instance.">
    <form class="space-y-4" onsubmit={submit}>
      <div {@attach focus}>
        <Input label="Email" type="email" name="email" bind:value={email} autocomplete="username" required />
      </div>
      <Input label="Password" type="password" name="password" bind:value={password} autocomplete="current-password" required />
      {#if message}<p role="alert" class="text-xs text-destructive">{message}</p>{/if}
      <Button class="w-full" type="submit" variant="highlighted" loading={busy}>Sign in</Button>
    </form>
  </AuthShell>
{:else}
  <AuthShell
    title="Two-factor authentication"
    description={useRecoveryCode
      ? 'Enter one of the recovery codes you saved when setting up two-factor authentication.'
      : 'Enter the 6-digit code from your authenticator app.'}
  >
    <form class="space-y-4" onsubmit={submitCode}>
      {#if useRecoveryCode}
        <div {@attach focus}>
          <Input label="Recovery code" name="recovery_code" bind:value={code} autocomplete="one-time-code" required />
        </div>
      {:else}
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
          class="h-12 w-full rounded-md border border-border bg-transparent px-3 text-center font-mono text-xl tracking-[0.5em] outline-none focus:ring-1 focus:ring-ring"
        />
      {/if}
      {#if message}<p role="alert" class="text-xs text-destructive">{message}</p>{/if}
      <Button class="w-full" type="submit" variant="highlighted" loading={busy}>Verify and continue</Button>
      <button
        type="button"
        class="text-sm text-muted-foreground underline underline-offset-2 hover:text-foreground"
        onclick={() => switchCode(!useRecoveryCode)}
      >
        {useRecoveryCode ? 'Use an authenticator code' : 'Use a recovery code'}
      </button>
    </form>
    {#snippet footer()}
      Not your account?
      <button
        type="button"
        class="font-medium text-foreground underline underline-offset-2"
        onclick={() => ((step = 'password'), (password = ''), (message = ''))}>Back to sign in</button
      >
    {/snippet}
  </AuthShell>
{/if}
