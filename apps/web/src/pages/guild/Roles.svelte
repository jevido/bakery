<script lang="ts">
  // The Guild's Roles, as Discord's Roles settings: top Position first,
  // @everyone pinned last. Every Member reads them; with manage_roles the
  // Roles below one's own highest are dragged (or moved with the buttons)
  // into a new order and new ones created. Roles at or above it are locked,
  // as the server refuses to move them.
  import { api } from '../../lib/api'
  import { myRank } from '../../lib/hierarchy'
  import Icon from '../../lib/Icon.svelte'
  import { go, href, rolePath } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { GuildRole } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'

  let roles = $state.raw<GuildRole[] | null>(null)
  let error = $state('')
  let busy = $state(false)
  // The Role being dragged and the row it hovers, by index among `ordered`.
  let dragging = $state<number | null>(null)
  let over = $state<number | null>(null)

  const ordered = $derived((roles ?? []).filter((r) => !r.base))
  const everyone = $derived(roles?.find((r) => r.base) ?? null)
  const canManage = $derived(session.can('manage_roles'))
  const rank = $derived(roles ? myRank(roles) : 0)

  async function load() {
    roles = (await api<{ roles: GuildRole[] }>('GET', '/roles')).roles
  }
  load().catch((e) => (error = e.message))

  /** Whether the signed-in Member may move this Role: below their highest Role. */
  function movable(r: GuildRole): boolean {
    return canManage && rank > r.position
  }

  /** Moves the Role at from to to (both movable) and saves the order; an error shows and restores it. */
  async function move(from: number, to: number) {
    if (busy || from === to || to < 0 || to >= ordered.length) return
    if (!movable(ordered[from]) || !movable(ordered[to])) return
    const next = [...ordered]
    const [r] = next.splice(from, 1)
    next.splice(to, 0, r)
    const before = roles
    roles = everyone ? [...next, everyone] : next
    busy = true
    error = ''
    try {
      roles = (await api<{ roles: GuildRole[] }>('PUT', '/roles/order', { role_ids: next.map((x) => x.id) })).roles
    } catch (err) {
      roles = before
      error = err instanceof Error ? err.message : String(err)
    } finally {
      busy = false
    }
  }

  async function create() {
    busy = true
    error = ''
    try {
      const { role } = await api<{ role: GuildRole }>('POST', '/roles', { name: 'new role' })
      go(rolePath(role.id))
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      busy = false
    }
  }

  function drop(e: DragEvent, to: number) {
    e.preventDefault()
    const from = dragging
    dragging = over = null
    if (from !== null) move(from, to)
  }
</script>

<SettingsSection
  id="guild-roles-section"
  title="Roles"
  helper="Members hold any number of Roles and get the Permissions of all of them. A Role higher in the list is above the ones below it; you can only change Roles below your own highest Role."
>
  {#snippet actions()}
    {#if canManage}
      <Button variant="highlighted" onclick={create} disabled={busy} data-testid="create-role">Create role</Button>
    {/if}
  {/snippet}
  {#if roles === null && !error}
    <Spinner text="Loading…" />
  {:else}
    {#if error}<p class="text-sm text-red-600 dark:text-red-400" data-testid="roles-error">{error}</p>{/if}
    <ol class="flex flex-col gap-1" aria-label="Roles, highest first">
      {#each ordered as r, i (r.id)}
        <li
          class={[
            'flex items-center gap-3 rounded-md border px-3 py-2 dark:border-white/[0.08]',
            over === i && dragging !== i ? 'border-coollabs dark:border-coollabs' : 'border-neutral-200',
            dragging === i && 'opacity-50',
          ]}
          draggable={movable(r) && !busy}
          ondragstart={(e) => {
            dragging = i
            e.dataTransfer?.setData('text/plain', String(r.id))
          }}
          ondragend={() => (dragging = over = null)}
          ondragover={(e) => {
            if (dragging !== null && movable(r)) {
              e.preventDefault()
              over = i
            }
          }}
          ondrop={(e) => drop(e, i)}
          data-testid="role-row"
          data-role={r.name}
        >
          {#if movable(r)}
            <span class="cursor-grab text-neutral-400 select-none" aria-hidden="true">⠿</span>
          {:else}
            <span title="Only Roles below your highest Role can be changed" data-testid="role-locked">
              <Icon name="lock" class="size-4 text-neutral-400" />
            </span>
          {/if}
          <span class="size-3 shrink-0 rounded-full" style:background-color={r.color}></span>
          <a class="min-w-0 flex-1 truncate font-medium" href={href(rolePath(r.id))}>{r.name}</a>
          <span class="text-xs text-neutral-500 dark:text-fg-faint">{r.members} {r.members === 1 ? 'member' : 'members'}</span>
          {#if movable(r)}
            <button
              class="px-1 disabled:opacity-30"
              aria-label="Move {r.name} up"
              disabled={busy || i === 0 || !movable(ordered[i - 1])}
              onclick={() => move(i, i - 1)}>↑</button
            >
            <button
              class="px-1 disabled:opacity-30"
              aria-label="Move {r.name} down"
              disabled={busy || i === ordered.length - 1}
              onclick={() => move(i, i + 1)}>↓</button
            >
          {/if}
        </li>
      {/each}
      {#if everyone}
        <li class="flex items-center gap-3 rounded-md border border-neutral-200 px-3 py-2 dark:border-white/[0.08]" data-testid="role-row" data-role="@everyone">
          <span class="w-4"></span>
          <span class="size-3 shrink-0 rounded-full" style:background-color={everyone.color}></span>
          <a class="min-w-0 flex-1 truncate font-medium" href={href(rolePath(everyone.id))}>{everyone.name}</a>
          <span class="text-xs text-neutral-500 dark:text-fg-faint">every member</span>
        </li>
      {/if}
    </ol>
  {/if}
</SettingsSection>
