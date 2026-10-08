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

/** An Agent as the author or Actor of something in work. */
export type WorkAgent = { id: number; name: string; icon: string }

/** A Run's hold on an Issue while its Agent works on it. */
export type IssueCheckout = { run_id: number; agent: WorkAgent | null; checked_out_at: string }

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

/** An Issue's Assignee: a Member, or an Agent with its Agent icon. */
export type IssueAssignee = { id: number; name: string; kind: 'member' | 'agent'; icon?: string }

/** An Assignee as a picker's value: "member:<id>" or "agent:<id>", null for none. */
export const assigneeKey = (a: Pick<IssueAssignee, 'id' | 'kind'> | null) => (a ? `${a.kind}:${a.id}` : null)

/** The PATCH that makes a picked Assignee key the Issue's only Assignee. */
export function assigneePatch(key: string | null): Pick<IssueInput, 'assignee_id' | 'assignee_agent_id'> {
  const [kind, id] = key ? key.split(':') : []
  return { assignee_id: kind === 'member' ? Number(id) : null, assignee_agent_id: kind === 'agent' ? Number(id) : null }
}

export type Issue = {
  id: number
  number: number
  identifier: string
  title: string
  description?: string
  status: IssueStatus
  priority: Priority
  assignee: IssueAssignee | null
  project: { id: number; name: string } | null
  /** The Issue's Application: one Application of its Project, which its Runs work in. */
  application?: { id: number; name: string } | null
  goal: { id: number; title: string } | null
  parent: IssueRef | null
  created_by: WorkMember | null
  created_by_agent?: WorkAgent | null
  /** The live Run holding the Issue's Checkout and its Agent; null when none does. */
  checkout?: IssueCheckout | null
  started_at: string | null
  completed_at: string | null
  cancelled_at: string | null
  created_at: string
  updated_at: string
  /** On list rows: how many of its Blockers are not done yet. */
  unresolved_blockers?: number
  /** With an Inbox filter (inbox, touched or unread): whether the asking Member has not read its latest change. */
  unread?: boolean
  /** With an Inbox filter: whether the asking Member archived it from their Inbox. */
  archived?: boolean
  /** With an Inbox filter: the asking Member's Last touch. */
  last_touched_at?: string | null
}

/**
 * Something an Issue produced outside The Bakery's own records: its Pull
 * request (open, merged or closed) or the link of that Pull request's
 * Preview (deploying, ready, failed or removed). external_id is the Pull
 * request's number for both; url is empty while a Preview has no link yet.
 */
export type WorkProduct = {
  id: number
  type: 'pull_request' | 'preview_url'
  application_id: number
  /** The git host's key: github, gitlab, gitea or forgejo. */
  provider: string
  external_id: string
  title: string
  url: string
  status: 'open' | 'merged' | 'closed' | 'deploying' | 'ready' | 'failed' | 'removed'
  created_by_run_id: number | null
  created_by: WorkMember | null
  created_by_agent: WorkAgent | null
  created_at: string
  updated_at: string
}

/** One Issue as its page shows it: with its Sub-issues, Blockers both ways and Work products. */
export type IssueDetail = Issue & { description: string; children: Issue[]; blocked_by: Blocker[]; blocking: Blocker[]; work_products: WorkProduct[] }

/** What creating or changing an Issue sends; null clears a reference. */
export type IssueInput = Partial<{
  title: string
  description: string
  status: IssueStatus
  priority: Priority
  assignee_id: number | null
  /** An Agent as the Assignee instead of a Member; not both. */
  assignee_agent_id: number | null
  project_id: number | null
  /** One Application of the Issue's Project; moving the Issue to another Project clears it. */
  application_id: number | null
  goal_id: number | null
  parent_id: number | null
  /** The whole set of Issues that block it; [] clears them. */
  blocked_by_ids: number[]
}>

/**
 * What GET /api/issues keeps: comma lists of statuses and priorities, an
 * assignee, project, goal or parent as an id, "none" (or "me" and
 * "agent:<id>" for the assignee), q to search, and a page by limit and offset. inbox, touched and
 * unread take only "me": the asking Member's Inbox (Touched, not archived),
 * Touched, and Unread.
 */
