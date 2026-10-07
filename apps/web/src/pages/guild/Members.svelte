<script lang="ts">
  // The Current guild's Members: everyone reads the list, the Guild Master
  // first; admins invite, change Roles, remove, reset two-factor and see
  // the open Invitations; the Guild Master offers the Guild Master to
  // another Member and can withdraw the offer.
  import { api, ApiError } from '../../lib/api'
  import CopyButton from '../../lib/CopyButton.svelte'
  import Field from '../../lib/Field.svelte'
  import { session } from '../../lib/session.svelte'
  import type { GuildDetails, Invitation, Member, Offer, Role } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'

  const grantable: Invitation['role'][] = ['admin', 'member', 'viewer']
  const describe: Record<Role, string> = {
    owner: 'Runs the installation; admin in every guild',
    admin: 'Also manages servers, storage settings and members',
    member: 'Adds, changes and deploys applications, databases and services',
    viewer: 'Reads everything except secrets, changes nothing',
  }

  let members = $state.raw<Member[] | null>(null)
  let invitations = $state.raw<Invitation[]>([])
  // The Guild's open Transfer offer, null without one.
  let offer = $state.raw<Offer | null>(null)
  let offerError = $state('')
  let loadError = $state('')
  let rowError = $state<Record<number, string>>({})

  let email = $state('')
  let role = $state<Invitation['role']>('member')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)
  // The link of the Invitation just made; the API never shows it again.
  let invited = $state.raw<{ email: string; link: string; emailed: boolean; emailError?: string } | null>(null)

  async function load() {
    const [m, i, g] = await Promise.all([
      api<{ members: Member[] }>('GET', '/members'),
      session.can('manage_members') ? api<{ invitations: Invitation[] }>('GET', '/invitations') : { invitations: [] },
      api<{ guild: GuildDetails }>('GET', '/guilds/current'),
    ])
    members = m.members
    invitations = i.invitations
    offer = g.guild.offer
  }
  load().catch((e) => (loadError = e.message))

  /** Whether the signed-in person may change this Member: with manage_members, never for the Guild Master or the Instance admin, never for themselves. */
  function manageable(m: Member): boolean {
    return session.can('manage_members') && !m.guild_master && !m.instance_admin && m.id !== session.member?.id
  }

  async function offerGuildMaster(m: Member) {
    offerError = ''
    try {
      await api('POST', '/guilds/current/guild-master-offer', { member_id: m.id })
    } catch (err) {
      offerError = err instanceof Error ? err.message : String(err)
    }
    await load()
  }

  async function withdrawOffer() {
    offerError = ''
    try {
      await api('DELETE', '/guilds/current/guild-master-offer')
    } catch (err) {
      offerError = err instanceof Error ? err.message : String(err)
    }
    await load()
  }

  async function invite(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const r = await api<{ invitation: Invitation; path: string; link?: string; emailed: boolean; email_error?: string }>(
        'POST',
        '/invitations',
        { email, role },
      )
      invited = {
        email: r.invitation.email,
        link: r.link ?? location.origin + r.path,
        emailed: r.emailed,
        emailError: r.email_error,
      }
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

  async function resetTwoFactor(m: Member) {
    if (!confirm(`Turn off two-factor for ${m.name} (${m.email})? They are signed out, sign in with only their password, and should set it up again.`)) return
    rowError = {}
    try {
      await api('DELETE', `/members/${m.id}/two-factor`)
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

<h2>Members</h2>

{#if session.can('manage_members')}
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
      {#if invited.emailed}
        <p>Emailed the link to <strong>{invited.email}</strong>. It works once, for 7 days; you can also send it yourself.</p>
      {:else}
        {#if invited.emailError}
          <p class="error small">Emailing the link failed: {invited.emailError}</p>
        {/if}
        <p>Send this link to <strong>{invited.email}</strong>. It works once, for 7 days.</p>
      {/if}
      <div class="link">
        <input readonly value={invited.link} aria-label="Invitation link" />
        <CopyButton text={invited.link} />
      </div>
    </div>
  {/if}
</form>
{/if}

{#if loadError}
  <p class="error">{loadError}</p>
{:else if members === null}
  <p class="muted">Loading…</p>
{:else}
  {#if offer}
    <div class="card offer" data-testid="guild-master-offer">
      <p>
        {#if offer.to?.id === session.member?.id}
          You are offered the Guild Master of this guild. Nothing changes until you accept it above.
        {:else}
          The Guild Master is offered to <strong>{offer.to?.name}</strong> ({offer.to?.email}). Nothing changes until
          they accept; the offer expires on {when.format(new Date(offer.expires_at))}.
        {/if}
      </p>
      {#if session.guildMaster}<button onclick={withdrawOffer}>Withdraw</button>{/if}
    </div>
  {/if}
  {#if offerError}<p class="error">{offerError}</p>{/if}
  <table>
    <thead><tr><th>Name</th><th>Email</th><th>Role</th><th>2FA</th><th></th></tr></thead>
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
            {:else if m.guild_master}
              <span
                class="inline-flex items-center rounded-full bg-amber-400/20 px-2 py-0.5 text-xs font-semibold whitespace-nowrap text-amber-800 dark:text-amber-300"
                data-testid="guild-master">Guild Master</span
              >
              {#if m.instance_admin}<span class="muted">· Instance admin</span>{/if}
            {:else if m.instance_admin}
              Instance admin
            {:else}
              {m.role}
            {/if}
            {#if rowError[m.id]}<small class="error">{rowError[m.id]}</small>{/if}
          </td>
          <td class={m.two_factor ? '' : 'muted'}>{m.two_factor ? 'on' : 'off'}</td>
          <td>
            <div class="row-actions">
              {#if session.guildMaster && !m.guild_master && !offer}
                <ConfirmationModal
                  title="Transfer Guild Master?"
                  buttonTitle="Transfer Guild Master"
                  actions={[
                    `${m.name} is offered the Guild Master of ${session.guild?.name}.`,
                    'Nothing changes until they accept. Until then you can withdraw the offer, and it expires after 7 days.',
                    'Once they accept, they are the Guild Master and you are not. You both keep your other Roles.',
                  ]}
                  warningMessage="Only the Guild Master can transfer it, so you cannot take it back yourself once it is accepted."
                  confirmWithText={false}
                  step2ButtonText="Offer"
                  onconfirm={() => offerGuildMaster(m)}
                />
              {/if}
              {#if manageable(m)}
                {#if m.two_factor}<button onclick={() => resetTwoFactor(m)}>Reset 2FA</button>{/if}
                <button class="danger" onclick={() => remove(m)}>Remove</button>
              {/if}
            </div>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>

  {#if session.can('manage_members')}
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
{/if}

<style>
  .offer {
    display: flex;
    gap: 1rem;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
  }
  .offer p {
    margin: 0;
  }
  .row-actions {
    display: flex;
    gap: 0.4rem;
    justify-content: flex-end;
  }
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
