<script lang="ts">
  // A Project's Permission overrides, as Discord's channel permission
  // screen: the Roles (@everyone always first) and Members with an override
  // on the left, and for the one picked, each Permission a Project can
  // override with deny / inherit / allow on the right. Only a Role below
  // one's own highest (or @everyone) and a Member ranked below one are
  // changed, and only Permissions one holds in this Project are switched
  // (the server's SetOverride and CanGrant). In Paperclip's settings page
  // frame and groups, the targets as its EntityRows and each setting as a
  // segmented button group (ui/src/pages/CompanySettings.tsx; MIT, see NOTICE).
  import { Check, Lock, Plus, Slash, X } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import { untrack } from 'svelte'
  import * as Avatar from '$lib/components/ui/avatar'
  import { buttonVariants } from '$lib/components/ui/button'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { canEditRole, canManage } from '../lib/hierarchy'
  import EntityRow from '../lib/EntityRow.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import { href } from '../lib/router.svelte'
  import type { Member, Permission } from '../lib/session.svelte'
  import type { GuildRole, PermissionInfo, Project } from '../lib/types'
  import Button from '../lib/ui/Button.svelte'
  import Callout from '../lib/ui/Callout.svelte'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import SettingsGroup from '../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../lib/settings/SettingsPage.svelte'
  import { toast } from '../lib/ui/toast.svelte'
  import UnsavedBar from '../lib/ui/UnsavedBar.svelte'

  let { id }: { id: number } = $props()

  type Override = { role_id: number | null; member_id: number | null; allow: Permission[]; deny: Permission[] }
  type Setting = 'deny' | 'inherit' | 'allow'
  /** Who an override is for: "role:{id}" or "member:{id}". */
  type Target = `role:${number}` | `member:${number}`

  let project = $state.raw<Project | null>(null)
  let overrides = $state.raw<Override[]>([])
  let roles = $state.raw<GuildRole[]>([])
  let members = $state.raw<Member[]>([])
  let permissions = $state.raw<PermissionInfo[]>([])
  let loadError = $state('')

  /** Picked but not saved yet, from "Add role or member". */
  let added = $state.raw<Target[]>([])
  let picked = $state<Target | null>(null)
  let settings = $state<Partial<Record<Permission, Setting>>>({})
  let saving = $state(false)
  let formError = $state('')

  const targetOf = (o: Override): Target => (o.role_id !== null ? `role:${o.role_id}` : `member:${o.member_id ?? 0}`)
  const idOf = (t: Target) => Number(t.split(':')[1])
  const isRole = (t: Target) => t.startsWith('role:')

  const base = $derived(roles.find((r) => r.base) ?? null)
  const byTarget = $derived(new Map(overrides.map((o) => [targetOf(o), o])))
  const roleOf = (t: Target) => roles.find((r) => r.id === idOf(t)) ?? null
  const memberOf = (t: Target) => members.find((m) => m.id === idOf(t)) ?? null

  /** The left list: @everyone, then Roles top first, then Members, each with an override or just added. */
  const targets = $derived.by(() => {
    const all = new Set<Target>([...(base ? [`role:${base.id}` as Target] : []), ...overrides.map(targetOf), ...added])
    const rank = (t: Target) => (isRole(t) ? (roleOf(t)?.base ? 1e9 : (roleOf(t)?.position ?? 0)) : -1)
    return [...all].sort((a, b) => rank(b) - rank(a) || nameOf(a).localeCompare(nameOf(b)))
  })

  function nameOf(t: Target): string {
    return (isRole(t) ? roleOf(t)?.name : memberOf(t)?.name) ?? '?'
  }

  /** Whether the signed-in Member may change the override for t. */
  function editable(t: Target): boolean {
    if (isRole(t)) {
      const r = roleOf(t)
      return r !== null && canEditRole(r, roles)
    }
    const m = memberOf(t)
    return m !== null && canManage(m, 'manage_roles', roles)
  }

  /** What "Add role or member" offers: who has no override yet and may be changed. */
  const addable = $derived.by(() => {
    const shown = new Set(targets)
    const rs = roles.filter((r) => !r.base && !shown.has(`role:${r.id}`) && canEditRole(r, roles)).sort((a, b) => b.position - a.position)
    const ms = members.filter((m) => !shown.has(`member:${m.id}`) && canManage(m, 'manage_roles', roles))
    return { roles: rs, members: ms }
  })

  function savedSettings(t: Target): Partial<Record<Permission, Setting>> {
    const o = byTarget.get(t)
    const out: Partial<Record<Permission, Setting>> = {}
    for (const p of permissions) out[p.key] = o?.allow.includes(p.key) ? 'allow' : o?.deny.includes(p.key) ? 'deny' : 'inherit'
    return out
  }

  function pick(t: Target) {
    picked = t
    reset()
  }

  function reset() {
    settings = picked ? savedSettings(picked) : {}
    formError = ''
  }

  async function load() {
    const [p, o, r, m, ps] = await Promise.all([
      api<{ project: Project }>('GET', `/projects/${id}`),
      api<{ overrides: Override[] }>('GET', `/projects/${id}/permissions`),
      api<{ roles: GuildRole[] }>('GET', '/roles'),
      api<{ members: Member[] }>('GET', '/members'),
      api<{ permissions: PermissionInfo[] }>('GET', '/permissions'),
    ])
    project = p.project
    overrides = o.overrides
    roles = r.roles
    members = m.members
    permissions = ps.permissions.filter((x) => x.overridable)
    untrack(() => {
      if (picked === null || !targets.includes(picked)) picked = targets[0] ?? null
      reset()
    })
  }
  $effect(() => {
    void id
    untrack(() =>
      load().catch((e) => {
        loadError = e instanceof ApiError && e.status === 403 ? 'forbidden' : e.message
      }),
    )
  })

  const dirty = $derived.by(() => {
    if (!picked || !editable(picked)) return false
    const saved = savedSettings(picked)
    return permissions.some((p) => settings[p.key] !== saved[p.key]) || (!byTarget.has(picked) && added.includes(picked))
  })

  async function save() {
    if (!picked || saving || !dirty) return
    saving = true
    formError = ''
    const keys = (s: Setting) => permissions.filter((p) => settings[p.key] === s).map((p) => p.key)
    try {
      await api('PUT', `/projects/${id}/permissions/${isRole(picked) ? 'roles' : 'members'}/${idOf(picked)}`, {
        allow: keys('allow'),
        deny: keys('deny'),
      })
      added = added.filter((t) => t !== picked)
      await load()
      // An override on a Role one holds changes one's own Permissions here.
      await projectAccess.refresh()
      toast.success('Permissions saved.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      formError = Object.values(err.errors)[0] ?? err.message
    } finally {
      saving = false
    }
  }

  async function remove() {
    if (!picked) return
    const t = picked
    formError = ''
    try {
      const saved = byTarget.has(t)
      if (saved) await api('DELETE', `/projects/${id}/permissions/${isRole(t) ? 'roles' : 'members'}/${idOf(t)}`)
      added = added.filter((x) => x !== t)
      picked = null
      await load()
      if (!saved) return
      await projectAccess.refresh()
      toast.success('Override removed.')
    } catch (err) {
      formError = err instanceof Error ? err.message : String(err)
    }
  }

  function add(t: Target) {
    added = [...added, t]
    pick(t)
  }

  const segments: { value: Setting; label: string; icon: Component<{ class?: string }>; on: string }[] = [
    { value: 'deny', label: 'Deny', icon: X, on: 'bg-destructive/15 text-destructive' },
    { value: 'inherit', label: 'Inherit', icon: Slash, on: 'bg-accent text-foreground' },
    { value: 'allow', label: 'Allow', icon: Check, on: 'bg-green-500/15 text-green-700 dark:text-green-400' },
  ]

  function initials(m: Member): string {
    const words = m.name.trim().split(/\s+/).filter(Boolean)
    return (words.length > 1 ? words[0][0] + words[words.length - 1][0] : (words[0] ?? m.email).slice(0, 2)).toUpperCase()
  }

  const crumbName = $derived(project?.name)
  $effect(() => {
    if (crumbName !== undefined)
      breadcrumb.set({ label: 'Projects', href: href('/projects') }, { label: crumbName, href: href(`/project/${id}`) }, { label: 'Permissions' })
  })
</script>

{#snippet leadingOf(t: Target)}
  {@const r = isRole(t) ? roleOf(t) : null}
  {@const m = isRole(t) ? null : memberOf(t)}
  {#if r}
    <span class="size-2.5 shrink-0 rounded-full" style:background-color={r.color}></span>
  {:else if m}
    <Avatar.Root size="sm"><Avatar.Fallback>{initials(m)}</Avatar.Fallback></Avatar.Root>
  {/if}
{/snippet}

{#if loadError === 'forbidden'}
  <SettingsPage icon={Lock} title="Project permissions">
    <Callout type="info" title="Read only">You need the Manage roles permission to see or change project permissions.</Callout>
  </SettingsPage>
{:else if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !project}
  <PageSkeleton />
{:else}
  <SettingsPage icon={Lock} title="Project permissions" data-testid="project-permissions">
    {#snippet actions()}
      <a href={href(`/project/${id}/edit`)} class={buttonVariants({ variant: 'outline', size: 'sm' })}>Project settings</a>
    {/snippet}
    <p class="-mt-4 max-w-2xl text-sm text-muted-foreground">
      Allow or deny a role or a member something in <strong class="font-medium text-foreground">{project.name}</strong> only. A member's own
      override beats their roles', and a role's deny beats a role's allow.
    </p>

    <div class="grid gap-8 lg:grid-cols-[18rem_1fr]">
      <section class="min-w-0 space-y-4">
        <div class="flex items-center justify-between gap-2">
          <div class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Roles and members</div>
          {#if addable.roles.length + addable.members.length > 0}
            <DropdownMenu.Root>
              <DropdownMenu.Trigger
                class={buttonVariants({ variant: 'ghost', size: 'icon-sm' })}
                aria-label="Add role or member"
                title="Add role or member"
              >
                <Plus class="size-4" />
              </DropdownMenu.Trigger>
              <DropdownMenu.Content align="end" class="max-h-80 w-60 overflow-y-auto">
                {#if addable.roles.length > 0}
                  <DropdownMenu.Label>Roles</DropdownMenu.Label>
                  {#each addable.roles as r (r.id)}
                    <DropdownMenu.Item onSelect={() => add(`role:${r.id}`)} data-testid="add-target" data-target="role:{r.id}">
                      <span class="size-2.5 shrink-0 rounded-full" style:background-color={r.color}></span>
                      <span class="truncate">{r.name}</span>
                    </DropdownMenu.Item>
                  {/each}
                {/if}
                {#if addable.roles.length > 0 && addable.members.length > 0}<DropdownMenu.Separator />{/if}
                {#if addable.members.length > 0}
                  <DropdownMenu.Label>Members</DropdownMenu.Label>
                  {#each addable.members as m (m.id)}
                    <DropdownMenu.Item onSelect={() => add(`member:${m.id}`)} data-testid="add-target" data-target="member:{m.id}">
                      <Avatar.Root size="sm"><Avatar.Fallback>{initials(m)}</Avatar.Fallback></Avatar.Root>
                      <span class="truncate">{m.name}</span>
                    </DropdownMenu.Item>
                  {/each}
                {/if}
              </DropdownMenu.Content>
            </DropdownMenu.Root>
          {/if}
        </div>
        <div class="overflow-hidden rounded-md border border-border" data-testid="override-targets">
          {#each targets as t (t)}
            <EntityRow
              title={nameOf(t)}
              subtitle={isRole(t) ? (roleOf(t)?.base ? 'Every member' : 'Role') : (memberOf(t)?.email ?? 'Member')}
              selected={picked === t}
              class={picked === t ? 'bg-accent/70!' : ''}
              onclick={() => pick(t)}
              data-testid="override-target"
            >
              {#snippet leading()}{@render leadingOf(t)}{/snippet}
              {#snippet trailing()}
                {#if !byTarget.has(t) && added.includes(t)}<span class="text-xs text-muted-foreground">Unsaved</span>{/if}
              {/snippet}
            </EntityRow>
          {/each}
        </div>
      </section>

      {#if picked}
        {@const canEdit = editable(picked)}
        <form
          class="min-w-0 space-y-8"
          onsubmit={(e) => {
            e.preventDefault()
            save()
          }}
        >
          {#if canEdit}
            <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
          {:else}
            <Callout type="info" title="Read only">
              {isRole(picked) ? 'This role is' : 'This member is'} at or above your highest role, so you cannot change their override.
            </Callout>
          {/if}
          {#if formError}<p class="text-sm text-destructive" data-testid="override-error">{formError}</p>{/if}

          <SettingsGroup
            label={nameOf(picked)}
            hint="Inherit keeps what the guild's roles give. You can only switch permissions you hold in this project."
            data-testid="override-section"
          >
            <div class="divide-y divide-border">
              {#each permissions as p (p.key)}
                <div class="flex items-center justify-between gap-4 py-3 first:pt-0" data-testid="override-permission" data-permission={p.key}>
                  <span class="min-w-0">
                    <span class="block text-sm font-medium">{p.name}</span>
                    <span class="block text-xs text-muted-foreground">{p.description}</span>
                  </span>
                  <div class="flex shrink-0 items-center overflow-hidden rounded-md border border-border" role="radiogroup" aria-label={p.name}>
                    {#each segments as s (s.value)}
                      <button
                        type="button"
                        role="radio"
                        aria-checked={settings[p.key] === s.value}
                        aria-label={s.label}
                        title={s.label}
                        data-setting={s.value}
                        disabled={!canEdit || !projectAccess.can(p.key)}
                        class={[
                          'flex size-8 items-center justify-center border-l border-border transition-colors first:border-l-0 focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none focus-visible:ring-inset disabled:cursor-not-allowed disabled:opacity-50',
                          settings[p.key] === s.value ? s.on : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground',
                        ]}
                        onclick={() => (settings[p.key] = s.value)}
                      >
                        <s.icon class="size-3.5" />
                      </button>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          </SettingsGroup>

          {#if canEdit && byTarget.has(picked)}
            <SettingsGroup label="Danger Zone" destructive>
              <p class="text-sm text-muted-foreground">
                Removing the override sends <strong class="font-medium text-foreground">{nameOf(picked)}</strong> back to what the guild's roles
                give in this project.
              </p>
              <ConfirmationModal
                title="Remove override?"
                buttonTitle="Remove override"
                variant="error"
                actions={[`${nameOf(picked)} goes back to what the guild's roles give in this project.`]}
                confirmWithText={false}
                step2ButtonText="Remove"
                onconfirm={remove}
              />
            </SettingsGroup>
          {:else if canEdit && added.includes(picked)}
            <div><Button onclick={remove}>Cancel</Button></div>
          {/if}
        </form>
      {/if}
    </div>
  </SettingsPage>
{/if}
