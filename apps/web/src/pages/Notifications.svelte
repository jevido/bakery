<script lang="ts">
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { api, ApiError } from '../lib/api'
  import Field from '../lib/Field.svelte'
  import { session } from '../lib/session.svelte'
  import type { ChannelKind, Delivery, EventKind, NotificationChannel } from '../lib/types'

  const kinds: { kind: ChannelKind; label: string }[] = [
    { kind: 'email', label: 'Email' },
    { kind: 'discord', label: 'Discord' },
    { kind: 'telegram', label: 'Telegram' },
    { kind: 'slack', label: 'Slack' },
    { kind: 'pushover', label: 'Pushover' },
    { kind: 'webhook', label: 'Webhook' },
    { kind: 'ntfy', label: 'ntfy' },
  ]
  const kindLabel = (k: ChannelKind) => kinds.find((x) => x.kind === k)?.label ?? k

  let channels = $state.raw<NotificationChannel[] | null>(null)
  let eventKinds = $state.raw<EventKind[]>([])
  let error = $state('')

  /** The channel being edited, 'new' for the add form, null when closed. */
  let editing = $state<NotificationChannel | 'new' | null>(null)
  let form = $state(blank('email'))
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  /** The outcome of each channel's latest Test, by id. */
  let tests = $state<Record<number, { ok: boolean; error?: string; busy?: boolean }>>({})
  /** Which channel's Deliveries are shown, and them. */
  let expanded = $state<number | null>(null)
  let deliveries = $state.raw<Delivery[] | null>(null)

  function blank(kind: ChannelKind) {
    return {
      name: '',
      kind,
      host: '',
      port: '587',
      security: 'starttls',
      username: '',
      password: '',
      from: '',
      to: '',
      url: '',
      bot_token: '',
      chat_id: '',
      topic: '',
      token: '',
      secret: '',
      user_key: '',
      api_token: '',
      // Not edited here; carried so a save keeps them.
      ping: false,
      thread_ids: {} as Record<string, string>,
      events: eventKinds.filter((e) => e.default).map((e) => e.kind),
    }
  }

  async function load() {
    const [c, e] = await Promise.all([
      api<{ channels: NotificationChannel[] }>('GET', '/notification-channels'),
      api<{ event_kinds: EventKind[] }>('GET', '/notification-event-kinds'),
    ])
    channels = c.channels
    eventKinds = e.event_kinds
  }
  // Everything here is for admins; the API refuses the rest.
  if (session.isAdmin) load().catch((e) => (error = e.message))

  function open(c: NotificationChannel | 'new') {
    editing = c
    errors = {}
    message = ''
    if (c === 'new') {
      form = blank('email')
      return
    }
    const s = c.settings
    form = {
      ...blank(c.kind),
      name: c.name,
      host: s.host ?? '',
      port: String(s.port ?? ''),
      security: s.security ?? 'starttls',
      username: s.username ?? '',
      from: s.from ?? '',
      to: (s.to ?? []).join(', '),
      url: c.kind === 'ntfy' ? (s.url ?? '') : '',
      chat_id: s.chat_id ?? '',
      topic: s.topic ?? '',
      ping: s.ping,
      thread_ids: { ...s.thread_ids },
      events: [...c.event_kinds],
    }
  }

  function payload() {
    const f = form
    return {
      name: f.name,
      kind: f.kind,
      event_kinds: f.events,
      settings: {
        host: f.host,
        port: Number(f.port) || 0,
        security: f.security,
        username: f.username,
        password: f.password,
        from: f.from,
        to: f.to.split(/[\s,;]+/).filter(Boolean),
        url: f.url,
        bot_token: f.bot_token,
        chat_id: f.chat_id,
        topic: f.topic,
        token: f.token,
        secret: f.secret,
        user_key: f.user_key,
        api_token: f.api_token,
        ping: f.ping,
        thread_ids: f.thread_ids,
      },
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      if (editing === 'new') await api('POST', '/notification-channels', payload())
      else if (editing) await api('PATCH', `/notification-channels/${editing.id}`, payload())
      editing = null
      await load()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = err.errors
      message = Object.keys(err.errors).length === 0 ? err.message : ''
    } finally {
      busy = false
    }
  }

  async function test(c: NotificationChannel) {
    tests[c.id] = { ok: false, busy: true }
    try {
      await api('POST', `/notification-channels/${c.id}/test`)
      tests[c.id] = { ok: true }
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      tests[c.id] = { ok: false, error: err.message }
    }
    if (expanded === c.id) await showDeliveries(c.id)
  }

  async function showDeliveries(id: number) {
    expanded = id
    deliveries = null
    const r = await api<{ deliveries: Delivery[] }>('GET', `/notification-channels/${id}/deliveries`)
    if (expanded === id) deliveries = r.deliveries
  }

  function toggle(c: NotificationChannel) {
    if (expanded === c.id) expanded = null
    else showDeliveries(c.id).catch((e) => (error = e.message))
  }

  async function remove(c: NotificationChannel) {
    if (!confirm(`Delete the notification channel ${c.name}? Nothing is sent to it any more.`)) return
    error = ''
    try {
      await api('DELETE', `/notification-channels/${c.id}`)
      if (expanded === c.id) expanded = null
      await load()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      error = err.message
    }
  }

  function where(c: NotificationChannel): string {
    const s = c.settings
    switch (c.kind) {
      case 'email':
        return (s.to ?? []).join(', ')
      case 'telegram':
        return `chat ${s.chat_id}`
      case 'ntfy':
        return `${s.url}/${s.topic}`
      default:
        return s.url_host ?? ''
    }
  }

  const label = (kind: string) => eventKinds.find((e) => e.kind === kind)?.label ?? kind
  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
  const keep = $derived(editing !== null && editing !== 'new' ? 'unchanged' : '')

  $effect(() => breadcrumb.set({ label: 'Notifications' }))
