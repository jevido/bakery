// The work context's Approvals: a Member asks the Board to decide, and those
// with the approve Permission approve, reject or ask for a revision. Every
// call answers in the Current guild. The labels and icons are Paperclip's
// (ui/src/components/ApprovalPayload.tsx; MIT, see NOTICE).
import { ShieldCheck } from '@lucide/svelte'
import { api } from './api'
import type { Issue, WorkMember } from './work'

/** Where an Approval stands; pending and revision_requested are Actionable. */
export type ApprovalStatus = 'pending' | 'revision_requested' | 'approved' | 'rejected'

/** What a request_board_approval asks: Paperclip's BoardApprovalPayload. */
export type ApprovalPayload = {
  title: string
  summary: string
  recommended_action: string
  next_action_on_approval: string
  risks: string[]
}

export type Approval = {
  id: number
  type: string
  status: ApprovalStatus
  payload: ApprovalPayload
  requester: WorkMember | null
  decided_by: WorkMember | null
  decision_note: string | null
  decided_at: string | null
  created_at: string
  updated_at: string
}

export type ApprovalComment = { id: number; body: string; author: WorkMember | null; created_at: string }

/** Only Board Approvals exist until agents come; hiring and budgets add theirs. */
export const approvalTypeLabels: Record<string, string> = { request_board_approval: 'Board Approval' }
export const approvalTypeLabel = (type: string) => approvalTypeLabels[type] ?? type
export const approvalTypeIcon = (_type: string) => ShieldCheck

const firstText = (...values: unknown[]) => {
  for (const v of values) if (typeof v === 'string' && v.trim()) return v.trim()
  return null
}

/** What the Approval is about: its title, else its summary or recommended action. */
export const approvalSubject = (payload: Partial<ApprovalPayload> | null) =>
  firstText(payload?.title, payload?.summary, payload?.recommended_action)

/** "Board Approval: <title>", as Paperclip's approvalLabel. */
export function approvalLabel(type: string, payload: Partial<ApprovalPayload> | null): string {
  const subject = approvalSubject(payload)
  return subject ? `${approvalTypeLabel(type)}: ${subject}` : approvalTypeLabel(type)
}

export const isActionable = (a: Approval) => a.status === 'pending' || a.status === 'revision_requested'

export const listApprovals = (status = '') =>
  api<{ approvals: Approval[] }>('GET', `/approvals${status ? `?status=${status}` : ''}`).then((r) => r.approvals)
export const getApproval = (id: number) => api<{ approval: Approval }>('GET', `/approvals/${id}`).then((r) => r.approval)
export const requestApproval = (payload: ApprovalPayload, issueIds: number[]) =>
  api<{ approval: Approval }>('POST', '/approvals', { type: 'request_board_approval', payload, issue_ids: issueIds }).then((r) => r.approval)
export const listApprovalIssues = (id: number) => api<{ issues: Issue[] }>('GET', `/approvals/${id}/issues`).then((r) => r.issues)
export const listIssueApprovals = (issue: number | string) =>
  api<{ approvals: Approval[] }>('GET', `/issues/${issue}/approvals`).then((r) => r.approvals)

/** A Decision, with an optional Decision note. */
export type Decision = 'approve' | 'reject' | 'request-revision'
export const decide = (id: number, decision: Decision, note = '') =>
  api<{ approval: Approval }>('POST', `/approvals/${id}/${decision}`, { decision_note: note }).then((r) => r.approval)
/** Makes the Requester's own Approval pending again; without a payload it keeps the one it has. */
export const resubmitApproval = (id: number, payload?: ApprovalPayload) =>
  api<{ approval: Approval }>('POST', `/approvals/${id}/resubmit`, payload ? { payload } : {}).then((r) => r.approval)

export const listApprovalComments = (id: number) =>
  api<{ comments: ApprovalComment[] }>('GET', `/approvals/${id}/comments`).then((r) => r.comments)
export const writeApprovalComment = (id: number, body: string) =>
  api<{ comment: ApprovalComment }>('POST', `/approvals/${id}/comments`, { body }).then((r) => r.comment)
