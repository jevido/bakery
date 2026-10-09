<script lang="ts">
  // Paperclip's Skill page (SkillDetailPage, SkillTree and SkillPane in
  // ui/src/pages/CompanySkills.tsx; MIT, see NOTICE): the Skill's name, slug
  // and description over the Overview, Files and Agents tabs. Overview
  // renders SKILL.md with its frontmatter as a key/value block; Files is the
  // tree beside the open file, viewed as Markdown or code and, for a Member
  // with manage_skills, edited, added and deleted; Agents lists the Agents
  // that have the Skill. A Skill file is edited whole, frontmatter and all,
  // where Paperclip edits a Markdown file's body under a separate
  // frontmatter: a Skill's name and description live only in SKILL.md's
  // frontmatter, so that is where they are changed. Left out, as the first
  // Skills slice leaves them out: Versions and their diff, the source,
  // stars, forks, categories, sharing, folders, update checks, test runs,
  // Open in Studio, and attaching Agents from here (the Agent page's Skills
  // section does that).
  import { Code2, Eye, FileCode2, FileText, Folder, FolderOpen, Pencil, Plus, Save, Trash2, Users } from '@lucide/svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import { agentStatusText, agentStatusTones } from '@bakery/ui/agentStatus'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button } from '@bakery/ui/components/ui/button'
  import { Input } from '@bakery/ui/components/ui/input'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import Markdown from '@bakery/ui/Markdown.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { ago } from '../../lib/format'
  import MarkdownField from '../../lib/MarkdownField.svelte'
  import PageHeader from '../../lib/PageHeader.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { go, href, skillPath, type SkillTab } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import {
    buildTree,
    deleteSkill,
    deleteSkillFile,
    formatBytes,
    frontmatterFields,
    getSkill,
    getSkillFile,
    parentFolders,
    skillMarkdown,
    splitFrontmatter,
    writeSkillFile,
    type SkillDetail,
    type SkillFile,
    type SkillTreeNode,
  } from '../../lib/skills'
  import FieldError from '../../lib/ui/FieldError.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import Textarea from '../../lib/ui/Textarea.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { id, tab, path }: { id: number; tab: SkillTab; path: string | null } = $props()

  let skill = $state.raw<SkillDetail | null>(null)
  let loadError = $state('')
  let file = $state.raw<SkillFile | null>(null)
  let fileError = $state('')
  let fileLoading = $state(false)
  let viewMode = $state<'preview' | 'code'>('preview')
  let editMode = $state(false)
  let draft = $state('')
  let saving = $state(false)
  let saveError = $state('')
  /** A file Add file opened in the editor that is not saved yet. */
  let pending = $state<string | null>(null)
  let adding = $state(false)
  let newPath = $state('')
  let deletingFile = $state(false)
  let deleting = $state(false)
  let refusal = $state<{ message: string; agents: { id: number; name: string }[] } | null>(null)
  const expanded = new SvelteSet<string>()

  const canManage = $derived(session.can('manage_skills'))
  const message = (e: unknown) => (e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
  const isMarkdown = (p: string) => p.toLowerCase().endsWith('.md')

  /** The file open on Files; Overview always shows SKILL.md. */
  const selected = $derived(tab === 'files' ? (path ?? skillMarkdown) : skillMarkdown)
  const tree = $derived(skill ? buildTree(skill.files) : [])
  const split = $derived(file && file.encoding === 'utf8' && isMarkdown(file.path) ? splitFrontmatter(file.content) : null)

  $effect(() => {
    getSkill(id)
      .then((k) => (skill = k))
      .catch((e) => (loadError = message(e)))
  })
  $effect(() => breadcrumb.set({ label: 'Skills', href: href('/skills') }, { label: skill?.name ?? 'Skill' }))

  // A file chosen in the tree, or the tab changed, ends an edit and loads it.
  let loading = 0
  function loadFile(p: string) {
    const mine = ++loading
    fileLoading = true
    fileError = ''
    getSkillFile(id, p)
      .then((f) => mine === loading && (file = f))
      .catch((e) => {
        if (mine !== loading) return
        file = null
        fileError = e instanceof ApiError && e.status === 404 ? `This skill has no file ${p}.` : message(e)
      })
      .finally(() => mine === loading && (fileLoading = false))
  }
  $effect(() => {
    const p = selected
    if (tab === 'agents') return
    editMode = false
    pending = null
    saveError = ''
    viewMode = 'preview'
    for (const folder of parentFolders(p)) expanded.add(folder)
    loadFile(p)
  })

  function startEdit() {
    if (!file) return
    draft = file.content
    saveError = ''
    editMode = true
  }

  function cancelEdit() {
    editMode = false
    saveError = ''
    if (pending) {
      pending = null
      loadFile(selected)
    }
  }

  async function save() {
    if (!file || saving) return
    saving = true
    saveError = ''
    try {
      skill = await writeSkillFile(id, { path: file.path, content: draft, encoding: 'utf8', executable: file.executable })
      toast.success('File saved', file.path)
      if (pending) {
        const p = pending
        pending = null
        if (p === selected) loadFile(p)
        else go(skillPath(id, 'files', p))
      } else {
        editMode = false
        loadFile(file.path)
      }
    } catch (e) {
      saveError = message(e)
    } finally {
      saving = false
    }
  }

  function addFile() {
    const p = newPath.trim()
    if (!p) return
    adding = false
    newPath = ''
    if (skill?.files.some((f) => f.path === p)) {
      go(skillPath(id, 'files', p))
      return
    }
    for (const folder of parentFolders(p)) expanded.add(folder)
    pending = p
    file = { path: p, content: '', encoding: 'utf8', executable: false, size: 0 }
    fileError = ''
    draft = ''
    saveError = ''
    editMode = true
  }

  async function removeFile() {
    if (!file) return
    deletingFile = false
    try {
      skill = await deleteSkillFile(id, file.path)
      toast.success('File deleted', file.path)
      go(skillPath(id, 'files'))
    } catch (e) {
      toast.error('File not deleted', message(e))
    }
  }

  async function remove() {
    if (!skill) return
    try {
      await deleteSkill(id)
      deleting = false
      toast.success('Skill deleted', skill.name)
      go('/skills')
    } catch (e) {
      if (e instanceof ApiError && Array.isArray(e.body.agents)) {
        refusal = { message: e.message, agents: e.body.agents as { id: number; name: string }[] }
      } else {
        deleting = false
        toast.error('Skill not deleted', message(e))
      }
    }
  }

  const tabs: { value: SkillTab; label: string; icon: typeof FileText }[] = [
    { value: 'overview', label: 'Overview', icon: FileText },
    { value: 'files', label: 'Files', icon: FolderOpen },
    { value: 'agents', label: 'Agents', icon: Users },
  ]
</script>

{#snippet frontmatterBlock(frontmatter: string)}
  <dl class="grid gap-x-4 gap-y-1 rounded-md border border-border bg-muted/30 px-3 py-2 font-mono text-xs sm:grid-cols-[auto_1fr]" aria-label="Frontmatter">
    {#each frontmatterFields(frontmatter) as [key, value] (key)}
      <dt class="text-muted-foreground">{key}</dt>
      <dd class="min-w-0 [overflow-wrap:anywhere]">{value}</dd>
    {/each}
  </dl>
{/snippet}

{#snippet treeRows(nodes: SkillTreeNode[], depth: number)}
  {#each nodes as node (node.path)}
    {#if node.kind === 'dir'}
      {@const open = expanded.has(node.path)}
      <button
        type="button"
        class="flex h-8 w-full items-center gap-2 pr-3 text-left text-sm text-muted-foreground hover:bg-accent/30 hover:text-foreground"
        style:padding-left="{8 + depth * 14}px"
        aria-expanded={open}
        onclick={() => (open ? expanded.delete(node.path) : expanded.add(node.path))}
      >
        {#if open}<FolderOpen class="size-3.5 shrink-0" />{:else}<Folder class="size-3.5 shrink-0" />{/if}
        <span class="truncate">{node.name}</span>
      </button>
      {#if open}{@render treeRows(node.children, depth + 1)}{/if}
    {:else}
      {@const FileIcon = node.file?.kind === 'script' ? FileCode2 : FileText}
      <a
        href={href(skillPath(id, 'files', node.path))}
        class={['flex h-8 w-full items-center gap-2 pr-3 text-sm no-underline hover:bg-accent/30 hover:text-foreground', node.path === (pending ?? selected) ? 'bg-accent/40 text-foreground' : 'text-muted-foreground']}
        style:padding-left="{8 + depth * 14}px"
        aria-current={node.path === (pending ?? selected) ? 'page' : undefined}
      >
        <FileIcon class="size-3.5 shrink-0" />
        <span class="truncate">{node.name}</span>
      </a>
    {/if}
  {/each}
{/snippet}

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !skill}
  <PageSkeleton />
{:else}
  {@const k = skill}
  <div class="chrome space-y-6">
    <PageHeader title={k.name} description={k.description || undefined}>
      {#snippet meta()}
        <span class="font-mono">{k.slug}</span>
        <span>· {k.file_count} file{k.file_count === 1 ? '' : 's'} · {formatBytes(k.size)}</span>
        <span>· {k.agents_count} agent{k.agents_count === 1 ? '' : 's'}</span>
      {/snippet}
      {#snippet actions()}
        {#if canManage}
          <Button variant="outline" size="sm" onclick={() => ((refusal = null), (deleting = true))}><Trash2 class="size-3.5" />Delete</Button>
        {/if}
      {/snippet}
    </PageHeader>

    <Tabs.Root value={tab} onValueChange={(v) => go(skillPath(id, v as SkillTab))}>
      <Tabs.List variant="line" class="justify-start" aria-label="Skill tabs">
        {#each tabs as t (t.value)}
          <Tabs.Trigger value={t.value}><t.icon class="size-3.5" />{t.label}</Tabs.Trigger>
        {/each}
      </Tabs.List>
    </Tabs.Root>

    {#if tab === 'overview'}
      <div class="space-y-6">
        <section class="space-y-3" aria-label="About">
          <h2 class="text-sm font-medium">About</h2>
          {#if fileLoading && !file}
            <PageSkeleton />
          {:else if fileError}
            <p class="text-sm text-destructive">{fileError}</p>
          {:else if split}
            {#if split.frontmatter}{@render frontmatterBlock(split.frontmatter)}{/if}
            <div data-testid="skill-overview">
              {#if split.body.trim()}
                <Markdown source={split.body} />
              {:else}
                <p class="text-sm text-muted-foreground">{k.description || 'No overview yet.'}</p>
              {/if}
            </div>
          {/if}
        </section>
        <section class="grid min-w-0 gap-3 text-sm sm:grid-cols-2">
          <div class="min-w-0 border-b border-border py-2">
            <div class="text-xs text-muted-foreground">Slug</div>
            <div class="mt-1 truncate font-mono">{k.slug}</div>
          </div>
          <div class="min-w-0 border-b border-border py-2">
            <div class="text-xs text-muted-foreground">Files</div>
            <div class="mt-1">{k.file_count} · {formatBytes(k.size)}</div>
          </div>
          <div class="min-w-0 border-b border-border py-2">
            <div class="text-xs text-muted-foreground">Created</div>
            <div class="mt-1" title={new Date(k.created_at).toLocaleString()}>{ago(k.created_at)}{k.created_by ? ` by ${k.created_by.name}` : ''}</div>
          </div>
          <div class="min-w-0 border-b border-border py-2">
            <div class="text-xs text-muted-foreground">Updated</div>
            <div class="mt-1" title={new Date(k.updated_at).toLocaleString()}>{ago(k.updated_at)}</div>
          </div>
        </section>
      </div>
    {:else if tab === 'files'}
      <div class="grid min-h-140 gap-0 lg:grid-cols-[16rem_1fr]">
        <aside class="border-b border-border pb-3 lg:border-r lg:border-b-0 lg:pr-3 lg:pb-0">
          <div class="mb-2 flex items-center justify-between gap-2">
            <span class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Files</span>
            {#if canManage}
              <Button variant="ghost" size="xs" onclick={() => (adding = true)}><Plus class="size-3" />Add file</Button>
            {/if}
          </div>
          <nav aria-label="Skill files">
            {@render treeRows(tree, 0)}
            {#if pending && !k.files.some((f) => f.path === pending)}
              <span class="flex h-8 items-center gap-2 bg-accent/40 pr-3 pl-2 text-sm text-foreground italic">
                <FileText class="size-3.5 shrink-0" />
                <span class="truncate">{pending}</span>
              </span>
            {/if}
          </nav>
        </aside>
        <section class="min-w-0 pt-3 lg:pt-0 lg:pl-5" aria-label="File">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-3 border-b border-border pb-3">
            <div class="min-w-0 truncate font-mono text-sm" data-testid="skill-file-path">{file?.path ?? selected}</div>
            <div class="flex items-center gap-2">
              {#if file && file.encoding === 'utf8' && isMarkdown(file.path) && !editMode}
                <div class="flex items-center border border-border" role="group" aria-label="View as">
                  <button type="button" class={['px-3 py-1.5 text-sm', viewMode === 'preview' ? 'text-foreground' : 'text-muted-foreground']} aria-pressed={viewMode === 'preview'} onclick={() => (viewMode = 'preview')}>
                    <span class="flex items-center gap-1.5"><Eye class="size-3.5" />View</span>
                  </button>
                  <button type="button" class={['border-l border-border px-3 py-1.5 text-sm', viewMode === 'code' ? 'text-foreground' : 'text-muted-foreground']} aria-pressed={viewMode === 'code'} onclick={() => (viewMode = 'code')}>
                    <span class="flex items-center gap-1.5"><Code2 class="size-3.5" />Code</span>
                  </button>
                </div>
              {/if}
              {#if canManage && file && file.encoding === 'utf8'}
                {#if editMode}
                  <Button variant="ghost" size="sm" disabled={saving} onclick={cancelEdit}>Cancel</Button>
                  <Button size="sm" disabled={saving} onclick={save}><Save class="size-3.5" />{saving ? 'Saving...' : 'Save'}</Button>
                {:else}
                  <Button variant="ghost" size="sm" onclick={startEdit}><Pencil class="size-3.5" />Edit</Button>
                {/if}
              {/if}
              {#if canManage && file && !editMode && file.path !== skillMarkdown}
                <Button variant="ghost" size="sm" onclick={() => (deletingFile = true)}><Trash2 class="size-3.5" />Delete file</Button>
              {/if}
            </div>
          </div>
          <FieldError error={saveError} />
          {#if fileLoading && !editMode}
            <PageSkeleton />
          {:else if fileError}
            <p class="text-sm text-destructive">{fileError}</p>
          {:else if !file}
            <p class="text-sm text-muted-foreground">Select a file to inspect.</p>
          {:else if file.encoding === 'base64'}
            <div class="rounded-md border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
              <p class="font-medium text-foreground">Binary file</p>
              <p>{formatBytes(file.size)}</p>
            </div>
          {:else if editMode}
            {#if isMarkdown(file.path)}
              <MarkdownField bind:value={draft} label="Content" placeholder="Write Markdown..." rows={22} onsubmit={save} />
            {:else}
              <Textarea bind:value={draft} rows={22} class="font-mono text-sm" aria-label="Content" />
            {/if}
          {:else if split && viewMode === 'preview'}
            {#if split.frontmatter}<div class="mb-4">{@render frontmatterBlock(split.frontmatter)}</div>{/if}
            <Markdown source={split.body} />
          {:else}
            <pre class="overflow-x-auto font-mono text-sm wrap-break-word whitespace-pre-wrap text-foreground"><code>{file.content}</code></pre>
          {/if}
        </section>
      </div>
    {:else}
      <div class="space-y-3">
        <p class="text-sm text-muted-foreground">{k.agents.length} agent{k.agents.length === 1 ? '' : 's'} attached</p>
        {#if k.agents.length === 0}
          <div class="rounded-md border border-dashed border-border py-8 text-center text-sm text-muted-foreground">
            No agents are using this skill yet. Switch it on in the Skills section of an agent's page.
          </div>
        {:else}
          <ul class="border-y border-border" aria-label="Agents with this skill">
            {#each k.agents as a (a.id)}
              <li class="flex items-center gap-3 border-b border-border py-3 text-sm last:border-b-0">
                <AgentIcon icon={a.icon} class="size-4 shrink-0 text-muted-foreground" />
                <a href={href(`/agents/${a.id}`)} class="min-w-0 flex-1 truncate font-medium text-inherit no-underline hover:underline">{a.name}</a>
                <StatusBadge type={agentStatusTones[a.status]} label={agentStatusText(a)} />
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}
  </div>

  <Modal bind:open={adding} variant="none" title="Add file" subtitle="A path inside the skill, such as templates/notes.md. It opens empty in the editor." onclose={() => (newPath = '')}>
    <label class="grid gap-1.5 text-sm font-medium">
      Path
      <!-- svelte-ignore a11y_autofocus -->
      <Input
        autofocus
        class="font-mono"
        placeholder="templates/notes.md"
        bind:value={newPath}
        onkeydown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            addFile()
          }
        }}
      />
    </label>
    {#snippet footer()}
      <Button disabled={!newPath.trim()} onclick={addFile}>Add file</Button>
    {/snippet}
  </Modal>

  <AlertDialog.Root bind:open={deletingFile}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>Delete file?</AlertDialog.Title>
        <AlertDialog.Description>"{file?.path}" leaves the skill. Agents that have it no longer get it on their next run.</AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
        <AlertDialog.Action onclick={removeFile}>Delete file</AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <AlertDialog.Root bind:open={deleting}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>Delete skill?</AlertDialog.Title>
        <AlertDialog.Description>"{k.name}" and all its files are deleted. This cannot be undone.</AlertDialog.Description>
      </AlertDialog.Header>
      {#if refusal}
        <div class="space-y-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm" role="alert">
          <p>{refusal.message}</p>
          <ul class="list-inside list-disc">
            {#each refusal.agents as a (a.id)}
              <li><a href={href(`/agents/${a.id}`)} class="font-medium hover:underline">{a.name}</a></li>
            {/each}
          </ul>
        </div>
      {/if}
      <AlertDialog.Footer>
        <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
        {#if !refusal}
          <Button variant="destructive" onclick={remove}>Delete skill</Button>
        {/if}
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
{/if}
