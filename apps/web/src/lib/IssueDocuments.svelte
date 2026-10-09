<script lang="ts">
  // Paperclip's IssueDocumentsSection with its DocumentFrameHeader
  // (ui/src/components/IssueDocumentsSection.tsx; MIT, see NOTICE): an
  // Issue's documents as bordered cards, each with its title, key, a "rev N"
  // menu of its Revisions and when it was updated, the Markdown below. Edit
  // in place saves a new Revision against the one shown (its Base revision);
  // when someone saved first the draft stays, with Reload and Compare. An
  // older Revision opens read-only, with Compare with current and Restore.
  // Saving is explicit here, with an optional change summary: Paperclip
  // autosaves on blur. Left out with what the Issue does not have yet: locks,
  // annotations, folding, feedback votes, agents and plan approvals.
  import { Copy, Download, Ellipsis, FilePenLine, GitCompare, Plus, Trash2, ChevronDown, Check } from '@lucide/svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Button, buttonVariants } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import { Input } from '@bakery/ui/components/ui/input'
  import { ApiError } from './api'
  import DocumentDiff from './DocumentDiff.svelte'
  import { ago, formatDate } from './format'
  import Actor from './Actor.svelte'
  import Markdown from '@bakery/ui/Markdown.svelte'
  import MarkdownField from './MarkdownField.svelte'
  import { toast } from './ui/toast.svelte'
  import {
    deleteIssueDocument,
    documentKeyPattern,
    getIssueDocument,
    listDocumentRevisions,
    listIssueDocuments,
    restoreDocumentRevision,
    saveIssueDocument,
    type DocumentRevision,
    type IssueDocument,
  } from './work'

  let {
    issue,
    editable,
    onchange,
  }: {
    /** The Issue's id or identifier. */
    issue: number | string
    /** Whether the Member may change documents (manage_work). */
    editable: boolean
    /** After a document was saved, restored or deleted, so the Issue shows its new updated time. */
    onchange?: () => void
  } = $props()

  /** A document being written: a new one (no base) or an edit of the Revision shown. */
  type Draft = { key: string; title: string; body: string; summary: string; base: number | null }

  let documents = $state.raw<IssueDocument[] | null>(null)
  let draft = $state<Draft | null>(null)
  let saving = $state(false)
  /** The newest document when saving the draft found someone had saved first. */
  let conflict = $state.raw<IssueDocument | null>(null)
  let revisions = $state.raw<Record<string, DocumentRevision[]>>({})
  /** The older Revision shown read-only, per document. */
  let viewing = $state.raw<{ key: string; revision: DocumentRevision } | null>(null)
  let diff = $state.raw<{ key: string; old: { label: string; body: string }; new: { label: string; body: string } } | null>(null)
  let diffOpen = $state(false)
  let deleting = $state.raw<IssueDocument | null>(null)
  let copied = $state<string | null>(null)

  const creating = $derived(draft !== null && draft.base === null)
  const keyError = $derived.by(() => {
    if (!creating || !draft!.key) return ''
    if (!documentKeyPattern.test(draft!.key)) return 'Use lowercase letters, digits, _ and -, starting with a letter or digit.'
    if (documents?.some((d) => d.key === draft!.key)) return 'This issue already has a document with that key.'
    return ''
  })

  function load() {
    listIssueDocuments(issue)
      .then((ds) => (documents = ds))
      .catch((e) => toast.error(e instanceof Error ? e.message : String(e)))
  }
  load()

  function failed(e: unknown) {
    toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
  }

  function replace(doc: IssueDocument) {
    const rest = (documents ?? []).filter((d) => d.key !== doc.key)
    documents = [...rest, doc].sort((a, b) => a.key.localeCompare(b.key))
    const { [doc.key]: _, ...others } = revisions
    revisions = others
  }

  function startNew() {
    viewing = null
    conflict = null
    draft = { key: '', title: '', body: '', summary: '', base: null }
  }

  function startEdit(doc: IssueDocument) {
    viewing = null
    conflict = null
    draft = { key: doc.key, title: doc.title, body: doc.body, summary: '', base: doc.latest_revision_id }
  }

  function cancel() {
    draft = null
    conflict = null
  }

  async function save() {
    if (!draft || saving || keyError || !draft.key || !draft.body.trim()) return
    saving = true
    try {
      const doc = await saveIssueDocument(issue, draft.key, {
        title: draft.title.trim(),
        body: draft.body,
        ...(draft.summary.trim() ? { change_summary: draft.summary.trim() } : {}),
        ...(draft.base !== null ? { base_revision_id: draft.base } : {}),
      })
      replace(doc)
      draft = null
      conflict = null
      onchange?.()
    } catch (e) {
      if (e instanceof ApiError && e.status === 409 && draft?.base !== null) {
        conflict = await getIssueDocument(issue, draft!.key).catch(() => null)
      } else failed(e)
    } finally {
      saving = false
    }
  }

  /** Drops the draft and shows the newest Revision. */
  function reload() {
    if (conflict) replace(conflict)
    draft = null
    conflict = null
  }

  function loadRevisions(key: string) {
    listDocumentRevisions(issue, key)
      .then((rs) => (revisions = { ...revisions, [key]: rs }))
      .catch(failed)
  }

  async function restore(key: string, revision: DocumentRevision) {
    try {
      const doc = await restoreDocumentRevision(issue, key, revision.id)
      replace(doc)
      viewing = null
      toast.success(`Restored revision ${revision.number} as revision ${doc.latest_revision_number}.`)
      onchange?.()
    } catch (e) {
      failed(e)
    }
  }

  async function remove(doc: IssueDocument) {
    try {
      await deleteIssueDocument(issue, doc.key)
      documents = (documents ?? []).filter((d) => d.key !== doc.key)
      if (viewing?.key === doc.key) viewing = null
      onchange?.()
    } catch (e) {
      failed(e)
    }
  }

  function compare(key: string, old: { label: string; body: string }, next: { label: string; body: string }) {
    diff = { key, old, new: next }
    diffOpen = true
  }

  async function copy(doc: IssueDocument) {
    try {
      await navigator.clipboard.writeText(doc.body)
      copied = doc.key
      setTimeout(() => copied === doc.key && (copied = null), 1500)
    } catch {
      toast.error('Could not copy to the clipboard.')
    }
  }

  function download(doc: IssueDocument) {
    const url = URL.createObjectURL(new Blob([doc.body], { type: 'text/markdown' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `${doc.key}.md`
    a.click()
    URL.revokeObjectURL(url)
  }
</script>

<section class="space-y-3" aria-label="Documents">
  {#if documents !== null && (documents.length > 0 || creating || editable)}
    <div class="flex min-w-0 flex-wrap items-center gap-2">
      <h3 class="w-full shrink-0 text-sm font-medium text-muted-foreground sm:w-auto">Documents</h3>
      {#if editable}
        <div class="flex min-w-0 flex-wrap items-center gap-2 sm:ml-auto">
          <Button variant="outline" size="sm" class="shrink-0 shadow-none" onclick={startNew} disabled={creating}><Plus class="size-3.5" />New document</Button>
        </div>
      {/if}
    </div>
  {/if}

  {#if creating && draft}
    <div class="space-y-3 rounded-lg border border-border bg-accent/10 p-3" data-testid="new-document">
      <!-- svelte-ignore a11y_autofocus -->
      <Input
        autofocus
        value={draft.key}
        oninput={(e: Event) => (draft!.key = (e.currentTarget as HTMLInputElement).value.toLowerCase())}
        placeholder="Document key"
        aria-label="Document key"
      />
      {#if keyError}<p class="text-xs text-destructive">{keyError}</p>{/if}
      <Input bind:value={draft.title} placeholder="Optional title" aria-label="Document title" />
      <MarkdownField bind:value={draft.body} label="Document body" placeholder="Markdown body" rows={8} onsubmit={save} />
      <div class="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onclick={cancel}>Cancel</Button>
        <Button size="sm" disabled={!draft.key || !!keyError || !draft.body.trim() || saving} onclick={save}>{saving ? 'Saving...' : 'Create document'}</Button>
      </div>
    </div>
  {/if}

  {#each documents ?? [] as doc (doc.id)}
    {@const preview = viewing?.key === doc.key ? viewing.revision : null}
    {@const editing = draft !== null && draft.base !== null && draft.key === doc.key}
    {@const shown = preview ?? { title: doc.title, body: doc.body, number: doc.latest_revision_number, created_at: doc.updated_at }}
    <div id="document-{doc.key}" data-document={doc.key} class="rounded-lg border border-border p-3">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-2">
            {#if shown.title}<span class="truncate text-sm font-semibold text-foreground">{shown.title}</span>{/if}
            <Badge variant="outline" class="border-border font-mono text-[10px] tracking-widest text-muted-foreground uppercase">{doc.key}</Badge>
            <DropdownMenu.Root onOpenChange={(open) => open && loadRevisions(doc.key)}>
              <DropdownMenu.Trigger
                class={[
                  buttonVariants({ variant: 'ghost', size: 'sm' }),
                  'h-auto gap-1 px-1.5 py-0 text-[11px] font-normal text-muted-foreground hover:text-foreground',
                  preview && 'text-amber-700 hover:text-amber-800 dark:text-amber-300 dark:hover:text-amber-200',
                ]}
                aria-label="Revision history of {doc.key}"
                title="Revision history"
              >
                rev {shown.number}<ChevronDown class="size-3" />
              </DropdownMenu.Trigger>
              <DropdownMenu.Content align="start" class="w-72">
                <DropdownMenu.Label>Revision history</DropdownMenu.Label>
                {#if !revisions[doc.key]}
                  <DropdownMenu.Item disabled>Loading revisions...</DropdownMenu.Item>
                {:else}
                  {#each revisions[doc.key] as r (r.id)}
                    {@const current = r.id === doc.latest_revision_id}
                    <DropdownMenu.Item
                      class="items-start"
                      data-revision={r.number}
                      onSelect={() => {
                        if (editing) cancel()
                        viewing = current ? null : { key: doc.key, revision: r }
                      }}
                    >
                      <span class="flex size-3.5 shrink-0 items-center pt-0.5">{#if r.number === shown.number}<Check class="size-3.5" />{/if}</span>
                      <div class="flex min-w-0 flex-col">
                        <div class="flex items-center gap-2">
                          <span class="font-medium">rev {r.number}</span>
                          {#if current}<Badge variant="outline" class="border-border px-1.5 text-[10px] tracking-widest text-muted-foreground uppercase">Current</Badge>{/if}
                        </div>
                        <div class="mt-1 flex min-w-0 items-center gap-1.5 text-[11px] text-muted-foreground">
                          <span class="truncate" title={formatDate(r.created_at)}>{ago(r.created_at)} • {r.created_by_agent?.name ?? r.created_by?.name ?? 'Someone'}</span>
                        </div>
                        {#if r.change_summary}<span class="mt-0.5 truncate text-[11px] text-muted-foreground">{r.change_summary}</span>{/if}
                      </div>
                    </DropdownMenu.Item>
                  {/each}
                {/if}
              </DropdownMenu.Content>
            </DropdownMenu.Root>
            <a href="#document-{doc.key}" onclick={(e) => e.preventDefault()} class="truncate text-[11px] text-muted-foreground no-underline transition-colors hover:text-foreground hover:underline" title={formatDate(shown.created_at)}>
              updated {ago(shown.created_at)}
            </a>
          </div>
          {#if (doc.updated_by || doc.updated_by_agent) && !preview}
            <div class="mt-1 text-[11px] text-muted-foreground"><Actor member={doc.updated_by} agent={doc.updated_by_agent} size="xs" /></div>
          {/if}
        </div>
        <div class="flex shrink-0 items-center gap-1">
          <Button variant="ghost" size="icon-xs" class="text-muted-foreground" title="Copy document" aria-label="Copy document" onclick={() => copy(doc)}>
            {#if copied === doc.key}<Check />{:else}<Copy />{/if}
          </Button>
          <DropdownMenu.Root>
            <DropdownMenu.Trigger class={[buttonVariants({ variant: 'ghost', size: 'icon-xs' }), 'text-muted-foreground']} aria-label="Document actions" title="Document actions">
              <Ellipsis class="size-3.5" />
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="end" class="w-48">
              {#if editable}<DropdownMenu.Item onSelect={() => startEdit(doc)}><FilePenLine />Edit document</DropdownMenu.Item>{/if}
              <DropdownMenu.Item onSelect={() => download(doc)}><Download />Download document</DropdownMenu.Item>
              {#if doc.latest_revision_number > 1}
                <DropdownMenu.Item
                  onSelect={async () => {
                    const rs = revisions[doc.key] ?? (await listDocumentRevisions(issue, doc.key).catch(() => []))
                    const prev = rs.find((r) => r.number === doc.latest_revision_number - 1)
                    if (prev) compare(doc.key, { label: `rev ${prev.number}`, body: prev.body }, { label: `rev ${doc.latest_revision_number}`, body: doc.body })
                  }}><GitCompare />View diff</DropdownMenu.Item
                >
              {/if}
              {#if editable}
                <DropdownMenu.Separator />
                <DropdownMenu.Item variant="destructive" onSelect={() => (deleting = doc)}><Trash2 />Delete document</DropdownMenu.Item>
              {/if}
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        </div>
      </div>

      <div class="mt-3 space-y-3">
        {#if preview}
          <div class="rounded-md border border-amber-500/30 bg-amber-500/5 px-3 py-3" role="status" data-testid="revision-preview">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div class="space-y-1">
                <p class="text-sm font-medium text-amber-800 dark:text-amber-200">Viewing revision {preview.number}</p>
                <p class="text-xs text-muted-foreground">This is an older revision. Restoring it saves it as a new revision; the history keeps every one.</p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onclick={() => compare(doc.key, { label: `rev ${preview.number}`, body: preview.body }, { label: `rev ${doc.latest_revision_number} (current)`, body: doc.body })}
                  >Compare with current</Button
                >
                <Button variant="outline" size="sm" onclick={() => (viewing = null)}>Back to current</Button>
                {#if editable}<Button size="sm" onclick={() => restore(doc.key, preview)}>Restore this revision</Button>{/if}
              </div>
            </div>
          </div>
        {/if}

        {#if editing && draft}
          {#if conflict}
            <div class="rounded-md border border-amber-500/30 bg-amber-500/5 px-3 py-3" role="alert" data-testid="document-conflict">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <p class="text-sm font-medium text-amber-800 dark:text-amber-200">Someone saved a newer revision</p>
                  <p class="text-xs text-muted-foreground">
                    Revision {conflict.latest_revision_number} was saved while you were editing. Your text is still here and was not saved.
                  </p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onclick={() => compare(doc.key, { label: `rev ${conflict!.latest_revision_number}`, body: conflict!.body }, { label: 'Your text', body: draft!.body })}
                    >Compare</Button
                  >
                  <Button size="sm" onclick={reload}>Reload</Button>
                </div>
              </div>
            </div>
          {/if}
          <Input bind:value={draft.title} placeholder="Optional title" aria-label="Document title" />
          <MarkdownField bind:value={draft.body} label="Document body" rows={10} autofocus onsubmit={save} />
          <Input bind:value={draft.summary} placeholder="What changed (optional)" aria-label="Change summary" />
          <div class="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onclick={cancel}>Cancel</Button>
            <Button size="sm" disabled={!draft.body.trim() || saving || conflict !== null} onclick={save}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        {:else}
          <Markdown source={shown.body} class="text-sm" />
        {/if}
      </div>
    </div>
  {/each}
</section>

{#if diff}
  {@const key = diff.key}
  <DocumentDiff bind:open={diffOpen} old={diff.old} new={diff.new}>
    {#snippet heading()}Diff — <span class="font-mono text-sm">{key}</span>{/snippet}
  </DocumentDiff>
{/if}

<AlertDialog.Root open={deleting !== null} onOpenChange={(open) => !open && (deleting = null)}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Delete the {deleting?.key} document?</AlertDialog.Title>
      <AlertDialog.Description>The document and every revision of it are deleted for good.</AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action
        class={buttonVariants({ variant: 'destructive' })}
        data-testid="confirm-delete-document"
        onclick={() => {
          const d = deleting
          deleting = null
          if (d) remove(d)
        }}>Delete</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
