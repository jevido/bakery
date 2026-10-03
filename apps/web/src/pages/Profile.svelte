<script lang="ts">
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { renderSVG } from 'uqr'
  import { api, ApiError } from '../lib/api'
  import CopyButton from '../lib/CopyButton.svelte'
  import Field from '../lib/Field.svelte'
  import { session, type Member } from '../lib/session.svelte'

  type TwoFactorStatus = { state: 'off' | 'pending' | 'on'; recovery_codes_left: number }

  // Profile
  let name = $state(session.member?.name ?? '')
  let nameErrors = $state<Record<string, string>>({})
  let nameSaved = $state(false)

  async function saveName(e: SubmitEvent) {
    e.preventDefault()
    nameErrors = {}
    nameSaved = false
    try {
      const { member } = await api<{ member: Member }>('PATCH', '/me', { name })
      session.signedIn(member)
      name = member.name
      nameSaved = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      nameErrors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    }
  }

  // Password
  let currentPassword = $state('')
  let newPassword = $state('')
  let repeatPassword = $state('')
  let passwordErrors = $state<Record<string, string>>({})
  let passwordSaved = $state(false)

  async function savePassword(e: SubmitEvent) {
    e.preventDefault()
    passwordErrors = {}
    passwordSaved = false
    if (newPassword !== repeatPassword) {
      passwordErrors = { repeat_password: 'the passwords are not the same' }
      return
    }
    try {
      const { member } = await api<{ member: Member }>('POST', '/me/password', {
        current_password: currentPassword,
        new_password: newPassword,
      })
      session.signedIn(member)
      currentPassword = newPassword = repeatPassword = ''
      passwordSaved = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      passwordErrors = Object.keys(err.errors).length ? err.errors : { new_password: err.message }
    }
  }

  // Sessions
  let signedOutOthers = $state(false)

  async function signOutOthers() {
    if (!confirm('Sign out every other browser and device where you are signed in? This one stays signed in.')) return
    await api('POST', '/me/sign-out-others')
    signedOutOthers = true
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

<h1>Profile</h1>

<h2>Profile details</h2>
<form class="card form" onsubmit={saveName}>
  <Field label="Name" bind:value={name} error={nameErrors.name} autocomplete="name" required />
  <p class="muted small">Email: {session.member?.email} · Role: {session.member?.role}</p>
  <div class="actions">
    {#if nameSaved}<span class="muted">Saved</span>{/if}
    <button class="primary">Save</button>
  </div>
</form>

<h2>Password</h2>
<form class="card form" onsubmit={savePassword}>
  <Field label="Current password" type="password" bind:value={currentPassword} error={passwordErrors.current_password} autocomplete="current-password" required />
  <Field label="New password (at least 12 characters)" type="password" bind:value={newPassword} error={passwordErrors.new_password} autocomplete="new-password" required />
  <Field label="New password again" type="password" bind:value={repeatPassword} error={passwordErrors.repeat_password} autocomplete="new-password" required />
  <div class="actions">
    {#if passwordSaved}<span class="muted">Password changed; other browsers are signed out</span>{/if}
    <button class="primary">Change password</button>
  </div>
</form>

<h2>Sessions</h2>
<div class="card form">
  <p class="muted">Signed in somewhere you no longer trust? End every session except this one.</p>
  <div class="actions">
    {#if signedOutOthers}<span class="muted">Other browsers are signed out</span>{/if}
    <button onclick={signOutOthers}>Sign out everywhere else</button>
  </div>
</div>

<h2>Two-factor authentication</h2>
<div class="card form" data-testid="two-factor">
  {#if statusError}
    <p class="error">{statusError}</p>
  {:else if status === null}
    <p class="muted">Loading…</p>
  {:else if recoveryCodes}
    <p>
      Keep these recovery codes somewhere safe. Each one signs you in once without your phone. <strong>The Bakery cannot show them again.</strong>
    </p>
    <ul class="codes mono" data-testid="recovery-codes">
      {#each recoveryCodes as c (c)}<li>{c}</li>{/each}
    </ul>
    <div class="actions">
      <CopyButton text={recoveryCodes.join('\n')} label="Copy all" />
      <button type="button" onclick={download}>Download</button>
      <button class="primary" type="button" onclick={() => (recoveryCodes = null)}>I have saved them</button>
    </div>
  {:else if setup}
    <p>Scan this code with your authenticator app, or type the key in by hand. Then enter the 6-digit code it shows.</p>
    <div class="setup">
      <div class="qr" role="img" aria-label="QR code for your authenticator app">{@html qr}</div>
      <div class="key">
        <span class="muted small">Key</span>
        <code class="mono" data-testid="two-factor-secret">{groupedSecret}</code>
        <CopyButton text={setup.secret} />
      </div>
    </div>
    <form class="inline" onsubmit={confirmSetup}>
      <label class="field">
        <span class="muted small">Code</span>
        <input bind:value={code} inputmode="numeric" autocomplete="one-time-code" maxlength="6" required aria-label="Code" />
      </label>
      <button class="primary" disabled={busy}>Turn on</button>
      <button type="button" onclick={() => (setup = null)}>Cancel</button>
    </form>
    {#if twoFactorError}<p class="error">{twoFactorError}</p>{/if}
  {:else if status.state === 'on'}
    <p>
      <strong>On.</strong>
      <span class={status.recovery_codes_left < 3 ? 'error' : 'muted'}>
        {status.recovery_codes_left} recovery {status.recovery_codes_left === 1 ? 'code' : 'codes'} left{status.recovery_codes_left < 3 ? '; make new ones' : ''}.
      </span>
    </p>
    {#if action}
      <form class="grid" onsubmit={submitAction}>
        {#if action === 'disable'}
          <Field label="Password" type="password" bind:value={password} autocomplete="current-password" required />
        {/if}
        <label class="field">
          <span class="muted small">{action === 'disable' ? 'Code from your app, or a recovery code' : 'Code from your app'}</span>
          <input bind:value={code} autocomplete="one-time-code" required aria-label="Code" />
        </label>
        <div class="actions">
          <button type="button" onclick={() => (action = '')}>Cancel</button>
          <button class={action === 'disable' ? 'danger' : 'primary'} disabled={busy}>
            {action === 'disable' ? 'Turn off' : 'Make new codes'}
          </button>
        </div>
      </form>
    {:else}
      <div class="actions">
        <button onclick={() => (action = 'regenerate')}>New recovery codes</button>
        <button class="danger" onclick={() => (action = 'disable')}>Turn off</button>
      </div>
    {/if}
    {#if twoFactorError}<p class="error">{twoFactorError}</p>{/if}
  {:else}
    <p class="muted">Off. With two-factor on, signing in also asks for a code from an authenticator app on your phone.</p>
    <div class="actions">
      <button class="primary" onclick={startSetup} disabled={busy}>Set up two-factor</button>
    </div>
    {#if twoFactorError}<p class="error">{twoFactorError}</p>{/if}
  {/if}
</div>

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 36rem;
    margin-bottom: 1.5rem;
  }
  .grid {
    display: grid;
    gap: 0.8rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.6rem;
  }
  p {
    margin: 0;
  }
  .small {
    font-size: 0.8rem;
  }
  .setup {
    display: flex;
    gap: 1.25rem;
    align-items: center;
    flex-wrap: wrap;
  }
  .qr {
    width: 11rem;
    background: #fff;
    border-radius: 6px;
    line-height: 0;
  }
  .qr :global(svg) {
    width: 100%;
    height: auto;
  }
  .key {
    display: grid;
    gap: 0.4rem;
    justify-items: start;
  }
  .key code {
    font-size: 1rem;
    word-break: break-all;
  }
  .inline {
    display: flex;
    align-items: end;
    gap: 0.6rem;
  }
  .field {
    display: grid;
    gap: 0.3rem;
  }
  .inline input {
    width: 8rem;
    letter-spacing: 0.15em;
  }
  .codes {
    display: grid;
    grid-template-columns: repeat(2, max-content);
    gap: 0.3rem 2rem;
    list-style: none;
    padding: 0;
    margin: 0;
  }
</style>
