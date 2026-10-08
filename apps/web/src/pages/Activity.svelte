<script lang="ts">
  // Paperclip's Activity page (ui/src/pages/audit/CompanyActivity.tsx over
  // AuditFeed.tsx in its "all" mode, rows from components/ActivityRow.tsx;
  // MIT, see NOTICE): every change to the Guild's Goals, Issues and Approvals, newest
  // first, fifty at a time under "Load more". The entity and Actor selects
  // live in the hash query (#/activity?entity=issue&actor=3), so a reload
  // keeps them. Left out: the Agent Actions mode, the action and date
  // filters and the CSV export, which wait for agents.
  import { untrack } from 'svelte'
  import * as Avatar from '@bakery/ui/components/ui/avatar'
  import { Button as UiButton } from '@bakery/ui/components/ui/button'
  import * as Select from '@bakery/ui/components/ui/select'
  import { api } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { ago } from '../lib/format'
  import { initials } from '../lib/Identity.svelte'
  import PageHeader from '../lib/PageHeader.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { href } from '../lib/router.svelte'
  import type { Member } from '../lib/session.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import { activityVerb, listActivity, type ActivityEntity, type ActivityEvent } from '../lib/work'

  const pageSize = 50
  const entities = [
    { value: 'all', label: 'All activity' },
    { value: 'issue', label: 'Issues' },
    { value: 'goal', label: 'Goals' },
    { value: 'approval', label: 'Approvals' },
  ]

  // The filters as the hash query holds them, read again when a link or Back
  // changes the hash under the open page.
  function fromHash() {
    const query = new URLSearchParams(location.hash.split('?')[1] ?? '')
    const e = query.get('entity')
    const a = query.get('actor') ?? ''
    return { entity: e === 'issue' || e === 'goal' || e === 'approval' ? e : 'all', actor: /^\d+$/.test(a) ? a : 'everyone' }
  }
  const initial = fromHash()
  let entity = $state(initial.entity)
  let actor = $state(initial.actor)
  $effect(() => {
    const follow = () => {
      if (!location.hash.startsWith('#/activity')) return
      const f = fromHash()
      entity = f.entity
      actor = f.actor
    }
    window.addEventListener('hashchange', follow)
    return () => window.removeEventListener('hashchange', follow)
  })
  $effect(() => {
    const q = new URLSearchParams()
    if (entity !== 'all') q.set('entity', entity)
    if (actor !== 'everyone') q.set('actor', actor)
    const next = `#/activity${q.size ? `?${q}` : ''}`
    if (location.hash !== next) history.replaceState(history.state, '', next)
  })

  let events = $state.raw<ActivityEvent[] | null>(null)
  let more = $state(false)
  let loadingMore = $state(false)
  let loadError = $state('')
  let members = $state.raw<Member[]>([])
  api<{ members: Member[] }>('GET', '/members').then((r) => (members = r.members)).catch(() => {})

  // Each change of the filters asks again from the newest; a late answer
  // for filters already left behind is dropped.
  let asked = 0
  function load(before?: number) {
    const ask = ++asked
    return listActivity({
      entity: entity === 'all' ? undefined : (entity as ActivityEntity),
      actor: actor === 'everyone' ? undefined : Number(actor),
      before,
      limit: pageSize,
    })
      .then((page) => {
        if (ask !== asked) return
        events = [...(before ? (events ?? []) : []), ...page]
        more = page.length >= pageSize
        loadError = ''
      })
      .catch((e) => {
        if (ask === asked) loadError = e.message
      })
  }
  $effect(() => {
    void [entity, actor]
    untrack(() => load())
  })

  function loadMore() {
    loadingMore = true
    load(events?.at(-1)?.id).finally(() => (loadingMore = false))
  }

  const filtered = $derived(entity !== 'all' || actor !== 'everyone')
  const actorLabel = $derived(actor === 'everyone' ? 'Everyone' : (members.find((m) => String(m.id) === actor)?.name ?? 'Member'))
  const paths: Record<ActivityEntity, (e: ActivityEvent) => string> = {
    issue: (e) => `/issues/${e.entity.identifier}`,
    goal: (e) => `/goals/${e.entity.id}`,
    approval: (e) => `/approvals/${e.entity.id}`,
  }
  const link = (e: ActivityEvent) => (e.entity.exists ? href(paths[e.entity.type](e)) : null)

  $effect(() => breadcrumb.set({ label: 'Activity' }))
