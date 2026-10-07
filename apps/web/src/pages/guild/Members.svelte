<script lang="ts">
  // The Current guild's Members: everyone reads the list, the Guild Master
  // first, each with the Roles they hold as pills. With manage_roles a
  // Member gives and takes Roles below their own highest, from people whose
  // highest Role is below theirs; with manage_members they invite (with
  // Roles), remove, reset two-factor and see the open Invitations. The
  // Guild Master offers the Guild Master to another Member and can withdraw
  // the offer.
  import { api, ApiError } from '../../lib/api'
  import CopyButton from '../../lib/CopyButton.svelte'
  import Field from '../../lib/Field.svelte'
  import { session } from '../../lib/session.svelte'
  import { canAssign, canManage } from '../../lib/hierarchy'
  import Icon from '../../lib/Icon.svelte'
  import type { GuildDetails, GuildRole, Invitation, Member, Offer, RoleRef } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'

  let members = $state.raw<Member[] | null>(null)
  let roles = $state.raw<GuildRole[]>([])
  // The Member whose Role picker is open.
  let picking = $state<number | null>(null)
  let invitations = $state.raw<Invitation[]>([])
  // The Guild's open Transfer offer, null without one.
  let offer = $state.raw<Offer | null>(null)
  let offerError = $state('')
  let loadError = $state('')
  let rowError = $state<Record<number, string>>({})

  let email = $state('')
  // The Roles the Invitation gives; Member by default once the Roles load.
  let inviteRoles = $state<number[] | null>(null)
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)
  // The link of the Invitation just made; the API never shows it again.
  let invited = $state.raw<{ email: string; link: string; emailed: boolean; emailError?: string } | null>(null)

  async function load() {
    const [m, i, g, r] = await Promise.all([
      api<{ members: Member[] }>('GET', '/members'),
      session.can('manage_members') ? api<{ invitations: Invitation[] }>('GET', '/invitations') : { invitations: [] },
      api<{ guild: GuildDetails }>('GET', '/guilds/current'),
      api<{ roles: GuildRole[] }>('GET', '/roles'),
    ])
    roles = r.roles
    inviteRoles ??= roles.filter((x) => !x.base && x.name === 'Member' && canAssign(x, roles)).map((x) => x.id)
    members = m.members
    invitations = i.invitations
    offer = g.guild.offer
  }
  load().catch((e) => (loadError = e.message))

  /** Whether the signed-in person may remove this Member or reset their two-factor. */
  function manageable(m: Member): boolean {
    return canManage(m, 'manage_members', roles)
  }

  /** Whether the signed-in person may give or take Roles of this Member at all. */
  function reRolable(m: Member): boolean {
    return canManage(m, 'manage_roles', roles)
  }

  /** The Roles the signed-in person may give, top first. */
  const assignable = $derived(roles.filter((r) => canAssign(r, roles)))

  function roleOf(ref: RoleRef): GuildRole | undefined {
    return roles.find((r) => r.id === ref.id)
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
        { email, role_ids: inviteRoles ?? [] },
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

  async function reRole(m: Member, roleID: number, give: boolean) {
    rowError = {}
    picking = null
    try {
      await api(give ? 'PUT' : 'DELETE', `/members/${m.id}/roles/${roleID}`)
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

  function toggleInviteRole(id: number, on: boolean) {
    const now = inviteRoles ?? []
    inviteRoles = on ? [...now, id] : now.filter((x) => x !== id)
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
</script>

<h2>Members</h2>

{#if session.can('manage_members')}
<form class="card form" onsubmit={invite}>
  <h2>Invite someone</h2>
  <div class="row">
    <Field label="Email" type="email" bind:value={email} error={errors.email} placeholder="dev@example.com" required />
  </div>
  <fieldset class="invite-roles" aria-label="Roles of the invitation">
    <legend>Roles</legend>
    {#each assignable as r (r.id)}
      <label class="role-choice" data-testid="invite-role" data-role={r.name}>
        <input type="checkbox" checked={inviteRoles?.includes(r.id) ?? false} onchange={(e) => toggleInviteRole(r.id, e.currentTarget.checked)} />
        <span class="dot" style:background-color={r.color}></span>{r.name}
      </label>
    {:else}
      <p class="muted small">You can give no roles; they join with only @everyone.</p>
    {/each}
    {#if errors.role_ids || errors.role}<small class="error">{errors.role_ids ?? errors.role}</small>{/if}
  </fieldset>
  <p class="muted small">Everyone also holds @everyone. You can only give roles below your own highest role.</p>
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
    <thead><tr><th>Name</th><th>Email</th><th>Roles</th><th>2FA</th><th></th></tr></thead>
    <tbody>
      {#each members as m (m.id)}
        <tr data-testid="member">
          <td>{m.name}{m.id === session.member?.id ? ' (you)' : ''}</td>
          <td class="muted">{m.email}</td>
          <td>
            {#if m.guild_master}
              <span
                class="inline-flex items-center rounded-full bg-amber-400/20 px-2 py-0.5 text-xs font-semibold whitespace-nowrap text-amber-800 dark:text-amber-300"
                data-testid="guild-master">Guild Master</span
              >
              {#if m.instance_admin}<span class="muted">· Instance admin</span>{/if}
            {:else if m.instance_admin}
              <span class="muted">Instance admin</span>
            {/if}
            <div class="pills" data-testid="member-roles">
              {#each m.roles ?? [] as ref (ref.id)}
                {@const r = roleOf(ref)}
                <span class="pill" data-testid="role-pill" data-role={ref.name}>
                  <span class="dot" style:background-color={ref.color}></span>{ref.name}
                  {#if r && reRolable(m) && canAssign(r, roles)}
                    <button class="pill-x" aria-label="Remove {ref.name} from {m.email}" onclick={() => reRole(m, ref.id, false)}>
                      <Icon name="x" class="size-3" />
                    </button>
                  {/if}
                </span>
              {/each}
              {#if reRolable(m)}
                {@const left = assignable.filter((r) => !m.roles?.some((h) => h.id === r.id))}
                {#if left.length}
                  <span class="picker">
                    <button class="pill add" aria-label="Give {m.email} a role" aria-expanded={picking === m.id} onclick={() => (picking = picking === m.id ? null : m.id)}>
                      <Icon name="plus" class="size-3" />
                    </button>
                    {#if picking === m.id}
                      <span class="menu" role="menu">
                        {#each left as r (r.id)}
                          <button role="menuitem" onclick={() => reRole(m, r.id, true)} data-testid="role-option" data-role={r.name}>
                            <span class="dot" style:background-color={r.color}></span>{r.name}
                          </button>
                        {/each}
                      </span>
                    {/if}
                  </span>
                {/if}
              {/if}
            </div>
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
      <thead><tr><th>Email</th><th>Roles</th><th>Expires</th><th></th></tr></thead>
      <tbody>
        {#each invitations as i (i.id)}
          <tr data-testid="invitation">
            <td>{i.email}</td>
            <td>
              <div class="pills">
                {#each i.roles as ref (ref.id)}<span class="pill"><span class="dot" style:background-color={ref.color}></span>{ref.name}</span>{:else}<span class="muted">@everyone</span>{/each}
              </div>
            </td>
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
    gap: 0.8rem;
  }
  .invite-roles {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    border: 0;
    padding: 0;
    margin: 0;
  }
  .invite-roles legend {
    font-size: 0.8rem;
    color: var(--muted);
    margin-bottom: 0.3rem;
  }
  .role-choice {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.85rem;
  }
  .pills {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    align-items: center;
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    border: 1px solid var(--border, rgb(0 0 0 / 0.12));
    border-radius: 999px;
    padding: 0.05rem 0.5rem;
    font-size: 0.75rem;
    white-space: nowrap;
  }
  .pill.add {
    padding: 0.2rem;
    background: transparent;
  }
  .pill-x {
    display: inline-flex;
    padding: 0;
    background: transparent;
    border: 0;
    cursor: pointer;
  }
  .dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 999px;
    display: inline-block;
  }
  .picker {
    position: relative;
  }
  .menu {
    position: absolute;
    z-index: 10;
    top: 100%;
    left: 0;
    margin-top: 0.25rem;
    display: grid;
    min-width: 10rem;
    padding: 0.25rem;
    border: 1px solid var(--border, rgb(0 0 0 / 0.12));
    border-radius: 0.4rem;
    background: var(--bg, white);
    box-shadow: 0 4px 12px rgb(0 0 0 / 0.15);
  }
  .menu button {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    text-align: left;
    background: transparent;
    border: 0;
    padding: 0.3rem 0.5rem;
    font-size: 0.85rem;
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
