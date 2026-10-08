# agents

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/agents`)

## Purpose

Holds a Guild's Agents: the AI workers its people hire, who hired each one
(its Hirer), what it does (its Job and Title), whom it reports to (its
Manager, and so the Guild's Org chart), its Agent status and its Runs: each
execution of the Agent by `claude` on its Hirer's Desktop app, with the
Transcript the Desktop reports. An Agent holds
Roles through its Agent membership in guilds, so it can only do what those
Roles allow, and it is never placed above its Hirer. Hiring always goes
through a `hire_agent` Approval the Board decides in work.

It is **not** responsible (yet) for Heartbeats, git, budgets, instructions,
skills or keys: later phases of the guilds goal add them. It never executes
anything: the Runner in the Desktop app does, and reports back. Who an Issue
is assigned to belongs to work; agents only answers whether an Agent may be
one. It does not own Members,
Roles, Approvals or the Activity; it stores ids and asks guilds, work and
identity about them.

## Language

The terms (Agent, Hirer, Job, Title, Agent icon, Capabilities, Agent status,
Manager, Org chart, Hire, Pause, Resume, Terminate, Agent membership, Run,
Run status, Run event, Transcript, Invocation source, Run usage, Runner,
Lease) are in
[`glossary.md`](../../glossary.md).

| Term | Meaning |
| ---- | ------- |
| Reports to | The other side of Manager: an Agent's direct reports are the Agents whose Manager it is. |
| Chain of command | An Agent's Manager, that one's Manager, and so on up to the top of the Org chart. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Agent | Belongs to one Guild, and has one Hirer, a person with a Membership in that Guild when it is hired. Name 1–100 characters after trimming, unique (case-insensitive) among the Guild's Agents that are not terminated, as Paperclip's shortname uniqueness. Job from the glossary's list (`general` when none is given). Title at most 200 characters, Capabilities at most 20000, both optional. Agent icon from the glossary's list, or none. The Manager is an Agent of the same Guild that is not terminated, never the Agent itself and never one of its reports: setting it walks the new Manager's Chain of command up at most 50 levels, as Paperclip's `getChainOfCommand`, and refuses a cycle (422). Agent status moves only `pending_approval → idle` (its Approval approved), `pending_approval → terminated` (rejected, or terminated by a person), `idle`, `running` or `error` → `paused`, `paused → idle`, `idle` or `error` → `running` (a Run of it is claimed), `running → idle` (its Run `succeeded` or was `cancelled`), `running → error` (its Run `failed` or was `lost`), and any of `idle`, `running`, `error` or `paused` → `terminated`; nothing leaves `terminated`. A `pending_approval` Agent cannot be edited, paused or given Roles, so its Approval's payload is what the Board sees; it can only be terminated, which cancels its Approval. A terminated Agent cannot be edited. Only the Run moves set `running` and `error`; a person never does. When an Agent is terminated, its direct reports report to its Manager, or become roots when it had none. |
| Run | Belongs to one Guild and one Agent of it, and optionally one Issue of that Guild. Starts `queued`. Run status moves only `queued → running` (claimed by a Desktop of the Agent's Hirer), `queued → cancelled`, and `running → succeeded`, `failed`, `cancelled` or `lost`; a final status never changes. At most one `running` Run per Agent; further ones wait `queued` and are claimed in order of creation. Its Run events have strictly increasing `seq` per Run, and appending a `seq` it already has is a no-op. Run events are appended only while it is `running`. A `running` Run whose Lease runs out becomes `lost`, and a new `queued` Run with the same Agent, Issue and Invocation source takes its place, its `retry_of_run_id` pointing at the lost one (Paperclip's `retryOfRunId`). Run usage is set once, when it finishes. |

The Agent's Roles are not part of the Agent aggregate: they are its Agent
membership's in guilds, which keeps the rule that each is below the
Hirer's highest Role.

### Commands

Who may run each is in brackets, by Permission in the Agent's Guild.
"Manage" means holding `hire_agents` and being the Agent's Hirer or ranking
above the Hirer in the Guild's hierarchy; the Guild Master and the Instance
admin always may (with `hire_agents`, which they always hold).

- Read the Guild's Agents, one Agent and the Org chart [`view_resources`].
- `Hire(name, job, title, icon, manager, capabilities, roles)`
  [`hire_agents`]: the asking Member is the Hirer; the Instance admin
  acting in a Guild they have no Membership in cannot hire (403). Each
  requested Role must be below the Hirer's highest Role and grant only
  Permissions the Hirer holds (administrator only when they hold it);
  otherwise 422. Creates the Agent `pending_approval` with
  its Agent membership and Roles, and asks work for a `hire_agent`
  Approval with the Hirer as Requester, in that order; if asking fails the
  Agent and its Agent membership are removed again.
- `Approved`, `Rejected` (no Permission of its own; it follows the Board's
  Decision on the `hire_agent` Approval, through `work.OnApprovalDecided`):
  the Agent becomes `idle` or `terminated`.
- `Edit(name, job, title, icon, manager, capabilities)` [manage]: only an
  `idle` or `paused` Agent.
- `Pause` [manage]: only an `idle`, `running` or `error` Agent; cancels
  its `queued` and `running` Runs. `Resume` [manage]: only a `paused` Agent
  (resuming a terminated one is 422).
- `Terminate` [manage]: from any status but `terminated`; ends its Agent
  membership, moves its direct reports up to its Manager, cancels its
  Approval when it was `pending_approval` and its `queued` and `running`
  Runs, and asks work to clear it as the Agent assignee of its open
  Issues.
- `AddRole(role)`, `RemoveRole(role)` [manage]: only an `idle` or `paused`
  Agent; a Role added must be below the Hirer's highest Role and the
  asking person's, and grant only Permissions the asking person holds, as
  at the hire, since managing an Agent does not need `manage_roles`; a
  Role removed must be below the asking person's highest (422 otherwise).
- `StartRun(agent, issue)` [manage]: only an `idle`, `running` or
  `error` Agent (a `paused`, `pending_approval` or `terminated` one is
  422), only on an Issue of its Guild whose Agent assignee is that Agent
  (422 otherwise; work answers it through a published call). Creates a
  `queued` Run with Invocation source `on_demand`; it waits behind the
  Agent's running Run, if any.
- `CancelRun(run)` [manage the Run's Agent]: only a `queued` or `running`
  Run; it becomes `cancelled`. The Desktop running it sees that on its next
  report (409 there) and on its Run stream, and stops `claude`.
- `ClaimRun(run)` [a Desktop key of the Agent's Hirer only]: the oldest
  `queued` Run of an Agent that has no `running` Run becomes `running`,
  atomically, with its Lease starting; a second claim is 409, a Desktop of
  another person 404 (it must not learn the Run exists).
- `AppendRunEvents(run, events)` [the claiming Desktop's person]: appends
  Run events by `seq`, renews the Lease; on a Run that is no longer
  `running`, 409 with its status.
- `KeepLease(run)` [the claiming Desktop's person]: renews the Lease when
  `claude` is quiet.
- `FinishRun(run, status, usage, error)` [the claiming Desktop's person]:
  `succeeded`, `failed` or `cancelled`, with its Run usage and an error
  message when it failed.
- `LoseRun` (no Permission; the server's own sweep): a `running` Run whose
  Lease ran out becomes `lost` and is requeued as above.
- Read Runs (filter by Agent, Issue, Run status), one Run, its Run events
  after a `seq`, and its Transcript live [`view_resources`].
- When the Hirer leaves the Guild or is removed from it, guilds tells
  agents and every Agent they hired there is terminated, with whoever
  removed them as the Actor (removal is the only way to leave today).
  There is no account deletion yet. Should one come, it would not pass
  through guilds: the foreign keys on `agents.hirer_member_id` and
  `memberships.hirer_member_id` cascade, so the Hirer's Agents and Agent
  memberships would go with the account, and the Activity would keep
  naming those Agents (`exists: false`).

"Manage" is `hire_agents` plus being the Agent's Hirer, the Instance
admin, or ranking above the Hirer (the Guild Master ranks above everyone);
403 otherwise. The Agent JSON's `can_manage` answers it for the asker.

A status move the Agent's status does not allow (pausing a `paused` Agent,
resuming an `idle` one, terminating a `terminated` one) is 422 naming the
status. Adding a Role the Agent holds, removing one it does not, or an edit
that changes nothing answers the Agent unchanged and records nothing.

### Domain events

Each command publishes its event after its change is stored, and records
it in work's Activity through `work.RecordActivity`, with the person who
did it as Actor.

| Event | Published by | Action |
| ----- | ------------ | ------ |
| `AgentHired` | `Hire` | `agent.hired` |
| `AgentUpdated` | `Edit`, when something changed | `agent.updated` |
| `AgentPaused` | `Pause` | `agent.paused` |
| `AgentResumed` | `Resume` | `agent.resumed` |
| `AgentTerminated` | `Terminate`, `Rejected`, the Hirer leaving | `agent.terminated` |
| `AgentRoleAdded` | `AddRole` | `agent.role_added` |
| `AgentRoleRemoved` | `RemoveRole` | `agent.role_removed` |
| `RunStarted` | `StartRun` | `run.started` |
| `RunClaimed` | `ClaimRun` | none |
| `RunFinished` (with its Run status) | `FinishRun`, `CancelRun`, `LoseRun`, Pause, Terminate | `run.finished` |

The Run events themselves are never Activity: one Run writes hundreds of
them, and the Activity would drown. `RunClaimed` only moves the Agent to
`running`. `run.finished` has as Actor the person who cancelled, paused or
terminated, or else the Agent's Hirer (whose Desktop reported it, or whose
Desktop went quiet), since the Activity has no system Actor yet.

The `details` of each Action are in the work document. Approving the hire
is recorded by work as `approval.approved`; the Agent becoming `idle`
records nothing more.

## Integration

- **Publishes:** the agents API, all in the Current guild (an id from
  another Guild is 404), 403 without the Permission a route needs, 422 for
  a broken invariant.

  | Route | Permission | Body | Answers |
  | ----- | ---------- | ---- | ------- |
  | `GET /api/agents` | `view_resources` | | `{"agents": [Agent]}`, by name; filter `status`: `all` (every Agent but the terminated ones, the default, as Paperclip's All tab), `active` (idle), `paused`, `pending` (pending approval) or `terminated` |
  | `POST /api/agents` | `hire_agents` | `{name, job, title, icon, reports_to, capabilities, role_ids}` | 201 `{"agent": Agent, "approval_id": id}` |
  | `GET /api/agents/{id}` | `view_resources` | | `{"agent": Agent}` |
  | `PATCH /api/agents/{id}` | manage | any of `{name, job, title, icon, reports_to, capabilities}` | `{"agent": Agent}` |
  | `POST /api/agents/{id}/pause`, `.../resume`, `.../terminate` | manage | | `{"agent": Agent}` |
  | `PUT /api/agents/{id}/roles/{role_id}` | manage | | `{"agent": Agent}` |
  | `DELETE /api/agents/{id}/roles/{role_id}` | manage | | `{"agent": Agent}` |
  | `GET /api/org` | `view_resources` | | `{"org": [Org node]}`, the roots of the Org chart |
  | `POST /api/agents/{id}/runs` | manage | `{issue_id}` | 201 `{"run": Run}` |
  | `GET /api/runs` | `view_resources` | | `{"runs": [Run]}`, newest first; filters `agent`, `issue`, `status` |
  | `GET /api/runs/{id}` | `view_resources` | | `{"run": Run}` |
  | `POST /api/runs/{id}/cancel` | manage | | `{"run": Run}` |
  | `GET /api/runs/{id}/events?after=` | `view_resources` | | `{"events": [Run event]}`, by `seq`, after the given one |
  | `GET /api/runs/{id}/stream` | `view_resources` | | server-sent events: the Run events after `Last-Event-ID` (or `after`), then each new one and each Run status change, until the Run is final |

  For a Desktop key, across every Guild of its person (no Current guild
  needed; a Run of someone else's Agent is 404):

  | Route | Body | Answers |
  | ----- | ---- | ------- |
  | `GET /api/desktop/runs/stream` | | server-sent events: each `queued` Run of the person's Agents, now and as they come, and each cancel of a Run the Desktop holds |
  | `POST /api/runs/{id}/claim` | | `{"run": Run, "prompt": text}`; 409 when already claimed or no longer `queued` |
  | `POST /api/runs/{id}/events` | `{"events": [{seq, type, stream, message, payload}]}` | `{"run": Run}`; 409 when not `running` |
  | `POST /api/runs/{id}/lease` | | `{"run": Run}`; 409 when not `running` |
  | `POST /api/runs/{id}/finish` | `{status, usage, error}` | `{"run": Run}`; 409 when not `running` |

  A Run is `{id, agent: {id, name}, issue: {id, number, title} | null,
  status, invocation_source, retry_of_run_id, error, usage: {input_tokens,
  output_tokens, cached_input_tokens, turns, duration_ms, cost_usd} | null,
  hirer: {id, name}, created_at, started_at, finished_at}`. A Run event is
  `{seq, type, stream, message, payload, created_at}`. Tasks that build
  these routes keep this table in step with what they ship.

  An Agent is `{id, name, job, job_label, title, icon, capabilities, status,
  reports_to: {id, name} | null, hirer: {id, name} | null, roles: [{id,
  name, color, position}], approval_id, can_manage, created_at,
  updated_at, paused_at, terminated_at}`. An Org node is `{id, name, job,
  job_label, title, icon, status, reports: [Org node]}`; an Agent whose
  Manager is terminated is a root, each level ordered by name. A hire
  whose Roles guilds refuses is 422 on `role_ids`; a name taken is 422 on
  `name`.
- **Consumes:**
  - from guilds: `guilds.Auth`, `guilds.Can(permission)`,
    `guilds.Owns` (an `{id}` outside the Current guild is 404),
    `guilds.Current(ctx)`, `guilds.MemberID(ctx)` (the Hirer),
    `guilds.Permissions(ctx)` and `guilds.InstanceAdmin(ctx)` (who is
    asking, for manage), the Agent membership calls (`JoinAgent`,
    `AssignAgentRole`, `RemoveAgentRole`, `LeaveAgent`, `AgentRoles`, with
    `AgentRoleRefused` telling a refused Role from a failure),
    `guilds.RoleNames` for the hire's payload, `guilds.RankAbove` (who
    ranks above a Hirer), and the hook for a Member leaving or being
    removed. It registers `guilds.OnGuildDeleting`, so a
    Guild with Agents that are not terminated is not deleted.
  - from work: `work.RequestApproval` and `work.CancelApproval` for the
    `hire_agent` Approval, `work.OnApprovalDecided` (registered for
    `hire_agent`), `work.RecordActivity` for every event above, and
    `work.OnAgentNames` (registered, so the Activity knows which Agents
    still exist), the Issue call that answers an Issue's Agent assignee,
    number, title and description (for a Run's check and its prompt), and
    the call that clears a terminated Agent as Assignee. It registers the
    hook by which work asks whether an Agent may be an Assignee (in the
    Guild, not terminated).
  - from identity: the Desktop key principal (its person, across Guilds)
    for the Desktop routes.
  - from identity: `identity.Members(ctx, ids)` for Hirers' names.

## Why it's shaped this way

- **Agents are their own context.** They will grow Runs, Heartbeats, the
  desktop protocol, budgets and skills; neither work (the Board's plans)
  nor guilds (who may do what) should carry that. Agents talks to both
  through their published functions only.
- **Job, not "role".** Paperclip calls an agent's place in the org chart
  its `role`. In The Bakery a Role is a guilds Role with Permissions, so
  the org chart word is Job; its values and labels are Paperclip's.
- **Hiring always needs an Approval.** The goal says hiring needs one, so
  Paperclip's `requireBoardApprovalForNewAgents` switch is left out. A
  Hirer who holds `approve` may approve their own hire, as with every
  Approval; a Guild that wants four eyes gives its Hirers no `approve`.
- **The Agent exists from the moment it is hired**, as in Paperclip, so the
  Approval names a real Agent, the Agent shows on the Agents page as
  pending approval, and the Decision only moves its status.
- **One permission system.** An Agent holds Roles through its own Agent
  membership in guilds, so `Can`, Project overrides and the hierarchy apply
  to it unchanged. Paperclip's `agents.permissions` jsonb and
  `principal_permission_grants` are left out.
- **Never above the Hirer, kept in two places.** A Role at or above the
  Hirer's highest Role is refused, and when the Hirer drops, guilds removes
  the Agent's Roles at or above the Hirer's new highest in the same change.
- **The Hirer owns the Agent, not the Board.** The Agent runs on the
  Hirer's desktop with the Hirer's subscription, so the Hirer (and whoever
  ranks above them) manages it. When the Hirer leaves the Guild, their
  Agents are terminated: nobody else's desktop could run them.
- **Agents block deleting their Guild** until they are terminated, like
  Goals and Issues, so a Guild is never deleted under running work.
- **Terminating moves the reports up.** Paperclip leaves a terminated
  agent's reports pointing at it and shows them as roots; moving them to
  the terminated Agent's Manager keeps the tree whole.
- **No adapter, model, environment, instructions, skills, API keys, budget
  or appearance yet.** `claude` on the Hirer's desktop is the only runtime
  and comes with the desktop app; avatars are the Agent icon only.
- **No hard delete.** Paperclip's `DELETE /agents/:id` is left out: a
  terminated Agent stays as a record, so the Activity and later Runs keep
  naming it.
- **Runs live in agents.** A Run is an Agent's execution, as Paperclip keeps
  heartbeat runs with the agent's heartbeat service. A context of its own
  would share every invariant (the Agent's status, its Hirer, Pause and
  Terminate) through calls back into agents.
- **The server never executes anything**, so Paperclip's adapters,
  process ids, log stores, watchdogs, runtime modes, session resumes and
  liveness classifier are left out. The Desktop reports every Run event,
  the usage and the outcome; the server stores and relays them. A Run
  event is a trimmed `heartbeat_run_events` row: `seq`, `type`, `stream`,
  `message` and `payload`, with no source instance, color or level.
- **Only the Hirer's Desktops may claim a Run**, because the Run spends the
  Hirer's Claude subscription. Another person's Desktop gets 404 so it
  learns nothing about Agents it cannot run.
- **A lost Run is requeued, not failed.** The goal says work waits while a
  desktop is offline. The lost Run stays as a record of what happened and
  the new one points back at it, as Paperclip's `retryOfRunId`; the Agent
  shows `error` until the new Run is claimed.
- **Only `claude`.** Paperclip's Codex, Gemini, OpenCode, Cursor, HTTP and
  process adapters are left out, as the goal fixes.
- **Assigning an Issue to an Agent does not start a Run yet.** Paperclip
  wakes the assignee with a Heartbeat; Heartbeats are Order step 6. Until
  then a person presses Run, the `on_demand` Invocation source.
- **One running Run per Agent.** Paperclip allows a configurable
  concurrency; one keeps the Agent status meaningful and a laptop's
  subscription from being spent twice at once.
- **No built-in agents.** A Guild starts with none; people hire them.
- **The Org chart is a view on the Agents page**, as in Paperclip's
  streamlined UI, whose `/org` route redirects there. Its SVG and PNG
  export are left out.
- **No Request revision for a hire.** A `pending_approval` Agent cannot be
  changed, so there is nothing to revise; see the work document.
