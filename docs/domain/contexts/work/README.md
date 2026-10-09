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
the Board to make, with their Linked issues and Approval comments, and the
Guild's Routines: recurring work that, on a schedule, on a signed webhook from outside or when someone
presses Run, creates an Execution Issue for an Agent, and the Conversations:
an Issue a Member holds with one Agent of the Guild to chat with it, its
Comments the messages. Every
Goal, Issue, Approval and Routine belongs to exactly one Guild, and an Issue in a
Project follows that Project's Permission overrides.

It also holds each Issue's Checkout: which live Run of its Agent assignee
works on it now. It is **not** responsible (yet) for document locks: a
later phase of the guilds goal may add them. Agents and their Runs
are the agents context's; work holds only an Issue's Agent assignee and the
Run id of its Checkout, asks agents whether that Run is live, and
tells agents when an Agent is assigned an open Issue or one of its Issues
gets a Comment, so agents can wake it.
It does not own Members, Guilds or Projects either; it stores their ids and asks guilds and projects about them.

## Language

Shared terms (Board, Goal, Goal level, Goal status, Issue, Issue status,
Priority, Assignee, Issue prefix, Issue identifier, Comment, Blocker, Issue
document, Document key, Revision, Base revision, Restore, Activity, Activity event,
Action, Actor, Inbox, Inbox tab, Touched, Last touch, Unread, Read mark,
Inbox archive, Resurface, Approval, Approval type, Approval status,
Actionable, Requester, Decision, Decision note, Request revision, Resubmit,
Approval comment, Linked issue, Issue's Application, Work product, Routine,
Routine status, Routine trigger, Next run, Routine run, Routine run status,
Concurrency policy, Catch-up policy, Execution Issue, Live execution Issue,
Webhook trigger, Public id, Webhook secret, Signing mode, Replay window,
Webhook delivery, Idempotency key, Routine variable, Built-in variable,
Routine revision, Snapshot, Restore (a Routine revision), Conversation,
Conversation agent, Conversation owner, Conversation state, Session
boundary, New session) are in
[`glossary.md`](../../glossary.md).

