// The work context's API: Goals, Issues and the Comments on an Issue.
// Every call answers in the Current guild.
import { api } from './api'

/** How wide a Goal reaches, widest first. */
export const goalLevels = ['guild', 'agent', 'task'] as const
export type GoalLevel = (typeof goalLevels)[number]

/** Where a Goal stands, in the order a Goal moves through them. */
export const goalStatuses = ['planned', 'active', 'achieved', 'cancelled'] as const
export type GoalStatus = (typeof goalStatuses)[number]

export const goalLevelLabels: Record<GoalLevel, string> = { guild: 'Guild', agent: 'Agent', task: 'Task' }

/** A Member as a Goal or an Issue names them. */
export type WorkMember = { id: number; name: string }

export type Goal = {
  id: number
  title: string
  description: string
  level: GoalLevel
  status: GoalStatus
  parent_id: number | null
  owner: WorkMember | null
  created_at: string
  updated_at: string
}

/** An Issue as a Goal's page lists it. */
export type GoalIssue = { id: number; identifier: string; title: string; status: IssueStatus }

/** One Goal as its page shows it: with the Issues that serve it. */
export type GoalDetail = Goal & { issues: GoalIssue[]; issue_counts: Record<string, number> }

/** What creating or changing a Goal sends; null clears a parent or owner. */
export type GoalInput = Partial<{
  title: string
  description: string
  level: GoalLevel
  status: GoalStatus
  parent_id: number | null
  owner_id: number | null
}>

export const listGoals = () => api<{ goals: Goal[] }>('GET', '/goals').then((r) => r.goals)
export const getGoal = (id: number) => api<{ goal: GoalDetail }>('GET', `/goals/${id}`).then((r) => r.goal)
export const createGoal = (input: GoalInput) => api<{ goal: Goal }>('POST', '/goals', input).then((r) => r.goal)
export const updateGoal = (id: number, patch: GoalInput) => api<{ goal: Goal }>('PATCH', `/goals/${id}`, patch).then((r) => r.goal)
export const deleteGoal = (id: number) => api<void>('DELETE', `/goals/${id}`)

/** Where an Issue stands, in the order an Issue moves through them (Paperclip's issueStatusOrder). */
export const issueStatuses = ['backlog', 'todo', 'in_progress', 'in_review', 'blocked', 'done', 'cancelled'] as const
export type IssueStatus = (typeof issueStatuses)[number]

/** How urgent an Issue is, most urgent first. */
export const priorities = ['critical', 'high', 'medium', 'low'] as const
export type Priority = (typeof priorities)[number]

/** "in_progress" as a person reads it: "In Progress". */
export const workLabel = (key: string) => key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())

export type IssueRef = { id: number; identifier: string; title: string }

/** An Issue in another Issue's blocked_by or blocking, with its status so a resolved (done) one shows. */
export type Blocker = IssueRef & { status: IssueStatus }

export type Issue = {
  id: number
  number: number
  identifier: string
  title: string
  description?: string
  status: IssueStatus
  priority: Priority
  assignee: WorkMember | null
  project: { id: number; name: string } | null
  goal: { id: number; title: string } | null
  parent: IssueRef | null
  created_by: WorkMember | null
  started_at: string | null
  completed_at: string | null
  cancelled_at: string | null
  created_at: string
  updated_at: string
  /** On list rows: how many of its Blockers are not done yet. */
  unresolved_blockers?: number
}

/** One Issue as its page shows it: with its Sub-issues and Blockers both ways. */
export type IssueDetail = Issue & { description: string; children: Issue[]; blocked_by: Blocker[]; blocking: Blocker[] }

/** What creating or changing an Issue sends; null clears a reference. */
export type IssueInput = Partial<{
  title: string
  description: string
  status: IssueStatus
  priority: Priority
  assignee_id: number | null
  project_id: number | null
  goal_id: number | null
  parent_id: number | null
  /** The whole set of Issues that block it; [] clears them. */
  blocked_by_ids: number[]
}>

/**
 * What GET /api/issues keeps: comma lists of statuses and priorities, an
 * assignee, project, goal or parent as an id, "none" (or "me" for the
 * assignee), q to search, and a page by limit and offset.
 */
