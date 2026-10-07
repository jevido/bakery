<script lang="ts">
  // One Role of the Guild, as Discord's Role settings: its name and color,
  // its Permissions grouped as Discord groups them, who holds it, and
  // deleting it. Only a Role below one's own highest is changed, and only
  // Permissions one holds are switched (the server's CanGrant); @everyone
  // keeps its name and color and is never deleted. In Paperclip's settings
  // page frame and groups, its Permissions as Paperclip's ToggleField rows
  // (ui/src/components/agent-config-primitives.tsx; MIT, see NOTICE).
  import { ArrowLeft, Shield } from '@lucide/svelte'
  import { Switch } from '$lib/components/ui/switch'
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { canAssign, canEditRole, canManage } from '../../lib/hierarchy'
  import { go, guildPath, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Permission } from '../../lib/session.svelte'
  import type { GuildRole, Member, PermissionInfo } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Callout from '../../lib/ui/Callout.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'

  let { id }: { id: number } = $props()

  const groups: { title: string; keys: Permission[]; hint?: string }[] = [
    { title: 'General', keys: ['view_resources', 'manage_guild', 'manage_roles', 'manage_members'] },
    { title: 'Resources', keys: ['see_secrets', 'deploy', 'manage_applications', 'manage_servers', 'manage_notifications'] },
    {
      title: 'Agents and work',
      keys: ['manage_work', 'hire_agents', 'approve', 'manage_budgets'],
      hint: 'Hire agents, Approve and Manage budgets are checked once agents exist.',
    },
    { title: 'Advanced', keys: ['administrator'] },
  ]
  // Discord's role color swatches.
  const swatches = ['#1abc9c', '#2ecc71', '#3498db', '#9b59b6', '#e91e63', '#f1c40f', '#e67e22', '#e74c3c', '#95a5a6', '#607d8b']

  let roles = $state.raw<GuildRole[] | null>(null)
  let permissions = $state.raw<PermissionInfo[]>([])
  let members = $state.raw<Member[]>([])
  let loadError = $state('')

  let name = $state('')
  let color = $state('#99aab5')
  let granted = $state<Permission[]>([])
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)
  let rowError = $state('')

  const role = $derived(roles?.find((r) => r.id === id) ?? null)
  const editable = $derived(role !== null && roles !== null && canEditRole(role, roles))
  const holders = $derived(members.filter((m) => m.roles?.some((r) => r.id === id)))
  const byKey = $derived(new Map(permissions.map((p) => [p.key, p])))

  function reset() {
    if (!role) return
    name = role.name
    color = role.color
    granted = [...role.permissions]
    errors = {}
  }

  async function load() {
    const [r, p, m] = await Promise.all([
      api<{ roles: GuildRole[] }>('GET', '/roles'),
      api<{ permissions: PermissionInfo[] }>('GET', '/permissions'),
      api<{ members: Member[] }>('GET', '/members'),
    ])
    roles = r.roles
    permissions = p.permissions
    members = m.members
    untrack(reset)
  }
  $effect(() => {
    void id
    untrack(() => load().catch((e) => (loadError = e.message)))
  })

  const sameSet = (a: string[], b: string[]) => a.length === b.length && a.every((k) => b.includes(k))
  const dirty = $derived(
    editable && role !== null && (name !== role.name || color.toLowerCase() !== role.color || !sameSet(granted, role.permissions)),
  )

  function toggle(key: Permission, on: boolean) {
    granted = on ? [...granted, key] : granted.filter((k) => k !== key)
  }

  async function save() {
    if (saving || !dirty || !role) return
    saving = true
    errors = {}
    try {
      const body: Record<string, unknown> = { permissions: granted }
      if (!role.base) Object.assign(body, { name, color })
      await api('PATCH', `/roles/${id}`, body)
      await load()
      // Editing a Role one holds changes one's own Permissions.
      await session.refresh()
      toast.success('Role saved.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { form: err.message }
    } finally {
      saving = false
    }
  }

  async function remove() {
    try {
      await api('DELETE', `/roles/${id}`)
      await session.refresh()
      toast.success('Role deleted.')
      go(guildPath('roles'))
    } catch (err) {
      errors = { form: err instanceof Error ? err.message : String(err) }
    }
  }

  async function takeFrom(m: Member) {
    rowError = ''
    try {
      await api('DELETE', `/members/${m.id}/roles/${id}`)
    } catch (err) {
      rowError = err instanceof Error ? err.message : String(err)
    }
    await load()
  }
</script>

<a class="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground" href={href(guildPath('roles'))}>
  <ArrowLeft class="size-3.5" />Roles
</a>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if roles === null}
  <Spinner text="Loading…" />
{:else if role === null}
  <p class="text-sm text-muted-foreground">This guild has no such role.</p>
{:else}
  <SettingsPage icon={Shield} title={role.name} data-testid="role-page">
    <form
      class="space-y-8"
      onsubmit={(e) => {
        e.preventDefault()
        save()
      }}
    >
      {#if editable}
        <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
      {:else}
        <Callout type="info" title="Read only">
          {session.can('manage_roles')
            ? 'This role is at or above your highest role, so you cannot change it.'
            : 'You need the Manage roles permission to change roles.'}
        </Callout>
      {/if}
      {#if errors.form}<p class="text-sm text-destructive" data-testid="role-error">{errors.form}</p>{/if}

      <SettingsGroup label="Display" hint={role.base ? 'Every member holds @everyone; only its permissions change.' : 'How this role shows on the Members page.'}>
        <Input label="Role name" bind:value={name} error={errors.name} required disabled={!editable || role.base} />
        <div class="space-y-1.5">
          <span class="text-sm font-medium">Color</span>
          <div class="flex flex-wrap items-center gap-2">
            <input
              type="color"
              bind:value={color}
              disabled={!editable || role.base}
              aria-label="Role color"
              class="h-8 w-10 cursor-pointer rounded-md border border-border bg-transparent disabled:cursor-not-allowed disabled:opacity-50"
            />
            {#each swatches as s (s)}
              <button
                type="button"
                class={['size-6 rounded-full border-2 disabled:cursor-not-allowed disabled:opacity-50', color.toLowerCase() === s ? 'border-foreground' : 'border-transparent']}
                style:background-color={s}
                aria-label="Color {s}"
                disabled={!editable || role.base}
                onclick={() => (color = s)}
              ></button>
            {/each}
          </div>
          {#if errors.color}<p class="text-xs text-destructive">{errors.color}</p>{/if}
        </div>
      </SettingsGroup>

      {#each groups as g (g.title)}
        <SettingsGroup label="{g.title} permissions" hint={g.hint ?? (g.title === 'General' ? 'You can only switch permissions you hold yourself.' : undefined)}>
          <div class="divide-y divide-border">
            {#each g.keys as key (key)}
              {@const p = byKey.get(key)}
              {#if p}
                <label class="flex items-start justify-between gap-4 py-3 first:pt-0" data-testid="permission-toggle" data-permission={key}>
                  <span class="min-w-0">
                    <span class="block text-sm font-medium">{p.name}</span>
                    <span class="block text-xs text-muted-foreground">{p.description}</span>
                    {#if key === 'administrator'}
                      <span class="mt-1 block text-xs text-amber-700 dark:text-amber-300">
                        Grants every permission and bypasses every project override. Give it with care.
                      </span>
                    {/if}
                  </span>
                  <Switch
                    class="mt-0.5"
                    checked={granted.includes(key)}
                    disabled={!editable || !session.can(key)}
                    onCheckedChange={(on) => toggle(key, on)}
                    aria-label={p.name}
                  />
                </label>
              {/if}
            {/each}
          </div>
        </SettingsGroup>
      {/each}

      {#if !role.base}
        <SettingsGroup label="Members" hint="Who holds this role. Assign it on the Members page.">
          {#if rowError}<p class="text-sm text-destructive">{rowError}</p>{/if}
          {#if holders.length === 0}
            <p class="text-sm text-muted-foreground">Nobody holds this role.</p>
          {:else}
            <ul class="overflow-hidden rounded-xl border border-border">
              {#each holders as m (m.id)}
                <li class="flex items-center justify-between gap-4 border-b border-border px-4 py-2 text-sm last:border-b-0" data-testid="role-holder">
                  <span class="min-w-0 truncate"><span class="font-medium">{m.name}</span> <span class="text-muted-foreground">{m.email}</span></span>
                  {#if canAssign(role, roles) && canManage(m, 'manage_roles', roles)}
                    <Button onclick={() => takeFrom(m)}>Remove</Button>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </SettingsGroup>

        {#if editable}
          <SettingsGroup label="Danger Zone" destructive>
            <p class="text-sm text-muted-foreground">Deleting this role takes it from its members; they lose the permissions it gave them.</p>
            <ConfirmationModal
              title="Delete role?"
              buttonTitle="Delete role"
              variant="error"
              actions={[`The role ${role.name} is deleted, and its ${role.members} member(s) lose the permissions it gave them.`]}
              confirmWithText={false}
              step2ButtonText="Delete"
              onconfirm={remove}
            />
          </SettingsGroup>
        {/if}
      {/if}
    </form>
  </SettingsPage>
{/if}
