# work

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/work`)

## Purpose

Holds the work a Guild's Board plans and tracks together: the Guild's Goals
(a tree of outcomes it works toward), its Issues (the pieces of work, each
with a status, a Priority and an Assignee, tied to a Project and a Goal),
and the Comments people write on an Issue. Every Goal and Issue belongs to
exactly one Guild, and an Issue in a Project follows that Project's
Permission overrides.

It is **not** responsible (yet) for agents, Runs, checkout, blockers, Issue
documents and revisions, the Inbox, Activity or Approvals: later phases of
the guilds goal add them. It does not own Members, Guilds or Projects
either; it stores their ids and asks guilds and projects about them.

## Language

Shared terms (Board, Goal, Goal level, Goal status, Issue, Issue status,
Priority, Assignee, Issue prefix, Issue identifier, Comment) are in
[`glossary.md`](../../glossary.md).

| Term | Meaning |
| ---- | ------- |
| Sub-goal | A Goal whose parent is another Goal. |
| Sub-issue | An Issue whose parent is another Issue. |
| Owner (of a Goal) | The Member a Goal is in the hands of; optional. |
| Status times | An Issue's started, completed and cancelled times, set and cleared by its Issue status changes as the glossary says. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Goal | Belongs to one Guild. Title 1–200 characters. Goal level and Goal status from their lists. Its parent Goal is in the same Guild and is never the Goal itself or one of its Sub-goals (no cycle). Its owner, if any, is a Member of the Guild. |
| Issue | Belongs to one Guild. Its number is unique in the Guild, taken from the Guild's Issue counter (kept in work) when it is created, and never changes or comes back. Title 1–200 characters. Issue status and Priority from their lists. Its parent Issue is in the same Guild and never the Issue itself or one of its Sub-issues (no cycle). Its Assignee is a Member of the Guild, its Project is in the Guild and its Goal is in the Guild. The Status times follow its Issue status: started is set once on the first move to `in_progress`; completed is set on `done` and cancelled on `cancelled`, and each is cleared when the Issue moves back out. |
| Comment | Belongs to one Issue and is written by one Member. Body 1–20000 characters. Only its author edits or deletes it. A deleted Comment keeps its place in the thread, its body gone, shown as "deleted". |

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
- `DeleteIssue()` [`manage_work`]: its Sub-issues lose their parent; its
  Comments go with it.
- `WriteComment(body)` [`manage_work`]: moves the Issue's last update to
  now, so a discussed Issue sorts up in lists, as Paperclip's does. This
  touches only the Issue's timestamp, none of its rules, so it is stored
  with the Comment in one transaction. `EditComment(body)`,
  `DeleteComment()` [`manage_work`, and only the author; anyone else is
  refused (403), and a deleted Comment cannot be changed (409)].

### Domain events

None yet. Inbox and Activity, in a later phase, are where they start.

## Integration

- **Publishes:** the Goals and Issues API (`/api/goals`, `/api/issues`,
  `/api/issues/{issue}/comments`), an Issue addressed by its id or its Issue
  identifier.
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