export type IssueFilter = Partial<{
  status: string[]
  priority: string[]
  assignee: string
  project: string
  goal: string
  parent: string
  q: string
  limit: number
  offset: number
}>

export function listIssues(filter: IssueFilter = {}) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(filter)) {
    const v = Array.isArray(value) ? value.join(',') : String(value ?? '')
    if (v !== '') query.set(key, v)
  }
  const qs = query.toString()
  return api<{ issues: Issue[] }>('GET', `/issues${qs ? `?${qs}` : ''}`).then((r) => r.issues)
}
/** key is an Issue's id or its Issue identifier (DEF-12). */
export const getIssue = (key: number | string) => api<{ issue: IssueDetail }>('GET', `/issues/${key}`).then((r) => r.issue)
export const createIssue = (input: IssueInput) => api<{ issue: IssueDetail }>('POST', '/issues', input).then((r) => r.issue)
export const updateIssue = (key: number | string, patch: IssueInput) =>
  api<{ issue: IssueDetail }>('PATCH', `/issues/${key}`, patch).then((r) => r.issue)
export const deleteIssue = (key: number | string) => api<void>('DELETE', `/issues/${key}`)

/** One Comment in an Issue's thread; a deleted one keeps its place with no body. */
export type Comment = {
  id: number
  body: string
  deleted: boolean
  author: WorkMember | null
  created_at: string
  updated_at: string
  edited: boolean
}

export const listComments = (issue: number | string) =>
  api<{ comments: Comment[] }>('GET', `/issues/${issue}/comments`).then((r) => r.comments)
export const writeComment = (issue: number | string, body: string) =>
  api<{ comment: Comment }>('POST', `/issues/${issue}/comments`, { body }).then((r) => r.comment)
export const editComment = (issue: number | string, id: number, body: string) =>
  api<{ comment: Comment }>('PATCH', `/issues/${issue}/comments/${id}`, { body }).then((r) => r.comment)
export const deleteComment = (issue: number | string, id: number) => api<void>('DELETE', `/issues/${issue}/comments/${id}`)

/** The rule the API holds a Document key to: lowercase letters, digits, _ and -, starting with a letter or digit. */
export const documentKeyPattern = /^[a-z0-9][a-z0-9_-]{0,63}$/

/** An Issue document at its newest Revision. */
export type IssueDocument = {
  id: number
  key: string
  title: string
  body: string
  format: 'markdown'
  latest_revision_id: number
  latest_revision_number: number
  created_by: WorkMember | null
  updated_by: WorkMember | null
  created_at: string
  updated_at: string
}

/** One saved version of an Issue document. */
export type DocumentRevision = {
  id: number
  number: number
  title: string
  body: string
  change_summary: string | null
  created_by: WorkMember | null
  created_at: string
}

/** What saving a document sends; `base_revision_id` is the Revision the text was written against, absent for a new key. */
export type DocumentInput = { title: string; body: string; change_summary?: string; base_revision_id?: number }

export const listIssueDocuments = (issue: number | string) =>
  api<{ documents: IssueDocument[] }>('GET', `/issues/${issue}/documents`).then((r) => r.documents)
export const getIssueDocument = (issue: number | string, key: string) =>
  api<{ document: IssueDocument }>('GET', `/issues/${issue}/documents/${key}`).then((r) => r.document)
export const saveIssueDocument = (issue: number | string, key: string, input: DocumentInput) =>
  api<{ document: IssueDocument }>('PUT', `/issues/${issue}/documents/${key}`, input).then((r) => r.document)
export const deleteIssueDocument = (issue: number | string, key: string) => api<void>('DELETE', `/issues/${issue}/documents/${key}`)
/** Newest first. */
export const listDocumentRevisions = (issue: number | string, key: string) =>
  api<{ revisions: DocumentRevision[] }>('GET', `/issues/${issue}/documents/${key}/revisions`).then((r) => r.revisions)
export const restoreDocumentRevision = (issue: number | string, key: string, revision: number) =>
  api<{ document: IssueDocument }>('POST', `/issues/${issue}/documents/${key}/revisions/${revision}/restore`).then((r) => r.document)