</script>

<h1>Notifications</h1>

{#if !session.isAdmin}
  <p class="muted">Notifications are managed by admins.</p>
{:else}
  <p class="muted">
    Where The Bakery tells you that a deployment or backup failed, a server became unreachable or came back, or a disk is almost
    full. The first email channel also sends invitations to the people you invite.
  </p>
  {#if error}<p class="error">{error}</p>{/if}
  {#if channels === null}
    {#if !error}<p class="muted">Loading…</p>{/if}
  {:else if channels.length === 0}
    <p class="muted">No channels yet.</p>
  {:else}
    <table>
      <thead><tr><th>Name</th><th>Kind</th><th>Sends to</th><th>Events</th><th></th></tr></thead>
      <tbody>
        {#each channels as c (c.id)}
          <tr data-testid="channel">
            <td>{c.name}</td>
            <td>{kindLabel(c.kind)}</td>
            <td class="mono small">{where(c)}</td>
            <td>
              <div class="tags">
                {#each c.event_kinds as k (k)}<span class="tag">{label(k)}</span>{/each}
              </div>
            </td>
            <td>
              <div class="buttons">
                <button onclick={() => test(c)} disabled={tests[c.id]?.busy}>Test</button>
                <button onclick={() => toggle(c)} aria-expanded={expanded === c.id}>Deliveries</button>
                <button onclick={() => open(c)}>Edit</button>
                <button class="danger" onclick={() => remove(c)}>Delete</button>
              </div>
            </td>
          </tr>
          {#if tests[c.id] && !tests[c.id].busy}
            <tr class="sub">
              <td colspan="5">
                <p class={tests[c.id].ok ? 'ok' : 'error'} role="status">
                  {tests[c.id].ok ? 'Test notification sent.' : `Test failed: ${tests[c.id].error}`}
                </p>
              </td>
            </tr>
          {/if}
          {#if expanded === c.id}
            <tr class="sub">
              <td colspan="5">
                {#if deliveries === null}
                  <p class="muted">Loading…</p>
                {:else if deliveries.length === 0}
                  <p class="muted">Nothing sent yet.</p>
                {:else}
                  <table class="deliveries">
                    <thead><tr><th>When</th><th>Event</th><th>Title</th><th>Status</th><th>Attempts</th></tr></thead>
                    <tbody>
                      {#each deliveries as d (d.id)}
                        <tr>
                          <td class="muted">{when.format(new Date(d.created_at))}</td>
                          <td>{label(d.event_kind)}</td>
                          <td>
                            {d.title}
                            {#if d.last_error}<div class="error small">{d.last_error}</div>{/if}
                          </td>
                          <td><span class={['status', d.status]}>{d.status}</span></td>
                          <td>{d.attempts}</td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                {/if}
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  {/if}

  {#if editing === null}
    <p><button onclick={() => open('new')}>Add channel</button></p>
  {:else}
    <form class="form" onsubmit={save}>
      <h3>{editing === 'new' ? 'Add channel' : `Edit ${editing.name}`}</h3>
      {#if editing === 'new'}
        <label class="field">
          <span>Kind</span>
          <select bind:value={form.kind}>
            {#each kinds as k (k.kind)}<option value={k.kind}>{k.label}</option>{/each}
          </select>
        </label>
      {/if}
      <Field label="Name" bind:value={form.name} error={errors.name} required />

      {#if form.kind === 'email'}
        <div class="row">
          <Field label="SMTP server" bind:value={form.host} error={errors.host} placeholder="smtp.example.com" required />
          <Field label="Port" type="number" bind:value={form.port} error={errors.port} required />
          <label class="field">
            <span>Security</span>
            <select bind:value={form.security}>
              <option value="starttls">STARTTLS</option>
              <option value="tls">TLS</option>
              <option value="none">None</option>
            </select>
          </label>
        </div>
        <div class="row">
          <Field label="Username (optional)" bind:value={form.username} error={errors.username} autocomplete="off" />
          <Field label="Password" type="password" bind:value={form.password} error={errors.password} autocomplete="new-password" placeholder={keep} />
        </div>
        <Field label="From" type="email" bind:value={form.from} error={errors.from} placeholder="bakery@example.com" required />
        <Field label="To (comma-separated)" bind:value={form.to} error={errors.to} placeholder="ops@example.com" required />
      {:else if form.kind === 'discord' || form.kind === 'slack' || form.kind === 'webhook'}
        <Field
          label={form.kind === 'webhook' ? 'URL' : 'Incoming webhook URL'}
          bind:value={form.url}
          error={errors.url}
          placeholder={keep || (form.kind === 'discord' ? 'https://discord.com/api/webhooks/…' : form.kind === 'slack' ? 'https://hooks.slack.com/services/…' : 'https://example.com/hook')}
          autocomplete="off"
          required={editing === 'new'}
        />
        {#if form.kind === 'webhook'}
          <Field label="Secret (optional, signs each request)" type="password" bind:value={form.secret} error={errors.secret} autocomplete="new-password" placeholder={keep} />
          <p class="muted small">The Bakery POSTs JSON; with a secret, the header <span class="mono">X-Bakery-Signature: sha256=…</span> is the HMAC-SHA256 of the body.</p>
        {/if}
      {:else if form.kind === 'telegram'}
        <Field label="Bot token" type="password" bind:value={form.bot_token} error={errors.bot_token} autocomplete="new-password" placeholder={keep || '123456:ABC-DEF…'} required={editing === 'new'} />
        <Field label="Chat id" bind:value={form.chat_id} error={errors.chat_id} placeholder="-1001234567890 or @channel" required />
      {:else if form.kind === 'pushover'}
        <Field label="User key" type="password" bind:value={form.user_key} error={errors.user_key} autocomplete="new-password" placeholder={keep} required={editing === 'new'} />
        <Field label="API token" type="password" bind:value={form.api_token} error={errors.api_token} autocomplete="new-password" placeholder={keep} required={editing === 'new'} />
      {:else if form.kind === 'ntfy'}
        <div class="row">
          <Field label="Server" bind:value={form.url} error={errors.url} placeholder="https://ntfy.sh" />
          <Field label="Topic" bind:value={form.topic} error={errors.topic} required />
        </div>
        <Field label="Access token (optional)" type="password" bind:value={form.token} error={errors.token} autocomplete="new-password" placeholder={keep} />
      {/if}

      <fieldset>
        <legend>Events</legend>
        {#each eventKinds as e (e.kind)}
          <label class="check"><input type="checkbox" value={e.kind} bind:group={form.events} /> {e.label}</label>
        {/each}
        {#if errors.event_kinds}<small class="error">{errors.event_kinds}</small>{/if}
      </fieldset>

      {#if message}<p class="error">{message}</p>{/if}
      <div class="actions">
        <span class="spacer"></span>
        <button type="button" onclick={() => (editing = null)}>Cancel</button>
        <button class="primary" disabled={busy}>Save</button>
      </div>
    </form>
  {/if}
{/if}

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 40rem;
    margin-top: 1rem;
  }
  h3,
  p {
    margin: 0;
  }
  p + p,
  p + table,
  table + p,
  h1 + p {
    margin-top: 0.8rem;
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
  fieldset {
    display: grid;
    gap: 0.3rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.6rem 0.8rem;
  }
  .check {
    display: flex;
    gap: 0.4rem;
    align-items: center;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .spacer {
    flex: 1;
  }
  .buttons {
    display: flex;
    gap: 0.4rem;
    justify-content: flex-end;
  }
  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
  }
  .tag {
    font-size: 0.75rem;
    padding: 0.05rem 0.4rem;
    border-radius: 999px;
    background: var(--hover);
  }
  .small {
    font-size: 0.8rem;
  }
  .sub td {
    background: var(--panel);
  }
  .deliveries {
    width: 100%;
  }
  .status.sent,
  .ok {
    color: var(--ok);
  }
  .status.failed {
    color: var(--danger);
  }
</style>
