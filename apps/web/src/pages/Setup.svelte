<script lang="ts">
  // Coolify's auth/register.blade.php for the first user: here it creates
  // the Owner. "Password again" is checked here; the API takes one password.
  import { api, ApiError } from '../lib/api'
  import Icon from '../lib/Icon.svelte'
  import { session, type Member } from '../lib/session.svelte'
  import AuthAlert from '../lib/ui/AuthAlert.svelte'
  import AuthShell from '../lib/ui/AuthShell.svelte'
  import Button from '../lib/ui/Button.svelte'
  import Input from '../lib/ui/Input.svelte'

  let name = $state('')
  let email = $state('')
  let password = $state('')
  let passwordAgain = $state('')
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    errors = {}
    message = ''
    if (password !== passwordAgain) {
      errors = { password_confirmation: 'The passwords do not match.' }
      return
    }
    busy = true
    try {
      const { member } = await api<{ member: Member }>('POST', '/setup', { name, email, password })
      session.signedIn(member)
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

  function focus(el: HTMLElement) {
    el.querySelector('input')?.focus()
  }
</script>

<AuthShell description="Create the Owner account for this instance.">
  <div class="flex flex-col gap-4">
    <AuthAlert type="warning">
      <p class="font-medium">Full instance access</p>
      <p class="mt-0.5 text-black/70 dark:text-white/70">This first account becomes the Owner.</p>
    </AuthAlert>
    {#if message}<AuthAlert type="error"><p>{message}</p></AuthAlert>{/if}
    <form class="flex flex-col gap-4" onsubmit={submit}>
      <div {@attach focus}>
        <Input label="Name" name="name" bind:value={name} error={errors.name} autocomplete="name" required />
      </div>
      <Input label="Email" type="email" name="email" bind:value={email} error={errors.email} autocomplete="email" required />
      <Input
        label="Password"
        type="password"
        name="password"
        bind:value={password}
        error={errors.password}
        autocomplete="new-password"
        required
      />
      <Input
        label="Password again"
        type="password"
        name="password_confirmation"
        bind:value={passwordAgain}
        error={errors.password_confirmation}
        autocomplete="new-password"
        required
      />
      <div class="auth-guidance">
        <Icon name="info-circle" class="mt-0.5 size-4 shrink-0" />
        <p>Use at least 12 characters.</p>
      </div>
      <Button class="w-full justify-center" type="submit" variant="highlighted" loading={busy}>Create account</Button>
    </form>
  </div>
</AuthShell>
