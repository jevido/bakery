<script lang="ts">
  // Paperclip's CommentThread with its CommentCard
  // (ui/src/components/CommentThread.tsx; MIT, see NOTICE): an Issue's
  // Comments oldest first, each a bordered card with its author, when it was
  // written (and "edited" when so) and its Markdown, then the composer, where
  // Ctrl/⌘+Enter sends. A deleted Comment keeps its place as "Comment
  // deleted". Edit and Delete are The Bakery's own: Paperclip's people do
  // not change a Comment once it is written, here its author may. Left out
  // with what the Issue does not have yet: runs, approvals and events in the
  // timeline, queued Comments, mentions, image uploads, feedback votes and
  // reassigning from the composer.
  import { Pencil, Trash2 } from '@lucide/svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button, buttonVariants } from '@bakery/ui/components/ui/button'
  import { ApiError } from './api'
  import { ago, formatDate } from './format'
  import Identity from './Identity.svelte'
  import Markdown from './Markdown.svelte'
  import MarkdownField from './MarkdownField.svelte'
  import { session } from './session.svelte'
  import { toast } from './ui/toast.svelte'
  import { deleteComment, editComment, listComments, writeComment, type Comment } from './work'

  let {
    issue,
    editable,
    onchange,
  }: {
    /** The Issue's id or identifier. */
    issue: number | string
    /** Whether the Member may write Comments (manage_work). */
    editable: boolean
    /** After a Comment was written, changed or deleted, so the Issue shows its new updated time. */
    onchange?: () => void
  } = $props()

  let comments = $state.raw<Comment[] | null>(null)
  let body = $state('')
  let posting = $state(false)
  let editing = $state<number | null>(null)
  let draft = $state('')
  let deleting = $state.raw<Comment | null>(null)

  const me = $derived(session.member?.id)

  function load() {
    listComments(issue)
      .then((cs) => (comments = cs))
      .catch((e) => toast.error(e instanceof Error ? e.message : String(e)))
  }
  load()

  function failed(e: unknown) {
    toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
  }

  async function post() {
    if (!body.trim() || posting) return
    posting = true
    try {
      const c = await writeComment(issue, body.trim())
      comments = [...(comments ?? []), c]
      body = ''
      onchange?.()
    } catch (e) {
      failed(e)
    } finally {
      posting = false
    }
  }

  async function save(c: Comment) {
    if (!draft.trim()) return
    try {
      const updated = await editComment(issue, c.id, draft.trim())
      comments = (comments ?? []).map((x) => (x.id === c.id ? updated : x))
      editing = null
      onchange?.()
    } catch (e) {
      failed(e)
    }
  }

  async function remove(c: Comment) {
    try {
      await deleteComment(issue, c.id)
      load()
      onchange?.()
    } catch (e) {
      failed(e)
    }
  }
</script>

<section class="space-y-4" aria-label="Comments">
  <h3 class="text-sm font-semibold">Comments ({comments?.filter((c) => !c.deleted).length ?? 0})</h3>

  {#if comments}
    <div class="space-y-3">
      {#each comments as c (c.id)}
        <div
          id="comment-{c.id}"
          data-comment={c.id}
          class={['min-w-0 overflow-hidden rounded-sm border border-border p-3', c.deleted && 'bg-muted/30 text-muted-foreground']}
        >
          <div class="mb-1 flex items-center justify-between gap-2">
            {#if c.author}<Identity name={c.author.id === me ? 'You' : c.author.name} size="sm" />{:else}<Identity name="Someone" size="sm" />{/if}
            <span class="flex items-center gap-1.5">
              <a href="#comment-{c.id}" onclick={(e) => e.preventDefault()} class="text-xs text-muted-foreground no-underline transition-colors hover:text-foreground hover:underline" title={formatDate(c.created_at)}>
                {ago(c.created_at)}{#if c.edited && !c.deleted}<span title="Edited {ago(c.updated_at)}"> · edited</span>{/if}
              </a>
              {#if editable && !c.deleted && c.author?.id === me && editing !== c.id}
                <Button
                  variant="ghost"
                  size="icon-xs"
                  title="Edit comment"
                  aria-label="Edit comment"
                  onclick={() => {
                    editing = c.id
                    draft = c.body
                  }}><Pencil /></Button
                >
                <Button variant="ghost" size="icon-xs" title="Delete comment" aria-label="Delete comment" onclick={() => (deleting = c)}><Trash2 /></Button>
              {/if}
            </span>
          </div>
          {#if c.deleted}
            <div class="text-sm text-muted-foreground italic">Comment deleted</div>
          {:else if editing === c.id}
            <div class="space-y-2">
              <MarkdownField bind:value={draft} label="Edit comment" rows={4} autofocus onsubmit={() => save(c)} />
              <div class="flex justify-end gap-2">
                <Button variant="ghost" size="sm" onclick={() => (editing = null)}>Cancel</Button>
                <Button size="sm" disabled={!draft.trim()} onclick={() => save(c)}>Save</Button>
              </div>
            </div>
          {:else}
            <Markdown source={c.body} class="text-sm" />
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  {#if editable}
    <div class="space-y-2">
      <MarkdownField bind:value={body} label="Comment" placeholder="Leave a comment..." rows={3} onsubmit={post} />
      <div class="flex items-center justify-end gap-3">
        <Button size="sm" disabled={!body.trim() || posting} onclick={post}>{posting ? 'Posting...' : 'Comment'}</Button>
      </div>
    </div>
  {/if}
</section>

<AlertDialog.Root open={deleting !== null} onOpenChange={(open) => !open && (deleting = null)}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Delete comment?</AlertDialog.Title>
      <AlertDialog.Description>The comment's text is gone for good; the thread shows where it was.</AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action
        class={buttonVariants({ variant: 'destructive' })}
        data-testid="confirm-delete-comment"
        onclick={() => {
          const c = deleting
          deleting = null
          if (c) remove(c)
        }}>Delete</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
