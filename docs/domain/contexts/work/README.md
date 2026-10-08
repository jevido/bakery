# work

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/work`)

## Purpose

Holds the work a Guild's Board plans and tracks together: the Guild's Goals
(a tree of outcomes it works toward), its Issues (the pieces of work, each
with a status, a Priority and an Assignee, tied to a Project and a Goal),
the Comments people write on an Issue, which Issues block which (Blockers),
and the Issue documents (plans, specs, notes) kept on an Issue with their
Revisions. Every Goal and Issue belongs to exactly one Guild, and an Issue
in a Project follows that Project's Permission overrides.

It is **not** responsible (yet) for agents, Runs, checkout, document locks,
the Inbox, Activity or Approvals: later phases of the guilds goal add them.
It does not own Members, Guilds or Projects either; it stores their ids and asks guilds and projects about them.

## Language

Shared terms (Board, Goal, Goal level, Goal status, Issue, Issue status,
Priority, Assignee, Issue prefix, Issue identifier, Comment, Blocker, Issue
document, Document key, Revision, Base revision, Restore) are in
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
| Issue | Belongs to one Guild. Its number is unique in the Guild, taken from the Guild's Issue counter (kept in work) when it is created, and never changes or comes back. Title 1–200 characters. Issue status and Priority from their lists. Its parent Issue is in the same Guild and never the Issue itself or one of its Sub-issues (no cycle). Its Assignee is a Member of the Guild, its Project is in the Guild and its Goal is in the Guild. The Status times follow its Issue status: started is set once on the first move to `in_progress`; completed is set on `done` and cancelled on `cancelled`, and each is cleared when the Issue moves back out. Its Blockers are Issues of the same Guild, never the Issue itself, and never form a cycle: an Issue may not be blocked by an Issue that is, directly or through others, blocked by it. |
| Comment | Belongs to one Issue and is written by one Member. Body 1–20000 characters. Only its author edits or deletes it. A deleted Comment keeps its place in the thread, its body gone, shown as "deleted". |
| Issue document | Belongs to one Issue and its Guild. Its Document key is unique per Issue and never changes. Title at most 200 characters (may be empty), body at most 524288 characters, Markdown only. Its Revisions are numbered 1, 2, … without gaps; each save and each Restore adds exactly one Revision, and Revisions are never changed. A save needs the newest Revision as its Base revision (none for the first save), or it is refused. Restoring the newest Revision is refused, since it would change nothing. Deleting the Issue document removes its Revisions. |

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

Reading an Issue's Blockers, Issue documents and Revisions needs what
reading the Issue needs.

### Domain events

None yet. Inbox and Activity, in a later phase, are where they start.

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
  | `GET /api/issues` | `{"issues": [Issue]}`, most recently updated first; filters `status` and `priority` (comma lists), `assignee` (an id, `me` or `none`), `project`, `goal`, `parent` (an id or `none`), `q` (title, description, identifier), `limit` (≤ 200) and `offset` |
  | `POST /api/issues` | 201 `{"issue": Issue + "children": [Issue]}` |
  | `GET /api/issues/{issue}`, `PATCH /api/issues/{issue}` | `{"issue": Issue + "children": [Issue]}` |
  | `DELETE /api/issues/{issue}` | 204 |
  | `GET /api/issues/{issue}/comments` | `{"comments": [Comment]}`, oldest first |
  | `POST /api/issues/{issue}/comments` | 201 `{"comment": Comment}` |
  | `PATCH /api/issues/{issue}/comments/{comment}` | `{"comment": Comment}` |
  | `DELETE /api/issues/{issue}/comments/{comment}` | 204; the Comment keeps its place in the list, `deleted` and without its body |
  | `GET /api/issues/{issue}/documents` | `{"documents": [Issue document]}`, by Document key, without `body` |
  | `GET /api/issues/{issue}/documents/{key}` | `{"document": Issue document}`; 404 for an unknown key |
  | `PUT /api/issues/{issue}/documents/{key}` | 201 `{"document": Issue document}` on the first save, 200 after; 409 for a stale `base_revision` |
  | `DELETE /api/issues/{issue}/documents/{key}` | 204 |
  | `GET /api/issues/{issue}/documents/{key}/revisions` | `{"revisions": [Revision]}`, newest first |
  | `POST /api/issues/{issue}/documents/{key}/revisions/{revision}/restore` | `{"document": Issue document}`; 409 for the newest Revision |

  A Goal is `{id, title, description, level, status, parent_id, owner:
  {id, name} | null, created_at, updated_at}`; it is written with `title`,
  `description`, `level`, `status`, `parent_id` and `owner_id`. An Issue is
  `{id, number, identifier, title, description, status, priority, assignee,
  project, goal, parent, created_by, started_at, completed_at, cancelled_at,
  created_at, updated_at}`, the references as `{id, name}` (`{id, title}`
  for a Goal, `{id, identifier, title}` for a parent) or null, and lists leave
  `description` out and add `unresolved_blockers`, how many of its
  Blockers the person may see are not `done`; it is written with `title`, `description`, `status`,
  `priority`, `assignee_id`, `project_id`, `goal_id` and `parent_id`, any of
  them null to clear it. `PATCH` also takes `blocked_by_ids`, a list of
  Issue ids (422 for the Issue itself, another Guild's Issue or a cycle),
  and an Issue on its own answers with `blocked_by` and `blocking`, each a
  list of `{id, identifier, title, status}` the person may see. A Comment is `{id, body, deleted, author, created_at,
  updated_at, edited}`. An Issue document is `{id, key, title, body,
  latest_revision, created_by, updated_by, created_at, updated_at}`, written
  with `title`, `body`, `change_summary` and `base_revision` (null or left
  out for the first save). A Revision is `{revision, title, body,
  change_summary, author, created_at}`. (Tasks 02 and 03 of phase 33
  settle the exact fields.)
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
- **No labels, attachments or read states yet.** Paperclip's Issues carry
  labels, file attachments and per-person read markers. None of them is
  needed for a Board to plan and talk about work, and attachments would
  need storage of their own; they are left out until a phase needs them.
  The Inbox, in a later phase, is where read states would start.
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
- **No atomic checkout yet.** Paperclip's checkout takes an agent and a Run
  (`POST /issues/:id/checkout` with `agentId`), so it comes with agents.
