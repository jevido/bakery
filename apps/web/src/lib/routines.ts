// The work context's Routines: recurring work that, on a Schedule or when
// someone runs it, creates an Execution Issue for its Agent. Every call
// answers in the Current guild. The run status labels are Paperclip's
// (ui/src/components/RoutineList.tsx; MIT, see NOTICE).
import { api } from "./api";
import type { Priority } from "./work";

export const routineStatuses = ["active", "paused", "archived"] as const;
export type RoutineStatus = (typeof routineStatuses)[number];

export const concurrencyPolicies = [
  "coalesce_if_active",
  "always_enqueue",
  "skip_if_active",
] as const;
export type ConcurrencyPolicy = (typeof concurrencyPolicies)[number];

export const catchUpPolicies = ["skip_missed", "enqueue_missed_with_cap"] as const;
export type CatchUpPolicy = (typeof catchUpPolicies)[number];

/** Paperclip's one-line help for each policy. */
export const policyHelp: Record<ConcurrencyPolicy | CatchUpPolicy, string> = {
  coalesce_if_active: "If a run is already active, keep just one follow-up run queued.",
  always_enqueue: "Queue every trigger occurrence, even if the routine is already running.",
  skip_if_active: "Drop new trigger occurrences while a run is still active.",
  skip_missed: "Ignore windows that were missed while the scheduler or routine was paused.",
  enqueue_missed_with_cap:
    "Catch up missed schedule windows after recovery; sub-hourly schedules are combined into one catch-up run, slower schedules replay each missed window up to a cap.",
};

export type RoutineTriggerKind = "schedule" | "api";

export type RoutineTrigger = {
  id: number;
  kind: RoutineTriggerKind;
  label: string;
  enabled: boolean;
  cron_expression: string | null;
  timezone: string | null;
  next_run_at: string | null;
  last_fired_at: string | null;
  last_result: string | null;
};

export type RoutineRunSource = "schedule" | "manual" | "api" | "webhook";
export type RoutineRunStatus =
  | "received"
  | "issue_created"
  | "coalesced"
  | "skipped"
  | "completed"
  | "failed";

export type RoutineRun = {
  id: number;
  routine: { id: number; title: string };
  source: RoutineRunSource;
  status: RoutineRunStatus;
  triggered_at: string;
  completed_at: string | null;
  failure_reason: string | null;
  trigger: { id: number; kind: RoutineTriggerKind; label: string } | null;
  issue: { id: number; identifier: string; title: string; status: string } | null;
};

export type Routine = {
  id: number;
  title: string;
  description: string;
  project: { id: number; name: string } | null;
  goal: { id: number; title: string } | null;
  parent_issue: { id: number; identifier: string; title: string } | null;
  assignee_agent: { id: number; name: string; icon: string } | null;
  priority: Priority;
  status: RoutineStatus;
  concurrency_policy: ConcurrencyPolicy;
  catch_up_policy: CatchUpPolicy;
  last_triggered_at: string | null;
  created_at: string;
  updated_at: string;
  triggers: RoutineTrigger[];
  last_run: RoutineRun | null;
};

/** One Routine as GET /api/routines/{id} answers it, with its latest runs. */
export type RoutineDetail = Routine & { recent_runs: RoutineRun[] };

/** A Routine's editable fields; null clears a link. */
export type RoutineInput = {
  title: string;
  description: string;
  priority: Priority;
  status: RoutineStatus;
  concurrency_policy: ConcurrencyPolicy;
  catch_up_policy: CatchUpPolicy;
  project_id: number | null;
  goal_id: number | null;
  parent_issue_id: number | null;
  assignee_agent_id: number | null;
};

export type TriggerInput = {
  kind: RoutineTriggerKind;
  label: string;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
};

/** Paperclip's formatRoutineRunStatus: "issue_created" reads "issue created". */
export const routineRunStatusLabel = (s: string) => s.replaceAll("_", " ");

/** Paperclip's nextRoutineStatus: the toggle turns a Routine on or off. */
export const nextRoutineStatus = (status: RoutineStatus, enabled: boolean): RoutineStatus =>
  status === "archived" && enabled ? "active" : enabled ? "active" : "paused";

const routineOf = (r: { routine: RoutineDetail }) => r.routine;
const triggerOf = (r: { trigger: RoutineTrigger }) => r.trigger;

export const listRoutines = () =>
  api<{ routines: Routine[] }>("GET", "/routines").then((r) => r.routines);
export const getRoutine = (id: number) =>
  api<{ routine: RoutineDetail }>("GET", `/routines/${id}`).then(routineOf);
export const createRoutine = (input: Partial<RoutineInput> & { title: string }) =>
  api<{ routine: RoutineDetail }>("POST", "/routines", input).then(routineOf);
export const updateRoutine = (id: number, patch: Partial<RoutineInput>) =>
  api<{ routine: RoutineDetail }>("PATCH", `/routines/${id}`, patch).then(routineOf);
