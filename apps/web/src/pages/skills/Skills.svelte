<script lang="ts">
  // Paperclip's Skills page (CompanySkills and SkillList in
  // ui/src/pages/CompanySkills.tsx; MIT, see NOTICE): New skill over a
  // searchable list of the Guild's Skills, each row with its slug,
  // description, files and the Agents that have it. New skill is only for a
  // Member with manage_skills. Left out, as the first Skills slice leaves
  // them out: the discovery grid and its tabs, the catalog, sources and
  // their filter, stars, forks, categories, sharing, folders, update checks
  // and test runs; the file tree each row expands to is on the Skill's Files
  // tab instead.
  import { Boxes, Plus } from '@lucide/svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import NewSkillDialog from '../../lib/NewSkillDialog.svelte'
  import PageHeader from '../../lib/PageHeader.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { href, skillPath } from '../../lib/router.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import { session } from '../../lib/session.svelte'
  import { formatBytes, listSkills, type Skill } from '../../lib/skills'
  import Empty from '../../lib/ui/Empty.svelte'

  let skills = $state.raw<Skill[] | null>(null)
  let loadError = $state('')
  let query = $state('')
  let creating = $state(false)

  const canManage = $derived(session.can('manage_skills'))

  $effect(() => {
    listSkills()
      .then((ks) => (skills = ks))
      .catch((e) => (loadError = e instanceof ApiError ? e.message : String(e)))
  })
  $effect(() => breadcrumb.set({ label: 'Skills' }))

  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase()
    if (!skills || !q) return skills
    return skills.filter((k) => `${k.name} ${k.slug} ${k.description}`.toLowerCase().includes(q))
  })
</script>

<div class="chrome space-y-6">
  <PageHeader title="Skills" description="Instructions and files the guild's agents load when they run. Give a skill to an agent on its page.">
    {#snippet actions()}
      {#if canManage}
        <Button onclick={() => (creating = true)}><Plus class="size-4" />New skill</Button>
      {/if}
    {/snippet}
  </PageHeader>

  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}

  {#if skills === null && !loadError}
    <PageSkeleton />
  {:else if skills && skills.length === 0}
    <div class="py-12">
      <Empty
        title="No skills yet."
        description="A skill is a SKILL.md with the files it needs, which claude loads when one of the guild's agents runs.{canManage ? ' Use New skill to write the first one.' : ''}"
        icon="skills"
      />
    </div>
  {:else if shown}
    <div class="flex items-center justify-between gap-3">
      <SearchField bind:value={query} label="Search skills" />
      <p class="shrink-0 text-sm text-muted-foreground">{shown.length} skill{shown.length === 1 ? '' : 's'}</p>
    </div>
    {#if shown.length === 0}
      <p class="py-10 text-center text-sm text-muted-foreground">No skills match this filter.</p>
    {:else}
      <div class="rounded-lg border" role="list" aria-label="Skills">
        {#each shown as k (k.id)}
          <div role="listitem" class="group relative flex flex-col gap-1.5 border-b px-3 py-3 transition-colors last:border-b-0 hover:bg-accent/50 sm:flex-row sm:items-center sm:gap-4">
            <div class="flex min-w-0 flex-1 items-start gap-2.5">
              <Boxes class="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              <div class="min-w-0 space-y-0.5">
                <div class="flex min-w-0 items-baseline gap-2">
                  <a href={href(skillPath(k.id))} class="truncate text-sm font-medium text-inherit no-underline after:absolute after:inset-0">{k.name}</a>
                  <span class="truncate font-mono text-xs text-muted-foreground">{k.slug}</span>
                </div>
                {#if k.description}<p class="truncate text-xs text-muted-foreground" title={k.description}>{k.description}</p>{/if}
              </div>
            </div>
            <span class="w-36 shrink-0 text-xs text-muted-foreground sm:text-right">{k.file_count} file{k.file_count === 1 ? '' : 's'} · {formatBytes(k.size)}</span>
            <span class="w-20 shrink-0 text-xs text-muted-foreground sm:text-right">{k.agents_count} agent{k.agents_count === 1 ? '' : 's'}</span>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<NewSkillDialog bind:open={creating} />
