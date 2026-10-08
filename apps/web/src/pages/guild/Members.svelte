<script lang="ts">
  // The Current guild's Members, in Paperclip's CompanyAccess (ui/src/pages/
  // CompanyAccess.tsx) and InvitesSection (ui/src/components/access/
  // InvitesSection.tsx; MIT, see NOTICE): a Members and an Invites tab.
  // Everyone reads the list, the Guild Master first, each with the Roles they
  // hold as pills. With manage_roles a Member gives and takes Roles below
  // their own highest, from people whose highest Role is below theirs; with
  // manage_members they invite (with Roles), remove, reset two-factor and see
  // the open Invitations. The Guild Master offers the Guild Master to another
  // Member and can withdraw the offer. Paperclip picks one role per invite
  // with radios; a Guild's Invitation gives any set of Roles, so checkboxes.
  import { Check, Crown, EllipsisVertical, Plus, ShieldCheck, X } from '@lucide/svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import * as Avatar from '@bakery/ui/components/ui/avatar'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Button, buttonVariants } from '@bakery/ui/components/ui/button'
  import { Checkbox } from '@bakery/ui/components/ui/checkbox'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import { api, ApiError } from '../../lib/api'
  import { canAssign, canManage } from '../../lib/hierarchy'
  import { session } from '../../lib/session.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
  import type { GuildDetails, GuildRole, Invitation, Member, Offer, RoleRef } from '../../lib/types'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Input from '../../lib/ui/Input.svelte'

  let members = $state.raw<Member[] | null>(null)
  let roles = $state.raw<GuildRole[]>([])
  let invitations = $state.raw<Invitation[]>([])
  // The Guild's open Transfer offer, null without one.
  let offer = $state.raw<Offer | null>(null)
  let offerError = $state('')
  let loadError = $state('')
  let rowError = $state<Record<number, string>>({})
  let tab = $state('members')

  let email = $state('')
  // The Roles the Invitation gives; Member by default once the Roles load.
  let inviteRoles = $state<number[] | null>(null)
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)
  // The link of the Invitation just made; the API never shows it again.
  let invited = $state.raw<{ email: string; link: string; emailed: boolean; emailError?: string } | null>(null)

  // The question the alert dialog asks before a row action; null when closed.
  let asking = $state.raw<{ title: string; body: string; points?: string[]; action: string; destructive?: boolean; run: () => Promise<void> } | null>(null)

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

  function initials(m: Member): string {
    const words = m.name.trim().split(/\s+/).filter(Boolean)
    return (words.length > 1 ? words[0][0] + words[words.length - 1][0] : (words[0] ?? m.email).slice(0, 2)).toUpperCase()
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
    try {
      await api(give ? 'PUT' : 'DELETE', `/members/${m.id}/roles/${roleID}`)
    } catch (err) {
      rowError = { [m.id]: err instanceof Error ? err.message : String(err) }
    }
    await load()
  }

  async function rowAction(m: Member, method: 'DELETE', path: string) {
    rowError = {}
    try {
      await api(method, path)
    } catch (err) {
      rowError = { [m.id]: err instanceof Error ? err.message : String(err) }
    }
    await load()
  }

  function remove(m: Member) {
    asking = {
      title: `Remove ${m.name}?`,
      body: `${m.name} (${m.email}) is signed out at once and their API tokens stop working.`,
      action: 'Remove',
      destructive: true,
      run: () => rowAction(m, 'DELETE', `/members/${m.id}`),
    }
  }

  function resetTwoFactor(m: Member) {
    asking = {
      title: `Turn off two-factor for ${m.name}?`,
      body: `${m.name} (${m.email}) is signed out, signs in with only their password, and should set it up again.`,
      action: 'Reset 2FA',
      run: () => rowAction(m, 'DELETE', `/members/${m.id}/two-factor`),
    }
  }

  function transfer(m: Member) {
    asking = {
      title: 'Transfer Guild Master?',
      body: 'Only the Guild Master can transfer it, so you cannot take it back yourself once it is accepted.',
      points: [
        `${m.name} is offered the Guild Master of ${session.guild?.name}.`,
        'Nothing changes until they accept. Until then you can withdraw the offer, and it expires after 7 days.',
        'Once they accept, they are the Guild Master and you are not. You both keep your other Roles.',
      ],
      action: 'Offer',
      run: () => offerGuildMaster(m),
    }
  }

  function revoke(i: Invitation) {
    asking = {
      title: `Revoke the invitation of ${i.email}?`,
      body: 'Its link stops working.',
      action: 'Revoke',
      destructive: true,
      run: async () => {
        await api('DELETE', `/invitations/${i.id}`)
        await load()
      },
    }
  }

  function toggleInviteRole(id: number, on: boolean) {
    const now = inviteRoles ?? []
    inviteRoles = on ? [...now, id] : now.filter((x) => x !== id)
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
</script>

{#snippet roleBadge(ref: RoleRef)}
  <span class="inline-block size-2 shrink-0 rounded-full" style:background-color={ref.color}></span>{ref.name}
{/snippet}

<SettingsPage icon={ShieldCheck} title="Guild Members">
  {#snippet actions()}
    {#if session.can('manage_members') && tab !== 'invites'}
      <Button onclick={() => (tab = 'invites')}><Plus />Invite people</Button>
    {/if}
  {/snippet}

  {#if loadError}
    <p class="text-sm text-destructive">{loadError}</p>
  {:else if members === null}
    <p class="text-sm text-muted-foreground">Loading guild members…</p>
  {:else}
    <Tabs.Root bind:value={tab} class="flex flex-col gap-4">
      {#if session.can('manage_members')}
        <Tabs.List variant="line" class="justify-start">
          <Tabs.Trigger value="members">Members</Tabs.Trigger>
          <Tabs.Trigger value="invites" data-testid="invites-tab">Invites</Tabs.Trigger>
        </Tabs.List>
      {/if}

      <Tabs.Content value="members" class="space-y-4">
        {#if offer}
          <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-amber-500/10 px-4 py-3 text-sm text-amber-800 dark:text-amber-200" data-testid="guild-master-offer">
            <p>
              {#if offer.to?.id === session.member?.id}
                You are offered the Guild Master of this guild. Nothing changes until you accept it above.
              {:else}
                The Guild Master is offered to <strong class="font-medium">{offer.to?.name}</strong> ({offer.to?.email}). Nothing changes
                until they accept; the offer expires on {when.format(new Date(offer.expires_at))}.
              {/if}
            </p>
            {#if session.guildMaster}<Button size="sm" variant="outline" onclick={withdrawOffer}>Withdraw</Button>{/if}
          </div>
        {/if}
        {#if offerError}<p class="text-sm text-destructive">{offerError}</p>{/if}

        <div class="overflow-x-auto">
          <table class="w-full min-w-(--sz-44rem) text-left text-sm">
            <thead>
              <tr class="border-b border-border text-muted-foreground">
                <th class="px-3 py-2 font-medium">Name</th>
                <th class="px-3 py-2 font-medium">Email</th>
                <th class="px-3 py-2 font-medium">Roles</th>
                <th class="px-3 py-2 font-medium">2FA</th>
                <th class="px-3 py-2 text-right font-medium">Action</th>
              </tr>
            </thead>
            <tbody>
              {#each members as m (m.id)}
                {@const left = reRolable(m) ? assignable.filter((r) => !m.roles?.some((h) => h.id === r.id)) : []}
                {@const transferable = session.guildMaster && !m.guild_master && !offer}
                <tr class="border-b border-border last:border-b-0" data-testid="member">
                  <td class="px-3 py-3">
                    <div class="flex min-w-0 items-center gap-2.5">
                      <Avatar.Root size="sm"><Avatar.Fallback>{initials(m)}</Avatar.Fallback></Avatar.Root>
                      <span class="truncate font-medium">{m.name}{m.id === session.member?.id ? ' (you)' : ''}</span>
                    </div>
                  </td>
                  <td class="px-3 py-3 text-muted-foreground">{m.email}</td>
                  <td class="px-3 py-3">
                    <div class="flex flex-wrap items-center gap-1.5" data-testid="member-roles">
                      {#if m.guild_master}
                        <Badge class="gap-1 bg-amber-400/20 text-amber-800 dark:text-amber-300" data-testid="guild-master"><Crown />Guild Master</Badge>
                      {/if}
                      {#if m.instance_admin}<Badge variant="secondary">Instance admin</Badge>{/if}
                      {#each m.roles ?? [] as ref (ref.id)}
                        {@const r = roleOf(ref)}
                        <Badge variant="outline" class="gap-1.5" data-testid="role-pill" data-role={ref.name}>
                          {@render roleBadge(ref)}
                          {#if r && reRolable(m) && canAssign(r, roles)}
                            <button
                              type="button"
                              class="-mr-1 rounded-sm text-muted-foreground hover:text-foreground"
                              aria-label="Remove {ref.name} from {m.email}"
                              onclick={() => reRole(m, ref.id, false)}><X class="size-3" /></button
                            >
                          {/if}
                        </Badge>
                      {/each}
                      {#if left.length}
                        <DropdownMenu.Root>
                          <DropdownMenu.Trigger
                            class="inline-flex size-5 items-center justify-center rounded-full border border-dashed border-border text-muted-foreground hover:bg-accent hover:text-foreground"
                            aria-label="Give {m.email} a role"
                          >
                            <Plus class="size-3" />
                          </DropdownMenu.Trigger>
                          <DropdownMenu.Content align="start" class="min-w-40">
                            <DropdownMenu.Label>Give a role</DropdownMenu.Label>
                            {#each left as r (r.id)}
                              <DropdownMenu.Item onSelect={() => reRole(m, r.id, true)} data-testid="role-option" data-role={r.name}>
                                {@render roleBadge(r)}
                              </DropdownMenu.Item>
                            {/each}
                          </DropdownMenu.Content>
                        </DropdownMenu.Root>
                      {/if}
                    </div>
                    {#if rowError[m.id]}<p class="mt-1 text-xs text-destructive">{rowError[m.id]}</p>{/if}
                  </td>
                  <td class="px-3 py-3">
                    <Badge variant={m.two_factor ? 'secondary' : 'outline'} class={m.two_factor ? '' : 'text-muted-foreground'}>
                      {#if m.two_factor}<Check />{/if}{m.two_factor ? 'on' : 'off'}
                    </Badge>
                  </td>
                  <td class="px-3 py-3 text-right">
                    {#if transferable || manageable(m)}
                      <DropdownMenu.Root>
                        <DropdownMenu.Trigger class={buttonVariants({ variant: 'ghost', size: 'icon-sm' })} aria-label="Actions for {m.email}" data-testid="member-actions">
                          <EllipsisVertical />
                        </DropdownMenu.Trigger>
                        <DropdownMenu.Content align="end" class="min-w-44">
                          {#if transferable}
                            <DropdownMenu.Item onSelect={() => transfer(m)}><Crown />Transfer Guild Master</DropdownMenu.Item>
                          {/if}
                          {#if manageable(m)}
                            {#if m.two_factor}
                              <DropdownMenu.Item onSelect={() => resetTwoFactor(m)}>Reset 2FA</DropdownMenu.Item>
                            {/if}
                            <DropdownMenu.Item variant="destructive" onSelect={() => remove(m)}>Remove</DropdownMenu.Item>
                          {/if}
                        </DropdownMenu.Content>
                      </DropdownMenu.Root>
                    {/if}
                  </td>
                </tr>
              {:else}
                <tr><td colspan="5" class="px-3 py-8 text-muted-foreground">No members in this guild yet.</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      </Tabs.Content>

      {#if session.can('manage_members')}
        <Tabs.Content value="invites" class="space-y-8">
          <p class="max-w-3xl text-sm text-muted-foreground">
            Invite people to join this guild. Each link works once, for 7 days, and gives the roles you pick.
          </p>

          <form class="space-y-4 rounded-xl border border-border p-5" onsubmit={invite}>
            <div class="space-y-1">
              <h2 class="text-sm font-semibold">Invite a person</h2>
              <p class="text-sm text-muted-foreground">Send an invitation link and choose the roles it gives.</p>
            </div>
            <div class="max-w-md">
              <Input label="Email" type="email" bind:value={email} error={errors.email} placeholder="dev@example.com" required />
            </div>
            <fieldset class="space-y-3" aria-label="Roles of the invitation">
              <legend class="text-sm font-medium">Choose roles</legend>
              <div class="rounded-xl border border-border">
                {#each assignable as r, i (r.id)}
                  <label class={['flex cursor-pointer items-center gap-3 px-4 py-3', i > 0 && 'border-t border-border']} data-testid="invite-role" data-role={r.name}>
                    <Checkbox checked={inviteRoles?.includes(r.id) ?? false} onCheckedChange={(on) => toggleInviteRole(r.id, on)} />
                    <span class="inline-flex items-center gap-2 text-sm font-medium">
                      <span class="inline-block size-2.5 rounded-full" style:background-color={r.color}></span>{r.name}
                    </span>
                    {#if r.name === 'Member'}<Badge variant="outline" class="text-muted-foreground">Default</Badge>{/if}
                  </label>
                {:else}
                  <p class="px-4 py-3 text-sm text-muted-foreground">You can give no roles; they join with only @everyone.</p>
                {/each}
              </div>
              {#if errors.role_ids || errors.role}<p class="text-xs text-destructive">{errors.role_ids ?? errors.role}</p>{/if}
            </fieldset>
            <div class="rounded-lg border border-border px-4 py-3 text-sm text-muted-foreground">
              Everyone also holds @everyone. You can only give roles below your own highest role.
            </div>
            <div class="flex flex-wrap items-center gap-3">
              <Button type="submit" disabled={busy}>{busy ? 'Inviting…' : 'Invite'}</Button>
              <span class="text-sm text-muted-foreground">Open invitations are listed below until accepted, revoked or expired.</span>
            </div>
            {#if invited}
              <div class="space-y-3 rounded-lg border border-border px-4 py-4" data-testid="invitation-link">
                <div class="space-y-1">
                  <div class="text-sm font-medium">Latest invite link</div>
                  <div class="text-sm text-muted-foreground">
                    {#if invited.emailed}
                      Emailed the link to <strong class="font-medium text-foreground">{invited.email}</strong>. It works once, for 7 days; you can also send it yourself.
                    {:else}
                      {#if invited.emailError}<span class="block text-destructive">Emailing the link failed: {invited.emailError}</span>{/if}
                      Send this link to <strong class="font-medium text-foreground">{invited.email}</strong>. It works once, for 7 days.
                    {/if}
                  </div>
                </div>
                <CopyButton text={invited.link} label="Invitation link" />
              </div>
            {/if}
          </form>

          <section class="rounded-xl border border-border">
            <div class="space-y-1 px-5 py-4">
              <h2 class="text-sm font-semibold">Open invitations</h2>
              <p class="text-sm text-muted-foreground">An invitation stays here until it is accepted, revoked or expires.</p>
            </div>
            {#if invitations.length === 0}
              <div class="border-t border-border px-5 py-8 text-sm text-muted-foreground">None.</div>
            {:else}
              <div class="overflow-x-auto border-t border-border">
                <table class="min-w-full text-left text-sm">
                  <thead>
                    <tr class="border-b border-border">
                      <th class="px-5 py-3 font-medium text-muted-foreground">For</th>
                      <th class="px-5 py-3 font-medium text-muted-foreground">Roles</th>
                      <th class="px-5 py-3 font-medium text-muted-foreground">Expires</th>
                      <th class="px-5 py-3 text-right font-medium text-muted-foreground">Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each invitations as i (i.id)}
                      <tr class="border-b border-border last:border-b-0" data-testid="invitation">
                        <td class="px-5 py-3 align-top">{i.email}</td>
                        <td class="px-5 py-3 align-top">
                          <div class="flex flex-wrap gap-1.5">
                            {#each i.roles as ref (ref.id)}
                              <Badge variant="outline" class="gap-1.5">{@render roleBadge(ref)}</Badge>
                            {:else}
                              <span class="text-muted-foreground">@everyone</span>
                            {/each}
                          </div>
                        </td>
                        <td class="px-5 py-3 align-top text-muted-foreground">{when.format(new Date(i.expires_at))}</td>
                        <td class="px-5 py-3 text-right align-top">
                          <Button size="sm" variant="outline" onclick={() => revoke(i)}>Revoke</Button>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          </section>
        </Tabs.Content>
      {/if}
    </Tabs.Root>
  {/if}
</SettingsPage>

<AlertDialog.Root open={asking !== null} onOpenChange={(open) => !open && (asking = null)}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{asking?.title}</AlertDialog.Title>
      {#if asking?.points}
        <ul class="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
          {#each asking.points as point (point)}<li>{point}</li>{/each}
        </ul>
      {/if}
      <AlertDialog.Description>{asking?.body}</AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action
        class={buttonVariants({ variant: asking?.destructive ? 'destructive' : 'default' })}
        onclick={() => {
          const run = asking?.run
          asking = null
          run?.()
        }}
        data-testid="confirm-action">{asking?.action}</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