/** Runs it now: manual, or api when it names one of its api triggers. */
export const runRoutine = (id: number, triggerId?: number) =>
  api<{ routine_run: RoutineRun }>(
    "POST",
    `/routines/${id}/run`,
    triggerId ? { trigger_id: triggerId } : undefined,
  ).then((r) => r.routine_run);
export const addTrigger = (routineId: number, input: Partial<TriggerInput> & { kind: RoutineTriggerKind }) =>
  api<{ trigger: RoutineTrigger }>("POST", `/routines/${routineId}/triggers`, input).then(triggerOf);
export const updateTrigger = (id: number, patch: Partial<Omit<TriggerInput, "kind">>) =>
  api<{ trigger: RoutineTrigger }>("PATCH", `/routine-triggers/${id}`, patch).then(triggerOf);
export const deleteTrigger = (id: number) =>
  api<void>("DELETE", `/routine-triggers/${id}`);
export const routineRuns = (id: number, limit = 50) =>
  api<{ routine_runs: RoutineRun[] }>("GET", `/routines/${id}/runs?limit=${limit}`).then(
    (r) => r.routine_runs,
  );
/** The Current guild's Routine runs across the Routines the asker may view. */
export const recentRoutineRuns = (limit = 50) =>
  api<{ routine_runs: RoutineRun[] }>("GET", `/routine-runs?limit=${limit}`).then(
    (r) => r.routine_runs,
  );

/**
 * Paperclip's summarizeRoutineSchedule (ui/src/components/RoutineOverview.tsx):
 * the Routine's enabled Schedules in a line, the first one's cron and time
 * zone under it, and the soonest Next run.
 */
export function summarizeSchedule(triggers: RoutineTrigger[]): { label: string; detail: string; nextRunAt: string | null } {
  const schedules = triggers.filter((t) => t.kind === "schedule" && t.enabled);
  const nextRunAt =
    schedules
      .map((t) => t.next_run_at)
      .filter((at): at is string => at !== null)
      .sort((a, b) => Date.parse(a) - Date.parse(b))[0] ?? null;
  if (schedules.length === 0) {
    const apis = triggers.filter((t) => t.kind === "api" && t.enabled).length;
    return apis > 0
      ? { label: `${apis} API trigger${apis === 1 ? "" : "s"}`, detail: "Runs when the API is called", nextRunAt: null }
      : { label: "No active schedule", detail: "Manual runs only", nextRunAt: null };
  }
  const first = schedules[0];
  return {
    label: schedules.length === 1 ? "1 active schedule" : `${schedules.length} active schedules`,
    detail: `${first.cron_expression ?? ""}${first.timezone ? ` · ${first.timezone}` : ""}`,
    nextRunAt,
  };
}

/**
 * The Routine's state as its page shows it, Paperclip's automationLabel: a
 * Routine without an Agent is a Draft whatever its status.
 */
export function routineState(r: Pick<Routine, "status" | "assignee_agent">): "archived" | "draft" | "active" | "paused" {
  if (r.status === "archived") return "archived";
  return r.assignee_agent ? r.status : "draft";
}

/** Paperclip's RoutineActivityRow labels for the routine.* Actions. */
const routineActionLabels: Record<string, string> = {
  "routine.created": "Routine created",
  "routine.updated": "Routine updated",
  "routine.archived": "Routine archived",
  "routine.trigger_created": "Trigger added",
  "routine.trigger_updated": "Trigger updated",
  "routine.trigger_deleted": "Trigger removed",
  "routine.trigger_secret_rotated": "Webhook secret rotated",
  "routine.run_triggered": "Routine started",
  "routine.webhook_rejected": "Webhook delivery rejected",
};
export const routineActionLabel = (action: string) =>
  routineActionLabels[action] ?? action.replace(/^routine\./, "").replaceAll("_", " ");

/** Paperclip's summarizeEvent: the one line of what a routine.* event says. */
export function routineEventSummary(action: string, details: Record<string, unknown>): string {
  const str = (v: unknown) => (typeof v === "string" ? v : "");
  if (action === "routine.run_triggered") {
    const source = str(details.source);
    return `${source ? source[0].toUpperCase() + source.slice(1) : "Run"} · ${routineRunStatusLabel(str(details.status))}`;
  }
  if (action.startsWith("routine.trigger_") || action === "routine.webhook_rejected") {
    const name = str(details.label) || ({ api: "API", webhook: "Webhook" }[str(details.kind)] ?? "Schedule");
    if (action === "routine.webhook_rejected") return `${name} · ${str(details.reason)}`;
    const changes = details.changes as Record<string, unknown> | undefined;
    if (changes) return `${name} · ${Object.keys(changes).map((k) => k.replaceAll("_", " ")).join(", ")}`;
    return details.cron_expression ? `${name} · ${details.cron_expression} · ${str(details.timezone)}` : name;
  }
  if (action === "routine.updated" && details.changes) {
    return Object.keys(details.changes as Record<string, unknown>)
      .map((k) => k.replaceAll("_", " "))
      .join(", ");
  }
  return "";
}
