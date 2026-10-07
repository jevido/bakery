<script lang="ts">
  // Paperclip's IssueDetail (ui/src/pages/IssueDetail.tsx; MIT, see NOTICE)
  // as its classic layout draws it, without the task chat: the parent chain,
  // the status, priority, identifier and Project over the title and the
  // Markdown description, both edited in place, the Sub-issues, then the
  // Comments; the properties panel on the right. Each change is one PATCH,
  // and the panel says "Saving..." until it answered. Without manage_work
  // the page only reads. Delete sits in the More actions menu with Add
  // sub-issue, and is The Bakery's own: Paperclip hides an Issue instead.
  // Left out until the Issue has them: agents and runs, checkout, blockers,
  // documents, attachments, work products, approvals and the activity tab.
  import { ChevronRight, Ellipsis, Plus, Trash2 } from '@lucide/svelte'
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import { Button, buttonVariants } from '$lib/components/ui/button'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import CommentThread from '../../lib/CommentThread.svelte'
  import { ago } from '../../lib/format'
  import Identity from '../../lib/Identity.svelte'
  import InlineEditor from '../../lib/InlineEditor.svelte'
  import IssueProperties from '../../lib/IssueProperties.svelte'
  import NewIssueDialog from '../../lib/NewIssueDialog.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import PriorityIcon from '../../lib/PriorityIcon.svelte'
  import { go, href } from '../../lib/router.svelte'
  import { session, type Member } from '../../lib/session.svelte'
  import StatusIcon from '../../lib/StatusIcon.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import NotFound from '../NotFound.svelte'
  import { deleteIssue, getIssue, listGoals, listIssues, updateIssue, type Goal, type Issue, type IssueDetail, type IssueInput } from '../../lib/work'

  /** The Issue's identifier (DEF-12) or id, as the address holds it. */
  let { key }: { key: string } = $props()

  let issue = $state.raw<IssueDetail | null>(null)
  let missing = $state(false)
  let loadError = $state('')
  let saving = $state(0)
  let members = $state.raw<Member[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  let goals = $state.raw<Goal[]>([])
  let issues = $state.raw<Issue[]>([])
  let creating = $state(false)
  let deleting = $state(false)

  const editable = $derived(session.can('manage_work'))

  function load() {
    getIssue(key)
      .then((i) => (issue = i))
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) missing = true
        else loadError = e.message
      })
  }
  load()
  api<{ members: Member[] }>('GET', '/members').then((r) => (members = r.members)).catch(() => {})
  api<{ projects: { id: number; name: string }[] }>('GET', '/projects').then((r) => (projects = r.projects)).catch(() => {})
  listGoals().then((gs) => (goals = gs)).catch(() => {})
  listIssues().then((is) => (issues = is)).catch(() => {})

  $effect(() => breadcrumb.set({ label: 'Issues', href: href('/issues') }, { label: issue?.identifier ?? key }))

  // The parent chain, top first, from the Guild's Issues; the direct parent
  // is on the Issue itself, so it shows before the list has loaded.
  const ancestors = $derived.by(() => {
    if (!issue?.parent) return []
    const chain = [issue.parent]
    const seen = new Set([issue.id, issue.parent.id])
    let next = issues.find((i) => i.id === issue!.parent!.id)?.parent
    while (next && !seen.has(next.id)) {
      chain.unshift(next)
      seen.add(next.id)
      next = issues.find((i) => i.id === next!.id)?.parent
    }
    return chain
  })

  async function save(patch: IssueInput) {
    saving++
    try {
      issue = await updateIssue(issue!.id, patch)
      if (patch.parent_id !== undefined) listIssues().then((is) => (issues = is)).catch(() => {})
    } catch (e) {
      toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      saving--
    }
  }

  async function remove() {
    try {
      await deleteIssue(issue!.id)
      toast.success(`${issue!.identifier} deleted.`)
      go('/issues')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  function refresh() {
    getIssue(issue!.id)
      .then((i) => (issue = i))
      .catch(() => {})
  }
</script>

{#if missing}
  <NotFound title="Issue not found" description="This issue does not exist or you cannot see it." />
{:else if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if issue === null}
  <PageSkeleton />
{:else}
  {@const i = issue}
  <div class="flex flex-col gap-6 lg:flex-row">
    <div class="max-w-3xl min-w-0 flex-1 space-y-6">
      {#if ancestors.length > 0}
        <nav class="flex flex-wrap items-center gap-1 text-xs text-muted-foreground" aria-label="Parent issues">
          {#each ancestors as a, n (a.id)}
            <span class="flex items-center gap-1">
              {#if n > 0}<ChevronRight class="size-3 shrink-0" />{/if}
              <a href={href(`/issues/${a.identifier}`)} class="max-w-50 truncate transition-colors hover:text-foreground" title={a.title}>{a.title}</a>
            </span>
          {/each}
          <ChevronRight class="size-3 shrink-0" />
          <span class="max-w-50 truncate text-foreground/60">{i.title}</span>
        </nav>
      {/if}

      <div class="space-y-3" data-testid="issue-detail-header">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <StatusIcon status={i.status} size="lg" onchange={editable ? (status) => save({ status }) : undefined} />
          <PriorityIcon priority={i.priority} onchange={editable ? (priority) => save({ priority }) : undefined} />
          <span class="shrink-0 font-mono text-sm text-muted-foreground">{i.identifier}</span>
          {#if i.project}
            <a
              href={href(`/project/${i.project.id}`)}
              class="-mx-1 inline-flex min-w-0 items-center gap-1 rounded px-1 py-0.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
            >
              <span class="truncate">{i.project.name}</span>
            </a>
          {:else}
            <span class="-mx-1 inline-flex items-center gap-1 px-1 py-0.5 text-xs text-muted-foreground opacity-50">No project</span>
          {/if}
          {#if i.created_by}
            <span class="inline-flex items-center gap-1 text-xs text-muted-foreground">
              <Identity name={i.created_by.name} size="xs" /> opened {ago(i.created_at)}
            </span>
          {/if}
          {#if editable}
            <div class="ml-auto flex shrink-0 items-center">
              <DropdownMenu.Root>
                <DropdownMenu.Trigger class={buttonVariants({ variant: 'ghost', size: 'icon-xs' })} aria-label="More issue actions" title="More issue actions">
                  <Ellipsis class="size-4" />
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="end" class="w-52">
                  <DropdownMenu.Item onSelect={() => (creating = true)}><Plus />Add sub-issue</DropdownMenu.Item>
                  <DropdownMenu.Item variant="destructive" onSelect={() => (deleting = true)}><Trash2 />Delete issue</DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Root>
            </div>
          {/if}
        </div>

        <InlineEditor label="Title" value={i.title} {editable} as="h2" class="text-xl font-bold" onsave={(title) => save({ title })} />
        <InlineEditor
          label="Description"
          value={i.description}
          {editable}
          multiline
          placeholder="Add a description..."
          class="text-sm leading-7 text-foreground"
          onsave={(description) => save({ description })}
        />
      </div>

      <section class="space-y-3" aria-label="Sub-issues">
        <div class="flex items-center justify-between gap-2">
          <h3 class="text-sm font-medium text-muted-foreground">Sub-issues</h3>
          {#if editable}
            <Button variant="outline" size="sm" class="shadow-none" onclick={() => (creating = true)}><Plus class="size-3.5" />Add sub-issue</Button>
          {/if}
        </div>
        {#if i.children.length === 0}
          <p class="text-sm text-muted-foreground">No sub-issues.</p>
        {:else}
          <div class="rounded-lg border">
            {#each i.children as child (child.id)}
              <a
                href={href(`/issues/${child.identifier}`)}
                data-issue={child.identifier}
                class="flex min-w-0 items-center gap-2 border-b px-3 py-2 text-sm text-inherit no-underline transition-colors last:border-b-0 hover:bg-accent/50"
              >
                <StatusIcon status={child.status} />
                <PriorityIcon priority={child.priority} class="ml-1" />
                <span class={['min-w-0 flex-1 truncate', (child.status === 'done' || child.status === 'cancelled') && 'text-muted-foreground']}>{child.title}</span>
                {#if child.assignee}<Identity name={child.assignee.name} size="sm" />{/if}
                <span class="w-20 shrink-0 text-right font-mono text-xs text-muted-foreground">{child.identifier}</span>
              </a>
            {/each}
          </div>
        {/if}
      </section>

      <CommentThread issue={i.id} {editable} onchange={refresh} />
    </div>

    <aside class="w-full shrink-0 border-t pt-4 lg:w-80 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-6" aria-label="Properties">
      <div class="mb-2 flex h-6 items-center justify-between">
        <h3 class="text-sm font-medium">Properties</h3>
        {#if saving > 0}<span class="text-xs text-muted-foreground" role="status">Saving...</span>{/if}
      </div>
      <IssueProperties issue={i} {members} {projects} {goals} {issues} {editable} onsave={save} />
    </aside>
  </div>

  <AlertDialog.Root open={deleting} onOpenChange={(open) => (deleting = open)}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>Delete {i.identifier}?</AlertDialog.Title>
        <AlertDialog.Description>
          "{i.title}" and its comments are deleted for good. Its sub-issues stay, without a parent.
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
        <AlertDialog.Action class={buttonVariants({ variant: 'destructive' })} data-testid="confirm-delete-issue" onclick={remove}>Delete</AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <NewIssueDialog bind:open={creating} parentId={i.id} projectId={i.project?.id ?? null} goalId={i.goal?.id ?? null} oncreated={refresh} />
{/if}