</script>

<div class="chrome space-y-4">
  <PageHeader title="Activity" description="Everything that happened to the guild's work, newest first. Each line is one recorded action." />

  <div class="flex flex-wrap items-end gap-3 border-y border-border py-3" role="toolbar" aria-label="Activity filters">
    <label class="grid gap-1 text-(length:--text-micro) font-medium text-muted-foreground">
      <span>Entity</span>
      <Select.Root type="single" bind:value={entity}>
        <Select.Trigger class="w-40" aria-label="Entity">{entities.find((e) => e.value === entity)?.label}</Select.Trigger>
        <Select.Content>
          {#each entities as e (e.value)}
            <Select.Item value={e.value} label={e.label} />
          {/each}
        </Select.Content>
      </Select.Root>
    </label>
    <label class="grid gap-1 text-(length:--text-micro) font-medium text-muted-foreground">
      <span>Actor</span>
      <Select.Root type="single" bind:value={actor}>
        <Select.Trigger class="w-52" aria-label="Actor">{actorLabel}</Select.Trigger>
        <Select.Content>
          <Select.Item value="everyone" label="Everyone" />
          {#each members as m (m.id)}
            <Select.Item value={String(m.id)} label={m.name} />
          {/each}
        </Select.Content>
      </Select.Root>
    </label>
    {#if filtered}
      <UiButton variant="ghost" size="sm" onclick={() => ((entity = 'all'), (actor = 'everyone'))}>Clear filters</UiButton>
    {/if}
  </div>

  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}
  {#if events === null && !loadError}
    <PageSkeleton />
  {:else if events && events.length === 0 && !filtered}
    <Empty title="No activity yet." icon="activity" />
  {:else if events && events.length === 0}
    <p class="py-10 text-center text-sm text-muted-foreground">No activity matches these filters.</p>
  {:else if events}
    <ul class="overflow-hidden rounded-lg border border-border" aria-label="Activity">
      {#each events as event (event.id)}
        {@const name = event.actor?.name ?? 'Board'}
        {@const verb = activityVerb(event.action)}
        {@const to = link(event)}
        <li class="border-b border-border last:border-b-0" data-activity={event.action}>
          <svelte:element
            this={to ? 'a' : 'div'}
            href={to}
            class={['dashboard-list-row flex items-center gap-2 text-sm text-inherit no-underline', to && 'cursor-pointer transition-colors hover:bg-accent/50']}
          >
            <Avatar.Root size="sm" aria-hidden="true">
              <Avatar.Fallback>{initials(name)}</Avatar.Fallback>
            </Avatar.Root>
            <p class="flex h-6 min-w-0 flex-1 items-center gap-1.5">
              <span class="max-w-1/2 shrink-0 truncate" title="{name} {verb}"><span>{name}</span> <span class="text-muted-foreground">{verb}</span></span>
              <span class="min-w-0 flex-1 truncate" title={event.entity.title}>{event.entity.title}</span>
            </p>
            <span class="w-20 shrink-0 truncate text-right font-mono text-(length:--text-micro) text-muted-foreground">{event.entity.identifier ?? ''}</span>
            <span class="w-28 shrink-0 text-right text-xs whitespace-nowrap text-muted-foreground">{ago(event.created_at)}</span>
          </svelte:element>
        </li>
      {/each}
    </ul>
    {#if more}
      <div class="flex justify-center">
        <UiButton variant="outline" size="sm" disabled={loadingMore} onclick={loadMore}>{loadingMore ? 'Loading…' : 'Load more'}</UiButton>
      </div>
    {/if}
  {/if}
</div>
