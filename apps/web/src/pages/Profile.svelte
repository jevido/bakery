<script lang="ts">
  // Paperclip's ProfileSettings (ui/src/pages/ProfileSettings.tsx; MIT, see
  // NOTICE): the title and the header card with the initials, name and email,
  // over Coolify's groups (resources/views/livewire/profile/index.blade.php;
  // Apache-2.0, see NOTICE). Paperclip's avatar upload and Coolify's profile
  // picture and email change are features The Bakery does not have yet, so
  // the email is read-only text and the avatar shows initials.
  import { UserRoundPen } from '@lucide/svelte'
  import { renderSVG } from 'uqr'
  import * as Avatar from '$lib/components/ui/avatar'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { session, type Account } from '../lib/session.svelte'
  import SettingsGroup from '../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../lib/settings/SettingsPage.svelte'
  import Button from '../lib/ui/Button.svelte'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import CopyButton from '../lib/ui/CopyButton.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import { toast } from '../lib/ui/toast.svelte'

  type TwoFactorStatus = { state: 'off' | 'pending' | 'on'; recovery_codes_left: number }

  const currentName = $derived(session.member?.name?.trim() || 'Account')
  const initials = $derived.by(() => {
    const parts = currentName.split(/\s+/).filter(Boolean)
    if (parts.length >= 2) return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase()
    return currentName.slice(0, 2).toUpperCase()
  })

  // Profile
  let name = $state(session.member?.name ?? '')
  let nameErrors = $state<Record<string, string>>({})
  let savingName = $state(false)

  async function saveName(e: SubmitEvent) {
    e.preventDefault()
    nameErrors = {}
    savingName = true
    try {
      const { member } = await api<{ member: Account }>('PATCH', '/me', { name })
      session.signedIn(member)
      name = member.name
      toast.success('Saved')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      nameErrors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      savingName = false
    }
  }

  // Password
  let currentPassword = $state('')
  let newPassword = $state('')
  let repeatPassword = $state('')
  let passwordErrors = $state<Record<string, string>>({})
  let savingPassword = $state(false)

  async function savePassword(e: SubmitEvent) {
    e.preventDefault()
    passwordErrors = {}
    if (newPassword !== repeatPassword) {
      passwordErrors = { repeat_password: 'the passwords are not the same' }
      return
    }
    savingPassword = true
    try {
      const { member } = await api<{ member: Account }>('POST', '/me/password', {
        current_password: currentPassword,
        new_password: newPassword,
      })
      session.signedIn(member)
      currentPassword = newPassword = repeatPassword = ''
      toast.success('Password changed; other browsers are signed out')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      passwordErrors = Object.keys(err.errors).length ? err.errors : { new_password: err.message }
    } finally {
      savingPassword = false
    }
  }

  // Sessions
  async function signOutOthers() {
    await api('POST', '/me/sign-out-others')
    toast.success('Other browsers are signed out')
  }

  // Two-factor
  let status = $state.raw<TwoFactorStatus | null>(null)
  let statusError = $state('')
  // A secret being set up, until it is confirmed with a code.
  let setup = $state.raw<{ secret: string; otpauth_uri: string } | null>(null)
  // Recovery codes just handed out; the API never shows them again.
  let recoveryCodes = $state.raw<string[] | null>(null)
  let code = $state('')
  let password = $state('')
  let twoFactorError = $state('')
  let action = $state<'' | 'regenerate' | 'disable'>('')
  let busy = $state(false)
  let copiedCodes = $state(false)

  async function loadStatus() {
    status = await api<TwoFactorStatus>('GET', '/me/two-factor')
  }
  loadStatus().catch((e) => (statusError = e.message))

  let qr = $derived(setup ? renderSVG(setup.otpauth_uri, { border: 2 }) : '')
  let groupedSecret = $derived(setup ? (setup.secret.match(/.{1,4}/g) ?? []).join(' ') : '')

  function failed(err: unknown) {
    if (!(err instanceof ApiError)) throw err
    twoFactorError = Object.values(err.errors)[0] ?? err.message
  }

  async function run(f: () => Promise<void>) {
    busy = true
    twoFactorError = ''
    try {
      await f()
    } catch (err) {
      failed(err)
    } finally {
      busy = false
    }
  }

  function startSetup() {
    return run(async () => {
      setup = await api<{ secret: string; otpauth_uri: string }>('POST', '/me/two-factor')
      code = ''
    })
  }

  function confirmSetup(e: SubmitEvent) {
    e.preventDefault()
    return run(async () => {
      const r = await api<{ recovery_codes: string[] }>('POST', '/me/two-factor/confirm', { code })
      recoveryCodes = r.recovery_codes
      setup = null
      code = ''
      session.signedIn({ ...session.member!, two_factor: true })
      await loadStatus()
    })
  }

  function submitAction(e: SubmitEvent) {
    e.preventDefault()
    return run(async () => {
      if (action === 'regenerate') {
        const r = await api<{ recovery_codes: string[] }>('POST', '/me/two-factor/recovery-codes', { code })
        recoveryCodes = r.recovery_codes
      } else {
        // A Recovery code has a dash; an Authenticator code is 6 digits.
        const second = /^\s*\d{6}\s*$/.test(code) ? { code } : { recovery_code: code }
        await api('DELETE', '/me/two-factor', { password, ...second })
        session.signedIn({ ...session.member!, two_factor: false })
        recoveryCodes = null
      }
      action = ''
      code = password = ''
      await loadStatus()
    })
  }

  async function copyCodes() {
    await navigator.clipboard?.writeText(recoveryCodes!.join('\n'))
    copiedCodes = true
    setTimeout(() => (copiedCodes = false), 1500)
  }

  function download() {
    const text = `The Bakery recovery codes for ${session.member?.email}\nEach works once.\n\n${recoveryCodes!.join('\n')}\n`
    const url = URL.createObjectURL(new Blob([text], { type: 'text/plain' }))
    const a = document.createElement('a')
    a.href = url
    a.download = 'bakery-recovery-codes.txt'
    a.click()
    URL.revokeObjectURL(url)
  }

  $effect(() => breadcrumb.set({ label: 'Profile' }))