export type IssueFilter = Partial<{
  status: string[]
  priority: string[]
  assignee: string
  project: string
  goal: string
  parent: string
  q: string
  inbox: 'me'
  touched: 'me'
  unread: 'me'
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

/** The asking Member's Read mark and Inbox archive of an Issue; nobody else's changes. */
export const markRead = (key: number | string) => api<void>('POST', `/issues/${key}/read`)
export const markUnread = (key: number | string) => api<void>('DELETE', `/issues/${key}/read`)
export const archiveFromInbox = (key: number | string) => api<void>('POST', `/issues/${key}/inbox-archive`)
export const unarchiveFromInbox = (key: number | string) => api<void>('DELETE', `/issues/${key}/inbox-archive`)

/** One Comment in an Issue's thread; a deleted one keeps its place with no body. */
export type Comment = {
  id: number
  body: string
  deleted: boolean
  author: WorkMember | null
  author_agent?: WorkAgent | null
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
  created_by_agent?: WorkAgent | null
  updated_by: WorkMember | null
  updated_by_agent?: WorkAgent | null
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
  created_by_agent?: WorkAgent | null
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

/**
 * An Activity event: one Action to a Goal, an Issue or an Approval by its
 * Actor (null once their account is gone). entity.exists is false once the
 * Goal or Issue is deleted (Approvals never are); details hold what changed,
 * by the Action.
 */
export type ActivityEvent = {
  id: number
  action: string
  actor: WorkMember | null
  actor_agent?: WorkAgent | null
  entity: { type: ActivityEntity; id: number; identifier?: string; title: string; exists: boolean }
  details: Record<string, unknown>
  created_at: string
}

/** What an Activity event is about. */
export type ActivityEntity = 'issue' | 'goal' | 'approval' | 'agent'

/** The Guild-wide feed's filters: entity kind, Actor, and before an event id for the next page. */
export type ActivityFilter = Partial<{ entity: ActivityEntity; actor: number | `agent:${number}`; before: number; limit: number }>

export function listActivity(filter: ActivityFilter = {}) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(filter)) if (value !== undefined) query.set(key, String(value))
  const qs = query.toString()
  return api<{ activity: ActivityEvent[] }>('GET', `/activity${qs ? `?${qs}` : ''}`).then((r) => r.activity)
}
/** Every event about one Issue, oldest first. */
export const listIssueActivity = (issue: number | string) =>
  api<{ activity: ActivityEvent[] }>('GET', `/issues/${issue}/activity`).then((r) => r.activity)

// Paperclip's ACTIVITY_ROW_VERBS (ui/src/lib/activity-format.ts; MIT, see
// NOTICE) for the Actions work records.
const activityVerbs: Record<string, string> = {
  'issue.created': 'created',
  'issue.updated': 'updated',
  'issue.deleted': 'deleted',
  'issue.checked_out': 'checked out',
  'issue.released': 'released',
  'issue.application_changed': 'changed the application of',
  'issue.pull_request_opened': 'opened a pull request for',
  'issue.pull_request_merged': 'merged the pull request of',
  'issue.pull_request_closed': 'closed the pull request of',
  'issue.preview_ready': 'deployed the preview of',
  'issue.preview_failed': 'failed the preview of',
  'issue.comment_added': 'commented on',
  'issue.comment_deleted': 'deleted a comment on',
  'issue.document_created': 'created document for',
  'issue.document_updated': 'updated document on',
  'issue.document_deleted': 'deleted document from',
  'goal.created': 'created',
  'goal.updated': 'updated',
  'goal.deleted': 'deleted',
  'approval.created': 'requested approval',
  'approval.approved': 'approved',
  'approval.rejected': 'rejected',
  'approval.revision_requested': 'asked for a revision of',
  'approval.resubmitted': 'resubmitted',
  'approval.comment_added': 'commented on',
  'run.started': 'started a run of',
  'run.finished': 'ended a run of',
}
/** The verb of an Activity row: "commented on" for issue.comment_added. */
export const activityVerb = (action: string) => activityVerbs[action] ?? action.replace(/[._]/g, ' ')

/**
 * A piece of an Issue-tab sentence: plain text, an Issue to link by its
 * identifier, or muted text (a Comment's snippet).
 */
export type ActivityPart = string | { issue: string } | { muted: string }