| Term | Meaning |
| ---- | ------- |
| Sub-goal | A Goal whose parent is another Goal. |
| Sub-issue | An Issue whose parent is another Issue. |
| Owner (of a Goal) | The Member a Goal is in the hands of; optional. |
| Blocked by | The Blockers of an Issue: the Issues it waits on. |
| Blocking | The Issues that have this Issue as a Blocker. |
| Resolved Blocker | A Blocker whose Issue status is `done`. An Issue with an unresolved Blocker is shown as waiting on it, whatever its own status. |
| Draft (Routine) | A Routine without an Agent assignee: it cannot run and its Schedule triggers do not fire. |
| Schedule trigger | A Routine trigger of kind `schedule`. |
| Status times | An Issue's started, completed and cancelled times, set and cleared by its Issue status changes as the glossary says. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Goal | Belongs to one Guild. Title 1–200 characters. Goal level and Goal status from their lists. Its parent Goal is in the same Guild and is never the Goal itself or one of its Sub-goals (no cycle). Its owner, if any, is a Member of the Guild. |
| Issue | Belongs to one Guild. Its number is unique in the Guild, taken from the Guild's Issue counter (kept in work) when it is created, and never changes or comes back. Title 1–200 characters. Issue status and Priority from their lists. Its parent Issue is in the same Guild and never the Issue itself or one of its Sub-issues (no cycle). Its Assignee is a Member of the Guild or an Agent of the Guild that is not terminated (its Agent assignee), never both; terminating the Agent clears it as Assignee of its open Issues. Its Project is in the Guild and its Goal is in the Guild. Its Issue's Application, if any, is an Application of its Project: an Issue without a Project has none, and moving the Issue to another Project (or out of one) clears it. The Status times follow its Issue status: started is set once on the first move to `in_progress`; completed is set on `done` and cancelled on `cancelled`, and each is cleared when the Issue moves back out. Its Blockers are Issues of the same Guild, never the Issue itself, and never form a cycle: an Issue may not be blocked by an Issue that is, directly or through others, blocked by it. Its Checkout (`checkout_run_id`, `checked_out_at`) names at most one Run, a Run of its Agent assignee; Checkout moves it from one of the expected statuses (`todo`, `backlog`, `blocked` by default, as Paperclip's MCP server's), or from `in_progress` when it is already the Agent's, to `in_progress`; an Issue assigned to anyone else cannot be checked out, and an Issue without Assignee takes the checking-out Agent as its Agent assignee. While that Run is `running`, an Agent's change to the Issue from any other Run is 409; once it is not, the Checkout is a Stale checkout, which the same Agent's next Run takes over. A new Assignee, set by anyone, clears the Checkout. Release clears it, moves `in_progress` back to `todo` and, on an open Issue, clears the Agent assignee. It is created by a Member or an Agent actor. An Issue may be a Conversation: its `conversation_agent_id`, `conversation_member_id`, `conversation_state` and `conversation_boundary_comment_id` are all null, or the first three are all set. A Conversation is the only one of its Guild, Conversation agent and Conversation owner; its Agent assignee is its Conversation agent and never changes (Release does not clear it); it has no Member assignee, Project, Goal, parent Issue, Sub-issues, Blockers (in either direction) or Issue's Application; it is created `in_review` (as Paperclip's) with the title "Chat with <Agent name>" (cut to 200 characters) and Conversation state `waiting`, and never becomes `done` or `cancelled`. Its Session boundary, if any, is a Comment of the same Issue. Each refused change is 422, "is fixed for a conversation" on the field, or "a conversation cannot be done or cancelled" on `status`. |
| Comment | Belongs to one Issue and is written by one Member or one Agent actor (`author_agent`). Body 1–20000 characters. Only its author edits or deletes it. A deleted Comment keeps its place in the thread, its body gone, shown as "deleted". |
| Issue document | Belongs to one Issue and its Guild. Its Document key is unique per Issue and never changes. Title at most 200 characters (may be empty), body at most 524288 characters, Markdown only. Its Revisions are numbered 1, 2, … without gaps; each save and each Restore adds exactly one Revision, and Revisions are never changed. A save needs the newest Revision as its Base revision (none for the first save), or it is refused. Restoring the newest Revision is refused, since it would change nothing. A Revision's author is a Member or an Agent actor. Deleting the Issue document removes its Revisions. |
| Activity event | Append-only. Belongs to one Guild and has one Actor, a Member or an Agent actor (none once that Member's account is gone, and none for what The Bakery recorded on its own: a Pull request or Preview changing on the git host) and one Action from the glossary's list, about exactly one Goal, Issue, Approval, Agent, Budget, Budget incident, Routine or Skill. It keeps the Issue's number and title, the Goal's title, the Approval's type and payload title, or the Agent's or Skill's name, as they were, so an event about a deleted one still reads, and the Issue's Project, so a deleted Issue's events stay hidden where it was. It is never changed, and goes only with its Guild. |
| Approval | Belongs to one Guild. Approval type and Approval status from their lists; it starts `pending`. A `request_board_approval` payload has a `title` of 1–200 characters and optional `summary`, `recommended_action` and `next_action_on_approval` (each at most 20000 characters) and `risks` (at most 20 strings of at most 500 characters), and nothing else. A `hire_agent` payload is `agent_id`, `name`, `job`, `title`, `icon`, `reports_to` (`{id, name}` or null), `capabilities` and `roles` (Role names), as the agents context sends it; its title is "Hire Agent: <name>". A `hire_agent` Approval is created only through `work.RequestApproval`, never gets Request revision (its Agent cannot change while it waits, so there is nothing to revise), and becomes `cancelled` when its Agent is terminated before a Decision; `cancelled` is not Actionable and never changes again. A `budget_override_required` payload is `scope_type`, `scope_id`, `scope_name`, `metric`, `window`, `threshold`, `amount`, `observed`, `warn_percent`, `window_start`, `window_end`, `budget_id` and `guidance`, as the agents context sends it; its title is "Budget override: <scope name>". It is created only through `work.RequestBudgetOverride`, has no Requester, becomes `cancelled` when its Budget goes with its Agent or Project before a Decision, and is decided only through `work.DecideBudgetOverride` when the agents context resolves its Budget incident: Approve, Reject, Request revision and Resubmit refuse it (422 `resolve the budget incident on the Costs page`), as Paperclip does. Approve and reject only from `pending` or `revision_requested` (Actionable); Request revision only from `pending`; Resubmit only from `revision_requested`, which clears the decider, the decision time and the Decision note. A Decision records its decider, time and optional Decision note (at most 20000 characters). Making the same Decision again on an Approval that already has it answers the Approval unchanged and records nothing, as Paperclip's `applied: false`; any other move from a status that does not allow it is refused (422). Its Linked issues are Issues of the same Guild, set when it is requested. Its Approval comments each have one author and a body of 1–20000 characters, and are never edited or deleted. The Approval, its links and its Approval comments go with their Guild; a Linked issue's link goes with the Issue. |
| Work product | Belongs to one Issue and its Guild, and goes with the Issue. Its type is `pull_request` or `preview_url`, and it names the Issue's Application it came from and the Pull request's number (`external_id`); at most one of each type per Issue and number. A `pull_request` has a Provider, a URL, a title and status `open`, `merged` or `closed`: `open` → `merged` or `closed`, `closed` → `open` when reopened, and `merged` never changes again. A `preview_url` has state `deploying`, `ready` (with the Preview's link), `failed` or `removed`: any state may follow `deploying`, `ready` or `failed`, and `removed` comes back to `deploying` only with a new Preview Deployment. It records who made it: the Run and its Agent, or the Member, or nobody when The Bakery recorded a Pull request someone opened by hand. |
| Routine | Belongs to one Guild. Title 1–200 characters; description at most 20000 characters, Markdown. Priority, Routine status, Concurrency policy and Catch-up policy from their lists. Its Agent assignee, if any, is an Agent of the Guild that is not terminated, asked of agents as for an Issue; terminating that Agent clears it, making the Routine a Draft, as terminating clears an open Issue's Agent assignee. Its Project, Goal and parent Issue, if any, are the Guild's. Its Routine status moves `active` ↔ `paused`, and either → `archived`; nothing leaves `archived`, and an archived Routine is not changed again. Deleting its Project deletes it, with its Routine triggers and Routine runs, as Paperclip's cascade. Its Routine triggers are part of it: a `schedule` one has a cron expression of five fields or one of the shortcuts a Scheduled backup takes, and a time zone that `time.LoadLocation` loads; an empty time zone is `UTC`; its `next_run_at` is the next tick after now, recomputed whenever its cron expression, time zone or `enabled` changes, or its Routine's status or Agent assignee does, and none while it is off or its Routine is paused, archived or a Draft (resuming counts from then, so ticks missed while paused never fire); an `api` one has neither. A `webhook` one has no cron and no `next_run_at`, but a Public id, a Webhook secret (stored encrypted with `facades.Crypt()`), a Signing mode and a Replay window (30 to 86400 seconds, 300 by default); its Signing mode and Replay window can change, its Public id never; rotating its secret replaces it at once, so the old one stops working, and clears its last Webhook delivery. A label is at most 100 characters. An archived Routine's triggers are not added, changed or deleted (409). It records `last_triggered_at`, the last time it ran. Its Routine variables are part of it: names unique, kept in step with the `{{name}}` placeholders in its title and description (Built-in variables aside) whenever either changes, a default must have its variable's type, and a `select` has at least one option and a default among them. A Schedule trigger can only be added or enabled while every required variable has a default, else 422 `trigger` naming them; a change to the Routine or its variables that leaves an enabled Schedule trigger's required variable without a default is 422 as well. Its Routine revisions are part of it: numbered 1, 2, … without gaps, exactly one added under the Routine's lock right after each change to what the Snapshot holds (the append itself locks the Routine's row, so two never share a number), with Paperclip's Change summary (create: "Created routine"; a change, archiving included: "Updated routine"; a trigger added, changed or deleted: "Created <kind> trigger", "Updated <kind> trigger" or "Deleted <kind> trigger"; Rotate secret: "Rotated webhook trigger secret"; an Agent's termination clearing the Agent assignee: "Agent terminated"; Restore: "Restored from revision N"), never changed, and deleted with the Routine. `latest_revision_id` and `latest_revision_number` always name the newest. A change given a Base revision that is not the newest is refused (409). Restore is refused for the newest revision and for an archived Routine (409), and when the Snapshot's Agent assignee can no longer be assigned (422, as Paperclip's `assertAssignableAgent`); a Goal or parent Issue gone since is cleared. Restore puts back the Snapshot's fields, Routine variables and Routine triggers: a trigger in the Snapshot that still exists is updated to it, one added since is deleted, and a Webhook trigger deleted since is recreated with the same id but a new Public id and Webhook secret; Next runs are recomputed. |
| Routine run | Belongs to one Routine and its Guild, and goes with the Routine. Append-only except its Routine run status: it starts `received` and moves once to `issue_created`, `coalesced`, `skipped` or `failed`; `issue_created` moves to `completed` when its Execution Issue becomes `done`, or `failed` when it becomes `cancelled` or `blocked`; `failed` goes back to `issue_created` when that Issue is reopened, as Paperclip's. `coalesced` and `skipped` name the Live execution Issue they were linked to and never change again. Its source is `schedule` (with its Schedule trigger), `manual` (with the Member who pressed Run), `api` (with its `api` Routine trigger, if one was named) or `webhook` (with its Webhook trigger and its Idempotency key, if the delivery had one; unique per Webhook trigger). It keeps the values of the Routine variables it ran with. It records the newest Routine revision when it was created (`routine_revision_id`), none for Routine runs from before revisions. |
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
  with the Comment in one transaction. A Comment by the Issue's own Agent
  assignee does not wake it (Paperclip skips self-wakes). `EditComment(body)`,
  `DeleteComment()` [`manage_work`, and only the author; anyone else is
  refused (403), and a deleted Comment cannot be changed (409)].
- `OpenConversation(agent)` [`manage_work`; a Member only, an Agent
  principal is 403]: answers the asking Member's Conversation with that
  Agent, or creates it under the Guild's Issue counter. The Agent must be
  the Guild's and not terminated (422 `agent_id`). Two requests at once
  answer the same Conversation. Recorded as `issue.conversation_opened`.
- `WriteComment(body)` on a Conversation [`manage_work`, and only its
  Conversation owner, or an Agent principal only as its Conversation
  agent; anyone else is 403]: in the same transaction, the owner's message
  makes it `active`, a New session (`/new`) becomes its Session boundary
  and leaves the Conversation state as it was, and the Conversation
  agent's Comment makes it `waiting`. A Comment an Agent writes keeps its
  Run's id (`run_id`).
- `SaveDocument(key, title, body, change summary, base revision)`
  [`manage_work`]: creates the Issue document on its first save, and adds
  a Revision on every save; a stale Base revision is refused (409).
- `Checkout(expected statuses)` [an Agent principal with `manage_work`,
  only; a person is 403]: atomically takes the Issue for the Agent's Run,
  as the Issue's invariants say; another live Run's Checkout, or a status
  outside the expected ones, is 409 naming the holder or the status.
  Taking over a Stale checkout of the same Agent is a Checkout like any
  other. Recorded as `issue.checked_out`.
- `Release()` [the Run holding the Checkout only; 409 for any other]: gives
  the Issue back, as the Issue's invariants say. Recorded as
  `issue.released`.
- `SetApplication(application)` [`manage_work`], as part of
  `CreateIssue` and `ChangeIssue` (`application_id`, null clears): 422
  `the application is not in the issue's project` for an Application of
  another Project, or for an Issue without a Project. Recorded as
  `issue.application_changed`.
- `OpenPullRequest(title, body)` [`manage_work` on the Issue's Project;
  for a Run key, only the Run holding the Checkout, 409 otherwise]: asks
  deployments to open (or find) the Pull request from the Issue's Agent
  branch into its Application's branch, and records it as a
  `pull_request` Work product. Title defaults to `<Issue identifier>
  <Issue title>`, body to a line linking the Issue page. 422 without an
  Issue's Application, a Git host token, a known Provider, or a pushed
  Agent branch. An open Pull request it already recorded is answered
  unchanged.
- `FollowPullRequest(event)` and `FollowPreview(event)` [The Bakery
  itself, from deployments' hooks]: move the Work products of the
  Application and Pull request number, as the Work product's invariants
  say. An `opened` event whose head is an Agent branch of an Issue of the
  Guild with that Application records a `pull_request` Work product when
  there is none yet.
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
  and `budget_override_required` come only from the agents context
  (below).
- `Approve(note)`, `Reject(note)` [`approve`]: a Decision on an Actionable
  Approval; never on a `budget_override_required` one (422).
- `RequestRevision(note)` [`approve`]: on a `pending` Approval, never a
  `hire_agent` or `budget_override_required` one.
- `Resubmit(payload)` [`manage_work`, and only the Requester; anyone else
  is refused (403)]: on a `revision_requested` Approval, replacing the
  payload when one is given.
- `CommentOnApproval(body)` [`manage_work`].
- For other contexts, without a route of their own: `work.RequestApproval`
  (a `hire_agent` Approval, its Requester the Hirer, checked by the
  calling context), `work.RequestBudgetOverride` (a
  `budget_override_required` one with no Requester), `work.CancelApproval` (an Actionable `hire_agent`
  Approval whose Agent was terminated, or a `budget_override_required` one
  whose Budget went), `work.DecideBudgetOverride` (a
  Decision on an Actionable `budget_override_required` Approval, approved
  or rejected, by the person who resolved its Budget incident, with their
  Decision note; recorded as `approval.approved` or `approval.rejected`)
  and `work.RecordActivity` (one Activity event about an Agent, a Budget,
  a Budget incident or a Skill, with its Actor, Action and details).

Reading Approvals, their Linked issues and Approval comments needs
`view_resources`. An Approval has no Project, so everyone who may read the
Guild's work sees it; a Linked issue in a Project the person may not view
is left out of what they read.

Reading an Issue's Blockers, Issue documents and Revisions needs what
reading the Issue needs.

- `CreateRoutine(title, description, assignee, project, goal, parent,
  priority, status, concurrency policy, catch-up policy)`,
  `ChangeRoutine(..., base revision)` (pausing, resuming and archiving
  included) [`manage_work`]. A Base revision is optional; a stale one is
  refused (409).
- `AddTrigger(kind, cron, time zone, enabled, label, signing mode, replay
  window)`, `ChangeTrigger(...)` (never its kind or Public id),
  `DeleteTrigger()` [`manage_work`; an Agent only on a Routine assigned to
  itself]. Adding a `webhook` one answers its Webhook secret, once.
- `RotateTriggerSecret()` [`manage_work`; an Agent only on a Routine
  assigned to itself]: a `webhook` Routine trigger only (422 otherwise);
  answers the new Webhook secret, once.
- `RoutineRevisions()` [`view_resources`, as reading the Routine]: newest
  first, at most 100.
- `RestoreRoutineRevision(revision)` [`manage_work`; an Agent only on a
  Routine assigned to itself and only a revision whose Agent assignee is
  itself, else 403]: answers the restored Routine, the new Routine
  revision and the Webhook secret of each recreated Webhook trigger, once.
- `FireWebhookTrigger(public id, headers, body)` [no Session; the Signing
  mode is the authentication]: an unknown Public id, or a trigger of an
  archived Routine, is 404; a paused Routine or a disabled trigger is 409;
  a body that is not JSON is 415, a JSON value that is not an object 400,
  one over 1 MB 413. Bad credentials, or a `hmac_sha256` timestamp outside
  the Replay window, are 401 and record the rejected Webhook delivery.
  Every signature is compared in constant time. A delivery whose
  Idempotency key this trigger has seen answers that Routine run and makes
  none. Otherwise it records the accepted Webhook delivery and runs as
  `RunRoutine` with source `webhook` and Actor nobody.
- `RunRoutine(source, trigger, variables)` [`manage_work` for `manual` and `api`; the
  scheduler for `schedule`]: refused for an archived Routine (409) and a
  Draft (422 `default agent required`), and for a `schedule` firing of a
  paused Routine (409); `manual` and `api` work while paused. A named
  Routine trigger must be this Routine's (403), `enabled` (409) and of the
  source's kind (422); an unknown one is 422. A refused Run records
  nothing. Otherwise it records a Routine run and, unless the Concurrency
  policy finds a Live execution Issue, creates its Execution Issue through
  the same path as `CreateIssue`: Issue status `todo`, the Routine's title,
  description, Priority, Project, Goal, parent and Agent assignee, created
  by whoever triggered it (the Member who pressed Run or called the API,
  or the Agent principal; nobody for a `schedule` firing). Creating it
  publishes `IssueCreated` and wakes the Agent through `OnIssueAssigned`
  as any assignment does. An Issue that cannot be created (its Agent was
  terminated meanwhile) fails the Routine run with the reason. It sets the
  Routine's `last_triggered_at` and the trigger's `last_fired_at` and
  `last_result` (the Routine run status). Its Routine variables' values
  come from `variables` (for `manual` and `api`), from a Webhook
  delivery's payload (its top-level fields, then its `variables` object,
  as Paperclip's `collectProvidedRoutineVariables`) or from their defaults
  (always, for `schedule`); a missing required variable or a value of the
  wrong type is 422 `variables.<name>` and records nothing. The Execution
  Issue's title and description have every placeholder replaced by its
  value and the Built-in variables by theirs. Two Routine runs of one Routine
  take turns on a Postgres advisory lock, so both cannot miss each other's
  Execution Issue. The Routine run follows its Execution Issue's status
  from then on (see the Routine run's invariants); deleting that Issue
  unlinks it, failing it ("Execution issue deleted") if it was still
  `issue_created`.
- The scheduler (`work.Start`): a ticker that fires each `enabled`
  Schedule trigger of an `active` Routine whose `next_run_at` has passed,
  once: it claims the trigger by moving its `next_run_at` on in the same
  update that checks the old value, so two ticks (or two processes) never
  fire it twice. `skip_missed` fires once however many ticks it missed;
  `enqueue_missed_with_cap` fires once per missed tick, at most 25, and
  once only for a cron that ticks more often than hourly (more than 24
  ticks in the day after now, as Paperclip's `isSubHourlyCronExpression`).
  A capped trigger's `next_run_at` stays at the first tick it did not
  make up, so the next tick makes up the rest.

Reading Routines, their Routine triggers and Routine runs needs
`view_resources`; a Routine with a Project also needs `view_resources` in
that Project, else it is hidden (404), as an Issue. An Agent principal may
read every Routine the Agent may view, and create, change, add triggers to
and run only Routines assigned to itself; it never assigns one to another
Agent (403), as Paperclip's access rules for agents.

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
| `ConversationOpened` | `OpenConversation`, when it created one | `issue.conversation_opened` | the Conversation agent's id and name |
| `IssueCheckedOut` | `Checkout` | `issue.checked_out` | the Run |
| `IssueReleased` | `Release` | `issue.released` | the Run |
| `DocumentSaved` | `SaveDocument`, `RestoreRevision` | `issue.document_created` on the first save, else `issue.document_updated` | Document key, title, Revision number, and the restored Revision's number for a Restore |
| `DocumentDeleted` | `DeleteDocument` | `issue.document_deleted` | Document key, title |
| `IssueApplicationChanged` | `SetApplication`, when it changed | `issue.application_changed` | the Application's id and name, from → to |
| `PullRequestOpened` | `OpenPullRequest`, when it recorded a new one; `FollowPullRequest` for one opened by hand | `issue.pull_request_opened` | Provider, number, URL |
| `WorkProductMoved` | `FollowPullRequest`, `FollowPreview`; Actor nobody (The Bakery itself) | `issue.pull_request_merged`, `issue.pull_request_closed`, `issue.preview_ready`, `issue.preview_failed` (other moves record none) | number, URL; a Pull request's Provider |
| `ApprovalRequested` | `RequestApproval` | `approval.created` | Approval type, the payload's title, the Linked issues |
| `ApprovalApproved` | `Approve`, when it changed something | `approval.approved` | Approval type, the payload's title, Decision note |
| `ApprovalRejected` | `Reject`, when it changed something | `approval.rejected` | Approval type, the payload's title, Decision note |
| `RevisionRequested` | `RequestRevision` | `approval.revision_requested` | Approval type, the payload's title, Decision note |
| `ApprovalResubmitted` | `Resubmit` | `approval.resubmitted` | Approval type, the payload's title |
| `ApprovalCommentWritten` | `CommentOnApproval` | `approval.comment_added` | the Approval comment and its first 140 characters |
| `ApprovalCancelled` | `CancelApproval` | `approval.cancelled` | Approval type, the payload's title |
| `RoutineCreated` | `CreateRoutine` | `routine.created` | title |
| `RoutineChanged` | `ChangeRoutine`, when something changed and it is not archiving | `routine.updated` | each changed field, from → to |
| `RoutineArchived` | `ChangeRoutine` to `archived` | `routine.archived` | title |
| `RoutineTriggerAdded` | `AddTrigger` | `routine.trigger_created` | kind, label, cron expression and time zone |
| `RoutineTriggerChanged` | `ChangeTrigger`, when something changed | `routine.trigger_updated` | each changed field, from → to |
| `RoutineTriggerDeleted` | `DeleteTrigger` | `routine.trigger_deleted` | kind, label |
| `RoutineRunTriggered` | `RunRoutine` | `routine.run_triggered` | source, Routine run status, the Execution Issue; Actor nobody for a `schedule`, `api` or `webhook` firing |
| `RoutineTriggerSecretRotated` | `RotateTriggerSecret` | `routine.trigger_secret_rotated` | the trigger's label |
| `RoutineRevisionRestored` | `RestoreRoutineRevision` | `routine.revision_restored` | the new and the restored-from revision numbers, the number of triggers |
| `WebhookDeliveryRejected` | `FireWebhookTrigger`, refused for bad credentials or outside the Replay window | `routine.webhook_rejected` | the trigger's label and the reason; Actor nobody |

The agents context's own events (`AgentHired`, `AgentUpdated`,
`AgentPaused`, `AgentResumed`, `AgentTerminated`, `AgentRoleAdded`,
`AgentRoleRemoved`, `RunStarted`, `RunFinished`) reach the Activity through
`work.RecordActivity` as `agent.hired`, `agent.updated`, `agent.paused`,
`agent.resumed`, `agent.terminated`, `agent.role_added`,
`agent.role_removed`, `run.started` and `run.finished`, each about the
Agent. Its Budget events reach it as `budget.updated` (about the Budget,
the person as Actor), `budget.soft_threshold_crossed` and
`budget.hard_threshold_crossed` (about the Budget incident, with no Actor,
since nobody did it) and `budget.incident_resolved` (about the Budget
incident, the person as Actor). Its Skill events reach it as
`skill.created`, `skill.file_updated`, `skill.file_deleted` and
`skill.deleted` (about the Skill) and `agent.skills_synced` (about the
Agent), the person as Actor.

Every Issue event also carries the Issue's number, title and Project; every
Goal event the Goal's title, and every Approval event the Approval's
type and the payload's title. Every Routine event carries the Routine's
title and Project.

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
  | `GET /api/chats` | `{"issues": [Issue]}`, the asking Member's Conversations, most recently updated first |
  | `GET /api/chats/{agent_id}` | `{"issue": Issue + "children": []}` the asking Member's Conversation with that Agent, or `{"issue": null}` |
  | `POST /api/chats/{agent_id}` | `{"issue": Issue + "children": []}`, 200 whether it was opened now or before (`manage_work`) |
  | `GET /api/routines`, `POST /api/routines` | `{"routines": [Routine]}`, each with its `last_run`; 201 `{"routine": Routine}`. A Routine run is `{id, routine: {id, title}, source, status, triggered_at, completed_at, failure_reason, trigger: {id, kind, label} or null, issue: {id, identifier, title, status} or null}`, and an Issue names the Routine it came from in `routine: {id, title}` or null |
  | `GET /api/routines/{id}`, `PATCH /api/routines/{id}` | `{"routine": Routine + "triggers": [Routine trigger] + "recent_runs": [Routine run]}` |
  | `GET /api/routines/{id}/runs`, `GET /api/routine-runs` | `{"routine_runs": [Routine run]}`, newest first, of the Routine or of every Routine the person may view (Paperclip's Recent Runs); `limit` 1 to 200, 50 by default |
  | `POST /api/routines/{id}/triggers` | 201 `{"trigger": Routine trigger}`; for a `webhook` one also `"secret_material": {webhook_path, webhook_secret}`, the only time the secret is shown. A Routine trigger carries, for a `webhook` one, `signing_mode`, `replay_window_sec`, `webhook_path` (`/api/routine-triggers/public/{public_id}/fire`; the dashboard adds its own origin), `last_rotated_at` and `last_delivery: {status, received_at}` or null, and never the secret |
  | `GET /api/routines/{id}/revisions` | `{"revisions": [Routine revision]}`, newest first, at most 100. A Routine revision is `{id, routine_id, revision_number, title, description, snapshot: {version: 1, routine: {...}, triggers: [...]}, change_summary, restored_from_revision_id, author: {kind: "member" or "agent", id, name, icon (an Agent's)} or null, created_at}`, the Snapshot in snake_case too. A Routine carries `latest_revision_id` and `latest_revision_number`, and a Routine run its `routine_revision_id` (null before revisions). `PATCH /api/routines/{id}` takes an optional `base_revision_id`; a stale one is 409 `{"message", "current_revision_id", "current_revision_number"}`, as an Issue document's |
  | `POST /api/routines/{id}/revisions/{revision}/restore` | `{"routine", "revision", "restored_from_revision_id", "restored_from_revision_number", "secret_materials": [{"trigger_id", "webhook_path", "webhook_secret"}]}`, one secret per recreated Webhook trigger, shown only this once; 409 for the newest revision or an archived Routine, 422 when the Snapshot's Agent assignee cannot be assigned |
  | `PATCH /api/routine-triggers/{id}`, `DELETE /api/routine-triggers/{id}` | `{"trigger": Routine trigger}`, 204 |
  | `POST /api/routine-triggers/{id}/rotate-secret` | `{"trigger": Routine trigger, "secret_material": {webhook_path, webhook_secret}}` |
  | `POST /api/routine-triggers/public/{public_id}/fire` | no Session. 202 `{"routine_run": {id, routine_id, trigger_id, source, status, triggered_at, issue_id, coalesced_into_run_id}}`, ids only since the sender is no Member (the first one again for a repeated Idempotency key); 400 for a JSON value that is not an object, 401 for bad credentials, 404 for an unknown Public id, 409 for a paused Routine or a disabled trigger, 413 for a body over 1 MB, 415 for a body that is not JSON, 422 `variables.<name>` for a missing or wrong variable |
  | `POST /api/routines/{id}/run` | 202 `{"routine_run": Routine run}`; body optional: `{"trigger_id"}` names an `api` Routine trigger and makes it an `api` Routine run, else it is `manual`; `{"variables": {name: value}}` fills the Routine variables. A Routine carries `variables: [{name, label, type, default_value, required, options}]`, and a Routine run its `variables: {name: value}` with the Built-in variables too (null for a Routine without placeholders) |
  | `GET /api/issues/{issue}/documents/{key}` | `{"document": Issue document}`; 404 for an unknown key, 422 for a malformed one |
  | `PUT /api/issues/{issue}/documents/{key}` | 201 `{"document": Issue document}` on the first save, 200 after; 409 for a missing or stale `base_revision_id` (with `current_revision_id` and `current_revision_number`) or a `base_revision_id` on a new key |
  | `DELETE /api/issues/{issue}/documents/{key}` | 204 |
  | `GET /api/issues/{issue}/documents/{key}/revisions` | `{"revisions": [Revision]}`, newest first |
  | `POST /api/issues/{issue}/documents/{key}/revisions/{revision}/restore` | `{"document": Issue document}`; 409 for the newest Revision |
  | `POST /api/issues/{issue}/pull-requests` | `{title?, body?}`; 201 `{"work_product": Work product}` for a new Pull request, 200 with the same one when the Issue already has it open; 422 with a readable message when it cannot be opened |
  | `GET /api/activity` | `{"activity": [Activity event]}`, newest first; filters `entity` (`issue`, `goal`, `approval`, `agent`, `budget`, `budget_incident` or `routine`), `entity_id` (one Goal, Issue, Approval, Agent, Budget, Budget incident or Routine of that `entity`; only with `entity`), `actor` (a Member id, or `agent:<id>`), `before` (an Activity event id, for the next page) and `limit` (default 50, 1 to 200); anything else in them is 422. A page is never short while older events the person may see are left |
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

  Conversations are left out of `GET /api/issues` unless it is given `q`
  (as Paperclip's search finds them), and always out of its counts, the
  Inbox and `InboxCount`, a Goal's or a Project's Issues and Issue counts,
  Sub-issues, `OpenIssuesOfAgent` and `InboxOfAgent`.

  A Goal is `{id, title, description, level, status, parent_id, owner:
  {id, name} | null, created_at, updated_at}`; it is written with `title`,
  `description`, `level`, `status`, `parent_id` and `owner_id`. An Issue is
  `{id, number, identifier, title, description, status, priority, assignee,
  project, goal, parent, created_by, started_at, completed_at, cancelled_at,
  created_at, updated_at}`, the references as `{id, name}` (`{id, title}`
  for a Goal, `{id, identifier, title}` for a parent) or null, and lists leave
  `description` out and add `unresolved_blockers`, an `assignee` is `{id,
  name, kind}` with `kind` `member` or `agent` (an Agent's with its
  `icon` too), how many of its
  Blockers the person may see are not `done`; it is written with `title`, `description`, `status`,
  `priority`, `assignee_id` (a Member) or `assignee_agent_id` (an Agent,
  422 for one that is terminated or of another Guild; setting one clears
  the other), `project_id`, `goal_id` and `parent_id`, any of
  them null to clear it. `PATCH` also takes `blocked_by_ids`, a list of
  Issue ids (422 for the Issue itself, another Guild's Issue or a cycle),
  and an Issue on its own answers with `blocked_by` and `blocking`, each a
  list of `{id, identifier, title, status}` the person may see. Every Issue
  carries `conversation: {agent: {id, name, icon}, member_id, state,
  boundary_comment_id} | null`. A Comment is `{id, body, deleted, author, created_at,
  updated_at, edited, run_id}`, `run_id` the Run an Agent wrote it in (null
  for a Member's). An Issue document is `{id, key, title, body,
  format: "markdown", latest_revision_id, latest_revision_number,
  created_by, updated_by, created_at, updated_at}`, written with `title`,
  `body`, `change_summary` and `base_revision_id`, the id of the Revision
  the edit started from (left out for the first save), as Paperclip's
  `baseRevisionId`. A Revision is `{id, number, title, body,
  change_summary, created_by, created_at}`; a Restore's change summary is
  "Restored from revision N". An Activity event is `{id, action, actor:
  {id, name} | null, entity: {type: "issue" | "goal" | "approval" | "agent" | "budget" | "budget_incident", id, identifier?,
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
  {type: "agent", id, title, exists}`, the Agent's name; one about a Budget
  or a Budget incident has `entity: {type: "budget" | "budget_incident",
  id, title, exists: true}`, the Budget scope's name, and one about a
  Project's Budget is hidden, as an Issue's is, from whoever may not view
  that Project; one about a Skill has `entity: {type: "skill", id, title,
  exists}`, the Skill's name. Its `details` by
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
  | `issue.checked_out`, `issue.released` | `issue_number`, `issue_title`, `run_id` |
  | `issue.conversation_opened` | `issue_number`, `issue_title`, `agent_id`, `agent_name` |
  | `issue.document_created`, `issue.document_updated` | `issue_number`, `issue_title`, `key`, `title`, `revision_number`, and `restored_from` for a Restore |
  | `issue.document_deleted` | `issue_number`, `issue_title`, `key`, `title` |
  | `approval.created` | `type`, `title`, `issue_ids` |
  | `approval.approved`, `approval.rejected`, `approval.revision_requested` | `type`, `title`, `decision_note` |
  | `approval.resubmitted` | `type`, `title` |
  | `approval.comment_added` | `type`, `title`, `comment_id`, `snippet` |
  | `approval.cancelled` | `type`, `title` |
  | `agent.hired` | `name`, `job`, `approval_id` |
  | `agent.updated` | `name`, `changes`: field → `{from, to}` for `name`, `job`, `title`, `icon`, `reports_to` (`{id, name}`), `heartbeat.enabled`, `heartbeat.interval_sec` and `heartbeat.wake_on_demand`, and `capabilities: true` when they changed |
  | `agent.paused`, `agent.resumed`, `agent.terminated` | `name`; `agent.paused` also `pause_reason` (`manual` or `budget`) |
  | `agent.role_added`, `agent.role_removed` | `name`, `role` (the Role's name when it was added or removed) |
  | `run.started` | `name`, `run_id`, `agent` (`{id, name}`), `issue` (`{id, identifier}`) when the Run has one |
  | `run.finished` | as `run.started`, and `status` (the Run status it ended in) |
  | `budget.updated` | `name` (the scope's), `scope_type`, `scope_id`, `metric`, `window`, `amount`, `warn_percent`, `hard_stop`, `notify` |
  | `budget.soft_threshold_crossed`, `budget.hard_threshold_crossed` | `name`, `budget_id`, `scope_type`, `scope_id`, `metric`, `amount`, `observed`; the hard one also `approval_id` |
  | `budget.incident_resolved` | `name`, `action` (`raise_budget_and_resume` or `keep_paused`), `amount` (the new one, or null), `scope_type`, `scope_id` |
  | `skill.created`, `skill.deleted` | `name`, `slug` |
  | `skill.file_updated`, `skill.file_deleted` | `name`, `slug`, `path` |
  | `agent.skills_synced` | `name`, `added` and `removed` (the Skills' names) |

  References are stored as ids and answered with their names as they are
  when read, null for none. One that no longer exists, or an Issue or
  Project the person may not view, keeps its id with a null name (or
  title and identifier), so the event never names what is hidden.
  Routes open to Agent principals (a Run key, through
  `guilds.AuthAgents`), with the Agent membership's Permissions: reading
  Goals, Issues, their Comments, Issue documents and Revisions, the
  Activity, Approvals with their Issues and comments (`view_resources`);
  creating and changing Issues, writing Comments and editing or deleting
  its own, saving Issue documents, requesting an Approval and commenting
  on one (`manage_work`); and, for Run keys only,
  `POST /api/issues/{id}/checkout` `{expected_statuses}` and
  `POST /api/issues/{id}/release` (a person is 403 `only an agent's run
  can check out an issue`). Checkout answers the Issue (200, also when
  this Run holds it already); 409 is `{message: "checked out by another
  run", run_id}` for another live Run, `{message: "issue status is …",
  status}` for an unexpected status, or `issue is assigned to someone
  else`; Release by any other Run is 409 `issue is not checked out by this
  run`. An Agent's `PATCH`, Comment or Issue document save on an Issue
  another live Run holds is the same 409 with `run_id`. Every Issue
  carries `checkout: {run_id, agent: {id, name, icon}, checked_out_at}`,
  null when there is none or it is Stale, `application: {id, name}` (its
  Issue's Application, or null) and, on `GET /api/issues/{issue}`,
  `work_products`, newest first. `POST /api/issues/{id}/pull-requests` is
  open to Run keys too. What an Agent writes carries `author_agent`
  (Comments, Approval comments), `created_by_agent` (Issues, Issue
  documents and Revisions), `updated_by_agent` (Issue documents) or
  `requester_agent` (Approvals) `{id, name, icon}` beside the Member field,
  which is then null; its Activity events carry `actor_agent` beside a null
  `actor`, and `GET /api/activity?actor=agent:<id>` filters by it. Goal
  changes, deleting Issues and Issue documents, restoring Revisions, Read
  marks, the Inbox and sidebar badges, Approval Decisions and resubmitting
  stay a person's: 403 `agents cannot use this route`.

- **Consumes:**
  - from agents: the hook it registers (`work.OnRunLive`) to tell which
    Runs are live (`running`), for Checkout and Stale checkouts, and
    `work.OnIssuesWithLiveRuns(f)`: given a Guild and some Issues, which
    of them have a `queued` or `running` Run, for a Routine's Live
    execution Issue. Until agents registers it, no Issue has one.
  - Routines publish nothing new to agents: an Execution Issue wakes its
    Agent through `work.OnIssueAssigned` like any assignment (Wake reason
    `issue_assigned`, Invocation source `assignment`), with the Member who
    pressed Run, or nobody for a `schedule` or `api` firing.
  - from guilds: `guilds.Auth`, `guilds.AuthAgents`, `guilds.Can(permission)`,
    `guilds.AgentID(ctx)` and `guilds.RunID(ctx)` (the Agent principal's
    Agent and Run),
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
    `projects.ApplicationInProject(ctx, guild, project, application)` and
    `projects.ApplicationNames(ctx, guild, ids)` check and name the
    Issue's Application, and `projects.OnApplicationDeleted` clears it.
  - from deployments: `deployments.OpenPullRequest(ctx, application,
    head, title, body)` for `OpenPullRequest`, `deployments.OnPullRequest`
    for every Pull request event (opened, pushed, closed, merged) and the
    Preview hooks (`DeploymentFinished` with a Preview number, Preview
    deploying and Preview removed) with `deployments.PreviewURL` for its
    link, which move the Work products. Each is recorded with no Actor, since
    nobody in the Guild did it.
- **Publishes to other contexts** (Go functions, no routes):
  `work.RequestApproval(ctx, guild, hirer, HireAgentRequest)` (a
  `hire_agent` Approval; answers its id),
  `work.RequestBudgetOverride(ctx, guild, BudgetOverrideRequest)` (a
  `budget_override_required` Approval with no Requester; answers its id),
  `work.CancelApproval(ctx, guild, actor, id)`, `work.OnApprovalDecided(type, f)` (called after an approve or
  reject of that type is stored, with the guild, the Approval's id and type,
  whether it was approved, the decider and the hire's Agent; the same
  Decision made again calls it again, so a failed `f` is healed by deciding
  again, and its error answers the Decision 500; the agents context
  registers for `hire_agent`), `work.RecordActivity(ctx, AgentActivity)`
  (one Activity event about an Agent, a Budget, a Budget incident or a
  Skill, with its name kept in the details; only the `agent.*`, `run.*`,
  `budget.*` and `skill.*` Actions), `work.DecideBudgetOverride(ctx, guild, actor, id, approved,
  note)` (no `OnApprovalDecided` hook follows it) and `work.OnAgentNames(f)` (the agents
  context names the Guild's Agents that still exist, so the Activity can
  tell `exists`; until it registers, every Agent counts as existing),
  `work.OnAgentAssignees(f)` (the agents context names the Guild's Agents
  among some ids, with their Agent icon and whether they are terminated,
  terminated ones included so a done Issue still shows its Agent; an
  Agent may be an Assignee when it is the Guild's and not terminated, and
  until it registers, no Agent may), `work.IssueForRun(ctx, guild, issue)` (an
  Issue's identifier, title, description, Issue status, Project, Agent
  assignee and, for a Conversation, its owner and Session boundary, for a
  Run's check and its prompt; it does not check who may
  view it, so agents asks guilds for that) and `work.UnassignAgent(ctx, guild,
  agent, actor)` (a terminated Agent stops being the Assignee of the open
  Issues, each recorded as `issue.updated` with the terminating person as
  Actor), `work.OnIssueAssigned(f)` (called after an Issue is created with,
  or changed to, an Agent assignee while its Issue status is not
  `backlog`, `done` or `cancelled`, and after an Issue with an Agent
  assignee moves out of `backlog` into an open status; with the Issue's
  brief and the Member who did it), `work.OnIssueCommented(f)` (called
  after a Comment is written on an Issue whose Agent assignee is set and
  whose Issue status is not `done` or `cancelled`; with the Issue's brief,
  the Comment and its author, and whether the Issue is a Conversation and
  the Comment a New session, so agents wakes the Agent with
  `conversation_message` for its owner's message and not at all for
  `/new`), `work.ConversationHistory(ctx, guild, issue, limit)` (the
  newest `limit` Comments of a Conversation after its Session boundary,
  deleted ones left out, oldest first, with their author's name, for a
  Conversation Run's prompt), `work.ReplyInConversation(ctx, guild,
  issue, agent, run, body)` (the Completion reply: `body`, cut to the
  longest Comment, written through `WriteComment` as the Conversation
  agent in that Run, and nothing when the Agent already has a Comment
  that is not deleted with that `run_id`, or the Issue is no
  Conversation of that Agent) and `work.OpenIssuesOfAgent(ctx, guild,
  agent)` (the Issues assigned to an Agent with Issue status `todo`,
  `in_progress` or `in_review`, oldest first, with identifier, Issue
  status, Priority and title, for the Heartbeat timer's check and its
  prompt), `work.InboxOfAgent(ctx, guild, agent)` (the Issues assigned
  to an Agent in `in_progress`, `in_review`, `todo` or `blocked`, in that
  order, then most urgent and oldest first, with identifier, Issue
  status, Priority, Project and its name, Goal, parent and the Run
  holding its Checkout, for the Agent's own inbox; agents leaves out
  Projects the Agent may not view) and `work.CommentsForRun(ctx, guild, ids)` (the Guild's Comments
  with those ids, in order, with their author's name, leaving out deleted
  ones, for the prompt of a Run that comments woke). A hook's error is logged and never fails the person's request:
  the Issue or Comment is already stored, and the Agent wakes on the next
  change or its timer. Work never imports the contexts that call them.

## Why it's shaped this way

- **A Conversation is an Issue**, as in Paperclip. A Comment already wakes
  the Agent assignee, a Run already attaches to an Issue and streams to its
  page, and the Agent already writes Comments through the API; a table of
  its own messages would duplicate all of that. It stays out of the Issues
  list, the Inbox and the Agent's open Issues so it never reads as work.
- **No experimental switch, no work modes, no confirmation cards, no
  attachments.** Paperclip hides Agent chat behind `enableAgentChat`; The
  Bakery has no experimental settings and ships chat as a feature. Its
  Ask/Plan work modes and `request_confirmation` cards sit on issue
  interactions The Bakery does not have, Comments have no attachments,
  and chat channels (Slack, Telegram, email) are left out.
- **Board chat is a later slice.** Paperclip's Conference Room runs
  `claude` on the server, which The Bakery never does; it needs a Run on
  the asking person's own Desktop without an Agent.
- **Any Member with `manage_work` may chat** with any Agent of the Guild,
  as they may already comment on its Issues and so wake it. The Hirer
  still controls spending through Pause, `wake_on_demand` and Budgets.
- **Agents by id.** The chat routes name the Agent by its id, not
  Paperclip's shortname `agentRef`.

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
- **Wakes are hooks, not events on a bus.** Paperclip's issue routes call
  the heartbeat service directly. Work calls the hooks agents registered
  after its change is stored, so work never imports agents and a failed
  wake never undoes an assignment or a Comment.
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
- **Checkout only for a Run key.** Paperclip's UI never checks an Issue
  out for a person either; people assign, Agents check out. The Agent and
  the Run come from the key, not the body (Paperclip's `agentId` in the
  body is left out), so an Agent cannot check out for another.
- **Paperclip's checkout rules kept, its execution locks left out.**
  Expected statuses with Paperclip's defaults, 409 against another live
  Run, and the same Agent's next Run taking over a Stale checkout are
  Paperclip's `checkoutRunId` rules. The Checkout keeps only the Run: its
  Agent is always the Issue's Agent assignee, since a new Assignee ends it,
  and work asks agents whether the Run is live instead of reading `runs`.
  Its separate `executionRunId` lock,
  pause holds and run-scoped issue workspaces are left out: one Run per
  Agent at a time already keeps an Agent from racing itself, and git
  workspaces are the next slice of the goal.
- **Release unassigns, as in Paperclip.** A released open Issue goes back
  to the pool without an Assignee, so a person or another Agent picks it
  up, instead of waking the same Agent on its next Heartbeat.
- **An Agent does not wake itself.** A Comment by the Issue's own Agent
  assignee would queue a Run for the Agent that just wrote it, a loop
  Paperclip also cuts.
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
- **`cancelled` only for a hire or a gone Budget; no waking the Requester.** Paperclip cancels a `hire_agent` Approval when its Agent is
  terminated first, and so does The Bakery. A `budget_override_required`
  one is cancelled when its Budget goes with its Agent or Project, so no
  Approval waits on a Budget that no longer exists. It also cancels Approvals when
  their requesting agent goes away and wakes that agent after a Decision;
  here only people request Approvals, so neither applies yet.
- **A Budget override is decided on the Costs page only.** Its Decision
  changes a Budget and resumes a scope, which agents owns; deciding it
  through the generic Approve would approve without raising anything, so
  Approve, Reject, Request revision and Resubmit refuse it, as Paperclip's
  routes do, and agents decides it through `work.DecideBudgetOverride`
  when the incident is resolved. It has no Requester: The Bakery itself
  asked.
- **No Request revision for a hire.** A `pending_approval` Agent cannot be
  edited, so its Approval's payload is always what the Board sees, and
  there is nothing a revision could change. The Board approves or rejects;
  a Hirer who wants a different Agent terminates this one and hires again.
- **Work products are only `pull_request` and `preview_url`.** Paperclip's
  `issue_work_products` also has branches, commits, runtime services,
  artifacts and documents, a review state, a primary flag and a health
  status. A branch is always the Issue's Agent branch and a commit lives
  in its Pull request, Issue documents are already their own aggregate,
  and Artifacts are Order step 7; review happens on the git host. Two
  types with their own statuses say what a Board needs to see on an
  Issue: is there a Pull request, and can I look at it running.
- **The Issue's Application, not Paperclip's project workspaces.**
  Paperclip lets a project hold several workspaces (local folders or
  repositories) and an Issue pick one. In The Bakery a Project already
  holds Applications with a git source, so an Issue names one of them and
  that Application's repository and branch are where its Agent works.
  It is one column on the Issue, kept in work; projects is asked only
  whether the Application is in the Project and what it is called.
- **Work products follow the git host, not the Agent.** Once the Pull
  request is open, its status and Preview move only through deployments'
  Webhook and Preview hooks, so a Pull request merged or closed by a person
  on the git host shows on the Issue the same as one an Agent closed.
  Those changes have no Actor in the Activity.
- **Routines are work's.** A Routine is a template for Issues: it holds
  an Issue's fields and makes Issues, and its Routine runs follow those
  Issues' statuses. Agents, Wakes and Runs stay the agents context's, so a
  Routine reaches an Agent only by assigning it an Issue, through the same
  `OnIssueAssigned` every assignment uses.
- **Terminating an Agent makes its Routines Drafts.** Paperclip keeps the
  terminated Agent on the Routine and lets each dispatch refuse it. The
  Bakery clears it, as it clears an open Issue's Agent assignee, so the
  Routine page says "draft" instead of failing every scheduled firing.
- **Routines left out for now.** Paperclip's `app_webhook` and
  `fireflies_hmac` Signing modes (they serve Paperclip's own apps and one
  vendor), setup-pending Webhook triggers and their test receipts,
  folders, the activity gate, the secrets, environment and delivery
  sections, built-in and plugin-managed Routines, and Paperclip's split of
  `received` and its dispatch across processes. With Routine revisions and
  Restore, the Routine part of the guilds goal's Paperclip step is
  complete; the rest is not in it.
- **No `routine.revision_created` Action.** Each change that adds a
  Routine revision already records `routine.created`, `routine.updated`,
  `routine.archived` or a `routine.trigger_*` Action, so a second event
  would say the same twice, as with `routine.webhook_received`. Only a
  Restore gets its own, `routine.revision_restored`.
- **A smaller Snapshot.** Paperclip's Snapshot also holds `env`, the
  activity gate, the folder, the responsible user and `setupPending`. The
  Bakery's Routines have none of them, so its Snapshot has none either.
- **Restore refuses an archived Routine.** Paperclip would restore the
  Snapshot's status too. Nothing leaves `archived` in The Bakery, so an
  archived Routine is not restored (409), as it is not changed.
- **An Agent's termination adds a Routine revision.** Clearing the Agent
  assignee changes what the Snapshot holds, so it is a revision ("Agent
  terminated") like every other change. The newest revision always matches
  the Routine, and a Restore of an older one is refused only for an Agent
  that can no longer be assigned, never for a change it never saw.
- **Restore keeps a Project it cannot put back.** Paperclip writes the
  Snapshot's Project back as it is. In The Bakery, deleting a Project
  deletes its Routines, so a Snapshot's Project that is gone (or that the
  person may no longer view) is one the Routine has left since; the
  Routine stays in its current Project rather than being refused. A Goal
  or parent Issue gone since is cleared, as the database clears it.
- **Restore takes no Change summary.** Paperclip's dialog has a field for
  one, but its route ignores it and writes "Restored from revision N". The
  Bakery's dialog has no such field.
- **The Webhook secret lives encrypted on its trigger.** Paperclip keeps it
  in its secrets service and points at it. The Bakery has no secrets
  context, so it is encrypted with `facades.Crypt()` in work's own table,
  as the deployments context keeps an Application's Webhook secret. Work
  never imports deployments' Webhook types: the two only share a word.
- **`X-Bakery-Signature` and `X-Bakery-Timestamp`** in place of
  Paperclip's `X-Paperclip-*` headers, so no other product's name reaches
  a user. `X-Hub-Signature-256` stays, since git hosts send it.
- **`webhook_path` is a path.** Paperclip answers a full URL from its
  configured public address. The dashboard adds its own origin, as it does
  for an Application's Webhook, so the API needs no idea of its own
  address.
- **No `routine.webhook_received` Action.** An accepted Webhook delivery
  already shows as the Routine run's `routine.run_triggered` with source
  `webhook`; only a rejected one gets its own Action.
- **No paused Projects.** Paperclip skips a schedule while its Project is
  paused. The Bakery's Projects have no pause; a Project Budget's Hard stop
  already refuses the Wake, so the Execution Issue waits for the Budget.
