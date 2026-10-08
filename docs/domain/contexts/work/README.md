# work

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/work`)

## Purpose

Holds the work a Guild's Board plans and tracks together: the Guild's Goals
(a tree of outcomes it works toward), its Issues (the pieces of work, each
with a status, a Priority and an Assignee, tied to a Project and a Goal),
the Comments people write on an Issue, which Issues block which (Blockers),
the Issue documents (plans, specs, notes) kept on an Issue with their
Revisions, and the Activity: who did what to the Guild's Goals, Issues,
Approvals and (recorded for the agents context) Agents, and when. It also holds each Member's Inbox: their Read marks and Inbox
archives on the Guild's Issues, and the Approvals: decisions a Member asks
the Board to make, with their Linked issues and Approval comments. Every
Goal, Issue and Approval belongs to exactly one Guild, and an Issue in a
Project follows that Project's Permission overrides.

It is **not** responsible (yet) for checkout or document locks: later phases of the guilds goal add them. Agents and their Runs
are the agents context's; work holds only an Issue's Agent assignee.
It does not own Members, Guilds or Projects either; it stores their ids and asks guilds and projects about them.

## Language

Shared terms (Board, Goal, Goal level, Goal status, Issue, Issue status,
Priority, Assignee, Issue prefix, Issue identifier, Comment, Blocker, Issue
document, Document key, Revision, Base revision, Restore, Activity, Activity event,
Action, Actor, Inbox, Inbox tab, Touched, Last touch, Unread, Read mark,
Inbox archive, Resurface, Approval, Approval type, Approval status,
Actionable, Requester, Decision, Decision note, Request revision, Resubmit,
Approval comment, Linked issue) are in
[`glossary.md`](../../glossary.md).

| Term | Meaning |
| ---- | ------- |
| Sub-goal | A Goal whose parent is another Goal. |
| Sub-issue | An Issue whose parent is another Issue. |
| Owner (of a Goal) | The Member a Goal is in the hands of; optional. |
| Blocked by | The Blockers of an Issue: the Issues it waits on. |
| Blocking | The Issues that have this Issue as a Blocker. |
| Resolved Blocker | A Blocker whose Issue status is `done`. An Issue with an unresolved Blocker is shown as waiting on it, whatever its own status. |
| Status times | An Issue's started, completed and cancelled times, set and cleared by its Issue status changes as the glossary says. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Goal | Belongs to one Guild. Title 1–200 characters. Goal level and Goal status from their lists. Its parent Goal is in the same Guild and is never the Goal itself or one of its Sub-goals (no cycle). Its owner, if any, is a Member of the Guild. |
| Issue | Belongs to one Guild. Its number is unique in the Guild, taken from the Guild's Issue counter (kept in work) when it is created, and never changes or comes back. Title 1–200 characters. Issue status and Priority from their lists. Its parent Issue is in the same Guild and never the Issue itself or one of its Sub-issues (no cycle). Its Assignee is a Member of the Guild or an Agent of the Guild that is not terminated (its Agent assignee), never both; terminating the Agent clears it as Assignee of its open Issues. Its Project is in the Guild and its Goal is in the Guild. The Status times follow its Issue status: started is set once on the first move to `in_progress`; completed is set on `done` and cancelled on `cancelled`, and each is cleared when the Issue moves back out. Its Blockers are Issues of the same Guild, never the Issue itself, and never form a cycle: an Issue may not be blocked by an Issue that is, directly or through others, blocked by it. |
| Comment | Belongs to one Issue and is written by one Member. Body 1–20000 characters. Only its author edits or deletes it. A deleted Comment keeps its place in the thread, its body gone, shown as "deleted". |
| Issue document | Belongs to one Issue and its Guild. Its Document key is unique per Issue and never changes. Title at most 200 characters (may be empty), body at most 524288 characters, Markdown only. Its Revisions are numbered 1, 2, … without gaps; each save and each Restore adds exactly one Revision, and Revisions are never changed. A save needs the newest Revision as its Base revision (none for the first save), or it is refused. Restoring the newest Revision is refused, since it would change nothing. Deleting the Issue document removes its Revisions. |
| Activity event | Append-only. Belongs to one Guild and has one Actor (or none once that Member's account is gone) and one Action from the glossary's list, about exactly one Goal, Issue, Approval or Agent. It keeps the Issue's number and title, the Goal's title, the Approval's type and payload title, or the Agent's name, as they were, so an event about a deleted one still reads, and the Issue's Project, so a deleted Issue's events stay hidden where it was. It is never changed, and goes only with its Guild. |
| Approval | Belongs to one Guild. Approval type and Approval status from their lists; it starts `pending`. A `request_board_approval` payload has a `title` of 1–200 characters and optional `summary`, `recommended_action` and `next_action_on_approval` (each at most 20000 characters) and `risks` (at most 20 strings of at most 500 characters), and nothing else. A `hire_agent` payload is `agent_id`, `name`, `job`, `title`, `icon`, `reports_to` (`{id, name}` or null), `capabilities` and `roles` (Role names), as the agents context sends it; its title is "Hire Agent: <name>". A `hire_agent` Approval is created only through `work.RequestApproval`, never gets Request revision (its Agent cannot change while it waits, so there is nothing to revise), and becomes `cancelled` when its Agent is terminated before a Decision; `cancelled` is not Actionable and never changes again. Approve and reject only from `pending` or `revision_requested` (Actionable); Request revision only from `pending`; Resubmit only from `revision_requested`, which clears the decider, the decision time and the Decision note. A Decision records its decider, time and optional Decision note (at most 20000 characters). Making the same Decision again on an Approval that already has it answers the Approval unchanged and records nothing, as Paperclip's `applied: false`; any other move from a status that does not allow it is refused (422). Its Linked issues are Issues of the same Guild, set when it is requested. Its Approval comments each have one author and a body of 1–20000 characters, and are never edited or deleted. The Approval, its links and its Approval comments go with their Guild; a Linked issue's link goes with the Issue. |
| Read mark and Inbox archive | Per Member per Issue, at most one of each. Belongs to an Issue and its Guild, and goes with the Issue and with the Member's Membership. Only that Member sets or removes it. Neither is part of the Issue aggregate: they change nothing about the Issue, belong to one person, and many people write them at once, so each is its own small record keyed by (Issue, Member). |

### Commands

Who may run each is in brackets, by Permission in the Current guild. Reading
needs `view_resources`; an Issue with a Project also needs `view_resources`
in that Project after its Permission overrides, and is otherwise hidden
(404), in lists as on its own. A Goal has no Project.

- `CreateGoal(title, description, level, status, parent, owner)`,
  `ChangeGoal(...)`, `DeleteGoal()` [`manage_work`]. Deleting a Goal
  moves its Sub-goals under its own parent (none for a top Goal) and
  leaves its Issues without a Goal.
- `CreateIssue(title, description, status, priority, assignee, project,
  goal, parent)` [`manage_work`]: takes the Guild's next number.
- `ChangeIssue(...)` [`manage_work`]: any field but its number; a status
  change moves the Status times.
- `BlockWith(ids)` [`manage_work`], as part of `ChangeIssue`: replaces
  the Issue's Blockers with the given Issues, keeping any Blocker the
  person cannot see.
- `DeleteIssue()` [`manage_work`]: its Sub-issues lose their parent; its
  Comments, its Issue documents and its Blockers in both directions go
  with it.
- `WriteComment(body)` [`manage_work`]: moves the Issue's last update to
  now, so a discussed Issue sorts up in lists, as Paperclip's does. This
  touches only the Issue's timestamp, none of its rules, so it is stored
  with the Comment in one transaction. `EditComment(body)`,
  `DeleteComment()` [`manage_work`, and only the author; anyone else is
  refused (403), and a deleted Comment cannot be changed (409)].
- `SaveDocument(key, title, body, change summary, base revision)`
  [`manage_work`]: creates the Issue document on its first save, and adds
  a Revision on every save; a stale Base revision is refused (409).
- `RestoreRevision(revision)` [`manage_work`]: adds a new Revision with
  that Revision's title and body.
- `DeleteDocument()` [`manage_work`]: removes the Issue document and its
  Revisions.
- `MarkRead()`, `MarkUnread()`, `ArchiveFromInbox()`,
  `UnarchiveFromInbox()` [`view_resources`, on an Issue the Member may
  view; for the asking Member only]: set or remove that Member's Read mark
  or Inbox archive on the Issue. They publish no domain events and add no
  Activity events.
- `RequestApproval(type, payload, issue ids)` [`manage_work`]: the asking
  Member is the Requester; every Issue id must be the Guild's (422
  otherwise). Only `request_board_approval` through the API; `hire_agent`
  comes only from the agents context (below).
- `Approve(note)`, `Reject(note)` [`approve`]: a Decision on an Actionable
  Approval.
- `RequestRevision(note)` [`approve`]: on a `pending` Approval.
- `Resubmit(payload)` [`manage_work`, and only the Requester; anyone else
  is refused (403)]: on a `revision_requested` Approval, replacing the
  payload when one is given.
- `CommentOnApproval(body)` [`manage_work`].
- For other contexts, without a route of their own: `work.RequestApproval`
  (a `hire_agent` Approval, its Requester the Hirer, checked by the
  calling context), `work.CancelApproval` (an Actionable `hire_agent`
  Approval whose Agent was terminated) and `work.RecordActivity` (one
  Activity event about an Agent, with its Actor, Action and details).

Reading Approvals, their Linked issues and Approval comments needs
`view_resources`. An Approval has no Project, so everyone who may read the
Guild's work sees it; a Linked issue in a Project the person may not view
is left out of what they read.

Reading an Issue's Blockers, Issue documents and Revisions needs what
reading the Issue needs.

Reading the Activity needs `view_resources`. An event about an Issue
follows the Issue's current Project: while the person may not view that
Project, the Issue's own Activity is 404 and its events are left out of the
Guild's feed.

### Domain events

Each command publishes its event after its change is stored, and work
records each one as one Activity event with the Action beside it. Editing a
Comment publishes nothing (Paperclip records none).

| Event | Published by | Action | Carries |
| ----- | ------------ | ------ | ------- |
| `GoalCreated` | `CreateGoal` | `goal.created` | title, Goal level, Goal status |
| `GoalChanged` | `ChangeGoal`, when something changed | `goal.updated` | each changed field, from → to |
| `GoalDeleted` | `DeleteGoal` | `goal.deleted` | title |
| `IssueCreated` | `CreateIssue` | `issue.created` | number, title, Issue status, Priority |
| `IssueChanged` | `ChangeIssue`, when something changed | `issue.updated` | each changed field, from → to; Blockers added and removed |
| `IssueDeleted` | `DeleteIssue` | `issue.deleted` | number, title |
| `CommentWritten` | `WriteComment` | `issue.comment_added` | the Comment and its first 140 characters |
| `CommentDeleted` | `DeleteComment` | `issue.comment_deleted` | the Comment |
| `DocumentSaved` | `SaveDocument`, `RestoreRevision` | `issue.document_created` on the first save, else `issue.document_updated` | Document key, title, Revision number, and the restored Revision's number for a Restore |
| `DocumentDeleted` | `DeleteDocument` | `issue.document_deleted` | Document key, title |
| `ApprovalRequested` | `RequestApproval` | `approval.created` | Approval type, the payload's title, the Linked issues |
| `ApprovalApproved` | `Approve`, when it changed something | `approval.approved` | Approval type, the payload's title, Decision note |
| `ApprovalRejected` | `Reject`, when it changed something | `approval.rejected` | Approval type, the payload's title, Decision note |
| `RevisionRequested` | `RequestRevision` | `approval.revision_requested` | Approval type, the payload's title, Decision note |
| `ApprovalResubmitted` | `Resubmit` | `approval.resubmitted` | Approval type, the payload's title |
| `ApprovalCommentWritten` | `CommentOnApproval` | `approval.comment_added` | the Approval comment and its first 140 characters |
| `ApprovalCancelled` | `CancelApproval` | `approval.cancelled` | Approval type, the payload's title |

The agents context's own events (`AgentHired`, `AgentUpdated`,
`AgentPaused`, `AgentResumed`, `AgentTerminated`, `AgentRoleAdded`,
`AgentRoleRemoved`) reach the Activity through `work.RecordActivity` as
`agent.hired`, `agent.updated`, `agent.paused`, `agent.resumed`,
`agent.terminated`, `agent.role_added` and `agent.role_removed`.

Every Issue event also carries the Issue's number, title and Project; every
Goal event the Goal's title, and every Approval event the Approval's
type and the payload's title.

## Integration

- **Publishes:** the Goals and Issues API, all in the Current guild (an id
  from another Guild is 404), reading with `view_resources` and changing
  with `manage_work` (403 without it). An Issue is addressed by its id or
  its Issue identifier (`DEF-12`).

  | Route | Answers |
  | ----- | ------- |
  | `GET /api/goals` | `{"goals": [Goal]}` |
  | `POST /api/goals` | 201 `{"goal": Goal}` |
  | `GET /api/goals/{id}` | `{"goal": Goal + "issues": [Issue], "issue_counts": {status: n}}` |
  | `PATCH /api/goals/{id}`, `DELETE /api/goals/{id}` | `{"goal": Goal}`, 204 |
  | `GET /api/issues` | `{"issues": [Issue]}`, most recently updated first; filters `status` and `priority` (comma lists), `assignee` (a Member's id, `agent:<id>` for an Agent assignee, `me` or `none`), `project`, `goal`, `parent` (an id or `none`), `q` (title, description, identifier), `limit` (≤ 200) and `offset` |
  | `POST /api/issues` | 201 `{"issue": Issue + "children": [Issue]}` |
  | `GET /api/issues/{issue}`, `PATCH /api/issues/{issue}` | `{"issue": Issue + "children": [Issue]}` |
  | `DELETE /api/issues/{issue}` | 204 |
  | `GET /api/issues/{issue}/comments` | `{"comments": [Comment]}`, oldest first |
  | `POST /api/issues/{issue}/comments` | 201 `{"comment": Comment}` |
  | `PATCH /api/issues/{issue}/comments/{comment}` | `{"comment": Comment}` |
  | `DELETE /api/issues/{issue}/comments/{comment}` | 204; the Comment keeps its place in the list, `deleted` and without its body |
  | `GET /api/issues/{issue}/documents` | `{"documents": [Issue document]}`, by Document key |
  | `GET /api/issues/{issue}/documents/{key}` | `{"document": Issue document}`; 404 for an unknown key, 422 for a malformed one |
  | `PUT /api/issues/{issue}/documents/{key}` | 201 `{"document": Issue document}` on the first save, 200 after; 409 for a missing or stale `base_revision_id` (with `current_revision_id` and `current_revision_number`) or a `base_revision_id` on a new key |
  | `DELETE /api/issues/{issue}/documents/{key}` | 204 |
  | `GET /api/issues/{issue}/documents/{key}/revisions` | `{"revisions": [Revision]}`, newest first |
  | `POST /api/issues/{issue}/documents/{key}/revisions/{revision}/restore` | `{"document": Issue document}`; 409 for the newest Revision |
  | `GET /api/activity` | `{"activity": [Activity event]}`, newest first; filters `entity` (`issue`, `goal`, `approval` or `agent`), `actor` (a Member id), `before` (an Activity event id, for the next page) and `limit` (default 50, 1 to 200); anything else in them is 422. A page is never short while older events the person may see are left |
  | `GET /api/issues/{issue}/activity` | `{"activity": [Activity event]}`, oldest first, as Paperclip's issue activity |
  | `POST /api/issues/{issue}/read`, `DELETE /api/issues/{issue}/read` | Sets or removes the asking Member's Read mark, with `view_resources` only |
  | `POST /api/issues/{issue}/inbox-archive`, `DELETE /api/issues/{issue}/inbox-archive` | Sets or removes the asking Member's Inbox archive, with `view_resources` only |
  | `GET /api/sidebar-badges` | `{"inbox": n, "approvals": n}`: `approvals` is how many of the Guild's Approvals are Actionable, and `inbox` adds them to how many Issues in the asking Member's Mine tab are Unread, for every Member as in Paperclip, since the Unread tab shows those Approvals to everyone |
  | `GET /api/approvals` | `{"approvals": [Approval]}`, newest first and unpaged, as Paperclip's; filter `status`, a comma list of Approval statuses and `actionable` (both `pending` and `revision_requested`); anything else is 422 |
  | `POST /api/approvals` | 201 `{"approval": Approval}`; body `{type, payload, issue_ids}`; 422 for an unknown type or `hire_agent`, a payload that breaks the rules or an Issue id that is not the Guild's |
  | `GET /api/approvals/{id}` | `{"approval": Approval}` |
  | `GET /api/approvals/{id}/issues` | `{"issues": [Issue]}`, the Linked issues the person may see, as in Issue lists |
  | `POST /api/approvals/{id}/approve`, `.../reject`, `.../request-revision` | `{"approval": Approval}`; body `{decision_note}`, optional; the same Decision again answers the Approval unchanged; 422 when the Approval's status does not allow it |
  | `POST /api/approvals/{id}/resubmit` | `{"approval": Approval}`; body `{payload}`, optional; 403 for anyone but the Requester, 422 unless `revision_requested` |
  | `GET /api/approvals/{id}/comments` | `{"comments": [Approval comment]}`, oldest first |
  | `POST /api/approvals/{id}/comments` | 201 `{"comment": Approval comment}`; body `{body}` |
  | `GET /api/issues/{issue}/approvals` | `{"approvals": [Approval]}`, the Approvals linked to the Issue, newest first |

  `GET /api/issues` also takes `touched`, `unread` and `inbox`, each only
  `me` (anything else is 422): `touched=me` keeps the Issues the asking
  Member is Touched by (the Recent tab), `unread=me` the Unread ones (the
  Unread tab), and `inbox=me` the Touched ones they have not archived, or
  that Resurfaced since (the Mine tab). They combine with each other and
  with the other filters. With any of them, each Issue also answers
  `unread`, `last_touched_at` and `archived` (in their Inbox archive and
  not Resurfaced) for that Member. The list stays sorted by the Issue's
  latest Comment or update, newest first: a Comment moves `updated_at`.

  A Goal is `{id, title, description, level, status, parent_id, owner:
  {id, name} | null, created_at, updated_at}`; it is written with `title`,
  `description`, `level`, `status`, `parent_id` and `owner_id`. An Issue is
  `{id, number, identifier, title, description, status, priority, assignee,
  project, goal, parent, created_by, started_at, completed_at, cancelled_at,
  created_at, updated_at}`, the references as `{id, name}` (`{id, title}`
  for a Goal, `{id, identifier, title}` for a parent) or null, and lists leave
  `description` out and add `unresolved_blockers`, an `assignee` is `{id,
  name, kind}` with `kind` `member` or `agent`, how many of its
  Blockers the person may see are not `done`; it is written with `title`, `description`, `status`,
  `priority`, `assignee_id` (a Member) or `assignee_agent_id` (an Agent,
  422 for one that is terminated or of another Guild; setting one clears
  the other), `project_id`, `goal_id` and `parent_id`, any of
  them null to clear it. `PATCH` also takes `blocked_by_ids`, a list of
  Issue ids (422 for the Issue itself, another Guild's Issue or a cycle),
  and an Issue on its own answers with `blocked_by` and `blocking`, each a
  list of `{id, identifier, title, status}` the person may see. A Comment is `{id, body, deleted, author, created_at,
  updated_at, edited}`. An Issue document is `{id, key, title, body,
  format: "markdown", latest_revision_id, latest_revision_number,
  created_by, updated_by, created_at, updated_at}`, written with `title`,
  `body`, `change_summary` and `base_revision_id`, the id of the Revision
  the edit started from (left out for the first save), as Paperclip's
  `baseRevisionId`. A Revision is `{id, number, title, body,
  change_summary, created_by, created_at}`; a Restore's change summary is
  "Restored from revision N". An Activity event is `{id, action, actor:
  {id, name} | null, entity: {type: "issue" | "goal" | "approval" | "agent", id, identifier?,
  title, exists}, details, created_at}`, `exists` false once the Goal or
  Issue is deleted. An Approval is `{id, type, status, payload, requester:
  {id, name} | null, decided_by: {id, name} | null, decision_note,
  decided_at, created_at, updated_at}`, the payload in snake_case (`title`,
  `summary`, `recommended_action`, `next_action_on_approval`, `risks`; for
  a `hire_agent`: `agent_id`, `name`, `job`, `title`, `icon`,
  `capabilities`, `reports_to` (`{id, name}` or null) and `roles`). An
  Approval comment is `{id, body, author: {id, name} | null, created_at}`.
  An Activity event about an Approval has `entity: {type: "approval", id,
  title, exists}`, the payload's title; one about an Agent has `entity:
  {type: "agent", id, title, exists}`, the Agent's name. Its `details` by
  Action:

  | Action | `details` |
  | ------ | --------- |
  | `goal.created` | `title`, `level`, `status` |
  | `goal.updated` | `title`, `changes`: field → `{from, to}` for `title`, `level`, `status`, `parent` (`{id, title}`) and `owner` (`{id, name}`), and `description: true` when the description changed |
  | `goal.deleted` | `title` |
  | `issue.created` | `issue_number`, `issue_title`, `status`, `priority` |
  | `issue.updated` | `issue_number`, `issue_title`, `changes`: field → `{from, to}` for `title`, `status`, `priority`, `assignee` (`{id, name, kind}`), `project` (`{id, name}`), `goal` (`{id, title}`) and `parent` (`{id, identifier, title}`), `description: true` when the description changed, and `blockers: {added, removed}`, lists of `{id, identifier, title}` |
  | `issue.deleted` | `issue_number`, `issue_title` |
  | `issue.comment_added` | `issue_number`, `issue_title`, `comment_id`, `snippet` |
  | `issue.comment_deleted` | `issue_number`, `issue_title`, `comment_id` |
  | `issue.document_created`, `issue.document_updated` | `issue_number`, `issue_title`, `key`, `title`, `revision_number`, and `restored_from` for a Restore |
  | `issue.document_deleted` | `issue_number`, `issue_title`, `key`, `title` |
  | `approval.created` | `type`, `title`, `issue_ids` |
  | `approval.approved`, `approval.rejected`, `approval.revision_requested` | `type`, `title`, `decision_note` |
  | `approval.resubmitted` | `type`, `title` |
  | `approval.comment_added` | `type`, `title`, `comment_id`, `snippet` |
  | `approval.cancelled` | `type`, `title` |
  | `agent.hired` | `name`, `job`, `approval_id` |
  | `agent.updated` | `name`, `changes`: field → `{from, to}` for `name`, `job`, `title`, `icon` and `reports_to` (`{id, name}`), and `capabilities: true` when they changed |
  | `agent.paused`, `agent.resumed`, `agent.terminated` | `name` |
  | `agent.role_added`, `agent.role_removed` | `name`, `role` (the Role's name when it was added or removed) |

  References are stored as ids and answered with their names as they are
  when read, null for none. One that no longer exists, or an Issue or
  Project the person may not view, keeps its id with a null name (or
  title and identifier), so the event never names what is hidden.
- **Consumes:**
  - from guilds: `guilds.Auth`, `guilds.Can(permission)`,
    `guilds.Current(ctx)` (the Guild every Goal and Issue is stored and
    filtered by), `guilds.MemberID(ctx)` (who creates an Issue, and whom
    `assignee=me` means), `guilds.VisibleProjects(ctx, ids)` (which Projects the
    Member may view), `guilds.IssuePrefix(ctx, guild)` (for every Issue
    identifier) and `guilds.IsMember(ctx, guild, member)` (for Assignees and
    owners). It registers `guilds.OnGuildDeleting`, so a Guild with Goals or
    Issues is not deleted.
  - from identity: `identity.Members(ctx, ids)`, for the names of Goal
    owners, Assignees and the Members who created Issues.
  - from projects: `projects.ProjectNames(ctx, guildID, ids)`, for the
    names of Issues' Projects and before an Issue takes a Project (an id it
    does not name is not one of the Guild's), and
    `projects.OnProjectDeleted`: the Project's Issues keep existing and lose
    their Project. `issues.project_id` has no foreign key, since the
    Project is projects' row.
- **Publishes to other contexts** (Go functions, no routes):
  `work.RequestApproval(ctx, guild, hirer, HireAgentRequest)` (a
  `hire_agent` Approval; answers its id), `work.CancelApproval(ctx, guild,
  actor, id)`, `work.OnApprovalDecided(type, f)` (called after an approve or
  reject of that type is stored, with the guild, the Approval's id and type,
  whether it was approved, the decider and the hire's Agent; the same
  Decision made again calls it again, so a failed `f` is healed by deciding
  again, and its error answers the Decision 500; the agents context
  registers for `hire_agent`), `work.RecordActivity(ctx, AgentActivity)`
  (one Activity event about an Agent, with its name kept in the details;
  only the `agent.*` Actions) and `work.OnAgentNames(f)` (the agents
  context names the Guild's Agents that still exist, so the Activity can
  tell `exists`; until it registers, every Agent counts as existing),
  `work.OnAgentAssignable(f)` (the agents context answers whether an Agent
  of the Guild may be an Assignee: it exists and is not terminated; until
  it registers, no Agent may), `work.IssueForRun(ctx, guild, issue)` (an
  Issue's number, identifier, title, description and Agent assignee, for
  a Run's check and its prompt) and `work.ClearAgentAssignee(ctx, guild,
  actor, agent)` (a terminated Agent stops being the Assignee of the open
  Issues, each recorded as `issue.updated` with the terminating person as
  Actor). Work never imports the contexts that call them.

## Why it's shaped this way

- **Goal level without `team`.** Paperclip's levels are `company`, `team`,
  `agent` and `task`. Team is Coolify's word for a Guild, so `company`
  becomes `guild` and `team` is left out rather than mean a second thing.
- **The Issue identifier is derived, not stored.** Paperclip stores
  `identifier` on every Issue and rewrites them all when a company's prefix
  changes. The Bakery stores only the number and joins it with the Guild's
  Issue prefix when it is read, so changing the prefix renames every Issue
  at once and nothing can drift. The cost: an old `OLD-12` link stops
  working after a rename, as it would after Paperclip's rekey.
- **One `manage_work` Permission.** Paperclip splits task permissions
  (`tasks:assign` and its family) for its agents. The Board needs one switch
  for "may change work", seeded into the Member Role so a Member can plan
  work from the start; agents get their own Permissions in a later phase.
- **Two assignee columns, never both set.** As Paperclip's
  `assigneeUserId` and `assigneeAgentId`, an Issue keeps a Member and an
  Agent assignee apart, so each has its own foreign key and `assignee=me`
  never matches an Agent. Work asks agents (through a hook it registers)
  whether an Agent may be assigned, so work never reads agents' tables.
- **Issues hidden by the Project's `view_resources` override.** An Issue has
  no overrides of its own. Whoever may not view a Project may not see the
  work in it either, so the existing Permission overrides already say who
  sees an Issue, with nothing new to set.
- **The Issue prefix lives on the Guild, in guilds.** It is a Guild's
  property, edited on its General page, and work reads it through
  `guilds.IssuePrefix` instead of keeping a copy.
- **A deleted Goal's Sub-goals move up a level.** Paperclip's foreign key
  refuses to delete a Goal that has Sub-goals. Moving them under the
  deleted Goal's parent keeps the rest of the tree where it was, so
  deleting a middle Goal does not scatter its subtree to the top.
- **"Issues", not "Tasks".** Paperclip's pages say Issues while some of its
  code and permissions say tasks. The Bakery uses Issue everywhere a person
  or the code reads it, so one thing has one name.
- **A Textarea with a Preview tab, not a rich Markdown editor.** Paperclip
  writes descriptions and comments in MDXEditor. The Bakery stores the same
  Markdown and writes it in a plain Textarea with Write and Preview tabs,
  which keeps a large editor and its React dependencies out of the
  dashboard. The rendered Markdown is the same.
- **No labels or attachments yet.** Paperclip's Issues carry labels and
  file attachments. Neither is needed for a Board to plan and talk about
  work, and attachments would need storage of their own; they are left out
  until a phase needs them. Read states came with the Inbox.
- **A Viewer sees everything and changes nothing.** Reading needs only
  `view_resources`, so the base Role's Viewers read the Board's Goals,
  Issues and Comments; the dashboard hides New Issue, New Goal, the
  pickers, the comment box and Delete without `manage_work`, and the API
  refuses (403) what a hand-made request tries.
- **Blockers do not move the Issue status.** Paperclip uses unresolved
  Blockers to hold back agents' wake-ups, not to change an Issue's status.
  The Bakery has no agents yet, so `blocked` stays a status a person sets,
  and the Issue page shows unresolved Blockers beside it.
- **Blockers someone cannot see stay in place.** A Blocker in a Project
  hidden from a person is left out of the `blocked_by` and `blocking` they
  read, and is kept when they set `blocked_by_ids`. Hiding never lets
  someone remove what they cannot see.
- **Issue documents are their own aggregate.** A long document saved many
  times must not lock or rewrite its Issue, and Paperclip stores them apart
  (`documents`, `issue_documents`, `document_revisions`). The Bakery folds
  Paperclip's `documents` and `issue_documents` into one `issue_documents`
  table, since an Issue document here belongs to exactly one Issue.
- **No document locks, annotations, feedback votes or system documents.**
  Paperclip uses them for its agents' Runs and review flows, which come
  with agents.
- **A document is saved with Save, not autosaved.** Paperclip autosaves a
  document when its editor loses focus and on a conflict offers to keep the
  draft or overwrite. Here every save is a Revision someone chose to make,
  with an optional change summary, so History is not filled with half-typed
  states; on a conflict the draft stays and the person compares or reloads,
  never overwrites blindly.
- **Activity lives in work, and agents write to it through
  `work.RecordActivity`.** Paperclip keeps one company-wide `activity_log`
  for every entity. Here it is work's own table, and the agents context
  records its events by calling work's published `RecordActivity`, not by
  work subscribing to agents: agents already imports work for its
  Approvals, and work importing agents back would be an import cycle in Go.
  A context of its own for the feed waits until something outside the
  Guild's work and Agents needs it.
- **Recorded after the change, not in its transaction.** One transaction
  changes one aggregate. The Activity event is written by the event's
  handler right after the change is stored, and a failure to record it is
  logged, not shown to the person whose change already succeeded.
- **Nothing is backfilled.** Goals, Issues and Comments from before
  Activity have none; inventing events from `created_at` would give them an
  Actor and a time the log never saw.
- **Hidden by the Issue's current Project.** An event keeps the Issue's id,
  and visibility is the Issue's as it is now, so moving an Issue into a
  Project someone may not view hides its whole history from them, as it
  hides the Issue. A deleted Issue's events keep the Project it was last in.
- **No agents' mode, CSV export or action filter yet.** Paperclip's feed
  has an Agent Actions mode, a CSV export and filters by agent and by kind
  of action. With only the Board acting, entity kind and Actor are the
  filters that mean something.
- **No atomic checkout yet.** Paperclip's checkout takes an agent and a Run
  (`POST /issues/:id/checkout` with `agentId`), so it comes with agents.
- **Read marks and Inbox archives are not Activity.** Paperclip logs
  `issue.read_marked` and `issue.inbox_archived` in its activity log. Here
  they are one person's view of the work, not a change to it, and logging
  them would flood the Guild's feed with every opened Issue, so they are
  their own records and publish nothing.
- **Marking read or archiving needs only `view_resources`.** It changes
  nothing anyone else sees, so a Viewer keeps an Inbox like everyone else.
- **Touched through Activity counts only what was recorded.** The Activity
  table started empty and was not backfilled, so Issues older than it are
  Touched only through creation, assignment and Comments.
- **An Assignee's Last touch is the Issue's last change, not its
  `updated_at`.** Paperclip takes the Issue's `updatedAt` for an
  Assignee. Here a Comment moves `updated_at` too, so every Comment
  would count as the Assignee's own touch and their Issues would never
  be Unread. The Issue's last change is its latest `issue.created` or
  `issue.updated` Activity event instead.
- **No Blocked or All tab yet.** In Paperclip those tabs are filled by
  agents' failed Runs and join requests, which come with agents; Approvals
  are on Mine, Recent and Unread already.
- **Approvals live in work, not a context of their own.** They are the
  Board's decisions on its work and link to its Issues. The agents context
  asks work for a `hire_agent` Approval through `work.RequestApproval` and
  learns the Decision through `work.OnApprovalDecided`, rather than own a
  second kind of decision.
- **A Member can request an Approval from an Issue's page.** Paperclip's UI
  has no form for it; only agents create Approvals there. Without agents
  the feature would be unreachable, so the Issue page has a "Request
  approval" dialog.
- **The payload is snake_case on the wire** (`recommended_action`), like
  the rest of The Bakery's API, where Paperclip's keys are camelCase.
- **Issues are linked when the Approval is requested** (`issue_ids`).
  Paperclip's separate link and unlink routes wait until something needs
  them.
- **The Requester may decide their own Approval** if they hold `approve`,
  as in Paperclip. A Guild that wants four eyes gives its Requesters no
  `approve`.
- **Approvals in the Inbox have no read dot and no Archive.** Paperclip
  keeps an Approval's read and dismissed state in the browser's storage,
  so it is lost on another device. Here the Inbox shows Approvals by their
  status alone (Actionable ones in Mine and Unread), and a Decision is what
  takes one out of Unread.
- **Resubmit is the Requester's alone.** Paperclip lets any board user mark
  an Approval resubmitted, because there the Requester is an agent that
  cannot press the button. Here the Requester is a Member who can.
- **Approvals go with their Guild.** Approvals, their links and their
  Approval comments are removed with their Guild (cascade), like Activity
  events, and never block deleting it.
- **Approvals have no Project.** Everyone with `view_resources` sees every
  Approval and its comments; only Linked issues in Projects someone may not
  view are left out for them.
- **`cancelled` only for a hire; no budget Approvals; no waking the
  Requester.** Paperclip cancels a `hire_agent` Approval when its Agent is
  terminated first, and so does The Bakery. It also cancels Approvals when
  their requesting agent goes away and wakes that agent after a Decision;
  here only people request Approvals, so neither applies yet. The budget
  types come with budgets.
- **No Request revision for a hire.** A `pending_approval` Agent cannot be
  edited, so its Approval's payload is always what the Board sees, and
  there is nothing a revision could change. The Board approves or rejects;
  a Hirer who wants a different Agent terminates this one and hires again.
