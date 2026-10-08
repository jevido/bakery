<script lang="ts">
  // The Guild's Roles, as Discord's Roles settings: top Position first,
  // @everyone pinned last. Every Member reads them; with manage_roles the
  // Roles below one's own highest are dragged (or moved with the buttons)
  // into a new order and new ones created. Roles at or above it are locked,
  // as the server refuses to move them. Paperclip has no Roles page; the
  // rows are its EntityRow (ui/src/components/EntityRow.tsx; MIT, see NOTICE)
  // in a bordered list, under its settings page frame.
  import { ArrowDown, ArrowUp, GripVertical, Lock, Plus, Shield } from '@lucide/svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { api } from '../../lib/api'
  import { myRank } from '../../lib/hierarchy'
  import { go, href, rolePath } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { GuildRole } from '../../lib/types'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
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

{#snippet row(r: GuildRole)}
  <span class="size-3 shrink-0 rounded-full" style:background-color={r.color}></span>
  <a class="min-w-0 flex-1 truncate font-medium hover:underline" href={href(rolePath(r.id))}>{r.name}</a>
{/snippet}

<SettingsPage icon={Shield} title="Roles">
  {#snippet actions()}
    {#if canManage}
      <Button onclick={create} disabled={busy} data-testid="create-role"><Plus />Create role</Button>
    {/if}
  {/snippet}
  <p class="max-w-3xl text-sm text-muted-foreground">
    Members hold any number of roles and get the permissions of all of them. A role higher in the list is above the ones below it; you can
    only change roles below your own highest role.
  </p>
  {#if roles === null && !error}
    <Spinner text="Loading…" />
  {:else}
    {#if error}<p class="text-sm text-destructive" data-testid="roles-error">{error}</p>{/if}
    <ol class="max-w-3xl overflow-hidden rounded-xl border border-border" aria-label="Roles, highest first">
      {#each ordered as r, i (r.id)}
        <li
          class={[
            'flex items-center gap-3 border-b border-border px-4 py-2 text-sm transition-colors last:border-b-0 hover:bg-accent/50',
            over === i && dragging !== i && 'bg-accent/30 shadow-[inset_0_2px_0_var(--ring)]',
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
            <GripVertical class="size-4 shrink-0 cursor-grab text-muted-foreground" aria-hidden="true" />
          {:else}
            <span title="Only Roles below your highest Role can be changed" data-testid="role-locked">
              <Lock class="size-4 shrink-0 text-muted-foreground" />
            </span>
          {/if}
          {@render row(r)}
          <span class="text-xs text-muted-foreground">{r.members} {r.members === 1 ? 'member' : 'members'}</span>
          {#if movable(r)}
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label="Move {r.name} up"
              disabled={busy || i === 0 || !movable(ordered[i - 1])}
              onclick={() => move(i, i - 1)}><ArrowUp /></Button
            >
            <Button variant="ghost" size="icon-xs" aria-label="Move {r.name} down" disabled={busy || i === ordered.length - 1} onclick={() => move(i, i + 1)}
              ><ArrowDown /></Button
            >
          {/if}
        </li>
      {/each}
      {#if everyone}
        <li class="flex items-center gap-3 px-4 py-2 text-sm transition-colors hover:bg-accent/50" data-testid="role-row" data-role="@everyone">
          <span class="w-4 shrink-0"></span>
          {@render row(everyone)}
          <span class="text-xs text-muted-foreground">every member</span>
        </li>
      {/if}
    </ol>
  {/if}
</SettingsPage>
