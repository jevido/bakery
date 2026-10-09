<script lang="ts">
  // Paperclip's AgentSkillsTab and AgentSkillRow
  // (ui/src/pages/agent-skills/; MIT, see NOTICE) for the Agent page's Skills
  // section: "n of m enabled", the save status chip, the search, and the
  // Guild's library in two lists, Enabled on this agent and Available from
  // the library, each Skill with a switch. A change saves the whole set
  // after 300 ms, as Paperclip's autosave does; a failed save puts the
  // switches back as they were saved and shows why. Version pins, releases,
  // detected and connector skills are left out: none exist here.
  import { AlertCircle, CheckCircle2, Loader2, Store } from '@lucide/svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import { ApiError } from './api'
  import { href, skillPath } from './router.svelte'
  import SearchField from './SearchField.svelte'
  import { listAgentSkills, listSkills, syncAgentSkills, type Skill } from './skills'

  let { agentId, editable }: { agentId: number; editable: boolean } = $props()

  let library = $state.raw<Skill[] | null>(null)
  let saved = $state.raw<number[]>([])
  let draft = $state.raw<number[]>([])
  let saving = $state(false)
  let saveError = $state('')
  let search = $state('')
  let timer: ReturnType<typeof setTimeout> | undefined

  // The Agent page is keyed by id, so loading once is enough.
  function load() {
    Promise.all([listSkills(), listAgentSkills(agentId)])
      .then(([all, held]) => {
        library = all
        saved = draft = held.map((k) => k.id)
      })
      .catch((e) => (saveError = e.message))
  }
  load()

  $effect(() => () => clearTimeout(timer))

  const same = (a: number[], b: number[]) => a.length === b.length && a.every((id) => b.includes(id))
  const unsaved = $derived(!same(draft, saved))
  const matches = (k: Skill) => {
    const q = search.trim().toLowerCase()
    return !q || k.name.toLowerCase().includes(q) || k.slug.includes(q) || k.description.toLowerCase().includes(q)
  }
  const enabled = $derived((library ?? []).filter((k) => draft.includes(k.id)))
  const available = $derived((library ?? []).filter((k) => !draft.includes(k.id)))

  function toggle(id: number, on: boolean) {
    draft = on ? [...draft, id] : draft.filter((k) => k !== id)
    saveError = ''
    clearTimeout(timer)
    timer = setTimeout(save, 300)
  }

  async function save() {
    if (saving || same(draft, saved)) return
    const sent = draft
    saving = true
    try {
      saved = (await syncAgentSkills(agentId, sent)).map((k) => k.id)
    } catch (e) {
      draft = saved
      saveError = e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e)
    } finally {
      saving = false
    }
    // A switch changed while saving: send the newer set too.
    if (!saveError && !same(draft, saved)) save()
  }
</script>

{#snippet row(k: Skill, on: boolean)}
  <div class="flex min-h-11 items-center gap-3 border-b border-border px-3 py-2.5 transition-colors last:border-b-0 hover:bg-accent/50" data-skill={k.slug}>
    <a href={href(skillPath(k.id))} class="min-w-0 flex-1 rounded-sm no-underline outline-none focus-visible:ring-2 focus-visible:ring-ring">
      <span class="block truncate text-sm font-medium text-foreground">{k.name}</span>
      <span class="mt-0.5 block truncate text-xs text-muted-foreground">{k.description || k.slug}</span>
    </a>
    <Switch checked={on} disabled={!editable} aria-label={`${on ? 'Disable' : 'Enable'} ${k.name}`} onCheckedChange={(v) => toggle(k.id, v)} />
  </div>
{/snippet}

{#snippet list(title: string, skills: Skill[], on: boolean, empty: string)}
  {@const shown = skills.filter(matches)}
  <section class="overflow-hidden rounded-lg border border-border" aria-label={title}>
    <div class="flex items-center gap-2 bg-muted/50 px-3 py-2">
      <span class="text-xs font-medium text-muted-foreground">{title}</span>
      <span class="text-xs text-muted-foreground/70">{shown.length}</span>
    </div>
    {#each shown as k (k.id)}
      {@render row(k, on)}
    {:else}
      <div class="px-3 py-4 text-xs text-muted-foreground">{empty}</div>
    {/each}
  </section>
{/snippet}

{#if library === null}
  {#if saveError}
    <p class="text-xs text-destructive">{saveError}</p>
  {:else}
    <p class="text-xs text-muted-foreground">Loading…</p>
  {/if}
{:else if library.length === 0}
  <div class="flex flex-col items-center gap-3 rounded-lg border border-dashed border-border px-6 py-10 text-center" data-testid="skills-empty">
    <Store class="size-8 text-muted-foreground/60" />
    <div class="space-y-1">
      <p class="text-sm font-medium text-foreground">No skills in the guild's library</p>
      <p class="text-xs text-muted-foreground">Add skills to the guild, then enable them on this agent.</p>
    </div>
    <Button href={href('/skills')} variant="outline" size="sm"><Store class="size-3.5" />Browse skills</Button>
  </div>
{:else}
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-sm font-medium text-foreground">{draft.length} of {library.length} enabled</span>
      <span data-testid="skills-save-status">
        {#if saving}
          <span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><Loader2 class="size-3.5 animate-spin" />Saving…</span>
        {:else if saveError}
          <span class="inline-flex items-center gap-1.5 text-xs text-destructive"><AlertCircle class="size-3.5" />Couldn’t save</span>
        {:else if unsaved}
          <span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><Loader2 class="size-3.5 animate-spin" />Saving soon…</span>
        {:else}
          <span class="inline-flex items-center gap-1.5 text-xs text-(--status-task-done)"><CheckCircle2 class="size-3.5" />Saved</span>
        {/if}
      </span>
      <div class="ml-auto flex w-full items-center gap-2 sm:w-auto">
        <SearchField bind:value={search} label="Search skills" />
        <Button href={href('/skills')} variant="outline" size="sm" class="shrink-0"><Store class="size-3.5" />Browse skills</Button>
      </div>
    </div>
    {#if saveError}
      <p class="text-xs text-destructive" role="alert">{saveError}</p>
    {/if}
    {@render list('Enabled on this agent', enabled, true, search ? 'No enabled skills match your search.' : 'No skills enabled on this agent yet.')}
    {@render list(
      'Available from the library',
      available,
      false,
      search ? 'No available skills match your search.' : 'Every library skill is enabled on this agent.',
    )}
  </div>
{/if}
