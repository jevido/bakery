<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import CopyButton from '../lib/CopyButton.svelte'
  import Field from '../lib/Field.svelte'
  import { session } from '../lib/session.svelte'
  import type { Invitation, Member, Role } from '../lib/types'

  const grantable: Invitation['role'][] = ['admin', 'member', 'viewer']
  const describe: Record<Role, string> = {
    owner: 'Everything, and cannot be removed',
    admin: 'Also manages servers, storage settings and members',
    member: 'Adds, changes and deploys applications, databases and services',
    viewer: 'Reads everything except secrets, changes nothing',
  }

  let members = $state.raw<Member[] | null>(null)
  let invitations = $state.raw<Invitation[]>([])
  let loadError = $state('')
  let rowError = $state<Record<number, string>>({})

  let email = $state('')
  let role = $state<Invitation['role']>('member')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)
  // The link of the Invitation just made; the API never shows it again.
  let invited = $state.raw<{ email: string; link: string } | null>(null)

  async function load() {
    const [m, i] = await Promise.all([
      api<{ members: Member[] }>('GET', '/members'),
      api<{ invitations: Invitation[] }>('GET', '/invitations'),
    ])
    members = m.members
    invitations = i.invitations
  }
  load().catch((e) => (loadError = e.message))

  /** Whether the signed-in person may change this Member: not the Owner, not themselves. */
  function manageable(m: Member): boolean {
    return m.role !== 'owner' && m.id !== session.member?.id
  }

  async function invite(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const r = await api<{ invitation: Invitation; path: string; link?: string }>('POST', '/invitations', { email, role })
      invited = { email: r.invitation.email, link: r.link ?? location.origin + r.path }
      email = ''
      await load()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { email: err.message }
    } finally {
      busy = false
    }
  }

  async function changeRole(m: Member, next: Role) {
    rowError = {}
    try {
      await api('PATCH', `/members/${m.id}`, { role: next })
    } catch (err) {
      rowError = { [m.id]: err instanceof Error ? err.message : String(err) }
    }
    await load()
  }

  async function remove(m: Member) {
    if (!confirm(`Remove ${m.name} (${m.email})? They are signed out at once and their API tokens stop working.`)) return
    rowError = {}
    try {
      await api('DELETE', `/members/${m.id}`)
    } catch (err) {
      rowError = { [m.id]: err instanceof Error ? err.message : String(err) }
    }
    await load()
  }

  async function revoke(i: Invitation) {
    if (!confirm(`Revoke the invitation of ${i.email}? Its link stops working.`)) return
    await api('DELETE', `/invitations/${i.id}`)
    await load()
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
</script>

<h1>Members</h1>

<form class="card form" onsubmit={invite}>
  <h2>Invite someone</h2>
  <div class="row">
    <Field label="Email" type="email" bind:value={email} error={errors.email} placeholder="dev@example.com" required />
    <label class="field">
      <span>Role</span>
      <select bind:value={role} aria-label="Role of the invitation">
        {#each grantable as r (r)}<option value={r}>{r}</option>{/each}
      </select>
      {#if errors.role}<small class="error">{errors.role}</small>{/if}
    </label>
  </div>
  <p class="muted small">{describe[role]}.</p>
  <div class="actions">
    <button class="primary" disabled={busy}>Invite</button>
  </div>
  {#if invited}
    <div class="invited" data-testid="invitation-link">
      <p>Send this link to <strong>{invited.email}</strong>. It works once, for 7 days.</p>
      <div class="link">
        <input readonly value={invited.link} aria-label="Invitation link" />
        <CopyButton text={invited.link} />
      </div>
    </div>
  {/if}
</form>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if members === null}
  <p class="muted">Loading…</p>
{:else}
  <table>
    <thead><tr><th>Name</th><th>Email</th><th>Role</th><th></th></tr></thead>
    <tbody>
      {#each members as m (m.id)}
        <tr data-testid="member">
          <td>{m.name}{m.id === session.member?.id ? ' (you)' : ''}</td>
          <td class="muted">{m.email}</td>
          <td>
            {#if manageable(m)}
              <select value={m.role} aria-label="Role of {m.email}" onchange={(e) => changeRole(m, e.currentTarget.value as Role)}>
                {#each grantable as r (r)}<option value={r}>{r}</option>{/each}
              </select>
            {:else}
              {m.role}
            {/if}
            {#if rowError[m.id]}<small class="error">{rowError[m.id]}</small>{/if}
          </td>
          <td>
            {#if manageable(m)}<button class="danger" onclick={() => remove(m)}>Remove</button>{/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>

  <h2>Open invitations</h2>
  {#if invitations.length === 0}
    <p class="muted">None. An invitation stays here until it is accepted, revoked or expires.</p>
  {:else}
    <table>
      <thead><tr><th>Email</th><th>Role</th><th>Expires</th><th></th></tr></thead>
      <tbody>
        {#each invitations as i (i.id)}
          <tr data-testid="invitation">
            <td>{i.email}</td>
            <td>{i.role}</td>
            <td class="muted">{when.format(new Date(i.expires_at))}</td>
            <td><button class="danger" onclick={() => revoke(i)}>Revoke</button></td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
{/if}

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 36rem;
    margin-bottom: 1.5rem;
  }
  .form h2 {
    margin: 0;
  }
  .row {
    display: grid;
    grid-template-columns: 1fr 10rem;
    gap: 0.8rem;
  }
  .field {
    display: grid;
    gap: 0.3rem;
    align-content: start;
  }
  .field span {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
  }
  .small {
    font-size: 0.8rem;
    margin: 0;
  }
  .invited p {
    margin: 0 0 0.4rem;
  }
  .link {
    display: flex;
    gap: 0.5rem;
  }
  .link input {
    flex: 1;
    font-family: var(--mono, monospace);
  }
  td small {
    display: block;
  }
</style>
