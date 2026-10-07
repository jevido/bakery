<script lang="ts">
  // A Project's Permission overrides, as Discord's channel permission
  // screen: the Roles (@everyone always first) and Members with an override
  // on the left, and for the one picked, each Permission a Project can
  // override with deny / inherit / allow on the right. Only a Role below
  // one's own highest (or @everyone) and a Member ranked below one are
  // changed, and only Permissions one holds in this Project are switched
  // (the server's SetOverride and CanGrant).
  import { untrack } from 'svelte'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { canEditRole, canManage } from '../lib/hierarchy'
  import Icon from '../lib/Icon.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import { href } from '../lib/router.svelte'
  import type { Member, Permission } from '../lib/session.svelte'
  import type { GuildRole, PermissionInfo, Project } from '../lib/types'
  import Button from '../lib/ui/Button.svelte'
  import Callout from '../lib/ui/Callout.svelte'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import SettingsSection from '../lib/ui/SettingsSection.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import TableDropdown from '../lib/ui/TableDropdown.svelte'
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
        loadError = e instanceof ApiError && e.status === 403 ? 'You need the Manage roles permission to change project permissions.' : e.message
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

  const segments: { value: Setting; label: string; icon: string; on: string }[] = [
    { value: 'deny', label: 'Deny', icon: '✕', on: 'bg-red-600 text-white' },
    { value: 'inherit', label: 'Inherit', icon: '/', on: 'bg-neutral-500 text-white dark:bg-neutral-600' },
    { value: 'allow', label: 'Allow', icon: '✓', on: 'bg-green-600 text-white' },
  ]

  const crumbName = $derived(project?.name)
  $effect(() => {
    if (crumbName !== undefined)
      breadcrumb.set({ label: 'Projects', href: href('/projects') }, { label: crumbName, href: href(`/project/${id}`) }, { label: 'Permissions' })
  })
</script>

{#if loadError}
  <p class="text-sm text-error">{loadError}</p>
{:else if !project}
  <Spinner text="Loading…" />
{:else}
  <div class="chrome application-settings-form w-full">
    <header class="mb-5">
      <h1 class="truncate text-[24px]! leading-7! font-semibold! tracking-tight!">{project.name}</h1>
      <p class="mt-1 text-[13px] text-neutral-500 dark:text-fg-dim">
        Project permissions: allow or deny a role or a member something in this project only. A member's own override beats
        their roles', and a role's deny beats a role's allow.
      </p>
    </header>

    <div class="grid gap-6 lg:grid-cols-[16rem_1fr]">
      <aside class="flex flex-col gap-2">
        <div class="flex items-center justify-between">
          <h2 class="text-xs font-semibold tracking-wide text-neutral-500 uppercase dark:text-fg-faint">Roles and members</h2>
          {#if addable.roles.length + addable.members.length > 0}
            <TableDropdown panelClass="w-60! max-h-80 overflow-y-auto">
              {#snippet trigger({ open, toggle })}
                <button
                  type="button"
                  class="flex size-7 items-center justify-center rounded-md text-neutral-500 hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg"
                  aria-haspopup="listbox"
                  aria-expanded={open}
                  aria-label="Add role or member"
                  title="Add role or member"
                  onclick={toggle}
                >
                  <Icon name="plus" class="size-3.5" />
                </button>
              {/snippet}
              {#snippet children(close)}
                {#if addable.roles.length > 0}
                  <p class="px-2 pt-1 pb-1 text-[11px] font-semibold text-neutral-500 uppercase dark:text-fg-faint">Roles</p>
                  {#each addable.roles as r (r.id)}
                    <button
                      type="button"
                      role="option"
                      aria-selected="false"
                      class="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-[12px] hover:bg-neutral-100 dark:hover:bg-white/[0.06]"
                      onclick={() => {
                        add(`role:${r.id}`)
                        close()
                      }}
                    >
                      <span class="size-2.5 shrink-0 rounded-full" style:background-color={r.color}></span>
                      {r.name}
                    </button>
                  {/each}
                {/if}
                {#if addable.members.length > 0}
                  <p class="px-2 pt-2 pb-1 text-[11px] font-semibold text-neutral-500 uppercase dark:text-fg-faint">Members</p>
                  {#each addable.members as m (m.id)}
                    <button
                      type="button"
                      role="option"
                      aria-selected="false"
                      class="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-[12px] hover:bg-neutral-100 dark:hover:bg-white/[0.06]"
                      onclick={() => {
                        add(`member:${m.id}`)
                        close()
                      }}
                    >
                      <Icon name="profile" class="size-3" />
                      {m.name}
                    </button>
                  {/each}
                {/if}
              {/snippet}
            </TableDropdown>
          {/if}
        </div>
        <ul class="flex flex-col gap-0.5" data-testid="override-targets">
          {#each targets as t (t)}
            {@const r = isRole(t) ? roleOf(t) : null}
            <li>
              <button
                type="button"
                class={[
                  'flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-[13px] transition-colors',
                  picked === t ? 'control-selected' : 'hover:bg-neutral-100 dark:hover:bg-white/[0.06]',
                ]}
                aria-pressed={picked === t}
                data-testid="override-target"
                onclick={() => pick(t)}
              >
                {#if r}
                  <span class="size-2.5 shrink-0 rounded-full" style:background-color={r.color}></span>
                {:else}
                  <Icon name="profile" class="size-3" />
                {/if}
                <span class="truncate">{nameOf(t)}</span>
              </button>
            </li>
          {/each}
        </ul>
      </aside>

      {#if picked}
        {@const canEdit = editable(picked)}
        <form
          class="flex flex-col gap-4"
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
          {#if formError}<p class="text-sm text-red-600 dark:text-red-400" data-testid="override-error">{formError}</p>{/if}

          <SettingsSection
            id="override-section"
            title={nameOf(picked)}
            helper="Inherit keeps what the guild's roles give. You can only switch permissions you hold in this project."
          >
            <ul class="flex flex-col divide-y divide-neutral-200 dark:divide-white/[0.06]">
              {#each permissions as p (p.key)}
                <li class="flex items-center justify-between gap-4 py-2.5" data-testid="override-permission" data-permission={p.key}>
                  <span class="min-w-0">
                    <span class="block text-sm font-medium">{p.name}</span>
                    <span class="block text-xs text-neutral-500 dark:text-fg-faint">{p.description}</span>
                  </span>
                  <div
                    class="flex shrink-0 overflow-hidden rounded-md border border-neutral-200 dark:border-white/[0.08]"
                    role="radiogroup"
                    aria-label={p.name}
                  >
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
                          'flex size-8 items-center justify-center text-sm font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-50',
                          settings[p.key] === s.value ? s.on : 'text-neutral-500 hover:bg-neutral-100 dark:text-fg-faint dark:hover:bg-white/[0.06]',
                        ]}
                        onclick={() => (settings[p.key] = s.value)}
                      >
                        {s.icon}
                      </button>
                    {/each}
                  </div>
                </li>
              {/each}
            </ul>
          </SettingsSection>

          {#if canEdit && (byTarget.has(picked) || added.includes(picked))}
            <div>
              {#if byTarget.has(picked)}
                <ConfirmationModal
                  title="Remove override?"
                  buttonTitle="Remove override"
                  variant="error"
                  actions={[`${nameOf(picked)} goes back to what the guild's roles give in this project.`]}
                  confirmWithText={false}
                  step2ButtonText="Remove"
                  onconfirm={remove}
                />
              {:else}
                <Button onclick={remove}>Cancel</Button>
              {/if}
            </div>
          {/if}
        </form>
      {/if}
    </div>
  </div>
{/if}
