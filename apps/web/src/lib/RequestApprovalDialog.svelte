<script lang="ts">
  // The Bakery's own form for a Board Approval: Paperclip has none, since only
  // its agents ask (their request_board_approval payload is what it sends).
  // Built like NewIssueDialog on the dashboard's Modal: a title, a Markdown
  // summary, the recommended action, what happens on approval and the risks,
  // one per line with empty lines dropped. Given an Issue it requests a new
  // Approval linked to it; given an Approval (Resubmit on its page) it opens
  // prefilled and resubmits the edited request. Ctrl/⌘+Enter sends it.
  import { Input } from '$lib/components/ui/input'
  import { refreshBadges } from './inbox.svelte'
  import { Textarea } from '$lib/components/ui/textarea'
  import { ApiError } from './api'
  import { requestApproval, resubmitApproval, type Approval, type ApprovalPayload } from './approvals'
  import MarkdownField from './MarkdownField.svelte'
  import Button from './ui/Button.svelte'
  import Modal from './ui/Modal.svelte'

  let {
    open = $bindable(false),
    issueId = null,
    approval = null,
    onsaved,
  }: {
    open?: boolean
    /** The Issue a new Approval is linked to. */
    issueId?: number | null
    /** The Approval to resubmit, which prefills the form. */
    approval?: Approval | null
    onsaved?: (approval: Approval) => void
  } = $props()

  let title = $state('')
  let summary = $state('')
  let recommended = $state('')
  let onApproval = $state('')
  let risks = $state('')
  let saving = $state(false)
  let error = $state('')

  // Each opening starts from the Approval's request, or empty.
  $effect(() => {
    if (!open) return
    const p = approval?.payload
    title = p?.title ?? ''
    summary = p?.summary ?? ''
    recommended = p?.recommended_action ?? ''
    onApproval = p?.next_action_on_approval ?? ''
    risks = (p?.risks ?? []).join('\n')
    error = ''
  })

  async function submit() {
    if (!title.trim() || saving) return
    saving = true
    error = ''
    const payload: ApprovalPayload = {
      title: title.trim(),
      summary: summary.trim(),
      recommended_action: recommended.trim(),
      next_action_on_approval: onApproval.trim(),
      risks: risks
        .split('\n')
        .map((r) => r.trim())
        .filter(Boolean),
    }
    try {
      const saved = approval ? await resubmitApproval(approval.id, payload) : await requestApproval(payload, issueId ? [issueId] : [])
      refreshBadges()
      open = false
      onsaved?.(saved)
    } catch (e) {
      error = e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e)
    } finally {
      saving = false
    }
  }

  const submitOnShortcut = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      submit()
    }
  }
</script>

<Modal bind:open variant="none" title={approval ? 'Resubmit approval' : 'Request approval'}>
  <div class="space-y-3">
    <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
      Title
      <!-- svelte-ignore a11y_autofocus -->
      <Input bind:value={title} placeholder="What should the Board decide?" autofocus onkeydown={submitOnShortcut} />
    </label>
    <MarkdownField bind:value={summary} label="Summary" placeholder="Why this needs a decision..." rows={4} onsubmit={submit} />
    <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
      Recommended action
      <Input bind:value={recommended} onkeydown={submitOnShortcut} />
    </label>
    <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
      On approval
      <Input bind:value={onApproval} placeholder="What happens once it is approved" onkeydown={submitOnShortcut} />
    </label>
    <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
      Risks
      <Textarea bind:value={risks} rows={3} placeholder="One per line" onkeydown={submitOnShortcut} />
    </label>
    {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
  </div>
  {#snippet footer()}
    <Button variant="highlighted" disabled={!title.trim()} loading={saving} onclick={submit}>
      {approval ? (saving ? 'Resubmitting…' : 'Resubmit') : saving ? 'Requesting…' : 'Request approval'}
    </Button>
  {/snippet}
</Modal>
