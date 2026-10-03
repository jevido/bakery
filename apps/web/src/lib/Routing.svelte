<script lang="ts">
  import { session } from './session.svelte'
  import { api, ApiError } from './api'
  import type { RouteSettings, Redirect } from './types'

  let { applicationId, domains }: { applicationId: number; domains: string[] } = $props()

  type HeaderRow = { key: number; name: string; value: string }

  let loaded = $state(false)
  let loadError = $state('')
  let redirect = $state<Redirect>('both')
  let headers = $state<HeaderRow[]>([])
  let authEnabled = $state(false)
  let username = $state('')
  let password = $state('')
  let passwordSet = $state(false)
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let applied = $state(false)
  let busy = $state(false)
  let nextKey = 0

  // What the Redirect adds, as the proxy renders it (a counterpart that
  // is one of the Domains itself is served, not redirected).
  let counterparts = $derived.by(() => {
    const out: [string, string][] = []
    for (const d of domains) {
      let c = ''
      if (redirect === 'non-www' && !d.startsWith('www.')) c = 'www.' + d
      if (redirect === 'www' && d.startsWith('www.')) c = d.slice(4)
      if (c && c.includes('.') && !domains.includes(c)) out.push([c, d])
    }
    return out
  })

  function show(s: RouteSettings) {
    redirect = s.redirect
    headers = s.response_headers.map((h) => ({ key: nextKey++, ...h }))
    authEnabled = s.basic_auth.enabled
    username = s.basic_auth.username
    passwordSet = s.basic_auth.password_set
    password = ''
  }

  $effect(() => {
    loaded = false
    loadError = ''
    api<{ routing: RouteSettings }>('GET', `/applications/${applicationId}/routing`)
      .then((r) => {
        show(r.routing)
        loaded = true
      })
      .catch((e) => (loadError = e.message))
  })

  async function save(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    applied = false
    try {
      const r = await api<{ routing: RouteSettings }>('PUT', `/applications/${applicationId}/routing`, {
        redirect,
        response_headers: headers.filter((h) => h.name.trim() !== '').map((h) => ({ name: h.name.trim(), value: h.value })),
        basic_auth: { enabled: authEnabled, username, password },
      })
      show(r.routing)
      applied = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = err.errors
      if (Object.keys(err.errors).length === 0) message = err.message
    } finally {
      busy = false
    }
  }
</script>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if !loaded}
  <p class="muted">Loading…</p>
{:else}
  <form class="form" onsubmit={save}>
    <!-- A viewer sees the values and cannot change them. -->
    <fieldset class="contents" disabled={!session.canWrite}>
    <p class="muted">How the proxy treats this application's traffic. Saving applies it at once, without a deploy.</p>
    <fieldset>
      <legend>Redirect</legend>
      <select aria-label="Redirect" bind:value={redirect}>
        <option value="both">Allow www &amp; non-www</option>
        <option value="www">Redirect to www</option>
        <option value="non-www">Redirect to non-www</option>
      </select>
      {#if counterparts.length > 0}
        <ul class="mono preview">
          {#each counterparts as [from, to] (from)}<li>{from} → {to}</li>{/each}
        </ul>
      {:else if redirect !== 'both'}
        <p class="muted">No domain has a counterpart to redirect for this choice.</p>
      {/if}
      {#if errors.redirect}<small class="error">{errors.redirect}</small>{/if}
    </fieldset>
    <fieldset>
      <legend>Response headers</legend>
      <p class="muted">Set on every response, replacing a header of the same name the app sends.</p>
      {#each headers as h (h.key)}
        <div class="line">
          <input aria-label="Header name" bind:value={h.name} placeholder="X-Frame-Options" />
          <input aria-label="Header value" bind:value={h.value} placeholder="DENY" />
          <button type="button" aria-label="Remove header" onclick={() => (headers = headers.filter((x) => x.key !== h.key))}>×</button>
        </div>
      {/each}
      {#if errors.response_headers}<small class="error">{errors.response_headers}</small>{/if}
      {#if headers.length < 20}
        <div><button type="button" onclick={() => headers.push({ key: nextKey++, name: '', value: '' })}>Add header</button></div>
      {/if}
    </fieldset>
    <fieldset>
      <legend>Basic auth</legend>
      <label class="check">
        <input type="checkbox" bind:checked={authEnabled} />
        Ask for a username and password before any request reaches the app
      </label>
      {#if authEnabled}
        <div class="row">
          <label class="field">
            <span>Username</span>
            <input bind:value={username} autocomplete="off" aria-invalid={errors['basic_auth.username'] ? 'true' : undefined} />
            {#if errors['basic_auth.username']}<small class="error">{errors['basic_auth.username']}</small>{/if}
          </label>
          <label class="field">
            <span>Password</span>
            <input
              type="password"
              bind:value={password}
              autocomplete="new-password"
              placeholder={passwordSet ? 'unchanged' : ''}
              aria-invalid={errors['basic_auth.password'] ? 'true' : undefined}
            />
            {#if errors['basic_auth.password']}<small class="error">{errors['basic_auth.password']}</small>{/if}
          </label>
        </div>
        <p class="muted">Only a hash of the password is kept. Health checks are not affected.</p>
      {/if}
    </fieldset>
    {#if message}<p class="error">{message}</p>{/if}
    </fieldset>
    {#if session.canWrite}
    <div class="actions">
      {#if applied}<span class="ok">Applied.</span>{/if}
      <button class="primary" disabled={busy}>Save</button>
    </div>
    {/if}
  </form>
{/if}

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 40rem;
  }
  .form > .contents > p {
    margin: 0;
  }
  fieldset:not(.contents) {
    display: grid;
    gap: 0.8rem;
    border: 1px solid var(--border, #8884);
    border-radius: 0.4rem;
    padding: 0.8rem;
    margin: 0;
  }
  fieldset:not(.contents) p {
    margin: 0;
  }
  .preview {
    margin: 0;
    padding-left: 1.2rem;
    font-size: 0.85rem;
  }
  .line {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .line input {
    flex: 1;
    min-width: 0;
  }
  .row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: 0.8rem;
  }
  .field {
    display: grid;
    gap: 0.3rem;
  }
  .field span {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .check {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .actions {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    justify-content: flex-end;
  }
  .ok {
    color: var(--ok);
  }
</style>