</script>

<SettingsPage icon={UserRoundPen} title="Profile">
  <!-- Paperclip's header card: a gradient band behind a frosted panel. -->
  <div class="relative max-w-2xl overflow-hidden rounded-[28px] border border-border/70 bg-card" data-testid="profile-card">
    <div
      class="absolute inset-x-0 top-0 h-32 bg-[linear-gradient(135deg,var(--primary)_0%,var(--accent)_58%,color-mix(in_oklab,var(--background)_76%,white_24%)_100%)]"
    ></div>
    <div
      class="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(255,255,255,0.22),transparent_34%),radial-gradient(circle_at_bottom_left,rgba(255,255,255,0.08),transparent_36%)]"
    ></div>
    <div class="relative p-6 pt-10">
      <div
        class="flex flex-wrap items-end gap-5 rounded-[24px] border border-border/70 bg-background/92 p-5 shadow-[0_18px_44px_-28px_rgba(0,0,0,0.45)] backdrop-blur-sm"
      >
        <Avatar.Root size="lg" class="size-24 shadow-xl ring-4 ring-background data-[size=lg]:size-24">
          <Avatar.Fallback class="text-2xl">{initials}</Avatar.Fallback>
        </Avatar.Root>
        <div class="min-w-0 flex-1 space-y-1 pb-1">
          <h2 class="truncate text-2xl font-semibold text-foreground" data-testid="profile-name">{currentName}</h2>
          <p class="truncate text-sm text-muted-foreground">{session.member?.email}</p>
          <p class="text-sm text-muted-foreground">Role: {session.roleName}</p>
        </div>
      </div>
    </div>
  </div>

  <form onsubmit={saveName}>
    <SettingsGroup id="profile-details" label="Profile details">
      {#snippet actions()}
        <Button type="submit" variant="highlighted" loading={savingName}>Save</Button>
      {/snippet}
      <div class="grid gap-4 md:grid-cols-2">
        <Input id="name" label="Name" bind:value={name} error={nameErrors.name} autocomplete="name" required />
        <Input id="email" label="Email" value={session.member?.email ?? ''} readonly disabled />
      </div>
    </SettingsGroup>
  </form>

  <form onsubmit={savePassword}>
    <SettingsGroup id="password" label="Password" hint="Changing it signs out every other browser.">
      {#snippet actions()}
        <Button type="submit" loading={savingPassword}>Change password</Button>
      {/snippet}
      <div class="grid gap-4 md:grid-cols-2">
        <div class="md:col-span-2">
          <Input
            id="current_password"
            label="Current password"
            type="password"
            bind:value={currentPassword}
            error={passwordErrors.current_password}
            autocomplete="current-password"
            required
          />
        </div>
        <Input
          id="new_password"
          label="New password"
          helper="At least 12 characters."
          type="password"
          bind:value={newPassword}
          error={passwordErrors.new_password}
          autocomplete="new-password"
          required
        />
        <Input
          id="repeat_password"
          label="Confirm new password"
          type="password"
          bind:value={repeatPassword}
          error={passwordErrors.repeat_password}
          autocomplete="new-password"
          required
        />
      </div>
    </SettingsGroup>
  </form>

  <SettingsGroup id="sessions" label="Sessions">
    <div class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border px-4 py-3">
      <p class="text-sm text-muted-foreground">Signed in somewhere you no longer trust? End every session except this one.</p>
      <ConfirmationModal
        title="Sign out everywhere else?"
        actions={['End every session except this one.']}
        warningMessage="Every other browser and device where you are signed in has to sign in again. This one stays signed in."
        confirmWithText={false}
        step2ButtonText="Sign out everywhere else"
        onconfirm={signOutOthers}
      >
        {#snippet trigger(show)}
          <Button onclick={show}>Sign out everywhere else</Button>
        {/snippet}
      </ConfirmationModal>
    </div>
  </SettingsGroup>

  <SettingsGroup id="two-factor-authentication" label="Two-factor authentication" data-testid="two-factor">
    {#if statusError}
      <p class="text-sm text-destructive">{statusError}</p>
    {:else if status === null}
      <Spinner text="Loading…" />
    {:else if recoveryCodes}
      <p class="text-sm">
        Keep these recovery codes somewhere safe. Each one signs you in once without your phone.
        <strong>The Bakery cannot show them again.</strong>
      </p>
      <ul class="grid w-fit grid-cols-2 gap-x-8 gap-y-1 rounded-md border border-border px-4 py-3 font-mono text-sm" data-testid="recovery-codes">
        {#each recoveryCodes as c (c)}<li>{c}</li>{/each}
      </ul>
      <div class="flex flex-wrap justify-end gap-2">
        <Button onclick={copyCodes}>{copiedCodes ? 'Copied' : 'Copy all'}</Button>
        <Button onclick={download}>Download</Button>
        <Button variant="highlighted" onclick={() => (recoveryCodes = null)}>I have saved them</Button>
      </div>
    {:else if setup}
      <p class="text-sm text-muted-foreground">
        Scan this code with your authenticator app, or type the key in by hand. Then enter the 6-digit code it shows.
      </p>
      <div class="flex flex-wrap items-center gap-5">
        <div class="w-44 rounded-md bg-white leading-none [&_svg]:h-auto [&_svg]:w-full" role="img" aria-label="QR code for your authenticator app">
          {@html qr}
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-2">
          <span class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Key</span>
          <code class="font-mono text-base break-all" data-testid="two-factor-secret">{groupedSecret}</code>
          <CopyButton text={setup.secret} />
        </div>
      </div>
      <form class="flex flex-wrap items-end gap-2" onsubmit={confirmSetup}>
        <div class="w-40">
          <Input
            id="two-factor-code"
            label="Code"
            bind:value={code}
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength={6}
            required
            class="tracking-[0.15em]"
          />
        </div>
        <Button type="submit" variant="highlighted" loading={busy}>Turn on</Button>
        <Button onclick={() => (setup = null)}>Cancel</Button>
      </form>
      {#if twoFactorError}<p class="text-sm text-destructive">{twoFactorError}</p>{/if}
    {:else if status.state === 'on'}
      <p class="text-sm">
        <strong>On.</strong>
        <span class={status.recovery_codes_left < 3 ? 'text-destructive' : 'text-muted-foreground'}>
          {status.recovery_codes_left}
          recovery {status.recovery_codes_left === 1 ? 'code' : 'codes'} left{status.recovery_codes_left < 3 ? '; make new ones' : ''}.
        </span>
      </p>
      {#if action}
        <form class="grid gap-4 md:grid-cols-2" onsubmit={submitAction}>
          {#if action === 'disable'}
            <Input id="two-factor-password" label="Password" type="password" bind:value={password} autocomplete="current-password" required />
          {/if}
          <Input
            id="two-factor-action-code"
            label={action === 'disable' ? 'Code from your app, or a recovery code' : 'Code from your app'}
            bind:value={code}
            autocomplete="one-time-code"
            required
          />
          <div class="flex justify-end gap-2 md:col-span-2">
            <Button onclick={() => (action = '')}>Cancel</Button>
            <Button type="submit" variant={action === 'disable' ? 'error' : 'highlighted'} loading={busy}>
              {action === 'disable' ? 'Turn off' : 'Make new codes'}
            </Button>
          </div>
        </form>
      {:else}
        <div class="flex flex-wrap justify-end gap-2">
          <Button onclick={() => (action = 'regenerate')}>New recovery codes</Button>
          <Button variant="error" onclick={() => (action = 'disable')}>Turn off</Button>
        </div>
      {/if}
      {#if twoFactorError}<p class="text-sm text-destructive">{twoFactorError}</p>{/if}
    {:else}
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border px-4 py-3">
        <p class="text-sm text-muted-foreground">
          Off. With two-factor on, signing in also asks for a code from an authenticator app on your phone.
        </p>
        <Button variant="highlighted" onclick={startSetup} loading={busy}>Set up two-factor</Button>
      </div>
      {#if twoFactorError}<p class="text-sm text-destructive">{twoFactorError}</p>{/if}
    {/if}
  </SettingsGroup>
</SettingsPage>
