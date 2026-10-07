<script lang="ts">
  // Coolify's auth/register.blade.php for the first user, in Paperclip's
  // sign-up layout (ui/src/pages/Auth.tsx; MIT, see NOTICE): here it creates
  // the Instance admin. "Password again" is checked here; the API takes one password.
  import { api, ApiError } from '../lib/api'
  import { session, type Account } from '../lib/session.svelte'
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
      const { member } = await api<{ member: Account }>('POST', '/setup', { name, email, password })
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

<AuthShell
  title="Create the Instance admin"
  description="The first account on this instance runs the installation and becomes the Guild Master of its first guild."
>
  <form class="space-y-4" onsubmit={submit}>
    <div {@attach focus}>
      <Input label="Name" name="name" bind:value={name} error={errors.name} autocomplete="name" required />
    </div>
    <Input label="Email" type="email" name="email" bind:value={email} error={errors.email} autocomplete="username" required />
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
    <p class="text-xs text-muted-foreground">Use at least 12 characters.</p>
    {#if message}<p role="alert" class="text-xs text-destructive">{message}</p>{/if}
    <Button class="w-full" type="submit" variant="highlighted" loading={busy}>Create account</Button>
  </form>
  {#snippet footer()}
    This account has full access to the instance: its Servers, every guild and its settings.
  {/snippet}
</AuthShell>
