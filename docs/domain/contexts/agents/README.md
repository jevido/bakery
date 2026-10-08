# agents

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/agents`)

## Purpose

Holds a Guild's Agents: the AI workers its people hire, who hired each one
(its Hirer), what it does (its Job and Title), whom it reports to (its
Manager, and so the Guild's Org chart) and its Agent status. An Agent holds
Roles through its Agent membership in guilds, so it can only do what those
Roles allow, and it is never placed above its Hirer. Hiring always goes
through a `hire_agent` Approval the Board decides in work.

It is **not** responsible (yet) for Runs, Heartbeats, the desktop app that
runs an Agent, assigning Issues to Agents, budgets, instructions, skills or
keys: later phases of the guilds goal add them. It does not own Members,
Roles, Approvals or the Activity; it stores ids and asks guilds, work and
identity about them.

## Language

The terms (Agent, Hirer, Job, Title, Agent icon, Capabilities, Agent status,
Manager, Org chart, Hire, Pause, Resume, Terminate, Agent membership) are in
[`glossary.md`](../../glossary.md).

| Term | Meaning |
| ---- | ------- |
| Reports to | The other side of Manager: an Agent's direct reports are the Agents whose Manager it is. |
| Chain of command | An Agent's Manager, that one's Manager, and so on up to the top of the Org chart. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Agent | Belongs to one Guild, and has one Hirer, a person with a Membership in that Guild when it is hired. Name 1–100 characters after trimming, unique (case-insensitive) among the Guild's Agents that are not terminated, as Paperclip's shortname uniqueness. Job from the glossary's list (`general` when none is given). Title at most 200 characters, Capabilities at most 20000, both optional. Agent icon from the glossary's list, or none. The Manager is an Agent of the same Guild that is not terminated, never the Agent itself and never one of its reports: setting it walks the new Manager's Chain of command up at most 50 levels, as Paperclip's `getChainOfCommand`, and refuses a cycle (422). Agent status moves only `pending_approval → idle` (its Approval approved), `pending_approval → terminated` (rejected, or terminated by a person), `idle → paused`, `paused → idle` and `idle` or `paused` → `terminated`; nothing leaves `terminated`. A `pending_approval` Agent cannot be edited, paused or given Roles, so its Approval's payload is what the Board sees; it can only be terminated, which cancels its Approval. A terminated Agent cannot be edited. When an Agent is terminated, its direct reports report to its Manager, or become roots when it had none. |

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
  [`hire_agents`]: the asking Member is the Hirer. Each requested Role must
  be below the Hirer's highest Role and one they may assign (guilds'
  `CanAssign`); otherwise 422. Creates the Agent `pending_approval` with
  its Agent membership and Roles, and asks work for a `hire_agent`
  Approval with the Hirer as Requester, in that order; if asking fails the
  Agent and its Agent membership are removed again.
- `Approved`, `Rejected` (no Permission of its own; it follows the Board's
  Decision on the `hire_agent` Approval, through `work.OnApprovalDecided`):
  the Agent becomes `idle` or `terminated`.
- `Edit(name, job, title, icon, manager, capabilities)` [manage]: only an
  `idle` or `paused` Agent.
- `Pause` [manage]: only an `idle` Agent. `Resume` [manage]: only a
  `paused` Agent (resuming a terminated one is 422).
- `Terminate` [manage]: from any status but `terminated`; ends its Agent
  membership, moves its direct reports up to its Manager, and cancels its
  Approval when it was `pending_approval`.
- `AddRole(role)`, `RemoveRole(role)` [manage]: only an `idle` or `paused`
  Agent; a Role added must be below the Hirer's highest Role and one the
  asking person may assign (422 otherwise).
- When the Hirer leaves the Guild or is removed from it, guilds tells
  agents and every Agent they hired there is terminated.

Making the same change again (pausing a `paused` Agent, adding a Role it
holds) answers the Agent unchanged and records nothing.

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
    `guilds.Current(ctx)`, `guilds.MemberID(ctx)` (the Hirer), the Agent
    membership calls (create with Roles, add or remove a Role, end, read
    its Roles), `CanAssign` and the hierarchy (who ranks above a Hirer,
    which Roles are below the Hirer's highest), and the hook for a Member
    leaving or being removed. It registers `guilds.OnGuildDeleting`, so a
    Guild with Agents that are not terminated is not deleted.
  - from work: `work.RequestApproval` and `work.CancelApproval` for the
    `hire_agent` Approval, `work.OnApprovalDecided` (registered for
    `hire_agent`), `work.RecordActivity` for every event above, and
    `work.OnAgentNames` (registered, so the Activity knows which Agents
    still exist).
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
- **The Hirer owns the Agent, not the Board.** The Agent will run on the
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
- **No built-in agents.** A Guild starts with none; people hire them.
- **The Org chart is a view on the Agents page**, as in Paperclip's
  streamlined UI, whose `/org` route redirects there. Its SVG and PNG
  export are left out.
- **No Request revision for a hire.** A `pending_approval` Agent cannot be
  changed, so there is nothing to revise; see the work document.