type Named = { id: number; name?: string | null; title?: string | null; identifier?: string | null } | null
type Change<T> = { from: T; to: T }

const named = (ref: Named, fallback: string) => ref?.name ?? ref?.title ?? fallback

// One sentence per changed field of issue.updated, in the order the
// properties panel shows them; a Blocker is a link while its identifier is
// known (not when it is gone or in a Project the person may not view).
function issueChanges(changes: Record<string, unknown>): ActivityPart[] {
  const sentences: ActivityPart[][] = []
  const c = changes as Partial<{
    status: Change<string>
    priority: Change<string>
    assignee: Change<Named>
    project: Change<Named>
    goal: Change<Named>
    parent: Change<Named>
    title: Change<string>
    description: boolean
    blockers: { added: Named[]; removed: Named[] }
  }>
  if (c.status) sentences.push([`changed the status from ${workLabel(c.status.from)} to ${workLabel(c.status.to)}`])
  if (c.priority) sentences.push([`changed the priority from ${workLabel(c.priority.from)} to ${workLabel(c.priority.to)}`])
  if (c.assignee) sentences.push([c.assignee.to ? `assigned the issue to ${named(c.assignee.to, 'someone')}` : 'unassigned the issue'])
  if (c.project) sentences.push([c.project.to ? `moved the issue to project ${named(c.project.to, 'a project')}` : 'removed the project'])
  if (c.goal) sentences.push([c.goal.to ? `set the goal to ${named(c.goal.to, 'a goal')}` : 'removed the goal'])
  if (c.parent) {
    const p = c.parent.to
    if (!p) sentences.push(['removed the parent'])
    else sentences.push(p.identifier ? ['set the parent to ', { issue: p.identifier }] : ['set the parent to an issue'])
  }
  if (c.title) sentences.push([`renamed the issue to ${c.title.to}`])
  if (c.description) sentences.push(['updated the description'])
  for (const [verb, refs] of [['added', c.blockers?.added], ['removed', c.blockers?.removed]] as const) {
    for (const b of refs ?? []) sentences.push(b?.identifier ? [`${verb} blocker `, { issue: b.identifier }] : [`${verb} a blocker`])
  }
  if (sentences.length === 0) return ['updated the issue']
  return sentences.flatMap((s, n) => (n === 0 ? s : [', ', ...s]))
}

/**
 * What an Activity event on the Issue page says after its Actor, ported from
 * Paperclip's formatIssueActivityAction (ui/src/lib/activity-format.ts; MIT,
 * see NOTICE): "changed the status from Todo to In Progress", "commented".
 */
export function issueActivitySentence(event: ActivityEvent): ActivityPart[] {
  const d = event.details
  const key = typeof d.key === 'string' ? d.key : 'document'
  switch (event.action) {
    case 'issue.created':
      return ['created the issue']
    case 'issue.updated':
      return issueChanges((d.changes as Record<string, unknown>) ?? {})
    case 'issue.application_changed': {
      const to = d.to as Named
      return [to ? `set the application to ${named(to, 'an application')}` : 'removed the application']
    }
    case 'issue.pull_request_opened':
      return [`opened pull request #${d.number}`]
    case 'issue.pull_request_merged':
      return [`merged pull request #${d.number}`]
    case 'issue.pull_request_closed':
      return [`closed pull request #${d.number}`]
    case 'issue.preview_ready':
      return [`deployed the preview of pull request #${d.number}`]
    case 'issue.preview_failed':
      return [`failed to deploy the preview of pull request #${d.number}`]
    case 'issue.comment_added':
      return typeof d.snippet === 'string' && d.snippet ? ['commented ', { muted: d.snippet }] : ['commented']
    case 'issue.comment_deleted':
      return ['deleted a comment']
    case 'issue.document_created':
      return [`created document ${key}${typeof d.title === 'string' && d.title ? ` (${d.title})` : ''}`]
    case 'issue.document_updated':
      return [
        typeof d.restored_from === 'number'
          ? `restored document ${key} to rev ${d.restored_from} (rev ${d.revision_number})`
          : `updated document ${key} (rev ${d.revision_number})`,
      ]
    case 'issue.document_deleted':
      return [`deleted document ${key}`]
    default:
      return [event.action.replace(/[._]/g, ' ')]
  }
}
